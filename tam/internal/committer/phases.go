package committer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	corejira "agile-suite/core/jira"
	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// A Commit runs in phases, in dependency order, the way XTM's commit creates
// preconditions before containers before tests. Each phase reads the journal
// the one before it left, because a create rewrites every row that named its
// placeholder: a sprint's id, an epic's key under its stories, a story's key
// under its sub-tasks. A later phase then sends real ids only, and whatever
// still names a placeholder that could not be made real is held, not sent.

// phase is one step of a Commit. A later bundle adds a step by adding one
// entry to phases(); a step reads r.rows, reports into r.res, and holds what
// waits on a placeholder through r.deps.
type phase struct {
	name string
	run  func(ctx context.Context, r *commitRun)
}

// phases is the order a Commit runs in:
//
//  0. boards, so a draft sprint's originBoardId and a card queued onto a
//     drafted board both name a real board before anything else runs;
//  1. sprints, so a card moved into a draft sprint names a real one;
//  2. sprint changes: the edits, starts, completions and deletes, after the
//     creates so a draft sprint can be started in the same Commit, and in a
//     phase of their own so they read the ids the creates rewrote;
//  3. epics, so a story's Epic Link names a real key;
//  4. every other creatable type, so a sub-task's parent does;
//  5. sub-tasks;
//  6. edits, then the board moves (sprint moves, transitions, ranks), then
//     links, exactly as before phases existed.
func phases() []phase {
	return []phase{
		{name: "boards", run: func(ctx context.Context, r *commitRun) { r.createBoards(ctx); r.pushBoardAdds(ctx) }},
		{name: "sprints", run: func(ctx context.Context, r *commitRun) { r.createSprints(ctx) }},
		{name: "sprint changes", run: func(ctx context.Context, r *commitRun) { r.pushSprintWrites(ctx) }},
		{name: "epics", run: func(ctx context.Context, r *commitRun) { r.createDrafts(ctx, levelEpic) }},
		{name: "issues", run: func(ctx context.Context, r *commitRun) { r.createDrafts(ctx, levelIssue) }},
		{name: "sub-tasks", run: func(ctx context.Context, r *commitRun) { r.createDrafts(ctx, levelSubtask) }},
		{name: "edits", run: func(ctx context.Context, r *commitRun) { r.pushEdits(ctx) }},
		{name: "board moves", run: func(ctx context.Context, r *commitRun) { r.e.commitBoardMoves(ctx, r.profileID, r.res, r.deps) }},
		{name: "links", run: func(ctx context.Context, r *commitRun) { r.e.commitLinks(ctx, r.profileID, r.res, r.deps) }},
	}
}

// commitRun is one Commit's state across its phases.
type commitRun struct {
	e          *Engine
	profileID  string
	projectKey string
	res        *Result
	deps       *dependencies
	// rows is the journal as the last phase left it, oldest first.
	rows []journal.PendingChange
	// boardRealID maps a drafted board's negative id to the real one the
	// boards phase (boardcreate.go) gave it this Commit, so a later phase
	// reading a draft sprint's or a card's originBoardId can resolve it
	// before anything is sent: a placeholder board id would otherwise reach
	// Jira the moment the board itself was created but nothing else in this
	// Commit's journal rows knew its new id yet.
	boardRealID map[int]int
	// boardName carries a board this Commit itself created (boardcreate.go)
	// from its real id to the name it was drafted with, for the filter
	// check's message (checkBoardFilters). It is not a general board-name
	// lookup -- the committer has none, and does not own the board list --
	// so a board this Commit did not create is named by its id there.
	boardName map[int]string
}

// reload reads the journal again, oldest first.
func (r *commitRun) reload(ctx context.Context) error {
	all, err := r.e.repo.ListPendingChanges(ctx, r.profileID)
	if err != nil {
		return err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	r.rows = all
	return nil
}

// draftLevel is which create phase a draft belongs to.
type draftLevel int

const (
	levelEpic draftLevel = iota
	levelIssue
	levelSubtask
)

func levelOf(logicalType string) draftLevel {
	switch logicalType {
	case backend.TypeEpic:
		return levelEpic
	case backend.TypeSubtask:
		return levelSubtask
	}
	return levelIssue
}

// draftOrdinal is the n of TAM-NEW-n, which is the order drafts of one level
// are created in: the order they were drafted. A key that is not a draft key
// sorts last.
func draftOrdinal(key string) int {
	if !strings.HasPrefix(key, issuerepo.DraftPrefix) {
		return math.MaxInt
	}
	n, err := strconv.Atoi(strings.TrimPrefix(key, issuerepo.DraftPrefix))
	if err != nil {
		return math.MaxInt
	}
	return n
}

// errSprintNotRenamed is what a draft or a move into a draft sprint answers
// when that sprint's create row is gone but its negative id is still named:
// MarkSprintCreatedWithoutRekey cleared it after Jira created the sprint and
// the local rename failed. Nothing will make that id real, so it is neither
// held nor retried.
var errSprintNotRenamed = errors.New("its sprint was created in Jira but not renamed in TAM; move it to the sprint again")

// sprintNotRenamed says sprintID, bare or as a move value, is a draft
// sprint's negative id with no sprint_create row left in rows.
func sprintNotRenamed(rows []journal.PendingChange, sprintID string) bool {
	id := issuerepo.MoveID(sprintID)
	if n, err := strconv.Atoi(id); err != nil || n >= 0 {
		return false
	}
	for _, p := range rows {
		if p.EntityType == issuerepo.EntitySprintCreate && p.EntityKey == id {
			return false
		}
	}
	return true
}

// sprintCreator is the one Agile write the sprints phase makes. Both shipped
// backends answer it; a backend that does not fails each draft sprint with a
// reason and holds what waits on it.
type sprintCreator interface {
	CreateSprint(ctx context.Context, boardID int, d backend.SprintDraft) (backend.Sprint, error)
}

// createSprints creates every draft sprint, oldest first, and rewrites its
// negative id everywhere it is named. A sprint created here stays created
// whatever the later phases do.
func (r *commitRun) createSprints(ctx context.Context) {
	w, canCreate := r.e.b.(sprintCreator)
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntitySprintCreate {
			continue
		}
		var d issuerepo.DraftSprint
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, "draft sprint "+p.EntityKey, "the draft sprint could not be decoded: "+err.Error(), false))
			r.deps.block(p.EntityKey, "draft sprint "+p.EntityKey, "which could not be read")
			continue
		}
		// A sprint drafted onto a board that was itself still a draft
		// carries that board's negative id (originBoardId). The boards phase
		// runs before this one and rewrites it here, in r.boardRealID, the
		// moment the board is real; the board table's own row is repointed
		// too, but this draft's JSON is not, so this substitution is the
		// only place that happens for a sprint create.
		if real, ok := r.boardRealID[d.BoardID]; ok {
			d.BoardID = real
		}
		label := fmt.Sprintf("sprint %q", d.Name)
		// A board that is still a draft because its own create failed (or
		// is itself waiting on something else) leaves d.BoardID unresolved;
		// the sprint waits for it rather than naming a placeholder to Jira.
		if waits, blocked := r.deps.blockedBy(strconv.Itoa(d.BoardID)); blocked {
			r.deps.hold(r.res, p.EntityKey, issuerepo.EntitySprintCreate, p.ID, waits)
			r.deps.block(p.EntityKey, label, "which is waiting for "+r.deps.blocked[waits].label)
			continue
		}
		draftID, err := strconv.Atoi(p.EntityKey)
		if err != nil || draftID >= 0 {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, fmt.Sprintf("the draft sprint's id %q is not a draft id", p.EntityKey), false))
			r.deps.block(p.EntityKey, label, "which could not be sent")
			continue
		}
		if !canCreate {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, "this connection cannot create sprints", false))
			r.deps.block(p.EntityKey, label, "which this connection cannot create")
			continue
		}
		// No firewall here: a sprint create carries a board id, a name, a
		// goal and two dates, and none of them can reference a placeholder.
		made, err := w.CreateSprint(ctx, d.BoardID, d.SprintDraft())
		if err != nil {
			r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, err.Error(), !errors.Is(err, corejira.ErrNoAgile)))
			r.deps.block(p.EntityKey, label, "which Jira refused")
			continue
		}
		if made.ID <= 0 {
			r.forgetSprint(ctx, p, draftID, 0, d.Name, label, "Jira created the sprint but answered with no id; refresh the board to see it, then move its cards again")
			continue
		}
		fillSprint(&made, d)
		if err := r.e.repo.RekeySprint(ctx, r.profileID, draftID, made); err != nil {
			if errors.Is(err, issuerepo.ErrDraftSprintGone) {
				r.res.Failures = append(r.res.Failures, sprintFailure(p, d.Name, fmt.Sprintf("created in Jira as sprint %d, but the draft was discarded while Commit ran; refresh the board to see it", made.ID), false))
				continue
			}
			r.forgetSprint(ctx, p, draftID, made.ID, d.Name, label, fmt.Sprintf("created in Jira as sprint %d but the local rename failed: %v", made.ID, err))
			continue
		}
		r.res.CreatedSprints = append(r.res.CreatedSprints, CreatedSprint{DraftID: draftID, ID: made.ID, Name: made.Name})
	}
}

// fillSprint keeps the draft's own values where Jira's answer left a field
// empty, which a create that answers without a body does.
func fillSprint(made *backend.Sprint, d issuerepo.DraftSprint) {
	made.BoardID = d.BoardID
	if made.Name == "" {
		made.Name = d.Name
	}
	if made.StartDate == "" {
		made.StartDate = d.StartDate
	}
	if made.EndDate == "" {
		made.EndDate = d.EndDate
	}
	if made.Goal == "" {
		made.Goal = d.Goal
	}
}

// forgetSprint is a sprint Jira created that TAM could not rename locally:
// its draft is cleared so a retry does not create a second sprint, and what
// waited on it is held.
func (r *commitRun) forgetSprint(ctx context.Context, p journal.PendingChange, draftID, realID int, name, label, message string) {
	if err := r.e.repo.MarkSprintCreatedWithoutRekey(ctx, r.profileID, draftID, realID); err != nil {
		message += "; the draft could not be cleared either, so discard it before the next Commit or Jira gets a second sprint: " + err.Error()
	}
	r.res.Failures = append(r.res.Failures, sprintFailure(p, name, message, false))
	r.deps.block(p.EntityKey, label, "which Jira created but TAM could not rename; refresh the board, then move its cards again")
}

func sprintFailure(p journal.PendingChange, name, message string, retryable bool) Failure {
	return Failure{Key: name, EntityType: issuerepo.EntitySprintCreate, RowID: p.ID, Error: message, Retryable: retryable, Reachable: []string{}}
}

// createDrafts creates the drafts of one level in the order they were
// drafted. A draft whose parent or sprint is a placeholder this Commit could
// not make real is held; one whose parent is a placeholder nothing in this
// Commit knows is a failure, since no retry will create that parent.
func (r *commitRun) createDrafts(ctx context.Context, level draftLevel) {
	type pending struct {
		row   journal.PendingChange
		draft backend.IssueDraft
		bad   error
	}
	var todo []pending
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityIssueCreate {
			continue
		}
		var d backend.IssueDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			// A draft that will not decode has no type to phase it by, so it
			// is reported once, with the ordinary issues.
			if level == levelIssue {
				todo = append(todo, pending{row: p, bad: err})
			}
			continue
		}
		if levelOf(d.Type) == level {
			todo = append(todo, pending{row: p, draft: d})
		}
	}
	sort.SliceStable(todo, func(i, j int) bool {
		return draftOrdinal(todo[i].row.EntityKey) < draftOrdinal(todo[j].row.EntityKey)
	})
	for _, t := range todo {
		key := t.row.EntityKey
		if t.bad != nil {
			// A draft that will not decode will not decode on the next Commit
			// either; the row has to be discarded, not retried.
			r.res.Failures = append(r.res.Failures, failure(key, issuerepo.EntityIssueCreate, "the draft could not be decoded: "+t.bad.Error(), false))
			r.deps.block(key, key, "which could not be read")
			continue
		}
		if waits, held := r.deps.blockedBy(t.draft.ParentKey, t.draft.SprintID); held {
			r.deps.hold(r.res, key, issuerepo.EntityIssueCreate, t.row.ID, waits)
			continue
		}
		if isDraftKey(t.draft.ParentKey) {
			r.res.Failures = append(r.res.Failures, failure(key, issuerepo.EntityIssueCreate, fmt.Sprintf("its parent %s is not a draft this Commit could create; set the parent again", t.draft.ParentKey), false))
			r.deps.block(key, key, "which could not be sent")
			continue
		}
		if sprintNotRenamed(r.rows, t.draft.SprintID) {
			r.res.Failures = append(r.res.Failures, failure(key, issuerepo.EntityIssueCreate, errSprintNotRenamed.Error(), false))
			r.deps.block(key, key, "which could not be sent")
			continue
		}
		if err := assertNoPlaceholders(map[string]any{"parentKey": t.draft.ParentKey, "sprintId": t.draft.SprintID}); err != nil {
			r.res.Failures = append(r.res.Failures, failure(key, issuerepo.EntityIssueCreate, err.Error(), false))
			r.deps.block(key, key, "which could not be sent")
			continue
		}
		r.e.commitCreate(ctx, r, t.row, t.draft)
	}
}

// pushEdits pushes every edited issue's fields, keys in order.
func (r *commitRun) pushEdits(ctx context.Context) {
	byKey := map[string][]journal.PendingChange{}
	var keys []string
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityIssue || isDraftKey(p.EntityKey) {
			continue
		}
		if _, seen := byKey[p.EntityKey]; !seen {
			keys = append(keys, p.EntityKey)
		}
		byKey[p.EntityKey] = append(byKey[p.EntityKey], p)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rows := byKey[key]
		// parentKey is the one edited field that references another issue;
		// every other value is free text a placeholder check must not read.
		parent := ""
		for _, p := range rows {
			if p.Field == "parentKey" {
				parent = p.AfterVal
			}
		}
		// Every edit of the issue waits together, the way a conflict holds
		// every edit of an issue together: half an issue's edits pushed is
		// an intent Jira never saw whole.
		if waits, held := r.deps.blockedBy(parent); held {
			r.deps.hold(r.res, key, issuerepo.EntityIssue, 0, waits)
			continue
		}
		if isDraftKey(parent) {
			r.res.Failures = append(r.res.Failures, failure(key, issuerepo.EntityIssue, fmt.Sprintf("its parent %s is not a draft this Commit could create; set the parent again", parent), false))
			continue
		}
		r.e.commitEdit(ctx, r.profileID, key, rows, r.res)
	}
}
