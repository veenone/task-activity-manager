// Clearing and resolving an iteration result (#369).
//
// An iteration row is keyed execution first, "<execKey>:<testKey>:<iterationID>",
// so every helper that gathers a Test's pending rows by test-key prefix missed
// them: discarding a conflicted Test reported success and changed nothing.

package testrepo_test

import (
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func pendingFor(t *testing.T, repo *testrepo.Repository, entityKey string) (testrepo.PendingChange, bool) {
	t.Helper()
	rows, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	for _, row := range rows {
		if row.EntityKey == entityKey {
			return row, true
		}
	}
	return testrepo.PendingChange{}, false
}

// "Discard mine" on a conflicted Test has to reach the Test's iteration rows,
// not only the rows keyed by the Test itself.
func TestDiscardTestChangesClearsAnIterationResult(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceParameterizedRun("p1", liveShapedRun("EXECUTING")); err != nil {
		t.Fatalf("store the run: %v", err)
	}
	if err := repo.SetIterationStatus("p1", "EXEC-1", "QA-1", "11862", "BLOCKED"); err != nil {
		t.Fatalf("set iteration result: %v", err)
	}
	if err := repo.SetIterationStepStatus("p1", "EXEC-1", "QA-1", "11863", "11863-step-1", "BLOCKED"); err != nil {
		t.Fatalf("set step result: %v", err)
	}

	if err := repo.DiscardTestChanges("p1", "QA-1"); err != nil {
		t.Fatalf("discard the test's changes: %v", err)
	}

	if row, found := pendingFor(t, repo, "EXEC-1:QA-1:11862"); found {
		t.Errorf("iteration result survived the discard: %+v", row)
	}
	if row, found := pendingFor(t, repo, "EXEC-1:QA-1:11863:11863-step-1"); found {
		t.Errorf("step result survived the discard: %+v", row)
	}
	// Discarding restores what the backend reported, not the local pick.
	run := storedRun(t, repo)
	if run.Iterations[0].Status != "EXECUTING" {
		t.Errorf("iteration status = %q, want the remote EXECUTING back", run.Iterations[0].Status)
	}
	if run.Iterations[1].Steps[1].Status != "EXECUTING" {
		t.Errorf("step status = %q, want the remote EXECUTING back", run.Iterations[1].Steps[1].Status)
	}
}

// Discarding one row from the pending list must revert the cached status. The
// discard switch ends in a default that errors on an unknown entity type, and
// neither iteration entity had a case, so this failed outright.
func TestDiscardPendingChangeRevertsAnIterationStepResult(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceParameterizedRun("p1", liveShapedRun("PASS")); err != nil {
		t.Fatalf("store the run: %v", err)
	}
	if err := repo.SetIterationStepStatus("p1", "EXEC-1", "QA-1", "11862", "11862-step-1", "FAIL"); err != nil {
		t.Fatalf("set step result: %v", err)
	}
	row, found := pendingFor(t, repo, "EXEC-1:QA-1:11862:11862-step-1")
	if !found {
		t.Fatal("the step edit was never journalled")
	}

	if err := repo.DiscardPendingChange("p1", row.ID); err != nil {
		t.Fatalf("discard the step result: %v", err)
	}

	if run := storedRun(t, repo); run.Iterations[0].Steps[1].Status != "PASS" {
		t.Errorf("step status = %q, want the remote PASS back", run.Iterations[0].Steps[1].Status)
	}
	if row, found := pendingFor(t, repo, "EXEC-1:QA-1:11862:11862-step-1"); found {
		t.Errorf("the discarded row is still pending: %+v", row)
	}
}

// Keeping mine re-bases the row's before value onto what the backend holds now.
// An iteration commit compares those values rather than a version token, so
// without this the same conflict returns on every commit, exactly as it did for
// datasets (#277 W1).
func TestResolveConflictKeepingMineRebasesAnIterationResult(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceParameterizedRun("p1", liveShapedRun("EXECUTING")); err != nil {
		t.Fatalf("store the run: %v", err)
	}
	if err := repo.SetIterationStatus("p1", "EXEC-1", "QA-1", "11862", "BLOCKED"); err != nil {
		t.Fatalf("set iteration result: %v", err)
	}
	row, found := pendingFor(t, repo, "EXEC-1:QA-1:11862")
	if !found {
		t.Fatal("the iteration edit was never journalled")
	}

	if err := repo.ResolveConflictMerge("p1", "QA-1", "", []testrepo.ConflictDecision{{
		PendingID:   row.ID,
		EntityType:  "test_run_iteration",
		EntityKey:   "EXEC-1:QA-1:11862",
		Field:       "status",
		Choice:      "mine",
		RemoteValue: "ABORTED",
	}}); err != nil {
		t.Fatalf("resolve keeping mine: %v", err)
	}

	after, found := pendingFor(t, repo, "EXEC-1:QA-1:11862")
	if !found {
		t.Fatal("keeping mine dropped the row")
	}
	if after.BeforeVal != "ABORTED" {
		t.Errorf("before = %q, want the remote ABORTED so the commit stops conflicting", after.BeforeVal)
	}
	if after.AfterVal != "BLOCKED" {
		t.Errorf("after = %q, want my own BLOCKED kept", after.AfterVal)
	}
}
