package shareddb_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/shareddb"
	"agile-suite/core/store"
)

func TestOpenCreatesTheSharedTables(t *testing.T) {
	d, err := shareddb.Open(filepath.Join(t.TempDir(), "profiles.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	for _, stmt := range []string{
		`INSERT INTO profiles (id, name, jira_url, project_key, created_at) VALUES ('p1', 'One', 'https://j', 'ONE', '2026-01-01T00:00:00Z')`,
		`INSERT INTO connection (id, workspace_id, name) VALUES ('c1', 'p1', 'One')`,
		`INSERT INTO app_setting (key, value) VALUES ('theme', 'dark')`,
	} {
		if _, err := d.DB().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// TestAnOlderDatabaseGainsTheReportsColumns opens a file written before the
// reports space and root existed and checks the migration adds them without
// disturbing what the profile already had.
func TestAnOlderDatabaseGainsTheReportsColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.db")
	older := store.Schema{Version: 1, Base: `
CREATE TABLE IF NOT EXISTS confluence_profile (
	profile_id TEXT PRIMARY KEY,
	base_url TEXT NOT NULL DEFAULT '',
	space_key TEXT NOT NULL DEFAULT '',
	root_page_id TEXT NOT NULL DEFAULT ''
);`}
	old, err := store.Open(path, older)
	if err != nil {
		t.Fatalf("open the older database: %v", err)
	}
	if _, err := old.DB().Exec(
		`INSERT INTO confluence_profile (profile_id, base_url, space_key, root_page_id) VALUES ('p1', 'https://c', 'TEAM', '42')`,
	); err != nil {
		t.Fatalf("write the older row: %v", err)
	}
	if err := old.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	d, err := shareddb.Open(path)
	if err != nil {
		t.Fatalf("open and migrate: %v", err)
	}
	defer d.Close()
	var space, root, reportsSpace, reportsRoot string
	if err := d.DB().QueryRow(
		`SELECT space_key, root_page_id, reports_space_key, reports_root_page_id FROM confluence_profile WHERE profile_id = 'p1'`,
	).Scan(&space, &root, &reportsSpace, &reportsRoot); err != nil {
		t.Fatalf("read the migrated row: %v", err)
	}
	if space != "TEAM" || root != "42" {
		t.Errorf("the rituals configuration changed: space %q, root %q", space, root)
	}
	if reportsSpace != "" || reportsRoot != "" {
		t.Errorf("reports configuration = %q / %q, want both empty so the profile publishes where it always did", reportsSpace, reportsRoot)
	}
}

func TestDefaultPathIsUnderTheSuiteDir(t *testing.T) {
	p, err := shareddb.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "profiles.db" || filepath.Base(filepath.Dir(p)) != "agile-suite" {
		t.Fatalf("unexpected path %q", p)
	}
}
