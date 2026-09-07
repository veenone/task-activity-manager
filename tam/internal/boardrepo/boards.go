package boardrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"agile-suite/tam/internal/backend"
)

const upsertBoardSQL = `
	INSERT INTO board (profile_id, id, name, type, synced_at) VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(profile_id, id) DO UPDATE SET
		name = excluded.name, type = excluded.type, synced_at = excluded.synced_at`

const insertColumnSQL = `
	INSERT INTO board_column (profile_id, board_id, position, name, status_ids) VALUES (?, ?, ?, ?, ?)`

const insertSprintSQL = `
	INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date) VALUES (?, ?, ?, ?, ?, ?, ?)`

const insertIssueKeySQL = `
	INSERT INTO board_issue (profile_id, board_id, sprint_id, key, position) VALUES (?, ?, ?, ?, ?)`

const listBoardsSQL = `
	SELECT id, name, type FROM board WHERE profile_id = ? ORDER BY name, id`

const columnsSQL = `
	SELECT name, status_ids FROM board_column WHERE profile_id = ? AND board_id = ? ORDER BY position`

// listSprintsSQL puts the sprint the user most likely wants first: the
// active one, then the future ones, then what is closed, each group by start
// date. Jira sends the state lowercase and the backends keep it that way, so
// the CASE matches without folding.
const listSprintsSQL = `
	SELECT id, board_id, name, state, start_date, end_date FROM sprint
	WHERE profile_id = ? AND board_id = ?
	ORDER BY CASE state WHEN 'active' THEN 0 WHEN 'future' THEN 1 WHEN 'closed' THEN 2 ELSE 3 END, start_date, id`

const boardKeysSQL = `
	SELECT key FROM board_issue WHERE profile_id = ? AND board_id = ? AND sprint_id = ? ORDER BY position`

// UpsertBoards writes the boards a sync found. It adds and updates but never
// removes: a board that vanished from Jira is the sync's to spot, and
// RemoveBoards takes it away with its children.
func (r *Repository) UpsertBoards(ctx context.Context, profileID string, boards []backend.Board, syncedAt time.Time) error {
	stamp := syncedAt.UTC().Format(time.RFC3339)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		for _, b := range boards {
			if _, err := tx.ExecContext(ctx, upsertBoardSQL, profileID, b.ID, b.Name, b.Type, stamp); err != nil {
				return fmt.Errorf("upsert board %d: %w", b.ID, err)
			}
		}
		return nil
	})
}

// UpsertColumns replaces one board's columns. It deletes before it inserts,
// in one transaction, because a board whose columns went from five to three
// would otherwise keep the two Jira no longer has, and position is the
// primary key so nothing would even collide to reveal it.
func (r *Repository) UpsertColumns(ctx context.Context, profileID string, boardID int, cols []backend.BoardColumn) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM board_column WHERE profile_id = ? AND board_id = ?`, profileID, boardID); err != nil {
			return fmt.Errorf("clear columns of board %d: %w", boardID, err)
		}
		for i, c := range cols {
			ids, err := json.Marshal(nonNil(c.StatusIDs))
			if err != nil {
				return fmt.Errorf("status ids of column %q: %w", c.Name, err)
			}
			if _, err := tx.ExecContext(ctx, insertColumnSQL, profileID, boardID, i, c.Name, string(ids)); err != nil {
				return fmt.Errorf("insert column %q: %w", c.Name, err)
			}
		}
		return nil
	})
}

// UpsertSprints replaces one board's sprints, delete then insert in one
// transaction: a sprint deleted in Jira would otherwise stay in the picker
// forever, offering a sprint nobody can open.
func (r *Repository) UpsertSprints(ctx context.Context, profileID string, boardID int, sprints []backend.Sprint) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sprint WHERE profile_id = ? AND board_id = ?`, profileID, boardID); err != nil {
			return fmt.Errorf("clear sprints of board %d: %w", boardID, err)
		}
		for _, s := range sprints {
			if _, err := tx.ExecContext(ctx, insertSprintSQL, profileID, s.ID, boardID, s.Name, s.State, s.StartDate, s.EndDate); err != nil {
				return fmt.Errorf("insert sprint %d: %w", s.ID, err)
			}
		}
		return nil
	})
}

// UpsertIssueKeys replaces the membership of one board and sprint, delete
// then insert in one transaction. An insert alone only ever adds, so a card
// moved out of the sprint, or off the board, would keep being drawn in the
// place it left.
func (r *Repository) UpsertIssueKeys(ctx context.Context, profileID string, boardID int, sprintID string, keys []string) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM board_issue WHERE profile_id = ? AND board_id = ? AND sprint_id = ?`, profileID, boardID, sprintID); err != nil {
			return fmt.Errorf("clear issue keys of board %d: %w", boardID, err)
		}
		for i, key := range keys {
			if _, err := tx.ExecContext(ctx, insertIssueKeySQL, profileID, boardID, sprintID, key, i); err != nil {
				return fmt.Errorf("insert board key %s: %w", key, err)
			}
		}
		return nil
	})
}

// RemoveBoards drops the boards and everything hanging off them: their
// columns, their issue keys, and their sprints, in one transaction.
func (r *Repository) RemoveBoards(ctx context.Context, profileID string, boardIDs []int) error {
	if len(boardIDs) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		for _, id := range boardIDs {
			for _, table := range []string{"board_column", "board_issue", "sprint", "board"} {
				column := "board_id"
				if table == "board" {
					column = "id"
				}
				if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE profile_id = ? AND `+column+` = ?`, profileID, id); err != nil {
					return fmt.Errorf("remove %s of board %d: %w", table, id, err)
				}
			}
		}
		return nil
	})
}

// ListBoards returns the profile's cached boards by name.
func (r *Repository) ListBoards(ctx context.Context, profileID string) ([]Board, error) {
	rows, err := r.db.QueryContext(ctx, listBoardsSQL, profileID)
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	defer rows.Close()
	out := []Board{}
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Name, &b.Type); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Columns returns one board's columns in board order.
func (r *Repository) Columns(ctx context.Context, profileID string, boardID int) ([]backend.BoardColumn, error) {
	rows, err := r.db.QueryContext(ctx, columnsSQL, profileID, boardID)
	if err != nil {
		return nil, fmt.Errorf("board %d columns: %w", boardID, err)
	}
	defer rows.Close()
	out := []backend.BoardColumn{}
	for rows.Next() {
		var (
			c   backend.BoardColumn
			ids string
		)
		if err := rows.Scan(&c.Name, &ids); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(ids), &c.StatusIDs); err != nil {
			return nil, fmt.Errorf("status ids of column %q: %w", c.Name, err)
		}
		c.StatusIDs = nonNil(c.StatusIDs)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListSprints returns one board's sprints, active first, then future, then
// closed, each group by start date.
func (r *Repository) ListSprints(ctx context.Context, profileID string, boardID int) ([]Sprint, error) {
	rows, err := r.db.QueryContext(ctx, listSprintsSQL, profileID, boardID)
	if err != nil {
		return nil, fmt.Errorf("board %d sprints: %w", boardID, err)
	}
	defer rows.Close()
	out := []Sprint{}
	for rows.Next() {
		var s Sprint
		if err := rows.Scan(&s.ID, &s.BoardID, &s.Name, &s.State, &s.StartDate, &s.EndDate); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// issueKeys returns the keys one board holds for a sprint, in board order.
// An empty sprintID reads the board's own list, the way the sync stored it.
func (r *Repository) issueKeys(ctx context.Context, profileID string, boardID int, sprintID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, boardKeysSQL, profileID, boardID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("board %d issue keys: %w", boardID, err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

// inTx runs fn inside one transaction, so a replace never leaves the table
// holding a delete without its inserts.
func (r *Repository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
