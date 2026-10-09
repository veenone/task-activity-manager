package jira

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestProjectComponentDetailsDecodesLead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/project/QA/components" {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":"10","name":"Core","description":"d",
			"lead":{"name":"alice","displayName":"Alice A"},"assigneeType":"COMPONENT_LEAD"}]`)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).ProjectComponentDetails(context.Background(), "QA")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []Component{{ID: "10", Name: "Core", Description: "d", LeadName: "alice",
		LeadDisplayName: "Alice A", AssigneeType: "COMPONENT_LEAD"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestCreateComponentSendsBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/component" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"11","name":"API","assigneeType":"PROJECT_DEFAULT"}`)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).CreateComponent(context.Background(), ComponentInput{
		Project: "QA", Name: "API", Description: "rest", LeadUserName: "bob", AssigneeType: "PROJECT_DEFAULT",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != "11" || got.Name != "API" {
		t.Fatalf("got %+v", got)
	}
	want := map[string]any{"project": "QA", "name": "API", "description": "rest",
		"leadUserName": "bob", "assigneeType": "PROJECT_DEFAULT"}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body %v", body)
	}
}

func TestUpdateComponentOmitsProjectAndClearsLead(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/2/component/11" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"id":"11","name":"Api"}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).UpdateComponent(context.Background(), "11",
		ComponentInput{Project: "QA", Name: "Api", AssigneeType: "UNASSIGNED"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := map[string]any{"name": "Api", "description": "", "leadUserName": "", "assigneeType": "UNASSIGNED"}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body %v", body)
	}
}

func TestDeleteComponentMovesIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/2/component/11" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("moveIssuesTo"); got != "12" {
			t.Errorf("moveIssuesTo %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).DeleteComponent(context.Background(), "11", "12"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDeleteComponentWithoutMoveSendsNoQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("query %q", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).DeleteComponent(context.Background(), "11", ""); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestComponentWritesReport403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"errorMessages":["You do not have permission"],"errors":{}}`)
	}))
	defer srv.Close()
	c := newTestClient(srv)
	ctx := context.Background()

	_, errCreate := c.CreateComponent(ctx, ComponentInput{Project: "QA", Name: "X"})
	_, errUpdate := c.UpdateComponent(ctx, "1", ComponentInput{Name: "X"})
	errDelete := c.DeleteComponent(ctx, "1", "")
	for name, err := range map[string]error{"create": errCreate, "update": errUpdate, "delete": errDelete} {
		var he *HTTPError
		if !errors.As(err, &he) || he.Code != http.StatusForbidden {
			t.Errorf("%s: want *HTTPError 403, got %v", name, err)
			continue
		}
		if he.Message != "You do not have permission" {
			t.Errorf("%s: message %q", name, he.Message)
		}
	}
}

func TestComponentIssueCountAndUserSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/component/11/relatedIssueCounts":
			_, _ = io.WriteString(w, `{"issueCount":7}`)
		case "/rest/api/2/user/search":
			if r.URL.Query().Get("username") != "al" {
				t.Errorf("username %q", r.URL.Query().Get("username"))
			}
			_, _ = io.WriteString(w, `[{"name":"alice","displayName":"Alice A"}]`)
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := newTestClient(srv)

	n, err := c.ComponentIssueCount(context.Background(), "11")
	if err != nil || n != 7 {
		t.Fatalf("count %d err %v", n, err)
	}
	users, err := c.SearchUsers(context.Background(), "al")
	if err != nil || !reflect.DeepEqual(users, []User{{Name: "alice", DisplayName: "Alice A"}}) {
		t.Fatalf("users %+v err %v", users, err)
	}
}
