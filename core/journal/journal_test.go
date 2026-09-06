package journal_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"agile-suite/core/journal"
	"agile-suite/core/store"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "j.db"), store.Schema{Version: 1, Base: journal.DDL})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db.DB()
}

func inTx(t *testing.T, db *sql.DB, fn func(tx *sql.Tx) error) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("tx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestUpsertCoalescesAndDropsOnRevert(t *testing.T) {
	db := openDB(t)
	inTx(t, db, func(tx *sql.Tx) error {
		return journal.Upsert(tx, "p1", "issue", "PLAT-1", "summary", "old", "new", "2026-09-01T00:00:00Z")
	})
	rows, err := journal.List(db, "p1")
	if err != nil || len(rows) != 1 {
		t.Fatalf("after insert: %v rows=%d", err, len(rows))
	}
	if rows[0].BeforeVal != "old" || rows[0].AfterVal != "new" || rows[0].BaseVersion != "2026-09-01T00:00:00Z" || rows[0].ID == 0 {
		t.Errorf("row = %+v", rows[0])
	}
	inTx(t, db, func(tx *sql.Tx) error {
		return journal.Upsert(tx, "p1", "issue", "PLAT-1", "summary", "new", "newer", "2026-09-01T00:00:00Z")
	})
	rows, _ = journal.List(db, "p1")
	if len(rows) != 1 || rows[0].BeforeVal != "old" || rows[0].AfterVal != "newer" {
		t.Errorf("second edit should update after_val and keep before_val: %+v", rows)
	}
	inTx(t, db, func(tx *sql.Tx) error {
		return journal.Upsert(tx, "p1", "issue", "PLAT-1", "summary", "newer", "old", "2026-09-01T00:00:00Z")
	})
	rows, _ = journal.List(db, "p1")
	if len(rows) != 0 {
		t.Errorf("reverting to the original value should delete the row: %+v", rows)
	}
}

func TestPutNeverTreatsAMatchAsARevert(t *testing.T) {
	db := openDB(t)
	inTx(t, db, func(tx *sql.Tx) error {
		return journal.Put(tx, "p1", "issue", "PLAT-1", "labels", "a", "b", "v1")
	})
	inTx(t, db, func(tx *sql.Tx) error {
		return journal.Put(tx, "p1", "issue", "PLAT-1", "labels", "b", "a", "v1")
	})
	rows, _ := journal.List(db, "p1")
	if len(rows) != 1 || rows[0].AfterVal != "a" {
		t.Errorf("Put must keep the row on a coincidental revert: %+v", rows)
	}
}

func TestAuditEntriesAndActor(t *testing.T) {
	db := openDB(t)
	if journal.Actor() == "" {
		t.Error("Actor() must never be empty")
	}
	inTx(t, db, func(tx *sql.Tx) error {
		if err := journal.Audit(tx, "p1", "issue", "PLAT-1", "edit", "summary", "old", "new", ""); err != nil {
			return err
		}
		return journal.Audit(tx, "p1", "issue", "PLAT-2", "create", "", "", `{"summary":"x"}`, "draft")
	})
	all, err := journal.Entries(db, "p1", "", 10)
	if err != nil || len(all) != 2 {
		t.Fatalf("all entries: %v %d", err, len(all))
	}
	if all[0].EntityKey != "PLAT-2" || all[0].Actor == "" || all[0].OccurredAt == "" {
		t.Errorf("newest first with actor and time: %+v", all[0])
	}
	one, _ := journal.Entries(db, "p1", "PLAT-1", 10)
	if len(one) != 1 || one[0].Action != "edit" || one[0].AfterVal != "new" {
		t.Errorf("entity filter: %+v", one)
	}
	if other, _ := journal.Entries(db, "p2", "", 10); len(other) != 0 {
		t.Errorf("profiles are isolated: %+v", other)
	}
}

func TestGetDeleteAndEntityHelpers(t *testing.T) {
	db := openDB(t)
	inTx(t, db, func(tx *sql.Tx) error {
		if err := journal.Upsert(tx, "p1", "issue", "PLAT-1", "summary", "a", "b", "v1"); err != nil {
			return err
		}
		if err := journal.Upsert(tx, "p1", "issue", "PLAT-1", "priority", "Low", "High", "v1"); err != nil {
			return err
		}
		return journal.Upsert(tx, "p1", "issue", "PLAT-2", "summary", "c", "d", "v2")
	})
	byKey, err := journal.ListForKey(db, "p1", "PLAT-1")
	if err != nil || len(byKey) != 2 {
		t.Fatalf("ListForKey: %v %d", err, len(byKey))
	}
	got, err := journal.Get(db, "p1", byKey[0].ID)
	if err != nil || got.EntityKey != "PLAT-1" {
		t.Errorf("Get: %+v %v", got, err)
	}
	if _, err := journal.Get(db, "p1", 9999); !errors.Is(err, journal.ErrNotFound) {
		t.Errorf("Get missing: %v", err)
	}
	inTx(t, db, func(tx *sql.Tx) error {
		return journal.SetBaseVersion(tx, "p1", "PLAT-1", "v9")
	})
	byKey, _ = journal.ListForKey(db, "p1", "PLAT-1")
	for _, r := range byKey {
		if r.BaseVersion != "v9" {
			t.Errorf("SetBaseVersion missed %+v", r)
		}
	}
	inTx(t, db, func(tx *sql.Tx) error {
		return journal.Delete(tx, "p1", []int64{byKey[0].ID})
	})
	if rest, _ := journal.ListForKey(db, "p1", "PLAT-1"); len(rest) != 1 {
		t.Errorf("Delete by id: %+v", rest)
	}
	var n int
	inTx(t, db, func(tx *sql.Tx) (err error) {
		n, err = journal.DeleteForKey(tx, "p1", "PLAT-1")
		return err
	})
	if n != 1 {
		t.Errorf("DeleteForKey removed %d, want 1", n)
	}
	if all, _ := journal.List(db, "p1"); len(all) != 1 || all[0].EntityKey != "PLAT-2" {
		t.Errorf("PLAT-2 must survive: %+v", all)
	}
}
