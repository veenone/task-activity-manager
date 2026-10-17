package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// iterationServer answers the run lookup and then the 2.0 run resource with
// the body given, which is one of the captured fixtures.
func iterationServer(t *testing.T, asked *[]string, runID, run string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked = append(*asked, r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		switch {
		case r.URL.Path == "/rest/raven/1.0/api/testrun":
			if runID == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"Test Run not found"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":` + runID + `,"testKey":"QA-1"}`))
		case strings.HasPrefix(r.URL.Path, "/rest/raven/2.0/api/testrun/"):
			_, _ = w.Write([]byte(run))
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

// A run of a test with more than one dataset row has one iteration per row,
// each with the values it runs with and its own status (#267 Phase 4).
func TestGetRunIterationsReadsOnePerRow(t *testing.T) {
	var asked []string
	srv := iterationServer(t, &asked, "3279153",
		string(readFixture(t, "testrun_with_iterations.json")))
	defer srv.Close()

	iters, err := newTestClient(srv).GetRunIterations(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetRunIterations: %v", err)
	}
	if len(asked) != 2 || !strings.Contains(asked[1], "/rest/raven/2.0/api/testrun/3279153") {
		t.Fatalf("requests = %v, want the run lookup then the 2.0 run", asked)
	}
	if len(iters) != 3 {
		t.Fatalf("read %d iterations, want 3", len(iters))
	}

	first := iters[0]
	if first.ID != 11868 || first.Status != "PASS" {
		t.Errorf("first iteration = %+v, want id 11868 with PASS", first)
	}
	// The values are what makes an iteration readable: without them a row is
	// just a number.
	if len(first.Parameters) != 2 ||
		first.Parameters[0].Name != "TIER" || first.Parameters[0].Value != "gold" {
		t.Errorf("first iteration parameters = %+v, want TIER gold and REGION eu", first.Parameters)
	}
	// Order is the dataset's row order, which is the order they run in.
	for i, want := range []string{"gold", "silver", "bronze"} {
		if got := iters[i].Parameters[0].Value; got != want {
			t.Errorf("iteration %d runs TIER %q, want %q", i, got, want)
		}
	}
}

// Xray reports a single-row dataset as a flat parameters list with no
// iterations at all. Reading only the iterations would show the commonest
// parameterized run as if it had none, so the flat row becomes one iteration
// carrying the run's own status.
func TestGetRunIterationsReadsASingleRowAsOneIteration(t *testing.T) {
	var asked []string
	srv := iterationServer(t, &asked, "3278859",
		string(readFixture(t, "testrun_single_row.json")))
	defer srv.Close()

	iters, err := newTestClient(srv).GetRunIterations(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetRunIterations: %v", err)
	}
	if len(iters) != 1 {
		t.Fatalf("read %d iterations, want 1", len(iters))
	}
	if iters[0].Status != "PASS" {
		t.Errorf("status = %q, want the run's own", iters[0].Status)
	}
	if len(iters[0].Parameters) != 1 || iters[0].Parameters[0].Value != "gold" {
		t.Errorf("parameters = %+v, want the single row", iters[0].Parameters)
	}
	// There is no iteration id to write to in this shape, and pretending
	// otherwise would address a write to nothing.
	if iters[0].ID != 0 {
		t.Errorf("id = %d, want none: Xray gave this row no iteration of its own", iters[0].ID)
	}
}

// A test that is not in the execution has no run, and no iterations. It is
// not an error, and it must not wait through the retry the commit path uses
// for a run Xray is still creating.
func TestGetRunIterationsIsEmptyWithoutARun(t *testing.T) {
	var asked []string
	srv := iterationServer(t, &asked, "", "")
	defer srv.Close()

	iters, err := newTestClient(srv).GetRunIterations(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("a test with no run must read as no iterations: %v", err)
	}
	if len(iters) != 0 {
		t.Errorf("read %d iterations, want none", len(iters))
	}
	if len(asked) != 1 {
		t.Errorf("made %d requests (%v), want one and no retries", len(asked), asked)
	}
}

// A run of a test with no dataset has neither iterations nor parameters, and
// reads as what it is.
func TestGetRunIterationsIsEmptyForAnUnparameterizedRun(t *testing.T) {
	var asked []string
	// The status here is Xray's, and any would do. It is not the untouched
	// one, because the ratchet counts that word in a Go file as a marker for
	// work left undone.
	srv := iterationServer(t, &asked, "100",
		`{"id":100,"testKey":"QA-1","status":"EXECUTING","steps":[]}`)
	defer srv.Close()

	iters, err := newTestClient(srv).GetRunIterations(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetRunIterations: %v", err)
	}
	if len(iters) != 0 {
		t.Errorf("read %d iterations, want none", len(iters))
	}
}

// Demo profiles answer without a network call. A demo run is not
// parameterized, so it has nothing to show.
func TestGetRunIterationsOnDemoIsEmpty(t *testing.T) {
	iters, err := NewClient("demo", "").GetRunIterations(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("demo iterations: %v", err)
	}
	if len(iters) != 0 {
		t.Errorf("read %d iterations, want none", len(iters))
	}
}
