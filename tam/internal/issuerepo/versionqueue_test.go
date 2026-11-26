package issuerepo_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// seedRelease caches four releases (10100 and 10103 open, 10101 released,
// 10102 archived), PLAT-1 in 10100 and PLAT-2 in 10100 and 10103.
func seedRelease(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	ctx := context.Background()
	if err := repo.ReplaceVersions(ctx, "p1", "PLAT", []backend.Version{
		{ID: 10100, Project: "PLAT", Name: "v1.2"},
		{ID: 10101, Project: "PLAT", Name: "v1.1", Released: true},
		{ID: 10102, Project: "PLAT", Name: "v1.0", Released: true, Archived: true},
		{ID: 10103, Project: "PLAT", Name: "v1.3"},
	}); err != nil {
		t.Fatal(err)
	}
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", FixVersions: []int{10100}, Updated: v1},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeTask, Summary: "two", Status: "In Progress", FixVersions: []int{10100, 10103}, Updated: v1},
	}
	if err := repo.UpsertPage(ctx, "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
}

func TestQueueReleaseJournalsTheMovesAndTheRelease(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	err := repo.QueueVersionRelease(ctx, "p1", 10100, "2026-12-15", 10103, []issuerepo.ReleaseMove{{Key: "PLAT-1", To: 10103}, {Key: "PLAT-2", To: 0}})
	if err != nil {
		t.Fatal(err)
	}
	edits := map[string][2]string{}
	for _, p := range pendingOf(t, repo, issuerepo.EntityIssue) {
		edits[p.EntityKey] = [2]string{p.BeforeVal, p.AfterVal}
	}
	want := map[string][2]string{"PLAT-1": {"10100", "10103"}, "PLAT-2": {"10100, 10103", "10103"}}
	if !reflect.DeepEqual(edits, want) {
		t.Fatalf("edits %v, want %v", edits, want)
	}
	rel := pendingOf(t, repo, issuerepo.EntityVersionRelease)
	if len(rel) != 1 || rel[0].EntityKey != "10100" {
		t.Fatalf("release rows %+v", rel)
	}
	var r issuerepo.VersionRelease
	if err := json.Unmarshal([]byte(rel[0].AfterVal), &r); err != nil || r.ReleaseDate != "2026-12-15" || r.MoveTo != 10103 || r.Name != "v1.2" || r.MoveToName != "v1.3" {
		t.Fatalf("release %+v %v", r, err)
	}
	// Jira still holds the release open until Commit.
	if v, _ := versionNamed(t, repo, "v1.2"); v.Released {
		t.Fatal("released locally before Commit")
	}
	one, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !reflect.DeepEqual(one.FixVersions, []int{10103}) {
		t.Fatalf("PLAT-1 row %v", one.FixVersions)
	}
}

func TestQueueReleaseRefuses(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	draft, _ := repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	cases := map[string]func() error{
		"a draft":             func() error { return repo.QueueVersionRelease(ctx, "p1", draft.ID, "2026-12-15", 0, nil) },
		"a released one":      func() error { return repo.QueueVersionRelease(ctx, "p1", 10101, "2026-12-15", 0, nil) },
		"onto a released one": func() error { return repo.QueueVersionRelease(ctx, "p1", 10100, "2026-12-15", 10101, nil) },
		"onto itself":         func() error { return repo.QueueVersionRelease(ctx, "p1", 10100, "2026-12-15", 10100, nil) },
		"a bad date":          func() error { return repo.QueueVersionRelease(ctx, "p1", 10100, "15 Dec", 0, nil) },
		"an issue not in it": func() error {
			return repo.QueueVersionRelease(ctx, "p1", 10103, "2026-12-15", 0, []issuerepo.ReleaseMove{{Key: "PLAT-1", To: 0}})
		},
	}
	for name, run := range cases {
		if err := run(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if n := len(pendingOf(t, repo, issuerepo.EntityVersionRelease)) + len(pendingOf(t, repo, issuerepo.EntityIssue)); n != 0 {
		t.Fatalf("%d rows journaled", n)
	}
	if err := repo.QueueVersionRelease(ctx, "p1", 10100, "2026-12-15", 0, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.QueueVersionRelease(ctx, "p1", 10100, "2026-12-16", 0, nil); err == nil {
		t.Error("a second release was queued over the first")
	}
}

func TestQueueArchiveAndUnarchive(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	if err := repo.QueueVersionArchive(ctx, "p1", 10101, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.QueueVersionArchive(ctx, "p1", 10102, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.QueueVersionArchive(ctx, "p1", 10102, true); err == nil {
		t.Error("archived an archived release")
	}
	got := map[string]bool{}
	for _, p := range pendingOf(t, repo, issuerepo.EntityVersionArchive) {
		var a issuerepo.VersionArchive
		if err := json.Unmarshal([]byte(p.AfterVal), &a); err != nil {
			t.Fatal(err)
		}
		got[p.EntityKey] = a.Archived
	}
	if !reflect.DeepEqual(got, map[string]bool{"10101": true, "10102": false}) {
		t.Fatalf("archive rows %v", got)
	}
}

func TestQueueDeleteSupersedesAnEdit(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	if err := repo.EditVersion(ctx, "p1", 10100, issuerepo.VersionFields{Name: "v1.2b"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.QueueVersionDelete(ctx, "p1", 10100, 10103); err != nil {
		t.Fatal(err)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityVersionEdit); len(rows) != 0 {
		t.Fatalf("edit kept %+v", rows)
	}
	if _, ok := versionNamed(t, repo, "v1.2"); !ok {
		t.Fatal("the edit was not reverted")
	}
	del := pendingOf(t, repo, issuerepo.EntityVersionDelete)
	var d issuerepo.VersionDelete
	if len(del) != 1 || json.Unmarshal([]byte(del[0].AfterVal), &d) != nil || d.MoveTo != 10103 || d.Name != "v1.2" {
		t.Fatalf("delete rows %+v", del)
	}
	if err := repo.EditVersion(ctx, "p1", 10100, issuerepo.VersionFields{Name: "v1.2c"}); err == nil {
		t.Error("edited a release queued for deletion")
	}
}

func TestQueueDeleteOfADraftDiscardsIt(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	draft, _ := repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	if err := repo.QueueVersionDelete(ctx, "p1", draft.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := versionNamed(t, repo, "v2.0"); ok {
		t.Fatal("draft kept")
	}
	if rows := pendingOf(t, repo, issuerepo.EntityVersionCreate); len(rows) != 0 {
		t.Fatalf("create row kept %+v", rows)
	}
}

func TestDiscardingADraftTargetClearsTheReleaseCarryOver(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	draft, _ := repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	if err := repo.QueueVersionRelease(ctx, "p1", 10100, "2026-12-15", draft.ID, []issuerepo.ReleaseMove{{Key: "PLAT-1", To: draft.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardDraftVersion(ctx, "p1", draft.ID); err != nil {
		t.Fatal(err)
	}
	rel := pendingOf(t, repo, issuerepo.EntityVersionRelease)
	var r issuerepo.VersionRelease
	if len(rel) != 1 || json.Unmarshal([]byte(rel[0].AfterVal), &r) != nil || r.MoveTo != 0 || r.MoveToName != "" {
		t.Fatalf("release %+v", rel)
	}
	edits := pendingOf(t, repo, issuerepo.EntityIssue)
	if len(edits) != 1 || edits[0].BeforeVal != "10100" || edits[0].AfterVal != "" {
		t.Fatalf("PLAT-1 is now descoped, edits %+v", edits)
	}
}
