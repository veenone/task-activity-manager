package testrepo

import "testing"

func TestGetSubTaskTraceability(t *testing.T) {
	r := newTestRepo(t) // shared helper in sankey_crossproject_test.go
	const p = "p1"

	// Two sub-task execs under one parent, one standalone exec (excluded).
	seedContainer(t, r, p, "DEMO-STE-1", "testexec", "Sub 1", "Open")
	seedContainer(t, r, p, "DEMO-STE-2", "testexec", "Sub 2", "Open")
	seedContainer(t, r, p, "DEMO-TE-9", "testexec", "Standalone", "Open")
	setContainerParent(t, r, p, "DEMO-STE-1", "DEMO-S-1")
	setContainerParent(t, r, p, "DEMO-STE-2", "DEMO-S-1")
	// DEMO-TE-9 keeps parent_key = '' (standalone).

	seedContainerTest(t, r, p, "DEMO-STE-1", "DEMO-1", "PASS")
	seedContainerTest(t, r, p, "DEMO-STE-1", "DEMO-2", "FAIL")
	seedContainerTest(t, r, p, "DEMO-STE-2", "DEMO-3", "PASS")
	seedContainerTest(t, r, p, "DEMO-TE-9", "DEMO-4", "PASS") // standalone, excluded

	sk, err := r.GetSubTaskTraceability(p, nil)
	if err != nil {
		t.Fatalf("GetSubTaskTraceability: %v", err)
	}

	// 3 memberships under sub-task execs (the standalone one is excluded).
	sumLayer := func(layer int) int {
		n := 0
		for _, nd := range sk.Nodes {
			if nd.Layer == layer {
				n += nd.Value
			}
		}
		return n
	}
	if sumLayer(0) != 3 || sumLayer(1) != 3 || sumLayer(2) != 3 {
		t.Fatalf("layers should each total 3 memberships, got %d/%d/%d", sumLayer(0), sumLayer(1), sumLayer(2))
	}
	if !hasNode(sk, "parent:DEMO-S-1") {
		t.Errorf("missing parent node")
	}
	if hasNode(sk, "exec:DEMO-TE-9") {
		t.Errorf("standalone execution must be excluded")
	}

	// Parent filter to a non-existent parent yields an empty (not error) result.
	empty, err := r.GetSubTaskTraceability(p, []string{"NOPE-1"})
	if err != nil {
		t.Fatalf("filtered: %v", err)
	}
	if len(empty.Nodes) != 0 {
		t.Errorf("unknown parent filter should yield no nodes, got %d", len(empty.Nodes))
	}
}

// setContainerParent sets parent_key on a seeded container.
func setContainerParent(t *testing.T, r *Repository, profileID, key, parent string) {
	t.Helper()
	if _, err := r.db.Exec(
		`UPDATE test_container SET parent_key = ? WHERE profile_id = ? AND jira_key = ?`,
		parent, profileID, key); err != nil {
		t.Fatalf("set parent on %s: %v", key, err)
	}
}
