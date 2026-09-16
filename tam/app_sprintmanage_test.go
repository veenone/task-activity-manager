package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/issuerepo"
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

	createCalls int
}

func (b *manageLifecycleBackend) BoardSprints(context.Context, int) ([]backend.Sprint, error) {
	return b.sprints, nil
}

func (b *manageLifecycleBackend) CreateSprint(context.Context, int, backend.SprintDraft) (backend.Sprint, error) {
	b.createCalls++
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

// Creating a sprint is journaled now: the binding answers with a draft under
// a negative id, every picker offers it at once, and Jira hears nothing
// until Commit.
func TestCreateSprintDraftsTheSprintAndSendsNothingToJira(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	fake := &manageLifecycleBackend{simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")}
	a.backends[p.ID] = fake
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	got, err := a.CreateSprint(p.ID, 1, "Sprint 15", "Ship promos", "2026-09-16", "2026-09-30")
	if err != nil {
		t.Fatalf("CreateSprint: %v", err)
	}
	if got.Sprint.ID != -1 || got.Sprint.State != "future" || got.Sprint.Goal != "Ship promos" || got.Note != "" {
		t.Errorf("CreateSprint = %+v", got)
	}
	if fake.createCalls != 0 {
		t.Errorf("Jira was asked to create the sprint %d times, want none before Commit", fake.createCalls)
	}
	open, err := a.ListOpenSprints(p.ID)
	if err != nil || len(open) != 1 || open[0] != (boardrepo.SprintChoice{ID: -1, Name: "Sprint 15", BoardName: "PLAT Scrum", State: "future", Draft: true}) {
		t.Errorf("open sprints = %+v, %v", open, err)
	}
	listed, _ := a.ListBoardSprints(p.ID, 1)
	if len(listed) != 1 || !strings.HasPrefix(listed[0].StartDate, "2026-09-16") || !listed[0].Draft {
		t.Errorf("board sprints = %+v, want the draft with converted dates", listed)
	}
	pending, _ := a.ListPendingChanges(p.ID)
	if len(pending) != 1 || pending[0].EntityType != issuerepo.EntitySprintCreate || pending[0].EntityKey != "-1" {
		t.Errorf("pending = %+v", pending)
	}
	if _, err := a.CreateSprint(p.ID, 7, "Sprint 16", "", "2026-09-16", "2026-09-30"); err == nil || !strings.Contains(err.Error(), "not in the cache") {
		t.Errorf("a board the cache does not hold = %v", err)
	}
}

// A draft sprint is edited and deleted locally, cards and all.
func TestEditAndDeleteOfADraftSprintStayLocal(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	fake := &manageLifecycleBackend{simpleBoardBackend: *twoBoards("PLAT Scrum", "PLAT Kanban", "To Do")}
	a.backends[p.ID] = fake
	if err := a.repo.UpsertPage(a.ctx, p.ID, []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", StatusID: "1", Updated: "2026-09-01T00:00:00Z"},
	}, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum},
		[]backend.BoardColumn{{Name: "To Do", StatusIDs: []string{"1"}}}, nil, map[string][]string{"": {"PLAT-1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSprint(p.ID, 1, "Sprint 15", "", "2026-09-16", "2026-09-30"); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveIssueToSprint(p.ID, "PLAT-1", "-1"); err != nil {
		t.Fatal(err)
	}

	details, err := a.ListBoardSprintDetails(p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) < 1 || details[0].ID != -1 || !details[0].Draft || details[0].Total != 1 || details[0].Issues[0].Key != "PLAT-1" {
		t.Fatalf("details = %+v, want the draft sprint holding the moved card", details)
	}

	if note, err := a.EditSprint(p.ID, 1, -1, "Sprint 15 promos", "", "2026-09-16", "2026-10-01", false); err != nil || note != "" {
		t.Fatalf("EditSprint = %q, %v", note, err)
	}
	if fake.editedID != 0 {
		t.Errorf("the edit reached Jira for sprint %d", fake.editedID)
	}
	if listed, _ := a.ListBoardSprints(p.ID, 1); len(listed) != 1 || listed[0].Name != "Sprint 15 promos" {
		t.Errorf("board sprints = %+v", listed)
	}

	if note, err := a.DeleteSprint(p.ID, 1, -1); err != nil || note != "" {
		t.Fatalf("DeleteSprint = %q, %v", note, err)
	}
	if listed, _ := a.ListBoardSprints(p.ID, 1); len(listed) != 0 {
		t.Errorf("board sprints after delete = %+v", listed)
	}
	if pending, _ := a.ListPendingChanges(p.ID); len(pending) != 0 {
		t.Errorf("pending after delete = %+v, want the create and the move both gone", pending)
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
