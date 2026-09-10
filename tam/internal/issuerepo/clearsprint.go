package issuerepo

import (
	"context"
	"fmt"
)

// ClearSprint blanks the sprint columns of every cached issue that still
// names one deleted sprint, so the Backlog and Epics sprint filters and the
// detail panel's Sprint field stop offering a sprint Jira no longer has the
// moment the delete lands, rather than only after the next full sync.
// Without it SprintField would keep re-adding the deleted sprint as its own
// option from sprint_name, and ListSprints would keep feeding it to those
// same two filters, so a deleted sprint would linger in three views.
//
// It is scoped by the cached sprint_id column, not by a join back to
// board_issue. That column is written optimistically, ahead of Commit, by
// every board move (applyMoveColumns), so an issue with a pending move to a
// different sprint already carries that sprint's id here and does not match
// the sprint being deleted; it is left alone, pending move and all.
//
// It runs AFTER boardrepo.DeleteSprintEverywhere, in its own transaction.
// That comment carries the reason the two cannot share one and what a crash
// between them leaves. sprintID is the string the issue cache stores, where
// the board tables key the same sprint by an integer.
func (r *Repository) ClearSprint(ctx context.Context, profileID, sprintID string) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE issue SET sprint_id = '', sprint_name = '' WHERE profile_id = ? AND sprint_id = ?`,
		profileID, sprintID); err != nil {
		return fmt.Errorf("clear sprint %s: %w", sprintID, err)
	}
	return nil
}
