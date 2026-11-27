package issuerepo_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// seedReleaseIssues caches PLAT-1 in release 10100 and PLAT-2 in none.
func seedReleaseIssues(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", FixVersions: []int{10100}, Updated: v1},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeTask, Summary: "two", Status: "To Do", Updated: v1},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
}

func versionNamed(t *testing.T, repo *issuerepo.Repository, name string) (backend.Version, bool) {
	t.Helper()
	vs, err := repo.Versions(context.Background(), "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.Name == name {
			return v, true
		}
	}
	return backend.Version{}, false
}

func pendingOf(t *testing.T, repo *issuerepo.Repository, entityType string) []journal.PendingChange {
	t.Helper()
	all, err := repo.ListPendingChanges(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	var out []journal.PendingChange
	for _, p := range all {
		if p.EntityType == entityType {
			out = append(out, p)
		}
	}
	return out
}

func TestCreateDraftVersionJournalsAndCaches(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	made, err := repo.CreateDraftVersion(context.Background(), "p1", "PLAT", issuerepo.VersionFields{Name: " v2.0 ", StartDate: "2026-12-01", ReleaseDate: "2026-12-15"})
	if err != nil {
		t.Fatal(err)
	}
	if made.ID >= 0 || !made.Draft || made.Name != "v2.0" {
		t.Fatalf("made %+v", made)
	}
	got, ok := versionNamed(t, repo, "v2.0")
	if !ok || got.ID != made.ID || !got.Draft || got.ReleaseDate != "2026-12-15" {
		t.Fatalf("cached %+v %v", got, ok)
	}
	rows := pendingOf(t, repo, issuerepo.EntityVersionCreate)
	if len(rows) != 1 || rows[0].EntityKey != strconv.Itoa(made.ID) {
		t.Fatalf("journal %+v", rows)
	}
	var d issuerepo.DraftVersion
	if err := json.Unmarshal([]byte(rows[0].AfterVal), &d); err != nil || d.Name != "v2.0" || d.Project != "PLAT" {
		t.Fatalf("draft %+v %v", d, err)
	}
	// A second draft takes the next id down, never the first one's.
	again, err := repo.CreateDraftVersion(context.Background(), "p1", "PLAT", issuerepo.VersionFields{Name: "v2.1"})
	if err != nil || again.ID != made.ID-1 {
		t.Fatalf("second draft %+v %v", again, err)
	}
}

func TestCreateDraftVersionRefusesBadInput(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	for name, f := range map[string]issuerepo.VersionFields{
		"duplicate name":    {Name: "V1.2"},
		"no name":           {Name: "  "},
		"bad date":          {Name: "v3", ReleaseDate: "15/12/2026"},
		"ends before start": {Name: "v3", StartDate: "2026-12-15", ReleaseDate: "2026-12-01"},
	} {
		if _, err := repo.CreateDraftVersion(context.Background(), "p1", "PLAT", f); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if rows := pendingOf(t, repo, issuerepo.EntityVersionCreate); len(rows) != 0 {
		t.Fatalf("journaled %+v", rows)
	}
}

func TestEditDraftVersionRewritesTheCreate(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	made, _ := repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	if err := repo.EditVersion(ctx, "p1", made.ID, issuerepo.VersionFields{Name: "v2.0 final", Description: "d"}); err != nil {
		t.Fatal(err)
	}
	rows := pendingOf(t, repo, issuerepo.EntityVersionCreate)
	if len(rows) != 1 || len(pendingOf(t, repo, issuerepo.EntityVersionEdit)) != 0 {
		t.Fatalf("journal %+v", rows)
	}
	var d issuerepo.DraftVersion
	_ = json.Unmarshal([]byte(rows[0].AfterVal), &d)
	if d.Name != "v2.0 final" || d.Description != "d" || d.Project != "PLAT" {
		t.Fatalf("draft %+v", d)
	}
	if _, ok := versionNamed(t, repo, "v2.0 final"); !ok {
		t.Fatal("row not renamed")
	}
}

func TestEditVersionJournalsAndKeepsTheFirstBefore(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	if err := repo.EditVersion(ctx, "p1", 10100, issuerepo.VersionFields{Name: "v1.2b"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.EditVersion(ctx, "p1", 10100, issuerepo.VersionFields{Name: "v1.2c", ReleaseDate: "2026-12-20"}); err != nil {
		t.Fatal(err)
	}
	rows := pendingOf(t, repo, issuerepo.EntityVersionEdit)
	if len(rows) != 1 || rows[0].EntityKey != "10100" {
		t.Fatalf("journal %+v", rows)
	}
	var was, now issuerepo.VersionFields
	_ = json.Unmarshal([]byte(rows[0].BeforeVal), &was)
	_ = json.Unmarshal([]byte(rows[0].AfterVal), &now)
	if was.Name != "v1.2" || now.Name != "v1.2c" || now.ReleaseDate != "2026-12-20" {
		t.Fatalf("before %+v after %+v", was, now)
	}
	if v, ok := versionNamed(t, repo, "v1.2c"); !ok || v.ReleaseDate != "2026-12-20" {
		t.Fatalf("row %+v", v)
	}
	// Discarding the edit puts the cached release back.
	if err := repo.DiscardPendingChange(ctx, "p1", rows[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := versionNamed(t, repo, "v1.2"); !ok {
		t.Fatal("discard did not restore the name")
	}
}

func TestEditVersionWithNoChangeJournalsNothing(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	if err := repo.EditVersion(context.Background(), "p1", 10100, issuerepo.VersionFields{Name: "v1.2"}); err != nil {
		t.Fatal(err)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityVersionEdit); len(rows) != 0 {
		t.Fatalf("journal %+v", rows)
	}
}

func TestDiscardDraftVersionTakesItOffIssues(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	seedReleaseIssues(t, repo)
	ctx := context.Background()
	made, _ := repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	draft := strconv.Itoa(made.ID)
	if err := repo.EditField(ctx, "p1", "PLAT-1", "fixVersions", "10100, "+draft); err != nil {
		t.Fatal(err)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-2", "fixVersions", draft); err != nil {
		t.Fatal(err)
	}
	newKey, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "new", FixVersions: []int{made.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardDraftVersion(ctx, "p1", made.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := versionNamed(t, repo, "v2.0"); ok {
		t.Fatal("draft row kept")
	}
	one, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	two, _ := repo.GetIssue(ctx, "p1", "PLAT-2")
	if !reflect.DeepEqual(one.FixVersions, []int{10100}) || len(two.FixVersions) != 0 {
		t.Fatalf("rows %v %v", one.FixVersions, two.FixVersions)
	}
	edits := pendingOf(t, repo, issuerepo.EntityIssue)
	// PLAT-1's edit now changes nothing and goes; so does PLAT-2's.
	if len(edits) != 0 {
		t.Fatalf("edits left %+v", edits)
	}
	if d := draftOf(t, repo, newKey); len(d.FixVersions) != 0 {
		t.Fatalf("draft issue keeps %v", d.FixVersions)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityVersionCreate); len(rows) != 0 {
		t.Fatalf("create row kept %+v", rows)
	}
}

func TestDiscardDraftVersionKeepsTheRestOfAnEdit(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	seedReleaseIssues(t, repo)
	ctx := context.Background()
	made, _ := repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	// PLAT-2 gains 10100 and the draft; the discard leaves it gaining 10100.
	if err := repo.EditField(ctx, "p1", "PLAT-2", "fixVersions", "10100, "+strconv.Itoa(made.ID)); err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardDraftVersion(ctx, "p1", made.ID); err != nil {
		t.Fatal(err)
	}
	edits := pendingOf(t, repo, issuerepo.EntityIssue)
	if len(edits) != 1 || edits[0].AfterVal != "10100" || edits[0].BeforeVal != "" {
		t.Fatalf("edits %+v", edits)
	}
	two, _ := repo.GetIssue(ctx, "p1", "PLAT-2")
	if !reflect.DeepEqual(two.FixVersions, []int{10100}) {
		t.Fatalf("row %v", two.FixVersions)
	}
}

func TestReplaceVersionsKeepsAPendingEdit(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	if err := repo.EditVersion(ctx, "p1", 10100, issuerepo.VersionFields{Name: "v1.2b"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceVersions(ctx, "p1", "PLAT", []backend.Version{{ID: 10100, Project: "PLAT", Name: "v1.2"}, {ID: 10103, Project: "PLAT", Name: "v1.3"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := versionNamed(t, repo, "v1.2b"); !ok {
		t.Fatal("the sync overwrote a release with a pending edit")
	}
	if _, ok := versionNamed(t, repo, "v1.3"); !ok {
		t.Fatal("the sync did not add the new release")
	}
	if _, ok := versionNamed(t, repo, "v1.2"); ok {
		t.Fatal("the release is listed twice")
	}
}
