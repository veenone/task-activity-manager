// Package store is the SQLite layer both apps build on. It opens a database
// with the pragmas the desktop apps rely on and walks a caller-supplied schema
// through the same three stages XTM's store has always used: base tables,
// then the migrations an older file is missing, then indexes, recording the
// resulting version in a meta table. The schema belongs to the caller; this
// package only knows how to apply one.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// Migration upgrades a database whose recorded version is below Version.
type Migration struct {
	Version int
	Apply   func(db *sql.DB) error
}

// Schema describes one database: the tables a fresh install gets, the ordered
// migrations that bring an older file up to date, and the indexes applied
// last so they can reference columns a migration adds.
type Schema struct {
	Version    int
	Base       string
	Migrations []Migration
	Indexes    string
}

// DB wraps an open database.
type DB struct {
	db *sql.DB
}

const metaTable = `CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

// Open opens (or creates) the SQLite file at path and applies s. Calling it on
// a database that is already current is a no-op apart from re-recording the
// version.
//
// WAL lets readers proceed alongside a writer, so the UI's parallel queries
// don't queue behind a sync's write transaction; busy_timeout makes a briefly
// locked file wait instead of failing with "database is locked" when a
// previous instance is still letting go of it.
func Open(path string, s Schema) (*DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := apply(db, s); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func apply(db *sql.DB, s Schema) error {
	if _, err := db.Exec(metaTable); err != nil {
		return fmt.Errorf("create meta: %w", err)
	}
	if s.Base != "" {
		if _, err := db.Exec(s.Base); err != nil {
			return fmt.Errorf("apply tables: %w", err)
		}
	}
	current, err := ReadSchemaVersion(db)
	if err != nil {
		return err
	}
	for _, m := range s.Migrations {
		if current >= m.Version {
			continue
		}
		if err := m.Apply(db); err != nil {
			return fmt.Errorf("migration v%d: %w", m.Version, err)
		}
	}
	if s.Indexes != "" {
		if _, err := db.Exec(s.Indexes); err != nil {
			return fmt.Errorf("apply indexes: %w", err)
		}
	}
	if _, err := db.Exec(
		"INSERT INTO meta (key, value) VALUES ('schema_version', ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		s.Version,
	); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}
	return nil
}

// DB exposes the underlying handle for the managers built on top.
func (d *DB) DB() *sql.DB { return d.db }

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

// ReadSchemaVersion returns the version recorded in meta, or 0 for a fresh
// file.
func ReadSchemaVersion(db *sql.DB) (int, error) {
	var raw string
	err := db.QueryRow("SELECT value FROM meta WHERE key = 'schema_version'").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	v, _ := strconv.Atoi(raw)
	return v, nil
}

// AddColumnIfMissing runs ALTER TABLE ... ADD COLUMN and treats "duplicate
// column" as success, so a migration stays idempotent when a fresh install
// already has the column from the base schema.
func AddColumnIfMissing(db *sql.DB, table, columnDDL string) error {
	_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, columnDDL))
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	return nil
}
