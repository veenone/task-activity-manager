package main

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/permission"
)

// seedCatalogApp is seedPermissionApp with one open release and one
// component cached for PLAT.
func seedCatalogApp(t *testing.T, perms map[string]bool) (*App, string) {
	t.Helper()
	a, id := seedPermissionApp(t, perms)
	ctx := context.Background()
	if err := a.repo.ReplaceVersions(ctx, id, "PLAT", []backend.Version{{ID: 10100, Project: "PLAT", Name: "v1.2"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.repo.ReplaceComponents(ctx, id, "PLAT", []backend.Component{{ID: 10000, Project: "PLAT", Name: "Auth"}}); err != nil {
		t.Fatal(err)
	}
	return a, id
}

// Jira lets an account set a fix version only with Resolve Issues on top of
// Edit Issues, so holding Edit Issues alone is not enough for that field.
func TestEditIssueRefusesFixVersionsWithoutResolveIssues(t *testing.T) {
	a, id := seedCatalogApp(t, map[string]bool{permission.EditIssues: true, permission.ResolveIssues: false})
	err := a.EditIssue(id, "PLAT-1", "fixVersions", "10100")
	if err == nil || !strings.Contains(err.Error(), "Resolve Issues") {
		t.Fatalf("got %v, want a refusal naming Resolve Issues", err)
	}
	if err := a.EditIssue(id, "PLAT-1", "components", "10000"); err != nil {
		t.Fatalf("components needs only Edit Issues: %v", err)
	}
}

func TestGetVersionsAndComponentsReadTheProfilesProject(t *testing.T) {
	a, id := seedCatalogApp(t, nil)
	vs, err := a.GetVersions(id)
	if err != nil || len(vs) != 1 || vs[0].Name != "v1.2" {
		t.Fatalf("versions %+v, %v", vs, err)
	}
	cs, err := a.GetComponents(id)
	if err != nil || len(cs) != 1 || cs[0].Name != "Auth" {
		t.Fatalf("components %+v, %v", cs, err)
	}
}
