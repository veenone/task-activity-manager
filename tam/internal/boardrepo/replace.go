package boardrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"agile-suite/tam/internal/backend"
)

// ReplaceBoard writes everything one board holds in a single transaction:
// its row, its columns, its sprints, and the membership of every scope the
// sync read for it, keyed by sprint id with "" for the board's own list.
// It is the only write path into the four board tables, and one
// transaction is the reason: a reader must never catch a board with new
// columns against old membership.
//
// Every one of the board's board_issue rows is deleted before the scopes
// are inserted, not just the scopes being written, so a sprint that stopped
// being active, or was deleted in Jira, does not leave its membership
// behind for good.
//
// The row is stamped with the wall clock rather than the pass's start time:
// synced_at is a record of when the row was written and nothing reads it
// back.
func (r *Repository) ReplaceBoard(ctx context.Context, profileID string, b backend.Board, cols []backend.BoardColumn, sprints []backend.Sprint, keys map[string][]string) error {
	stamp := time.Now().UTC().Format(time.RFC3339)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if err := writeBoardRow(ctx, tx, profileID, b, stamp); err != nil {
			return err
		}
		if err := writeColumns(ctx, tx, profileID, b.ID, cols); err != nil {
			return err
		}
		if err := writeSprints(ctx, tx, profileID, b.ID, sprints); err != nil {
			return err
		}
		if err := deleteBoardIssues(ctx, tx, profileID, b.ID); err != nil {
			return err
		}
		for sprintID, k := range keys {
			if err := writeIssueKeys(ctx, tx, profileID, b.ID, sprintID, k); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeBoardRow inserts or updates one board's row.
func writeBoardRow(ctx context.Context, tx *sql.Tx, profileID string, b backend.Board, stamp string) error {
	if _, err := tx.ExecContext(ctx, upsertBoardSQL, profileID, b.ID, b.Name, b.Type, stamp); err != nil {
		return fmt.Errorf("upsert board %d: %w", b.ID, err)
	}
	return nil
}

// writeColumns replaces one board's columns, delete then insert.
func writeColumns(ctx context.Context, tx *sql.Tx, profileID string, boardID int, cols []backend.BoardColumn) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM board_column WHERE profile_id = ? AND board_id = ?`, profileID, boardID); err != nil {
		return fmt.Errorf("clear columns of board %d: %w", boardID, err)
	}
	for i, c := range cols {
		ids, err := json.Marshal(backend.NonNil(c.StatusIDs))
		if err != nil {
			return fmt.Errorf("status ids of column %q: %w", c.Name, err)
		}
		if _, err := tx.ExecContext(ctx, insertColumnSQL, profileID, boardID, i, c.Name, string(ids)); err != nil {
			return fmt.Errorf("insert column %q: %w", c.Name, err)
		}
	}
	return nil
}

// writeSprints replaces one board's sprints, delete then insert.
func writeSprints(ctx context.Context, tx *sql.Tx, profileID string, boardID int, sprints []backend.Sprint) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM sprint WHERE profile_id = ? AND board_id = ?`, profileID, boardID); err != nil {
		return fmt.Errorf("clear sprints of board %d: %w", boardID, err)
	}
	for _, s := range sprints {
		if _, err := tx.ExecContext(ctx, insertSprintSQL, profileID, s.ID, boardID, s.Name, s.State, s.StartDate, s.EndDate); err != nil {
			return fmt.Errorf("insert sprint %d: %w", s.ID, err)
		}
	}
	return nil
}

// writeIssueKeys inserts one scope's membership in board order, keeping the
// first sighting of a key and skipping any repeat. Clearing what was there
// is the caller's: ReplaceBoard drops the whole board first.
//
// A repeat is ordinary rather than exceptional: the board endpoints are
// walked with startAt, and a card whose rank changes between two pages
// comes back on both. The scope's key is (profile_id, board_id, sprint_id,
// key), so inserting the repeat would abort the board's whole write over a
// card that is already there.
func writeIssueKeys(ctx context.Context, tx *sql.Tx, profileID string, boardID int, sprintID string, keys []string) error {
	seen := make(map[string]bool, len(keys))
	position := 0
	for _, key := range keys {
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, err := tx.ExecContext(ctx, insertIssueKeySQL, profileID, boardID, sprintID, key, position); err != nil {
			return fmt.Errorf("insert board key %s: %w", key, err)
		}
		position++
	}
	return nil
}

// deleteBoardIssues clears every scope one board holds, the board's own
// list and each of its sprints.
func deleteBoardIssues(ctx context.Context, tx *sql.Tx, profileID string, boardID int) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM board_issue WHERE profile_id = ? AND board_id = ?`, profileID, boardID); err != nil {
		return fmt.Errorf("clear issue keys of board %d: %w", boardID, err)
	}
	return nil
}
