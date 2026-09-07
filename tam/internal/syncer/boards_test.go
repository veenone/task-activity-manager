package syncer_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/syncer"
	"agile-suite/tam/internal/tamstore"
)

// newBoardRepos opens one tam.db and hands back both repositories the
// boards pass needs, sharing the handle the way app.go does: issuerepo for
// the profile settings the pass writes, boardrepo for the board tables
// themselves.
func newBoardRepos(t *testing.T) (*issuerepo.Repository, *boardrepo.Repository) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return issuerepo.New(db.DB()), boardrepo.New(db.DB())
}

func scrumAndKanban() []backend.Board {
	return []backend.Board{
		{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum},
		{ID: 2, Name: "PLAT Kanban", Type: backend.BoardTypeKanban},
	}
}

func TestSyncBoardsLandsScrumAndKanbanBoardsWithColumnsSprintsAndKeys(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := &fake{
		boards: scrumAndKanban(),
		columns: map[int][]backend.BoardColumn{
			1: {{Name: "To Do", StatusIDs: []string{"1"}}, {Name: "Done", StatusIDs: []string{"5"}}},
			2: {{Name: "To Do", StatusIDs: []string{"1"}}, {Name: "Done", StatusIDs: []string{"5"}}},
		},
		sprints: map[int][]backend.Sprint{
			1: {
				{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed"},
				{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"},
				{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"},
			},
			2: {},
		},
		issueKeys: map[int]map[string][]string{
			1: {"": {"PLAT-1", "PLAT-2"}, "12": {"PLAT-1"}, "13": {"PLAT-2"}},
			2: {"": {"PLAT-3"}},
		},
	}
	e := syncer.New(fb, repo)
	e.Boards = boards

	var events []syncer.Progress
	sum, err := e.SyncBoards(context.Background(), "p1", "PLAT", func(p syncer.Progress) { events = append(events, p) })
	if err != nil {
		t.Fatalf("sync boards: %v", err)
	}
	if sum.Boards != 2 || sum.Columns != 4 || sum.Sprints != 3 {
		t.Errorf("summary = %+v, want 2 boards, 4 columns, 3 sprints", sum)
	}
	// Cards: board 1's own list (2) + sprint 12 (1) + sprint 13 (1), plus
	// board 2's own list (1). Closed sprint 11 is never asked for.
	if sum.Cards != 5 {
		t.Errorf("cards = %d, want 5", sum.Cards)
	}
	if len(sum.Dropped) != 0 {
		t.Errorf("dropped = %v, want none", sum.Dropped)
	}

	got, err := boards.ListBoards(context.Background(), "p1")
	if err != nil || len(got) != 2 {
		t.Fatalf("list boards = %+v, %v", got, err)
	}
	cols, err := boards.Columns(context.Background(), "p1", 1)
	if err != nil || len(cols) != 2 {
		t.Fatalf("board 1 columns = %+v, %v", cols, err)
	}
	sp, err := boards.ListSprints(context.Background(), "p1", 1)
	if err != nil || len(sp) != 3 {
		t.Fatalf("board 1 sprints = %+v, %v", sp, err)
	}

	if len(events) == 0 {
		t.Fatal("expected progress frames for the boards phase")
	}
	for _, ev := range events {
		if ev.Phase != "boards" {
			t.Errorf("event phase = %q, want boards", ev.Phase)
		}
	}
	if events[0].Stage != "PLAT Scrum" {
		t.Errorf("first event stage = %q, want the board name", events[0].Stage)
	}
}

func TestSyncBoardsOnlyFetchesActiveAndFutureSprintKeys(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := &fake{
		boards:  []backend.Board{{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}},
		columns: map[int][]backend.BoardColumn{1: {{Name: "To Do", StatusIDs: []string{"1"}}}},
		sprints: map[int][]backend.Sprint{1: {
			{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed"},
			{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active"},
			{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"},
		}},
		issueKeys: map[int]map[string][]string{1: {"": {}, "12": {}, "13": {}}},
	}
	e := syncer.New(fb, repo)
	e.Boards = boards
	if _, err := e.SyncBoards(context.Background(), "p1", "PLAT", nil); err != nil {
		t.Fatalf("sync boards: %v", err)
	}

	seen := map[string]bool{}
	for _, r := range fb.keysRequested {
		seen[r.SprintID] = true
	}
	if !seen[""] || !seen["12"] || !seen["13"] {
		t.Errorf("keys requested = %+v, want the board's own list plus sprints 12 and 13", fb.keysRequested)
	}
	if seen["11"] {
		t.Error("sprint 11 is closed; its keys must not be fetched")
	}
}

func TestSyncBoardsRemovesABoardJiraStoppedReturning(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := &fake{
		boards: scrumAndKanban(),
		columns: map[int][]backend.BoardColumn{
			1: {{Name: "To Do", StatusIDs: []string{"1"}}},
			2: {{Name: "To Do", StatusIDs: []string{"1"}}},
		},
		sprints:   map[int][]backend.Sprint{1: {}, 2: {}},
		issueKeys: map[int]map[string][]string{1: {"": {}}, 2: {"": {}}},
	}
	e := syncer.New(fb, repo)
	e.Boards = boards
	if _, err := e.SyncBoards(context.Background(), "p1", "PLAT", nil); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	fb.boards = []backend.Board{{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}}
	sum, err := e.SyncBoards(context.Background(), "p1", "PLAT", nil)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if sum.Boards != 1 {
		t.Errorf("boards landed = %d, want 1", sum.Boards)
	}
	got, err := boards.ListBoards(context.Background(), "p1")
	if err != nil || len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("boards after removal = %+v, %v", got, err)
	}
	if cols, _ := boards.Columns(context.Background(), "p1", 2); len(cols) != 0 {
		t.Errorf("removed board 2 still has columns: %+v", cols)
	}
}

func TestSyncBoardsConfigFailureDropsOneBoardAndKeepsThePreviousCopyOfTheOther(t *testing.T) {
	repo, boards := newBoardRepos(t)
	fb := &fake{
		boards: scrumAndKanban(),
		columns: map[int][]backend.BoardColumn{
			1: {{Name: "To Do", StatusIDs: []string{"1"}}},
			2: {{Name: "To Do", StatusIDs: []string{"1"}}},
		},
		sprints:   map[int][]backend.Sprint{1: {}, 2: {}},
		issueKeys: map[int]map[string][]string{1: {"": {"PLAT-1"}}, 2: {"": {"PLAT-3"}}},
	}
	e := syncer.New(fb, repo)
	e.Boards = boards
	if _, err := e.SyncBoards(context.Background(), "p1", "PLAT", nil); err != nil {
		t.Fatalf("seed sync: %v", err)
	}

	// Board 1's configuration now fails, board 2's does not.
	fb.columnsErr = map[int]error{1: errors.New("jira: 403 Forbidden: Login required")}
	sum, err := e.SyncBoards(context.Background(), "p1", "PLAT", nil)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if sum.Boards != 1 {
		t.Errorf("boards landed = %d, want 1 (board 2 only)", sum.Boards)
	}
	if len(sum.Dropped) != 1 || sum.Dropped[0] != "PLAT Scrum: jira: 403 Forbidden: Login required" {
		t.Errorf("dropped = %v", sum.Dropped)
	}

	// Board 1's previous good copy (from the seed sync) must be untouched.
	cols, err := boards.Columns(context.Background(), "p1", 1)
	if err != nil || len(cols) != 1 || cols[0].Name != "To Do" {
		t.Errorf("board 1 columns after the failed sync = %+v, %v, want the seed copy kept", cols, err)
	}
	got, err := boards.ListBoards(context.Background(), "p1")
	if err != nil || len(got) != 2 {
		t.Fatalf("boards after a config failure = %+v, %v, want both boards still listed", got, err)
	}
}

func TestSyncBoardsWritesUnavailableSettingOnEveryRunAndClearsIt(t *testing.T) {
	repo, boards := newBoardRepos(t)
	ctx := context.Background()
	// Simulate a previous run against an instance with no Agile API: the
	// setting is already "true" before this run starts.
	if err := repo.SetProfileSetting(ctx, "p1", "boards_unavailable", "true"); err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	fb := &fake{
		boards:    []backend.Board{{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}},
		columns:   map[int][]backend.BoardColumn{1: {{Name: "To Do", StatusIDs: []string{"1"}}}},
		sprints:   map[int][]backend.Sprint{1: {}},
		issueKeys: map[int]map[string][]string{1: {"": {}}},
	}
	e := syncer.New(fb, repo)
	e.Boards = boards

	if _, err := e.SyncBoards(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatalf("sync boards: %v", err)
	}
	v, err := repo.ProfileSetting(ctx, "p1", "boards_unavailable")
	if err != nil || v != "" {
		t.Errorf("boards_unavailable = %q, %v, want cleared now that boards are answerable again", v, err)
	}
}
