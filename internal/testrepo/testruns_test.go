package testrepo_test

import (
	"testing"

	"xray-test-manager/internal/testrepo"
)

func TestGetTestRunHistory(t *testing.T) {
	repo := newRepo(t)

	// Seed the test case.
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login"},
	}); err != nil {
		t.Fatalf("seed test: %v", err)
	}

	// Seed two execution containers: one with fix versions, one without.
	if err := repo.UpsertContainers("p1", []testrepo.Container{
		{Key: "QA-TE-1", Kind: "testexec", Summary: "Cycle 1", Status: "Done", FixVersions: []string{"1.2.0"}},
		{Key: "QA-TE-2", Kind: "testexec", Summary: "Cycle 2", Status: "Done"},
	}); err != nil {
		t.Fatalf("seed containers: %v", err)
	}

	// Seed test_run rows for QA-1 in each execution.
	if err := repo.ReplaceRunsForExec("p1", "QA-TE-1", []testrepo.TestRunRow{
		{ExecKey: "QA-TE-1", TestKey: "QA-1", RunStatus: "FAIL", FinishedAt: "2026-05-01T10:00:00Z", Environment: "Staging"},
	}); err != nil {
		t.Fatalf("seed runs for QA-TE-1: %v", err)
	}
	if err := repo.ReplaceRunsForExec("p1", "QA-TE-2", []testrepo.TestRunRow{
		{ExecKey: "QA-TE-2", TestKey: "QA-1", RunStatus: "PASS", FinishedAt: "2026-06-01T09:00:00Z"},
	}); err != nil {
		t.Fatalf("seed runs for QA-TE-2: %v", err)
	}

	// Link QA-TE-1 to a Test Plan.
	if err := repo.ReplaceExecPlans("p1", "QA-TE-1", []string{"QA-TP-1"}); err != nil {
		t.Fatalf("seed exec plans: %v", err)
	}

	hist, err := repo.GetTestRunHistory("p1", "QA-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 {
		t.Fatalf("expected 2 run history entries, got %d", len(hist))
	}

	// Newest finished_at should be first (QA-TE-2 finished 2026-06-01).
	first := hist[0]
	if first.ExecKey != "QA-TE-2" {
		t.Errorf("hist[0].ExecKey = %q, want QA-TE-2 (newest first)", first.ExecKey)
	}
	if first.RunStatus != "PASS" {
		t.Errorf("hist[0].RunStatus = %q, want PASS", first.RunStatus)
	}
	if first.ExecSummary != "Cycle 2" {
		t.Errorf("hist[0].ExecSummary = %q, want Cycle 2", first.ExecSummary)
	}
	if len(first.PlanKeys) != 0 {
		t.Errorf("hist[0].PlanKeys = %v, want empty", first.PlanKeys)
	}

	// Second entry: QA-TE-1 (finished 2026-05-01).
	second := hist[1]
	if second.ExecKey != "QA-TE-1" {
		t.Errorf("hist[1].ExecKey = %q, want QA-TE-1", second.ExecKey)
	}
	if second.RunStatus != "FAIL" {
		t.Errorf("hist[1].RunStatus = %q, want FAIL", second.RunStatus)
	}
	if second.Environment != "Staging" {
		t.Errorf("hist[1].Environment = %q, want Staging", second.Environment)
	}
	if second.ExecSummary != "Cycle 1" {
		t.Errorf("hist[1].ExecSummary = %q, want Cycle 1", second.ExecSummary)
	}
	if len(second.FixVersions) != 1 || second.FixVersions[0] != "1.2.0" {
		t.Errorf("hist[1].FixVersions = %v, want [1.2.0]", second.FixVersions)
	}
	if len(second.PlanKeys) != 1 || second.PlanKeys[0] != "QA-TP-1" {
		t.Errorf("hist[1].PlanKeys = %v, want [QA-TP-1]", second.PlanKeys)
	}
}
