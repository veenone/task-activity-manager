package tamstore_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/tamstore"
)

// Version 22 adds done_agreement_tick, one row per item of the done
// agreement a reader has ticked for one issue. A whole new table, so baseDDL
// creates it on the next open of an older file and there is no migration
// entry to run, the way board_column_limit arrived at version 21. What is
// worth asserting is that the table is there and keyed the way an item's
// identity needs: on the item's own text, because a Confluence task carries
// nothing else that survives an edit of the list.
func TestSchemaVersionTwentyTwoAddsTheDoneAgreementTickTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	// At least 22, the version that adds the table below, rather than exactly
	// 22: version 21's own test pinned the number and failed the moment this
	// version arrived, which said nothing about the table it was written for.
	if v, _ := store.ReadSchemaVersion(db.DB()); v < 22 {
		t.Errorf("schema version = %d, want at least 22, the version that adds the table below", v)
	}
	if _, err := db.DB().Exec(
		`INSERT INTO done_agreement_tick (profile_id, board_id, issue_key, item_text) VALUES ('p1', 1, 'PLAT-1', 'Unit tests pass')`,
	); err != nil {
		t.Fatalf("write a tick: %v", err)
	}
	// The row is the tick, so writing it twice is the same tick: a second
	// row for the same words would be a second box drawn for one item.
	if _, err := db.DB().Exec(
		`INSERT INTO done_agreement_tick (profile_id, board_id, issue_key, item_text) VALUES ('p1', 1, 'PLAT-1', 'Unit tests pass')`,
	); err == nil {
		t.Error("a second tick for the same item was accepted, want the item's text to be part of the key")
	}
	var ticks int
	if err := db.DB().QueryRow(
		`SELECT count(*) FROM done_agreement_tick WHERE profile_id = 'p1' AND board_id = 1 AND issue_key = 'PLAT-1'`,
	).Scan(&ticks); err != nil {
		t.Fatalf("read the tick back: %v", err)
	}
	if ticks != 1 {
		t.Errorf("ticks = %d, want the one that was written", ticks)
	}
}
