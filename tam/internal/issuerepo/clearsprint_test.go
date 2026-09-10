package issuerepo_test

import (
	"context"
	"testing"
)

// TestClearSprintBlanksOnlyTheIssuesOfTheDeletedSprintAndLeavesAPendingMoveAlone
// covers both halves Step 3 asks for: the clear is scoped to the one sprint
// id, and an issue whose cached sprint column already reads a different
// sprint because a board move is pending against it is left exactly as it
// is, pending move included.
func TestClearSprintBlanksOnlyTheIssuesOfTheDeletedSprintAndLeavesAPendingMoveAlone(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)
	// PLAT-1 and PLAT-2 start in Sprint 12; PLAT-3 starts with no sprint.

	// PLAT-1 has a pending move away from the sprint being deleted: its
	// cached sprint_id already reads 13, ahead of Commit.
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-1", "13", "Sprint 13"); err != nil {
		t.Fatalf("move PLAT-1: %v", err)
	}

	if err := repo.ClearSprint(ctx, "p1", "12"); err != nil {
		t.Fatalf("clear sprint: %v", err)
	}

	// PLAT-2 was still cached in the deleted sprint and is blanked.
	two, err := repo.GetIssue(ctx, "p1", "PLAT-2")
	if err != nil {
		t.Fatalf("get PLAT-2: %v", err)
	}
	if two.SprintID != "" || two.SprintName != "" {
		t.Errorf("PLAT-2 = %+v, want its sprint columns blanked", two)
	}

	// PLAT-1 no longer reads Sprint 12 in the cache, so the clear must not
	// touch it, and its pending move to Sprint 13 must survive untouched.
	one, err := repo.GetIssue(ctx, "p1", "PLAT-1")
	if err != nil {
		t.Fatalf("get PLAT-1: %v", err)
	}
	if one.SprintID != "13" || one.SprintName != "Sprint 13" {
		t.Errorf("PLAT-1 = %+v, want its pending move to Sprint 13 left alone", one)
	}
	p := oneRow(t, repo, "PLAT-1")
	if p.AfterVal != "13|Sprint 13" {
		t.Errorf("PLAT-1 pending row = %+v, want the move to Sprint 13 still journaled", p)
	}

	// PLAT-3 never carried the deleted sprint and is untouched.
	three, err := repo.GetIssue(ctx, "p1", "PLAT-3")
	if err != nil {
		t.Fatalf("get PLAT-3: %v", err)
	}
	if three.SprintID != "" || three.SprintName != "" {
		t.Errorf("PLAT-3 = %+v, want it left as it was", three)
	}
}

// TestClearSprintOfASprintNobodyCarriesChangesNothing is the case a delete
// on a sprint no issue was ever synced into hits: nothing matches, and the
// call still succeeds.
func TestClearSprintOfASprintNobodyCarriesChangesNothing(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedBoardCards(t, repo)

	if err := repo.ClearSprint(ctx, "p1", "999"); err != nil {
		t.Fatalf("clear sprint: %v", err)
	}
	one, err := repo.GetIssue(ctx, "p1", "PLAT-1")
	if err != nil {
		t.Fatalf("get PLAT-1: %v", err)
	}
	if one.SprintID != "12" || one.SprintName != "Sprint 12" {
		t.Errorf("PLAT-1 = %+v, want it untouched", one)
	}
}
