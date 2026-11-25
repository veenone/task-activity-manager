package issuerepo_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

func TestReplaceVersionsRoundTrips(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	in := []backend.Version{
		{ID: 10101, Project: "PLAT", Name: "v1.3"},
		{ID: 10100, Project: "PLAT", Name: "v1.2", Description: "d", StartDate: "2026-12-01", ReleaseDate: "2026-12-15", Released: true, Archived: true},
	}
	if err := r.ReplaceVersions(ctx, "p1", "PLAT", in); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := r.Versions(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []backend.Version{in[1], in[0]} // dated first, undated last
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestReplaceVersionsKeepsDrafts(t *testing.T) {
	r, db := newRepoWithDB(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO project_version (profile_id, project, id, name, draft) VALUES ('p1', 'PLAT', -1, 'v9 draft', 1)`); err != nil {
		t.Fatalf("seed draft: %v", err)
	}
	if err := r.ReplaceVersions(ctx, "p1", "PLAT", []backend.Version{{ID: 10100, Project: "PLAT", Name: "v1.2"}}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := r.Versions(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 2 || !hasVersion(got, -1, true) || !hasVersion(got, 10100, false) {
		t.Fatalf("got %+v, want the draft kept beside the synced row", got)
	}
}

func TestReplaceVersionsDropsAnotherProjectsSyncedRows(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.ReplaceVersions(ctx, "p1", "A", []backend.Version{{ID: 1, Project: "A", Name: "a1"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceVersions(ctx, "p1", "B", []backend.Version{{ID: 2, Project: "B", Name: "b1"}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Versions(ctx, "p1", "A"); len(got) != 0 {
		t.Fatalf("project A still lists %+v after a sync of B", got)
	}
}

func TestReplaceVersionsWithNoneEmptiesSyncedRows(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.ReplaceVersions(ctx, "p1", "PLAT", []backend.Version{{ID: 1, Project: "PLAT", Name: "v1"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceVersions(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Versions(ctx, "p1", "PLAT"); len(got) != 0 {
		t.Fatalf("got %+v, want an empty answer to empty the cache", got)
	}
}

func TestReplaceComponentsRoundTrips(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	in := []backend.Component{
		{ID: 10001, Project: "PLAT", Name: "reports", AssigneeType: "PROJECT_DEFAULT"},
		{ID: 10000, Project: "PLAT", Name: "Auth", Description: "login", LeadName: "alee", LeadDisplayName: "A. Lee", AssigneeType: "COMPONENT_LEAD"},
	}
	if err := r.ReplaceComponents(ctx, "p1", "PLAT", in); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := r.Components(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if want := []backend.Component{in[1], in[0]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestPurgeProfileDropsTheCatalog(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	for _, p := range []string{"p1", "p2"} {
		if err := r.ReplaceVersions(ctx, p, "PLAT", []backend.Version{{ID: 1, Project: "PLAT", Name: "v1"}}); err != nil {
			t.Fatal(err)
		}
		if err := r.ReplaceComponents(ctx, p, "PLAT", []backend.Component{{ID: 1, Project: "PLAT", Name: "Auth"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.PurgeProfile(ctx, "p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	v1, _ := r.Versions(ctx, "p1", "PLAT")
	c1, _ := r.Components(ctx, "p1", "PLAT")
	if len(v1) != 0 || len(c1) != 0 {
		t.Fatalf("purged profile keeps %d versions and %d components", len(v1), len(c1))
	}
	v2, _ := r.Versions(ctx, "p2", "PLAT")
	c2, _ := r.Components(ctx, "p2", "PLAT")
	if len(v2) != 1 || len(c2) != 1 {
		t.Fatalf("other profile lost its catalog: %d versions, %d components", len(v2), len(c2))
	}
}

func TestUpsertPageKeepsComponentsAndFixVersions(t *testing.T) {
	r, db := newRepoWithDB(t)
	ctx := context.Background()
	page := []backend.Issue{
		{Key: "PLAT-1", Summary: "tagged", Components: []int{10000}, FixVersions: []int{10100, 10101}},
		{Key: "PLAT-2", Summary: "bare"},
	}
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := r.IssuesByKeys(ctx, db, "p1", []string{"PLAT-1", "PLAT-2"})
	if err != nil || len(got) != 2 {
		t.Fatalf("read: %v %+v", err, got)
	}
	byKey := map[string]backend.Issue{got[0].Key: got[0], got[1].Key: got[1]}
	if c, f := byKey["PLAT-1"].Components, byKey["PLAT-1"].FixVersions; !reflect.DeepEqual(c, []int{10000}) || !reflect.DeepEqual(f, []int{10100, 10101}) {
		t.Errorf("PLAT-1 components %v fix versions %v", c, f)
	}
	if c, f := byKey["PLAT-2"].Components, byKey["PLAT-2"].FixVersions; c == nil || f == nil || len(c)+len(f) != 0 {
		t.Errorf("PLAT-2 components %#v fix versions %#v, want empty and non-nil", c, f)
	}
}

func hasVersion(vs []backend.Version, id int, draft bool) bool {
	for _, v := range vs {
		if v.ID == id && v.Draft == draft {
			return true
		}
	}
	return false
}
