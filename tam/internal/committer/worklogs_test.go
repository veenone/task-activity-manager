package committer_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
)

var logAt = time.Date(2026, 9, 29, 1, 0, 0, 0, time.FixedZone("WIB", 7*60*60))

func logDraft(t *testing.T, timeSpent, comment string, at time.Time) backend.WorklogDraft {
	t.Helper()
	d, err := backend.NewWorklogDraft(timeSpent, comment, at)
	if err != nil {
		t.Fatalf("draft %q: %v", timeSpent, err)
	}
	return d
}

func TestCommitPushesJournalledWorklogs(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	if err := repo.LogWork(ctx, "p1", "PLAT-1", logDraft(t, "2h 30m", "Pairing", logAt)); err != nil {
		t.Fatal(err)
	}

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(f.worklogs) != 1 || f.worklogs[0] != "PLAT-1 2026-09-29T01:00:00.000+0700 2h 30m Pairing" {
		t.Fatalf("pushed = %v, want the entry with its own offset and the typed duration", f.worklogs)
	}
	if len(res.Logged) != 1 || res.Logged[0].Key != "PLAT-1" || res.Logged[0].TimeSpent != "2h 30m" {
		t.Errorf("result = %+v, want the entry named", res.Logged)
	}
	if len(res.Failures) != 0 {
		t.Errorf("failures = %+v", res.Failures)
	}
	left, _ := repo.PendingWorklogs(ctx, "p1", "PLAT-1")
	if len(left) != 0 {
		t.Errorf("journal still holds %+v after the push", left)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 2 || act[0].Action != "commit" || act[0].EntityType != issuerepo.EntityWorklog {
		t.Errorf("audit = %+v, want the commit recorded against the worklog row", act)
	}
}

// TestCommitKeepsTheOtherEntriesWhenOneWorklogFails is the I2 half: an entry
// is somebody's record of their day, so a refusal has to name the issue and
// what Jira said, keep that row, and let the rest land.
func TestCommitKeepsTheOtherEntriesWhenOneWorklogFails(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	if err := repo.LogWork(ctx, "p1", "PLAT-1", logDraft(t, "2h", "First", logAt)); err != nil {
		t.Fatal(err)
	}
	if err := repo.LogWork(ctx, "p1", "PLAT-2", logDraft(t, "30m", "Second", logAt)); err != nil {
		t.Fatal(err)
	}
	f.worklogErr = map[string]error{"PLAT-1": errors.New("Jira says: the worklog is not permitted")}

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("failures = %+v, want just the refused entry", res.Failures)
	}
	bad := res.Failures[0]
	if bad.Key != "PLAT-1" || bad.EntityType != issuerepo.EntityWorklog || !strings.Contains(bad.Error, "not permitted") {
		t.Errorf("failure = %+v, want the issue and Jira's own words", bad)
	}
	if bad.RowID == 0 {
		t.Error("the failure names no journal row, so nothing can discard exactly that entry")
	}
	// The refused entry stays pending and the other one went.
	kept, _ := repo.PendingWorklogs(ctx, "p1", "PLAT-1")
	if len(kept) != 1 || kept[0].TimeSpent != "2h" {
		t.Errorf("PLAT-1 pending = %+v, want the refused entry kept", kept)
	}
	gone, _ := repo.PendingWorklogs(ctx, "p1", "PLAT-2")
	if len(gone) != 0 {
		t.Errorf("PLAT-2 pending = %+v, want the entry that landed cleared", gone)
	}
	if len(res.Logged) != 1 || res.Logged[0].Key != "PLAT-2" {
		t.Errorf("logged = %+v, want only the entry that landed", res.Logged)
	}
}

// noWorklogs is a backend that answers IssueBackend and nothing more, which
// is what a connection that cannot log work looks like.
type noWorklogs struct{ backend.IssueBackend }

func TestCommitSaysWhenTheConnectionCannotLogWork(t *testing.T) {
	_, repo, f := setup(t)
	ctx := context.Background()
	if err := repo.LogWork(ctx, "p1", "PLAT-1", logDraft(t, "2h", "Pairing", logAt)); err != nil {
		t.Fatal(err)
	}

	blind := committer.New(noWorklogs{f}, repo, newBoardOrder())
	res, err := blind.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Error, "cannot log work") {
		t.Fatalf("failures = %+v, want one saying the connection cannot log work", res.Failures)
	}
	if res.Failures[0].Retryable {
		t.Error("a connection that cannot log work will not learn to by the next Commit")
	}
	kept, _ := repo.PendingWorklogs(ctx, "p1", "PLAT-1")
	if len(kept) != 1 {
		t.Errorf("pending = %+v, want the entry kept rather than dropped", kept)
	}
}

// TestCommitLogsWorkAgainstTheKeyACreateJustGave: a worklog drafted on a
// TAM-NEW issue is pushed in the same Commit, under the real key, because the
// phase runs after the creates and the rekey carried the row over.
func TestCommitLogsWorkAgainstTheKeyACreateJustGave(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	key, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "new one"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.LogWork(ctx, "p1", key, logDraft(t, "45m", "Kickoff", logAt)); err != nil {
		t.Fatal(err)
	}

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(res.Created) != 1 {
		t.Fatalf("created = %+v", res.Created)
	}
	real := res.Created[0].Key
	if len(f.worklogs) != 1 || !strings.HasPrefix(f.worklogs[0], real+" ") {
		t.Errorf("pushed = %v, want the entry logged against %s", f.worklogs, real)
	}
}
