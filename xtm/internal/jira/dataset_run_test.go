package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// runModelServer answers the run lookup and the model for that run. runID
// empty means the test has no run in that execution, which Xray reports as a
// 400.
func runModelServer(t *testing.T, asked *[]string, runID, model string) *httptest.Server {
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
		case r.URL.Path == "/rest/raven/1.0/dataset":
			_, _ = w.Write([]byte(model))
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

// An execution can define its own dataset for a test, and that is what runs
// there. The model answers it under the run's id (#277 W3).
func TestGetRunDatasetReadsWhatTheExecutionDefines(t *testing.T) {
	var asked []string
	srv := runModelServer(t, &asked, "3278859",
		`{"entityDataset":{"params":[{"param":{"id":2745,"name":"TIER","rank":0,`+
			`"combinatorial":false,"info":{"type":"TEXT","options":[]}},`+
			`"values":[{"value":"run-only","rank":0}]}]},`+
			`"resolvedDataset":{"params":[{"param":{"id":2744,"name":"TIER","rank":0,`+
			`"combinatorial":false,"info":{"type":"TEXT","options":[]}},`+
			`"values":[{"value":"from-the-test","rank":0}]}]},`+
			`"resolutionMessage":"This is the resolved dataset, defined in the Test."}`)
	defer srv.Close()

	ds, err := newTestClient(srv).GetRunDataset(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetRunDataset: %v", err)
	}
	if len(asked) != 2 || !strings.Contains(asked[1], "entityId=3278859&type=TEST_RUN") {
		t.Fatalf("requests = %v, want the run lookup then the model for that run", asked)
	}
	// entityDataset is what this level defines. resolvedDataset is what it
	// would inherit, and reading that instead would show the test's table
	// while claiming the execution overrides it.
	if len(ds.Parameters) != 1 || len(ds.Rows) != 1 || ds.Rows[0][0] != "run-only" {
		t.Errorf("dataset = %+v, want the table the execution defines", ds)
	}
}

// Nearly every run defines nothing, and that is not an error: the panel then
// shows the test's own dataset and says so.
func TestGetRunDatasetIsEmptyWhenTheExecutionDefinesNothing(t *testing.T) {
	var asked []string
	srv := runModelServer(t, &asked, "3278859",
		`{"entityDataset":{"params":[]},"resolvedDataset":{"params":[{"param":{"id":2744,`+
			`"name":"TIER","rank":0,"combinatorial":false,"info":{"type":"TEXT","options":[]}},`+
			`"values":[{"value":"from-the-test","rank":0}]}]},`+
			`"resolutionMessage":"This is the resolved dataset, defined in the Test."}`)
	defer srv.Close()

	ds, err := newTestClient(srv).GetRunDataset(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("GetRunDataset: %v", err)
	}
	if len(ds.Parameters) != 0 {
		t.Errorf("dataset = %+v, want nothing defined at the run", ds)
	}
}

// A test that is not in the execution has no run, which Xray reports as a
// 400. Nothing is overridden, so nothing is the answer; the lookup must not
// sit through the retry the commit path uses for a run Xray is still
// creating.
func TestGetRunDatasetIsEmptyWhenThereIsNoRun(t *testing.T) {
	var asked []string
	srv := runModelServer(t, &asked, "", "")
	defer srv.Close()

	ds, err := newTestClient(srv).GetRunDataset(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("a test with no run must read as no override: %v", err)
	}
	if len(ds.Parameters) != 0 {
		t.Errorf("dataset = %+v, want an empty one", ds)
	}
	if len(asked) != 1 {
		t.Errorf("made %d requests (%v), want one and no retries", len(asked), asked)
	}
}

// Demo profiles answer without a network call, and no demo execution defines
// a dataset of its own.
func TestGetRunDatasetOnDemoDefinesNothing(t *testing.T) {
	ds, err := NewClient("demo", "").GetRunDataset(context.Background(), "EXEC-1", "QA-1")
	if err != nil {
		t.Fatalf("demo run dataset: %v", err)
	}
	if len(ds.Parameters) != 0 {
		t.Errorf("dataset = %+v, want an empty one", ds)
	}
}
