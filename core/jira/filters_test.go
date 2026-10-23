package jira_test

import (
	"context"
	"errors"
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

// A filter you created is not starred just because you made it, so the
// favourites alone cannot answer "which of these are mine". This is the
// read that can.
func TestOwnedFiltersReadTheFiltersYouOwn(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"10200","name":"Mine, never starred","jql":"reporter = currentUser()"}]`))
	}))
	defer srv.Close()

	owned, err := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client()).OwnedFilters(context.Background())
	if err != nil {
		t.Fatalf("owned: %v", err)
	}
	if path != "/rest/api/2/filter/my" {
		t.Errorf("path = %q", path)
	}
	if len(owned) != 1 || owned[0].ID != "10200" || owned[0].Name != "Mine, never starred" {
		t.Fatalf("owned = %+v", owned)
	}
	if owned[0].JQL != "reporter = currentUser()" {
		t.Errorf("jql = %q", owned[0].JQL)
	}
}

// Not every Jira exposes the route. A 404 is the instance saying it cannot
// answer the question, which the caller turns back into the starred list
// rather than a failed view, so it has to be told apart from a real error.
func TestOwnedFiltersWithoutTheRouteIsErrNoOwnedFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client()).OwnedFilters(context.Background())
	if !errors.Is(err, jira.ErrNoOwnedFilters) {
		t.Fatalf("err = %v, want ErrNoOwnedFilters", err)
	}
}

// A 500 is a real failure and must not be read as "no such route", or a
// broken instance would quietly look like an old one.
func TestOwnedFiltersPassARealFailureThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client()).OwnedFilters(context.Background())
	if err == nil || errors.Is(err, jira.ErrNoOwnedFilters) {
		t.Fatalf("err = %v, want a real failure", err)
	}
}
