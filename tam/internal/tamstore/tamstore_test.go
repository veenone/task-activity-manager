package tamstore_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/tamstore"
)

func TestSchemaVersionThreeAddsTheJournalTablesToAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Turn the fresh database into a version 2 one: drop the journal tables
	// and rewind the recorded version.
	for _, stmt := range []string{
		`DROP TABLE pending_change`, `DROP TABLE audit_log`,
		`UPDATE meta SET value = '2' WHERE key = 'schema_version'`,
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
	for _, table := range []string{"issue", "pending_change", "audit_log"} {
		var name string
		if err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Errorf("table %s after upgrade: %v", table, err)
		}
	}
	// The upgrade records the current schema version, not the one the
	// tables were added in.
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
}

// The user cache arrives the same idempotent way the journal tables did: an
// older database picks it up when it is next opened.
func TestSchemaVersionFourAddsTheUserCacheToAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`DROP TABLE jira_user`,
		`UPDATE meta SET value = '3' WHERE key = 'schema_version'`,
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
	var name string
	if err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'jira_user'`).Scan(&name); err != nil {
		t.Errorf("jira_user after upgrade: %v", err)
	}
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
}

func TestOpenRecordsVersionOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	v, err := store.ReadSchemaVersion(db.DB())
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if v != tamstore.Schema.Version {
		t.Errorf("version = %d, want %d", v, tamstore.Schema.Version)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Reopening a current database is a no-op.
	again, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_ = again.Close()
}

func TestSchemaVersionTwoHasTheIssueTables(t *testing.T) {
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"issue", "issue_link", "sync_state", "profile_setting"} {
		var name string
		err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %s: %v", table, err)
		}
	}
}

func TestSchemaVersionFiveAddsTheBoardTablesToAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Turn the fresh database into a version 3 one: drop the board tables
	// and rewind the recorded version.
	for _, stmt := range []string{
		`DROP TABLE board`, `DROP TABLE board_column`, `DROP TABLE board_issue`, `DROP TABLE sprint`,
		`UPDATE meta SET value = '3' WHERE key = 'schema_version'`,
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
	for _, table := range []string{"board", "board_column", "board_issue", "sprint"} {
		var name string
		if err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Errorf("table %s after upgrade: %v", table, err)
		}
	}
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
}

// openAtVersionThree writes a version 3 database at path: an issue row with
// no status_id column and a sync_state row carrying a watermark.
func openAtVersionThree(t *testing.T, path string) {
	t.Helper()
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE issue DROP COLUMN status_id`,
		`INSERT INTO issue (profile_id, key, summary, status) VALUES ('p1', 'PLAT-412', 'Promo code', 'In Progress')`,
		`INSERT INTO sync_state (profile_id, last_synced, last_full, last_error) VALUES ('p1', '2026-09-05T10:42:00Z', '2026-09-01T09:00:00Z', '')`,
		`UPDATE meta SET value = '3' WHERE key = 'schema_version'`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// readMigrated reads back the two facts the version 5 migration is
// responsible for: the issue's status id and the profile's watermark.
func readMigrated(t *testing.T, path string) (statusID, lastSynced, lastFull string) {
	t.Helper()
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	if err := db.DB().QueryRow(`SELECT status_id FROM issue WHERE profile_id = 'p1' AND key = 'PLAT-412'`).Scan(&statusID); err != nil {
		t.Fatalf("read status_id: %v", err)
	}
	if err := db.DB().QueryRow(`SELECT last_synced, last_full FROM sync_state WHERE profile_id = 'p1'`).Scan(&lastSynced, &lastFull); err != nil {
		t.Fatalf("read sync_state: %v", err)
	}
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
	return statusID, lastSynced, lastFull
}

func TestVersionFiveMigrationAddsStatusIDAndClearsEveryWatermark(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	openAtVersionThree(t, path)

	statusID, lastSynced, lastFull := readMigrated(t, path)
	if statusID != "" {
		t.Errorf("status_id = %q, want empty for a row cached before version 5", statusID)
	}
	if lastSynced != "" {
		t.Errorf("last_synced = %q, want empty so the next sync refetches every issue", lastSynced)
	}
	if lastFull != "2026-09-01T09:00:00Z" {
		t.Errorf("last_full = %q, want the migration to leave it alone", lastFull)
	}
}

func TestVersionFiveMigrationIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	openAtVersionThree(t, path)
	for i := 0; i < 3; i++ {
		statusID, lastSynced, _ := readMigrated(t, path)
		if statusID != "" || lastSynced != "" {
			t.Fatalf("open %d: status_id = %q, last_synced = %q; want both empty", i+1, statusID, lastSynced)
		}
	}
}
