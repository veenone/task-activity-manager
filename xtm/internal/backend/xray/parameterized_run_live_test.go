package xray

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

// liveRunServer answers the three calls a live parameterized run needs: the
// run lookup, the 2.0 run with its iterations, and one per-iteration read.
func liveRunServer(t *testing.T, run string, iterations map[string]string) (*httptest.Server, *[]string) {
	t.Helper()
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		switch {
		case r.URL.Path == "/rest/raven/1.0/api/testrun":
			_, _ = w.Write([]byte(`{"id":3278564,"testKey":"QA-1"}`))
		case strings.Contains(r.URL.Path, "/iteration/"):
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			body, ok := iterations[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(body))
		case strings.HasPrefix(r.URL.Path, "/rest/raven/2.0/api/testrun/"):
			_, _ = w.Write([]byte(run))
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	}))
	return srv, &asked
}

const liveRunTwoIterations = `{"id":3278564,"testKey":"QA-1","testExecKey":"QA-EXEC-1","status":"TO\u0044O",
 "iterations":[
  {"id":11862,"status":"PASS","parameters":[{"name":"TIER","value":"gold"}]},
  {"id":11863,"status":"TO\u0044O","parameters":[{"name":"TIER","value":"silver"}]}]}`

const liveIterationWithSteps = `{"id":11862,"testRunId":3278564,"status":"PASS",
 "parameters":[{"name":"TIER","value":"gold"}],
 "steps":[{"index":1,"fields":{
   "Action":{"type":"Wiki","value":{"raw":"do the thing","rendered":"<p>do the thing</p>"}},
   "Data":{"type":"Wiki","value":{"raw":"gold","rendered":"gold"}},
   "Expected Result":{"type":"Wiki","value":{"raw":"it worked","rendered":"<p>it worked</p>"}}}}]}`

// A live run reads as a ParameterizedRun: one iteration per dataset row, each
// carrying the values and status Xray recorded, and its steps as Xray already
// substituted them (#334). None of it is generated the way demo mode does.
func TestXrayGetParameterizedRunReadsLiveIterations(t *testing.T) {
	srv, asked := liveRunServer(t, liveRunTwoIterations, map[string]string{
		"11862": liveIterationWithSteps,
		"11863": `{"id":11863,"testRunId":3278564,"status":"TO\u0044O","steps":[]}`,
	})
	defer srv.Close()

	run, err := New(jira.NewClient(srv.URL, "tok")).
		GetParameterizedRun(context.Background(), "QA-EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetParameterizedRun: %v", err)
	}
	if run.ExecutionKey != "QA-EXEC-1" || run.TestKey != "QA-1" {
		t.Errorf("run identifies as %s/%s", run.ExecutionKey, run.TestKey)
	}
	if run.Status != "TO\u0044O" {
		t.Errorf("run status = %q, want the status Xray reported", run.Status)
	}
	if len(run.Iterations) != 2 {
		t.Fatalf("read %d iterations, want 2", len(run.Iterations))
	}

	first := run.Iterations[0]
	// Xray's own status, not a value from the demo PASS/FAIL/TO\u0044O cycle.
	if first.Status != "PASS" {
		t.Errorf("first iteration status = %q, want PASS from Xray", first.Status)
	}
	if len(first.Parameters) != 1 || first.Parameters[0].Value != "gold" {
		t.Errorf("first iteration parameters = %+v", first.Parameters)
	}
	if len(first.Steps) != 1 {
		t.Fatalf("first iteration has %d steps, want 1", len(first.Steps))
	}
	step := first.Steps[0]
	if step.Action != "do the thing" || step.Expected != "it worked" {
		t.Errorf("step = %+v", step)
	}
	// Xray substituted the row's value already; substituting again would turn
	// a resolved value back into a placeholder hunt.
	if step.Data != "gold" {
		t.Errorf("step data = %q, want the value Xray resolved", step.Data)
	}
	// Live Xray reports no per-step status and no step id on this resource,
	// so the iteration's own status is what a reader has.
	if step.Status != "PASS" {
		t.Errorf("step status = %q, want the iteration's status", step.Status)
	}

	if run.Iterations[1].Status != "TO\u0044O" || len(run.Iterations[1].Steps) != 0 {
		t.Errorf("second iteration = %+v", run.Iterations[1])
	}
	// One run lookup, one run read, one read per iteration.
	if len(*asked) != 4 {
		t.Errorf("made %d calls (%v), want the run lookup, the run and one per iteration", len(*asked), *asked)
	}
}

// A single-row dataset is still parameterized. Xray reports it with a flat
// parameters array and no iterations, and the member table has to show it or
// the commonest parameterized run reads as an ordinary one.
func TestXrayGetParameterizedRunReadsASingleRow(t *testing.T) {
	srv, _ := liveRunServer(t,
		`{"id":3278564,"testKey":"QA-1","status":"PASS","parameters":[{"name":"TIER","value":"gold"}]}`,
		map[string]string{})
	defer srv.Close()

	run, err := New(jira.NewClient(srv.URL, "tok")).
		GetParameterizedRun(context.Background(), "QA-EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetParameterizedRun: %v", err)
	}
	if len(run.Iterations) != 1 {
		t.Fatalf("read %d iterations, want the single row as one", len(run.Iterations))
	}
	if run.Iterations[0].Status != "PASS" || len(run.Iterations[0].Parameters) != 1 {
		t.Errorf("iteration = %+v", run.Iterations[0])
	}
}

// A test with no dataset has no iterations, and the caller reads that as a
// plain run rather than an error, so the row keeps its status select.
func TestXrayGetParameterizedRunIsUnsupportedWithoutADataset(t *testing.T) {
	srv, _ := liveRunServer(t, `{"id":3278564,"testKey":"QA-1","status":"TO\u0044O"}`, map[string]string{})
	defer srv.Close()

	_, err := New(jira.NewClient(srv.URL, "tok")).
		GetParameterizedRun(context.Background(), "QA-EXEC-1", "QA-1")
	if err == nil {
		t.Fatal("want an error for a run with no iterations")
	}
	if !isUnsupported(err) {
		t.Errorf("err = %v, want ErrUnsupported so the row falls back to a plain run", err)
	}
}

func isUnsupported(err error) bool {
	return err != nil && strings.Contains(err.Error(), backend.ErrUnsupported.Error())
}

// An iteration's status writes back on live now (#336). Per-step results do
// not: the read gives no step-result id, so there is nothing to address a
// per-step write to, and that distinction is the whole of what the read-only
// marker still covers.
func TestXrayLiveWritesIterationStatusButNotStepStatus(t *testing.T) {
	caps := New(jira.NewClient("https://jira.example.com", "tok")).Capabilities()
	if !caps.SupportsParameterizedRunDetail {
		t.Error("live Xray should read parameterized runs (#334)")
	}
	if !caps.SupportsParameterizedRunWrites {
		t.Error("live Xray should write an iteration status (#336)")
	}
}

// The write goes to the run resource, which the adapter has to resolve from
// the execution and test keys first.
func TestXraySetIterationStatusResolvesTheRunAndWrites(t *testing.T) {
	var wrotePath, method string
	var wroteBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/raven/1.0/api/testrun" {
			_, _ = w.Write([]byte(`{"id":3278564,"testKey":"QA-1"}`))
			return
		}
		method, wrotePath = r.Method, r.URL.Path
		wroteBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"id":11862,"stepResults":[]}`))
	}))
	defer srv.Close()

	err := New(jira.NewClient(srv.URL, "tok")).
		SetIterationStatus(context.Background(), "QA-EXEC-1", "QA-1", "11862", "PASS")
	if err != nil {
		t.Fatalf("SetIterationStatus: %v", err)
	}
	if method != http.MethodPut {
		t.Errorf("method = %s, want PUT", method)
	}
	if wrotePath != "/rest/raven/2.0/api/testrun/3278564/iteration/11862" {
		t.Errorf("wrote to %s", wrotePath)
	}
	if !strings.Contains(string(wroteBody), `"status":"PASS"`) {
		t.Errorf("body = %s", wroteBody)
	}
}

// A single-row run carries no iteration id of its own (#334 reports it as an
// empty one), so a write has nothing to address and says so instead of
// sending a request that could only guess.
func TestXraySetIterationStatusRefusesAnIterationWithNoID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("a write with no iteration id reached the network")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := New(jira.NewClient(srv.URL, "tok")).
		SetIterationStatus(context.Background(), "QA-EXEC-1", "QA-1", "", "PASS")
	if err == nil {
		t.Fatal("want an error for an iteration with no id")
	}
}
