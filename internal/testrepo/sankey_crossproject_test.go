package testrepo

import (
	"path/filepath"
	"testing"

	"xray-test-manager/internal/store"
)

// newTestRepo opens a fresh temp-backed Repository for direct-insert seeding in
// internal (white-box) tests that need package-private helpers like projectKeyOf.
func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewRepository(st)
}

// seedContainer inserts one container (test plan / set / execution) directly.
func seedContainer(t *testing.T, r *Repository, profileID, jiraKey, kind, summary, status string) {
	t.Helper()
	if _, err := r.db.Exec(
		`INSERT INTO test_container (profile_id, jira_key, kind, summary, status)
		 VALUES (?, ?, ?, ?, ?)`,
		profileID, jiraKey, kind, summary, status); err != nil {
		t.Fatalf("seed container %s: %v", jiraKey, err)
	}
}

// seedContainerTest inserts one container/test membership directly, carrying the
// run status for execution memberships.
func seedContainerTest(t *testing.T, r *Repository, profileID, containerKey, testKey, runStatus string) {
	t.Helper()
	if _, err := r.db.Exec(
		`INSERT INTO test_container_test (profile_id, container_key, test_key, run_status)
		 VALUES (?, ?, ?, ?)`,
		profileID, containerKey, testKey, runStatus); err != nil {
		t.Fatalf("seed membership %s/%s: %v", containerKey, testKey, err)
	}
}

func TestProjectKeyOf(t *testing.T) {
	cases := map[string]string{
		"RND_P_4TFINT_05-123": "RND_P_4TFINT_05",
		"XRAYINT-TE-1":        "XRAYINT",
		"DEMO-9":              "DEMO",
		"NODASH":              "NODASH",
	}
	for in, want := range cases {
		if got := projectKeyOf(in); got != want {
			t.Errorf("projectKeyOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTraceabilityCrossProjectOnly(t *testing.T) {
	r := newTestRepo(t)
	const profileID = "p1"
	const projectKey = "DEMO"

	// Two executions: one in-project, one cross-project, each running one test.
	seedContainer(t, r, profileID, "DEMO-TE-1", "testexec", "Cycle 1", "Open")
	seedContainer(t, r, profileID, "XRAYINT-TE-1", "testexec", "Integration", "Open")
	seedContainerTest(t, r, profileID, "DEMO-TE-1", "DEMO-1", "PASS")
	seedContainerTest(t, r, profileID, "XRAYINT-TE-1", "DEMO-1", "FAIL")

	all, err := r.GetTraceabilitySankey(profileID, projectKey, nil, nil, false)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if !hasNode(all, "exec:DEMO-TE-1") || !hasNode(all, "exec:XRAYINT-TE-1") {
		t.Fatalf("unfiltered flow should contain both executions: %+v", all.Nodes)
	}

	cross, err := r.GetTraceabilitySankey(profileID, projectKey, nil, nil, true)
	if err != nil {
		t.Fatalf("cross: %v", err)
	}
	if hasNode(cross, "exec:DEMO-TE-1") {
		t.Errorf("cross-project-only should drop in-project execution DEMO-TE-1")
	}
	if !hasNode(cross, "exec:XRAYINT-TE-1") {
		t.Errorf("cross-project-only should keep cross-project execution XRAYINT-TE-1")
	}
}

func hasNode(s Sankey, id string) bool {
	for _, n := range s.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}
