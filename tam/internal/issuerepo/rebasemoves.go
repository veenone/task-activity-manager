package issuerepo

import (
	"context"
	"database/sql"
	"fmt"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// RebaseMoves is what Override means for a board row. A pending transition
// or sprint move is held back as a conflict when the row's before_val no
// longer matches the status or sprint Jira holds, and nothing about that
// comparison reads a base version, so rebasing the edits alone would leave
// the user meeting the identical conflict on every Commit from here on.
// Rewriting before_val to the remote value is the rebase: the next
// classification sees a card sitting where the journal says it was and
// pushes the move the user made.
//
// The after_val is left exactly as it was. Override takes the user's move
// anyway; only what it was measured against changes. A rank has no
// before_val to rebase and is never held back, so it is not touched.
func (r *Repository) RebaseMoves(ctx context.Context, profileID, key string, remote backend.Issue) error {
	rows := []struct {
		entityType string
		field      string
		before     string
	}{
		{EntityTransition, FieldStatusID, MoveValue(remote.StatusID, remote.Status)},
		{EntitySprintMove, FieldSprintID, MoveValue(remote.SprintID, remote.SprintName)},
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		for _, row := range rows {
			res, err := tx.ExecContext(ctx,
				`UPDATE pending_change SET before_val = ? WHERE profile_id = ? AND entity_key = ? AND entity_type = ?`,
				row.before, profileID, key, row.entityType)
			if err != nil {
				return fmt.Errorf("rebase the %s of %s: %w", row.entityType, key, err)
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			if err := journal.Audit(tx, profileID, row.entityType, key, "override", row.field, "", row.before,
				"the pending move rebased onto what Jira holds now"); err != nil {
				return err
			}
		}
		return nil
	})
}
