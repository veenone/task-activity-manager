package sharedmigrate_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"agile-suite/core/shareddb"
	"agile-suite/xtm/internal/sharedmigrate"
	"agile-suite/xtm/internal/store"
)

// openBoth opens a fresh XTM store (the source) and a fresh shared database
// (the target) in the test's temp dir and returns their handles.
func openBoth(t *testing.T) (src, dst *sql.DB) {
	t.Helper()
	xtm, err := store.Open(filepath.Join(t.TempDir(), "xtm.db"))
	if err != nil {
		t.Fatal(err)
	}
	shared, err := shareddb.Open(filepath.Join(t.TempDir(), "profiles.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = xtm.Close(); _ = shared.Close() })
	return xtm.DB(), shared.DB()
}

func mustExec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestImportCopiesRowsOnceAndKeepsExistingTargets(t *testing.T) {
	src, dst := openBoth(t)
	mustExec(t, src, `INSERT INTO profiles (id, name, jira_url, project_key, created_at, bug_issue_type) VALUES ('p1', 'One', 'https://j', 'ONE', '2026-01-01T00:00:00Z', 'Defect')`)
	mustExec(t, src, `INSERT INTO connection (id, workspace_id, name, url) VALUES ('p1', 'p1', 'One', 'https://j')`)
	mustExec(t, src, `INSERT INTO app_setting (key, value) VALUES ('theme', 'dark')`)
	// A row already in the shared file must win over the copy.
	mustExec(t, dst, `INSERT INTO app_setting (key, value) VALUES ('theme', 'light')`)

	if err := sharedmigrate.ImportFromStore(src, dst); err != nil {
		t.Fatalf("first import: %v", err)
	}
	if got := count(t, dst, "profiles"); got != 1 {
		t.Fatalf("profiles copied = %d; want 1", got)
	}
	if got := count(t, dst, "connection"); got != 1 {
		t.Fatalf("connections copied = %d; want 1", got)
	}
	var issueType, mode, url string
	if err := dst.QueryRow(`SELECT bug_issue_type, bug_project_mode, jira_url FROM profiles WHERE id = 'p1'`).Scan(&issueType, &mode, &url); err != nil {
		t.Fatalf("read copied profile: %v", err)
	}
	if issueType != "Defect" || mode != "test" || url != "https://j" {
		t.Fatalf("copied profile values = %q/%q/%q; want Defect/test/https://j", issueType, mode, url)
	}
	var theme string
	if err := dst.QueryRow(`SELECT value FROM app_setting WHERE key = 'theme'`).Scan(&theme); err != nil || theme != "light" {
		t.Fatalf("theme = %q, %v; the existing shared value must be kept", theme, err)
	}

	// A profile added to the old store afterwards must NOT be copied: the
	// import is a one-time move, not a sync.
	mustExec(t, src, `INSERT INTO profiles (id, name, jira_url, project_key, created_at) VALUES ('p2', 'Two', 'https://j', 'TWO', '2026-01-02T00:00:00Z')`)
	if err := sharedmigrate.ImportFromStore(src, dst); err != nil {
		t.Fatalf("second import: %v", err)
	}
	if got := count(t, dst, "profiles"); got != 1 {
		t.Fatalf("second import copied again: profiles = %d; want 1", got)
	}
}
