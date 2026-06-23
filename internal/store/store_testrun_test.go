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
