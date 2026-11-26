package issuerepo_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func draftOf(t *testing.T, repo *issuerepo.Repository, key string) backend.IssueDraft {
	t.Helper()
	pend, _ := repo.ListPendingChanges(context.Background(), "p1")
	for _, p := range pend {
		if p.EntityType == issuerepo.EntityIssueCreate && p.EntityKey == key {
			var d backend.IssueDraft
			if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
				t.Fatal(err)
			}
			return d
		}
	}
	t.Fatalf("no create row for %s", key)
	return backend.IssueDraft{}
}

func TestCreateDraftStoresCatalogIDs(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedCatalog(t, repo)
	key, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "x", Components: []int{10000}, FixVersions: []int{10100}})
	if err != nil {
		t.Fatal(err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", key)
	if !reflect.DeepEqual(iss.Components, []int{10000}) || !reflect.DeepEqual(iss.FixVersions, []int{10100}) {
		t.Fatalf("row %v %v", iss.Components, iss.FixVersions)
	}
	if d := draftOf(t, repo, key); !reflect.DeepEqual(d.Components, []int{10000}) || !reflect.DeepEqual(d.FixVersions, []int{10100}) {
		t.Fatalf("draft %+v", d)
	}
	// An edit of a draft rewrites the draft, which is what the create sends.
	if err := repo.EditField(ctx, "p1", key, "components", "10001"); err != nil {
		t.Fatal(err)
	}
	if d := draftOf(t, repo, key); !reflect.DeepEqual(d.Components, []int{10001}) {
		t.Fatalf("draft after edit %+v", d.Components)
	}
}

func TestCreateDraftRefusesAReleasedVersion(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	if _, err := repo.CreateDraft(context.Background(), "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "x", FixVersions: []int{10101}}); err == nil {
		t.Fatal("a draft took a released version")
	}
}
