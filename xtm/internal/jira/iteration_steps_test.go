package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stepServer answers the per-iteration resource with the body given.
func stepServer(t *testing.T, asked *[]string, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked = append(*asked, r.URL.Path)
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = w.Write([]byte(body))
	}))
}

// The steps of one iteration, read from the live capture. Xray keys a step's
// fields by their display name and wraps each in {type, value:{raw, rendered}},
// which is nothing like the flat action/data/expected the demo path builds.
func TestGetIterationStepsReadsTheDisplayNameFields(t *testing.T) {
	var asked []string
	srv := stepServer(t, &asked, string(readFixture(t, "testrun_iteration_steps.json")))
	defer srv.Close()

	steps, err := newTestClient(srv).GetIterationSteps(context.Background(), "3278564", 11862)
	if err != nil {
		t.Fatalf("GetIterationSteps: %v", err)
	}
	if len(asked) != 1 || !strings.Contains(asked[0], "/rest/raven/2.0/api/testrun/3278564/iteration/11862") {
		t.Fatalf("requests = %v, want the per-iteration resource", asked)
	}
	if len(steps) != 2 {
		t.Fatalf("read %d steps, want 2", len(steps))
	}

	first := steps[0]
	if first.Rank != 1 {
		t.Errorf("first step rank = %d, want the index Xray gave it", first.Rank)
	}
	if first.Action != "step 1 - input data" {
		t.Errorf("action = %q", first.Action)
	}
	// Xray substitutes the row's value into Data before it answers. Reading
	// the raw value and leaving it alone is what keeps XTM from substituting
	// a second time over text that is already resolved.
	if first.Data != "text combi - 1" {
		t.Errorf("data = %q, want the value Xray already substituted", first.Data)
	}
	if first.Expected != "result step-1" {
		t.Errorf("expected = %q", first.Expected)
	}
	// raw, never rendered: the rendered form is HTML and the grid shows text.
	if strings.Contains(first.Action, "<p>") {
		t.Errorf("action carries the rendered HTML: %q", first.Action)
	}

	if steps[1].Data != "" {
		t.Errorf("a step with no data reads as empty, got %q", steps[1].Data)
	}
}

// A step Xray reports with no recognisable fields still takes its place in the
// list, because dropping it would renumber every step after it.
func TestGetIterationStepsKeepsAStepWithNoKnownFields(t *testing.T) {
	var asked []string
	srv := stepServer(t, &asked, `{"id":1,"testRunId":2,"status":"TO\u0044O","steps":[
		{"index":1,"fields":{"Something Else":{"type":"Wiki","value":{"raw":"x"}}}},
		{"index":2,"fields":{"Action":{"type":"Wiki","value":{"raw":"second"}}}}]}`)
	defer srv.Close()

	steps, err := newTestClient(srv).GetIterationSteps(context.Background(), "2", 1)
	if err != nil {
		t.Fatalf("GetIterationSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("read %d steps, want both", len(steps))
	}
	if steps[0].Action != "" || steps[1].Action != "second" {
		t.Errorf("steps = %+v", steps)
	}
}

// An iteration with no steps is ordinary: a test can be parameterized and have
// no step definitions at all.
func TestGetIterationStepsIsEmptyWhenXrayReportsNone(t *testing.T) {
	var asked []string
	srv := stepServer(t, &asked, `{"id":1,"testRunId":2,"status":"TO\u0044O","steps":[]}`)
	defer srv.Close()

	steps, err := newTestClient(srv).GetIterationSteps(context.Background(), "2", 1)
	if err != nil {
		t.Fatalf("GetIterationSteps: %v", err)
	}
	if len(steps) != 0 {
		t.Errorf("read %d steps, want none", len(steps))
	}
}
