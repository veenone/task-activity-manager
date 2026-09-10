package boardrepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

// kanbanSprints are the second board's, so the open list has to reach past
// the first board to find them and has to say which board each came from.
func kanbanSprints() []backend.Sprint {
	return []backend.Sprint{
		{ID: 21, BoardID: 2, Name: "Hardening", State: "closed", StartDate: "2026-08-01T09:00:00Z", EndDate: "2026-08-15T09:00:00Z"},
		{ID: 22, BoardID: 2, Name: "Platform 9", State: "future", StartDate: "2026-08-25T09:00:00Z", EndDate: "2026-09-08T09:00:00Z"},
	}
}

func TestOpenSprintsSpanEveryBoardAndNameTheirs(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	for _, b := range sampleBoards() {
		sprints := kanbanSprints()
		if b.ID == 1 {
			sprints = sampleSprints()
		}
		if err := r.ReplaceBoard(ctx, "p1", b, sampleColumns(), sprints, nil); err != nil {
			t.Fatalf("replace board %d: %v", b.ID, err)
		}
	}

	open, err := r.OpenSprints(ctx, "p1")
	if err != nil {
		t.Fatalf("open sprints: %v", err)
	}
	// Sprint 12 is the only active one, so it leads whatever its dates say;
	// the three future ones follow by start date, across both boards. The
	// two closed ones, Sprint 11 and Hardening, are not offered at all.
	want := []boardrepo.SprintChoice{
		{ID: 12, Name: "Sprint 12", BoardName: "PLAT Scrum", State: "active"},
		{ID: 22, Name: "Platform 9", BoardName: "PLAT Kanban", State: "future"},
		{ID: 13, Name: "Sprint 13", BoardName: "PLAT Scrum", State: "future"},
		{ID: 14, Name: "Sprint 14", BoardName: "PLAT Scrum", State: "future"},
	}
	if len(open) != len(want) {
		t.Fatalf("open sprints = %+v, want %+v", open, want)
	}
	for i, w := range want {
		if open[i] != w {
			t.Errorf("open sprint %d = %+v, want %+v", i, open[i], w)
		}
	}
}

func TestOpenSprintsOfAProfileWithNoBoardsAreEmptyAndNotAnError(t *testing.T) {
	r, _ := newRepo(t)
	open, err := r.OpenSprints(context.Background(), "p1")
	if err != nil {
		t.Fatalf("open sprints: %v", err)
	}
	if open == nil || len(open) != 0 {
		t.Errorf("open sprints = %+v, want an empty slice", open)
	}
}

// TestOpenSprintsFoldsOneSprintOnTwoBoardsButKeepsTwoSprintsSharingAName is
// the shape Jira actually produces: the sprint table's key is
// (profile_id, board_id, id), so a sprint whose filter reaches two boards is
// stored once per board and would otherwise answer OpenSprints with two
// rows for what is one choice. A sprint id is global in Jira, so folding on
// it is safe; two sprints that merely share a name are not the same sprint
// and both have to survive the fold.
func TestOpenSprintsFoldsOneSprintOnTwoBoardsButKeepsTwoSprintsSharingAName(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	// Sprint 20 is one sprint whose filter reaches both boards: it is
	// written once per board, with the same id both times, but the two
	// copies carry different start dates so which one the query would sort
	// first is not left to an unspecified tiebreak. Sprint 30 and Sprint 31
	// are two different sprints that happen to share a name.
	board1Sprints := []backend.Sprint{
		{ID: 20, BoardID: 1, Name: "Cross-team", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
		{ID: 30, BoardID: 1, Name: "Sprint Alpha", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
	}
	board2Sprints := []backend.Sprint{
		{ID: 20, BoardID: 2, Name: "Cross-team", State: "active", StartDate: "2026-08-20T09:00:00Z", EndDate: "2026-09-03T09:00:00Z"},
		{ID: 31, BoardID: 2, Name: "Sprint Alpha", State: "future", StartDate: "2026-09-02T09:00:00Z", EndDate: "2026-09-16T09:00:00Z"},
	}
	for _, b := range sampleBoards() {
		sprints := board2Sprints
		if b.ID == 1 {
			sprints = board1Sprints
		}
		if err := r.ReplaceBoard(ctx, "p1", b, sampleColumns(), sprints, nil); err != nil {
			t.Fatalf("replace board %d: %v", b.ID, err)
		}
	}

	open, err := r.OpenSprints(ctx, "p1")
	if err != nil {
		t.Fatalf("open sprints: %v", err)
	}
	ids := make([]int, len(open))
	for i, s := range open {
		ids[i] = s.ID
	}
	// 20 appears once, kept from the first board the query orders it under;
	// 30 and 31 are two different sprints, so both are kept.
	want := []int{20, 30, 31}
	if len(ids) != len(want) {
		t.Fatalf("open sprint ids = %v, want %v", ids, want)
	}
	for i, w := range want {
		if ids[i] != w {
			t.Errorf("open sprint %d id = %d, want %d (full: %+v)", i, ids[i], w, open)
		}
	}
	if open[0].BoardName != "PLAT Scrum" {
		t.Errorf("sprint 20 board = %q, want the first board in the query's order (PLAT Scrum)", open[0].BoardName)
	}
}

// TestOpenSprintsBreaksATieOnBoardIdWhenEverythingElseMatches is the case the
// test above avoids by giving its two copies of Sprint 20 different start
// dates: a sprint Jira hands to two boards ordinarily carries the very same
// state, start date, and id in both copies, since it is one sprint. Without
// board.id as a last ORDER BY term, which BoardName the fold above keeps
// would be whatever order SQLite happened to return two otherwise identical
// rows in.
func TestOpenSprintsBreaksATieOnBoardIdWhenEverythingElseMatches(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	for _, b := range sampleBoards() {
		sprint := backend.Sprint{
			ID: 20, BoardID: b.ID, Name: "Cross-team", State: "active",
			StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z",
		}
		if err := r.ReplaceBoard(ctx, "p1", b, sampleColumns(), []backend.Sprint{sprint}, nil); err != nil {
			t.Fatalf("replace board %d: %v", b.ID, err)
		}
	}

	open, err := r.OpenSprints(ctx, "p1")
	if err != nil {
		t.Fatalf("open sprints: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("one sprint folds to one row: %+v", open)
	}
	// The lower board id wins, deterministically, regardless of sampleBoards
	// writing the Kanban board (id 2) first.
	if open[0].BoardName != "PLAT Scrum" {
		t.Errorf("open sprint board = %q, want the lower board id's name (PLAT Scrum)", open[0].BoardName)
	}
}
