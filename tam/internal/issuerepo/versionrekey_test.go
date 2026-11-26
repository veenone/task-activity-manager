package issuerepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func TestRekeyVersionMakesTheDraftReal(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	draft, _ := repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	d := strconv.Itoa(draft.ID)
	if err := repo.EditField(ctx, "p1", "PLAT-1", "fixVersions", "10100, "+d); err != nil {
		t.Fatal(err)
	}
	newKey, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "new", FixVersions: []int{draft.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.QueueVersionDelete(ctx, "p1", 10103, draft.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.RekeyVersion(ctx, "p1", draft.ID, backend.Version{ID: 10200, Project: "PLAT", Name: "v2.0"}); err != nil {
		t.Fatal(err)
	}
	v, ok := versionNamed(t, repo, "v2.0")
	if !ok || v.ID != 10200 || v.Draft {
		t.Fatalf("release %+v", v)
	}
	one, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !reflect.DeepEqual(one.FixVersions, []int{10100, 10200}) {
		t.Fatalf("PLAT-1 row %v", one.FixVersions)
	}
	edits := pendingOf(t, repo, issuerepo.EntityIssue)
	if len(edits) != 1 || edits[0].AfterVal != "10100, 10200" {
		t.Fatalf("edits %+v", edits)
	}
	if dr := draftOf(t, repo, newKey); !reflect.DeepEqual(dr.FixVersions, []int{10200}) {
		t.Fatalf("draft issue %v", dr.FixVersions)
	}
	del := pendingOf(t, repo, issuerepo.EntityVersionDelete)
	var vd issuerepo.VersionDelete
	if len(del) != 1 || json.Unmarshal([]byte(del[0].AfterVal), &vd) != nil || vd.MoveTo != 10200 {
		t.Fatalf("delete %+v", del)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityVersionCreate); len(rows) != 0 {
		t.Fatalf("create row kept %+v", rows)
	}
}

func TestRekeyVersionOfAGoneDraft(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	err := repo.RekeyVersion(context.Background(), "p1", -7, backend.Version{ID: 10200, Name: "x"})
	if !errors.Is(err, issuerepo.ErrDraftVersionGone) {
		t.Fatalf("err %v", err)
	}
}

func TestApplyVersionCommittedUpdatesTheCache(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	if err := repo.QueueVersionRelease(ctx, "p1", 10100, "2026-12-15", 0, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.QueueVersionArchive(ctx, "p1", 10101, true); err != nil {
		t.Fatal(err)
	}
	all, _ := repo.ListPendingChanges(ctx, "p1")
	for _, p := range all {
		if err := repo.ApplyVersionCommitted(ctx, "p1", p); err != nil {
			t.Fatal(err)
		}
	}
	if v, _ := versionNamed(t, repo, "v1.2"); !v.Released || v.ReleaseDate != "2026-12-15" {
		t.Fatalf("released %+v", v)
	}
	if v, _ := versionNamed(t, repo, "v1.1"); !v.Archived {
		t.Fatalf("archived %+v", v)
	}
	if rest, _ := repo.ListPendingChanges(ctx, "p1"); len(rest) != 0 {
		t.Fatalf("journal left %+v", rest)
	}
}

func TestApplyVersionDeleteMovesItsIssues(t *testing.T) {
	repo := newRepo(t)
	seedRelease(t, repo)
	ctx := context.Background()
	if err := repo.QueueVersionDelete(ctx, "p1", 10100, 10103); err != nil {
		t.Fatal(err)
	}
	del := pendingOf(t, repo, issuerepo.EntityVersionDelete)
	if err := repo.ApplyVersionCommitted(ctx, "p1", del[0]); err != nil {
		t.Fatal(err)
	}
	if _, ok := versionNamed(t, repo, "v1.2"); ok {
		t.Fatal("deleted release kept")
	}
	one, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	two, _ := repo.GetIssue(ctx, "p1", "PLAT-2")
	if !reflect.DeepEqual(one.FixVersions, []int{10103}) || !reflect.DeepEqual(two.FixVersions, []int{10103}) {
		t.Fatalf("rows %v %v", one.FixVersions, two.FixVersions)
	}
}
