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
// out (nextDraftID), so a discarded or committed draft's id is never handed
// out again.
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
		id, err := nextDraftID(ctx, tx, profileID, "board", draftBoardSeq)
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
// half here the same way the issue_board repoint reaches into its id half.
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
		// A rank carries the board in its value's third field (RankValue);
		// left naming the draft, the commit pass's board-scoped order read
		// would come back empty and the drop would never resolve.
		if err := repointRows(ctx, tx, profileID, EntityRank, func(field, value string) (string, string, bool) {
			neighbour, before, boardID := ParseRank(value)
			return field, RankValue(neighbour, before, realID), boardID == draftID
		}); err != nil {
			return err
		}
		// An issue_board row carries the board in its FIELD (BoardField), so
		// the journal's uniqueness gives one row per key per board, and in
		// its value's id half (boardId|scope), which keeps its scope.
		fromField, toField := BoardField(draftID), BoardField(realID)
		if err := repointRows(ctx, tx, profileID, EntityIssueBoard, func(field, value string) (string, string, bool) {
			return toField, MoveValue(to, MoveRawName(value)), field == fromField
		}); err != nil {
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

// repointRows rewrites the field and after_val of every journal row of
// entityType that fn matches, to what fn answers.
func repointRows(ctx context.Context, tx *sql.Tx, profileID, entityType string, fn func(field, value string) (string, string, bool)) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, field, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ?`,
		profileID, entityType)
	if err != nil {
		return fmt.Errorf("%s rows: %w", entityType, err)
	}
	type repoint struct {
		id           int64
		field, value string
	}
	var todo []repoint
	for rows.Next() {
		var rp repoint
		if err := rows.Scan(&rp.id, &rp.field, &rp.value); err != nil {
			rows.Close()
			return err
		}
		if field, value, ok := fn(rp.field, rp.value); ok {
			todo = append(todo, repoint{id: rp.id, field: field, value: value})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, rp := range todo {
		if _, err := tx.ExecContext(ctx,
			`UPDATE pending_change SET field = ?, after_val = ? WHERE profile_id = ? AND id = ?`,
			rp.field, rp.value, profileID, rp.id); err != nil {
			return fmt.Errorf("repoint %s %d: %w", entityType, rp.id, err)
		}
	}
	return nil
}

// discardDraftBoard takes the board with the journal row that would have
// created it, the way discardDraftSprint does for a sprint.
//
// Without this, discarding a board_create row would take the ordinary path
// and delete only the journal row, leaving the board behind with draft = 1.
// That board would sit in every picker, never be created, because the row
// that would have created it is gone, and never be discardable again for the
// same reason: stranded state with no way out, which is worse than either a
// board that stays or a board that goes.
//
// The queued adds go too. An issue_board row names a board that is about to
// stop existing, so keeping it would strand a second row against a third
// missing id. Each one is audited rather than dropped silently: the user
// queued those cards deliberately and is entitled to see where they went.
func discardDraftBoard(ctx context.Context, tx *sql.Tx, profileID, key string) error {
	draftID, err := strconv.Atoi(key)
	if err != nil {
		return fmt.Errorf("draft board id %q: %w", key, err)
	}
	field := BoardField(draftID)
	rows, err := journal.List(tx, profileID)
	if err != nil {
		return err
	}
	for _, p := range rows {
		if p.EntityType != EntityIssueBoard || p.Field != field {
			continue
		}
		if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "discard", p.Field, p.AfterVal, p.BeforeVal,
			"the draft board it was queued onto was discarded"); err != nil {
			return err
		}
	}
	// The dependent tables first, then the board, so a failure part way
	// cannot leave a board with rows hanging off an id nothing owns.
	for _, table := range []string{"board_column", "board_issue", "sprint"} {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM `+table+` WHERE profile_id = ? AND board_id = ?`, profileID, draftID); err != nil {
			return fmt.Errorf("drop %s of draft board %d: %w", table, draftID, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM board WHERE profile_id = ? AND id = ? AND draft = 1`, profileID, draftID); err != nil {
		return fmt.Errorf("drop draft board %d: %w", draftID, err)
	}
	return nil
}
