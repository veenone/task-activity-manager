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
//
// Only the row that actually disagrees with Jira is rebased. An issue is
// held back whole, so an edits-only conflict reaches here beside board rows
// the commit pass had classified as ready to push or as already satisfied,
// and rewriting those would audit an override that overrode nothing.
func (r *Repository) RebaseMoves(ctx context.Context, profileID, key string, remote backend.Issue) error {
	rows := []struct {
		entityType string
		field      string
		remoteID   string
		before     string
	}{
		{EntityTransition, FieldStatusID, remote.StatusID, MoveValue(remote.StatusID, remote.Status)},
		{EntitySprintMove, FieldSprintID, remote.SprintID, MoveValue(remote.SprintID, remote.SprintName)},
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		for _, row := range rows {
			if !heldMove(ctx, tx, profileID, key, row.entityType, row.field, row.remoteID) {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE pending_change SET before_val = ? WHERE profile_id = ? AND entity_key = ? AND entity_type = ? AND field = ?`,
				row.before, profileID, key, row.entityType, row.field); err != nil {
				return fmt.Errorf("rebase the %s of %s: %w", row.entityType, key, err)
			}
			if err := journal.Audit(tx, profileID, row.entityType, key, "override", row.field, "", row.before,
				"the pending move rebased onto what Jira holds now"); err != nil {
				return err
			}
		}
		return nil
	})
}

// heldMove is the committer's own classification, asked again here: a row
// exists, and its before and its after both disagree with what Jira holds,
// which is neither a push nor a satisfaction but a conflict. A read error
// answers false rather than failing the resolution, since the worst it
// costs is a rebase this Override did not need; the next Commit says so
// either way.
func heldMove(ctx context.Context, q execer, profileID, key, entityType, field, remoteID string) bool {
	before, after, held, err := readJournaledMove(ctx, q, profileID, entityType, key, field)
	if err != nil || !held {
		return false
	}
	return MoveID(before) != remoteID && MoveID(after) != remoteID
}
