package demo

import (
	"context"
	"fmt"
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
		{ID: 11, BoardID: scrumBoardID, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z", EndDate: "2026-08-18T09:00:00Z"},
		{ID: 12, BoardID: scrumBoardID, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
		{ID: 13, BoardID: scrumBoardID, Name: "Sprint 13", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
	}
}

// findDemoSprint is one of the scrum board's three sprints by id, its own
// dataset state, with nothing overlaid.
func findDemoSprint(sprintID int) (backend.Sprint, bool) {
	for _, s := range demoSprints() {
		if s.ID == sprintID {
			return s, true
		}
	}
	return backend.Sprint{}, false
}

// sprintsOverlay is demoSprints with StartSprint's and CompleteSprint's own
// state changes applied, and a started sprint's name and dates taken from
// the draft it was started with rather than the dataset's own, unstarted
// ones: a demo start has nowhere else to show the reader what the dialog
// just set. Callers hold b.mu.
func (b *Backend) sprintsOverlay() []backend.Sprint {
	base := demoSprints()
	out := make([]backend.Sprint, len(base))
	for i, s := range base {
		if state, ok := b.sprintState[s.ID]; ok {
			s.State = state
		}
		if draft, ok := b.sprintDraft[s.ID]; ok {
			if draft.Name != "" {
				s.Name = draft.Name
			}
			if draft.StartDate != "" {
				s.StartDate = draft.StartDate
			}
			if draft.EndDate != "" {
				s.EndDate = draft.EndDate
			}
		}
		out[i] = s
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
	target, ok := findDemoSprint(sprintID)
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
	target, ok := findDemoSprint(sprintID)
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
		if name = demoSprintName(sprintID); name == "" {
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

// demoSprintName is the name of one of the scrum board's three sprints, or
// "" for an id the demo does not have.
func demoSprintName(sprintID string) string {
	for _, s := range demoSprints() {
		if strconv.Itoa(s.ID) == sprintID {
			return s.Name
		}
	}
	return ""
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
