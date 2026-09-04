package shareddb_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/shareddb"
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

func TestDefaultPathIsUnderTheSuiteDir(t *testing.T) {
	p, err := shareddb.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "profiles.db" || filepath.Base(filepath.Dir(p)) != "agile-suite" {
		t.Fatalf("unexpected path %q", p)
	}
}
