// Package tamstore is Task Activity Manager's own database: everything that
// is not a profile, connection, or global setting, because those live in the
// shared profiles.db. Version 1 carries no app tables. The issue tables arrive
// with the Phase 1 spec as migration 2.
package tamstore

import (
	"os"
	"path/filepath"

	"agile-suite/core/store"
)

// Schema is TAM's database layout.
var Schema = store.Schema{Version: 1}

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
