package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

// worklogBackend is a Jira that holds one entry and records every read and
// write, so a test can say what the binding asked it for.
type worklogBackend struct {
	stubIssueBackend
	reads  []string
	writes []string
	held   []backend.Worklog
}

func (w *worklogBackend) Worklogs(_ context.Context, key string) ([]backend.Worklog, error) {
	w.reads = append(w.reads, key)
	return append([]backend.Worklog{}, w.held...), nil
}

func (w *worklogBackend) AddWorklog(_ context.Context, key string, d backend.WorklogDraft) error {
	w.writes = append(w.writes, key+" "+d.TimeSpent)
	return nil
}

func seedWorklogApp(t *testing.T) (*App, string, *worklogBackend) {
	t.Helper()
	a := newTestApp(t)
	p := newTestProfile(t, a)
	b := &worklogBackend{held: []backend.Worklog{{
		ID: "10001", Author: "ranand", AuthorName: "R. Anand",
		Started: "2026-09-28T09:00:00.000+0700", TimeSpent: "1h", Seconds: 3600, Comment: "Yesterday",
	}}}
	a.backends[p.ID] = b
	rows := []backend.Issue{{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", Updated: "2026-09-01T00:00:00Z"}}
	if err := a.repo.UpsertPage(context.Background(), p.ID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return a, p.ID, b
}

func TestListWorklogsPutsThePendingEntriesAfterJirasOwn(t *testing.T) {
	a, id, b := seedWorklogApp(t)
	if err := a.LogWork(id, "PLAT-1", "2h 30m", "Pairing"); err != nil {
		t.Fatalf("LogWork: %v", err)
	}
	if len(b.writes) != 0 {
		t.Fatalf("writes = %v, want nothing sent to Jira before Commit", b.writes)
	}

	logs, err := a.ListWorklogs(id, "PLAT-1")
	if err != nil {
		t.Fatalf("ListWorklogs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("logs = %+v, want Jira's entry and the pending one", logs)
	}
	if logs[0].TimeSpent != "1h" || logs[0].Pending {
		t.Errorf("first = %+v, want Jira's own entry", logs[0])
	}
	if logs[1].TimeSpent != "2h 30m" || !logs[1].Pending || logs[1].PendingID == 0 {
		t.Errorf("second = %+v, want the pending entry with its journal row", logs[1])
	}
	// The entry is dated by the machine's own zone, so its day is the day the
	// user logged it rather than whatever UTC makes of the instant.
	wantDay := time.Now().Format("2006-01-02")
	if !strings.HasPrefix(logs[1].Started, wantDay) {
		t.Errorf("started = %q, want it dated %s", logs[1].Started, wantDay)
	}
	if got := time.Now().Format("-0700"); !strings.HasSuffix(logs[1].Started, got) {
		t.Errorf("started = %q, want it to carry this machine's offset %s", logs[1].Started, got)
	}
	if len(b.reads) != 1 || b.reads[0] != "PLAT-1" {
		t.Errorf("reads = %v, want one read of the issue", b.reads)
	}
}

// TestListWorklogsOnADraftNeverAsksJira: a draft has no issue in Jira, so
// asking would be a 404 about a key Jira has never heard of.
func TestListWorklogsOnADraftNeverAsksJira(t *testing.T) {
	a, id, b := seedWorklogApp(t)
	key, err := a.repo.CreateDraft(context.Background(), id, "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "new one"})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if err := a.LogWork(id, key, "45m", "Kickoff"); err != nil {
		t.Fatalf("LogWork: %v", err)
	}

	logs, err := a.ListWorklogs(id, key)
	if err != nil {
		t.Fatalf("ListWorklogs: %v", err)
	}
	if len(logs) != 1 || logs[0].TimeSpent != "45m" || !logs[0].Pending {
		t.Errorf("logs = %+v, want the journalled entry alone", logs)
	}
	if len(b.reads) != 0 {
		t.Errorf("reads = %v, want Jira left alone for a draft", b.reads)
	}
}

func TestLogWorkAndCheckWorkDurationRefuseTheSameValues(t *testing.T) {
	a, id, _ := seedWorklogApp(t)
	for _, bad := range []string{"", "banana", "0m"} {
		checked := a.CheckWorkDuration(bad)
		if checked == nil {
			t.Errorf("CheckWorkDuration(%q) allowed it", bad)
			continue
		}
		logged := a.LogWork(id, "PLAT-1", bad, "")
		if logged == nil || logged.Error() != checked.Error() {
			t.Errorf("LogWork(%q) = %v, want the same refusal the form got: %v", bad, logged, checked)
		}
	}
	if err := a.CheckWorkDuration("2h 30m"); err != nil {
		t.Errorf("CheckWorkDuration(2h 30m) = %v", err)
	}
	logs, _ := a.ListWorklogs(id, "PLAT-1")
	if len(logs) != 1 {
		t.Errorf("logs = %+v, want only Jira's entry: no refusal wrote a row", logs)
	}
}
