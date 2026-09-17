package committer

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strconv"

	corejira "agile-suite/core/jira"
	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/sprints"
)

// SprintWriter pushes a journaled edit or delete of a sprint Jira holds.
// sprints.ForCommit answers it; the app wires it per Commit, since it is
// built over the profile's backend and both caches.
type SprintWriter interface {
	Edit(ctx context.Context, profileID string, boardID, sprintID int, d backend.SprintDraft, clearGoal bool) (string, error)
	Delete(ctx context.Context, profileID string, boardID, sprintID int) (string, error)
}

// pushSprintWrites pushes every sprint_edit row, then every sprint_delete
// row, oldest first. Edits go first so a sprint renamed and another deleted
// in one Commit never depend on each other's order. A row the sprint's own
// state refuses stays with a failure no retry fixes; any other failure is
// worth a retry.
func (r *commitRun) pushSprintWrites(ctx context.Context) {
	var rows []journal.PendingChange
	for _, p := range r.rows {
		if p.EntityType == issuerepo.EntitySprintEdit || p.EntityType == issuerepo.EntitySprintDelete {
			rows = append(rows, p)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].EntityType == issuerepo.EntitySprintEdit && rows[j].EntityType != issuerepo.EntitySprintEdit
	})
	for _, p := range rows {
		r.pushSprintWrite(ctx, p)
	}
}

func (r *commitRun) pushSprintWrite(ctx context.Context, p journal.PendingChange) {
	var boardID int
	var name, did, note string
	var d backend.SprintDraft
	var e issuerepo.SprintEdit
	var del issuerepo.SprintDelete
	var err error
	if p.EntityType == issuerepo.EntitySprintEdit {
		err = json.Unmarshal([]byte(p.AfterVal), &e)
		boardID, name, did, d = e.BoardID, e.Name, " edited", e.SprintDraft()
	} else {
		err = json.Unmarshal([]byte(p.AfterVal), &del)
		boardID, name, did = del.BoardID, del.Name, " deleted"
	}
	if err != nil {
		r.fail(p, "sprint "+p.EntityKey, "the change could not be decoded: "+err.Error(), false)
		return
	}
	sprintID, err := strconv.Atoi(p.EntityKey)
	if err != nil || sprintID <= 0 {
		r.fail(p, name, "the sprint id "+p.EntityKey+" is not a sprint Jira holds", false)
		return
	}
	if r.e.Sprints == nil {
		r.fail(p, name, "this connection cannot manage sprints", false)
		return
	}
	if p.EntityType == issuerepo.EntitySprintEdit {
		note, err = r.e.Sprints.Edit(ctx, r.profileID, boardID, sprintID, d, e.ClearGoal)
	} else {
		note, err = r.e.Sprints.Delete(ctx, r.profileID, boardID, sprintID)
	}
	if err != nil {
		r.fail(p, name, err.Error(), !errors.Is(err, sprints.ErrRefused) && !errors.Is(err, corejira.ErrNoAgile))
		return
	}
	if note != "" {
		log.Printf("tam: sprint %d%s on Commit, with a note: %s", sprintID, did, note)
	}
	if err := r.e.repo.MarkCommitted(ctx, r.profileID, []journal.PendingChange{p}); err != nil {
		// Jira has the change; the next Commit's push reads Jira's state
		// again, so a second edit is harmless and a second delete finds the
		// sprint gone.
		r.fail(p, name, "pushed to Jira but the journal could not be cleared: "+err.Error(), true)
		return
	}
	r.res.SprintsChanged = append(r.res.SprintsChanged, name+did)
}

func (r *commitRun) fail(p journal.PendingChange, name, message string, retryable bool) {
	r.res.Failures = append(r.res.Failures, Failure{Key: name, EntityType: p.EntityType, RowID: p.ID, Error: message, Retryable: retryable, Reachable: []string{}})
}
