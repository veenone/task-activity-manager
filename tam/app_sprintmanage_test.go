package main

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

// manageLifecycleBackend answers the three management writes for real, so
// CreateSprint and EditSprint can be exercised past their guards without a
// live Jira. Everything else it answers comes from simpleBoardBackend, the
// same way lifecycleBackend borrows it for the two ceremonies.
type manageLifecycleBackend struct {
	simpleBoardBackend
	// sprints is what BoardSprints answers with: the cache's own view of
	// what the board holds, which is what requireEditable and the refresh
	// both read.
	sprints []backend.Sprint
	made    backend.Sprint

	editedID        int
	editedDraft     backend.SprintDraft
	editedClearGoal bool
}

func (b *manageLifecycleBackend) BoardSprints(context.Context, int) ([]backend.Sprint, error) {
	return b.sprints, nil
}

func (b *manageLifecycleBackend) CreateSprint(context.Context, int, backend.SprintDraft) (backend.Sprint, error) {
	return b.made, nil
}

func (b *manageLifecycleBackend) EditSprint(_ context.Context, sprintID int, d backend.SprintDraft, clearGoal bool) error {
	b.editedID = sprintID
	b.editedDraft = d
	b.editedClearGoal = clearGoal
	return nil
}

var _ backend.BoardBackend = (*manageLifecycleBackend)(nil)

// TestManagementBindingsRequireAProfile is requireProfile's own guard,
// ahead of anything that would need a backend at all: an empty profile id
// never reaches a.acquire or the service.
func TestManagementBindingsRequireAProfile(t *testing.T) {
	a := newTestApp(t)

	if _, err := a.CreateSprint("", 1, "Sprint 14", "", "2026-09-09", "2026-09-23"); err == nil {
		t.Error("CreateSprint with no profile = nil error, want a refusal")
	}
	if _, err := a.EditSprint("", 1, 13, "Sprint 13", "", "2026-09-09", "2026-09-23", false); err == nil {
		t.Error("EditSprint with no profile = nil error, want a refusal")
	}
	if _, err := a.DeleteSprint("", 1, 13); err == nil {
		t.Error("DeleteSprint with no profile = nil error, want a refusal")
	}
}

// TestManagementBindingsAreRefusedWhileABoardsRefreshHoldsTheLock is the
// guard app_sprints.go's own two ceremonies are already tested against
// (TestALifecycleCallIsRefusedWhileABoardsRefreshHoldsTheLock): these three
// take the same a.acquire(p.ID, "sprint") lock, under the same name, so a
// boards refresh in flight refuses them exactly the way it refuses a start
// or a completion. Testing only requireProfile would leave this untested,
// since a valid profile with the guard free never reaches acquire at all.
func TestManagementBindingsAreRefusedWhileABoardsRefreshHoldsTheLock(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")

	if err := a.acquire(p.ID, "boards refresh"); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer a.release(p.ID)

	if _, err := a.CreateSprint(p.ID, 1, "Sprint 14", "", "2026-09-09", "2026-09-23"); err == nil || !strings.Contains(err.Error(), "boards refresh") {
		t.Errorf("CreateSprint err = %v, want it to name the operation that is running", err)
	}
	if _, err := a.EditSprint(p.ID, 1, 13, "Sprint 13 renamed", "", "2026-09-09", "2026-09-23", false); err == nil || !strings.Contains(err.Error(), "boards refresh") {
		t.Errorf("EditSprint err = %v, want it to name the operation that is running", err)
	}
	if _, err := a.DeleteSprint(p.ID, 1, 13); err == nil || !strings.Contains(err.Error(), "boards refresh") {
		t.Errorf("DeleteSprint err = %v, want it to name the operation that is running", err)
	}
}

// TestCreateSprintReachesTheServiceAndReturnsTheSprintJiraMade is the
// binding's happy path, past both guards: the sprint Jira made travels
// back whole, and the board's own re-read lands with no note since the
// backend answers with a real list carrying it.
func TestCreateSprintReachesTheServiceAndReturnsTheSprintJiraMade(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	made := backend.Sprint{ID: 14, BoardID: 1, Name: "Sprint 14", State: "future", Goal: "Ship the grid"}
	a.backends[p.ID] = &manageLifecycleBackend{
		simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do"),
		sprints:            []backend.Sprint{made},
		made:               made,
	}

	got, err := a.CreateSprint(p.ID, 1, "Sprint 14", "Ship the grid", "2026-09-09", "2026-09-23")
	if err != nil {
		t.Fatalf("CreateSprint: %v", err)
	}
	if got.Note != "" {
		t.Errorf("note = %q, want none: the board's re-read carried the new sprint", got.Note)
	}
	if got.Sprint.ID != 14 || got.Sprint.Goal != "Ship the grid" {
		t.Errorf("CreateSprint = %+v, want the sprint Jira made, goal and all", got.Sprint)
	}
}

// TestEditSprintSendsTheClearGoalFlagThrough is the one argument this
// binding carries that Start and Complete do not: an empty goal box left
// that way, and one asking to clear a goal that was there, are different
// requests to the backend, and the flag is what tells them apart on the
// wire. requireEditable reads the sprint's state from the backend rather
// than the cache, so the fixture's board has to answer BoardSprints with a
// sprint that is not closed for the edit to reach the backend at all.
func TestEditSprintSendsTheClearGoalFlagThrough(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	backendFake := &manageLifecycleBackend{
		simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do"),
		sprints:            []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "active"}},
	}
	a.backends[p.ID] = backendFake

	if _, err := a.EditSprint(p.ID, 1, 13, "Sprint 13 renamed", "", "2026-09-09", "2026-09-23", true); err != nil {
		t.Fatalf("EditSprint: %v", err)
	}
	if backendFake.editedID != 13 || !backendFake.editedClearGoal {
		t.Errorf("edit sent (id %d, clearGoal %v), want sprint 13 with the goal cleared", backendFake.editedID, backendFake.editedClearGoal)
	}
	if backendFake.editedDraft.Name != "Sprint 13 renamed" {
		t.Errorf("edited name = %q, want the one the dialog collected", backendFake.editedDraft.Name)
	}
}
