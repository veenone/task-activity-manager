// Package tamstore is Task Activity Manager's own database: everything that
// is not a profile, connection, or global setting, because those live in the
// shared profiles.db. Version 1 carries no app tables. Version 2 adds the
// issue tables.
package tamstore

import (
	"os"
	"path/filepath"

	"agile-suite/core/journal"
	"agile-suite/core/store"
)

// Schema is TAM's local schema. Version 3 adds the shared journal tables;
// their statements are idempotent, so an older database picks them up on
// its next open without a migration step.
var Schema = store.Schema{
	Version: 3,
	Base:    baseDDL + journal.DDL,
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
);`

const indexDDL = `
CREATE INDEX IF NOT EXISTS issue_profile_type   ON issue (profile_id, type);
CREATE INDEX IF NOT EXISTS issue_profile_sprint ON issue (profile_id, sprint_id);`

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
