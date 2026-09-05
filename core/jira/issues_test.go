package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSearchIssuesPassesPagingAndFieldsAndDecodesRawFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("jql") != `project = PLAT ORDER BY key ASC` {
			t.Errorf("jql = %q", q.Get("jql"))
		}
		if q.Get("startAt") != "50" || q.Get("maxResults") != "25" || q.Get("fields") != "summary,status" {
			t.Errorf("paging/fields = %v", q)
		}
		_, _ = w.Write([]byte(`{"total":77,"issues":[{"id":"10001","key":"PLAT-1","fields":{"summary":"One","status":{"name":"To Do"}}}]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	page, err := c.SearchIssues(context.Background(), "project = PLAT ORDER BY key ASC", []string{"summary", "status"}, 50, 25)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if page.Total != 77 || len(page.Issues) != 1 {
		t.Fatalf("page = %+v", page)
	}
	iss := page.Issues[0]
	if iss.ID != "10001" || iss.Key != "PLAT-1" {
		t.Errorf("issue = %+v", iss)
	}
	if string(iss.Fields["summary"]) != `"One"` {
		t.Errorf("summary raw = %s", iss.Fields["summary"])
	}
}

func TestGetIssueRequestsFieldsAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/PLAT-9" || r.URL.Query().Get("fields") != "description,issuelinks" {
			t.Errorf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"id":"9","key":"PLAT-9","fields":{"description":"d"}}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	iss, err := c.GetIssue(context.Background(), "PLAT-9", []string{"description", "issuelinks"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.Key != "PLAT-9" || string(iss.Fields["description"]) != `"d"` {
		t.Errorf("issue = %+v", iss)
	}
}

func TestIssueTypesReadsTheProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/project/PLAT" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"key":"PLAT","issueTypes":[{"id":"1","name":"Task","subtask":false},{"id":"5","name":"Sub-task","subtask":true}]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	types, err := c.IssueTypes(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("types: %v", err)
	}
	if len(types) != 2 || types[0].Name != "Task" || !types[1].Subtask {
		t.Errorf("types = %+v", types)
	}
}

func TestCustomFieldIDLoadsOnceAndReportsAbsence(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Path != "/rest/api/2/field" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":"summary","name":"Summary","custom":false},{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true}]`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	ctx := context.Background()
	id, err := c.CustomFieldID(ctx, "sprint")
	if err != nil || id != "customfield_10020" {
		t.Fatalf("Sprint = %q, %v", id, err)
	}
	id, err = c.CustomFieldID(ctx, "Story Points")
	if err != nil || id != "customfield_10016" {
		t.Fatalf("Story Points = %q, %v", id, err)
	}
	if _, err := c.CustomFieldID(ctx, "Epic Link"); !errors.Is(err, ErrFieldNotFound) {
		t.Fatalf("Epic Link err = %v, want ErrFieldNotFound", err)
	}
	if _, err := c.CustomFieldID(ctx, "Summary"); !errors.Is(err, ErrFieldNotFound) {
		t.Fatalf("a system field is not a custom field: err = %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("/rest/api/2/field fetched %d times, want 1", n)
	}
}

func TestCustomFieldIDRejectsEmptyName(t *testing.T) {
	c := NewClientWithHTTP("https://jira.example", "t", nil)
	if _, err := c.CustomFieldID(context.Background(), "  "); err == nil {
		t.Fatal("want an error for an empty field name")
	}
}
