package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"agile-suite/core/jira"
)

func TestProjectsListsEveryVisibleProjectByKeyAndName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/project" {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":"1","key":"A","name":"Alpha","projectCategory":{"name":"X"}},{"id":"2","key":"B","name":"Beta"}]`))
	}))
	defer srv.Close()
	got, err := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client()).Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (jira.Project{Key: "A", Name: "Alpha"}) || got[1].Key != "B" {
		t.Fatalf("got %+v", got)
	}
}

func TestNoVisibleProjectIsAnEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	got, err := jira.NewClientWithHTTP(srv.URL, "tok", srv.Client()).Projects(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}
