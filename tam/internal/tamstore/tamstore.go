// Package tamstore is Task Activity Manager's own database: everything that
// is not a profile, connection, or global setting, because those live in the
// shared profiles.db. Version 1 carries no app tables. Version 2 adds the
// issue tables, version 3 the shared journal tables, version 4 the
// cached Jira user list, version 5 the board tables and the issue's status
// id, version 6 re-keys the sprint table by board, version 7 adds the
// sprint's goal, version 8 adds the sprint_report table and the sprint's
// complete_date, and version 9 adds local ritual documents.
package tamstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
// Version 8 adds two things: sprint_report, a plain CREATE TABLE IF NOT
// EXISTS that needs no migration entry at all, and the sprint's
// complete_date, a column add in version 7's own shape.
// Version 9 adds ritual_document through the same idempotent base DDL path
// as sprint_report.
var Schema = store.Schema{
	Version: 11,
	Base:    baseDDL + sprintDDL + sprintReportDDL + ritualDocumentDDL + journal.DDL,
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
		// Boards Refresh rewrites it.
		Apply: func(db *sql.DB) error {
			return store.AddColumnIfMissing(db, "sprint", "goal TEXT NOT NULL DEFAULT ''")
		},
	}, {
		Version: 8,
		// sprint_report is a brand new table, so it needs nothing here:
		// sprintReportDDL is already part of Base, and CREATE TABLE IF NOT
		// EXISTS picks it up on an older database's next open the same way
		// the journal tables and the user cache did at versions 3 and 4.
		// complete_date is the one piece of this version that does need a
		// migration entry, for the same reason goal did at version 7: a
		// table that already exists is not touched by CREATE TABLE IF NOT
		// EXISTS, primary key or columns alike. On a database still behind
		// version 6, migration 6 runs first and rebuilds sprint from
		// sprintDDL, which by then already carries this column, so this add
		// lands as the duplicate-column no-op AddColumnIfMissing treats as
		// success.
		//
		// Nothing here backfills a sprint cached before this version, the
		// same as goal before it: a sprint is not read by an issue sync, so
		// complete_date stays empty until the Boards view's own Refresh
		// rewrites it.
		Apply: func(db *sql.DB) error {
			return store.AddColumnIfMissing(db, "sprint", "complete_date TEXT NOT NULL DEFAULT ''")
		},
	}, {
		Version: 10,
		// ritual_document arrived whole at version 9, so Base created it and
		// no migration was needed. issues_json is a column on a table that now
		// exists, which CREATE TABLE IF NOT EXISTS will not touch, so it needs
		// the same treatment goal and complete_date got at versions 7 and 8.
		//
		// The old issue_keys_json is left in place and unread rather than
		// dropped: SQLite makes column removal a table rebuild, and a rebuild
		// here would risk a user's unpublished drafts to reclaim nothing.
		Apply: func(db *sql.DB) error {
			if err := store.AddColumnIfMissing(db, "ritual_document", "issues_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
				return err
			}
			return convertRitualIssueKeys(db)
		},
	}, {
		Version: 11,
		// Some development builds recorded v10 before adding issues_json.
		// Repair those files without restoring deliberately cleared selections
		// in healthy v10 databases, where the legacy keys may still be present.
		Apply: func(db *sql.DB) error {
			var exists int
			if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('ritual_document') WHERE name = 'issues_json'`).Scan(&exists); err != nil {
				return fmt.Errorf("check ritual issue column: %w", err)
			}
			if exists != 0 {
				return nil
			}
			if err := store.AddColumnIfMissing(db, "ritual_document", "issues_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
				return err
			}
			return convertRitualIssueKeys(db)
		},
	}},
	Indexes: indexDDL,
}

// convertRitualIssueKeys rewrites version 9's bare key array into version 10's
// objects, so a draft written before per-issue remarks existed keeps its issues
// and their order. Only rows that have not already been converted are touched.
func convertRitualIssueKeys(db *sql.DB) error {
	rows, err := db.Query(`SELECT profile_id, board_id, sprint_id, ritual_type, issue_keys_json
		FROM ritual_document WHERE issues_json = '[]' AND issue_keys_json <> '[]'`)
	if err != nil {
		return fmt.Errorf("read ritual issue keys: %w", err)
	}
	type target struct {
		profileID  string
		boardID    int
		sprintID   int
		ritualType string
		issuesJSON string
	}
	var targets []target
	for rows.Next() {
		var t target
		var keysJSON string
		if err := rows.Scan(&t.profileID, &t.boardID, &t.sprintID, &t.ritualType, &keysJSON); err != nil {
			rows.Close()
			return fmt.Errorf("scan ritual issue keys: %w", err)
		}
		var keys []string
		if err := json.Unmarshal([]byte(keysJSON), &keys); err != nil {
			// A row nobody can parse is left exactly as it is rather than
			// emptied: the draft is the user's unpublished work.
			continue
		}
		converted := make([]struct {
			Key    string `json:"key"`
			Remark string `json:"remark"`
		}, 0, len(keys))
		for _, k := range keys {
			converted = append(converted, struct {
				Key    string `json:"key"`
				Remark string `json:"remark"`
			}{Key: k})
		}
		encoded, err := json.Marshal(converted)
		if err != nil {
			rows.Close()
			return fmt.Errorf("encode ritual issues: %w", err)
		}
		t.issuesJSON = string(encoded)
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate ritual issue keys: %w", err)
	}
	rows.Close()

	for _, t := range targets {
		if _, err := db.Exec(`UPDATE ritual_document SET issues_json = ?
			WHERE profile_id = ? AND board_id = ? AND sprint_id = ? AND ritual_type = ?`,
			t.issuesJSON, t.profileID, t.boardID, t.sprintID, t.ritualType); err != nil {
			return fmt.Errorf("write ritual issues for %s: %w", t.ritualType, err)
		}
	}
	return nil
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
	profile_id    TEXT NOT NULL,
	id            INTEGER NOT NULL,
	board_id      INTEGER NOT NULL,
	name          TEXT NOT NULL DEFAULT '',
	state         TEXT NOT NULL DEFAULT '',
	start_date    TEXT NOT NULL DEFAULT '',
	end_date      TEXT NOT NULL DEFAULT '',
	goal          TEXT NOT NULL DEFAULT '',
	complete_date TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, board_id, id)
);`

// sprintReportDDL is a sprint's reconstructed report, one row per board's
// copy of a sprint for the same reason sprintDDL's key carries board_id:
// Jira hands one sprint to every board whose filter reaches it, and a
// report's own done rule comes from its board's last column, so a key
// without the board would let one board's report answer for another's.
// algo_version is compared against internal/reports.AlgoVersion on read,
// so a row a lower version wrote is rebuilt rather than served.
const sprintReportDDL = `
CREATE TABLE IF NOT EXISTS sprint_report (
	profile_id   TEXT NOT NULL,
	board_id     INTEGER NOT NULL,
	sprint_id    INTEGER NOT NULL,
	unit         TEXT NOT NULL DEFAULT '',
	algo_version INTEGER NOT NULL DEFAULT 0,
	built_at     TEXT NOT NULL DEFAULT '',
	series_json  TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, board_id, sprint_id)
);`

// ritualDocumentDDL holds one locally editable document for each ritual in
// one board's copy of a sprint. The four-part key keeps profiles, boards,
// sprints, and ritual types from overwriting one another. Publish metadata
// stays with the draft so an explicit publish can update its existing page.
const ritualDocumentDDL = `
CREATE TABLE IF NOT EXISTS ritual_document (
	profile_id        TEXT NOT NULL,
	board_id          INTEGER NOT NULL,
	sprint_id         INTEGER NOT NULL,
	ritual_type       TEXT NOT NULL,
	title             TEXT NOT NULL DEFAULT '',
	remark            TEXT NOT NULL DEFAULT '',
	body              TEXT NOT NULL DEFAULT '',
	issue_keys_json   TEXT NOT NULL DEFAULT '[]',
	issues_json       TEXT NOT NULL DEFAULT '[]',
	confluence_page_id TEXT NOT NULL DEFAULT '',
	confluence_version INTEGER NOT NULL DEFAULT 0,
	status            TEXT NOT NULL DEFAULT '',
	updated_at        TEXT NOT NULL DEFAULT '',
	published_at      TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, board_id, sprint_id, ritual_type)
);`

const indexDDL = `
CREATE INDEX IF NOT EXISTS issue_profile_type   ON issue (profile_id, type);
CREATE INDEX IF NOT EXISTS issue_profile_sprint ON issue (profile_id, sprint_id);
CREATE INDEX IF NOT EXISTS jira_user_profile_display ON jira_user (profile_id, display_name);
CREATE INDEX IF NOT EXISTS board_issue_lookup    ON board_issue (profile_id, board_id, sprint_id);
CREATE INDEX IF NOT EXISTS ritual_document_profile ON ritual_document (profile_id);`

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
