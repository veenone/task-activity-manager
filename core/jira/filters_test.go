package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"agile-suite/core/jira"
)

// The favourite filters are what a dashboard is built from: the saved
// searches a person already keeps, rather than JQL they have to retype.
func TestFavouriteFiltersReadTheirNameAndTheirJQL(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"10100","name":"My open bugs","jql":"assignee = currentUser() AND type = Bug"},
			{"id":"10101","name":"Platform this quarter","jql":"project in (PLAT, OPS)"}
		]`))
	}))
	defer srv.Close()

	c := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	filters, err := c.FavouriteFilters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if path != "/rest/api/2/filter/favourite" {
		t.Errorf("path = %q", path)
	}
	if len(filters) != 2 {
		t.Fatalf("filters = %+v", filters)
	}
	if filters[0].ID != "10100" || filters[0].Name != "My open bugs" {
		t.Errorf("first filter = %+v", filters[0])
	}
	if filters[1].JQL != "project in (PLAT, OPS)" {
		t.Errorf("second filter jql = %q", filters[1].JQL)
	}
}

// An instance that answers with nothing is a person with no favourites,
// which is a fact and not a failure.
func TestNoFavouriteFiltersIsAnEmptyListRatherThanAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	filters, err := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client()).FavouriteFilters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("filters = %+v, want none", filters)
	}
}

// Who wrote a filter is the only thing that separates one a person owns
// from one somebody else shared with them, and Data Center has no route
// that answers that question for a whole list. It answers it per filter,
// in the list TAM already reads.
func TestFavouriteFiltersCarryTheirOwner(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"10100","name":"Mine","jql":"reporter = currentUser()",
			 "owner":{"name":"achmarah","displayName":"A Person"}},
			{"id":"10101","name":"Theirs","jql":"project = PLAT",
			 "owner":{"name":"someone-else","displayName":"Another Person"}}
		]`))
	}))
	defer srv.Close()

	c := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	filters, err := c.FavouriteFilters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if len(filters) != 2 {
		t.Fatalf("filters = %+v", filters)
	}
	if filters[0].Owner.Name != "achmarah" {
		t.Errorf("first owner = %q, want the username /myself answers with", filters[0].Owner.Name)
	}
	if filters[1].Owner.Name != "someone-else" {
		t.Errorf("second owner = %q", filters[1].Owner.Name)
	}
}

// A Jira that names its users some other way, Cloud among them, leaves the
// username empty. An empty owner is a real answer and must stay empty
// rather than read as a match against another empty one.
func TestFavouriteFiltersLeaveAnUnnamedOwnerEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"10100","name":"Mine","jql":"project = PLAT",
			"owner":{"accountId":"5b10a2844c20165700ede21g"}}]`))
	}))
	defer srv.Close()

	c := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	filters, err := c.FavouriteFilters(context.Background())
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	if filters[0].Owner.Name != "" {
		t.Errorf("owner = %q, want empty when the instance names users another way", filters[0].Owner.Name)
	}
}

// A filter nobody starred is in no list Data Center will enumerate, so the
// only way to reach one is by the id a reader reads off a Jira URL.
func TestOneFilterByIDReadsItsNameAndJQL(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"25475","name":"All Open Bugs","jql":"type = Bug and resolution is empty",
			"owner":{"name":"someone-else"}}`))
	}))
	defer srv.Close()

	c := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	f, err := c.Filter(context.Background(), "25475")
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if path != "/rest/api/2/filter/25475" {
		t.Errorf("path = %q", path)
	}
	if f.Name != "All Open Bugs" || f.JQL != "type = Bug and resolution is empty" {
		t.Errorf("filter = %+v", f)
	}
	if f.Owner.Name != "someone-else" {
		t.Errorf("owner = %q, want the owner this route carries too", f.Owner.Name)
	}
}

// An id with a slash or a space in it is not a filter id, and sending it
// raw would reach a different route or none at all.
func TestOneFilterByIDEscapesWhatItIsGiven(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// EscapedPath, not Path: Path has already been unescaped, so a
		// slash sent as %2F and one sent raw read the same there.
		path = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","name":"n","jql":"project = PLAT"}`))
	}))
	defer srv.Close()

	c := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if _, err := c.Filter(context.Background(), "1/permission"); err != nil {
		t.Fatalf("filter: %v", err)
	}
	if path != "/rest/api/2/filter/1%2Fpermission" {
		t.Errorf("path = %q, want the id escaped into one segment", path)
	}
}
