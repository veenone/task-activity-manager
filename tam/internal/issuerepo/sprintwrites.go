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

// Editing and deleting a sprint Jira already holds. Both are journal rows
// now, pushed by Commit's sprints phase through internal/sprints, which still
// asks Jira for the sprint's state before it writes. What is checked here is
// the cache's copy, so a refusal the cache can already give costs no Commit.
//
// An edit changes the cached sprint row and the cards' sprint name at once,
// the way an issue edit changes its row, and its before value is what a
// discard puts back. A delete changes nothing locally: Jira still holds the
// sprint and its cards until Commit, and the push's own cache work removes
// them then.

// EntitySprintEdit is the journal entity type of an edit to a real sprint.
// Its key is the sprint id as text, its field FieldEdit, its after_val the
// SprintEdit as JSON and its before_val the cached SprintDraft as JSON.
const EntitySprintEdit = "sprint_edit"

// EntitySprintDelete is the journal entity type of a delete of a real sprint.
// Its key is the sprint id as text, its field FieldDelete, its after_val a
// SprintDelete as JSON and its before_val the sprint's name.
const EntitySprintDelete = "sprint_delete"

// FieldEdit and FieldDelete are the one field of each, so a second edit of a
// sprint replaces the first and keeps the first one's before value.
const (
	FieldEdit   = "edit"
	FieldDelete = "delete"
)

// SprintEdit is what a sprint_edit row carries. ClearGoal tells an empty
// goal meant as "remove it" from one meant as "leave it", as the Agile
// partial update needs; the dates are already in the Agile format.
type SprintEdit struct {
	BoardID   int    `json:"boardId"`
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	ClearGoal bool   `json:"clearGoal"`
}

// SprintDraft is the part of the edit the Agile update takes.
func (e SprintEdit) SprintDraft() backend.SprintDraft {
	return backend.SprintDraft{Name: e.Name, Goal: e.Goal, StartDate: e.StartDate, EndDate: e.EndDate}
}

// SprintDelete is what a sprint_delete row carries.
type SprintDelete struct {
	BoardID int    `json:"boardId"`
	Name    string `json:"name"`
}

// JournalSprintEdit journals an edit of a sprint Jira holds and applies it to
// the cache. A closed sprint, one the cache does not hold, and one already
// queued for deletion are refused with nothing written.
func (r *Repository) JournalSprintEdit(ctx context.Context, profileID string, sprintID int, e SprintEdit) error {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return errors.New("a sprint needs a name")
	}
	key := strconv.Itoa(sprintID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		was, state, err := cachedSprint(ctx, tx, profileID, e.BoardID, sprintID)
		if err != nil {
			return err
		}
		if state == "closed" {
			return fmt.Errorf("sprint %d is closed, and its dates are what velocity and burndown are computed from, so TAM does not edit it", sprintID)
		}
		if _, queued, err := pendingSprintRow(ctx, tx, profileID, EntitySprintDelete, key); err != nil {
			return err
		} else if queued {
			return fmt.Errorf("sprint %d is waiting to be deleted on Commit; discard the delete before editing it", sprintID)
		}
		prev, edited, err := pendingSprintRow(ctx, tx, profileID, EntitySprintEdit, key)
		if err != nil {
			return err
		}
		if edited && e.Goal == "" && !e.ClearGoal {
			// The dialog reopens on the goal an earlier edit cleared and so
			// cannot ask to clear it again; Jira has not been told yet.
			var p SprintEdit
			e.ClearGoal = json.Unmarshal([]byte(prev.AfterVal), &p) == nil && p.ClearGoal
		}
		now := was
		now.Name, now.StartDate, now.EndDate = e.Name, e.StartDate, e.EndDate
		if e.ClearGoal {
			now.Goal = ""
		} else if e.Goal != "" {
			now.Goal = e.Goal
		}
		before, err := json.Marshal(was)
		if err != nil {
			return err
		}
		after, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntitySprintEdit, key, FieldEdit, string(before), string(after), ""); err != nil {
			return err
		}
		if err := writeSprintFields(ctx, tx, profileID, key, now); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntitySprintEdit, key, "edit", FieldEdit, was.Name, e.Name, "")
	})
}

// JournalSprintDelete queues a sprint Jira holds for deletion on Commit. Only
// a future sprint the cache holds is queued; a pending edit of it is
// reverted and dropped, since the delete makes it moot. Whether cards in the
// sprint have pending changes is the caller's check, as it reads the cards.
func (r *Repository) JournalSprintDelete(ctx context.Context, profileID string, boardID, sprintID int) error {
	key := strconv.Itoa(sprintID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		// The edit is reverted first so the row names the sprint as Jira has
		// it; a refusal below rolls the revert back with everything else.
		if edit, edited, err := pendingSprintRow(ctx, tx, profileID, EntitySprintEdit, key); err != nil {
			return err
		} else if edited {
			if err := discardOne(ctx, tx, profileID, edit); err != nil {
				return err
			}
		}
		was, state, err := cachedSprint(ctx, tx, profileID, boardID, sprintID)
		if err != nil {
			return err
		}
		if state != "future" {
			return fmt.Errorf("sprint %d is %s, and only a sprint that has never been started can be deleted; complete it instead, or press Refresh if TAM still shows it as future", sprintID, state)
		}
		after, err := json.Marshal(SprintDelete{BoardID: boardID, Name: was.Name})
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntitySprintDelete, key, FieldDelete, was.Name, string(after), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntitySprintDelete, key, "delete", FieldDelete, was.Name, "", "queued for Commit")
	})
}

// cachedSprint reads a real sprint's fields and state from the cache,
// preferring the given board's row, since Jira hands one sprint to every
// board whose filter reaches it under the same values.
func cachedSprint(ctx context.Context, tx *sql.Tx, profileID string, boardID, sprintID int) (backend.SprintDraft, string, error) {
	if sprintID <= 0 {
		return backend.SprintDraft{}, "", fmt.Errorf("sprint %d is not a sprint Jira holds", sprintID)
	}
	var d backend.SprintDraft
	var state string
	err := tx.QueryRowContext(ctx,
		`SELECT name, goal, start_date, end_date, state FROM sprint
		 WHERE profile_id = ? AND id = ? AND draft = 0 ORDER BY board_id = ? DESC LIMIT 1`,
		profileID, sprintID, boardID).Scan(&d.Name, &d.Goal, &d.StartDate, &d.EndDate, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return d, "", fmt.Errorf("sprint %d is not in the cache; press Refresh", sprintID)
	}
	if err != nil {
		return d, "", fmt.Errorf("read sprint %d: %w", sprintID, err)
	}
	return d, strings.ToLower(state), nil
}

// pendingSprintRow is the one row of a sprint entity under a key, if any.
func pendingSprintRow(ctx context.Context, tx *sql.Tx, profileID, entityType, key string) (journal.PendingChange, bool, error) {
	rows, err := journal.ListForKey(tx, profileID, key)
	if err != nil {
		return journal.PendingChange{}, false, err
	}
	for _, p := range rows {
		if p.EntityType == entityType {
			return p, true, nil
		}
	}
	return journal.PendingChange{}, false, nil
}

// writeSprintFields sets a real sprint's cached fields on every board that
// holds it and renames the cards in it.
func writeSprintFields(ctx context.Context, tx *sql.Tx, profileID, key string, d backend.SprintDraft) error {
	id, err := strconv.Atoi(key)
	if err != nil {
		return fmt.Errorf("sprint id %q: %w", key, err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE sprint SET name = ?, goal = ?, start_date = ?, end_date = ? WHERE profile_id = ? AND id = ? AND draft = 0`,
		d.Name, d.Goal, d.StartDate, d.EndDate, profileID, id); err != nil {
		return fmt.Errorf("write sprint %s: %w", key, err)
	}
	return rewriteSprintID(ctx, tx, profileID, key, key, d.Name)
}

// discardSprintEdit puts the cached sprint back as the edit found it.
func discardSprintEdit(ctx context.Context, tx *sql.Tx, profileID string, p journal.PendingChange) error {
	var was backend.SprintDraft
	if err := json.Unmarshal([]byte(p.BeforeVal), &was); err != nil {
		return fmt.Errorf("decode sprint %s before its edit: %w", p.EntityKey, err)
	}
	return writeSprintFields(ctx, tx, profileID, p.EntityKey, was)
}
