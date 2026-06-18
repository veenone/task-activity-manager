package testrepo

import "testing"

func TestExecutionsForPlans(t *testing.T) {
	r := newTestRepo(t)
	const p = "p1"

	seedContainer(t, r, p, "DEMO-TP-1", "testplan", "Plan 1", "Open")
	seedContainer(t, r, p, "DEMO-TP-2", "testplan", "Plan 2", "Open")
	seedContainer(t, r, p, "DEMO-TE-1", "testexec", "Exec 1", "Open")
	seedContainer(t, r, p, "DEMO-TE-2", "testexec", "Exec 2", "Open")

	// Plan 1 holds DEMO-1; Exec 1 runs DEMO-1; Exec 2 runs DEMO-2 (in Plan 2).
	seedContainerTest(t, r, p, "DEMO-TP-1", "DEMO-1", "")
	seedContainerTest(t, r, p, "DEMO-TP-2", "DEMO-2", "")
	seedContainerTest(t, r, p, "DEMO-TE-1", "DEMO-1", "PASS")
	seedContainerTest(t, r, p, "DEMO-TE-2", "DEMO-2", "FAIL")

	// For Plan 1 only Exec 1 shares a test.
	got, err := r.ExecutionsForPlans(p, []string{"DEMO-TP-1"})
	if err != nil {
		t.Fatalf("ExecutionsForPlans: %v", err)
	}
	if len(got) != 1 || got[0].Key != "DEMO-TE-1" {
		t.Fatalf("want [DEMO-TE-1], got %+v", got)
	}

	// Empty plan list returns all executions.
	all, err := r.ExecutionsForPlans(p, nil)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 executions, got %d", len(all))
	}
}
