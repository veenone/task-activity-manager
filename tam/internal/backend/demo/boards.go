package demo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
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
// one active, one future, in Jira's lowercase states.
func (b *Backend) BoardSprints(_ context.Context, boardID int) ([]backend.Sprint, error) {
	if err := knownBoard(boardID); err != nil {
		return nil, err
	}
	if boardID != scrumBoardID {
		return []backend.Sprint{}, nil
	}
	return demoSprints(), nil
}

// demoSprints is the scrum board's three sprints. It is a function rather
// than a literal inside BoardSprints because the sprint move names its
// destination from the same list.
func demoSprints() []backend.Sprint {
	return []backend.Sprint{
		{ID: 11, BoardID: scrumBoardID, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z", EndDate: "2026-08-18T09:00:00Z"},
		{ID: 12, BoardID: scrumBoardID, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
		{ID: 13, BoardID: scrumBoardID, Name: "Sprint 13", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
	}
}

// BoardIssueKeys answers with the dataset's own keys: one sprint's issues
// when sprintID is set, and every issue a board would hold when it is not.
// Requirements are left out because neither demo board collects them: a
// board holds the work items, and the demo's requirements have no sprint
// and no status either column maps.
func (b *Backend) BoardIssueKeys(_ context.Context, boardID int, sprintID string) ([]string, error) {
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
func (b *Backend) Transition(_ context.Context, key, targetStatusID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, ok := b.find(key)
	if !ok {
		return fmt.Errorf("demo: no issue %s", key)
	}
	if err := b.refuseTransition(key, targetStatusID); err != nil {
		return err
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
func (b *Backend) CanTransition(_ context.Context, key, targetStatusID string) (backend.TransitionCheck, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(key); !ok {
		return backend.TransitionCheck{}, fmt.Errorf("demo: no issue %s", key)
	}
	check := backend.TransitionCheck{Reachable: reachableFrom(key), Allowed: b.refuseTransition(key, targetStatusID) == nil}
	return check, nil
}

// refuseTransition is the one staged workflow refusal: the curated story
// cannot reach Done.
func (b *Backend) refuseTransition(key, targetStatusID string) error {
	if key != b.ConflictKey() || targetStatusID != StatusID("Done") {
		return nil
	}
	return &backend.NoTransition{Key: key, TargetStatusID: targetStatusID, Reachable: reachableFrom(key)}
}

// reachableFrom is the demo's list of statuses a card can move to: the
// three the columns collect, less Done for the curated story.
func reachableFrom(key string) []string {
	names := []string{"To Do", "In Progress", "Done"}
	if !strings.HasSuffix(key, conflictKey[len(demo.ProjectKey):]) {
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
func (b *Backend) MoveIssuesToSprint(_ context.Context, sprintID string, keys []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	name := ""
	if sprintID != "" {
		if name = demoSprintName(sprintID); name == "" {
			return fmt.Errorf("demo: no sprint %s", sprintID)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, key := range keys {
		iss, ok := b.find(key)
		if !ok {
			return fmt.Errorf("demo: no issue %s", key)
		}
		iss.SprintID, iss.SprintName, iss.Updated = sprintID, name, now
		b.over[key] = iss
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
