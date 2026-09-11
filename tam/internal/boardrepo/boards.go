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
	INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date, goal, complete_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

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
	SELECT id, board_id, name, state, start_date, end_date, goal, complete_date FROM sprint
	WHERE profile_id = ? AND board_id = ?
	ORDER BY CASE state WHEN 'active' THEN 0 WHEN 'future' THEN 1 WHEN 'closed' THEN 2 ELSE 3 END, start_date, id`

const boardKeysSQL = `
	SELECT key FROM board_issue WHERE profile_id = ? AND board_id = ? AND sprint_id = ? ORDER BY position`

const sprintNameSQL = `
	SELECT name FROM sprint WHERE profile_id = ? AND id = ? AND name <> '' LIMIT 1`

const boardSprintSQL = `
	SELECT state FROM sprint WHERE profile_id = ? AND board_id = ? AND id = ? LIMIT 1`

// RemoveBoards drops the boards and everything hanging off them: their
// columns, their issue keys, their sprints, and any report built for one of
// those sprints on this board, in one transaction.
func (r *Repository) RemoveBoards(ctx context.Context, profileID string, boardIDs []int) error {
	if len(boardIDs) == 0 {
		return nil
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		for _, id := range boardIDs {
			for _, table := range []string{"board_column", "board_issue", "sprint", "sprint_report", "board"} {
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

// Columns returns one board's columns in board order, on the handle. A
// caller composing a whole board reads them inside its own snapshot
// instead, through columnsOf.
func (r *Repository) Columns(ctx context.Context, profileID string, boardID int) ([]backend.BoardColumn, error) {
	return columnsOf(ctx, r.db, profileID, boardID)
}

// columnsOf is the columns read itself, on whichever querier the caller
// hands it: the handle for a lone read, a read transaction for a board
// composed of several.
func columnsOf(ctx context.Context, q dbtx.Querier, profileID string, boardID int) ([]backend.BoardColumn, error) {
	rows, err := q.QueryContext(ctx, columnsSQL, profileID, boardID)
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
		if err := rows.Scan(&s.ID, &s.BoardID, &s.Name, &s.State, &s.StartDate, &s.EndDate, &s.Goal, &s.CompleteDate); err != nil {
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

// BoardSprintState is what this board's cached sprint list says about that
// sprint: its state, and whether the board holds it at all. A ceremony asks
// both before it acts.
//
// Whether the board holds it, because a completion judges "finished" against
// one board's last column while the cards come from the sprint, so a board
// and a sprint that have nothing to do with each other would decide where
// somebody's work goes and then close the sprint anyway. The board's own key
// is (profile_id, board_id, id), which is that whole question.
//
// The state, because a completion aimed at a sprint that never started
// empties it in Jira and only then finds out Jira will not close it. The
// state is Jira's own lowercase word, active, future or closed, kept as it
// arrived.
func (r *Repository) BoardSprintState(ctx context.Context, profileID string, boardID int, sprintID string) (string, bool, error) {
	if strings.TrimSpace(sprintID) == "" {
		return "", false, nil
	}
	var state string
	err := r.db.QueryRowContext(ctx, boardSprintSQL, profileID, boardID, sprintID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("sprint %s of board %d: %w", sprintID, boardID, err)
	}
	return state, true, nil
}

// SprintIssues returns the keys one board holds for one sprint, in the board
// order the view reads them back in. A completion subtracts the cards that
// left the sprint from this rather than replacing it with what a search
// answered: the search orders by key and is scoped by project and issue
// type, so writing its result back would alphabetize the board and could
// insert keys the board's own filter never drew.
func (r *Repository) SprintIssues(ctx context.Context, profileID string, boardID int, sprintID string) ([]string, error) {
	return issueKeys(ctx, r.db, profileID, boardID, sprintID)
}

// issueKeys returns the keys one board holds for a sprint, in board order.
// An empty sprintID reads the board's own list, the way the sync stored it.
// It reads on whichever querier the caller hands it, so the board and its
// membership can be read on one snapshot.
func issueKeys(ctx context.Context, q dbtx.Querier, profileID string, boardID int, sprintID string) ([]string, error) {
	rows, err := q.QueryContext(ctx, boardKeysSQL, profileID, boardID, sprintID)
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
