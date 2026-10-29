package syncer_test

import (
	"context"
	"errors"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

type iterationCommitBackend struct {
	backend.Backend
	run              backend.ParameterizedRun
	writes           bool
	iterationErr     map[string]error
	iterationCalls   []string
	iterationStepIDs []string
}

func (b *iterationCommitBackend) Capabilities() backend.Capabilities {
	return backend.Capabilities{SupportsParameterizedRunDetail: true, SupportsParameterizedRunWrites: b.writes}
}

func (b *iterationCommitBackend) GetParameterizedRun(context.Context, string, string) (backend.ParameterizedRun, error) {
	return b.run, nil
}

func (b *iterationCommitBackend) SetIterationStatus(_ context.Context, _, _ string, iterationID, status string) error {
	b.iterationCalls = append(b.iterationCalls, iterationID+":"+status)
	return b.iterationErr[iterationID]
}

func (b *iterationCommitBackend) SetIterationStepStatus(_ context.Context, _, _ string, iterationID, stepResultID, status string) error {
	b.iterationStepIDs = append(b.iterationStepIDs, iterationID+":"+stepResultID+":"+status)
	return b.iterationErr[stepResultID]
}

func seedIterationCommit(t *testing.T) (*iterationCommitBackend, *syncer.Engine, *testrepo.Repository, string) {
	t.Helper()
	repo, profileID := newSyncerRepoWithProfile(t)
	run := backend.ParameterizedRun{
		ID: "run-1", ExecutionKey: "QA-EXEC-1", TestKey: "QA-1",
		DatasetSourceKind: backend.DatasetSourceTest, DatasetSourceKey: "QA-1", Status: "PASS",
		Iterations: []backend.RunIteration{
			{ID: "iteration-1", Rank: 0, Status: "PASS", Parameters: []backend.IterationParameter{}, Steps: []backend.IterationStep{{ID: "result-1", DefinitionID: "step-1", Rank: 0, Status: "PASS"}}},
			{ID: "iteration-2", Rank: 1, Status: "PASS", Parameters: []backend.IterationParameter{}, Steps: []backend.IterationStep{{ID: "result-2", DefinitionID: "step-1", Rank: 0, Status: "PASS"}}},
		},
	}
	if err := repo.ReplaceParameterizedRun(profileID, run); err != nil {
		t.Fatalf("seed parameterized run: %v", err)
	}
	b := &iterationCommitBackend{run: run, writes: true, iterationErr: map[string]error{}}
	return b, syncer.New(b, repo), repo, profileID
}

func TestCommitIterationStatusClearsOnlyTheAcceptedChange(t *testing.T) {
	b, engine, repo, profileID := seedIterationCommit(t)
	if err := repo.SetIterationStatus(profileID, "QA-EXEC-1", "QA-1", "iteration-1", "FAIL"); err != nil {
		t.Fatalf("edit iteration: %v", err)
	}

	result, err := engine.CommitChanges(context.Background(), profileID, "QA")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Failed) != 0 || len(result.Conflicted) != 0 || len(b.iterationCalls) != 1 {
		t.Fatalf("result=%+v calls=%v", result, b.iterationCalls)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 0 {
		t.Fatalf("accepted iteration remains pending: %+v", pending)
	}
}

// A step result has nowhere to go on live Xray, because the iteration resource
// exposes no step-result id (#336). It is reported and left pending rather than
// failed, so Commit does not show an error the user cannot act on (#364).
func TestCommitSkipsAStepResultTheBackendCannotWrite(t *testing.T) {
	b, engine, repo, profileID := seedIterationCommit(t)
	b.iterationErr["result-1"] = backend.ErrUnsupported
	if err := repo.SetIterationStepStatus(profileID, "QA-EXEC-1", "QA-1", "iteration-1", "result-1", "BLOCKED"); err != nil {
		t.Fatalf("edit step: %v", err)
	}

	result, err := engine.CommitChanges(context.Background(), profileID, "QA")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Errorf("step result reported as failed: %+v", result.Failed)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("skipped = %+v, want the one step result", result.Skipped)
	}
	if result.Skipped[0].EntityType != "test_run_iteration_step" {
		t.Errorf("skipped the wrong entity: %+v", result.Skipped[0])
	}
	// Left pending, so it is still visible and still discardable.
	pending, _ := repo.ListPendingChanges(profileID)
	if len(pending) != 1 {
		t.Errorf("pending = %+v, want the step result to stay", pending)
	}
}

func TestCommitIterationOnDemo2ClearsTheAcceptedChange(t *testing.T) {
	repo, profileID := newSyncerRepoWithProfile(t)
	b := xray.New(jira.NewClient("demo", "token"))
	run, err := b.GetParameterizedRun(context.Background(), "DEMO2-STE-1", "DEMO2-1")
	if err != nil {
		t.Fatalf("load Demo2 run: %v", err)
	}
	if err := repo.ReplaceParameterizedRun(profileID, run); err != nil {
		t.Fatalf("seed Demo2 run: %v", err)
	}
	if err := repo.SetIterationStatus(profileID, run.ExecutionKey, run.TestKey, run.Iterations[0].ID, "FAIL"); err != nil {
		t.Fatalf("edit Demo2 iteration: %v", err)
	}

	result, err := syncer.New(b, repo).CommitChanges(context.Background(), profileID, "DEMO2")
	if err != nil || len(result.Failed) != 0 || len(result.Conflicted) != 0 {
		t.Fatalf("commit result=%+v err=%v", result, err)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 0 {
		t.Fatalf("accepted Demo2 result remains pending: %+v", pending)
	}
}

func TestCommitIterationBackendRefusalKeepsTheChange(t *testing.T) {
	b, engine, repo, profileID := seedIterationCommit(t)
	b.iterationErr["iteration-1"] = errors.New("write refused")
	if err := repo.SetIterationStatus(profileID, "QA-EXEC-1", "QA-1", "iteration-1", "FAIL"); err != nil {
		t.Fatal(err)
	}

	result, _ := engine.CommitChanges(context.Background(), profileID, "QA")
	if len(result.Failed) != 1 {
		t.Fatalf("failures = %+v, want one", result.Failed)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 1 {
		t.Fatalf("pending = %+v, want refused change retained", pending)
	}
}

func TestCommitIterationWithoutWriteCapabilitySkipsAndKeepsTheChange(t *testing.T) {
	b, engine, repo, profileID := seedIterationCommit(t)
	if err := repo.SetIterationStatus(profileID, "QA-EXEC-1", "QA-1", "iteration-1", "FAIL"); err != nil {
		t.Fatal(err)
	}
	b.writes = false

	result, _ := engine.CommitChanges(context.Background(), profileID, "QA")
	if len(result.Skipped) != 1 || len(b.iterationCalls) != 0 {
		t.Fatalf("skipped=%+v calls=%v", result.Skipped, b.iterationCalls)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 1 {
		t.Fatalf("pending = %+v, want unsupported change retained", pending)
	}
}

func TestCommitIterationDetectsRemoteStatusMismatch(t *testing.T) {
	b, engine, repo, profileID := seedIterationCommit(t)
	if err := repo.SetIterationStatus(profileID, "QA-EXEC-1", "QA-1", "iteration-1", "FAIL"); err != nil {
		t.Fatal(err)
	}
	b.run.Iterations[0].Status = "BLOCKED"

	result, _ := engine.CommitChanges(context.Background(), profileID, "QA")
	if len(result.Conflicted) != 1 || len(b.iterationCalls) != 0 {
		t.Fatalf("conflicts=%+v calls=%v", result.Conflicted, b.iterationCalls)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 1 {
		t.Fatalf("pending = %+v, want conflicted change retained", pending)
	}
}

func TestCommitIterationFailureDoesNotDeleteSiblingChange(t *testing.T) {
	b, engine, repo, profileID := seedIterationCommit(t)
	if err := repo.SetIterationStatus(profileID, "QA-EXEC-1", "QA-1", "iteration-1", "FAIL"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetIterationStepStatus(profileID, "QA-EXEC-1", "QA-1", "iteration-2", "result-2", "FAIL"); err != nil {
		t.Fatal(err)
	}
	b.iterationErr["iteration-1"] = errors.New("first failed")

	result, _ := engine.CommitChanges(context.Background(), profileID, "QA")
	if len(result.Failed) != 1 || len(b.iterationStepIDs) != 1 {
		t.Fatalf("result=%+v step calls=%v", result, b.iterationStepIDs)
	}
	pending, _ := repo.ListPendingChanges(profileID)
	if len(pending) != 1 || pending[0].EntityKey != "QA-EXEC-1:QA-1:iteration-1" {
		t.Fatalf("pending = %+v, want only failed sibling", pending)
	}
}
