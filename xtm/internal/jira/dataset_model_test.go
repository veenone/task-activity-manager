package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// modelServer answers the two requests a dataset read makes: the issue GET
// that resolves the key, and the JSON model itself.
func modelServer(t *testing.T, asked *[]string, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked = append(*asked, r.URL.Path+"?"+r.URL.RawQuery)
		if strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"2474230","key":"QA-1"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// The CSV export cannot say that a parameter is a list, which is how #283
// let an edit turn one into free text. The JSON model says so, and carries
// the parameter id a write needs to keep.
func TestGetDatasetReadsTheJSONModel(t *testing.T) {
	var asked []string
	srv := modelServer(t, &asked, http.StatusOK, string(readFixture(t, "dataset_model_test_level.json")))
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("GetDataset: %v", err)
	}
	if len(ds.Parameters) != 3 {
		t.Fatalf("read %d parameters, want 3", len(ds.Parameters))
	}

	// The model is addressed by numeric id, so the key is resolved first.
	if len(asked) != 2 || !strings.Contains(asked[1], "/rest/raven/1.0/dataset?entityId=2474230&type=TEST") {
		t.Errorf("requests = %v, want the issue lookup then the model", asked)
	}

	text, list, combi := ds.Parameters[0], ds.Parameters[1], ds.Parameters[2]
	if text.ID != 2726 || text.Name != "dataset sample - 1" {
		t.Errorf("first parameter = %+v, want id 2726", text)
	}
	if text.Type != "TEXT" || len(text.Options) != 0 {
		t.Errorf("first parameter type = %q options = %v, want TEXT with none", text.Type, text.Options)
	}
	// Everything a write has to hand back for the list to stay a list.
	if list.Type != "LIST" || list.ListID != 158 || list.ListScope != "PARAMETER_AD_HOC" {
		t.Errorf("list parameter = %+v, want its list binding", list)
	}
	if !reflect.DeepEqual(list.Options, []string{"list option 1", "list option 2", "list option 3"}) {
		t.Errorf("list options = %v, want the three allowed values", list.Options)
	}
	if !combi.Combinatorial || list.Combinatorial {
		t.Errorf("combinatorial flags = %v / %v, want only the third", list.Combinatorial, combi.Combinatorial)
	}

	// Values come back by column and have to be read out by row.
	want := [][]string{
		{"text combi - 1", "list option 1", "list 2 option 1"},
		{"text combi -2", "list option 2", "list 2 option 2"},
		{"", "", "list 3 option 3"},
	}
	if !reflect.DeepEqual(ds.Rows, want) {
		t.Errorf("rows = %q, want %q", ds.Rows, want)
	}
}

// A test with no dataset answers 200 with an empty params array on this
// route, so there is no 404 to interpret.
func TestGetDatasetReadsAnEmptyModelAsNoDataset(t *testing.T) {
	var asked []string
	srv := modelServer(t, &asked, http.StatusOK,
		`{"entityDataset":{"params":[]},"resolvedDataset":{"params":[]},`+
			`"resolutionMessage":"This is the resolved dataset, defined in the Test."}`)
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-2", "")
	if err != nil {
		t.Fatalf("GetDataset: %v", err)
	}
	if len(ds.Parameters) != 0 || len(ds.Rows) != 0 {
		t.Errorf("dataset = %+v, want an empty one", ds)
	}
}

// An instance below Xray 5.1 publishes neither dataset route. The panel shows
// nothing there, as it does for a test with no dataset.
func TestGetDatasetTreatsAMissingRouteAsNoDataset(t *testing.T) {
	var asked []string
	srv := modelServer(t, &asked, http.StatusNotFound,
		`<status><status-code>404</status-code></status>`)
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("a missing dataset route must read as no dataset: %v", err)
	}
	if len(ds.Parameters) != 0 {
		t.Errorf("dataset = %+v, want an empty one", ds)
	}
}

// A container-level read still goes through the 2.0 export: the JSON model
// addresses a run by its run id, which nothing above resolves yet (#277 W3).
func TestGetDatasetStillExportsForAContextKey(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("USERNAME\nalice\n"))
	}))
	defer srv.Close()

	ds, err := newTestClient(srv).GetDataset(context.Background(), "QA-1", "EXEC-1")
	if err != nil {
		t.Fatalf("GetDataset: %v", err)
	}
	if len(asked) != 1 || asked[0] != "/rest/raven/2.0/api/dataset/export" {
		t.Errorf("requests = %v, want the export alone", asked)
	}
	if len(ds.Parameters) != 1 || ds.Parameters[0].Name != "USERNAME" {
		t.Errorf("dataset = %+v, want the exported table", ds)
	}
}

// The write has to hand back what the read found, or Xray rebuilds the
// parameter: a new id, and a list rebuilt as free text (#283).
func TestSetDatasetReturnsTheParameterIdAndListBinding(t *testing.T) {
	var got datasetWrite
	srv := datasetWriteServer(t, &got, http.StatusOK, "Dataset Updated with success.")
	defer srv.Close()

	err := newTestClient(srv).SetDataset(context.Background(), "QA-1", Dataset{
		Parameters: []DatasetParameter{
			{
				ID: 2727, Name: "TIER", Type: "LIST",
				Options: []string{"gold", "silver"}, ListScope: "PARAMETER_AD_HOC", ListID: 158,
			},
			{Name: "REGION"},
		},
		Rows: [][]string{{"gold", "eu-west"}},
	})
	if err != nil {
		t.Fatalf("set dataset: %v", err)
	}

	body := string(got.body)
	for _, want := range []string{
		`"id":2727`,
		`"type":"LIST"`,
		`"options":["gold","silver"]`,
		`"paramTestListScope":"PARAMETER_AD_HOC"`,
		`"listId":158`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s is missing %s", body, want)
		}
	}
	// The parameter with no id is still announced as new, and carries no list
	// binding of its own.
	if !strings.Contains(body, `"id":-1`) {
		t.Errorf("body %s does not announce the new parameter", body)
	}
	if strings.Count(body, `"listId"`) != 1 {
		t.Errorf("body %s gave the new parameter a list binding", body)
	}
}
