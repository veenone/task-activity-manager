package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrationV54CreatesDatasetCatalogTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dataset-catalog.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create meta: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO meta (key, value) VALUES ('schema_version', '53')`); err != nil {
		t.Fatalf("stamp schema: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw database: %v", err)
	}

	st, err := Open(path)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	defer st.Close()

	statements := []string{
		`INSERT INTO dataset_catalog_state (profile_id, active_generation, last_synced_at) VALUES ('p1', 1, '2026-10-18T00:00:00Z')`,
		`INSERT INTO dataset_catalog_test (profile_id, generation, test_key, summary, project_key) VALUES ('p1', 1, 'QA-1', 'Test', 'QA')`,
		`INSERT INTO dataset_catalog_definition (profile_id, generation, test_key, scope_kind, context_key, parameters_json, rows_json) VALUES ('p1', 1, 'QA-1', 'test', '', '[]', '[]')`,
		`INSERT INTO dataset_catalog_usage (profile_id, generation, test_key, container_kind, container_key) VALUES ('p1', 1, 'QA-1', 'testplan', 'QA-PLAN-1')`,
		`INSERT INTO dataset_catalog_exec_plan (profile_id, generation, exec_key, plan_key) VALUES ('p1', 1, 'QA-EXEC-1', 'QA-PLAN-1')`,
	}
	for _, statement := range statements {
		if _, err := st.DB().Exec(statement); err != nil {
			t.Fatalf("insert catalog row: %v", err)
		}
	}
	if SchemaVersion() < 54 {
		t.Fatalf("SchemaVersion() = %d, want >= 54", SchemaVersion())
	}
}
