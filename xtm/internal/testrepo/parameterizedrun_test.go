package testrepo_test

import (
	"reflect"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend"
)

func parameterizedRun(testKey string) backend.ParameterizedRun {
	return backend.ParameterizedRun{
		ID: "run-" + testKey, ExecutionKey: "QA-EXEC-1", TestKey: testKey,
		DatasetSourceKind: backend.DatasetSourceExecution,
		DatasetSourceKey:  "QA-EXEC-1",
		Status:            "FAIL",
		Iterations: []backend.RunIteration{
			{
				ID: "iteration-1", Rank: 0, Status: "PASS",
				Parameters: []backend.IterationParameter{{Name: "USER", Value: "alice"}},
				Steps:      []backend.IterationStep{{ID: "result-1", DefinitionID: "step-1", Rank: 0, Status: "PASS", Action: "Sign in as alice", Data: "", Expected: "Dashboard"}},
			},
			{
				ID: "iteration-2", Rank: 1, Status: "FAIL",
				Parameters: []backend.IterationParameter{{Name: "USER", Value: "bob"}},
				Steps:      []backend.IterationStep{},
			},
		},
	}
}

func TestParameterizedRunRoundTripsWithOrderedNonNilChildren(t *testing.T) {
	repo := newRepo(t)
	want := parameterizedRun("QA-1")
	if err := repo.ReplaceParameterizedRun("p1", want); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, found, err := repo.ParameterizedRun("p1", "QA-EXEC-1", "QA-1")
	if err != nil || !found {
		t.Fatalf("read = (%+v, %v, %v), want found", got, found, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
	if got.Iterations[1].Steps == nil {
		t.Fatal("empty iteration steps must be non-nil")
	}
}

func TestReplaceParameterizedRunRemovesStaleChildrenButKeepsAnotherTest(t *testing.T) {
	repo := newRepo(t)
	first := parameterizedRun("QA-1")
	other := parameterizedRun("QA-2")
	other.Iterations[0].ID = "other-iteration"
	other.Iterations[0].Steps[0].ID = "other-result"
	other.Iterations = other.Iterations[:1]
	if err := repo.ReplaceParameterizedRun("p1", first); err != nil {
		t.Fatalf("seed first: %v", err)
	}
	if err := repo.ReplaceParameterizedRun("p1", other); err != nil {
		t.Fatalf("seed other: %v", err)
	}

	replacement := first
	replacement.Iterations = replacement.Iterations[:1]
	replacement.Iterations[0].Steps = []backend.IterationStep{}
	if err := repo.ReplaceParameterizedRun("p1", replacement); err != nil {
		t.Fatalf("replace first: %v", err)
	}
	got, _, _ := repo.ParameterizedRun("p1", "QA-EXEC-1", "QA-1")
	if len(got.Iterations) != 1 || len(got.Iterations[0].Steps) != 0 {
		t.Fatalf("replacement retained stale children: %+v", got)
	}
	kept, found, err := repo.ParameterizedRun("p1", "QA-EXEC-1", "QA-2")
	if err != nil || !found || len(kept.Iterations) != 1 {
		t.Fatalf("other test changed: (%+v, %v, %v)", kept, found, err)
	}
}

func TestInvalidParameterizedRunLeavesPreviousTreeUntouched(t *testing.T) {
	repo := newRepo(t)
	want := parameterizedRun("QA-1")
	if err := repo.ReplaceParameterizedRun("p1", want); err != nil {
		t.Fatalf("seed: %v", err)
	}
	invalid := parameterizedRun("QA-1")
	invalid.Iterations[1].ID = invalid.Iterations[0].ID
	if err := repo.ReplaceParameterizedRun("p1", invalid); err == nil || !strings.Contains(err.Error(), "duplicate iteration id") {
		t.Fatalf("invalid replacement error = %v", err)
	}
	got, found, err := repo.ParameterizedRun("p1", "QA-EXEC-1", "QA-1")
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("tree after invalid replacement = (%+v, %v, %v), want %+v", got, found, err, want)
	}
}

func TestSetIterationStatusCoalescesAndReverts(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceParameterizedRun("p1", parameterizedRun("QA-1")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetIterationStatus("p1", "QA-EXEC-1", "QA-1", "iteration-1", "FAIL"); err != nil {
		t.Fatalf("first edit: %v", err)
	}
	if err := repo.SetIterationStatus("p1", "QA-EXEC-1", "QA-1", "iteration-1", "BLOCKED"); err != nil {
		t.Fatalf("second edit: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "test_run_iteration" || changes[0].BeforeVal != "PASS" || changes[0].AfterVal != "BLOCKED" {
		t.Fatalf("coalesced change = %+v", changes)
	}
	if err := repo.SetIterationStatus("p1", "QA-EXEC-1", "QA-1", "iteration-1", "PASS"); err != nil {
		t.Fatalf("revert: %v", err)
	}
	changes, _ = repo.ListPendingChanges("p1")
	if len(changes) != 0 {
		t.Fatalf("revert left changes: %+v", changes)
	}
	if err := repo.SetIterationStatus("p1", "QA-EXEC-1", "QA-1", "missing", "PASS"); err == nil {
		t.Fatal("missing iteration edit succeeded")
	}
}

func TestSetIterationStepStatusUsesStableResultMembership(t *testing.T) {
	repo := newRepo(t)
	if err := repo.ReplaceParameterizedRun("p1", parameterizedRun("QA-1")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetIterationStepStatus("p1", "QA-EXEC-1", "QA-1", "iteration-1", "result-1", "FAIL"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "test_run_iteration_step" || changes[0].BeforeVal != "PASS" || changes[0].AfterVal != "FAIL" {
		t.Fatalf("step change = %+v", changes)
	}
	if err := repo.SetIterationStepStatus("p1", "QA-EXEC-1", "QA-1", "iteration-2", "result-1", "PASS"); err == nil {
		t.Fatal("step result from another iteration was accepted")
	}
}
