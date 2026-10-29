// The live reader's output has to survive the store (#362).
//
// app.GetParameterizedRun does not hand the backend's run to the view. It
// writes it with ReplaceParameterizedRun and reads it back, so a run the store
// refuses never reaches the member table at all. The adapter tests beside this
// one stop at the adapter's return value, which is why a run that no store
// would accept shipped as working.

package xray

import (
	"context"
	"path/filepath"
	"testing"

	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/testrepo"
)

func newRunStore(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "run.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st)
}

// Two steps in one iteration, which is what catches a step key that is not
// unique: test_run_iteration_step is keyed by step_result_id, so two steps
// sharing one would collide instead of storing.
const liveIterationWithTwoSteps = `{"id":11862,"testRunId":3278564,"status":"PASS",
 "parameters":[{"name":"TIER","value":"gold"}],
 "steps":[
  {"index":1,"fields":{
   "Action":{"type":"Wiki","value":{"raw":"sign in","rendered":"<p>sign in</p>"}},
   "Data":{"type":"Wiki","value":{"raw":"gold","rendered":"gold"}},
   "Expected Result":{"type":"Wiki","value":{"raw":"signed in","rendered":"<p>signed in</p>"}}}},
  {"index":2,"fields":{
   "Action":{"type":"Wiki","value":{"raw":"sign out","rendered":"<p>sign out</p>"}},
   "Data":{"type":"Wiki","value":{"raw":"gold","rendered":"gold"}},
   "Expected Result":{"type":"Wiki","value":{"raw":"signed out","rendered":"<p>signed out</p>"}}}}]}`

func TestLiveParameterizedRunSurvivesTheStore(t *testing.T) {
	srv, _ := liveRunServer(t, liveRunTwoIterations, map[string]string{
		"11862": liveIterationWithTwoSteps,
		"11863": `{"id":11863,"testRunId":3278564,"status":"TO\u0044O","steps":[]}`,
	})
	defer srv.Close()

	run, err := New(jira.NewClient(srv.URL, "tok")).
		GetParameterizedRun(context.Background(), "QA-EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetParameterizedRun: %v", err)
	}

	repo := newRunStore(t)
	if err := repo.ReplaceParameterizedRun("p1", run); err != nil {
		t.Fatalf("the store refused the live run: %v", err)
	}
	stored, found, err := repo.ParameterizedRun("p1", "QA-EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("read the stored run: %v", err)
	}
	if !found {
		t.Fatal("the stored run was not found, so the view would get nothing")
	}
	if len(stored.Iterations) != 2 {
		t.Fatalf("stored %d iterations, want 2", len(stored.Iterations))
	}
	// Both steps survive. A shared step key would keep one and lose the other.
	first := stored.Iterations[0]
	if len(first.Steps) != 2 {
		t.Fatalf("first iteration stored %d steps, want 2", len(first.Steps))
	}
	if first.Steps[0].Action != "sign in" || first.Steps[1].Action != "sign out" {
		t.Errorf("steps came back as %q and %q", first.Steps[0].Action, first.Steps[1].Action)
	}
	// The text Xray already resolved is what is stored, not a placeholder.
	if first.Steps[0].Data != "gold" {
		t.Errorf("step data = %q, want the value Xray resolved", first.Steps[0].Data)
	}
	if stored.Iterations[1].Status != "TO\u0044O" {
		t.Errorf("second iteration status = %q", stored.Iterations[1].Status)
	}
}

// A one-row dataset is the commonest parameterized run, and Xray gives its
// iteration no id of its own. The store has to keep it anyway, or that run
// reads as unparameterized.
func TestLiveSingleRowParameterizedRunSurvivesTheStore(t *testing.T) {
	const singleRow = `{"id":3278564,"testKey":"QA-1","testExecKey":"QA-EXEC-1","status":"EXECUTING",
	 "parameters":[{"name":"TIER","value":"gold"}]}`
	srv, _ := liveRunServer(t, singleRow, nil)
	defer srv.Close()

	run, err := New(jira.NewClient(srv.URL, "tok")).
		GetParameterizedRun(context.Background(), "QA-EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetParameterizedRun: %v", err)
	}
	if len(run.Iterations) != 1 {
		t.Fatalf("read %d iterations, want 1", len(run.Iterations))
	}

	repo := newRunStore(t)
	if err := repo.ReplaceParameterizedRun("p1", run); err != nil {
		t.Fatalf("the store refused the one-row live run: %v", err)
	}
	stored, found, err := repo.ParameterizedRun("p1", "QA-EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("read the stored run: %v", err)
	}
	if !found || len(stored.Iterations) != 1 {
		t.Fatalf("found=%v with %d iterations, want one stored iteration", found, len(stored.Iterations))
	}
	if len(stored.Iterations[0].Parameters) != 1 || stored.Iterations[0].Parameters[0].Value != "gold" {
		t.Errorf("stored parameters = %+v", stored.Iterations[0].Parameters)
	}
}
