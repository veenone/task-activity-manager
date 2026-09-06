package issuerepo

import (
	"context"
	"fmt"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// Rekey moves a draft to the key Jira assigned, across the row, its links,
// its journal rows, and its audit trail, and audits the creation.
func (r *Repository) Rekey(ctx context.Context, profileID, tempKey, realKey string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`UPDATE issue SET key = ? WHERE profile_id = ? AND key = ?`,
		`UPDATE issue_link SET from_key = ? WHERE profile_id = ? AND from_key = ?`,
		`UPDATE pending_change SET entity_key = ? WHERE profile_id = ? AND entity_key = ?`,
		`UPDATE audit_log SET entity_key = ? WHERE profile_id = ? AND entity_key = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, realKey, profileID, tempKey); err != nil {
			return fmt.Errorf("rekey %s to %s: %w", tempKey, realKey, err)
		}
	}
	if err := journal.Audit(tx, profileID, EntityIssue, realKey, "created", "", tempKey, realKey, "created in Jira"); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceRow overwrites every column of one issue from a fresh Jira read and
// drops its detail cache so the panel refetches. It does not touch the
// journal; callers delete the rows first when that is the intent.
func (r *Repository) ReplaceRow(ctx context.Context, profileID string, iss backend.Issue) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertIssue(ctx, tx, profileID, iss, time.Now()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE issue SET detail_json = NULL, detail_fetched_at = NULL WHERE profile_id = ? AND key = ?`,
		profileID, iss.Key); err != nil {
		return fmt.Errorf("clear detail for %s: %w", iss.Key, err)
	}
	// A pending edit that was not part of the push this row came from (made
	// while the commit was in flight, or left behind because only some of the
	// issue's fields were pushed) must survive the overwrite.
	if err := reapplyPending(ctx, tx, profileID, map[string]bool{iss.Key: true}); err != nil {
		return err
	}
	return tx.Commit()
}

// SetBaseVersion rebases an issue's pending changes onto the remote version
// the user chose to override, and audits the choice.
func (r *Repository) SetBaseVersion(ctx context.Context, profileID, key, version string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := journal.SetBaseVersion(tx, profileID, key, version); err != nil {
		return err
	}
	if err := journal.Audit(tx, profileID, EntityIssue, key, "override", "", "", version, "pending edits rebased onto the remote version"); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkCreatedWithoutRekey clears a draft's journal rows when Jira accepted
// the create but the local rename to the real key failed: it deletes the
// create (and any edit) rows under the temp key, same as MarkCommitted, then
// audits the creation under the temp key with the real key in the note, so a
// retry sees nothing pending and does not post the draft again. The draft
// row keeps its temporary key; the next sync brings the real row in.
func (r *Repository) MarkCreatedWithoutRekey(ctx context.Context, profileID, tempKey, realKey string, rows []journal.PendingChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := make([]int64, 0, len(rows))
	for _, p := range rows {
		ids = append(ids, p.ID)
		if err := journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "commit", p.Field, p.BeforeVal, p.AfterVal, ""); err != nil {
			return err
		}
	}
	if err := journal.Delete(tx, profileID, ids); err != nil {
		return err
	}
	note := fmt.Sprintf("created in Jira as %s but the local rename failed; the row keeps its temporary key until the next sync", realKey)
	if err := journal.Audit(tx, profileID, EntityIssue, tempKey, "created", "", tempKey, realKey, note); err != nil {
		return err
	}
	return tx.Commit()
}
