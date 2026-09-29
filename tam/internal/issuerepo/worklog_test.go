package issuerepo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func draft(timeSpent string, at time.Time) backend.WorklogDraft {
	d, err := backend.NewWorklogDraft(timeSpent, "Pairing", at)
	if err != nil {
		panic(err)
	}
	return d
}

var wlAt = time.Date(2026, 9, 29, 1, 0, 0, 0, time.FixedZone("WIB", 7*60*60))

func TestLogWorkJournalsAnEntryAndNothingReachesJira(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")

	if err := repo.LogWork(ctx, "p1", "PLAT-1", draft("2h 30m", wlAt)); err != nil {
		t.Fatalf("LogWork: %v", err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	if len(pend) != 1 || pend[0].EntityType != issuerepo.EntityWorklog {
		t.Fatalf("journal rows: %+v", pend)
	}
	// The row's field is the started stamp, so two entries on one issue are
	// two rows rather than the second replacing the first.
	if pend[0].Field != "2026-09-29T01:00:00.000+0700" {
		t.Errorf("field = %q, want the started stamp", pend[0].Field)
	}
	logs, err := repo.PendingWorklogs(ctx, "p1", "PLAT-1")
	if err != nil {
		t.Fatalf("PendingWorklogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("pending worklogs: %+v", logs)
	}
	got := logs[0]
	if got.TimeSpent != "2h 30m" || got.Seconds != 9000 || got.Comment != "Pairing" {
		t.Errorf("entry = %+v, want the typed duration, its seconds and the comment", got)
	}
	if !got.Pending || got.PendingID != pend[0].ID {
		t.Errorf("entry = %+v, want it marked pending and carrying its journal row id", got)
	}
	if got.Started != "2026-09-29T01:00:00.000+0700" {
		t.Errorf("started = %q, want the offset that dates it 29 Sep", got.Started)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !iss.Pending {
		t.Error("a pending worklog marks the row as having pending changes")
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 1 || act[0].Action != "worklog" || act[0].AfterVal != "2h 30m" {
		t.Errorf("audit: %+v", act)
	}
}

// TestLogWorkKeepsTwoEntriesOnOneIssue: the journal is unique on entity,
// key and field, so an entry whose field was fixed would be overwritten by
// the next one logged on the same issue and the user would lose a record of
// their day.
func TestLogWorkKeepsTwoEntriesOnOneIssue(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")

	if err := repo.LogWork(ctx, "p1", "PLAT-1", draft("2h", wlAt)); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := repo.LogWork(ctx, "p1", "PLAT-1", draft("30m", wlAt.Add(3*time.Hour))); err != nil {
		t.Fatalf("second: %v", err)
	}
	logs, _ := repo.PendingWorklogs(ctx, "p1", "PLAT-1")
	if len(logs) != 2 {
		t.Fatalf("pending worklogs = %+v, want both entries kept", logs)
	}
	total := logs[0].Seconds + logs[1].Seconds
	if total != 9000 {
		t.Errorf("seconds = %d, want 9000: 2h and 30m both journalled", total)
	}
}

func TestLogWorkRefusesABadDurationAndAnUnknownIssue(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")

	// The same rule the form was refused by, applied again at the store:
	// a draft reaching here with a duration Jira would refuse writes no row.
	bad := backend.WorklogDraft{Started: "2026-09-29T01:00:00.000+0700", TimeSpent: "banana", Seconds: 60}
	if err := repo.LogWork(ctx, "p1", "PLAT-1", bad); err == nil {
		t.Error("a duration Jira would refuse was journalled")
	} else if !strings.Contains(err.Error(), "2h 30m") {
		t.Errorf("error = %q, want the rule's own sentence", err)
	}
	if err := repo.LogWork(ctx, "p1", "NOPE-1", draft("1h", wlAt)); err == nil {
		t.Error("work was logged against an issue the cache does not have")
	}
	pend, _ := repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 0 {
		t.Errorf("journal = %+v, want nothing written by either refusal", pend)
	}
}

// TestDiscardWorklogDropsTheRowAndNothingElse is the I2 half: a logged entry
// is the user's own record of their day, so discarding one takes that row and
// leaves the entry logged beside it alone.
func TestDiscardWorklogDropsTheRowAndNothingElse(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	if err := repo.LogWork(ctx, "p1", "PLAT-1", draft("2h", wlAt)); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := repo.LogWork(ctx, "p1", "PLAT-1", draft("30m", wlAt.Add(3*time.Hour))); err != nil {
		t.Fatalf("second: %v", err)
	}
	logs, _ := repo.PendingWorklogs(ctx, "p1", "PLAT-1")
	if len(logs) != 2 {
		t.Fatalf("pending worklogs = %+v", logs)
	}

	if err := repo.DiscardPendingChange(ctx, "p1", logs[0].PendingID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	left, _ := repo.PendingWorklogs(ctx, "p1", "PLAT-1")
	if len(left) != 1 || left[0].TimeSpent != logs[1].TimeSpent {
		t.Fatalf("left = %+v, want only the other entry", left)
	}
	// The issue itself is untouched: a worklog changed no local column, so
	// there is nothing to put back and nothing to corrupt.
	iss, err := repo.GetIssue(ctx, "p1", "PLAT-1")
	if err != nil || iss.Summary != "one" {
		t.Errorf("issue after a discarded worklog = %+v, %v", iss, err)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 3 || act[0].Action != "discard" {
		t.Errorf("audit = %+v, want the discard recorded on top of the two entries", act)
	}
}
