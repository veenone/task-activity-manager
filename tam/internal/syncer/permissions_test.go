package syncer_test

import (
	"context"
	"errors"
	"testing"

	"agile-suite/tam/internal/permission"
	"agile-suite/tam/internal/syncer"
)

// permFake is the fake backend with an answer about permissions.
type permFake struct {
	*fake
	perms map[string]bool
	err   error
	asked []string
}

func (f *permFake) Permissions(_ context.Context, projectKey string) (map[string]bool, error) {
	f.asked = append(f.asked, projectKey)
	return f.perms, f.err
}

func TestSyncStoresTheGatedPermissionsOfTheProject(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()
	b := &permFake{fake: &fake{}, perms: map[string]bool{
		permission.ManageSprints: false, permission.EditIssues: true, "ADMINISTER": true,
	}}
	if _, err := syncer.New(b, repo).Sync(ctx, "p1", "PLAT", "", false, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(b.asked) != 1 || b.asked[0] != "PLAT" {
		t.Errorf("asked about %v, want PLAT once", b.asked)
	}
	got, err := repo.Permissions(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Allows("PLAT", permission.ManageSprints) || len(got["PLAT"]) != 2 {
		t.Errorf("stored %v, want manage sprints denied and only the gated keys kept", got)
	}
}

func TestAFailedPermissionReadKeepsTheSyncAndTheStoredAnswer(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()
	if err := repo.PutPermissions(ctx, "p1", "PLAT", map[string]bool{permission.ManageSprints: false}); err != nil {
		t.Fatal(err)
	}
	b := &permFake{fake: &fake{}, err: errors.New("jira: 500")}
	if _, err := syncer.New(b, repo).Sync(ctx, "p1", "PLAT", "", false, nil); err != nil {
		t.Fatalf("Sync failed over the permission read: %v", err)
	}
	got, _ := repo.Permissions(ctx, "p1")
	if got.Allows("PLAT", permission.ManageSprints) {
		t.Errorf("stored %v, want the earlier denial left in place", got)
	}
}
