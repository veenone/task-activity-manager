// Package tamstore is Task Activity Manager's own database: everything that
// is not a profile, connection, or global setting, because those live in the
// shared profiles.db. Version 1 carries no app tables. Version 2 adds the
// issue tables, version 3 the shared journal tables, version 4 the
// cached Jira user list, version 5 the board tables and the issue's status
// id, version 6 re-keys the sprint table by board, and version 7 adds the
// sprint's goal.
package tamstore

import (
	"database/sql"
	"os"
	"path/filepath"

	"agile-suite/core/journal"
	"agile-suite/core/store"
)

// Schema is TAM's local schema. Version 3 adds the shared journal tables
// and version 4 the cached Jira user list behind the assignee picker;
// both are plain CREATE TABLE IF NOT EXISTS, so an older database picks
// them up on its next open. Version 5 adds the board tables and the
// issue's status id, and that one needs the migration below, because
// CREATE TABLE IF NOT EXISTS cannot add a column to a table that is
// already there. Version 6 re-keys sprint by board, which needs a
// migration for the same reason: CREATE TABLE IF NOT EXISTS leaves a table
// that already exists exactly as it is, primary key included. Version 7
// adds the sprint's goal, the same shape as version 5's column add.
var Schema = store.Schema{
	Version: 7,
	Base:    baseDDL + sprintDDL + journal.DDL,
	Migrations: []store.Migration{{
		Version: 5,
		// SQLite has no ADD COLUMN IF NOT EXISTS, and a database created
		// fresh at version 5 already has the column from baseDDL, so a
		// duplicate-column error here is the expected no-op.
		Apply: func(db *sql.DB) error {
			// store.AddColumnIfMissing already treats "duplicate column" as
			// success, which is what a fresh version 5 file needs, since
			// baseDDL gave it the column a moment ago.
			if err := store.AddColumnIfMissing(db, "issue", "status_id TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			// The board matches cards to columns by status id, and an
			// incremental sync only re-reads issues Jira says changed, so
			// rows cached before version 5 would never get one. Clearing
			// the watermark drops the `updated >=` filter from the next
			// sync, which refetches every issue and fills the column. It
			// is not the same as a full sync: nothing is purged first,
			// and nothing here should be made to purge.
			//
			// issuerepo.ResetSyncCursor does this for one profile and
			// stays the way application code does it; a migration has no
			// profile in hand, so it clears them all in one statement.
			_, err := db.Exec(`UPDATE sync_state SET last_synced = ''`)
			return err
		},
	}, {
		Version: 6,
		// sprint was keyed (profile_id, id) through version 5, but the
		// boards sync clears and writes sprints one board at a time and
		// Jira Data Center hands the same sprint to every board whose
		// filter reaches it. Two scrum boards over one project therefore
		// collided on the second board's insert and took the whole pass
		// down. SQLite cannot change a primary key in place, and the table
		// is a cache the next sync refills with nothing joining to its
		// rows, so dropping it costs one sync and nothing else. A database
		// created fresh at version 6 gets the new key from baseDDL and
		// drops an empty table here.
		Apply: func(db *sql.DB) error {
			if _, err := db.Exec(`DROP TABLE IF EXISTS sprint`); err != nil {
				return err
			}
			_, err := db.Exec(sprintDDL)
			return err
		},
	}, {
		Version: 7,
		// Jira has sent a sprint's goal on every read since the boards work
		// landed in Phase 3a; nothing between here and the wire kept it.
		// This follows version 5's shape rather than version 6's: a plain
		// column add costs nothing a drop and recreate would also cost,
		// and dropping sprint here would empty every board's sprint picker
		// until the user next presses Refresh in the Boards view. On a
		// database still at version 5, migration 6 runs first and rebuilds
		// sprint from sprintDDL, which by then already carries this
		// column, so this add lands as the duplicate-column no-op
		// AddColumnIfMissing treats as success.
		//
		// Unlike version 5's status id, nothing here clears a watermark to
		// backfill the column: a sprint is not read by an issue sync, and
		// its only refresh is a board's own Refresh button. Every sprint
		// cached before this version keeps an empty goal until the next
		// Boards Refresh rewrites it, and that gap is deliberate, not an
		// oversight left for later.
		Apply: func(db *sql.DB) error {
			return store.AddColumnIfMissing(db, "sprint", "goal TEXT NOT NULL DEFAULT ''")
		},
	}},
	Indexes: indexDDL,
}

const baseDDL = `
CREATE TABLE IF NOT EXISTS issue (
	profile_id        TEXT NOT NULL,
	key               TEXT NOT NULL,
	id                TEXT NOT NULL DEFAULT '',
	project           TEXT NOT NULL DEFAULT '',
	type              TEXT NOT NULL DEFAULT '',
	summary           TEXT NOT NULL DEFAULT '',
	status            TEXT NOT NULL DEFAULT '',
	status_id         TEXT NOT NULL DEFAULT '',
	assignee          TEXT NOT NULL DEFAULT '',
	reporter          TEXT NOT NULL DEFAULT '',
	priority          TEXT NOT NULL DEFAULT '',
	labels            TEXT NOT NULL DEFAULT '[]',
	sprint_id         TEXT NOT NULL DEFAULT '',
	sprint_name       TEXT NOT NULL DEFAULT '',
	parent_key        TEXT NOT NULL DEFAULT '',
	story_points      REAL,
	rank              TEXT NOT NULL DEFAULT '',
	created           TEXT NOT NULL DEFAULT '',
	updated           TEXT NOT NULL DEFAULT '',
	synced_at         TEXT NOT NULL DEFAULT '',
	detail_json       TEXT,
	detail_fetched_at TEXT,
	PRIMARY KEY (profile_id, key)
);
CREATE TABLE IF NOT EXISTS issue_link (
	profile_id TEXT NOT NULL,
	from_key   TEXT NOT NULL,
	to_key     TEXT NOT NULL,
	link_type  TEXT NOT NULL,
	direction  TEXT NOT NULL,
	to_summary TEXT NOT NULL DEFAULT '',
	to_type    TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, from_key, to_key, link_type, direction)
);
CREATE TABLE IF NOT EXISTS sync_state (
	profile_id  TEXT PRIMARY KEY,
	last_synced TEXT NOT NULL DEFAULT '',
	last_full   TEXT NOT NULL DEFAULT '',
	last_error  TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS profile_setting (
	profile_id TEXT NOT NULL,
	key        TEXT NOT NULL,
	value      TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, key)
);
-- The people who can be assigned an issue on this profile's instance, so the
-- assignee picker answers a keystroke from disk instead of a round trip, and
-- still answers at all when Jira cannot be reached. name is the username the
-- write path sends; display_name is what the user reads.
CREATE TABLE IF NOT EXISTS jira_user (
	profile_id   TEXT NOT NULL,
	name         TEXT NOT NULL,
	display_name TEXT NOT NULL DEFAULT '',
	cached_at    TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, name)
);
CREATE TABLE IF NOT EXISTS board (
	profile_id TEXT NOT NULL,
	id         INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	type       TEXT NOT NULL DEFAULT '',
	synced_at  TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, id)
);
CREATE TABLE IF NOT EXISTS board_column (
	profile_id TEXT NOT NULL,
	board_id   INTEGER NOT NULL,
	position   INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	status_ids TEXT NOT NULL DEFAULT '[]',
	PRIMARY KEY (profile_id, board_id, position)
);
CREATE TABLE IF NOT EXISTS board_issue (
	profile_id TEXT NOT NULL,
	board_id   INTEGER NOT NULL,
	sprint_id  TEXT NOT NULL DEFAULT '',
	key        TEXT NOT NULL,
	position   INTEGER NOT NULL,
	PRIMARY KEY (profile_id, board_id, sprint_id, key)
);`

// sprintDDL is its own statement because the version 6 migration recreates
// the table with it, and a second copy of the columns is how the two would
// drift apart. The key carries board_id: one Jira sprint belongs to every
// board whose filter reaches it, and each board caches its own copy.
const sprintDDL = `
CREATE TABLE IF NOT EXISTS sprint (
	profile_id TEXT NOT NULL,
	id         INTEGER NOT NULL,
	board_id   INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	state      TEXT NOT NULL DEFAULT '',
	start_date TEXT NOT NULL DEFAULT '',
	end_date   TEXT NOT NULL DEFAULT '',
	goal       TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, board_id, id)
);`

const indexDDL = `
CREATE INDEX IF NOT EXISTS issue_profile_type   ON issue (profile_id, type);
CREATE INDEX IF NOT EXISTS issue_profile_sprint ON issue (profile_id, sprint_id);
CREATE INDEX IF NOT EXISTS jira_user_profile_display ON jira_user (profile_id, display_name);
CREATE INDEX IF NOT EXISTS board_issue_lookup    ON board_issue (profile_id, board_id, sprint_id);`

// Open opens (or creates) TAM's database at path.
func Open(path string) (*store.DB, error) { return store.Open(path, Schema) }

// DefaultDir is <user config dir>/task-activity-manager, created if missing.
// The log file lives there too.
func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "task-activity-manager")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	return appDir, nil
}

// DefaultPath is DefaultDir joined with tam.db.
func DefaultPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tam.db"), nil
}
