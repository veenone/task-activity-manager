package issuerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agile-suite/core/journal"
)

// DiscardPendingChange reverts one journal row: a field edit goes back to
// its before value, a create row takes its draft row with it.
func (r *Repository) DiscardPendingChange(ctx context.Context, profileID string, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := journal.Get(tx, profileID, id)
	if err != nil {
		return err
	}
	if err := discardOne(ctx, tx, profileID, p); err != nil {
		return err
	}
	return tx.Commit()
}

// DiscardAllPendingChanges reverts every journal row of the profile and
// returns how many it reverted.
func (r *Repository) DiscardAllPendingChanges(ctx context.Context, profileID string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	all, err := journal.List(tx, profileID)
	if err != nil {
		return 0, err
	}
	for _, p := range all {
		if err := discardOne(ctx, tx, profileID, p); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(all), nil
}

// DiscardKey reverts every pending change of one issue, which is what a
// keep-remote resolution does before it replaces the row.
func (r *Repository) DiscardKey(ctx context.Context, profileID, key string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := journal.ListForKey(tx, profileID, key)
	if err != nil {
		return 0, err
	}
	for _, p := range rows {
		if err := discardOne(ctx, tx, profileID, p); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func discardOne(ctx context.Context, tx *sql.Tx, profileID string, p journal.PendingChange) error {
	switch {
	case p.EntityType == EntityIssueCreate:
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key = ?`, profileID, p.EntityKey); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue WHERE profile_id = ? AND key = ?`, profileID, p.EntityKey); err != nil {
			return fmt.Errorf("drop draft %s: %w", p.EntityKey, err)
		}
	case p.EntityType == EntityLink:
		// A link that was never pushed: nothing on the row to revert.
	default:
		var exists int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM issue WHERE profile_id = ? AND key = ?`, profileID, p.EntityKey).Scan(&exists)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// A full sync no longer returns this issue. There is no row left
			// to revert; still drop the journal row and record the discard.
		case err != nil:
			return fmt.Errorf("check %s exists: %w", p.EntityKey, err)
		default:
			if err := writeField(ctx, tx, profileID, p.EntityKey, p.Field, p.BeforeVal); err != nil {
				return fmt.Errorf("revert %s.%s: %w", p.EntityKey, p.Field, err)
			}
		}
	}
	if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "discard", p.Field, p.AfterVal, p.BeforeVal, "")
}

// MarkCommitted deletes the journal rows a commit pushed and audits each.
func (r *Repository) MarkCommitted(ctx context.Context, profileID string, changes []journal.PendingChange) error {
	if len(changes) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := make([]int64, 0, len(changes))
	for _, p := range changes {
		ids = append(ids, p.ID)
		if err := journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "commit", p.Field, p.BeforeVal, p.AfterVal, ""); err != nil {
			return err
		}
	}
	if err := journal.Delete(tx, profileID, ids); err != nil {
		return err
	}
	return tx.Commit()
}
