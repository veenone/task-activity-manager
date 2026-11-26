package committer_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestCommitPushesFixVersions(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := h.repo.ReplaceVersions(ctx, "p1", "PLAT", []backend.Version{{ID: 10100, Project: "PLAT", Name: "v1.2"}, {ID: 10101, Project: "PLAT", Name: "v1.3"}}); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-1", "fixVersions", "10100, 10101"); err != nil {
		t.Fatal(err)
	}
	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || len(h.jira.updates) != 1 || h.jira.updates[0] != "PLAT-1 fixVersions=10100, 10101" {
		t.Fatalf("failures %+v, updates %v", res.Failures, h.jira.updates)
	}
}

// Until drafted releases get a create phase (PR 3 of #605), an edit naming
// one must stop at the firewall rather than reach Jira as id -1.
func TestCommitRefusesADraftReleaseID(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.db.Exec(`INSERT INTO project_version (profile_id, project, id, name, draft) VALUES ('p1', 'PLAT', -1, 'v9', 1)`); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-1", "fixVersions", "-1"); err != nil {
		t.Fatal(err)
	}
	res, err := h.eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.jira.updates) != 0 {
		t.Fatalf("sent to Jira: %v", h.jira.updates)
	}
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Error, "placeholder") {
		t.Fatalf("failures %+v, want one naming the placeholder", res.Failures)
	}
}
