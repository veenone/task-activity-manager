package jira_test

import (
	"context"
	"strings"
	"testing"
)

// The search names the reader and a day either side of the range, because
// Jira reads worklogDate on its own zone and the entries are kept by the day
// their own offset puts them on. What comes back is narrowed to the reader's
// entries inside the range, and an issue the search cut short is read whole.
func TestWorklogsByKeepsTheReadersEntriesInsideTheRange(t *testing.T) {
	b, f := newBackend(t, twoFields)
	f.searchBody = `{"total":3,"issues":[
		{"key":"PLAT-412","fields":{"summary":"Retry failed payments","worklog":{"total":2,"worklogs":[
			{"id":"1","author":{"name":"Demo"},"started":"2026-11-17T09:00:00.000+0700","timeSpent":"3h","timeSpentSeconds":10800},
			{"id":"2","author":{"name":"ranand"},"started":"2026-11-17T10:00:00.000+0700","timeSpent":"1h","timeSpentSeconds":3600}
		]}}},
		{"key":"OPS-88","fields":{"summary":"Handover","worklog":{"total":1,"worklogs":[
			{"id":"3","author":{"name":"demo"},"started":"2026-11-23T08:00:00.000+0700","timeSpent":"30m","timeSpentSeconds":1800}
		]}}},
		{"key":"PLAT-398","fields":{"summary":"Refund webhook","worklog":{"total":21,"worklogs":[]}}}
	]}`
	f.worklogs = `{"startAt":0,"maxResults":200,"total":1,"worklogs":[
		{"id":"4","author":{"name":"demo"},"started":"2026-11-18T13:00:00.000+0700","timeSpent":"2h","timeSpentSeconds":7200}
	]}`

	got, err := b.WorklogsBy(context.Background(), "demo", "2026-11-16", "2026-11-22")
	if err != nil {
		t.Fatalf("WorklogsBy: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("entries = %+v, want the reader's two inside the week", got)
	}
	if got[0].Key != "PLAT-412" || got[0].Summary != "Retry failed payments" || got[0].Log.Seconds != 10800 {
		t.Errorf("first = %+v, want the reader's 3h on PLAT-412", got[0])
	}
	if got[1].Key != "PLAT-398" || got[1].Log.ID != "4" {
		t.Errorf("second = %+v, want the entry read from the issue the search cut short", got[1])
	}
	if len(f.searches) != 1 {
		t.Fatalf("searches = %v", f.searches)
	}
	for _, want := range []string{"worklogAuthor = currentUser()", `worklogDate >= "2026-11-15"`, `worklogDate <= "2026-11-23"`} {
		if !strings.Contains(f.searches[0], want) {
			t.Errorf("search %q is missing %s", f.searches[0], want)
		}
	}
}

func TestWorklogsByRefusesADayItCannotRead(t *testing.T) {
	b, f := newBackend(t, twoFields)
	if _, err := b.WorklogsBy(context.Background(), "demo", "last week", "2026-11-22"); err == nil {
		t.Fatal("WorklogsBy read a range that starts on no day")
	}
	if len(f.searches) != 0 {
		t.Errorf("searches = %v, want Jira left alone", f.searches)
	}
}
