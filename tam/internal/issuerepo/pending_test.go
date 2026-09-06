package issuerepo_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

// newRepoDB is newRepo plus the raw handle, for tests that seed the journal
// directly.
func newRepoDB(t *testing.T) (*issuerepo.Repository, *sql.DB) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return issuerepo.New(db.DB()), db.DB()
}

func TestListIssuesFlagsPendingAndDraftRows(t *testing.T) {
	repo, db := newRepoDB(t)
	ctx := context.Background()
	rows := []backend.Issue{
		{Key: "PLAT-1", Type: backend.TypeTask, Summary: "one", Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-2", Type: backend.TypeTask, Summary: "two", Updated: "2026-09-01T00:00:00Z"},
		{Key: issuerepo.DraftPrefix + "1", Type: backend.TypeTask, Summary: "draft", Status: issuerepo.StatusDraft},
	}
	if err := repo.UpsertPage(ctx, "p1", rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := journal.Upsert(db, "p1", issuerepo.EntityIssue, "PLAT-1", "summary", "one", "uno", "2026-09-01T00:00:00Z"); err != nil {
		t.Fatalf("journal: %v", err)
	}
	page, err := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string][2]bool{}
	for _, iss := range page.Issues {
		got[iss.Key] = [2]bool{iss.Pending, iss.Draft}
	}
	want := map[string][2]bool{"PLAT-1": {true, false}, "PLAT-2": {false, false}, "TAM-NEW-1": {false, true}}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: pending/draft = %v, want %v", k, got[k], w)
		}
	}
	one, err := repo.GetIssue(ctx, "p1", "PLAT-1")
	if err != nil || !one.Pending {
		t.Errorf("GetIssue must carry the flag too: %+v %v", one, err)
	}
	keys, err := repo.PendingKeys(ctx, "p1")
	if err != nil || len(keys) != 1 || keys[0] != "PLAT-1" {
		t.Errorf("PendingKeys = %v %v", keys, err)
	}
	if other, _ := repo.PendingKeys(ctx, "p2"); len(other) != 0 {
		t.Errorf("profiles are isolated: %v", other)
	}
}

func TestFullSyncClearKeepsDraftRows(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed := []backend.Issue{
		{Key: "PLAT-1", Type: backend.TypeTask, Summary: "one"},
		{Key: issuerepo.DraftPrefix + "1", Type: backend.TypeTask, Summary: "draft", Status: issuerepo.StatusDraft},
	}
	if err := repo.UpsertPage(ctx, "p1", seed, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	remote := []backend.Issue{{Key: "PLAT-2", Type: backend.TypeTask, Summary: "two"}}
	if err := repo.UpsertPage(ctx, "p1", remote, time.Now(), true); err != nil {
		t.Fatalf("full: %v", err)
	}
	page, _ := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{})
	keys := []string{}
	for _, iss := range page.Issues {
		keys = append(keys, iss.Key)
	}
	if len(keys) != 2 || keys[0] != "TAM-NEW-1" || keys[1] != "PLAT-2" {
		t.Errorf("after a full clear the draft survives and PLAT-1 goes: %v", keys)
	}
}

func TestPendingReadsDelegateToTheJournal(t *testing.T) {
	repo, db := newRepoDB(t)
	ctx := context.Background()
	if err := journal.Upsert(db, "p1", issuerepo.EntityIssue, "PLAT-1", "summary", "a", "b", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := journal.Put(db, "p1", issuerepo.EntityIssueCreate, "TAM-NEW-1", issuerepo.FieldCreate, "", `{"summary":"x"}`, ""); err != nil {
		t.Fatal(err)
	}
	if err := journal.Audit(db, "p1", issuerepo.EntityIssue, "PLAT-1", "edit", "summary", "a", "b", ""); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListPendingChanges(ctx, "p1")
	if err != nil || len(all) != 2 {
		t.Fatalf("ListPendingChanges: %v %d", err, len(all))
	}
	one, err := repo.PendingForKey(ctx, "p1", "TAM-NEW-1")
	if err != nil || len(one) != 1 || one[0].EntityType != issuerepo.EntityIssueCreate {
		t.Errorf("PendingForKey: %+v %v", one, err)
	}
	act, err := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if err != nil || len(act) != 1 || act[0].Action != "edit" {
		t.Errorf("ListActivity: %+v %v", act, err)
	}
	if none, _ := repo.ListActivity(ctx, "p1", "PLAT-2", 0); len(none) != 0 {
		t.Errorf("activity is per key: %+v", none)
	}
}
