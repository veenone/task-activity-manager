package syncer_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/syncer"
)

// catalogFake is the fake backend with the project's versions and components.
type catalogFake struct {
	*fake
	versions    []backend.Version
	components  []backend.Component
	versionsErr error
}

func (f *catalogFake) ProjectVersions(context.Context, string) ([]backend.Version, error) {
	return f.versions, f.versionsErr
}

func (f *catalogFake) ProjectComponents(context.Context, string) ([]backend.Component, error) {
	return f.components, nil
}

func TestSyncStoresCatalog(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()
	b := &catalogFake{fake: &fake{},
		versions:   []backend.Version{{ID: 10100, Project: "PLAT", Name: "v1.2", ReleaseDate: "2026-12-15"}},
		components: []backend.Component{{ID: 10000, Project: "PLAT", Name: "Auth"}}}
	if _, err := syncer.New(b, repo).Sync(ctx, "p1", "PLAT", "", false, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	vs, _ := repo.Versions(ctx, "p1", "PLAT")
	cs, _ := repo.Components(ctx, "p1", "PLAT")
	if !reflect.DeepEqual(vs, b.versions) || !reflect.DeepEqual(cs, b.components) {
		t.Errorf("stored versions %+v components %+v", vs, cs)
	}
}

func TestSyncKeepsCatalogWhenItsReadFails(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()
	kept := []backend.Version{{ID: 1, Project: "PLAT", Name: "v1"}}
	if err := repo.ReplaceVersions(ctx, "p1", "PLAT", kept); err != nil {
		t.Fatal(err)
	}
	b := &catalogFake{fake: &fake{pages: [][]backend.Issue{{{Key: "PLAT-1", Summary: "s"}}}},
		versionsErr: errors.New("jira: 500"),
		components:  []backend.Component{{ID: 10000, Project: "PLAT", Name: "Auth"}}}
	if _, err := syncer.New(b, repo).Sync(ctx, "p1", "PLAT", "", false, nil); err != nil {
		t.Fatalf("Sync failed over the catalog read: %v", err)
	}
	if n, _ := repo.CountIssues(ctx, "p1"); n != 1 {
		t.Errorf("%d issues stored, want the page to land", n)
	}
	if vs, _ := repo.Versions(ctx, "p1", "PLAT"); !reflect.DeepEqual(vs, kept) {
		t.Errorf("versions %+v, want the earlier ones kept", vs)
	}
	if cs, _ := repo.Components(ctx, "p1", "PLAT"); len(cs) != 1 {
		t.Errorf("components %+v, want them refreshed despite the version failure", cs)
	}
}

func TestSyncWithoutCatalogBackendLeavesCatalogEmpty(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()
	if _, err := syncer.New(&fake{}, repo).Sync(ctx, "p1", "PLAT", "", false, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if vs, _ := repo.Versions(ctx, "p1", "PLAT"); len(vs) != 0 {
		t.Errorf("versions %+v", vs)
	}
}
