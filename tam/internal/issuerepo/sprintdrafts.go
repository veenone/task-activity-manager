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

// A sprint drafted in TAM. Creating a sprint used to reach Jira the moment
// the dialog was confirmed, which made a plan that starts with a new sprint
// impossible to draft offline. It is journaled now, the way a TAM-NEW issue
// is: a sprint_create row carrying the draft, and a row in sprint under a
// negative id with draft = 1, so every picker that reads the sprint table
// offers it and every card moved into it journals an ordinary issue_sprint
// row naming that negative id. Commit's first phase creates it in Jira and
// RekeySprint rewrites the id everywhere before any move naming it is sent.
//
// This package writes those sprint rows although the sprint table is
// boardrepo's. It writes only draft = 1 rows and RekeySprint's two
// statements that turn one real, both on the draft's own board, because the
// row has to land and go in the same transaction as its journal row, and a
// discard has to revert the moves into it in that transaction too; two
// repositories would mean two transactions and a crash window leaving a
// sprint nobody can discard.
//
// Only the creation moves into the journal. Starting, completing, editing
// and deleting a sprint Jira already holds stay immediate writes in
// internal/sprints.

// EntitySprintCreate is the journal entity type of a drafted sprint. Its key
// is the negative id as text, its field FieldCreate, and its after_val the
// DraftSprint as JSON.
const EntitySprintCreate = "sprint_create"

// draftSprintSeq is the profile setting holding the last negative id handed
// out, so an id is never handed out twice even after its draft is discarded
// or committed: a stale reference to an old draft can then never attach to
// a new one.
const draftSprintSeq = "draft_sprint_seq"

// ErrDraftSprintGone is what a write aimed at a draft sprint answers when its
// sprint_create row is no longer in the journal: discarded, or committed.
var ErrDraftSprintGone = errors.New("issuerepo: the draft sprint is no longer in the journal")

// DraftSprint is what a sprint_create row carries. BoardName is kept for the
// Pending changes dialog, which has no board list of its own to name it from.
type DraftSprint struct {
	BoardID   int    `json:"boardId"`
	BoardName string `json:"boardName"`
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// SprintDraft is the part of the draft the Agile create takes.
func (d DraftSprint) SprintDraft() backend.SprintDraft {
	return backend.SprintDraft{Name: d.Name, Goal: d.Goal, StartDate: d.StartDate, EndDate: d.EndDate}
}

// CreateDraftSprint journals a new sprint on a board and writes its draft
// row, and answers with the sprint as the pickers will read it. The dates
// arrive already in the Agile API's own format: converting a date input's
// bare day is internal/sprints' job, the same as for a sprint Jira holds.
func (r *Repository) CreateDraftSprint(ctx context.Context, profileID string, d DraftSprint) (backend.Sprint, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return backend.Sprint{}, errors.New("a sprint needs a name")
	}
	if d.BoardID <= 0 {
		return backend.Sprint{}, errors.New("a sprint needs the board it belongs to")
	}
	var made backend.Sprint
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		id, err := nextDraftSprintID(ctx, tx, profileID)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("encode draft sprint: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date, goal, draft)
			 VALUES (?, ?, ?, ?, 'future', ?, ?, ?, 1)`,
			profileID, id, d.BoardID, d.Name, d.StartDate, d.EndDate, d.Goal); err != nil {
			return fmt.Errorf("insert draft sprint: %w", err)
		}
		key := strconv.Itoa(id)
		if err := journal.Put(tx, profileID, EntitySprintCreate, key, FieldCreate, "", string(encoded), ""); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, EntitySprintCreate, key, "create", "", "", d.Name, "drafted on board "+strconv.Itoa(d.BoardID)); err != nil {
			return err
		}
		made = backend.Sprint{ID: id, BoardID: d.BoardID, Name: d.Name, State: "future", StartDate: d.StartDate, EndDate: d.EndDate, Goal: d.Goal}
		return nil
	})
	if err != nil {
		return backend.Sprint{}, err
	}
	return made, nil
}

// nextDraftSprintID is one below the lowest of the stored sequence and every
// negative id already in the sprint table, and records itself as the new
// sequence inside the caller's transaction, so two creates cannot share it.
func nextDraftSprintID(ctx context.Context, tx *sql.Tx, profileID string) (int, error) {
	lowest := 0
	var stored string
	err := tx.QueryRowContext(ctx, `SELECT value FROM profile_setting WHERE profile_id = ? AND key = ?`, profileID, draftSprintSeq).Scan(&stored)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("draft sprint sequence: %w", err)
	}
	if n, perr := strconv.Atoi(stored); perr == nil && n < lowest {
		lowest = n
	}
	var inTable sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MIN(id) FROM sprint WHERE profile_id = ? AND id < 0`, profileID).Scan(&inTable); err != nil {
		return 0, fmt.Errorf("lowest draft sprint id: %w", err)
	}
	if inTable.Valid && int(inTable.Int64) < lowest {
		lowest = int(inTable.Int64)
	}
	next := lowest - 1
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO profile_setting (profile_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(profile_id, key) DO UPDATE SET value = excluded.value`,
		profileID, draftSprintSeq, strconv.Itoa(next)); err != nil {
		return 0, fmt.Errorf("record draft sprint sequence: %w", err)
	}
	return next, nil
}

// EditDraftSprint rewrites a draft sprint's name, goal and dates, locally,
// and renames it everywhere its name is carried: the sprint row, the cached
// cards in it, the journaled moves into it, and the draft issues in it. The
// board is the draft's own and does not change.
func (r *Repository) EditDraftSprint(ctx context.Context, profileID string, draftID int, d DraftSprint) (backend.Sprint, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return backend.Sprint{}, errors.New("a sprint needs a name")
	}
	key := strconv.Itoa(draftID)
	var made backend.Sprint
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		was, _, err := readDraftSprint(ctx, tx, profileID, key)
		if err != nil {
			return err
		}
		d.BoardID, d.BoardName = was.BoardID, was.BoardName
		encoded, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("encode draft sprint: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			string(encoded), profileID, EntitySprintCreate, key); err != nil {
			return fmt.Errorf("rewrite draft sprint %s: %w", key, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE sprint SET name = ?, goal = ?, start_date = ?, end_date = ? WHERE profile_id = ? AND id = ? AND draft = 1`,
			d.Name, d.Goal, d.StartDate, d.EndDate, profileID, draftID); err != nil {
			return fmt.Errorf("rewrite draft sprint row %s: %w", key, err)
		}
		if err := rewriteSprintID(ctx, tx, profileID, key, key, d.Name); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, EntitySprintCreate, key, "edit", "", was.Name, d.Name, ""); err != nil {
			return err
		}
		made = backend.Sprint{ID: draftID, BoardID: d.BoardID, Name: d.Name, State: "future", StartDate: d.StartDate, EndDate: d.EndDate, Goal: d.Goal}
		return nil
	})
	if err != nil {
		return backend.Sprint{}, err
	}
	return made, nil
}

// DiscardDraftSprint is the Sprints view's Delete on a draft: exactly what
// discarding its sprint_create row from Pending changes does.
func (r *Repository) DiscardDraftSprint(ctx context.Context, profileID string, draftID int) error {
	key := strconv.Itoa(draftID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := journal.ListForKey(tx, profileID, key)
		if err != nil {
			return err
		}
		for _, p := range rows {
			if p.EntityType == EntitySprintCreate {
				return discardOne(ctx, tx, profileID, p)
			}
		}
		return ErrDraftSprintGone
	})
}

// readDraftSprint decodes the draft a sprint_create row carries, and answers
// with the stored text beside it for the audit entry that retires the row.
func readDraftSprint(ctx context.Context, tx *sql.Tx, profileID, key string) (DraftSprint, string, error) {
	var raw string
	err := tx.QueryRowContext(ctx,
		`SELECT after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		profileID, EntitySprintCreate, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DraftSprint{}, "", ErrDraftSprintGone
	}
	if err != nil {
		return DraftSprint{}, "", fmt.Errorf("read draft sprint %s: %w", key, err)
	}
	var d DraftSprint
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return DraftSprint{}, "", fmt.Errorf("decode draft sprint %s: %w", key, err)
	}
	return d, raw, nil
}

// discardDraftSprint is what discarding a sprint_create row takes with it:
// every journaled move into the sprint is reverted and dropped, every draft
// issue in it leaves it, and its draft row goes. The create row itself is
// deleted and audited by discardOne, like every other row.
func discardDraftSprint(ctx context.Context, tx *sql.Tx, profileID, key string) error {
	all, err := journal.List(tx, profileID)
	if err != nil {
		return err
	}
	for _, p := range all {
		if p.EntityType != EntitySprintMove || MoveID(p.AfterVal) != key {
			continue
		}
		if err := revertMove(ctx, tx, profileID, p); err != nil {
			return err
		}
		if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
			return err
		}
		if err := journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "discard", p.Field, p.AfterVal, p.BeforeVal, "the draft sprint it was moving into was discarded"); err != nil {
			return err
		}
	}
	if err := rewriteSprintID(ctx, tx, profileID, key, "", ""); err != nil {
		return err
	}
	draftID, _ := strconv.Atoi(key)
	if _, err := tx.ExecContext(ctx, `DELETE FROM sprint WHERE profile_id = ? AND id = ? AND draft = 1`, profileID, draftID); err != nil {
		return fmt.Errorf("drop draft sprint %s: %w", key, err)
	}
	return nil
}

// rewriteSprintID repoints every place a sprint id is carried from one id to
// another, under the name given: the cached cards' sprint columns, both
// halves of every journaled sprint move, and every draft issue's JSON. It is
// how an edit renames a draft (from == to), how a discard empties it (to ==
// ""), and how RekeySprint makes it real. A move whose value becomes the
// backlog is packed empty, the way MoveValue packs every backlog move.
func rewriteSprintID(ctx context.Context, tx *sql.Tx, profileID, from, to, name string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE issue SET sprint_id = ?, sprint_name = ? WHERE profile_id = ? AND sprint_id = ?`,
		to, name, profileID, from); err != nil {
		return fmt.Errorf("repoint cards from sprint %s: %w", from, err)
	}
	moves, err := tx.QueryContext(ctx,
		`SELECT id, before_val, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ?`,
		profileID, EntitySprintMove)
	if err != nil {
		return fmt.Errorf("sprint moves naming %s: %w", from, err)
	}
	type repoint struct {
		id            int64
		before, after string
	}
	var todo []repoint
	for moves.Next() {
		var rp repoint
		if err := moves.Scan(&rp.id, &rp.before, &rp.after); err != nil {
			moves.Close()
			return err
		}
		changed := false
		if MoveID(rp.before) == from {
			rp.before, changed = MoveValue(to, name), true
		}
		if MoveID(rp.after) == from {
			rp.after, changed = MoveValue(to, name), true
		}
		if changed {
			todo = append(todo, rp)
		}
	}
	moves.Close()
	if err := moves.Err(); err != nil {
		return err
	}
	for _, rp := range todo {
		if _, err := tx.ExecContext(ctx,
			`UPDATE pending_change SET before_val = ?, after_val = ? WHERE profile_id = ? AND id = ?`,
			rp.before, rp.after, profileID, rp.id); err != nil {
			return fmt.Errorf("repoint sprint move %d: %w", rp.id, err)
		}
	}
	drafts, err := draftsWhere(ctx, tx, profileID, func(d backend.IssueDraft) bool { return d.SprintID == from })
	if err != nil {
		return err
	}
	for _, key := range drafts {
		if err := editDraft(ctx, tx, profileID, key, func(d *backend.IssueDraft) {
			d.SprintID, d.SprintName = to, name
		}); err != nil {
			return err
		}
	}
	return nil
}

// draftsWhere lists the keys of the drafts whose create JSON matches. A row
// that will not decode is skipped: it cannot name anything, and Commit
// reports it on its own.
func draftsWhere(ctx context.Context, tx *sql.Tx, profileID string, match func(backend.IssueDraft) bool) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT entity_key, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ?`,
		profileID, EntityIssueCreate)
	if err != nil {
		return nil, fmt.Errorf("draft issues: %w", err)
	}
	var keys []string
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		var d backend.IssueDraft
		if json.Unmarshal([]byte(raw), &d) == nil && match(d) {
			keys = append(keys, key)
		}
	}
	rows.Close()
	return keys, rows.Err()
}
