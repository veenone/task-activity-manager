package tamstore_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/tamstore"
)

// Version 27 gives issue_link Jira's link id, which removing a link needs
// (#612), and clears the watermark so the next sync refills every issue's
// links with their ids. The old shape is seeded, not created by the new DDL.
func TestVersionTwentySevenAddsTheLinkIDAndClearsEveryWatermark(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE issue_link DROP COLUMN link_id`,
		`INSERT INTO issue_link (profile_id, from_key, to_key, link_type, direction) VALUES ('p1', 'PLAT-1', 'PLAT-2', 'Blocks', 'outward')`,
		`INSERT INTO sync_state (profile_id, last_synced, last_full, last_error) VALUES ('p1', '2026-11-05T10:42:00Z', '2026-11-01T09:00:00Z', '')`,
		`UPDATE meta SET value = '26' WHERE key = 'schema_version'`,
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
	var id string
	if err := db.DB().QueryRow(`SELECT link_id FROM issue_link WHERE profile_id = 'p1' AND from_key = 'PLAT-1'`).Scan(&id); err != nil {
		t.Fatalf("read the kept link: %v", err)
	}
	if id != "" {
		t.Errorf("link_id = %q, want empty until a sync carries one", id)
	}
	var lastSynced, lastFull string
	if err := db.DB().QueryRow(`SELECT last_synced, last_full FROM sync_state WHERE profile_id = 'p1'`).Scan(&lastSynced, &lastFull); err != nil {
		t.Fatalf("read sync_state: %v", err)
	}
	if lastSynced != "" || lastFull != "2026-11-01T09:00:00Z" {
		t.Errorf("sync_state = %q, %q; want last_synced cleared and last_full kept", lastSynced, lastFull)
	}
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
}
