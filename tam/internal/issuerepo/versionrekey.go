package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// RekeyVersion turns a draft release Jira has just created into the real
// one: its negative id becomes made.ID on every issue, pending edit, draft
// issue and release change that named it, its row takes Jira's fields and
// stops being a draft, and its version_create row is cleared. A draft whose
// create row is gone (discarded while Commit ran) answers
// ErrDraftVersionGone and changes nothing.
func (r *Repository) RekeyVersion(ctx context.Context, profileID string, draftID int, made backend.Version) error {
	key := strconv.Itoa(draftID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		p, ok, err := pendingSprintRow(ctx, tx, profileID, EntityVersionCreate, key)
		if err != nil {
			return err
		}
		if !ok {
			return ErrDraftVersionGone
		}
		if err := rewriteVersionID(ctx, tx, profileID, draftID, made.ID); err != nil {
			return err
		}
		// A sync that ran while Commit did may have cached Jira's copy
		// already; the draft row carries the same release and wins.
		if _, err := tx.ExecContext(ctx, `DELETE FROM project_version WHERE profile_id = ? AND id = ?`, profileID, made.ID); err != nil {
			return fmt.Errorf("clear release %d before the rekey: %w", made.ID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE project_version SET id = ?, name = ?, description = ?, start_date = ?, release_date = ?, draft = 0
			 WHERE profile_id = ? AND id = ? AND draft = 1`,
			made.ID, made.Name, made.Description, made.StartDate, made.ReleaseDate, profileID, draftID); err != nil {
			return fmt.Errorf("rekey release %s to %d: %w", key, made.ID, err)
		}
		if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityVersionCreate, key, "commit", "", "", strconv.Itoa(made.ID), "created in Jira as release "+strconv.Itoa(made.ID))
	})
}

// ApplyVersionCommitted is the cache work after Commit pushed one release
// change, done in the transaction that clears its row: a release marks the
// row released on its date, an archive sets the archived flag, and a delete
// moves the release's issues to the delete's target, as Jira just did, and
// drops the row. An edit's change is already in the cache.
func (r *Repository) ApplyVersionCommitted(ctx context.Context, profileID string, p journal.PendingChange) error {
	id, err := strconv.Atoi(p.EntityKey)
	if err != nil {
		return fmt.Errorf("release id %q: %w", p.EntityKey, err)
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		switch p.EntityType {
		case EntityVersionRelease:
			var v VersionRelease
			if err := json.Unmarshal([]byte(p.AfterVal), &v); err != nil {
				return fmt.Errorf("decode the release of %s: %w", p.EntityKey, err)
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE project_version SET released = 1, release_date = CASE WHEN ? = '' THEN release_date ELSE ? END WHERE profile_id = ? AND id = ?`,
				v.ReleaseDate, v.ReleaseDate, profileID, id); err != nil {
				return fmt.Errorf("mark release %d released: %w", id, err)
			}
		case EntityVersionArchive:
			var v VersionArchive
			if err := json.Unmarshal([]byte(p.AfterVal), &v); err != nil {
				return fmt.Errorf("decode the archive of %s: %w", p.EntityKey, err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE project_version SET archived = ? WHERE profile_id = ? AND id = ?`, v.Archived, profileID, id); err != nil {
				return fmt.Errorf("mark release %d archived: %w", id, err)
			}
		case EntityVersionDelete:
			var v VersionDelete
			if err := json.Unmarshal([]byte(p.AfterVal), &v); err != nil {
				return fmt.Errorf("decode the delete of %s: %w", p.EntityKey, err)
			}
			if err := rewriteIssueVersions(ctx, tx, profileID, id, v.MoveTo); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM project_version WHERE profile_id = ? AND id = ?`, profileID, id); err != nil {
				return fmt.Errorf("drop release %d: %w", id, err)
			}
		}
		if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "commit", p.Field, p.BeforeVal, p.AfterVal, "")
	})
}
