package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	corejira "agile-suite/core/jira"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

// historyFields is the field discovery body for the history tests: Sprint
// and Story Points behind their own per-instance customfield ids, the
// shape fields.go already resolves for every other read.
const historyFields = `[{"id":"customfield_10020","name":"Sprint","custom":true},{"id":"customfield_10016","name":"Story Points","custom":true}]`

func newHistoryServer(t *testing.T, searchBody string) (*jirabackend.Backend, *[]string) {
	t.Helper()
	var searches []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/field":
			_, _ = w.Write([]byte(historyFields))
		case "/rest/api/2/search":
			searches = append(searches, r.URL.RawQuery)
			_, _ = w.Write([]byte(searchBody))
		case "/rest/api/2/project/PLAT":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"1","name":"Task"},{"id":"18","name":"Story"},{"id":"19","name":"Sub-task","subtask":true}]}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	return jirabackend.New(c, "Requirement"), &searches
}

func TestSearchIssuesWithHistoryAsksForTheChangelogAndPagesTheTotal(t *testing.T) {
	b, searches := newHistoryServer(t, `{"total":9,"issues":[
		{"id":"1","key":"PLAT-1","fields":{"summary":"One","status":{"name":"Done"},"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[]},
		 "changelog":{"startAt":0,"maxResults":100,"total":1,"histories":[
			{"created":"2026-09-09T10:42:00.000+0000","items":[{"field":"status","fieldId":"status","fromString":"To Do","toString":"Done"}]}
		 ]}}
	]}`)
	out, total, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 9 || len(out) != 1 {
		t.Fatalf("total %d rows %d", total, len(out))
	}
	if len(*searches) != 1 {
		t.Fatalf("searches = %v", *searches)
	}
	if got := (*searches)[0]; !containsParam(got, "expand", "changelog") {
		t.Errorf("search query = %q, want expand=changelog", got)
	}
	if out[0].Truncated {
		t.Errorf("a changelog whose total matches its histories must not be truncated: %+v", out[0])
	}
	if len(out[0].Changes) != 1 || out[0].Changes[0].Field != "status" || out[0].Changes[0].From != "To Do" || out[0].Changes[0].To != "Done" {
		t.Errorf("changes = %+v", out[0].Changes)
	}
}

func TestATruncatedChangelogIsMarkedAndACompleteOneIsNot(t *testing.T) {
	b, _ := newHistoryServer(t, `{"total":1,"issues":[
		{"id":"1","key":"PLAT-1","fields":{"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[]},
		 "changelog":{"startAt":0,"maxResults":1,"total":5,"histories":[
			{"created":"2026-09-09T10:42:00.000+0000","items":[{"field":"status","fieldId":"status","fromString":"To Do","toString":"In Progress"}]}
		 ]}},
		{"id":"2","key":"PLAT-2","fields":{"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[]},
		 "changelog":{"startAt":0,"maxResults":100,"total":1,"histories":[
			{"created":"2026-09-09T10:42:00.000+0000","items":[{"field":"status","fieldId":"status","fromString":"To Do","toString":"Done"}]}
		 ]}}
	]}`)
	out, _, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("rows = %d", len(out))
	}
	if !out[0].Truncated {
		t.Errorf("PLAT-1's changelog total (5) exceeds its one history; it must be marked truncated")
	}
	if out[1].Truncated {
		t.Errorf("PLAT-2's changelog total (1) matches its one history; it must not be marked truncated")
	}
}

func TestSprintAndStoryPointsChangesMatchByTheDiscoveredCustomFieldId(t *testing.T) {
	// The Sprint change below carries the id fields.go actually discovered
	// for this instance and a field name ("Iteration") that is not
	// recognisable on its own: this is the fixture the review called for,
	// the one that fails if the normaliser is ever keyed on a literal id
	// instead of the per-instance discovery.
	b, _ := newHistoryServer(t, `{"total":1,"issues":[
		{"id":"1","key":"PLAT-1","fields":{"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[]},
		 "changelog":{"startAt":0,"maxResults":100,"total":2,"histories":[
			{"created":"2026-09-09T10:42:00.000+0000","items":[{"field":"Iteration","fieldId":"customfield_10020","fromString":"","toString":"Sprint 11"}]},
			{"created":"2026-09-09T11:00:00.000+0000","items":[{"field":"Story Points","fieldId":"customfield_10016","fromString":"2","toString":"3"}]}
		 ]}}
	]}`)
	out, _, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(out) != 1 || len(out[0].Changes) != 2 {
		t.Fatalf("changes = %+v", out)
	}
	if out[0].Changes[0].Field != "sprint" || out[0].Changes[0].To != "Sprint 11" {
		t.Errorf("sprint change = %+v", out[0].Changes[0])
	}
	if out[0].Changes[1].Field != "storyPoints" || out[0].Changes[1].From != "2" || out[0].Changes[1].To != "3" {
		t.Errorf("story points change = %+v", out[0].Changes[1])
	}
}

func TestAFieldTheNormaliserDoesNotRecogniseIsDropped(t *testing.T) {
	b, _ := newHistoryServer(t, `{"total":1,"issues":[
		{"id":"1","key":"PLAT-1","fields":{"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[]},
		 "changelog":{"startAt":0,"maxResults":100,"total":1,"histories":[
			{"created":"2026-09-09T10:42:00.000+0000","items":[{"field":"Assignee","fieldId":"assignee","fromString":"","toString":"R. Anand"}]}
		 ]}}
	]}`)
	out, _, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(out[0].Changes) != 0 {
		t.Errorf("changes = %+v, want none: assignee is not a field this report reads", out[0].Changes)
	}
}

// containsParam is a small helper since the search log above holds raw
// query strings rather than parsed ones.
func containsParam(rawQuery, key, value string) bool {
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return false
	}
	return q.Get(key) == value
}
