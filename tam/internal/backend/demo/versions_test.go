package demo_test

import (
	"context"
	"slices"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

func versionByID(t *testing.T, b *demobackend.Backend, id int) (backend.Version, bool) {
	t.Helper()
	vs, err := b.ProjectVersions(context.Background(), "ACME")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if v.ID == id {
			return v, true
		}
	}
	return backend.Version{}, false
}

func TestDemoVersionLifecycle(t *testing.T) {
	b := demobackend.New("ACME")
	var _ backend.VersionBackend = b
	ctx := context.Background()
	made, err := b.CreateVersion(ctx, "ACME", backend.VersionInput{Name: "v2.0", ReleaseDate: "2026-12-15"})
	if err != nil || made.ID <= 10103 || made.Name != "v2.0" {
		t.Fatalf("made %+v %v", made, err)
	}
	if v, ok := versionByID(t, b, made.ID); !ok || v.ReleaseDate != "2026-12-15" {
		t.Fatalf("listed %+v %v", v, ok)
	}
	if _, err := b.UpdateVersion(ctx, made.ID, backend.VersionInput{Name: "v2.0 final"}); err != nil {
		t.Fatal(err)
	}
	if v, _ := versionByID(t, b, made.ID); v.Name != "v2.0 final" || v.ReleaseDate != "" {
		t.Fatalf("updated %+v", v)
	}
	if err := b.ArchiveVersion(ctx, made.ID, true); err != nil {
		t.Fatal(err)
	}
	if v, _ := versionByID(t, b, made.ID); !v.Archived {
		t.Fatalf("archive %+v", v)
	}
	if err := b.DeleteVersion(ctx, made.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := versionByID(t, b, made.ID); ok {
		t.Fatal("deleted version still listed")
	}
	if err := b.DeleteVersion(ctx, made.ID, 0); err == nil {
		t.Fatal("deleted a version twice")
	}
}

// v1.2 (10102) is the active sprint's release in the dataset, so it holds
// finished and unfinished issues.
func TestDemoReleaseMovesTheUnfinishedIssues(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	open, err := b.UnresolvedInVersion(ctx, 10102)
	if err != nil || len(open) == 0 {
		t.Fatalf("unresolved %v %v", open, err)
	}
	if err := b.ReleaseVersion(ctx, 10102, "2026-12-15", 10103); err != nil {
		t.Fatal(err)
	}
	if v, _ := versionByID(t, b, 10102); !v.Released || v.ReleaseDate != "2026-12-15" {
		t.Fatalf("released %+v", v)
	}
	if left, _ := b.UnresolvedInVersion(ctx, 10102); len(left) != 0 {
		t.Fatalf("still unresolved in the released version: %v", left)
	}
	for _, key := range open {
		iss, _ := b.GetIssue(ctx, key)
		if slices.Contains(iss.FixVersions, 10102) || !slices.Contains(iss.FixVersions, 10103) {
			t.Errorf("%s fix versions %v", key, iss.FixVersions)
		}
	}
}
