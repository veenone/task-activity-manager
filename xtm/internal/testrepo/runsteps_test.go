// Step results on a plain execution, cached and journalled (#371 Phase 6).
//
// These are the same guarantees the parameterized tree earned the hard way: a
// refresh must not overwrite a result set locally (#364), and the uncommitted
// flag must come from the journal rather than a column so the store cannot
// claim a push that Commit would not make (#366).

package testrepo_test

import (
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

func runStepResults(status string) []backend.RunStepResult {
	return []backend.RunStepResult{
		{ID: "4001", DefinitionID: "3001", Rank: 1, Status: status, Action: "Open the gateway", Expected: "the console loads"},
		{ID: "4002", DefinitionID: "3002", Rank: 2, Status: status, Action: "Submit the form", Data: "valid payload", Expected: "202 Accepted"},
	}
}

func storedRunSteps(t *testing.T, repo *testrepo.Repository) []backend.RunStepResult {
	t.Helper()
	steps, found, err := repo.RunStepResults("p1", "501", "PROJ-101")
	if err != nil {
		t.Fatalf("read stored step results: %v", err)
	}
	if !found {
		t.Fatal("no stored step results")
	}
	return steps
}

func TestRunStepResultsRoundTripInStepOrder(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("NOT RUN")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got := storedRunSteps(t, repo)
	if len(got) != 2 || got[0].ID != "4001" || got[1].ID != "4002" {
		t.Fatalf("round trip = %+v, want both results in step order", got)
	}
	if got[0].Action != "Open the gateway" || got[1].Data != "valid payload" {
		t.Errorf("the step text did not survive: %+v", got)
	}
	if got[0].Uncommitted {
		t.Error("a result read from the backend is marked uncommitted")
	}
}

// A refresh keeps a result the user set and has not committed, and still takes
// the step text the backend reports. This is #364 on the new surface: the
// parameterized refresh used to delete and reinsert, which destroyed a local
// pick.
func TestRefreshKeepsALocallySetResultAndStillUpdatesTheStepText(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("NOT RUN")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetRunStepStatus("p1", "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("set locally: %v", err)
	}

	// The backend still reports what it last knew, and the step was reworded.
	refreshed := runStepResults("NOT RUN")
	refreshed[0].Action = "Open the gateway console"
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", refreshed); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	got := storedRunSteps(t, repo)
	if got[0].Status != "STEP FAILED" {
		t.Errorf("the refresh overwrote the local result with %q", got[0].Status)
	}
	if got[0].Action != "Open the gateway console" {
		t.Errorf("the refresh did not take the new step text: %q", got[0].Action)
	}
	if !got[0].Uncommitted {
		t.Error("a result with a pending change does not read as uncommitted")
	}
	// The untouched one takes the backend's value and is not flagged.
	if got[1].Status != "NOT RUN" || got[1].Uncommitted {
		t.Errorf("the untouched result is %+v, want the backend's value and no flag", got[1])
	}
}

// A result the backend no longer reports is dropped, or it stays behind as a
// row nothing can ever commit.
func TestRefreshDropsAResultTheBackendNoLongerReports(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("NOT RUN")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("NOT RUN")[:1]); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	got := storedRunSteps(t, repo)
	if len(got) != 1 || got[0].ID != "4001" {
		t.Fatalf("after the refresh the stored results are %+v, want only 4001", got)
	}
}

// Another execution of the same test has its own results, keyed by the same
// step ids. One execution's refresh must not touch the other's.
func TestRefreshLeavesAnotherExecutionOfTheSameTestAlone(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("NOT RUN")); err != nil {
		t.Fatalf("seed cycle 501: %v", err)
	}
	other := runStepResults("STEP PASSED")
	other[0].ID, other[1].ID = "5001", "5002"
	if err := repo.ReplaceRunStepResults("p1", "502", "PROJ-101", other); err != nil {
		t.Fatalf("seed cycle 502: %v", err)
	}
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("STEP FAILED")); err != nil {
		t.Fatalf("refresh cycle 501: %v", err)
	}

	kept, found, err := repo.RunStepResults("p1", "502", "PROJ-101")
	if err != nil || !found {
		t.Fatalf("the other execution's results are gone: (%v, %v)", found, err)
	}
	if len(kept) != 2 || kept[0].Status != "STEP PASSED" {
		t.Fatalf("the other execution now reads %+v", kept)
	}
}

// The journal row is what Commit reads, so its before value has to be the
// status the backend reported rather than the one being written.
func TestSetRunStepStatusJournalsTheBackendsValueAsTheBase(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("NOT RUN")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetRunStepStatus("p1", "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("set: %v", err)
	}
	pending, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	var found bool
	for _, c := range pending {
		if c.EntityType != "test_run_step" {
			continue
		}
		found = true
		if c.EntityKey != "501:PROJ-101:4001" {
			t.Errorf("journalled key is %q, want the execution, test and step result", c.EntityKey)
		}
		if c.BeforeVal != "NOT RUN" || c.AfterVal != "STEP FAILED" {
			t.Errorf("journalled %q -> %q, want NOT RUN -> STEP FAILED", c.BeforeVal, c.AfterVal)
		}
		if c.Field != "status" {
			t.Errorf("journalled field is %q, want status", c.Field)
		}
	}
	if !found {
		t.Fatal("setting a step result journalled nothing, so Commit would never push it")
	}
}

// Setting the value it already has journals nothing. Otherwise a Commit would
// carry a change that is not one, and the row would read as uncommitted.
func TestSetRunStepStatusToTheCurrentValueJournalsNothing(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("NOT RUN")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetRunStepStatus("p1", "501", "PROJ-101", "4001", "NOT RUN"); err != nil {
		t.Fatalf("set: %v", err)
	}
	pending, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	for _, c := range pending {
		if c.EntityType == "test_run_step" {
			t.Fatalf("journalled a change to the value already stored: %+v", c)
		}
	}
	if storedRunSteps(t, repo)[0].Uncommitted {
		t.Error("the result reads as uncommitted after a no-op")
	}
}

// A step result that does not belong to this execution is refused rather than
// journalled: a row keyed by an id the backend never reported can never
// commit, and Commit would report it failing forever.
func TestSetRunStepStatusRefusesAResultThisExecutionDoesNotHave(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceRunStepResults("p1", "501", "PROJ-101", runStepResults("NOT RUN")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := repo.SetRunStepStatus("p1", "501", "PROJ-101", "9999", "STEP FAILED")
	if err == nil {
		t.Fatal("setting a result this execution does not have succeeded")
	}
	if !strings.Contains(err.Error(), "9999") {
		t.Errorf("error is %q, want the step result named", err)
	}
	pending, _ := repo.ListPendingChanges("p1")
	for _, c := range pending {
		if c.EntityType == "test_run_step" {
			t.Fatalf("journalled a change for a result that does not exist: %+v", c)
		}
	}
}
