package dbtx_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/dbtx"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

// openTestDB opens a fresh tam.db, the same handle InRead runs against in
// production: WAL journal mode, no _txlock, through the driver Fix 2 is
// about.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db.DB()
}

// TestInReadPinsItsSnapshotAtTheFirstRead is InRead's own promise: a read
// that takes several statements sees the database as of its first one, not
// as of each. A writer commits on the handle, outside the transaction,
// between fn's two reads; the second read must still answer with the value
// the first one saw. The writer only starts once the first read is in, and
// fn only makes its second read once the writer's commit is in, so the
// ordering is exact rather than a timing race.
func TestInReadPinsItsSnapshotAtTheFirstRead(t *testing.T) {
	handle := openTestDB(t)
	ctx := context.Background()
	if _, err := handle.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES ('probe', 'before')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	committed := make(chan struct{})
	proceed := make(chan struct{})
	go func() {
		<-proceed
		if _, err := handle.ExecContext(context.Background(),
			`UPDATE meta SET value = 'after' WHERE key = 'probe'`); err != nil {
			t.Errorf("mutate: %v", err)
		}
		close(committed)
	}()

	err := dbtx.InRead(ctx, handle, func(q dbtx.Querier) error {
		var before string
		if err := q.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'probe'`).Scan(&before); err != nil {
			return err
		}
		if before != "before" {
			t.Fatalf("before = %q, want %q", before, "before")
		}
		close(proceed)
		<-committed

		var after string
		if err := q.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'probe'`).Scan(&after); err != nil {
			return err
		}
		if after != "before" {
			t.Errorf("after = %q, want the transaction's own snapshot %q, not the commit made once its first read was already in", after, "before")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InRead: %v", err)
	}

	var final string
	if err := handle.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'probe'`).Scan(&final); err != nil || final != "after" {
		t.Fatalf("final = %q, %v, want the commit visible on the handle once the read is done", final, err)
	}
}

// TestInReadKeepsTheIssueCacheOnTheBoardsSnapshot is the same promise,
// proven through the real reader a board composes through instead of a raw
// table: issuerepo.Repository.IssuesByKeys. boardrepo's own tests only ever
// exercise this against a fake IssueSource that answers the same rows
// whether or not it runs inside a transaction, so a read that quietly moved
// off q and onto r.db would still pass every one of them. This one calls
// the production method itself, so it fails if IssuesByKeys, DraftIssues,
// or PendingMoves ever stop reading on the querier they are handed: revert
// IssuesByKeys to read on r.db and the second call below sees the status id
// the writer committed after the first, not the one the read started with.
func TestInReadKeepsTheIssueCacheOnTheBoardsSnapshot(t *testing.T) {
	handle := openTestDB(t)
	repo := issuerepo.New(handle)
	ctx := context.Background()

	seed := backend.Issue{
		Key: "PLAT-1", Project: "PLAT", Type: "Story", Status: "To Do", StatusID: "1",
		Created: "2026-01-01T00:00:00Z", Updated: "2026-01-01T00:00:00Z",
	}
	if err := repo.UpsertPage(ctx, "p1", []backend.Issue{seed}, time.Now(), true); err != nil {
		t.Fatalf("seed: %v", err)
	}

	committed := make(chan struct{})
	proceed := make(chan struct{})
	go func() {
		<-proceed
		if _, err := handle.ExecContext(context.Background(),
			`UPDATE issue SET status_id = ? WHERE profile_id = ? AND key = ?`, "5", "p1", "PLAT-1"); err != nil {
			t.Errorf("mutate: %v", err)
		}
		close(committed)
	}()

	err := dbtx.InRead(ctx, handle, func(q dbtx.Querier) error {
		before, err := repo.IssuesByKeys(ctx, q, "p1", []string{"PLAT-1"})
		if err != nil {
			return err
		}
		if len(before) != 1 || before[0].StatusID != "1" {
			t.Fatalf("before = %+v, want status id 1", before)
		}
		close(proceed)
		<-committed

		after, err := repo.IssuesByKeys(ctx, q, "p1", []string{"PLAT-1"})
		if err != nil {
			return err
		}
		if len(after) != 1 || after[0].StatusID != "1" {
			t.Errorf("after = %+v, want the read's own snapshot (status id 1), not the commit made once it was already open (status id 5)", after)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InRead: %v", err)
	}

	final, err := repo.IssuesByKeys(ctx, handle, "p1", []string{"PLAT-1"})
	if err != nil || len(final) != 1 || final[0].StatusID != "5" {
		t.Fatalf("final = %+v, %v, want the commit visible on the handle once the read is done", final, err)
	}
}
