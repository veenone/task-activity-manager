package issuerepo_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// seedThree adds PLAT-2 holding component 10001 and OPS-1 from another
// project beside seedOne's PLAT-1.
func seedThree(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	seedOne(t, repo, "p1")
	seedCatalog(t, repo)
	rows := []backend.Issue{
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeTask, Summary: "two", Status: "To Do", Rank: "0|b", Updated: v1, Components: []int{10001}},
		{Key: "OPS-1", ID: "3", Project: "OPS", Type: backend.TypeTask, Summary: "ops", Status: "To Do", Rank: "0|c", Updated: v1},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
}

func componentsOf(t *testing.T, repo *issuerepo.Repository, key string) []int {
	t.Helper()
	iss, err := repo.GetIssue(context.Background(), "p1", key)
	if err != nil {
		t.Fatal(err)
	}
	return iss.Components
}

func TestSetCatalogAddsWithoutDuplicating(t *testing.T) {
	repo := newRepo(t)
	seedThree(t, repo)
	res, err := repo.SetCatalog(context.Background(), "p1", "PLAT", []string{"PLAT-1", "PLAT-2"}, "components", []int{10001, 10000}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := componentsOf(t, repo, "PLAT-2"); !reflect.DeepEqual(got, []int{10001, 10000}) {
		t.Fatalf("PLAT-2 %v, want 10001 kept first and 10000 added once", got)
	}
	if got := componentsOf(t, repo, "PLAT-1"); !reflect.DeepEqual(got, []int{10001, 10000}) {
		t.Fatalf("PLAT-1 %v", got)
	}
	if !reflect.DeepEqual(res.Changed, []string{"PLAT-1", "PLAT-2"}) {
		t.Fatalf("changed %v", res.Changed)
	}
}

func TestSetCatalogReplaces(t *testing.T) {
	repo := newRepo(t)
	seedThree(t, repo)
	if _, err := repo.SetCatalog(context.Background(), "p1", "PLAT", []string{"PLAT-2"}, "components", []int{10000}, false); err != nil {
		t.Fatal(err)
	}
	if got := componentsOf(t, repo, "PLAT-2"); !reflect.DeepEqual(got, []int{10000}) {
		t.Fatalf("PLAT-2 %v, want [10000]", got)
	}
	pend, _ := repo.ListPendingChanges(context.Background(), "p1")
	if len(pend) != 1 || pend[0].BeforeVal != "10001" || pend[0].AfterVal != "10000" {
		t.Fatalf("journal %+v", pend)
	}
}

func TestSetCatalogSkipsOtherProjects(t *testing.T) {
	repo := newRepo(t)
	seedThree(t, repo)
	res, err := repo.SetCatalog(context.Background(), "p1", "PLAT", []string{"OPS-1", "PLAT-1"}, "components", []int{10000}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Skipped, []string{"OPS-1"}) || !reflect.DeepEqual(res.Changed, []string{"PLAT-1"}) {
		t.Fatalf("result %+v", res)
	}
	if got := componentsOf(t, repo, "OPS-1"); len(got) != 0 {
		t.Fatalf("OPS-1 changed to %v", got)
	}
}

func TestSetCatalogLeavesAnUnchangedIssueAlone(t *testing.T) {
	repo := newRepo(t)
	seedThree(t, repo)
	res, err := repo.SetCatalog(context.Background(), "p1", "PLAT", []string{"PLAT-2"}, "components", []int{10001}, true)
	if err != nil {
		t.Fatal(err)
	}
	pend, _ := repo.ListPendingChanges(context.Background(), "p1")
	if len(res.Changed) != 0 || len(pend) != 0 {
		t.Fatalf("result %+v, journal %+v; want nothing", res, pend)
	}
}

func TestSetCatalogRefusesAReleasedVersionForTheWholeBatch(t *testing.T) {
	repo := newRepo(t)
	seedThree(t, repo)
	if _, err := repo.SetCatalog(context.Background(), "p1", "PLAT", []string{"PLAT-1", "PLAT-2"}, "fixVersions", []int{10101}, true); err == nil {
		t.Fatal("a released version was added")
	}
	if pend, _ := repo.ListPendingChanges(context.Background(), "p1"); len(pend) != 0 {
		t.Fatalf("a refused batch journaled %+v", pend)
	}
}
