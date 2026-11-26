package issuerepo_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// seedCatalog gives p1 two components, one open release, one released and
// one archived.
func seedCatalog(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	ctx := context.Background()
	if err := repo.ReplaceComponents(ctx, "p1", "PLAT", []backend.Component{
		{ID: 10000, Project: "PLAT", Name: "Auth"}, {ID: 10001, Project: "PLAT", Name: "Board"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceVersions(ctx, "p1", "PLAT", []backend.Version{
		{ID: 10100, Project: "PLAT", Name: "v1.2"},
		{ID: 10101, Project: "PLAT", Name: "v1.1", Released: true},
		{ID: 10102, Project: "PLAT", Name: "v1.0", Archived: true},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestEditComponentsJournalsIDsAndStoresArray(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	seedCatalog(t, repo)
	if err := repo.EditField(ctx, "p1", "PLAT-1", "components", "10000, 10001"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !reflect.DeepEqual(iss.Components, []int{10000, 10001}) {
		t.Fatalf("row components %v", iss.Components)
	}
	pend, _ := repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 1 || pend[0].Field != "components" || pend[0].BeforeVal != "" || pend[0].AfterVal != "10000, 10001" {
		t.Fatalf("journal: %+v", pend)
	}
	if got := issuerepo.FieldValue(iss, "components"); got != "10000, 10001" {
		t.Fatalf("FieldValue %q", got)
	}
}

func TestEditFixVersionsRefusesANewlyAddedReleasedVersion(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	seedCatalog(t, repo)
	for _, id := range []string{"10101", "10102"} {
		err := repo.EditField(ctx, "p1", "PLAT-1", "fixVersions", id)
		if err == nil || !strings.Contains(err.Error(), "cannot be added") {
			t.Fatalf("adding %s: got %v, want a refusal", id, err)
		}
	}
	// An issue that already has the released version keeps it when another
	// release is added beside it.
	if _, err := db.Exec(`UPDATE issue SET fix_versions = '[10101]' WHERE key = 'PLAT-1'`); err != nil {
		t.Fatal(err)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "fixVersions", "10101, 10100"); err != nil {
		t.Fatalf("keeping a released version: %v", err)
	}
}

func TestEditComponentsRefusesAnUnknownID(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	seedCatalog(t, repo)
	if err := repo.EditField(ctx, "p1", "PLAT-1", "components", "10000, 99999"); err == nil || !strings.Contains(err.Error(), "99999") {
		t.Fatalf("got %v, want a refusal naming 99999", err)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "components", "Auth"); err == nil {
		t.Fatal("a name was accepted as an id")
	}
	if pend, _ := repo.ListPendingChanges(ctx, "p1"); len(pend) != 0 {
		t.Fatalf("a refused edit was journaled: %+v", pend)
	}
}

// A sync that rewrites the row from Jira must not hide the pending edit.
func TestReapplyKeepsAPendingFixVersionEdit(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	seedCatalog(t, repo)
	if err := repo.EditField(ctx, "p1", "PLAT-1", "fixVersions", "10100"); err != nil {
		t.Fatal(err)
	}
	fresh := []backend.Issue{{
		Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do",
		Priority: "Medium", Labels: []string{"a"}, Rank: "0|a", Updated: v1,
	}}
	if err := repo.UpsertPage(ctx, "p1", fresh, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !reflect.DeepEqual(iss.FixVersions, []int{10100}) {
		t.Fatalf("fix versions after sync %v, want the pending [10100]", iss.FixVersions)
	}
}
