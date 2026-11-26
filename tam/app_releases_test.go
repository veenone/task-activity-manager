package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/permission"
)

// seedReleaseApp caches release 10100 with PLAT-1 (open) and PLAT-2 (done)
// in it, and release 10103 empty.
func seedReleaseApp(t *testing.T, perms map[string]bool) (*App, string) {
	t.Helper()
	a, id := seedPermissionApp(t, perms)
	ctx := context.Background()
	if err := a.repo.ReplaceVersions(ctx, id, "PLAT", []backend.Version{{ID: 10100, Project: "PLAT", Name: "v1.2"}, {ID: 10103, Project: "PLAT", Name: "v1.3"}}); err != nil {
		t.Fatal(err)
	}
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", StatusCategory: "new", FixVersions: []int{10100}, Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeTask, Summary: "two", Status: "Shipped", StatusCategory: "done", FixVersions: []int{10100}, Updated: "2026-09-01T00:00:00Z"},
	}
	if err := a.repo.UpsertPage(ctx, id, rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	return a, id
}

func TestListVersionSummariesCountsTheCachedIssues(t *testing.T) {
	a, id := seedReleaseApp(t, nil)
	got, err := a.ListVersionSummaries(id)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string][2]int{}
	for _, s := range got {
		counts[s.Name] = [2]int{s.Done, s.Total}
	}
	if counts["v1.2"] != [2]int{1, 2} || counts["v1.3"] != [2]int{0, 0} || len(got) != 2 {
		t.Fatalf("summaries %+v", got)
	}
}

func TestVersionIssuesListsTheReleasesIssues(t *testing.T) {
	a, id := seedReleaseApp(t, nil)
	got, err := a.VersionIssues(id, 10100)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, iss := range got {
		keys = append(keys, iss.Key)
	}
	if strings.Join(keys, ",") != "PLAT-1,PLAT-2" {
		t.Fatalf("keys %v", keys)
	}
	if none, _ := a.VersionIssues(id, 10103); len(none) != 0 || none == nil {
		t.Fatalf("empty release %#v", none)
	}
}

// A draft is local until Commit, so changing or dropping one is not gated,
// the rule draft sprints follow.
func TestADraftReleaseEditAndDeleteAreNotGated(t *testing.T) {
	a, id := seedReleaseApp(t, nil)
	draft, err := a.CreateVersion(id, "v2.0", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.repo.PutPermissions(context.Background(), id, "PLAT", map[string]bool{permission.AdministerProjects: false}); err != nil {
		t.Fatal(err)
	}
	if err := a.EditVersion(id, draft.ID, "v2.0 final", "", "", ""); err != nil {
		t.Fatalf("edit draft: %v", err)
	}
	if err := a.DeleteVersion(id, draft.ID, 0); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
}

// The Release dialog's choices are fix version edits, so they need what a
// fix version edit needs on top of Administer Projects.
func TestReleaseVersionNeedsResolveIssuesForItsMoves(t *testing.T) {
	a, id := seedReleaseApp(t, map[string]bool{permission.ResolveIssues: false})
	err := a.ReleaseVersion(id, 10100, "2026-12-15", 10103, []issuerepo.ReleaseMove{{Key: "PLAT-1", To: 10103}})
	if err == nil || !strings.Contains(err.Error(), "Resolve Issues") {
		t.Fatalf("got %v, want a refusal naming Resolve Issues", err)
	}
	if err := a.ReleaseVersion(id, 10100, "2026-12-15", 0, nil); err != nil {
		t.Fatalf("a release with no moves: %v", err)
	}
	pending, _ := a.ListPendingChanges(id)
	if len(pending) != 1 || pending[0].EntityType != issuerepo.EntityVersionRelease {
		t.Fatalf("pending %+v", pending)
	}
}
