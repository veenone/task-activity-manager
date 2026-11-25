package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// RemoveLink journals the removal of key's cached link l (#612). Jira
// deletes a link by its id, so l must carry the id a sync stored; the cached
// row stays until Commit lands the delete.
func (r *Repository) RemoveLink(ctx context.Context, profileID, key string, l backend.Link) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := removeLinkTx(ctx, tx, profileID, key, l); err != nil {
		return err
	}
	return tx.Commit()
}

// RetypeLink journals a change of l's type as a removal of l and an add of
// the same link under newType, removal first, so Commit deletes before it
// creates.
func (r *Repository) RetypeLink(ctx context.Context, profileID, key string, l backend.Link, newType string) error {
	newType = strings.TrimSpace(newType)
	if strings.EqualFold(newType, l.Type) {
		return fmt.Errorf("the link is already %s", l.Type)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := removeLinkTx(ctx, tx, profileID, key, l); err != nil {
		return err
	}
	add := backend.LinkDraft{Type: newType, Direction: l.Direction, ToKey: l.Key, ToSummary: l.Summary, ToType: l.IssueType}
	if err := addLinkTx(ctx, tx, profileID, key, add); err != nil {
		return err
	}
	return tx.Commit()
}

func removeLinkTx(ctx context.Context, tx *sql.Tx, profileID, key string, l backend.Link) error {
	if l.ID == "" {
		return errors.New("this link has no Jira id yet; sync, then remove it")
	}
	var exists int
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM issue_link WHERE profile_id = ? AND from_key = ? AND link_id = ?`,
		profileID, key, l.ID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s has no cached link %s", key, l.ID)
	} else if err != nil {
		return fmt.Errorf("check link exists: %w", err)
	}
	d := backend.LinkDraft{Type: l.Type, Direction: l.Direction, ToKey: l.Key, ToSummary: l.Summary, ToType: l.IssueType, RemoveID: l.ID}
	field := LinkField(d)
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM pending_change WHERE profile_id = ? AND entity_type = ?
		 AND json_extract(after_val, '$.removeId') = ?`,
		profileID, EntityLink, l.ID).Scan(&exists); err == nil {
		return fmt.Errorf("the removal of the link to %s is already pending", l.Key)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check pending removal: %w", err)
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode link removal: %w", err)
	}
	if err := journal.Put(tx, profileID, EntityLink, key, field, "", string(encoded), ""); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, EntityLink, key, "unlink", field, l.Key, "", "")
}

// DropCachedLink deletes the cached rows of the link with Jira's id, on both
// of its ends. Commit runs it once Jira has deleted the link.
func (r *Repository) DropCachedLink(ctx context.Context, profileID, linkID string) error {
	if linkID == "" {
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND link_id = ?`, profileID, linkID); err != nil {
		return fmt.Errorf("drop cached link %s: %w", linkID, err)
	}
	return nil
}
