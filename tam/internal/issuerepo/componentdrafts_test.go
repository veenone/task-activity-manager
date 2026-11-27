package issuerepo_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// seedComponentIssues caches PLAT-1 under Auth (10000) and PLAT-2 under none.
func seedComponentIssues(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", Components: []int{10000}, Updated: v1},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeTask, Summary: "two", Status: "To Do", Updated: v1},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
}

func componentNamed(t *testing.T, repo *issuerepo.Repository, name string) (backend.Component, bool) {
	t.Helper()
	cs, err := repo.Components(context.Background(), "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.Name == name {
			return c, true
		}
	}
	return backend.Component{}, false
}

func TestCreateDraftComponentJournalsAndCaches(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	made, err := repo.CreateDraftComponent(ctx, "p1", "PLAT", issuerepo.ComponentFields{Name: " Reports ", LeadName: "alee", LeadDisplayName: "A. Lee", AssigneeType: "COMPONENT_LEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if made.ID >= 0 || !made.Draft || made.Name != "Reports" || made.LeadDisplayName != "A. Lee" {
		t.Fatalf("made %+v", made)
	}
	got, ok := componentNamed(t, repo, "Reports")
	if !ok || got.ID != made.ID || !got.Draft || got.LeadName != "alee" || got.AssigneeType != "COMPONENT_LEAD" {
		t.Fatalf("cached %+v %v", got, ok)
	}
	rows := pendingOf(t, repo, issuerepo.EntityComponentCreate)
	if len(rows) != 1 || rows[0].EntityKey != strconv.Itoa(made.ID) {
		t.Fatalf("journal %+v", rows)
	}
	var d issuerepo.DraftComponent
	if err := json.Unmarshal([]byte(rows[0].AfterVal), &d); err != nil || d.Name != "Reports" || d.Project != "PLAT" || d.LeadName != "alee" {
		t.Fatalf("draft %+v %v", d, err)
	}
	// No default assignee given means the project's default.
	again, err := repo.CreateDraftComponent(ctx, "p1", "PLAT", issuerepo.ComponentFields{Name: "Search"})
	if err != nil || again.ID != made.ID-1 || again.AssigneeType != "PROJECT_DEFAULT" {
		t.Fatalf("second draft %+v %v", again, err)
	}
}

func TestCreateDraftComponentRefusesBadInput(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	for name, f := range map[string]issuerepo.ComponentFields{
		"duplicate name": {Name: "auth"},
		"no name":        {Name: "  "},
		"bad assignee":   {Name: "Reports", AssigneeType: "SOMEONE"},
	} {
		if _, err := repo.CreateDraftComponent(context.Background(), "p1", "PLAT", f); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if rows := pendingOf(t, repo, issuerepo.EntityComponentCreate); len(rows) != 0 {
		t.Fatalf("journaled %+v", rows)
	}
}

func TestEditDraftComponentRewritesTheCreate(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	made, _ := repo.CreateDraftComponent(ctx, "p1", "PLAT", issuerepo.ComponentFields{Name: "Reports"})
	if err := repo.EditComponent(ctx, "p1", made.ID, issuerepo.ComponentFields{Name: "Reporting", Description: "d"}); err != nil {
		t.Fatal(err)
	}
	rows := pendingOf(t, repo, issuerepo.EntityComponentCreate)
	if len(rows) != 1 || len(pendingOf(t, repo, issuerepo.EntityComponentEdit)) != 0 {
		t.Fatalf("journal %+v", rows)
	}
	var d issuerepo.DraftComponent
	_ = json.Unmarshal([]byte(rows[0].AfterVal), &d)
	if d.Name != "Reporting" || d.Description != "d" || d.Project != "PLAT" {
		t.Fatalf("draft %+v", d)
	}
	if _, ok := componentNamed(t, repo, "Reporting"); !ok {
		t.Fatal("row not renamed")
	}
}

func TestEditComponentJournalsKeepsTheFirstBeforeAndDiscards(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	if err := repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Login"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Sign-in", LeadName: "alee", LeadDisplayName: "A. Lee"}); err != nil {
		t.Fatal(err)
	}
	rows := pendingOf(t, repo, issuerepo.EntityComponentEdit)
	if len(rows) != 1 || rows[0].EntityKey != "10000" {
		t.Fatalf("journal %+v", rows)
	}
	var was, now issuerepo.ComponentFields
	_ = json.Unmarshal([]byte(rows[0].BeforeVal), &was)
	_ = json.Unmarshal([]byte(rows[0].AfterVal), &now)
	if was.Name != "Auth" || now.Name != "Sign-in" || now.LeadName != "alee" || now.AssigneeType != "PROJECT_DEFAULT" {
		t.Fatalf("before %+v after %+v", was, now)
	}
	if c, ok := componentNamed(t, repo, "Sign-in"); !ok || c.LeadDisplayName != "A. Lee" {
		t.Fatalf("row %+v", c)
	}
	if err := repo.DiscardPendingChange(ctx, "p1", rows[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := componentNamed(t, repo, "Auth"); !ok {
		t.Fatal("discard did not restore the name")
	}
}

func TestEditComponentBackToJirasDropsTheEdit(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	if err := repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Auth"}); err != nil {
		t.Fatal(err)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityComponentEdit); len(rows) != 0 {
		t.Fatalf("an edit that changes nothing journaled %+v", rows)
	}
	_ = repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Login"})
	if err := repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Auth"}); err != nil {
		t.Fatal(err)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityComponentEdit); len(rows) != 0 {
		t.Fatalf("an edit back to Jira's kept %+v", rows)
	}
}

func TestDiscardDraftComponentTakesItOffIssues(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	seedComponentIssues(t, repo)
	ctx := context.Background()
	made, _ := repo.CreateDraftComponent(ctx, "p1", "PLAT", issuerepo.ComponentFields{Name: "Reports"})
	draft := strconv.Itoa(made.ID)
	if err := repo.EditField(ctx, "p1", "PLAT-1", "components", "10000, "+draft); err != nil {
		t.Fatal(err)
	}
	newKey, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "new", Components: []int{made.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscardDraftComponent(ctx, "p1", made.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := componentNamed(t, repo, "Reports"); ok {
		t.Fatal("draft row kept")
	}
	one, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !reflect.DeepEqual(one.Components, []int{10000}) {
		t.Fatalf("PLAT-1 components %v", one.Components)
	}
	if edits := pendingOf(t, repo, issuerepo.EntityIssue); len(edits) != 0 {
		t.Fatalf("edits left %+v", edits)
	}
	d, err := repo.GetIssue(ctx, "p1", newKey)
	if err != nil || len(d.Components) != 0 {
		t.Fatalf("draft issue %+v %v", d.Components, err)
	}
}

func TestSyncKeepsAComponentWithAPendingEdit(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	_ = repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Login"})
	if err := repo.ReplaceComponents(ctx, "p1", "PLAT", []backend.Component{{ID: 10000, Project: "PLAT", Name: "Auth"}, {ID: 10001, Project: "PLAT", Name: "Board"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := componentNamed(t, repo, "Login"); !ok {
		t.Fatal("a sync overwrote the pending edit")
	}
}
