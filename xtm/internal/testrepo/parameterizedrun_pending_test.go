// A refresh must not overwrite a result set locally (#364).
//
// ReplaceParameterizedRun used to delete the whole iteration tree and reinsert
// it from the backend, so a result the user had set and not yet committed was
// replaced by whatever the backend reported. For a step that is always the
// iteration's own status, because the live reader has no per-step status.

package testrepo_test

import (
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

// liveShapedRun is what xray.liveParameterizedRun returns: two iterations, each
// with two steps, every step carrying the iteration's status.
func liveShapedRun(iterationStatus string) backend.ParameterizedRun {
	iteration := func(id string, rank int) backend.RunIteration {
		return backend.RunIteration{
			ID: id, Rank: rank, Status: iterationStatus,
			Parameters: []backend.IterationParameter{{Name: "TIER", Value: "gold"}},
			Steps: []backend.IterationStep{
				{ID: id + "-step-0", DefinitionID: "1", Rank: 0, Status: iterationStatus, Action: "sign in"},
				{ID: id + "-step-1", DefinitionID: "2", Rank: 1, Status: iterationStatus, Action: "sign out"},
			},
		}
	}
	return backend.ParameterizedRun{
		ID: "3278564", ExecutionKey: "EXEC-1", TestKey: "QA-1",
		DatasetSourceKind: backend.DatasetSourceTest, DatasetSourceKey: "QA-1",
		Status:     iterationStatus,
		Iterations: []backend.RunIteration{iteration("11862", 0), iteration("11863", 1)},
	}
}

func storedRun(t *testing.T, repo *testrepo.Repository) backend.ParameterizedRun {
	t.Helper()
	run, found, err := repo.ParameterizedRun("p1", "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("read stored run: %v", err)
	}
	if !found {
		t.Fatal("no stored run")
	}
	return run
}

func TestRefreshKeepsAStepResultSetLocally(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceParameterizedRun("p1", liveShapedRun("TO"+"DO")); err != nil {
		t.Fatalf("store the run: %v", err)
	}

	// The user sets one step's result. This journals a pending change.
	if err := repo.SetIterationStepStatus("p1", "EXEC-1", "QA-1", "11863", "11863-step-1", "BLOCKED"); err != nil {
		t.Fatalf("set the step result: %v", err)
	}

	// Xray is then re-read and reports the iteration as ABORTED, which makes
	// every step ABORTED too, because it has no per-step status to report.
	if err := repo.ReplaceParameterizedRun("p1", liveShapedRun("ABORTED")); err != nil {
		t.Fatalf("refresh the run: %v", err)
	}

	run := storedRun(t, repo)
	var found bool
	for _, iteration := range run.Iterations {
		for _, step := range iteration.Steps {
			if step.ID != "11863-step-1" {
				continue
			}
			found = true
			if step.Status != "BLOCKED" {
				t.Errorf("step result is %q after the refresh, want the local BLOCKED to survive", step.Status)
			}
			if !step.Uncommitted {
				t.Error("the surviving step result is not marked uncommitted, so it reads as a value Xray holds")
			}
		}
	}
	if !found {
		t.Fatal("the edited step is missing from the stored run")
	}
}

func TestRefreshKeepsAnIterationResultSetLocally(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceParameterizedRun("p1", liveShapedRun("TO"+"DO")); err != nil {
		t.Fatalf("store the run: %v", err)
	}
	if err := repo.SetIterationStatus("p1", "EXEC-1", "QA-1", "11862", "PASS"); err != nil {
		t.Fatalf("set the iteration result: %v", err)
	}
	if err := repo.ReplaceParameterizedRun("p1", liveShapedRun("FAIL")); err != nil {
		t.Fatalf("refresh the run: %v", err)
	}

	run := storedRun(t, repo)
	for _, iteration := range run.Iterations {
		switch iteration.ID {
		case "11862":
			if iteration.Status != "PASS" {
				t.Errorf("edited iteration is %q, want the local PASS to survive", iteration.Status)
			}
			if !iteration.Uncommitted {
				t.Error("edited iteration is not marked uncommitted")
			}
		case "11863":
			// Untouched, so the refresh is free to move it.
			if iteration.Status != "FAIL" {
				t.Errorf("untouched iteration is %q, want the refreshed FAIL", iteration.Status)
			}
			if iteration.Uncommitted {
				t.Error("untouched iteration is marked uncommitted")
			}
		}
	}
}

// A dataset that lost a row must not leave the extra iteration behind, or the
// view shows a result nothing can commit.
func TestRefreshDropsWhatTheBackendNoLongerReports(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceParameterizedRun("p1", liveShapedRun("TO"+"DO")); err != nil {
		t.Fatalf("store the run: %v", err)
	}
	shrunk := liveShapedRun("TO" + "DO")
	shrunk.Iterations = shrunk.Iterations[:1]
	shrunk.Iterations[0].Steps = shrunk.Iterations[0].Steps[:1]
	if err := repo.ReplaceParameterizedRun("p1", shrunk); err != nil {
		t.Fatalf("refresh the shrunk run: %v", err)
	}

	run := storedRun(t, repo)
	if len(run.Iterations) != 1 {
		t.Fatalf("stored %d iterations, want the one the backend still reports", len(run.Iterations))
	}
	if len(run.Iterations[0].Steps) != 1 {
		t.Fatalf("stored %d steps, want the one the backend still reports", len(run.Iterations[0].Steps))
	}
}
