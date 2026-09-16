package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// A board drafted in TAM, journaled the way a draft sprint is
// (sprintdrafts.go): a board_create row carrying the draft, and a row in
// board under a negative id with draft = 1, so every picker that reads the
// board table offers it. Commit's board-create phase creates it in Jira and
// RekeyBoard rewrites the id everywhere before anything naming it is sent.
// Nothing here talks to Jira: the journal is what Commit pushes.

// draftBoardSeq is the profile setting holding the last negative id handed
// out, the same scheme draftSprintSeq uses so a discarded or committed
// draft's id is never handed out again.
const draftBoardSeq = "draft_board_seq"

// DraftBoard is what a board_create row carries: the fields Commit needs to
// create the board in Jira.
type DraftBoard struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	FilterName string `json:"filterName"`
	JQL        string `json:"jql"`
}

// CreateDraftBoard journals a new board and writes its draft row, and
// answers with the board as the pickers will read it.
func (r *Repository) CreateDraftBoard(ctx context.Context, profileID string, d DraftBoard) (backend.Board, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return backend.Board{}, errors.New("a board needs a name")
	}
	var made backend.Board
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		id, err := nextDraftBoardID(ctx, tx, profileID)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("encode draft board: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO board (profile_id, id, name, type, draft) VALUES (?, ?, ?, ?, 1)`,
			profileID, id, d.Name, d.Type); err != nil {
			return fmt.Errorf("insert draft board: %w", err)
		}
		key := strconv.Itoa(id)
		if err := journal.Put(tx, profileID, EntityBoardCreate, key, FieldCreate, "", string(encoded), ""); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, EntityBoardCreate, key, "create", "", "", d.Name, "drafted"); err != nil {
			return err
		}
		made = backend.Board{ID: id, Name: d.Name, Type: d.Type}
		return nil
	})
	if err != nil {
		return backend.Board{}, err
	}
	return made, nil
}

// nextDraftBoardID mirrors nextDraftSprintID (sprintdrafts.go): one below
// the lowest of the stored sequence and every negative id already in the
// board table, recording itself as the new sequence inside the caller's
// transaction so two creates cannot share it.
func nextDraftBoardID(ctx context.Context, tx *sql.Tx, profileID string) (int, error) {
	lowest := 0
	var stored string
	err := tx.QueryRowContext(ctx, `SELECT value FROM profile_setting WHERE profile_id = ? AND key = ?`, profileID, draftBoardSeq).Scan(&stored)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("draft board sequence: %w", err)
	}
	if n, perr := strconv.Atoi(stored); perr == nil && n < lowest {
		lowest = n
	}
	var inTable sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MIN(id) FROM board WHERE profile_id = ? AND id < 0`, profileID).Scan(&inTable); err != nil {
		return 0, fmt.Errorf("lowest draft board id: %w", err)
	}
	if inTable.Valid && int(inTable.Int64) < lowest {
		lowest = int(inTable.Int64)
	}
	next := lowest - 1
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO profile_setting (profile_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(profile_id, key) DO UPDATE SET value = excluded.value`,
		profileID, draftBoardSeq, strconv.Itoa(next)); err != nil {
		return 0, fmt.Errorf("record draft board sequence: %w", err)
	}
	return next, nil
}

// RekeyBoard turns a draft board into the board Jira just created, in one
// transaction: the draft row takes Jira's id and stops being a draft, every
// table keyed by board_id (board_column, board_issue, sprint) repoints from
// the negative id to the real one, any rank journaled against the draft
// board (RankValue's board-carrying third field) repoints the same way, and
// every issue_board row queued onto the draft board (BoardField's
// board-carrying field) repoints too. The board_create row goes. A row
// Jira's id already has on the board table, which a boards refresh between
// the create and this call would leave, is replaced rather than collided
// with, the same way RekeySprint clears its own table first.
//
// ponytail: an issue_board row's scope can itself be a draft sprint's id
// (AddToBoard's non-backlog scope), and this does not repoint that. It is
// dead weight today: CreateDraftSprint refuses BoardID <= 0, so a sprint
// cannot be drafted onto a draft board, and a scope naming a draft sprint on
// a real board is rewriteSprintID's row to fix, not this one's. Upgrade path
// if CreateDraftSprint's guard is ever lifted: reach into after_val's scope
// half here the same way rekeyIssueBoard reaches into its id half.
func (r *Repository) RekeyBoard(ctx context.Context, profileID string, draftID, realID int) error {
	from, to := strconv.Itoa(draftID), strconv.Itoa(realID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		var raw string
		if err := tx.QueryRowContext(ctx,
			`SELECT after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			profileID, EntityBoardCreate, from).Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("draft board %s is no longer in the journal", from)
			}
			return fmt.Errorf("read draft board %s: %w", from, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM board WHERE profile_id = ? AND id = ? AND draft = 0`, profileID, realID); err != nil {
			return fmt.Errorf("clear board %s before the rekey: %w", to, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE board SET id = ?, draft = 0 WHERE profile_id = ? AND id = ? AND draft = 1`,
			realID, profileID, draftID); err != nil {
			return fmt.Errorf("rekey board %s to %s: %w", from, to, err)
		}
		for _, table := range []string{"board_column", "board_issue", "sprint"} {
			if _, err := tx.ExecContext(ctx,
				`UPDATE `+table+` SET board_id = ? WHERE profile_id = ? AND board_id = ?`,
				realID, profileID, draftID); err != nil {
				return fmt.Errorf("repoint %s from board %s: %w", table, from, err)
			}
		}
		if err := rekeyRankBoard(ctx, tx, profileID, draftID, realID); err != nil {
			return err
		}
		if err := rekeyIssueBoard(ctx, tx, profileID, draftID, realID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			profileID, EntityBoardCreate, from); err != nil {
			return fmt.Errorf("clear draft board %s: %w", from, err)
		}
		return journal.Audit(tx, profileID, EntityBoardCreate, from, "commit", FieldCreate, "", raw, "created in Jira as board "+to)
	})
}

// rekeyRankBoard repoints every rank journaled against the draft board's id
// (RankValue's "side|neighbour|board") to the real one, keeping the side
// and neighbour it was dropped with. Left naming the draft, the commit
// pass's board-scoped order read would come back empty and the drop would
// never resolve.
func rekeyRankBoard(ctx context.Context, tx *sql.Tx, profileID string, draftID, realID int) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ?`,
		profileID, EntityRank)
	if err != nil {
		return fmt.Errorf("rank rows on board %d: %w", draftID, err)
	}
	type repoint struct {
		id    int64
		value string
	}
	var todo []repoint
	for rows.Next() {
		var (
			id    int64
			value string
		)
		if err := rows.Scan(&id, &value); err != nil {
			rows.Close()
			return err
		}
		neighbour, before, boardID := ParseRank(value)
		if boardID == draftID {
			todo = append(todo, repoint{id: id, value: RankValue(neighbour, before, realID)})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, rp := range todo {
		if _, err := tx.ExecContext(ctx,
			`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND id = ?`,
			rp.value, profileID, rp.id); err != nil {
			return fmt.Errorf("repoint rank %d to board %d: %w", rp.id, realID, err)
		}
	}
	return nil
}

// rekeyIssueBoard repoints every issue_board row queued onto the draft
// board. AddToBoard folds the board id into the FIELD (BoardField), not the
// value, so the journal's own uniqueness gives one row per key per board;
// that means this is a field rewrite, rewriteSprintID's neighbour done on
// the field instead of the value. The value's id half (MoveValue's id,
// packed by AddToBoard as boardId|scope) still names the draft too and is
// rewritten alongside it, keeping the scope it was queued with.
func rekeyIssueBoard(ctx context.Context, tx *sql.Tx, profileID string, draftID, realID int) error {
	fromField, toField := BoardField(draftID), BoardField(realID)
	rows, err := tx.QueryContext(ctx,
		`SELECT id, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND field = ?`,
		profileID, EntityIssueBoard, fromField)
	if err != nil {
		return fmt.Errorf("issue_board rows on board %d: %w", draftID, err)
	}
	type repoint struct {
		id    int64
		value string
	}
	var todo []repoint
	for rows.Next() {
		var (
			id    int64
			value string
		)
		if err := rows.Scan(&id, &value); err != nil {
			rows.Close()
			return err
		}
		todo = append(todo, repoint{id: id, value: MoveValue(strconv.Itoa(realID), MoveRawName(value))})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, rp := range todo {
		if _, err := tx.ExecContext(ctx,
			`UPDATE pending_change SET field = ?, after_val = ? WHERE profile_id = ? AND id = ?`,
			toField, rp.value, profileID, rp.id); err != nil {
			return fmt.Errorf("repoint issue_board %d to board %d: %w", rp.id, realID, err)
		}
	}
	return nil
}
