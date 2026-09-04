package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

const widgetTable = `CREATE TABLE IF NOT EXISTS widget (id TEXT PRIMARY KEY);`

func openTemp(t *testing.T, s Schema) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "t.db"), s)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestOpenFreshRecordsVersionAndCreatesTables(t *testing.T) {
	d := openTemp(t, Schema{Version: 3, Base: widgetTable})
	v, err := ReadSchemaVersion(d.DB())
	if err != nil || v != 3 {
		t.Fatalf("version = %d, %v; want 3", v, err)
	}
	if _, err := d.DB().Exec(`INSERT INTO widget (id) VALUES ('a')`); err != nil {
		t.Fatalf("widget table missing: %v", err)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	s := Schema{Version: 1, Base: widgetTable}
	for i := 0; i < 2; i++ {
		d, err := Open(path, s)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		_ = d.Close()
	}
}

func TestMigrationsRunOnlyWhenBehind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	d, err := Open(path, Schema{Version: 1, Base: widgetTable})
	if err != nil {
		t.Fatal(err)
	}
	_ = d.Close()

	calls := 0
	v2 := Schema{Version: 2, Base: widgetTable, Migrations: []Migration{{
		Version: 2,
		Apply: func(db *sql.DB) error {
			calls++
			return AddColumnIfMissing(db, "widget", "colour TEXT NOT NULL DEFAULT ''")
		},
	}}}
	d, err = Open(path, v2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.DB().Exec(`INSERT INTO widget (id, colour) VALUES ('a', 'red')`); err != nil {
		t.Fatalf("colour column missing after migration: %v", err)
	}
	_ = d.Close()
	if calls != 1 {
		t.Fatalf("migration ran %d times on upgrade; want 1", calls)
	}

	d, err = Open(path, v2)
	if err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	if calls != 1 {
		t.Fatalf("migration re-ran on an up-to-date database (%d calls)", calls)
	}
}

func TestIndexesApplyAfterMigrations(t *testing.T) {
	// The index references a column only the migration adds, which is the
	// whole reason indexes run last.
	path := filepath.Join(t.TempDir(), "t.db")
	d, err := Open(path, Schema{Version: 1, Base: widgetTable})
	if err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	s := Schema{
		Version: 2,
		Base:    widgetTable,
		Migrations: []Migration{{Version: 2, Apply: func(db *sql.DB) error {
			return AddColumnIfMissing(db, "widget", "colour TEXT NOT NULL DEFAULT ''")
		}}},
		Indexes: `CREATE INDEX IF NOT EXISTS idx_widget_colour ON widget (colour);`,
	}
	d, err = Open(path, s)
	if err != nil {
		t.Fatalf("open with index: %v", err)
	}
	_ = d.Close()
}

func TestAddColumnIfMissingIsIdempotent(t *testing.T) {
	d := openTemp(t, Schema{Version: 1, Base: widgetTable})
	for i := 0; i < 2; i++ {
		if err := AddColumnIfMissing(d.DB(), "widget", "colour TEXT NOT NULL DEFAULT ''"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}
