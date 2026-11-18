package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

// ownWorklogBackend answers the reader's entries and records who was asked
// about and for which days.
type ownWorklogBackend struct {
	worklogBackend
	asked []string
	mine  []backend.LoggedWork
}

func (o *ownWorklogBackend) WorklogsBy(_ context.Context, author, fromDay, toDay string) ([]backend.LoggedWork, error) {
	o.asked = append(o.asked, author+" "+fromDay+" "+toDay)
	return append([]backend.LoggedWork{}, o.mine...), nil
}

func seedWorklogWeekApp(t *testing.T) (*App, string, *ownWorklogBackend) {
	t.Helper()
	a := newTestApp(t)
	p := newTestProfile(t, a)
	b := &ownWorklogBackend{mine: []backend.LoggedWork{{
		Key: "PLAT-2", Summary: "two",
		Log: backend.Worklog{ID: "9", Author: "demo", Started: "2026-11-17T09:00:00.000+0700", TimeSpent: "3h", Seconds: 10800},
	}}}
	a.backends[p.ID] = b
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "In Progress", StatusCategory: "indeterminate", Assignee: "Demo User", AssigneeName: "demo", Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeTask, Summary: "two", Status: "Done", StatusCategory: "done", Assignee: "R. Anand", AssigneeName: "ranand", Updated: "2026-09-01T00:00:00Z"},
	}
	if err := a.repo.UpsertPage(context.Background(), p.ID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return a, p.ID, b
}

func TestWorklogWeekJoinsJirasEntriesWithThePendingOnes(t *testing.T) {
	a, id, b := seedWorklogWeekApp(t)
	if err := a.repo.SetProfileSetting(context.Background(), id, settingJiraUsername, "demo"); err != nil {
		t.Fatalf("setting: %v", err)
	}
	if err := a.LogWork(id, "PLAT-1", "2h", "Pairing", "2026-11-18T10:00"); err != nil {
		t.Fatalf("LogWork: %v", err)
	}
	if err := a.LogWork(id, "PLAT-1", "1h", "Earlier", "2026-11-02T10:00"); err != nil {
		t.Fatalf("LogWork: %v", err)
	}

	v, err := a.WorklogWeek(id, "2026-11-16", "2026-11-22")
	if err != nil {
		t.Fatalf("WorklogWeek: %v", err)
	}
	if len(b.asked) != 1 || b.asked[0] != "demo 2026-11-16 2026-11-22" {
		t.Errorf("asked = %v, want one read for the connected user and the week", b.asked)
	}
	if len(v.Rows) != 2 || v.Rows[0].Issue.Key != "PLAT-1" || !v.Rows[0].Assigned || v.Rows[1].Issue.Key != "PLAT-2" || !v.Rows[1].Cached {
		t.Errorf("rows = %+v, want my open issue then the one I logged on", v.Rows)
	}
	if len(v.Entries) != 2 || v.Entries[0].Key != "PLAT-2" || v.Entries[1].Key != "PLAT-1" || !v.Entries[1].Log.Pending || v.Entries[1].Log.PendingID == 0 {
		t.Errorf("entries = %+v, want the 3h Jira holds then the pending 2h of the same week", v.Entries)
	}
}

func TestWorklogWeekSaysWhenTheJiraUserIsNotKnown(t *testing.T) {
	a, id, b := seedWorklogWeekApp(t)
	_, err := a.WorklogWeek(id, "2026-11-16", "2026-11-22")
	if err == nil || !strings.Contains(err.Error(), "does not know your Jira user") {
		t.Fatalf("WorklogWeek = %v, want it to say the user is unknown", err)
	}
	if len(b.asked) != 0 {
		t.Errorf("asked = %v, want Jira left alone", b.asked)
	}
}

func TestWorklogWeekRefusesARangeItCannotRead(t *testing.T) {
	a, id, _ := seedWorklogWeekApp(t)
	for _, r := range [][2]string{{"", "2026-11-22"}, {"2026-11-22", "2026-11-16"}, {"2026-01-01", "2026-11-22"}} {
		if _, err := a.WorklogWeek(id, r[0], r[1]); err == nil {
			t.Errorf("WorklogWeek(%q, %q) read it", r[0], r[1])
		}
	}
}
