package jira

import (
	"encoding/json"
	"testing"
)

// The fixtures these read are raw responses captured from a live Xray
// Server/DC instance while answering what #267 Phase 0 left open: whether a
// run exposes its iterations, and whether one can be written. No product code
// reads them yet; they are here so Phase 4 is built against bytes the server
// sent rather than against a reading of the documentation, which describes
// neither shape.

// run is the shape a reader needs out of the 2.0 test run resource.
type runShape struct {
	ID         int             `json:"id"`
	Status     string          `json:"status"`
	Parameters []iterationParm `json:"parameters"`
	Iterations []iterationShape
}

type iterationShape struct {
	ID         int             `json:"id"`
	Status     string          `json:"status"`
	Parameters []iterationParm `json:"parameters"`
}

type iterationParm struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// A run of a test with more than one dataset row carries one iteration per
// row, each with its own id, its own status and the values it runs with.
// Nothing in the 1.0 resource says any of this, and no /iteration collection
// exists: the run GET is where they live.
func TestTestRunCarriesItsIterations(t *testing.T) {
	var run runShape
	if err := json.Unmarshal(readFixture(t, "testrun_with_iterations.json"), &run); err != nil {
		t.Fatal(err)
	}

	if len(run.Iterations) != 3 {
		t.Fatalf("run has %d iterations, want 3", len(run.Iterations))
	}
	first := run.Iterations[0]
	if first.ID == 0 {
		t.Error("an iteration with no id cannot be written to")
	}
	if first.Status != "PASS" {
		t.Errorf("first iteration status = %q, want PASS", first.Status)
	}
	if len(first.Parameters) != 2 || first.Parameters[0].Name != "TIER" {
		t.Errorf("first iteration parameters = %+v, want the row it runs", first.Parameters)
	}
	// The run's own status is derived from its iterations: one passed of
	// three leaves the run executing, not passed.
	if run.Status != "EXECUTING" {
		t.Errorf("run status = %q, want EXECUTING while its iterations differ", run.Status)
	}
	// Iteration order is the row order, which is what an index can rely on.
	want := []string{"gold", "silver", "bronze"}
	for i, w := range want {
		if got := run.Iterations[i].Parameters[0].Value; got != w {
			t.Errorf("iteration %d runs TIER %q, want %q", i, got, w)
		}
	}
}

// A dataset of one row is not reported as one iteration. The run carries a
// flat parameters list and no iterations at all, so a reader that only looks
// for iterations shows a parameterized run as if it had none.
func TestTestRunOfASingleRowHasNoIterations(t *testing.T) {
	var run runShape
	if err := json.Unmarshal(readFixture(t, "testrun_single_row.json"), &run); err != nil {
		t.Fatal(err)
	}

	if len(run.Iterations) != 0 {
		t.Errorf("single-row run has %d iterations, want none", len(run.Iterations))
	}
	if len(run.Parameters) != 1 || run.Parameters[0].Value != "gold" {
		t.Errorf("parameters = %+v, want the single row the run holds", run.Parameters)
	}
}

// One iteration can be read on its own, which is where its step results would
// come from, and the id in it is the one a write is addressed to.
func TestOneIterationReadsOnItsOwn(t *testing.T) {
	var iter struct {
		iterationShape
		TestRunID int `json:"testRunId"`
	}
	if err := json.Unmarshal(readFixture(t, "testrun_iteration.json"), &iter); err != nil {
		t.Fatal(err)
	}
	if iter.ID != 11868 || iter.TestRunID != 3279153 {
		t.Errorf("iteration = %+v, want it to name itself and its run", iter)
	}
}

// Writing one is a PUT to that iteration with a status, and the answer names
// the iteration rather than repeating it. A client must not expect the
// updated iteration back.
func TestWritingAnIterationAnswersWithItsIDAlone(t *testing.T) {
	var written struct {
		ID          int   `json:"id"`
		StepResults []any `json:"stepResults"`
	}
	if err := json.Unmarshal(readFixture(t, "testrun_iteration_written.json"), &written); err != nil {
		t.Fatal(err)
	}
	if written.ID != 11868 {
		t.Errorf("write answered with id %d, want the iteration's own", written.ID)
	}
	if len(written.StepResults) != 0 {
		t.Errorf("step results = %v, want none for a status-only write", written.StepResults)
	}
}
