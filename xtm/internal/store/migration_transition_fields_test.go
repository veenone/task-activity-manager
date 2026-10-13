package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrationAddsTransitionFields covers #164's upgrade path: a database
// from before schema 52 has a pending_change table with no transition_fields
// column. The migration adds it without a resync, a queued row survives with
// the column NULL, and a transition queued afterwards can store its fields.
func TestMigrationAddsTransitionFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transition-fields.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	seed := []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`INSERT INTO meta (key, value) VALUES ('schema_version', '51')`,
		`CREATE TABLE pending_change (
			id INTEGER PRIMARY KEY AUTOINCREMENT, profile_id TEXT NOT NULL,
			entity_type TEXT NOT NULL, entity_key TEXT NOT NULL, field TEXT NOT NULL,
			before_val TEXT NOT NULL DEFAULT '', after_val TEXT NOT NULL DEFAULT '',
			base_version TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL,
			UNIQUE (profile_id, entity_type, entity_key, field))`,
		`INSERT INTO pending_change (profile_id, entity_type, entity_key, field, before_val, after_val, created_at)
		   VALUES ('p1', 'test_case', 'QA-1', 'status', 'Open', 'Done', '2026-10-01T00:00:00Z')`,
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

	var existing sql.NullString
	if err := st.DB().QueryRow(
		`SELECT transition_fields FROM pending_change WHERE entity_key = 'QA-1'`,
	).Scan(&existing); err != nil {
		t.Fatalf("read migrated row: %v", err)
	}
	if existing.Valid {
		t.Errorf("pre-migration row transition_fields = %q, want NULL", existing.String)
	}
	if _, err := st.DB().Exec(
		`UPDATE pending_change SET transition_fields = '{"resolution":{"id":"1"}}' WHERE entity_key = 'QA-1'`,
	); err != nil {
		t.Fatalf("write transition_fields after migration: %v", err)
	}
	if SchemaVersion() < 52 {
		t.Errorf("schema version %d, want at least 52", SchemaVersion())
	}
}
