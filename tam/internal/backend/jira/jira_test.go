package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

// fakeJira answers the four endpoints the backend touches and records the
// search requests it saw.
type fakeJira struct {
	fieldCalls int32
	searches   []string
	fields     string // the /rest/api/2/field body
}

func (f *fakeJira) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/myself":
			_, _ = w.Write([]byte(`{"name":"jdoe","displayName":"J. Doe"}`))
		case r.URL.Path == "/rest/api/2/field":
			atomic.AddInt32(&f.fieldCalls, 1)
			_, _ = w.Write([]byte(f.fields))
		case r.URL.Path == "/rest/api/2/search":
			f.searches = append(f.searches, r.URL.Query().Get("jql")+" | fields="+r.URL.Query().Get("fields"))
			_, _ = w.Write([]byte(`{"total":2,"issues":[
				{"id":"1","key":"PLAT-412","fields":{"summary":"Promo","status":{"name":"In Progress"},"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[],"customfield_10020":[{"id":12,"name":"Sprint 12"}],"customfield_10016":5}},
				{"id":"2","key":"PLAT-388","fields":{"summary":"Single use","status":{"name":"Approved"},"issuetype":{"name":"Business Requirement"},"project":{"key":"PLAT"},"labels":["promo"]}}
			]}`))
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/PLAT-412"):
			_, _ = w.Write([]byte(`{"id":"1","key":"PLAT-412","fields":{"description":"As a shopper","issuelinks":[{"type":{"name":"Tested By"},"inwardIssue":{"key":"XT-1018","fields":{"summary":"Applies discount","issuetype":{"name":"Test"}}}}],"customfield_10016":5}}`))
		case r.URL.Path == "/rest/api/2/project/PLAT":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"1","name":"Task"},{"id":"7","name":"Business Requirement"}]}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func newBackend(t *testing.T, fields string) (*jirabackend.Backend, *fakeJira) {
	t.Helper()
	f := &fakeJira{fields: fields}
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	c := corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	return jirabackend.New(c, "Business Requirement"), f
}

const twoFields = `[{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true}]`

func TestSearchBuildsTheScopeAndMapsDiscoveredFields(t *testing.T) {
	b, f := newBackend(t, twoFields)
	ctx := context.Background()
	if b.IsDemo() {
		t.Fatal("IsDemo = true")
	}
	u, err := b.TestConnection(ctx)
	if err != nil || u.DisplayName != "J. Doe" {
		t.Fatalf("connection = %+v, %v", u, err)
	}
	page, total, err := b.SearchIssuesPage(ctx, "PLAT", "labels = promo", "2026-09-05T10:42:00Z", backend.AllTypes, 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 2 || len(page) != 2 {
		t.Fatalf("total %d rows %d", total, len(page))
	}
	wantJQL := `project = PLAT AND issuetype in ("Task", "Epic", "Story", "Bug", "Business Requirement") AND (labels = promo) AND updated >= "2026-09-05 09:42" ORDER BY key ASC`
	if len(f.searches) != 1 || !strings.HasPrefix(f.searches[0], wantJQL+" | fields=") {
		t.Errorf("search request = %q", f.searches)
	}
	if !strings.Contains(f.searches[0], "customfield_10020") || !strings.Contains(f.searches[0], "customfield_10016") {
		t.Errorf("discovered ids not requested: %q", f.searches[0])
	}
	if page[0].SprintID != "12" || page[0].StoryPoints == nil || *page[0].StoryPoints != 5 || page[0].Type != "story" {
		t.Errorf("row 0 = %+v", page[0])
	}
	if page[1].Type != "requirement" {
		t.Errorf("row 1 type = %q, want requirement (the profile's name)", page[1].Type)
	}
	if _, _, err := b.SearchIssuesPage(ctx, "PLAT", "", "", backend.AllTypes, 0, 50); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&f.fieldCalls); n != 1 {
		t.Errorf("/rest/api/2/field fetched %d times across two searches, want 1", n)
	}
}

func TestMissingCustomFieldsLeaveColumnsEmpty(t *testing.T) {
	b, _ := newBackend(t, `[{"id":"summary","name":"Summary","custom":false}]`)
	page, _, err := b.SearchIssuesPage(context.Background(), "PLAT", "", "", backend.AllTypes, 0, 50)
	if err != nil {
		t.Fatalf("search with no custom fields must not fail: %v", err)
	}
	if page[0].SprintID != "" || page[0].StoryPoints != nil || page[0].Rank != "" {
		t.Errorf("row = %+v, want empty custom columns", page[0])
	}
}

func TestGetIssueDetailAndIssueTypes(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	ctx := context.Background()
	d, err := b.GetIssueDetail(ctx, "PLAT-412")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.Key != "PLAT-412" || d.Description != "As a shopper" || len(d.Links) != 1 || d.Links[0].Key != "XT-1018" {
		t.Errorf("detail = %+v", d)
	}
	if d.Fields["customfield_10016"] != 5.0 {
		t.Errorf("custom fields = %+v", d.Fields)
	}
	types, err := b.IssueTypes(ctx, "PLAT")
	if err != nil || len(types) != 2 || types[1].Name != "Business Requirement" {
		t.Errorf("types = %+v, %v", types, err)
	}
}
