package syncer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/syncer"
)

// A full sync that fails after its first page used to leave only that page:
// the first page's transaction had already deleted the rest of the cache, so
// lists and reports looked current while missing most issues (#534). The
// previous rows now stay until every page has landed.
func TestFullSyncFailingPartwayKeepsThePreviousCache(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	fb := &fake{pages: [][]backend.Issue{
		{issue("PLAT-1", "task")}, {issue("PLAT-2", "task")}, {issue("PLAT-3", "task")},
	}}
	e := syncer.New(fb, repo)
	e.PageSize = 1
	first := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	e.Now = fixedClock(first)
	if _, err := e.Sync(ctx, "p1", "PLAT", "", true, nil); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	before, _ := repo.SyncState(ctx, "p1")

	fb.failPage = 2
	fb.failErr = errors.New("jira: 502 Bad Gateway")
	e.Now = fixedClock(first.Add(time.Hour))
	_, err := e.Sync(ctx, "p1", "PLAT", "", true, nil)
	if !errors.As(err, new(*syncer.PartialSyncError)) {
		t.Fatalf("err = %v, want a partial sync", err)
	}
	for _, key := range []string{"PLAT-1", "PLAT-2", "PLAT-3"} {
		if got, err := repo.GetIssue(ctx, "p1", key); err != nil || got.Key != key {
			t.Errorf("%s = %q, %v; want it still cached after the full sync failed", key, got.Key, err)
		}
	}
	after, _ := repo.SyncState(ctx, "p1")
	if after.LastFull != before.LastFull {
		t.Errorf("last_full = %q, want %q kept", after.LastFull, before.LastFull)
	}
}

// The other half: a full sync that lands every page still drops the issues
// Jira no longer returns, and keeps a local draft.
func TestFullSyncThatFinishesDropsWhatJiraNoLongerHas(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	fb := &fake{pages: [][]backend.Issue{{issue("PLAT-1", "task")}, {issue("PLAT-2", "task")}}}
	e := syncer.New(fb, repo)
	e.PageSize = 1
	first := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	e.Now = fixedClock(first)
	if _, err := e.Sync(ctx, "p1", "PLAT", "", true, nil); err != nil {
		t.Fatalf("seed sync: %v", err)
	}
	drafts, err := repo.CreateDrafts(ctx, "p1", "PLAT", []backend.IssueDraft{{Type: "task", Summary: "local"}}, "")
	if err != nil || len(drafts) != 1 {
		t.Fatalf("draft: %v, %v", drafts, err)
	}

	fb.pages = [][]backend.Issue{{issue("PLAT-2", "task")}}
	e.Now = fixedClock(first.Add(time.Hour))
	if _, err := e.Sync(ctx, "p1", "PLAT", "", true, nil); err != nil {
		t.Fatalf("full sync: %v", err)
	}
	if _, err := repo.GetIssue(ctx, "p1", "PLAT-1"); err == nil {
		t.Error("PLAT-1 is still cached after a full sync Jira no longer returned it in")
	}
	if got, err := repo.GetIssue(ctx, "p1", "PLAT-2"); err != nil || got.Key != "PLAT-2" {
		t.Errorf("PLAT-2 = %q, %v; want it cached", got.Key, err)
	}
	if got, err := repo.GetIssue(ctx, "p1", drafts[0]); err != nil || got.Summary != "local" {
		t.Errorf("draft %s = %q, %v; want the local draft kept", drafts[0], got.Summary, err)
	}
}
