package committer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"

	corejira "agile-suite/core/jira"
	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/sprints"
)

// SprintWriter pushes a journaled edit, start, completion or delete of a
// sprint. sprints.ForCommit answers it; the app wires it per Commit, since it
// is built over the profile's backend and both caches.
type SprintWriter interface {
	Edit(ctx context.Context, profileID string, boardID, sprintID int, d backend.SprintDraft, clearGoal bool) (string, error)
	Start(ctx context.Context, profileID string, boardID, sprintID int, d backend.SprintDraft) (string, error)
	Complete(ctx context.Context, profileID string, boardID, sprintID int, moveTo string) (sprints.Completion, error)
	Delete(ctx context.Context, profileID string, boardID, sprintID int) (string, error)
}

// sprintWriteOrder is the order the sprint changes phase pushes in: a rename
// before anything else, a start before a completion, a delete last.
var sprintWriteOrder = map[string]int{
	issuerepo.EntitySprintEdit:     1,
	issuerepo.EntitySprintStart:    2,
	issuerepo.EntitySprintComplete: 3,
	issuerepo.EntitySprintDelete:   4,
}

// pushSprintWrites pushes every sprint edit, start, completion and delete,
// kind by kind in sprintWriteOrder and oldest first within a kind. A row the
// sprint's own state refuses stays with a failure no retry fixes; any other
// failure is worth a retry.
func (r *commitRun) pushSprintWrites(ctx context.Context) {
	var rows []journal.PendingChange
	for _, p := range r.rows {
		if sprintWriteOrder[p.EntityType] > 0 {
			rows = append(rows, p)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return sprintWriteOrder[rows[i].EntityType] < sprintWriteOrder[rows[j].EntityType]
	})
	for _, p := range rows {
		r.pushSprintWrite(ctx, p)
	}
}

func (r *commitRun) pushSprintWrite(ctx context.Context, p journal.PendingChange) {
	w, err := decodeSprintWrite(p)
	if err != nil {
		r.fail(p, "sprint "+p.EntityKey, "the change could not be decoded: "+err.Error(), false)
		return
	}
	sprintID, err := strconv.Atoi(p.EntityKey)
	if err == nil && sprintID < 0 && p.EntityType == issuerepo.EntitySprintStart {
		// A start of a draft sprint whose create did not land this Commit.
		if waits, held := r.deps.blockedBy(p.EntityKey); held {
			r.deps.hold(r.res, p.EntityKey, p.EntityType, p.ID, waits)
			return
		}
		r.fail(p, w.name, "the draft sprint it starts is no longer waiting to be created; discard this start and start the sprint again once Jira has it", false)
		return
	}
	if err != nil || sprintID <= 0 {
		r.fail(p, w.name, "the sprint id "+p.EntityKey+" is not a sprint Jira holds", false)
		return
	}
	if r.e.Sprints == nil {
		r.fail(p, w.name, "this connection cannot manage sprints", false)
		return
	}
	did, note, err := r.sendSprintWrite(ctx, p.EntityType, sprintID, w)
	if err != nil {
		r.fail(p, w.name, err.Error(), !errors.Is(err, sprints.ErrRefused) && !errors.Is(err, corejira.ErrNoAgile))
		return
	}
	if note != "" {
		log.Printf("tam: sprint %d%s on Commit, with a note: %s", sprintID, did, note)
	}
	if err := r.e.repo.MarkCommitted(ctx, r.profileID, []journal.PendingChange{p}); err != nil {
		// Jira has the change; the next Commit's push reads the sprint's
		// state again, so a second edit or start is refused or harmless, a
		// second completion finds the sprint closed, and a second delete
		// finds it gone.
		r.fail(p, w.name, "pushed to Jira but the journal could not be cleared: "+err.Error(), true)
		return
	}
	r.res.SprintsChanged = append(r.res.SprintsChanged, w.name+did)
}

// sprintWrite is a decoded row of any of the four kinds.
type sprintWrite struct {
	name     string
	edit     issuerepo.SprintEdit
	start    issuerepo.SprintStart
	complete issuerepo.SprintComplete
	del      issuerepo.SprintDelete
}

func decodeSprintWrite(p journal.PendingChange) (sprintWrite, error) {
	var w sprintWrite
	var err error
	switch p.EntityType {
	case issuerepo.EntitySprintEdit:
		err = json.Unmarshal([]byte(p.AfterVal), &w.edit)
		w.name = w.edit.Name
	case issuerepo.EntitySprintStart:
		err = json.Unmarshal([]byte(p.AfterVal), &w.start)
		w.name = w.start.Name
	case issuerepo.EntitySprintComplete:
		err = json.Unmarshal([]byte(p.AfterVal), &w.complete)
		w.name = w.complete.Name
	default:
		err = json.Unmarshal([]byte(p.AfterVal), &w.del)
		w.name = w.del.Name
	}
	return w, err
}

// sendSprintWrite makes the one call a row asks for and answers with what
// the result line says it did. A completion that moved cards and stopped is
// an error here, naming them, so its row stays for a retry.
func (r *commitRun) sendSprintWrite(ctx context.Context, entityType string, sprintID int, w sprintWrite) (string, string, error) {
	s := r.e.Sprints
	switch entityType {
	case issuerepo.EntitySprintEdit:
		note, err := s.Edit(ctx, r.profileID, w.edit.BoardID, sprintID, w.edit.SprintDraft(), w.edit.ClearGoal)
		return " edited", note, err
	case issuerepo.EntitySprintStart:
		note, err := s.Start(ctx, r.profileID, w.start.BoardID, sprintID, w.start.SprintDraft())
		return " started", note, err
	case issuerepo.EntitySprintComplete:
		done, err := s.Complete(ctx, r.profileID, w.complete.BoardID, sprintID, w.complete.MoveTo)
		if err == nil && done.Message != "" {
			err = errors.New(done.Message)
		}
		noun := "unfinished cards"
		if done.Moved == 1 {
			noun = "unfinished card"
		}
		return fmt.Sprintf(" completed, %d %s moved to %s", done.Moved, noun, done.MovedTo), done.Note, err
	}
	note, err := s.Delete(ctx, r.profileID, w.del.BoardID, sprintID)
	return " deleted", note, err
}

func (r *commitRun) fail(p journal.PendingChange, name, message string, retryable bool) {
	r.res.Failures = append(r.res.Failures, Failure{Key: name, EntityType: p.EntityType, RowID: p.ID, Error: message, Retryable: retryable, Reachable: []string{}})
}
