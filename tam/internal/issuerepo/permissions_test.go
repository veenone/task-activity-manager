package issuerepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/permission"
)

func TestPutPermissionsKeepsEachProjectsAnswer(t *testing.T) {
	r, ctx := newRepo(t), context.Background()
	if err := r.PutPermissions(ctx, "p1", "PLAT", map[string]bool{permission.ManageSprints: false}); err != nil {
		t.Fatal(err)
	}
	if err := r.PutPermissions(ctx, "p1", "OPS", map[string]bool{permission.ManageSprints: true}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Permissions(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Allows("PLAT", permission.ManageSprints) || !got.Allows("OPS", permission.ManageSprints) {
		t.Errorf("permissions = %v, want PLAT denied and OPS held", got)
	}
	other, err := r.Permissions(ctx, "p2")
	if err != nil || len(other) != 0 {
		t.Errorf("another profile reads %v (%v), want nothing", other, err)
	}
}

func TestPutPermissionsDoesNotOverwriteWithAnEmptyAnswer(t *testing.T) {
	r, ctx := newRepo(t), context.Background()
	if err := r.PutPermissions(ctx, "p1", "PLAT", map[string]bool{permission.EditIssues: false}); err != nil {
		t.Fatal(err)
	}
	if err := r.PutPermissions(ctx, "p1", "PLAT", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Permissions(ctx, "p1")
	if got.Allows("PLAT", permission.EditIssues) {
		t.Errorf("permissions = %v, want the earlier denial kept", got)
	}
}

func TestPermissionsReadsAnUnparseableValueAsNothingKnown(t *testing.T) {
	r, ctx := newRepo(t), context.Background()
	if err := r.SetProfileSetting(ctx, "p1", "jira_permissions", "{not json"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Permissions(ctx, "p1")
	if err != nil || !got.Allows("PLAT", permission.EditIssues) {
		t.Errorf("permissions = %v (%v), want everything allowed", got, err)
	}
}

func TestPurgeProfileRemovesItsPermissions(t *testing.T) {
	r, ctx := newRepo(t), context.Background()
	if err := r.PutPermissions(ctx, "p1", "PLAT", map[string]bool{permission.EditIssues: false}); err != nil {
		t.Fatal(err)
	}
	if err := r.PurgeProfile(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Permissions(ctx, "p1")
	if len(got) != 0 {
		t.Errorf("permissions after purge = %v, want none", got)
	}
}
