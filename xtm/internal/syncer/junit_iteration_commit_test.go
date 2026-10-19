package syncer_test

import (
	"context"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

type junitNewExecBackend struct {
	backend.Backend
	sourceKind backend.DatasetSourceKind
	writtenID  string
}

func (b *junitNewExecBackend) Capabilities() backend.Capabilities {
	return backend.Capabilities{SupportsContainers: true, SupportsParameterizedRunDetail: true, SupportsParameterizedRunWrites: true}
}
func (b *junitNewExecBackend) CreateContainer(context.Context, string, string, string) (string, error) {
	return "QA-EXEC-99", nil
}
func (b *junitNewExecBackend) AddTestsToContainer(context.Context, string, string, []string) error {
	return nil
}
func (b *junitNewExecBackend) GetParameterizedRun(context.Context, string, string) (backend.ParameterizedRun, error) {
	return backend.ParameterizedRun{
		ID: "run-99", ExecutionKey: "QA-EXEC-99", TestKey: "TC-1",
		DatasetSourceKind: b.sourceKind, DatasetSourceKey: "TC-1", Status: "PASS",
		Iterations: []backend.RunIteration{{ID: "server-iteration-bob", Rank: 0, Status: "PASS", Parameters: []backend.IterationParameter{{Name: "USER", Value: "bob"}}, Steps: []backend.IterationStep{}}},
	}, nil
}
func (b *junitNewExecBackend) SetIterationStatus(_ context.Context, _, _ string, iterationID, _ string) error {
	b.writtenID = iterationID
	return nil
}

func TestJUnitNewExecIterationResolvesStableIDAfterContainerCreation(t *testing.T) {
	repo, profileID := newSyncerRepoWithProfile(t)
	if err := repo.UpsertTests(profileID, []testrepo.TestCase{{Key: "TC-1", Summary: "Existing One"}}); err != nil {
		t.Fatal(err)
	}
	rows := []testrepo.JUnitNewExecRow{{
		Testcase: "Existing One", TestKey: "TC-1", Summary: "Existing One", Result: "FAIL",
		TargetKind: testrepo.JUnitTargetIteration, Parameters: []backend.IterationParameter{{Name: "USER", Value: "bob"}},
		DatasetSourceKind: backend.DatasetSourceTest, DatasetSourceKey: "TC-1",
	}}
	if _, err := repo.ApplyJUnitImportNewExec(profileID, "QA", "Imported", rows); err != nil {
		t.Fatal(err)
	}
	b := &junitNewExecBackend{sourceKind: backend.DatasetSourceTest}
	result, err := syncer.New(b, repo).CommitChanges(context.Background(), profileID, "QA")
	if err != nil || len(result.Failed) != 0 || len(result.Conflicted) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if b.writtenID != "server-iteration-bob" {
		t.Fatalf("written iteration = %q, want stable server id", b.writtenID)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 0 {
		t.Fatalf("pending after commit = %+v", pending)
	}
}

func TestJUnitNewExecIterationConflictsWhenDatasetSourceChanged(t *testing.T) {
	repo, profileID := newSyncerRepoWithProfile(t)
	if err := repo.UpsertTests(profileID, []testrepo.TestCase{{Key: "TC-1", Summary: "Existing One"}}); err != nil {
		t.Fatal(err)
	}
	rows := []testrepo.JUnitNewExecRow{{
		Testcase: "Existing One", TestKey: "TC-1", Summary: "Existing One", Result: "FAIL",
		TargetKind: testrepo.JUnitTargetIteration, Parameters: []backend.IterationParameter{{Name: "USER", Value: "bob"}},
		DatasetSourceKind: backend.DatasetSourceTest, DatasetSourceKey: "TC-1",
	}}
	if _, err := repo.ApplyJUnitImportNewExec(profileID, "QA", "Imported", rows); err != nil {
		t.Fatal(err)
	}
	b := &junitNewExecBackend{sourceKind: backend.DatasetSourceExecution}
	result, _ := syncer.New(b, repo).CommitChanges(context.Background(), profileID, "QA")
	if len(result.Conflicted) != 1 || b.writtenID != "" {
		t.Fatalf("result=%+v written=%q, want retained conflict", result, b.writtenID)
	}
}
