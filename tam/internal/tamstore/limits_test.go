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

// Version 21 adds board_column_limit, the limit a user sets in TAM for a
// column Jira sets none on. It is a whole new table, so baseDDL creates it on
// the next open of an older file and there is no migration entry to run; what
// is worth asserting is that the table is there and keyed the way a reorder
// needs, on the column's name rather than on its position.
func TestSchemaVersionTwentyOneAddsTheOwnLimitTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if v, _ := store.ReadSchemaVersion(db.DB()); v != 21 {
		t.Errorf("schema version = %d, want 21, the version that adds the table below", v)
	}
	if _, err := db.DB().Exec(
		`INSERT INTO board_column_limit (profile_id, board_id, column_name, wip_max) VALUES ('p1', 1, 'In Progress', 5)`,
	); err != nil {
		t.Fatalf("write a limit of the user's own: %v", err)
	}
	// The same column at another position is the same row: a position keyed
	// table would take this as a second limit and a reorder would leave two.
	if _, err := db.DB().Exec(
		`INSERT INTO board_column_limit (profile_id, board_id, column_name, wip_max) VALUES ('p1', 1, 'In Progress', 9)`,
	); err == nil {
		t.Error("a second limit for the same column was accepted, want the column's name to be the key")
	}
	var max int
	if err := db.DB().QueryRow(
		`SELECT wip_max FROM board_column_limit WHERE profile_id = 'p1' AND board_id = 1 AND column_name = 'In Progress'`,
	).Scan(&max); err != nil {
		t.Fatalf("read the limit back: %v", err)
	}
	if max != 5 {
		t.Errorf("wip_max = %d, want the 5 that was written", max)
	}
}
