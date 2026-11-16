package testrepo

import (
	"testing"

	"agile-suite/xtm/internal/backend"
)

// Discarding a pending row has to work for every type the journal can hold.
// These four were queued by their setters but had no case in
// DiscardPendingChange, so the discard reported "unknown entity_type" and left
// the row pending (#480).

// discardOnlyPending discards the single pending row the test has queued.
func discardOnlyPending(t *testing.T, r *Repository) {
	t.Helper()
	changes, err := r.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("pending = %+v, want exactly one row to discard", changes)
	}
	if err := r.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard %s: %v", changes[0].EntityType, err)
	}
	left, err := r.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending after discard: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("pending after discard = %+v, want none", left)
	}
}

func TestDiscardContainerEnvironmentsRestoresTheValueBeforeTheEdit(t *testing.T) {
	r := newTestRepo(t)
	if err := r.UpsertContainers("p1", []Container{
		{Key: "EX-1", Kind: "testexec", Summary: "Cycle 1", Environments: []string{"Prod"}},
	}); err != nil {
		t.Fatalf("seed exec: %v", err)
	}
	if err := r.SetContainerEnvironments("p1", "EX-1", []string{"Staging", "Chrome"}); err != nil {
		t.Fatalf("set environments: %v", err)
	}

	discardOnlyPending(t, r)

	got, err := r.ContainerEnvironments("p1", "EX-1")
	if err != nil {
		t.Fatalf("read environments: %v", err)
	}
	if len(got) != 1 || got[0] != "Prod" {
		t.Errorf("environments after discard = %v, want [Prod]", got)
	}
}

func TestDiscardRunStepStatusRestoresTheBackendsValue(t *testing.T) {
	r := newTestRepo(t)
	if err := r.ReplaceRunStepResults("p1", "501", "PROJ-101", []backend.RunStepResult{
		{ID: "4001", DefinitionID: "3001", Rank: 1, Status: "NOT RUN", Action: "Open the gateway"},
	}); err != nil {
		t.Fatalf("seed step results: %v", err)
	}
	if err := r.SetRunStepStatus("p1", "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("set step status: %v", err)
	}

	discardOnlyPending(t, r)

	steps, _, err := r.RunStepResults("p1", "501", "PROJ-101")
	if err != nil {
		t.Fatalf("read step results: %v", err)
	}
	if len(steps) != 1 || steps[0].Status != "NOT RUN" {
		t.Errorf("step results after discard = %+v, want 4001 back at NOT RUN", steps)
	}
}

func TestDiscardRunDefectShowsTheSyncedDefectsAgain(t *testing.T) {
	r := seedRunDefectsRepo(t)
	if err := r.AddTestRunDefect("p1", "QA-TE-1", "QA-1", "BUG-1"); err != nil {
		t.Fatalf("stage defect: %v", err)
	}

	discardOnlyPending(t, r)

	rows, err := r.GetExecutionMembersWithRuns("p1", "QA-TE-1")
	if err != nil {
		t.Fatalf("read members: %v", err)
	}
	qa1 := findMember(rows, "QA-1")
	if qa1 == nil {
		t.Fatal("QA-1 not found in execution members")
	}
	if len(qa1.Defects) != 0 {
		t.Errorf("defects after discard = %v, want none", qa1.Defects)
	}
}

func TestDiscardRunCommentShowsTheSyncedCommentAgain(t *testing.T) {
	r := seedRunDefectsRepo(t)
	if err := r.ReplaceRunsForExec("p1", "QA-TE-1", []TestRunRow{
		{ExecKey: "QA-TE-1", TestKey: "QA-1", RunStatus: "PASS", Comment: "looked fine"},
	}); err != nil {
		t.Fatalf("seed synced run: %v", err)
	}
	if err := r.SetTestRunComment("p1", "QA-TE-1", "QA-1", "custom note"); err != nil {
		t.Fatalf("stage comment: %v", err)
	}

	discardOnlyPending(t, r)

	rows, err := r.GetExecutionMembersWithRuns("p1", "QA-TE-1")
	if err != nil {
		t.Fatalf("read members: %v", err)
	}
	qa1 := findMember(rows, "QA-1")
	if qa1 == nil {
		t.Fatal("QA-1 not found in execution members")
	}
	if qa1.Comment != "looked fine" {
		t.Errorf("comment after discard = %q, want the synced \"looked fine\"", qa1.Comment)
	}
}

func TestDiscardAllGetsPastRunDefectAndCommentRows(t *testing.T) {
	r := seedRunDefectsRepo(t)
	if err := r.AddTestRunDefect("p1", "QA-TE-1", "QA-1", "BUG-1"); err != nil {
		t.Fatalf("stage defect: %v", err)
	}
	if err := r.SetTestRunComment("p1", "QA-TE-1", "QA-1", "custom note"); err != nil {
		t.Fatalf("stage comment: %v", err)
	}

	if _, err := r.DiscardAllPendingChanges("p1"); err != nil {
		t.Fatalf("discard all: %v", err)
	}

	left, err := r.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("pending after discard all = %+v, want none", left)
	}
}
