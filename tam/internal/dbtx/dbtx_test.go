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
// proven through the real readers a board composes through instead of a raw
// table: issuerepo's IssuesByKeys, DraftIssues and PendingMoves, which are
// the whole of boardrepo.IssueSource. boardrepo's own tests only ever
// exercise these against a fake IssueSource that answers the same rows
// whether or not it runs inside a transaction, so a read that quietly moved
// off q and onto r.db would still pass every one of them.
//
// This one calls the production methods themselves, and probes all three
// rather than the one that is easiest to seed: the writer beside it commits
// a status change, a new draft, and a journaled sprint move once the read is
// already open, and none of the three may be visible to it. Revert any of
// the three to read on r.db and the matching probe below sees what the
// writer committed after the read started.
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
		// A draft and a journaled move, written the way the app writes them,
		// so DraftIssues and PendingMoves each have something to see that
		// the open read must not.
		if _, err := repo.CreateDraft(context.Background(), "p1", "PLAT",
			backend.IssueDraft{Type: backend.TypeStory, Summary: "Drafted mid-read"}); err != nil {
			t.Errorf("create a draft: %v", err)
		}
		if err := repo.MoveToSprint(context.Background(), "p1", "PLAT-1", "13", "Sprint 13"); err != nil {
			t.Errorf("journal a sprint move: %v", err)
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
		drafts, err := repo.DraftIssues(ctx, q, "p1")
		if err != nil {
			return err
		}
		if len(drafts) != 0 {
			t.Fatalf("drafts before = %+v, want none", drafts)
		}
		moves, err := repo.PendingMoves(ctx, q, "p1")
		if err != nil {
			return err
		}
		if len(moves) != 0 {
			t.Fatalf("pending moves before = %+v, want none", moves)
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
		if drafts, err = repo.DraftIssues(ctx, q, "p1"); err != nil {
			return err
		}
		if len(drafts) != 0 {
			t.Errorf("drafts after = %+v, want the read's own snapshot, not the draft created once it was already open", drafts)
		}
		if moves, err = repo.PendingMoves(ctx, q, "p1"); err != nil {
			return err
		}
		if len(moves) != 0 {
			t.Errorf("pending moves after = %+v, want the read's own snapshot, not the move journaled once it was already open", moves)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InRead: %v", err)
	}

	// All three are visible on the handle once the read is done, so the
	// probes above are about when they became visible and not about whether
	// the writer wrote anything.
	final, err := repo.IssuesByKeys(ctx, handle, "p1", []string{"PLAT-1"})
	if err != nil || len(final) != 1 || final[0].StatusID != "5" {
		t.Fatalf("final = %+v, %v, want the commit visible on the handle once the read is done", final, err)
	}
	drafts, err := repo.DraftIssues(ctx, handle, "p1")
	if err != nil || len(drafts) != 1 {
		t.Fatalf("drafts = %+v, %v, want the one the writer created", drafts, err)
	}
	moves, err := repo.PendingMoves(ctx, handle, "p1")
	if err != nil || len(moves) != 1 {
		t.Fatalf("pending moves = %+v, %v, want the one the writer journaled", moves, err)
	}
}
