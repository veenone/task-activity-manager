package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

// newFilterServer answers the field discovery, the favourites and one
// page of a search, and records what the search was asked for.
func newFilterServer(t *testing.T, searchBody string) (*jirabackend.Backend, *[]string) {
	t.Helper()
	var searches []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/field":
			_, _ = w.Write([]byte(historyFields))
		case "/rest/api/2/filter/favourite":
			_, _ = w.Write([]byte(`[{"id":"10100","name":"My open bugs","jql":"type = Bug AND resolution = Unresolved"}]`))
		case "/rest/api/2/search":
			searches = append(searches, r.URL.RawQuery)
			_, _ = w.Write([]byte(searchBody))
		case "/rest/api/2/project/PLAT", "/rest/api/2/project/OPS":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"1","name":"Task"},{"id":"18","name":"Story"}]}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	return jirabackend.New(c, "Requirement"), &searches
}

func TestFiltersReadTheSavedSearchesAndTheirJQL(t *testing.T) {
	b, _ := newFilterServer(t, `{"total":0,"issues":[]}`)
	filters, err := b.Filters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(filters) != 1 || filters[0].Name != "My open bugs" || filters[0].ID != "10100" {
		t.Fatalf("filters = %+v", filters)
	}
	if filters[0].JQL != "type = Bug AND resolution = Unresolved" {
		t.Errorf("jql = %q, want the filter's own words", filters[0].JQL)
	}
}

// A filter is not bound to the profile's project, so the search runs the
// JQL as it stands: no project clause is added to it, and rows from
// another project come back as rows.
func TestSearchByJQLRunsTheFilterUnchangedAcrossProjects(t *testing.T) {
	b, searches := newFilterServer(t, `{"total":2,"issues":[
		{"id":"1","key":"PLAT-1","fields":{"summary":"One","status":{"name":"Done","statusCategory":{"key":"done"}},"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[],"timeoriginalestimate":28800}},
		{"id":"2","key":"OPS-9","fields":{"summary":"Two","status":{"name":"To Do","statusCategory":{"key":"new"}},"issuetype":{"name":"Task"},"project":{"key":"OPS"},"labels":[]}}
	]}`)
	page, total, err := b.SearchByJQL(context.Background(), "project in (PLAT, OPS)", 0, 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 2 || len(page) != 2 {
		t.Fatalf("total %d rows %d", total, len(page))
	}
	if !strings.Contains((*searches)[0], "project+in+%28PLAT%2C+OPS%29") {
		t.Errorf("search query = %q, want the filter's jql unchanged", (*searches)[0])
	}
	if page[1].Key != "OPS-9" || page[1].Project != "OPS" {
		t.Errorf("second row = %+v, want the row from the other project", page[1])
	}
	// The rows carry what the panels count, the time fields among them.
	if page[0].OriginalEstimateSeconds == nil || *page[0].OriginalEstimateSeconds != 28800 {
		t.Errorf("first row estimate = %v", page[0].OriginalEstimateSeconds)
	}
	if page[0].StatusCategory != "done" {
		t.Errorf("first row category = %q", page[0].StatusCategory)
	}
}
