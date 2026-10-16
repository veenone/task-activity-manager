package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// datasetServer answers the dataset export with body and status, and records
// the query it was asked.
func datasetServer(t *testing.T, gotQuery *string, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/raven/2.0/api/dataset/export" {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		*gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// The header row names the parameters and each later row is one iteration.
func TestGetDatasetReadsParametersAndRows(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusOK,
		"USERNAME,PASSWORD\nalice,hunter2\nbob,s3cret\n")
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	wantNames := []string{"USERNAME", "PASSWORD"}
	if got := parameterNames(ds.Parameters); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("parameters = %v, want %v", got, wantNames)
	}
	wantRows := [][]string{{"alice", "hunter2"}, {"bob", "s3cret"}}
	if !reflect.DeepEqual(ds.Rows, wantRows) {
		t.Fatalf("rows = %v, want %v", ds.Rows, wantRows)
	}
	if q != "testIssueKey=QA-1" {
		t.Fatalf("export query %q, want testIssueKey=QA-1", q)
	}
}

// Xray marks a combinatorial parameter with an asterisk suffix on the column
// name. The flag is ours to keep; the asterisk is not part of the name.
func TestGetDatasetStripsTheCombinatorialMarker(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusOK, "BROWSER,QUALITY*\nchrome,high\n")
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(ds.Parameters) != 2 {
		t.Fatalf("parameters = %v, want 2", ds.Parameters)
	}
	if ds.Parameters[0].Combinatorial {
		t.Errorf("BROWSER must not be combinatorial")
	}
	if got := ds.Parameters[1]; got.Name != "QUALITY" || !got.Combinatorial {
		t.Errorf("parameter 1 = %+v, want name QUALITY and combinatorial", got)
	}
}

// A quoted cell may hold the delimiter. encoding/csv handles this; the test
// pins it so a hand-rolled splitter can never replace the parser.
func TestGetDatasetKeepsACommaInsideAQuotedCell(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusOK, "CITY,NOTE\n\"Paris, France\",capital\n")
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	want := [][]string{{"Paris, France", "capital"}}
	if !reflect.DeepEqual(ds.Rows, want) {
		t.Fatalf("rows = %v, want %v", ds.Rows, want)
	}
}

// An instance below Xray 5.1 has no dataset endpoint. The feature is not
// version-gated, so a 404 must read as "this test has no dataset" rather than
// surfacing an error the user cannot act on.
func TestGetDatasetTreatsAMissingEndpointAsNoDataset(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusNotFound, "")
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("a 404 must not be an error, got %v", err)
	}
	if len(ds.Parameters) != 0 || len(ds.Rows) != 0 {
		t.Fatalf("dataset = %+v, want empty", ds)
	}
}

// A test with no dataset exports a header and nothing else.
func TestGetDatasetHandlesAHeaderWithNoRows(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusOK, "USERNAME\n")
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(ds.Parameters) != 1 || len(ds.Rows) != 0 {
		t.Fatalf("dataset = %+v, want one parameter and no rows", ds)
	}
}

// An empty body is not a parse failure. Nothing to show is a valid answer.
func TestGetDatasetHandlesAnEmptyBody(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusOK, "")
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(ds.Parameters) != 0 {
		t.Fatalf("parameters = %v, want none", ds.Parameters)
	}
}

// A row with fewer cells than the header is padded rather than dropped, so a
// short row still lines up under its parameters.
func TestGetDatasetPadsAShortRow(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusOK, "A,B,C\n1,2\n")
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	want := [][]string{{"1", "2", ""}}
	if !reflect.DeepEqual(ds.Rows, want) {
		t.Fatalf("rows = %v, want %v", ds.Rows, want)
	}
}

// A server error is a real failure and must reach the caller, unlike a 404.
func TestGetDatasetPropagatesAServerError(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusInternalServerError, "")
	defer srv.Close()

	if _, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", ""); err == nil {
		t.Fatal("a 500 must be an error")
	}
}

// A Test Plan or Test Execution dataset overrides the test's own. Phase 1 only
// reads the test level, but the context has to reach the query when it is set.
func TestGetDatasetSendsTheContextKeyWhenGiven(t *testing.T) {
	var q string
	srv := datasetServer(t, &q, http.StatusOK, "A\n1\n")
	defer srv.Close()

	if _, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "QA-99"); err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if q != "contextIssueKey=QA-99&testIssueKey=QA-1" {
		t.Fatalf("export query %q", q)
	}
}

// parameterNames is a test helper: the parameter names in order.
func parameterNames(ps []DatasetParameter) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}
