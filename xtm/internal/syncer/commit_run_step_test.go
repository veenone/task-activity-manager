// Committing a step result on a plain execution (#371 Phase 6).
//
// The guarantees are the ones commit_iteration.go established: check the
// current value by its stable server id before writing, hold a changed remote
// as a conflict, treat one already at the requested value as satisfied, and
// report rather than fail what the backend cannot express.

package syncer_test

import (
	"context"
	"errors"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

// runStepBackend implements backend.RunStepResultManager beside Backend, which
// is how a real one does it: the methods are not on Backend.
type runStepBackend struct {
	backend.Backend
	results  []backend.RunStepResult
	declares bool
	writeErr map[string]error
	writes   []string
}

func (b *runStepBackend) Capabilities() backend.Capabilities {
	return backend.Capabilities{SupportsRunStepResults: b.declares}
}

func (b *runStepBackend) GetRunStepResults(context.Context, string, string) ([]backend.RunStepResult, error) {
	return b.results, nil
}

func (b *runStepBackend) SetRunStepStatus(_ context.Context, _, _, stepResultID, status string) error {
	b.writes = append(b.writes, stepResultID+":"+status)
	return b.writeErr[stepResultID]
}

// unableBackend declares the capability off and implements nothing, which is
// every backend but Zephyr.
type unableBackend struct{ backend.Backend }

func (b *unableBackend) Capabilities() backend.Capabilities { return backend.Capabilities{} }

func seedRunStepCommit(t *testing.T, remote string) (*runStepBackend, *syncer.Engine, *testrepo.Repository, string) {
	t.Helper()
	repo, profileID := newSyncerRepoWithProfile(t)
	results := []backend.RunStepResult{
		{ID: "4001", DefinitionID: "3001", Rank: 1, Status: "NOT RUN", Action: "Open the gateway"},
		{ID: "4002", DefinitionID: "3002", Rank: 2, Status: "NOT RUN", Action: "Submit the form"},
	}
	if err := repo.ReplaceRunStepResults(profileID, "501", "PROJ-101", results); err != nil {
		t.Fatalf("seed step results: %v", err)
	}
	remoteResults := []backend.RunStepResult{
		{ID: "4001", DefinitionID: "3001", Rank: 1, Status: remote},
		{ID: "4002", DefinitionID: "3002", Rank: 2, Status: "NOT RUN"},
	}
	b := &runStepBackend{results: remoteResults, declares: true, writeErr: map[string]error{}}
	return b, syncer.New(b, repo), repo, profileID
}

func TestCommitRunStepResultClearsTheAcceptedChange(t *testing.T) {
	b, engine, repo, profileID := seedRunStepCommit(t, "NOT RUN")
	if err := repo.SetRunStepStatus(profileID, "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("edit step result: %v", err)
	}

	result, err := engine.CommitChanges(context.Background(), profileID, "PROJ")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Failed) != 0 || len(result.Conflicted) != 0 {
		t.Fatalf("result = %+v, want a clean commit", result)
	}
	if len(b.writes) != 1 || b.writes[0] != "4001:STEP FAILED" {
		t.Fatalf("wrote %v, want the one step result", b.writes)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 0 {
		t.Fatalf("the accepted step result is still pending: %+v", pending)
	}
}

// Somebody else recorded a different result since the local pick. Writing over
// it would discard their result silently.
func TestCommitRunStepResultHoldsAChangedRemoteAsAConflict(t *testing.T) {
	b, engine, repo, profileID := seedRunStepCommit(t, "STEP PASSED")
	if err := repo.SetRunStepStatus(profileID, "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("edit step result: %v", err)
	}

	result, err := engine.CommitChanges(context.Background(), profileID, "PROJ")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Conflicted) != 1 {
		t.Fatalf("conflicted = %+v, want the one step result", result.Conflicted)
	}
	field := result.Conflicted[0].Fields[0]
	if field.Remote != "STEP PASSED" || field.Mine != "STEP FAILED" || field.Base != "NOT RUN" {
		t.Errorf("conflict is %+v, want base NOT RUN, remote STEP PASSED, mine STEP FAILED", field)
	}
	if field.EntityType != "test_run_step" {
		t.Errorf("conflict entity type is %q", field.EntityType)
	}
	// The label reaches the conflict dialog, where "Dataset iteration result"
	// would name something this profile does not have.
	if field.Label != "Step result" {
		t.Errorf("conflict label is %q, want Step result", field.Label)
	}
	if len(b.writes) != 0 {
		t.Errorf("wrote over a conflicting remote result: %v", b.writes)
	}
}

// The remote already holds the requested value, so there is nothing to write
// and the local row has served its purpose.
func TestCommitRunStepResultAlreadyAtTheRequestedValueOnlyClearsLocally(t *testing.T) {
	b, engine, repo, profileID := seedRunStepCommit(t, "STEP FAILED")
	if err := repo.SetRunStepStatus(profileID, "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("edit step result: %v", err)
	}

	result, err := engine.CommitChanges(context.Background(), profileID, "PROJ")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Conflicted) != 0 || len(result.Failed) != 0 {
		t.Fatalf("result = %+v, want it treated as satisfied", result)
	}
	if len(b.writes) != 0 {
		t.Errorf("wrote a value the backend already held: %v", b.writes)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 0 {
		t.Errorf("the satisfied row is still pending: %+v", pending)
	}
}

// A result the backend no longer reports cannot be written by its id, and
// nothing local can fix that, so it is reported rather than failed every time.
func TestCommitRunStepResultTheBackendNoLongerReportsIsSkipped(t *testing.T) {
	b, engine, repo, profileID := seedRunStepCommit(t, "NOT RUN")
	if err := repo.SetRunStepStatus(profileID, "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("edit step result: %v", err)
	}
	b.results = b.results[1:]

	result, err := engine.CommitChanges(context.Background(), profileID, "PROJ")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].EntityType != "test_run_step" {
		t.Fatalf("skipped = %+v, want the one step result", result.Skipped)
	}
	if len(b.writes) != 0 {
		t.Errorf("wrote a result the backend does not report: %v", b.writes)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 1 {
		t.Errorf("the skipped row did not stay pending: %+v", pending)
	}
}

// A profile whose backend has no addressable step results, which is every
// backend but Zephyr. The row is reported and kept, not failed: an error on
// every Commit for something the user cannot act on is what #364 fixed.
func TestCommitRunStepResultOnABackendWithoutThemIsSkipped(t *testing.T) {
	repo, profileID := newSyncerRepoWithProfile(t)
	if err := repo.ReplaceRunStepResults(profileID, "501", "PROJ-101",
		[]backend.RunStepResult{{ID: "4001", DefinitionID: "3001", Rank: 1, Status: "NOT RUN"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetRunStepStatus(profileID, "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	result, err := syncer.New(&unableBackend{}, repo).CommitChanges(context.Background(), profileID, "PROJ")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Errorf("reported as failed: %+v", result.Failed)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("skipped = %+v, want the one step result", result.Skipped)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 1 {
		t.Errorf("the skipped row did not stay pending: %+v", pending)
	}
}

// A backend that declares the capability and does not implement the interface
// is a wiring mistake, not something the user did. Without the assertion the
// commit path would call a nil interface and take the app down mid-Commit, so
// the branch is a trust boundary and gets a test of its own (I1).
type lyingBackend struct{ backend.Backend }

func (b *lyingBackend) Capabilities() backend.Capabilities {
	return backend.Capabilities{SupportsRunStepResults: true}
}

func TestCommitRunStepResultOnABackendThatOnlyClaimsThemIsSkipped(t *testing.T) {
	repo, profileID := newSyncerRepoWithProfile(t)
	if err := repo.ReplaceRunStepResults(profileID, "501", "PROJ-101",
		[]backend.RunStepResult{{ID: "4001", DefinitionID: "3001", Rank: 1, Status: "NOT RUN"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.SetRunStepStatus(profileID, "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	result, err := syncer.New(&lyingBackend{}, repo).CommitChanges(context.Background(), profileID, "PROJ")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Skipped) != 1 || len(result.Failed) != 0 {
		t.Fatalf("result = %+v, want the row reported and kept", result)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 1 {
		t.Errorf("the skipped row did not stay pending: %+v", pending)
	}
}

// A refused write keeps its own row pending and says why.
func TestCommitRunStepResultRefusedByTheBackendStaysPending(t *testing.T) {
	b, engine, repo, profileID := seedRunStepCommit(t, "NOT RUN")
	b.writeErr["4001"] = errors.New("zephyr: PUT /stepResult/4001 -> 400 Bad Request")
	if err := repo.SetRunStepStatus(profileID, "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	result, err := engine.CommitChanges(context.Background(), profileID, "PROJ")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("failed = %+v, want the one step result", result.Failed)
	}
	if pending, _ := repo.ListPendingChanges(profileID); len(pending) != 1 {
		t.Errorf("a refused write cleared its row: %+v", pending)
	}
}
