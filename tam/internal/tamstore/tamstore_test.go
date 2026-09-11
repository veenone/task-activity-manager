package tamstore_test

import (
	"database/sql"
	"fmt"
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
// carries it, which is what the version 6 migration is for.
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

// openAtSprintKeyVersion writes a database at path recorded at version,
// with the sprint table in the shape versions 4 and 5 both left it: keyed
// by the sprint id alone, holding one board's copy of sprint 12. Neither
// version's own migration touches sprint, so one fixture covers a database
// rewound to either.
func openAtSprintKeyVersion(t *testing.T, path string, version int) {
	t.Helper()
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`DROP TABLE sprint`,
		oldSprintDDL,
		`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 12, 1, 'Sprint 12', 'active')`,
		fmt.Sprintf(`UPDATE meta SET value = '%d' WHERE key = 'schema_version'`, version),
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

func TestVersionSixMigrationRekeysSprintByBoard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	openAtSprintKeyVersion(t, path, 4)

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

// sprintDDLVersionSix is the sprint table as version 6 left it: keyed by
// board already, but before this plan's goal column existed. A developer
// database built from that version still carries this shape, which is what
// the version 7 migration is for.
const sprintDDLVersionSix = `CREATE TABLE sprint (
	profile_id TEXT NOT NULL,
	id         INTEGER NOT NULL,
	board_id   INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	state      TEXT NOT NULL DEFAULT '',
	start_date TEXT NOT NULL DEFAULT '',
	end_date   TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, board_id, id)
)`

// openAtVersionSix writes a version 6 database at path: sprint already keyed
// by board, holding one row, with no goal column yet.
func openAtVersionSix(t *testing.T, path string) {
	t.Helper()
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`DROP TABLE sprint`,
		sprintDDLVersionSix,
		`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date) VALUES ('p1', 12, 1, 'Sprint 12', 'active', '2026-08-18T09:00:00Z', '2026-09-01T09:00:00Z')`,
		`UPDATE meta SET value = '6' WHERE key = 'schema_version'`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestVersionSevenMigrationAddsGoalAndKeepsARowAlreadyKeyedByBoard is the
// plain case: sprint is already the shape version 6 left it, so the
// migration is a single ALTER TABLE ADD COLUMN and the row it was called for
// survives untouched.
func TestVersionSevenMigrationAddsGoalAndKeepsARowAlreadyKeyedByBoard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	openAtVersionSix(t, path)

	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
	var name, goal string
	if err := db.DB().QueryRow(`SELECT name, goal FROM sprint WHERE profile_id = 'p1' AND board_id = 1 AND id = 12`).Scan(&name, &goal); err != nil {
		t.Fatalf("read migrated sprint: %v", err)
	}
	if name != "Sprint 12" {
		t.Errorf("name = %q after the goal migration, want the row kept rather than rebuilt empty", name)
	}
	// Nothing backfills a sprint cached before this version: the sprint is
	// not part of an issue sync, and its only refresh is the Boards view's
	// own Refresh button, which this migration does not trigger.
	if goal != "" {
		t.Errorf("goal = %q, want empty until the next Boards Refresh fills it in", goal)
	}
}

// TestOldSprintKeyDatabasesReachVersionSevenWithTheGoalColumn covers what a
// database recorded at 4 or 5 goes through on its way to 7: migration 6
// drops sprint and recreates it from sprintDDL before migration 7 ever
// runs, and sprintDDL already carries the goal column, so migration 7's own
// ALTER TABLE lands on a column that is already there and takes the same
// duplicate-column no-op path a fresh install does. This does not pin
// migration 7 itself: deleting its Apply body still leaves migration 6
// supplying the column here. TestVersionSevenMigrationAddsGoalAndKeepsARowAlreadyKeyedByBoard,
// below, is the one test that depends on migration 7's own body.
func TestOldSprintKeyDatabasesReachVersionSevenWithTheGoalColumn(t *testing.T) {
	for _, version := range []int{4, 5} {
		path := filepath.Join(t.TempDir(), "tam.db")
		openAtSprintKeyVersion(t, path, version)

		db, err := tamstore.Open(path)
		if err != nil {
			t.Fatalf("version %d: reopen: %v", version, err)
		}
		if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
			t.Errorf("version %d: schema version = %d, want %d", version, v, tamstore.Schema.Version)
		}
		var rows int
		if err := db.DB().QueryRow(`SELECT count(*) FROM sprint`).Scan(&rows); err != nil {
			t.Fatalf("version %d: count sprints: %v", version, err)
		}
		if rows != 0 {
			t.Errorf("version %d: sprint rows after migrating = %d, want the cache emptied by migration 6, as it always is", version, rows)
		}
		if _, err := db.DB().Exec(
			`INSERT INTO sprint (profile_id, id, board_id, name, state, goal) VALUES ('p1', 12, 1, 'Sprint 12', 'active', 'Ship it')`,
		); err != nil {
			t.Errorf("version %d: insert naming the goal column: %v", version, err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("version %d: close: %v", version, err)
		}
	}
}

// TestFreshDatabaseHasTheSprintGoalColumn pins sprintDDL itself, not the
// version 7 migration. A plain fresh open cannot separate the two: a
// database created at version 0 runs every migration in turn, and version
// 7's own ALTER TABLE would supply the column even if sprintDDL never
// carried it. So this test records the newest schema version directly into
// a bare meta table before tamstore ever opens the file. apply's migration
// loop then finds every migration already satisfied (current is read once,
// already at or above every Version in the list) and runs none of them,
// which leaves Base, meaning baseDDL, sprintDDL, and journal.DDL, as the
// only statement that could have built the sprint table this insert relies
// on.
func TestFreshDatabaseHasTheSprintGoalColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw file: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		fmt.Sprintf(`INSERT INTO meta (key, value) VALUES ('schema_version', '%d')`, tamstore.Schema.Version),
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw file: %v", err)
	}

	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.DB().Exec(
		`INSERT INTO sprint (profile_id, id, board_id, name, state, goal) VALUES ('p1', 1, 1, 'Sprint 1', 'active', 'Ship it')`,
	); err != nil {
		t.Fatalf("insert with goal: %v", err)
	}
	var goal string
	if err := db.DB().QueryRow(`SELECT goal FROM sprint WHERE profile_id = 'p1' AND board_id = 1 AND id = 1`).Scan(&goal); err != nil {
		t.Fatalf("read goal: %v", err)
	}
	if goal != "Ship it" {
		t.Errorf("goal = %q, want what was inserted", goal)
	}
}

func TestSchemaVersionEightAddsTheReportTableToAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Turn the fresh database into a version 7 one: drop sprint_report,
	// which does not exist before this version at all, and rewind the
	// recorded version.
	for _, stmt := range []string{
		`DROP TABLE sprint_report`,
		`UPDATE meta SET value = '7' WHERE key = 'schema_version'`,
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
	if err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'sprint_report'`).Scan(&name); err != nil {
		t.Errorf("sprint_report after upgrade: %v", err)
	}
	// The upgrade records the current schema version, not the one the
	// table was added in, the same as the journal tables and the user
	// cache before it.
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
}

// sprintDDLVersionSeven is the sprint table as version 7 left it: keyed by
// board with a goal column, but before this plan's complete_date column
// existed. A developer database built from that version still carries this
// shape, which is what the version 8 migration's column add is for.
const sprintDDLVersionSeven = `CREATE TABLE sprint (
	profile_id TEXT NOT NULL,
	id         INTEGER NOT NULL,
	board_id   INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	state      TEXT NOT NULL DEFAULT '',
	start_date TEXT NOT NULL DEFAULT '',
	end_date   TEXT NOT NULL DEFAULT '',
	goal       TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, board_id, id)
)`

// openAtVersionSeven writes a version 7 database at path: sprint already
// keyed by board with a goal, holding one closed sprint's row, with no
// complete_date column yet.
func openAtVersionSeven(t *testing.T, path string) {
	t.Helper()
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, stmt := range []string{
		`DROP TABLE sprint`,
		sprintDDLVersionSeven,
		`INSERT INTO sprint (profile_id, id, board_id, name, state, start_date, end_date, goal) VALUES ('p1', 12, 1, 'Sprint 12', 'closed', '2026-08-18T09:00:00Z', '2026-09-01T09:00:00Z', 'Ship the promo code flow')`,
		`UPDATE meta SET value = '7' WHERE key = 'schema_version'`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestVersionEightMigrationAddsCompleteDateAndKeepsARowAlreadyCarryingAGoal
// is the plain case: sprint is already the shape version 7 left it, so the
// migration is a single ALTER TABLE ADD COLUMN and the row it was called
// for survives untouched.
func TestVersionEightMigrationAddsCompleteDateAndKeepsARowAlreadyCarryingAGoal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	openAtVersionSeven(t, path)

	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	if v, _ := store.ReadSchemaVersion(db.DB()); v != tamstore.Schema.Version {
		t.Errorf("schema version = %d, want %d", v, tamstore.Schema.Version)
	}
	var name, goal, completeDate string
	if err := db.DB().QueryRow(`SELECT name, goal, complete_date FROM sprint WHERE profile_id = 'p1' AND board_id = 1 AND id = 12`).Scan(&name, &goal, &completeDate); err != nil {
		t.Fatalf("read migrated sprint: %v", err)
	}
	if name != "Sprint 12" || goal != "Ship the promo code flow" {
		t.Errorf("name = %q goal = %q after the complete_date migration, want the row kept rather than rebuilt empty", name, goal)
	}
	// Nothing backfills a sprint cached before this version, the same as
	// goal before it: a sprint is not part of an issue sync, and its only
	// refresh is the Boards view's own Refresh button, which this
	// migration does not trigger.
	if completeDate != "" {
		t.Errorf("complete_date = %q, want empty until the next Boards Refresh fills it in", completeDate)
	}
}

// TestFreshDatabaseHasTheReportTableAndTheSprintCompleteDateColumn pins
// sprintReportDDL and sprintDDL themselves, the same way
// TestFreshDatabaseHasTheSprintGoalColumn pinned goal: a plain fresh open
// runs every migration in the list, so only inserting straight into both
// shapes proves Base itself carries them.
func TestFreshDatabaseHasTheReportTableAndTheSprintCompleteDateColumn(t *testing.T) {
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.DB().Exec(
		`INSERT INTO sprint (profile_id, id, board_id, name, state, complete_date) VALUES ('p1', 1, 1, 'Sprint 1', 'closed', '2026-08-21T10:00:00Z')`,
	); err != nil {
		t.Fatalf("insert sprint with complete_date: %v", err)
	}
	if _, err := db.DB().Exec(
		`INSERT INTO sprint_report (profile_id, board_id, sprint_id, unit, algo_version, built_at, series_json) VALUES ('p1', 1, 1, 'points', 1, '2026-09-10T00:00:00Z', '{}')`,
	); err != nil {
		t.Fatalf("insert sprint_report: %v", err)
	}
}
