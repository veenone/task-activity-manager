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
