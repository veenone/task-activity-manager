package boardrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/dbtx"
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

const sprintNameSQL = `
	SELECT name FROM sprint WHERE profile_id = ? AND id = ? AND name <> '' LIMIT 1`

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
		c.StatusIDs = backend.NonNil(c.StatusIDs)
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

// SprintName is the name of one sprint id, empty when no board of the
// profile holds it. It is what a caller journaling a move to that sprint
// names the destination with: the sprint list is this package's table, and
// a future sprint holds no issues to borrow a name from, so the issue cache
// cannot answer for it. Jira hands the same sprint to every board whose
// filter reaches it, under one name, so which board it is read from does
// not change the answer.
func (r *Repository) SprintName(ctx context.Context, profileID, sprintID string) (string, error) {
	if strings.TrimSpace(sprintID) == "" {
		return "", nil
	}
	var name string
	err := r.db.QueryRowContext(ctx, sprintNameSQL, profileID, sprintID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("name of sprint %s: %w", sprintID, err)
	}
	return name, nil
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

// inTx runs fn inside one transaction, through the helper issuerepo shares,
// so a replace never leaves the table holding a delete without its inserts.
func (r *Repository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return dbtx.In(ctx, r.db, fn)
}
