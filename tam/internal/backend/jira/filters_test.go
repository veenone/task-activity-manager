package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
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
		case "/rest/api/2/filter/my":
			_, _ = w.Write([]byte(`[]`))
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

// scopedFilters answers the two filter reads with whatever the case needs,
// so a test can say what the instance owns, what it has starred, and
// whether it knows about owned filters at all.
func scopedFilters(t *testing.T, ownedStatus int, owned, favourite string) *jirabackend.Backend {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/filter/my":
			if ownedStatus != http.StatusOK {
				w.WriteHeader(ownedStatus)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(owned))
		case "/rest/api/2/filter/favourite":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(favourite))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "Requirement")
}

func scopeOf(filters []backend.Filter, id string) (string, bool) {
	for _, f := range filters {
		if f.ID == id {
			return f.Scope, true
		}
	}
	return "", false
}

// The fault #346 was raised for: a filter you wrote and never starred was
// absent from the picker, because the favourites are the only list Jira
// stars for you.
func TestFiltersIncludeTheOnesYouOwnAndSayWhichTheyAre(t *testing.T) {
	b := scopedFilters(t, http.StatusOK,
		`[{"id":"10200","name":"Mine, never starred","jql":"reporter = currentUser()"}]`,
		`[{"id":"10100","name":"Team board","jql":"project = PLAT"}]`)
	filters, err := b.Filters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(filters) != 2 {
		t.Fatalf("filters = %+v, want the owned one and the starred one", filters)
	}
	if got, ok := scopeOf(filters, "10200"); !ok || got != "owned" {
		t.Errorf("scope of the owned filter = %q (found %v), want owned", got, ok)
	}
	if got, ok := scopeOf(filters, "10100"); !ok || got != "shared" {
		t.Errorf("scope of the starred filter = %q (found %v), want shared", got, ok)
	}
}

// Starring your own filter is normal, and it must not put the same filter
// in the picker twice.
func TestFiltersListAFilterYouOwnAndStarredOnce(t *testing.T) {
	b := scopedFilters(t, http.StatusOK,
		`[{"id":"10200","name":"Mine","jql":"reporter = currentUser()"}]`,
		`[{"id":"10200","name":"Mine","jql":"reporter = currentUser()"},{"id":"10100","name":"Team board","jql":"project = PLAT"}]`)
	filters, err := b.Filters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(filters) != 2 {
		t.Fatalf("filters = %+v, want each filter once", filters)
	}
	if got, _ := scopeOf(filters, "10200"); got != "owned" {
		t.Errorf("scope = %q, want owned: owning it outranks having starred it", got)
	}
}

// An instance with no /filter/my keeps the list it has always shown, and
// leaves the scope off rather than guessing it, so the view knows not to
// group what it cannot label.
func TestFiltersFallBackToTheStarredListWithoutAScope(t *testing.T) {
	b := scopedFilters(t, http.StatusNotFound, ``,
		`[{"id":"10100","name":"Team board","jql":"project = PLAT"}]`)
	filters, err := b.Filters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(filters) != 1 || filters[0].ID != "10100" {
		t.Fatalf("filters = %+v, want the starred list", filters)
	}
	if filters[0].Scope != "" {
		t.Errorf("scope = %q, want it left empty when the instance cannot say", filters[0].Scope)
	}
}

// A broken instance must not read as an old one.
func TestFiltersReportAFailedOwnedRead(t *testing.T) {
	b := scopedFilters(t, http.StatusInternalServerError, ``, `[]`)
	if _, err := b.Filters(context.Background()); err == nil {
		t.Fatal("want an error when the owned list cannot be read")
	}
}
