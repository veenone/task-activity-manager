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
)

// Starting and completing a sprint. Both are journal rows now, pushed by
// Commit's sprint changes phase through internal/sprints, and neither
// changes the cache until then: the sprint stays future or active on screen,
// with a chip saying what Commit will do. A completion stores the intent and
// the preview's count only; Commit works the unfinished set out again from
// Jira and reports what it really moved.

// EntitySprintStart is the journal entity type of a start. Its key is the
// sprint id as text, a draft's negative id included, its field FieldStart and
// its after_val the SprintStart as JSON.
const EntitySprintStart = "sprint_start"

// EntitySprintComplete is the journal entity type of a completion. Its key is
// the sprint id as text, its field FieldComplete and its after_val the
// SprintComplete as JSON.
const EntitySprintComplete = "sprint_complete"

// FieldStart and FieldComplete are the one field of each, so a second start
// of a sprint replaces the first.
const (
	FieldStart    = "start"
	FieldComplete = "complete"
)

// SprintStart is what a sprint_start row carries, the dates already in the
// Agile format.
type SprintStart struct {
	BoardID   int    `json:"boardId"`
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// SprintComplete is what a sprint_complete row carries. MoveTo is the
// destination sprint's id, empty for the backlog; PreviewCount is how many
// unfinished cards the dialog showed, which Commit does not rely on.
type SprintComplete struct {
	BoardID      int    `json:"boardId"`
	Name         string `json:"name"`
	MoveTo       string `json:"moveTo"`
	MoveToName   string `json:"moveToName"`
	PreviewCount int    `json:"previewCount"`
}

// JournalSprintStart queues a start for Commit. A draft sprint can be
// started too, on its own board, and Commit starts it once it has created
// it. A real sprint is refused unless the cache holds it as future, or while
// its delete is queued.
func (r *Repository) JournalSprintStart(ctx context.Context, profileID string, sprintID int, s SprintStart) error {
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return errors.New("a sprint needs a name")
	}
	key := strconv.Itoa(sprintID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if sprintID < 0 {
			d, _, err := readDraftSprint(ctx, tx, profileID, key)
			if err != nil {
				return err
			}
			s.BoardID = d.BoardID
		} else {
			_, state, err := cachedSprint(ctx, tx, profileID, s.BoardID, sprintID)
			if err != nil {
				return err
			}
			if state != "future" {
				return fmt.Errorf("sprint %d is %s, and only a sprint that has never been started can be started; press Refresh if TAM shows it wrongly", sprintID, state)
			}
			if err := refuseQueued(ctx, tx, profileID, sprintID, "starting", EntitySprintDelete); err != nil {
				return err
			}
		}
		after, err := json.Marshal(s)
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntitySprintStart, key, FieldStart, "", string(after), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntitySprintStart, key, "start", FieldStart, "", s.Name, "queued for Commit")
	})
}

// JournalSprintComplete queues a completion for Commit, naming the sprint and
// the destination from the cache. The checks that read the board cache (the
// state, the destination, the pending cards) are the caller's, as they are
// internal/sprints' own; this refuses a sprint the cache does not hold as a
// real one, and one whose delete is queued.
func (r *Repository) JournalSprintComplete(ctx context.Context, profileID string, sprintID int, c SprintComplete) error {
	key := strconv.Itoa(sprintID)
	return r.inTx(ctx, func(tx *sql.Tx) error {
		was, _, err := cachedSprint(ctx, tx, profileID, c.BoardID, sprintID)
		if err != nil {
			return err
		}
		if err := refuseQueued(ctx, tx, profileID, sprintID, "completing", EntitySprintDelete); err != nil {
			return err
		}
		c.Name, c.MoveToName = was.Name, "the backlog"
		if c.MoveTo != "" {
			c.MoveToName = "sprint " + c.MoveTo
			var name string
			if tx.QueryRowContext(ctx, `SELECT name FROM sprint WHERE profile_id = ? AND id = ? LIMIT 1`, profileID, c.MoveTo).Scan(&name) == nil && name != "" {
				c.MoveToName = name
			}
		}
		after, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if err := journal.Put(tx, profileID, EntitySprintComplete, key, FieldComplete, "", string(after), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntitySprintComplete, key, "complete", FieldComplete, "", c.MoveToName, "queued for Commit")
	})
}

// refuseQueued refuses doing to a sprint while one of the given rows is
// queued for it: "sprint 12 is waiting to be deleted on Commit; discard that
// before starting it".
func refuseQueued(ctx context.Context, tx *sql.Tx, profileID string, sprintID int, doing string, entityTypes ...string) error {
	waiting := map[string]string{EntitySprintDelete: "deleted", EntitySprintStart: "started", EntitySprintComplete: "completed"}
	for _, t := range entityTypes {
		if _, queued, err := pendingSprintRow(ctx, tx, profileID, t, strconv.Itoa(sprintID)); err != nil {
			return err
		} else if queued {
			return fmt.Errorf("sprint %d is waiting to be %s on Commit; discard that in Pending changes before %s it", sprintID, waiting[t], doing)
		}
	}
	return nil
}

// rekeySprintStart moves a draft sprint's pending start to the id Jira gave
// it, so Commit's sprint changes phase starts the real sprint. The start's
// boardId needs no rewrite: a sprint is only ever drafted onto a real board.
func rekeySprintStart(ctx context.Context, tx *sql.Tx, profileID, from, to string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE pending_change SET entity_key = ? WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		to, profileID, EntitySprintStart, from); err != nil {
		return fmt.Errorf("move the start of draft sprint %s to sprint %s: %w", from, to, err)
	}
	return nil
}
