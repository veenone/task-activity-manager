package tamstore_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/tamstore"
)

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
