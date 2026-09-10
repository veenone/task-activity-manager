package demo

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"agile-suite/tam/internal/backend"
)

// The only consumer is a type assertion, so drift would skip the sync silently; this fails the build.
var _ backend.BoardBackend = (*Backend)(nil)

// The demo's boards. Board 1 is the scrum board the sprints hang off,
// board 2 the kanban board that has none. Both are derived from the
// profile's project key, so a demo profile with any key gets boards named
// after it.
const (
	scrumBoardID  = 1
	kanbanBoardID = 2
)

// demoStatusIDs maps the dataset's status names to the Jira status ids the
// demo boards' columns collect. The dataset stores statuses as names, and
// the board joins cards to columns by id, so one table has to bridge them
// and both the column definitions and the tests read it here rather than
// each writing "1", "3", "5" of their own.
//
// To Do, In Progress, and Done use Jira's own default ids. Approved and
// Draft are the requirement statuses; they get ids of their own so a
// requirement is never mistaken for a local draft, which is what an empty
// status id means.
var demoStatusIDs = map[string]string{
	"To Do":       "1",
	"In Progress": "3",
	"Done":        "5",
	"Approved":    "10001",
	"Draft":       "10002",
}

// StatusID is the demo's status id for a status name, or "" for a name the
// dataset does not use.
func StatusID(name string) string { return demoStatusIDs[name] }

// StatusName is the demo's status name for a status id, or "" for an id no
// column collects. A transition is journaled by target status id, so the
// demo needs the way back to write a name onto the row it moves.
func StatusName(id string) string {
	for name, want := range demoStatusIDs {
		if want == id {
			return name
		}
	}
	return ""
}

// boardColumns are the three columns both demo boards show.
func boardColumns() []backend.BoardColumn {
	return []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{StatusID("To Do")}},
		{Name: "In Progress", StatusIDs: []string{StatusID("In Progress")}},
		{Name: "Done", StatusIDs: []string{StatusID("Done")}},
	}
}

// Boards is the pair of boards the demo profile shows.
func (b *Backend) Boards(context.Context, string) ([]backend.Board, error) {
	return []backend.Board{
		{ID: scrumBoardID, Name: b.project + " Scrum", Type: backend.BoardTypeScrum},
		{ID: kanbanBoardID, Name: b.project + " Kanban", Type: backend.BoardTypeKanban},
	}, nil
}

// BoardColumns gives both boards the same three columns.
func (b *Backend) BoardColumns(_ context.Context, boardID int) ([]backend.BoardColumn, error) {
	if err := knownBoard(boardID); err != nil {
		return nil, err
	}
	return boardColumns(), nil
}

// BoardSprints puts the three sprints on the scrum board only, one closed,
// one active, one future, in Jira's lowercase states, with any lifecycle
// action this run has taken (StartSprint, CompleteSprint) overlaid on top.
func (b *Backend) BoardSprints(_ context.Context, boardID int) ([]backend.Sprint, error) {
	if err := knownBoard(boardID); err != nil {
		return nil, err
	}
	if boardID != scrumBoardID {
		return []backend.Sprint{}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sprintsOverlay(), nil
}

// demoSprints is the scrum board's three sprints, their own dataset states,
// with nothing overlaid. It is a function rather than a literal so the
// sprint move and the overlay can both build on the same list.
func demoSprints() []backend.Sprint {
	return []backend.Sprint{
		{ID: 11, BoardID: scrumBoardID, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z", EndDate: "2026-08-18T09:00:00Z", Goal: "Ship the promo code redemption flow end to end"},
		{ID: 12, BoardID: scrumBoardID, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z", Goal: "Clear the checkout defect backlog before the freeze"},
		{ID: 13, BoardID: scrumBoardID, Name: "Sprint 13", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z", Goal: "Start the loyalty points redesign"},
	}
}

// findDemoSprint is one of the scrum board's sprints by id, its own dataset
// or created state, with nothing else overlaid: a tombstoned sprint is
// never found, whether the id names a dataset literal or one this run
// created, since DeleteSprint tombstones either kind the same way. Callers
// hold b.mu.
func (b *Backend) findDemoSprint(sprintID int) (backend.Sprint, bool) {
	if b.sprintDeleted[sprintID] {
		return backend.Sprint{}, false
	}
	for _, s := range demoSprints() {
		if s.ID == sprintID {
			return s, true
		}
	}
	if s, ok := b.sprintCreated[sprintID]; ok {
		return s, true
	}
	return backend.Sprint{}, false
}

// createdSprints is every sprint CreateSprint has made this run, ordered by
// id so the board's list is the same from one read to the next rather than
// at the mercy of map order. Callers hold b.mu.
func (b *Backend) createdSprints() []backend.Sprint {
	ids := make([]int, 0, len(b.sprintCreated))
	for id := range b.sprintCreated {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]backend.Sprint, 0, len(ids))
	for _, id := range ids {
		out = append(out, b.sprintCreated[id])
	}
	return out
}

// sprintEdit is what EditSprint recorded for one sprint: the fields it
// touched, on the same partial update rule Jira's own endpoint follows
// (empty means untouched), and clearGoal, the one deliberate exception that
// sends an empty goal on purpose.
type sprintEdit struct {
	draft     backend.SprintDraft
	clearGoal bool
}

// applyDraft is the one partial update rule every sprint draft follows,
// Jira's own endpoint included: a field the draft carries wins over
// whatever name, startDate, or endDate already holds, and goal follows a
// switch of its own, since clearGoal is the one deliberate exception that
// takes the goal away even though the draft's own is empty. clearGoalFlag
// is where EditSprint remembers that exception for the next edit to merge
// against; the two overlays that only ever apply a draft once, never pass
// one, so a nil clearGoalFlag simply means there is nothing to remember.
func applyDraft(name, startDate, endDate, goal *string, clearGoalFlag *bool, d backend.SprintDraft, clearGoal bool) {
	if d.Name != "" {
		*name = d.Name
	}
	if d.StartDate != "" {
		*startDate = d.StartDate
	}
	if d.EndDate != "" {
		*endDate = d.EndDate
	}
	switch {
	case d.Goal != "":
		*goal = d.Goal
		if clearGoalFlag != nil {
			*clearGoalFlag = false
		}
	case clearGoal:
		*goal = ""
		if clearGoalFlag != nil {
			*clearGoalFlag = true
		}
	}
}

// sprintsOverlay is the dataset's three sprints and this run's own created
// ones, tombstoned sprints dropped, with StartSprint's, CompleteSprint's,
// and EditSprint's own changes applied on top. A started sprint's name,
// dates, and goal come from the draft it was started with rather than the
// dataset's own, unstarted ones, since a demo start has nowhere else to
// show the reader what the dialog just set; an edited sprint's come from
// EditSprint's own draft over that, so an edit made after a start still
// wins. Callers hold b.mu.
func (b *Backend) sprintsOverlay() []backend.Sprint {
	base := append(demoSprints(), b.createdSprints()...)
	out := make([]backend.Sprint, 0, len(base))
	for _, s := range base {
		if b.sprintDeleted[s.ID] {
			continue
		}
		if state, ok := b.sprintState[s.ID]; ok {
			s.State = state
		}
		if draft, ok := b.sprintDraft[s.ID]; ok {
			applyDraft(&s.Name, &s.StartDate, &s.EndDate, &s.Goal, nil, draft, false)
		}
		if e, ok := b.sprintEdits[s.ID]; ok {
			applyDraft(&s.Name, &s.StartDate, &s.EndDate, &s.Goal, nil, e.draft, e.clearGoal)
		}
		out = append(out, s)
	}
	return out
}

// stateOf is a sprint's current state, overlay included, from the list
// sprintsOverlay returns, or "" for an id not in it.
func stateOf(sprints []backend.Sprint, sprintID int) string {
	for _, s := range sprints {
		if s.ID == sprintID {
			return s.State
		}
	}
	return ""
}

// StartSprint moves sprintID to active, enforcing the same rules a real
// Jira does rather than just the one about another active sprint: it
// refuses a sprint that is already active, and refuses reopening one that
// is already closed, so the offline walk-through rehearses what a real
// instance does instead of a friendlier demo-only shortcut. When another
// sprint on the same board is active it refuses and names it, borrowing
// Jira's own sentence for the message. The draft is held beside the state
// in sprintDraft, which is what lets sprintsOverlay show a started
// sprint's own name and dates rather than the dataset's unstarted ones.
// StartSprint never touches b.over, since that overlay is for issues, not
// sprints.
func (b *Backend) StartSprint(_ context.Context, sprintID int, draft backend.SprintDraft) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	target, ok := b.findDemoSprint(sprintID)
	if !ok {
		return fmt.Errorf("demo: no sprint %d", sprintID)
	}
	overlay := b.sprintsOverlay()
	switch stateOf(overlay, sprintID) {
	case "active":
		return fmt.Errorf("demo: sprint %s is already active", target.Name)
	case "closed":
		return fmt.Errorf("demo: sprint %s is closed and cannot be started again", target.Name)
	}
	for _, s := range overlay {
		if s.BoardID == target.BoardID && s.ID != sprintID && s.State == "active" {
			return fmt.Errorf("demo: another sprint is already active on this board: %s", s.Name)
		}
	}
	b.sprintState[sprintID] = "active"
	b.sprintDraft[sprintID] = draft
	return nil
}

// CompleteSprint moves sprintID to closed, refusing one that is not
// currently active: Jira completes only the sprint that is running, so a
// future sprint that never started and a sprint already closed are both
// refused here too, the same way a real instance would. It moves nothing
// itself: pushing the sprint's unfinished issues out, with
// MoveIssuesToSprint, is the caller's job, exactly as it is against a real
// Jira.
func (b *Backend) CompleteSprint(_ context.Context, sprintID int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	target, ok := b.findDemoSprint(sprintID)
	if !ok {
		return fmt.Errorf("demo: no sprint %d", sprintID)
	}
	if state := stateOf(b.sprintsOverlay(), sprintID); state != "active" {
		return fmt.Errorf("demo: sprint %s is not active and cannot be completed", target.Name)
	}
	b.sprintState[sprintID] = "closed"
	return nil
}

// BoardIssueKeys answers with the dataset's own keys: one sprint's issues
// when sprintID is set, and every issue a board would hold when it is not.
// Requirements are left out because neither demo board collects them: a
// board holds the work items, and the demo's requirements have no sprint
// and no status either column maps.
//
// A deleted sprint is never named here, but not because this filter knows
// about deletion: DeleteSprint's own walk clears sprint_id and sprint_name
// off every issue it held before the sprint's tombstone is set, so by the
// time this runs no cached issue carries the deleted id any more, and those
// same issues fall straight into the board's own list, sprintID empty,
// exactly the board scope a real Jira returns them to.
func (b *Backend) BoardIssueKeys(_ context.Context, boardID int, sprintID, _ string) ([]string, error) {
	if err := knownBoard(boardID); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	keys := []string{}
	for _, iss := range b.issues() {
		if iss.Type == backend.TypeRequirement {
			continue
		}
		if sprintID != "" && iss.SprintID != sprintID {
			continue
		}
		keys = append(keys, iss.Key)
	}
	return keys, nil
}

// Transition moves the card to the status the target id names, the way a
// workflow transition would. The curated story refuses the move into Done:
// a Data Center workflow routinely has no path from where a card sits to
// where it was dropped, and the offline walk-through has to be able to show
// that failure without a real Jira behind it.
func (b *Backend) Transition(_ context.Context, key string, targetStatusIDs []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, ok := b.find(key)
	if !ok {
		return fmt.Errorf("demo: no issue %s", key)
	}
	// The same rule the live backend follows: the first of the column's
	// statuses this issue may actually reach.
	targetStatusID, ok := b.firstAllowed(key, targetStatusIDs)
	if !ok {
		return b.refuseTransition(key, firstOf(targetStatusIDs))
	}
	name := StatusName(targetStatusID)
	if name == "" {
		return fmt.Errorf("demo: no status %s", targetStatusID)
	}
	iss.Status, iss.StatusID = name, targetStatusID
	iss.Updated = time.Now().UTC().Format(time.RFC3339)
	b.over[key] = iss
	return nil
}

// CanTransition answers what the demo's own workflow allows: every status
// the columns collect, minus the one the curated story is refused.
func (b *Backend) CanTransition(_ context.Context, key string, targetStatusIDs []string) (backend.TransitionCheck, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(key); !ok {
		return backend.TransitionCheck{}, fmt.Errorf("demo: no issue %s", key)
	}
	_, allowed := b.firstAllowed(key, targetStatusIDs)
	check := backend.TransitionCheck{Reachable: reachableFrom(key, b.ConflictKey()), Allowed: allowed}
	return check, nil
}

// firstAllowed is the first of the column's statuses this issue may reach,
// mirroring the live backend, which walks the column's statuses in order and
// takes the first its workflow offers a transition to.
func (b *Backend) firstAllowed(key string, targets []string) (string, bool) {
	for _, id := range targets {
		if id == "" || StatusName(id) == "" {
			continue
		}
		if b.refuseTransition(key, id) == nil {
			return id, true
		}
	}
	return "", false
}

// firstOf names the status a refusal is about: the one dropped on.
func firstOf(targets []string) string {
	if len(targets) == 0 {
		return ""
	}
	return targets[0]
}

// refuseTransition is the one staged workflow refusal: the curated story
// cannot reach Done.
func (b *Backend) refuseTransition(key, targetStatusID string) error {
	if key != b.ConflictKey() || targetStatusID != StatusID("Done") {
		return nil
	}
	return &backend.NoTransition{Key: key, TargetStatusID: targetStatusID, Reachable: reachableFrom(key, b.ConflictKey())}
}

// reachableFrom is the demo's list of statuses a card can move to: the
// three the columns collect, less Done for the curated story. The curated
// story is passed in rather than recognised by the tail of its key, which
// matched any key ending the same way and would have gone quietly wrong the
// moment either constant moved.
func reachableFrom(key, conflictKey string) []string {
	names := []string{"To Do", "In Progress", "Done"}
	if key != conflictKey {
		return names
	}
	return names[:2]
}

// RankIssue records nothing: the demo dataset has no rank of its own, and a
// made-up one would be a second source of truth. It still refuses a key it
// does not hold, so the commit pass's own guards are exercised offline.
func (b *Backend) RankIssue(_ context.Context, key, neighbourKey string, _ bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, k := range []string{key, neighbourKey} {
		if _, ok := b.find(k); !ok {
			return fmt.Errorf("demo: no issue %s", k)
		}
	}
	return nil
}

// MoveIssuesToSprint moves the batch onto the sprint, or onto the backlog
// when the id is empty. The name comes from the scrum board's sprints, so a
// moved card reads the same as one the dataset put there.
//
// Every key is looked up before any of them is written. Jira applies a
// sprint move to the whole batch or to none of it, and a demo that wrote
// the first half of a batch and then refused the rest would leave the
// overlay in a state no Commit could have produced.
func (b *Backend) MoveIssuesToSprint(_ context.Context, sprintID string, keys []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	name := ""
	if sprintID != "" {
		if name = b.demoSprintName(sprintID); name == "" {
			return fmt.Errorf("demo: no sprint %s", sprintID)
		}
	}
	moving := make([]backend.Issue, 0, len(keys))
	for _, key := range keys {
		iss, ok := b.find(key)
		if !ok {
			return fmt.Errorf("demo: no issue %s", key)
		}
		moving = append(moving, iss)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, iss := range moving {
		iss.SprintID, iss.SprintName, iss.Updated = sprintID, name, now
		b.over[iss.Key] = iss
	}
	return nil
}

// demoSprintName is the current name of one of the scrum board's sprints,
// dataset, created, started, and edited alike, or "" for an id the demo
// does not have or has deleted. It reads through sprintsOverlay rather than
// the dataset's own three, which is what lets MoveIssuesToSprint accept a
// sprint this run just created or renamed instead of refusing it as
// unknown. Callers hold b.mu.
func (b *Backend) demoSprintName(sprintID string) string {
	for _, s := range b.sprintsOverlay() {
		if strconv.Itoa(s.ID) == sprintID {
			return s.Name
		}
	}
	return ""
}

// CreateSprint makes a new sprint on boardID with the draft's name, dates,
// and goal, future by convention since Jira never hands back a sprint any
// other state. The kanban board is refused the same as BoardSprints refuses
// to read from it: the dataset gives it no sprints, and a real board
// without the Agile sprint field would refuse a create the same way.
func (b *Backend) CreateSprint(_ context.Context, boardID int, d backend.SprintDraft) (backend.Sprint, error) {
	if err := knownBoard(boardID); err != nil {
		return backend.Sprint{}, err
	}
	if boardID != scrumBoardID {
		return backend.Sprint{}, fmt.Errorf("demo: board %d has no sprints", boardID)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextSprintID
	b.nextSprintID++
	s := backend.Sprint{
		ID:        id,
		BoardID:   boardID,
		Name:      d.Name,
		State:     "future",
		StartDate: d.StartDate,
		EndDate:   d.EndDate,
		Goal:      d.Goal,
	}
	b.sprintCreated[id] = s
	return s, nil
}

// EditSprint records the draft's changes against sprintID, on the same
// partial update rule the Jira implementation follows: an empty field in
// the draft is left alone, and clearGoal is the one deliberate exception
// that takes the goal away even though the draft's own is empty.
//
// The change is merged into whatever this run already recorded, not
// substituted for it: a second edit that only renames a sprint must not
// undo an earlier edit's dates or an earlier clearGoal, the same way two
// separate PATCHes to a real Jira sprint would not undo each other.
func (b *Backend) EditSprint(_ context.Context, sprintID int, d backend.SprintDraft, clearGoal bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.findDemoSprint(sprintID); !ok {
		return fmt.Errorf("demo: no sprint %d", sprintID)
	}
	e := b.sprintEdits[sprintID]
	// A new goal cancels an earlier clear, and a new clear cancels an
	// earlier goal: the two are mutually exclusive states of the one
	// field, not two independent ones, which is what clearGoalFlag records.
	applyDraft(&e.draft.Name, &e.draft.StartDate, &e.draft.EndDate, &e.draft.Goal, &e.clearGoal, d, clearGoal)
	b.sprintEdits[sprintID] = e
	return nil
}

// DeleteSprint removes sprintID, mirroring what a real Jira delete does to
// its issues: every cached card carrying this sprint returns to the
// board's own scope, sprint id and name both cleared, rather than being
// deleted itself. The walk below matches issues by sprint id, not by
// findDemoSprint, so the tombstone's placement relative to it is not load
// bearing; it is set last only because that is the order the steps are
// listed in here.
func (b *Backend) DeleteSprint(_ context.Context, sprintID int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.findDemoSprint(sprintID); !ok {
		return fmt.Errorf("demo: no sprint %d", sprintID)
	}
	sid := strconv.Itoa(sprintID)
	now := time.Now().UTC().Format(time.RFC3339)
	for _, iss := range b.issues() {
		if iss.SprintID != sid {
			continue
		}
		iss.SprintID, iss.SprintName, iss.Updated = "", "", now
		b.over[iss.Key] = iss
	}
	b.sprintDeleted[sprintID] = true
	return nil
}

func knownBoard(boardID int) error {
	if boardID != scrumBoardID && boardID != kanbanBoardID {
		return fmt.Errorf("demo: no board %d", boardID)
	}
	return nil
}

// statusIDFor fills in an issue's status id from its status name, so every
// issue the demo hands out carries one the same way a live Jira issue does.
func statusIDFor(iss backend.Issue) backend.Issue {
	iss.StatusID = StatusID(iss.Status)
	return iss
}
