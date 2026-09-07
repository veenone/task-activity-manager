package tamstore_test

import (
	"database/sql"
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

// rewindToVersionThree puts the database back where the version 5 migration
// has work to do again: the recorded version at 3 and a watermark to clear.
// The status_id column stays, because that is the half of the migration
// that has to be a no-op the second time round.
func rewindToVersionThree(t *testing.T, path string) {
	t.Helper()
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("rewind: %v", err)
	}
	for _, stmt := range []string{
		`UPDATE sync_state SET last_synced = '2026-09-06T09:00:00Z'`,
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

func TestVersionFiveMigrationIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	openAtVersionThree(t, path)
	// Without the rewind the second open would record version 5 and skip
	// the body, which proves nothing about running it twice.
	for i := 0; i < 3; i++ {
		statusID, lastSynced, _ := readMigrated(t, path)
		if statusID != "" || lastSynced != "" {
			t.Fatalf("run %d: status_id = %q, last_synced = %q; want both empty", i+1, statusID, lastSynced)
		}
		rewindToVersionThree(t, path)
	}
}

// oldSprintDDL is the sprint table as version 4 created it, keyed by the
// sprint id alone. A developer database built from that version still
// carries it, which is what the version 5 migration is for.
const oldSprintDDL = `CREATE TABLE sprint (
	profile_id TEXT NOT NULL,
	id         INTEGER NOT NULL,
	board_id   INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	state      TEXT NOT NULL DEFAULT '',
	start_date TEXT NOT NULL DEFAULT '',
	end_date   TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, id)
)`

// openAtVersionFour writes a version 4 database at path: the sprint table
// with its old key, holding one board's copy of sprint 12.
func openAtVersionFour(t *testing.T, path string) {
	t.Helper()
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`DROP TABLE sprint`,
		oldSprintDDL,
		`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 12, 1, 'Sprint 12', 'active')`,
		`UPDATE meta SET value = '4' WHERE key = 'schema_version'`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// insertSprintForBoard writes one board's copy of a sprint, the way the
// boards sync does.
func insertSprintForBoard(db *sql.DB, boardID int) error {
	_, err := db.Exec(
		`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 12, ?, 'Sprint 12', 'active')`,
		boardID)
	return err
}

func TestVersionFiveMigrationRekeysSprintByBoard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	openAtVersionFour(t, path)

	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
	// The table is a cache: the migration drops what it held, and the next
	// sync refills it.
	var rows int
	if err := db.DB().QueryRow(`SELECT count(*) FROM sprint`).Scan(&rows); err != nil {
		t.Fatalf("count sprints: %v", err)
	}
	if rows != 0 {
		t.Errorf("sprint rows after the migration = %d, want the cache emptied", rows)
	}
	// Two boards sharing sprint 12 is what the old key made impossible.
	for _, boardID := range []int{1, 2} {
		if err := insertSprintForBoard(db.DB(), boardID); err != nil {
			t.Fatalf("board %d's copy of sprint 12: %v", boardID, err)
		}
	}
	if err := insertSprintForBoard(db.DB(), 1); err == nil {
		t.Error("one board's second copy of the same sprint was accepted; the key still has to hold")
	}
}

func TestFreshDatabaseKeysSprintByBoard(t *testing.T) {
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for _, boardID := range []int{1, 2} {
		if err := insertSprintForBoard(db.DB(), boardID); err != nil {
			t.Fatalf("board %d's copy of sprint 12: %v", boardID, err)
		}
	}
}
