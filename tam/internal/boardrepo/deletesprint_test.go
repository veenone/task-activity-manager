package boardrepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
)

// TestDeleteSprintEverywhereReachesBothBoardsCopyOfASharedSprint is the bug
// a one board fixture would ship: Jira hands the same sprint to every board
// whose filter reaches it, so deleting it under only the board that
// happened to be on screen would leave a second board's row standing, and
// OpenSprints would keep offering a sprint that no longer exists.
func TestDeleteSprintEverywhereReachesBothBoardsCopyOfASharedSprint(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	shared := backend.Sprint{ID: 12, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z"}

	for _, b := range sampleBoards() {
		sprints := []backend.Sprint{{ID: shared.ID, BoardID: b.ID, Name: shared.Name, State: shared.State, StartDate: shared.StartDate}}
		keys := map[string][]string{"": {"PLAT-1"}, "12": {"PLAT-1"}}
		if err := r.ReplaceBoard(ctx, "p1", b, oneColumn(), sprints, keys); err != nil {
			t.Fatalf("replace board %d: %v", b.ID, err)
		}
	}

	if err := r.DeleteSprintEverywhere(ctx, "p1", shared.ID); err != nil {
		t.Fatalf("delete sprint everywhere: %v", err)
	}

	for _, b := range sampleBoards() {
		sprints, err := r.ListSprints(ctx, "p1", b.ID)
		if err != nil {
			t.Fatalf("board %d sprints: %v", b.ID, err)
		}
		if len(sprints) != 0 {
			t.Errorf("board %d sprints = %+v, want the deleted sprint gone from every board", b.ID, sprints)
		}
		if got := boardKeys(t, db, "p1", b.ID, "12"); len(got) != 0 {
			t.Errorf("board %d sprint membership = %v, want it cleared with the sprint", b.ID, got)
		}
		if got := boardKeys(t, db, "p1", b.ID, ""); len(got) != 1 || got[0] != "PLAT-1" {
			t.Errorf("board %d own list = %v, want it untouched", b.ID, got)
		}
	}

	open, err := r.OpenSprints(ctx, "p1")
	if err != nil {
		t.Fatalf("open sprints: %v", err)
	}
	if len(open) != 0 {
		t.Errorf("open sprints = %+v, want the deleted sprint offered by no board", open)
	}
}

// TestDeleteSprintEverywhereLeavesOtherSprintsOfTheSameBoardAlone covers the
// ordinary neighbour case: deleting one sprint must not touch another
// sprint, or the board's own list, on the same board.
func TestDeleteSprintEverywhereLeavesOtherSprintsOfTheSameBoardAlone(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	keys := map[string][]string{
		"":   {"PLAT-1", "PLAT-2"},
		"12": {"PLAT-1"},
		"13": {"PLAT-2"},
	}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sampleSprints(), keys); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := r.DeleteSprintEverywhere(ctx, "p1", 12); err != nil {
		t.Fatalf("delete: %v", err)
	}

	sprints, err := r.ListSprints(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	for _, s := range sprints {
		if s.ID == 12 {
			t.Error("sprint 12 was deleted and is still cached")
		}
	}
	if len(sprints) != len(sampleSprints())-1 {
		t.Errorf("sprints = %+v, want only sprint 12 gone", sprints)
	}
	if got := boardKeys(t, db, "p1", 1, "13"); len(got) != 1 || got[0] != "PLAT-2" {
		t.Errorf("sprint 13 membership = %v, want it untouched", got)
	}
	if got := boardKeys(t, db, "p1", 1, ""); len(got) != 2 {
		t.Errorf("board's own list = %v, want it untouched", got)
	}
}

// TestDeleteSprintEverywhereOfASprintNobodyHoldsChangesNothing is the case a
// stale detail panel or a retried delete would hit: the sprint has already
// gone from every board, and there is nothing left to clean up.
func TestDeleteSprintEverywhereOfASprintNobodyHoldsChangesNothing(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	if err := r.ReplaceBoard(ctx, "p1", board, oneColumn(), sampleSprints(), nil); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := r.DeleteSprintEverywhere(ctx, "p1", 999); err != nil {
		t.Fatalf("delete: %v", err)
	}

	sprints, err := r.ListSprints(ctx, "p1", 1)
	if err != nil || len(sprints) != len(sampleSprints()) {
		t.Fatalf("sprints = %+v, %v, want them untouched", sprints, err)
	}
}
