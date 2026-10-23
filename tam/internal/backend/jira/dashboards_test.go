package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	corejira "agile-suite/core/jira"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

// dashboardServer answers the dashboard list and nothing else. A request
// for any other path is the failure: nothing on a Jira dashboard beyond its
// name and its URL is reachable on Data Center, so nothing else is asked
// for.
func dashboardServer(t *testing.T, body string, status int) *jirabackend.Backend {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/dashboard" {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "Requirement")
}

// The view draws a name and opens a URL. Both come from Jira, because a
// URL TAM assembled itself would be a guess at how the instance addresses
// its own pages.
func TestJiraDashboardsCarryTheNameAndTheURLThatOpensThem(t *testing.T) {
	b := dashboardServer(t, `{"startAt":0,"maxResults":1000,"total":1,"dashboards":[
		{"id":"25475","name":"PR Management - IST R&D",
		 "view":"https://jira.example/secure/Dashboard.jspa?selectPageId=25475"}]}`, http.StatusOK)
	boards, err := b.JiraDashboards(context.Background())
	if err != nil {
		t.Fatalf("dashboards: %v", err)
	}
	if len(boards) != 1 {
		t.Fatalf("dashboards = %+v", boards)
	}
	if boards[0].ID != "25475" || boards[0].Name != "PR Management - IST R&D" {
		t.Errorf("dashboard = %+v", boards[0])
	}
	if boards[0].URL != "https://jira.example/secure/Dashboard.jspa?selectPageId=25475" {
		t.Errorf("url = %q, want the one Jira answered with", boards[0].URL)
	}
}

// An instance with no dashboards this user can see is a fact about the
// instance, not a failure to read it.
func TestNoJiraDashboardsIsAnEmptyListRatherThanAnError(t *testing.T) {
	b := dashboardServer(t, `{"startAt":0,"maxResults":1000,"total":0,"dashboards":[]}`, http.StatusOK)
	boards, err := b.JiraDashboards(context.Background())
	if err != nil {
		t.Fatalf("dashboards: %v", err)
	}
	if len(boards) != 0 {
		t.Errorf("dashboards = %+v, want none", boards)
	}
}

// A failed read is reported. The signpost is worth nothing if it quietly
// draws an empty list when Jira is unreachable.
func TestJiraDashboardsReportAFailedRead(t *testing.T) {
	b := dashboardServer(t, "", http.StatusInternalServerError)
	if _, err := b.JiraDashboards(context.Background()); err == nil {
		t.Fatal("want an error when the dashboard list cannot be read")
	}
}
