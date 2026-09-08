package issuerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agile-suite/core/journal"
)

// What a journaled board value means in columns, and what putting one back
// is allowed to overwrite. The write path, the undo, the Discard path, and
// the post-sync replay all go through this file, so a card can only ever be
// in one of the places the journal accounts for.

// applyMoveColumns writes what a board value means on the issue row, so the
// board and the Backlog repaint together.
//
// The name columns take the name exactly as it was journaled, empty
// included. Writing the id there instead would fabricate a status name for
// an id the cache has never seen, statusNameFor would read that back as a
// real name, and the next card dropped on the same column would journal it
// as one.
func applyMoveColumns(ctx context.Context, q execer, profileID, key, entityType, value string) error {
	switch entityType {
	case EntityTransition:
		if _, err := q.ExecContext(ctx,
			`UPDATE issue SET status = ?, status_id = ? WHERE profile_id = ? AND key = ?`,
			MoveRawName(value), MoveID(value), profileID, key); err != nil {
			return fmt.Errorf("move %s to status %s: %w", key, MoveID(value), err)
		}
		return nil
	case EntitySprintMove:
		id, name := MoveID(value), ""
		if id != "" {
			name = MoveRawName(value)
		}
		if _, err := q.ExecContext(ctx,
			`UPDATE issue SET sprint_id = ?, sprint_name = ? WHERE profile_id = ? AND key = ?`,
			id, name, profileID, key); err != nil {
			return fmt.Errorf("move %s to sprint %s: %w", key, id, err)
		}
		return nil
	case EntityRank:
		// Decision 4: a rank stays out of the cache.
		return nil
	}
	return fmt.Errorf("%q is not a board move", entityType)
}

// holdsMove says whether the row's column still holds the value a journal
// row put there. It is the one question asked before any board value is put
// back, by both the Discard path and the undo a drag home makes.
//
// The version the intent was journaled against cannot answer it. A sync
// that only freshened the updated stamp, because someone commented on the
// issue in Jira, leaves the column exactly where this app wrote it: the
// replay puts the pending target back the moment the sync writes the row.
// Comparing stamps would read that as Jira having moved the card and leave
// it at a target no journal row accounts for any more.
func holdsMove(entityType, current, journaled string) bool {
	switch entityType {
	case EntityTransition, EntitySprintMove:
		return MoveID(current) == MoveID(journaled)
	}
	return false
}

// currentMoveValue is the value the row holds now for one board type, in
// the same packing the journal uses.
func currentMoveValue(row boardRow, entityType string) string {
	switch entityType {
	case EntityTransition:
		return MoveValue(row.statusID, row.status)
	case EntitySprintMove:
		return MoveValue(row.sprintID, row.sprintName)
	}
	return ""
}

// revertMove puts back the columns one board row changed. A rank has none.
// A row whose column has genuinely moved on, so that it no longer holds the
// value this row wrote, keeps what it has: that value came from Jira by way
// of a sync, and writing a stale before_val over it is worse than leaving
// it for the next sync to settle.
func revertMove(ctx context.Context, tx *sql.Tx, profileID string, p journal.PendingChange) error {
	if p.EntityType == EntityRank {
		return nil
	}
	row, err := readBoardRow(ctx, tx, profileID, p.EntityKey)
	if errors.Is(err, ErrNotFound) {
		// A full sync no longer returns this issue; there is no row left to
		// revert, and the journal row still goes.
		return nil
	}
	if err != nil {
		return err
	}
	if !holdsMove(p.EntityType, currentMoveValue(row, p.EntityType), p.AfterVal) {
		return nil
	}
	return applyMoveColumns(ctx, tx, profileID, p.EntityKey, p.EntityType, p.BeforeVal)
}
