package jira_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"agile-suite/core/jira"
)

// A Jira dashboard is a page of gadgets, and TAM renders none of them. What
// it needs is the name a reader recognises and the URL that opens it, which
// is the whole of what /dashboard answers with.
func TestDashboardsReadTheirNameAndTheURLThatOpensThem(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"startAt":0,"maxResults":1000,"total":2,"dashboards":[
			{"id":"25475","name":"PR Management - IST R&D","self":"https://jira.example/rest/api/2/dashboard/25475",
			 "view":"https://jira.example/secure/Dashboard.jspa?selectPageId=25475"},
			{"id":"10000","name":"System Dashboard","self":"https://jira.example/rest/api/2/dashboard/10000",
			 "view":"https://jira.example/secure/Dashboard.jspa?selectPageId=10000"}
		]}`))
	}))
	defer srv.Close()

	c := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	boards, err := c.Dashboards(context.Background())
	if err != nil {
		t.Fatalf("dashboards: %v", err)
	}
	if path != "/rest/api/2/dashboard" {
		t.Errorf("path = %q", path)
	}
	if len(boards) != 2 {
		t.Fatalf("dashboards = %+v", boards)
	}
	if boards[0].ID != "25475" || boards[0].Name != "PR Management - IST R&D" {
		t.Errorf("first dashboard = %+v", boards[0])
	}
	if boards[0].View != "https://jira.example/secure/Dashboard.jspa?selectPageId=25475" {
		t.Errorf("first view = %q, want the URL that opens the dashboard", boards[0].View)
	}
}

// The envelope pages, and Data Center clamps maxResults to its own limit
// whatever is asked for. A read that stopped at the first page would hide
// every dashboard after the twentieth on an instance that has hundreds.
func TestDashboardsPageToTheEndOfTheList(t *testing.T) {
	var starts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		starts = append(starts, r.URL.Query().Get("startAt"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("startAt") {
		case "0":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":2,"total":3,"dashboards":[
				{"id":"1","name":"One","view":"https://jira.example/d/1"},
				{"id":"2","name":"Two","view":"https://jira.example/d/2"}]}`))
		case "2":
			_, _ = w.Write([]byte(`{"startAt":2,"maxResults":2,"total":3,"dashboards":[
				{"id":"3","name":"Three","view":"https://jira.example/d/3"}]}`))
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	boards, err := c.Dashboards(context.Background())
	if err != nil {
		t.Fatalf("dashboards: %v", err)
	}
	if len(boards) != 3 {
		t.Fatalf("dashboards = %+v, want all three pages", boards)
	}
	if boards[2].Name != "Three" {
		t.Errorf("last dashboard = %+v", boards[2])
	}
	if fmt.Sprint(starts) != "[0 2]" {
		t.Errorf("startAt values = %v, want one page then the next", starts)
	}
}

// An instance reporting a total it will not serve must not spin the loop
// forever. The page that comes back short is the last one there is.
func TestDashboardsStopOnAShortPageEvenWhenTheTotalPromisesMore(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":900,"dashboards":[
			{"id":"1","name":"One","view":"https://jira.example/d/1"}]}`))
	}))
	defer srv.Close()

	c := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	boards, err := c.Dashboards(context.Background())
	if err != nil {
		t.Fatalf("dashboards: %v", err)
	}
	if len(boards) != 1 || calls != 1 {
		t.Errorf("dashboards = %d after %d calls, want 1 after 1", len(boards), calls)
	}
}
