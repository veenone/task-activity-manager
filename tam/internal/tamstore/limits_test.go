package tamstore_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/tamstore"
)

// Version 20 follows version 15's shape, a plain column add with nothing to
// backfill, and what is worth asserting is how a board cached before it
// reads. A boards pass replaces a board's columns wholesale, so the next
// sync fills the limits in; until then the two limit columns are NULL,
// which says this board has no limit stored rather than that every column's
// limit is zero.
func TestSchemaVersionTwentyAddsTheColumnLimitsToAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE board_column DROP COLUMN wip_min`,
		`ALTER TABLE board_column DROP COLUMN wip_max`,
		`ALTER TABLE board_column DROP COLUMN constraint_type`,
		`INSERT INTO board_column (profile_id, board_id, position, name, status_ids) VALUES ('p1', 1, 0, 'In Progress', '["3"]')`,
		`UPDATE meta SET value = '19' WHERE key = 'schema_version'`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	_ = db.Close()

	db, err = tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	var (
		name       string
		low, high  sql.NullInt64
		constraint string
	)
	if err := db.DB().QueryRow(
		`SELECT name, wip_min, wip_max, constraint_type FROM board_column WHERE profile_id = 'p1' AND board_id = 1 AND position = 0`,
	).Scan(&name, &low, &high, &constraint); err != nil {
		t.Fatalf("read the kept column: %v", err)
	}
	if name != "In Progress" || low.Valid || high.Valid || constraint != "" {
		t.Errorf("column = %q min %v max %v constraint %q, want the row kept with no limit stored yet", name, low, high, constraint)
	}
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
}
