package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/permission"
)

// seedComponentApp caches Auth (10000) with PLAT-1 under it and Board
// (10001) with none.
func seedComponentApp(t *testing.T, perms map[string]bool) (*App, string) {
	t.Helper()
	a, id := seedPermissionApp(t, perms)
	ctx := context.Background()
	if err := a.repo.ReplaceComponents(ctx, id, "PLAT", []backend.Component{{ID: 10000, Project: "PLAT", Name: "Auth"}, {ID: 10001, Project: "PLAT", Name: "Board"}}); err != nil {
		t.Fatal(err)
	}
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", Components: []int{10000}, Updated: "2026-09-01T00:00:00Z"},
	}
	if err := a.repo.UpsertPage(ctx, id, rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	return a, id
}

func TestListComponentSummariesCountsTheCachedIssues(t *testing.T) {
	a, id := seedComponentApp(t, nil)
	got, err := a.ListComponentSummaries(id)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, s := range got {
		counts[s.Name] = s.Issues
	}
	if len(got) != 2 || counts["Auth"] != 1 || counts["Board"] != 0 {
		t.Fatalf("summaries %+v", got)
	}
	issues, err := a.ComponentIssues(id, 10000)
	if err != nil || len(issues) != 1 || issues[0].Key != "PLAT-1" {
		t.Fatalf("issues %+v %v", issues, err)
	}
}

func TestComponentWritesNeedAdministerProjects(t *testing.T) {
	a, id := seedComponentApp(t, map[string]bool{permission.AdministerProjects: false})
	in := ComponentForm{Name: "Reports"}
	if _, err := a.CreateComponent(id, in); err == nil || !strings.Contains(err.Error(), "Administer Projects") {
		t.Fatalf("create: got %v, want a refusal naming Administer Projects", err)
	}
	if err := a.EditComponent(id, 10000, ComponentForm{Name: "Login"}); err == nil || !strings.Contains(err.Error(), "Administer Projects") {
		t.Fatalf("edit: got %v", err)
	}
	if err := a.DeleteComponent(id, 10000, 0); err == nil || !strings.Contains(err.Error(), "Administer Projects") {
		t.Fatalf("delete: got %v", err)
	}
	if pending, _ := a.ListPendingChanges(id); len(pending) != 0 {
		t.Fatalf("pending %+v", pending)
	}
}

func TestComponentWritesQueueAndADraftIsNotGated(t *testing.T) {
	a, id := seedComponentApp(t, nil)
	draft, err := a.CreateComponent(id, ComponentForm{Name: "Reports", LeadName: "alee", LeadDisplayName: "A. Lee", AssigneeType: "COMPONENT_LEAD"})
	if err != nil || draft.ID >= 0 || draft.LeadDisplayName != "A. Lee" {
		t.Fatalf("create %+v %v", draft, err)
	}
	if err := a.repo.PutPermissions(context.Background(), id, "PLAT", map[string]bool{permission.AdministerProjects: false}); err != nil {
		t.Fatal(err)
	}
	if err := a.EditComponent(id, draft.ID, ComponentForm{Name: "Reporting"}); err != nil {
		t.Fatalf("edit draft: %v", err)
	}
	if err := a.DeleteComponent(id, draft.ID, 0); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	pending, _ := a.ListPendingChanges(id)
	if len(pending) != 0 {
		t.Fatalf("pending %+v", pending)
	}
}
