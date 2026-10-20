package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGetPlanDatasetRequestsOnlyThePlanDefinition(t *testing.T) {
	var gotPath string
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("USER,REGION*\nalice,eu\n"))
	}))
	defer srv.Close()

	dataset, err := newTestClient(srv).GetPlanDataset(context.Background(), "QA PLAN/1", "QA TEST/2")
	if err != nil {
		t.Fatalf("GetPlanDataset: %v", err)
	}
	if gotPath != "/rest/raven/2.0/api/dataset/export" {
		t.Fatalf("path = %q, want the dataset export route", gotPath)
	}
	wantQuery := map[string][]string{
		"contextIssueKey": {"QA PLAN/1"},
		"resolved":        {"false"},
		"testIssueKey":    {"QA TEST/2"},
	}
	if !reflect.DeepEqual(gotQuery, wantQuery) {
		t.Fatalf("query = %#v, want %#v", gotQuery, wantQuery)
	}
	if len(dataset.Parameters) != 2 || dataset.Parameters[0].Name != "USER" || !dataset.Parameters[1].Combinatorial {
		t.Fatalf("dataset = %+v, want the Plan CSV definition", dataset)
	}
}

func TestGetPlanDatasetReturnsEmptyForAnInheritedMembership(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Dataset not found"}`))
	}))
	defer srv.Close()

	dataset, err := newTestClient(srv).GetPlanDataset(context.Background(), "QA-PLAN-1", "QA-2")
	if err != nil {
		t.Fatalf("GetPlanDataset inherited membership: %v", err)
	}
	if len(dataset.Parameters) != 0 || len(dataset.Rows) != 0 {
		t.Fatalf("dataset = %+v, want no Plan override", dataset)
	}
}

func TestGetPlanDatasetDoesNotHideAMissingRoute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<status><status-code>404</status-code></status>`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetPlanDataset(context.Background(), "QA-PLAN-1", "QA-2")
	if err == nil {
		t.Fatal("GetPlanDataset missing route = nil error, want unsupported route error")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusNotFound {
		t.Fatalf("error = %v, want *HTTPError with status 404", err)
	}
}

func TestGetPlanDatasetPadsShortCSVRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("A,B,C\n1,2\n"))
	}))
	defer srv.Close()

	dataset, err := newTestClient(srv).GetPlanDataset(context.Background(), "QA-PLAN-1", "QA-2")
	if err != nil {
		t.Fatalf("GetPlanDataset: %v", err)
	}
	want := [][]string{{"1", "2", ""}}
	if !reflect.DeepEqual(dataset.Rows, want) {
		t.Fatalf("rows = %#v, want %#v", dataset.Rows, want)
	}
}
