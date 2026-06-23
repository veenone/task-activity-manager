package store

import "testing"

func TestSchemaHasTestRunAndExecPlan(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	for _, tbl := range []string{"test_run", "exec_plan"} {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil || name != tbl {
			t.Fatalf("expected table %q to exist, err=%v name=%q", tbl, err, name)
		}
	}
	if schemaVersion < 28 {
		t.Fatalf("schemaVersion = %d, want >= 28", schemaVersion)
	}
}

func TestReplaceRunsForExecAndRead(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	runs := []TestRunRow{{
		ExecKey: "DEMO-EXEC-1", TestKey: "DEMO-1", RunStatus: "FAIL",
		StartedAt: "2026-06-01T10:00:00Z", FinishedAt: "2026-06-01T10:05:00Z",
		ExecutedBy: "achmarah", Environment: "Staging", Defects: `["DEMO-9"]`,
	}}
	if err := s.ReplaceRunsForExec("p1", "DEMO-EXEC-1", runs); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertExecPlan("p1", "DEMO-EXEC-1", "DEMO-TP-1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.RunsForTest("p1", "DEMO-1")
	if err != nil || len(got) != 1 || got[0].RunStatus != "FAIL" {
		t.Fatalf("RunsForTest = %+v, err=%v", got, err)
	}
}
