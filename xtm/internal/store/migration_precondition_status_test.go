package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrationAddsPreconditionStatus covers the upgrade path for #159: a
// database written before schema 51 has a precondition table with no status
// column, and the sync writes one on its next pass. Without the ALTER the
// insert fails with "table precondition has no column named status" and the
// whole precondition stage errors.
//
// The stored schema_version is deliberately already at the current version,
// the collision case TestMigrationAddsRequirementColumnsOnVersionCollision
// documents: the ALTER has to run on the table's actual shape rather than on
// what the version number claims.
func TestMigrationAddsPreconditionStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "precond-status.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	seed := []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`INSERT INTO meta (key, value) VALUES ('schema_version', '50')`,
		`CREATE TABLE precondition (
			profile_id TEXT NOT NULL, jira_key TEXT NOT NULL, summary TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
			condition TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (profile_id, jira_key))`,
		`INSERT INTO precondition (profile_id, jira_key, summary)
		   VALUES ('p1', 'PRE-1', 'card present')`,
	}
	for _, s := range seed {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed exec: %v", err)
		}
	}
	raw.Close()

	st, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// The row that predates the column reads as empty rather than failing,
	// which is what the view shows until the next sync fills it.
	var existing string
	if err := st.DB().QueryRow(
		`SELECT status FROM precondition WHERE profile_id = 'p1' AND jira_key = 'PRE-1'`,
	).Scan(&existing); err != nil {
		t.Fatalf("read migrated row: %v", err)
	}
	if existing != "" {
		t.Errorf("pre-migration row status = %q, want empty until the next sync", existing)
	}

	// And the sync's upsert can write one.
	if _, err := st.DB().Exec(
		`INSERT INTO precondition (profile_id, jira_key, summary, type, description, condition, status)
		 VALUES ('p1', 'PRE-2', 'card absent', 'Manual', 'd', 'Given x', 'Approved')`,
	); err != nil {
		t.Fatalf("insert precondition with status after migration: %v", err)
	}
	var written string
	if err := st.DB().QueryRow(
		`SELECT status FROM precondition WHERE profile_id = 'p1' AND jira_key = 'PRE-2'`,
	).Scan(&written); err != nil {
		t.Fatalf("read written row: %v", err)
	}
	if written != "Approved" {
		t.Errorf("status = %q, want Approved", written)
	}
}
