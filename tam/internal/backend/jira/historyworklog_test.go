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

// newWorklogHistoryServer is newHistoryServer with the per-issue worklog
// endpoint behind it, and a count of how often that endpoint was read: the
// whole point of asking the search for worklogs is that most issues cost
// no call of their own.
func newWorklogHistoryServer(t *testing.T, searchBody, worklogBody string) (*jirabackend.Backend, *[]string, *int) {
	t.Helper()
	var searches []string
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/2/field":
			_, _ = w.Write([]byte(historyFields))
		case r.URL.Path == "/rest/api/2/search":
			searches = append(searches, r.URL.RawQuery)
			_, _ = w.Write([]byte(searchBody))
		case r.URL.Path == "/rest/api/2/project/PLAT":
			_, _ = w.Write([]byte(`{"issueTypes":[{"id":"1","name":"Task"},{"id":"18","name":"Story"}]}`))
		case strings.HasSuffix(r.URL.Path, "/worklog"):
			calls++
			_, _ = w.Write([]byte(worklogBody))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	return jirabackend.New(c, "Requirement"), &searches, &calls
}

// twoIssuesOneCutShort is a sprint's search answer: one issue whose
// worklog came back whole, and one Jira says has forty entries while
// sending one, which is the cap a search puts on the field.
const twoIssuesOneCutShort = `{"total":2,"issues":[
	{"id":"1","key":"PLAT-1","fields":{"summary":"One","status":{"name":"Done"},"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[],
		"worklog":{"startAt":0,"maxResults":20,"total":2,"worklogs":[
			{"id":"1","started":"2026-08-04T10:00:00.000+0000","timeSpent":"3h","timeSpentSeconds":10800},
			{"id":"2","started":"2026-08-05T10:00:00.000+0000","timeSpent":"1h","timeSpentSeconds":3600}]}},
	 "changelog":{"startAt":0,"maxResults":100,"total":0,"histories":[]}},
	{"id":"2","key":"PLAT-2","fields":{"summary":"Two","status":{"name":"To Do"},"issuetype":{"name":"Story"},"project":{"key":"PLAT"},"labels":[],
		"worklog":{"startAt":0,"maxResults":20,"total":40,"worklogs":[
			{"id":"3","started":"2026-08-04T10:00:00.000+0000","timeSpent":"2h","timeSpentSeconds":7200}]}},
	 "changelog":{"startAt":0,"maxResults":100,"total":0,"histories":[]}}
]}`

// The history search asks for the worklogs with the rest of the row. A
// sprint of fifty issues is then one request rather than fifty-one.
func TestTheHistorySearchAsksForTheWorklogs(t *testing.T) {
	b, searches, _ := newWorklogHistoryServer(t, twoIssuesOneCutShort, "")
	if _, _, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50); err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(*searches) != 1 || !strings.Contains((*searches)[0], "worklog") {
		t.Errorf("search query = %v, want the worklog among its fields", *searches)
	}
}

// What the search returned whole is used as it stands, and only an issue
// whose worklog it cut short costs a call of its own: the search caps an
// issue's worklogs at 20, so an issue worked on for a month would
// otherwise read as one worked on for a week.
func TestOnlyACutShortWorklogCostsACallOfItsOwn(t *testing.T) {
	b, _, calls := newWorklogHistoryServer(t, twoIssuesOneCutShort, `{"startAt":0,"maxResults":200,"total":2,"worklogs":[
		{"id":"9","started":"2026-08-06T10:00:00.000+0000","timeSpent":"5h","timeSpentSeconds":18000},
		{"id":"10","started":"2026-08-07T10:00:00.000+0000","timeSpent":"4h","timeSpentSeconds":14400}]}`)
	page, _, err := b.SearchIssuesWithHistory(context.Background(), "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("rows = %d", len(page))
	}
	if len(page[0].Worklogs) != 2 || page[0].Worklogs[0].Seconds != 10800 {
		t.Errorf("PLAT-1 worklogs = %+v, want the two the search carried", page[0].Worklogs)
	}
	// The refetch replaces the partial list rather than adding to it, or
	// the first hour would be counted twice.
	if len(page[1].Worklogs) != 2 || page[1].Worklogs[0].Seconds != 18000 {
		t.Errorf("PLAT-2 worklogs = %+v, want the refetched pair", page[1].Worklogs)
	}
	if *calls != 1 {
		t.Errorf("worklog calls = %d, want exactly the one issue the search cut short", *calls)
	}
}
