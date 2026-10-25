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
		case "/rest/api/2/myself":
			_, _ = w.Write([]byte(`{"name":"reader"}`))
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

// filterServerAs answers the favourites and the connected user, so a test
// can say who is reading and who wrote each filter. It asks for nothing
// else: a request for any other path is the failure, /filter/my among them,
// because that route is what #348 removed.
func filterServerAs(t *testing.T, me string, meStatus int, favourite string) *jirabackend.Backend {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/myself":
			if meStatus != http.StatusOK {
				w.WriteHeader(meStatus)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"` + me + `"}`))
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

// The scope is who wrote the filter, read per filter, because Data Center
// has no route that answers it for a whole list. The literal strings are
// asserted on purpose: they are the wire values the picker groups on.
func TestFiltersSayWhichOnesThisUserWrote(t *testing.T) {
	b := filterServerAs(t, "reader", http.StatusOK, `[
		{"id":"10200","name":"Mine","jql":"reporter = currentUser()","owner":{"name":"reader"}},
		{"id":"10100","name":"Theirs","jql":"project = PLAT","owner":{"name":"somebody"}}
	]`)
	filters, err := b.Filters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(filters) != 2 {
		t.Fatalf("filters = %+v", filters)
	}
	if got, ok := scopeOf(filters, "10200"); !ok || got != "owned" {
		t.Errorf("scope of the reader's own filter = %q (found %v), want owned", got, ok)
	}
	if got, ok := scopeOf(filters, "10100"); !ok || got != "shared" {
		t.Errorf("scope of somebody else's filter = %q (found %v), want shared", got, ok)
	}
}

// A reader who cannot be named costs the labels, not the list. The filters
// read fine, so dropping them over a second read would lose more than it
// protects, and an unlabelled list is what this view showed before #346.
func TestFiltersKeepTheListWhenTheReaderCannotBeNamed(t *testing.T) {
	b := filterServerAs(t, "", http.StatusInternalServerError,
		`[{"id":"10100","name":"Team board","jql":"project = PLAT","owner":{"name":"somebody"}}]`)
	filters, err := b.Filters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(filters) != 1 || filters[0].ID != "10100" {
		t.Fatalf("filters = %+v, want the starred list", filters)
	}
	if filters[0].Scope != "" {
		t.Errorf("scope = %q, want it left off when the reader is unknown", filters[0].Scope)
	}
}

// An instance that names its users another way, Cloud by account id among
// them, leaves the owner empty. Two empty names are not a match, or every
// filter on such an instance would be labelled as this reader's own.
func TestFiltersLeaveTheScopeOffAFilterWithNoNamedOwner(t *testing.T) {
	b := filterServerAs(t, "reader", http.StatusOK,
		`[{"id":"10100","name":"Named another way","jql":"project = PLAT","owner":{"accountId":"5b10a2"}}]`)
	filters, err := b.Filters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if filters[0].Scope != "" {
		t.Errorf("scope = %q, want it left off when the owner has no username", filters[0].Scope)
	}
}

// Losing the filters themselves is still a failure.
func TestFiltersReportAFailedFavouriteRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	b := jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "Requirement")
	if _, err := b.Filters(context.Background()); err == nil {
		t.Fatal("want an error when the starred list cannot be read")
	}
}

// A filter reached by id is one the picker could not offer: Data Center
// enumerates the starred list and nothing else, so a filter nobody starred
// is in no list at all.
func TestFilterByIDReadsTheNameAndJQLBehindAnID(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if r.URL.Path != "/rest/api/2/filter/25475" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"25475","name":"Release blockers","jql":"project = PLAT AND priority = Blocker",
			"owner":{"name":"somebody"}}`))
	}))
	t.Cleanup(srv.Close)
	b := jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "Requirement")

	f, err := b.FilterByID(context.Background(), "  25475  ")
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if path != "/rest/api/2/filter/25475" {
		t.Errorf("path = %q, want the id trimmed before it is sent", path)
	}
	if f.ID != "25475" || f.Name != "Release blockers" {
		t.Errorf("filter = %+v", f)
	}
	if f.JQL != "project = PLAT AND priority = Blocker" {
		t.Errorf("jql = %q, want the filter's own words", f.JQL)
	}
	// No scope. The scope groups the picker, and a filter reached by id is
	// not in the picker, so reading /myself for a label nothing draws
	// would be a round trip spent on nothing.
	if f.Scope != "" {
		t.Errorf("scope = %q, want it left off a filter reached by id", f.Scope)
	}
}

// An id nobody can see is the common mistake with this field, and it has
// to come back as a failure rather than as an empty filter the view would
// then try to pin.
func TestFilterByIDReportsAnIDThisReaderCannotSee(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	b := jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "Requirement")
	if _, err := b.FilterByID(context.Background(), "999999"); err == nil {
		t.Fatal("want an error for an id that does not resolve")
	}
}
