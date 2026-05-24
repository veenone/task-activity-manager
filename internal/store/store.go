// Package store is the local SQLite persistence layer.
//
// The local database is a cache of Jira/Xray data plus a journal of pending
// changes; Jira remains the system of record. SQLite is accessed through the
// pure-Go modernc.org/sqlite driver so the app ships as a single binary with
// no cgo toolchain.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// schemaVersion is bumped whenever the schema changes.
const schemaVersion = 4

// schema is the canonical database layout for a fresh install. Existing
// databases are upgraded by applyMigrations.
const schema = `
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS profiles (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	jira_url    TEXT NOT NULL,
	project_key TEXT NOT NULL,
	created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sync_state (
	profile_id     TEXT PRIMARY KEY,
	last_synced_at TEXT,
	test_count     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS test_folder (
	profile_id TEXT NOT NULL,
	id         TEXT NOT NULL,
	parent_id  TEXT NOT NULL DEFAULT '',
	name       TEXT NOT NULL,
	PRIMARY KEY (profile_id, id)
);

CREATE INDEX IF NOT EXISTS idx_test_folder_parent ON test_folder(profile_id, parent_id);

CREATE TABLE IF NOT EXISTS test_case (
	profile_id  TEXT NOT NULL,
	jira_key    TEXT NOT NULL,
	jira_id     TEXT NOT NULL,
	summary     TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status      TEXT NOT NULL DEFAULT '',
	priority    TEXT NOT NULL DEFAULT '',
	labels      TEXT NOT NULL DEFAULT '',
	updated_at  TEXT NOT NULL DEFAULT '',
	folder_id   TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, jira_key)
);

CREATE INDEX IF NOT EXISTS idx_test_case_status  ON test_case(profile_id, status);
CREATE INDEX IF NOT EXISTS idx_test_case_updated ON test_case(profile_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_test_case_folder  ON test_case(profile_id, folder_id);

CREATE TABLE IF NOT EXISTS precondition (
	profile_id  TEXT NOT NULL,
	jira_key    TEXT NOT NULL,
	summary     TEXT NOT NULL,
	type        TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, jira_key)
);

CREATE TABLE IF NOT EXISTS test_precondition (
	profile_id       TEXT NOT NULL,
	test_key         TEXT NOT NULL,
	precondition_key TEXT NOT NULL,
	PRIMARY KEY (profile_id, test_key, precondition_key)
);

CREATE INDEX IF NOT EXISTS idx_test_precondition_test ON test_precondition(profile_id, test_key);
`

// Store wraps the SQLite connection for one local database file.
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) the SQLite database at path, applies the
// canonical schema, and runs any upgrade migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	if _, err := db.Exec(
		"INSERT INTO meta (key, value) VALUES ('schema_version', ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		schemaVersion,
	); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("record schema version: %w", err)
	}
	return &Store{db: db}, nil
}

// applyMigrations upgrades older databases to the current schema. Fresh
// installs already match the canonical layout via CREATE TABLE IF NOT EXISTS;
// this function handles the deltas for in-place upgrades from earlier
// versions.
func applyMigrations(db *sql.DB) error {
	current, err := readSchemaVersion(db)
	if err != nil {
		return err
	}

	// v3: add folder_id to test_case. Fresh installs already have it from
	// the CREATE above; this ALTER catches v1/v2 databases. The
	// "duplicate column" error is tolerated so the migration is idempotent.
	if current < 3 {
		if _, err := db.Exec(
			`ALTER TABLE test_case ADD COLUMN folder_id TEXT NOT NULL DEFAULT ''`,
		); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("v3 add folder_id: %w", err)
		}
	}
	// v4: precondition / test_precondition tables. Additive only — the
	// CREATE TABLE IF NOT EXISTS statements above cover both fresh installs
	// and upgrades, so no explicit migration step is needed.
	return nil
}

// readSchemaVersion returns the recorded schema version, or 0 for a database
// that has never had one written.
func readSchemaVersion(db *sql.DB) (int, error) {
	var raw string
	err := db.QueryRow(
		"SELECT value FROM meta WHERE key = 'schema_version'",
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	v, _ := strconv.Atoi(raw)
	return v, nil
}

// DB exposes the underlying connection for the repository layer.
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the database connection.
func (s *Store) Close() error { return s.db.Close() }
