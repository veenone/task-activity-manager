package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestWorklogsMapEverySideOfAnEntry(t *testing.T) {
	b, f := newBackend(t, twoFields)
	f.worklogs = `{"startAt":0,"maxResults":200,"total":1,"worklogs":[
		{"id":"10001","author":{"name":"ranand","displayName":"R. Anand"},"started":"2026-09-29T01:00:00.000+0700","timeSpent":"2h 30m","timeSpentSeconds":9000,"comment":"Pairing on the migration"}
	]}`

	logs, err := b.Worklogs(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("Worklogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("logs = %+v", logs)
	}
	got := logs[0]
	want := backend.Worklog{
		ID: "10001", Author: "ranand", AuthorName: "R. Anand",
		Started: "2026-09-29T01:00:00.000+0700", TimeSpent: "2h 30m", Seconds: 9000,
		Comment: "Pairing on the migration",
	}
	if got != want {
		t.Errorf("entry = %+v, want %+v", got, want)
	}
}

func TestAddWorklogPostsToTheIssuesWorklog(t *testing.T) {
	b, f := newBackend(t, twoFields)
	d := backend.WorklogDraft{Started: "2026-09-29T01:00:00.000+0700", TimeSpent: "90m", Comment: "Rebasing", Seconds: 5400}

	if err := b.AddWorklog(context.Background(), "PLAT-412", d); err != nil {
		t.Fatalf("AddWorklog: %v", err)
	}
	if len(f.writes) != 1 || !strings.HasPrefix(f.writes[0], "POST /rest/api/2/issue/PLAT-412/worklog ") {
		t.Fatalf("writes = %v", f.writes)
	}
	body := f.writes[0]
	for _, want := range []string{`"started":"2026-09-29T01:00:00.000+0700"`, `"timeSpent":"90m"`, `"comment":"Rebasing"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s is missing %s", body, want)
		}
	}
	// Seconds is TAM's own preview of the total and never Jira's input: Jira
	// parses the phrase itself, and sending both would let the two disagree.
	if strings.Contains(body, "seconds") {
		t.Errorf("body %s sends seconds; Jira parses timeSpent itself", body)
	}
}
