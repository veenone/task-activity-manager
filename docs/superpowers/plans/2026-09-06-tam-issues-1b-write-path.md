# Task Activity Manager issues, plan 1b: the write path

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** TAM edits and creates issues through a local journal, pushes them to Jira with Commit, holds conflicting issues back with an override or keep-remote resolution, marks pending rows in the grid, and shows each issue's local activity.

**Architecture:** `core/journal` is the `pending_change` and `audit_log` tables plus the transaction-level helpers lifted from XTM's `testrepo`; XTM's helpers become delegators. TAM's store moves to schema version 3 and gains the edit, draft, rekey, replace, and pending-key operations. The `IssueBackend` grows four methods (get one issue, update, create, create-meta fields) in both the Jira and demo implementations. A new `tam/internal/committer` package runs the commit pass and the two resolutions. The frontend adds the Pending changes dialog, the editable Details tab, the New issue dialog, the conflict dialog, and the Activity tab, wired to the shared reducer's `committing` state.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4, npm workspaces.

**Spec:** [`../specs/2026-09-05-tam-issues-design.md`](../specs/2026-09-05-tam-issues-design.md), section 13. **Mockups:** [`../specs/assets/2026-09-04-tam-shell-backlog.svg`](../specs/assets/2026-09-04-tam-shell-backlog.svg) (the write controls) and [`../specs/assets/2026-09-06-tam-pending-changes.svg`](../specs/assets/2026-09-06-tam-pending-changes.svg) (the Pending changes dialog with a conflict).

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, `agile-suite/tam`; each app module carries `replace agile-suite/core => ../core`. Run Go commands from inside the module directory.
- XTM is edited only where this plan names it: in Task 1, `xtm/internal/testrepo/testrepo.go`'s four helpers (`upsertPendingChange`, `putPendingChange`, `writeAudit`, `currentActor`) become one-line delegators with their bodies moved to `core/journal`, and its `os/user` import goes if unused. Every error string XTM produced stays byte-identical. Nothing else under `xtm/` changes.
- Task 1 lands as its own PR before Task 2 starts. Its gate is XTM's full Go suite (`go test ./internal/...` inside `xtm/`) and XTM's Vitest suite (159 tests) staying green with `go vet ./...` clean.
- Every later task leaves XTM's Go suite, every Vitest workspace, and `npm run typecheck --workspaces --if-present` green.
- `core` and `frontend/core` hold only what a task in this plan needs. `frontend/core` is not edited by this plan.
- Editable fields are exactly `summary`, `description`, `priority`, `labels`, `storyPoints`, `assignee`. Creatable types in 1b are `task`, `story`, `bug`. Status and sprint moves are the live path and are out of scope.
- A draft's key is `TAM-NEW-<n>` (prefix `TAM-NEW-`), numbered per profile from 1. Sync never deletes a draft row and never overwrites a column with a pending edit.
- The base version of an edit is the row's `updated` at the moment of the edit. A conflict is a remote `updated` that differs from a pending row's `base_version`.
- Bound method signatures (Task 6) are exactly: `EditIssue(profileID, key, field, value string) error`, `CreateIssue(profileID string, draft backend.IssueDraft) (string, error)`, `GetCreateFields(profileID, typeName string) ([]backend.FieldSpec, error)`, `ListPendingChanges(profileID string) ([]journal.PendingChange, error)`, `DiscardPendingChange(profileID string, id int64) error`, `DiscardAllPendingChanges(profileID string) (int, error)`, `CommitPendingChanges(profileID string) (committer.Result, error)`, `ResolveConflictOverride(profileID, key, remoteVersion string) error`, `ResolveConflictKeepRemote(profileID, key string) error`, `ListActivity(profileID, key string, limit int) ([]journal.AuditEntry, error)`.
- The PAT stays in the Jira client's Authorization header only. No token in SQLite, logs, or JSON to the frontend.
- UI text uses no em dashes. No AI attribution or mentions in any commit message, PR, file, or comment. Run the humanizer pass over prose, including code comments.
- Commit messages use the repo's conventional prefixes (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`) with a scope in parentheses where one applies, and no trailers.
- The working tree holds untracked local tooling files (`agentdb.rvf`, `.claude/`, `reference/`, `.superpowers/` and similar). Never add, commit, or delete them. Check state with `git status --short --untracked-files=no`. Wails may rewrite line endings under `tam/frontend/wailsjs/runtime` and `tam/frontend/package.json.md5`; revert those with `git checkout --` unless the task says otherwise.

## Decisions

1. **`core/journal` exposes `Get` and `Delete`, not a `Discard`.** XTM's discard reverts entity-specific columns before deleting the row, and TAM's does too, so the shared package offers the row read and the delete; each app writes the revert.
2. **Description edits live in `detail_json`.** The `issue` table has no description column; the detail cache does. `EditField` for `description` reads and writes the cached detail's `description` and journals it like any other field. The panel only offers the edit once the detail has loaded, so the current value is always known.
3. **A fourth backend method, `GetIssue`.** The spec named three additions; the commit pass also needs one issue's current fields to compare versions and to refresh a row. `SearchIssuesPage` cannot fetch by key without abusing its scope argument, so `GetIssue(ctx, key) (Issue, error)` is added alongside the three.
4. **The demo backend stages one conflict.** Its `GetIssue` for the curated story (`PLAT-412`, rekeyed to the profile's project) reports an `updated` one hour later than the cached row the first time it is asked after the app starts, then stops. The first Commit of an edit to that issue conflicts; override then commits; keep remote then refreshes. Every other demo issue commits cleanly.
5. **A re-sync of an edited issue keeps the edit and the old base.** The upsert leaves the pending fields' columns alone; `updated` is refreshed from Jira. If Jira had changed, the next Commit sees the mismatch and holds the issue back, which is the correct outcome; if only the local edit exists, the refreshed `updated` equals the old base and Commit proceeds.
6. **Commit and sync exclude each other in the app.** The Go in-flight map from plan 1a gains a second use: `CommitPendingChanges` refuses while a sync runs for the profile and vice versa. The frontend reducer's `committing` state does the same for the buttons.
7. **Pending and draft flags come from the query, not the row.** `ListIssues` computes `pending` with an `EXISTS` on `pending_change` and `draft` from the key prefix, so no column has to be kept in step.
8. **Conflict cards live inside the Pending changes dialog.** The mockup puts the held issue's base, mine, remote table among the other groups, so there is no separate conflict dialog. Edit cards show the key and the field rows; the journal carries no summary, and adding a read for it is not worth a bound method.
9. **`UpdateIssue` takes text values by logical field.** The journal holds text; the backend owns Jira's shapes (`{name}` objects, label lists, the points custom field), so the committer passes the journal through untouched.
10. **Shared label and points helpers sit in `backend`.** `SplitLabels`, `ParsePoints`, and `FormatPoints` are used by the store, both backends, and the committer, and `backend` is the package all of them already import.

## File structure

**Created**

- `core/journal/journal.go`, `core/journal/journal_test.go` (Task 1).
- `tam/internal/issuerepo/pending.go`, `pending_test.go`, `writes.go`, `writes_test.go` (Tasks 2, 3).
- `tam/internal/committer/committer.go`, `committer_test.go` (Task 5).
- `tam/app_writes.go` (Task 6).
- `tam/frontend/src/queries/pending.ts`, `components/PendingChangesModal.tsx`, `PendingChangesModal.test.tsx`, `components/ConflictCard.tsx` (placeholder) (Task 7).
- `tam/frontend/src/components/EditableFields.tsx`, `ActivityTab.tsx` (Task 8).
- `tam/frontend/src/components/NewIssueModal.tsx`, `NewIssueModal.test.tsx` (Task 9).
- `tam/frontend/src/components/ConflictCard.tsx` (final) (Task 10).

**Modified**

- `xtm/internal/testrepo/testrepo.go` (Task 1, delegators only).
- `tam/internal/tamstore/tamstore.go`, `tamstore_test.go`; `tam/internal/issuerepo/issues.go`, `issues_test.go`; `tam/internal/backend/backend.go` (Task 2).
- `tam/internal/backend/jira/jira.go`, `fields.go`, `jira_test.go`, `fields_test.go`; `tam/internal/backend/demo/demo.go`, `demo_test.go`; `tam/internal/syncer/syncer_test.go` (Task 4).
- `tam/app.go`, `tam/frontend/wailsjs/**` (regenerated, Task 6).
- `tam/frontend/src/api.ts`, `queries/keys.ts`, `queries/invalidate.ts`, `modals.ts`, `main.tsx`, `App.tsx`, `App.test.tsx`, `App.css`, `contexts/SyncContext.tsx` (Task 7).
- `tam/frontend/src/components/IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx`, `IssueTable.tsx`, `BacklogView.tsx`, `BacklogView.test.tsx`; `tam/internal/issuerepo/issues.go` again for the draft-first order (Tasks 8, 9).
- `tam/CLAUDE.md`, `README.md`, the spec's new 13.10 (Task 10).

---

### Task 1: `core/journal`, lifted from XTM

Move the pending-change and audit helpers XTM's `testrepo` uses into a shared package, with XTM's four functions delegating to it. This task is its own PR.

**Files:**
- Create: `core/journal/journal.go`, `core/journal/journal_test.go`
- Modify: `xtm/internal/testrepo/testrepo.go` (the four helpers at the end of the file, and the `os/user` import if it becomes unused)
- Test: `go test ./journal/` inside `core/`; `go test ./internal/... -count=1` and `go vet ./...` inside `xtm/`

**Interfaces:**
- Produces (package `agile-suite/core/journal`): `const DDL string`, `const EntityTypeMax`, `type PendingChange`, `type AuditEntry`, `type Execer interface`, `type Querier interface`, `ErrNotFound`, `Upsert(tx Execer, profileID, entityType, entityKey, field, before, after, baseVersion string) error`, `Put(tx Execer, ...same...) error`, `Audit(tx Execer, profileID, entityType, entityKey, action, field, before, after, note string) error`, `Actor() string`, `List(db Querier, profileID string) ([]PendingChange, error)`, `ListForKey(db Querier, profileID, entityKey string) ([]PendingChange, error)`, `Get(db Querier, profileID string, id int64) (PendingChange, error)`, `Delete(tx Execer, profileID string, ids []int64) error`, `DeleteForKey(tx Execer, profileID, entityKey string) (int, error)`, `SetBaseVersion(tx Execer, profileID, entityKey, version string) error`, `Entries(db Querier, profileID, entityKey string, limit int) ([]AuditEntry, error)`.

- [ ] **Step 1: Write the failing tests**

Create `core/journal/journal_test.go`:

```go
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
```

- [ ] **Step 2: Run them to see the failure**

Run (inside `core/`): `go test ./journal/`
Expected: FAIL, the package does not exist.

- [ ] **Step 3: The package**

Create `core/journal/journal.go`. The bodies of `Upsert`, `Put`, `Audit`, and `Actor` are XTM's, including every error string:

```go
// Package journal is the pending-change journal and audit trail each suite
// app keeps in its own database: the two tables, the transaction-level
// helpers every local edit goes through, and the reads that Commit and the
// activity views need. The helpers were lifted from XTM's testrepo, which
// now delegates to them. Reverting an entity's columns on discard stays in
// each app, since only the app knows its columns.
package journal

import (
	"database/sql"
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"
)

// DDL creates the two tables and their indexes. Both apps include it in
// their store's base schema; every statement is idempotent.
const DDL = `
CREATE TABLE IF NOT EXISTS pending_change (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_id   TEXT NOT NULL,
	entity_type  TEXT NOT NULL,
	entity_key   TEXT NOT NULL,
	field        TEXT NOT NULL,
	before_val   TEXT NOT NULL DEFAULT '',
	after_val    TEXT NOT NULL DEFAULT '',
	base_version TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL,
	UNIQUE (profile_id, entity_type, entity_key, field)
);
CREATE TABLE IF NOT EXISTS audit_log (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_id  TEXT NOT NULL,
	occurred_at TEXT NOT NULL,
	actor       TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL,
	entity_key  TEXT NOT NULL,
	action      TEXT NOT NULL,
	field       TEXT NOT NULL DEFAULT '',
	before_val  TEXT NOT NULL DEFAULT '',
	after_val   TEXT NOT NULL DEFAULT '',
	note        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_pending_change_profile ON pending_change(profile_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_profile_time ON audit_log(profile_id, occurred_at DESC);`

// ErrNotFound is returned by Get for an id the profile does not have.
var ErrNotFound = errors.New("journal: pending change not found")

// PendingChange is one journaled field change. BaseVersion is the remote
// version the edit was made against; Commit compares it with the remote.
type PendingChange struct {
	ID          int64  `json:"id"`
	EntityType  string `json:"entityType"`
	EntityKey   string `json:"entityKey"`
	Field       string `json:"field"`
	BeforeVal   string `json:"beforeVal"`
	AfterVal    string `json:"afterVal"`
	BaseVersion string `json:"baseVersion"`
	CreatedAt   string `json:"createdAt"`
}

// AuditEntry is one row of the local audit trail: who, what, when, and the
// before and after values.
type AuditEntry struct {
	ID         int64  `json:"id"`
	OccurredAt string `json:"occurredAt"`
	Actor      string `json:"actor"`
	EntityType string `json:"entityType"`
	EntityKey  string `json:"entityKey"`
	Action     string `json:"action"`
	Field      string `json:"field"`
	BeforeVal  string `json:"beforeVal"`
	AfterVal   string `json:"afterVal"`
	Note       string `json:"note"`
}

// Execer is what the write helpers need; *sql.Tx and *sql.DB both satisfy
// it. Writes belong inside the caller's transaction, next to the entity
// update they record.
type Execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Querier is what the reads need; *sql.Tx and *sql.DB both satisfy it.
type Querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Upsert records a field change, keeping one row per entity and field. A
// first edit inserts; a later edit updates after_val and keeps the original
// before_val; a value returning to the original before_val deletes the row.
func Upsert(tx Execer, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion string) error {
	var existingBefore string
	err := tx.QueryRow(
		`SELECT before_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, entityType, entityKey, field,
	).Scan(&existingBefore)

	now := time.Now().UTC().Format(time.RFC3339)

	if errors.Is(err, sql.ErrNoRows) {
		_, ierr := tx.Exec(
			`INSERT INTO pending_change
			   (profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, entityType, entityKey, field, currentVal, newValue, baseVersion, now,
		)
		if ierr != nil {
			return fmt.Errorf("insert pending change: %w", ierr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read existing pending: %w", err)
	}

	if newValue == existingBefore {
		if _, derr := tx.Exec(
			`DELETE FROM pending_change
			 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
			profileID, entityType, entityKey, field,
		); derr != nil {
			return fmt.Errorf("delete pending: %w", derr)
		}
		return nil
	}

	if _, uerr := tx.Exec(
		`UPDATE pending_change SET after_val = ?, created_at = ?
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		newValue, now, profileID, entityType, entityKey, field,
	); uerr != nil {
		return fmt.Errorf("update pending: %w", uerr)
	}
	return nil
}

// Put records or updates a field change without the revert check, for
// callers that have already decided the edit is genuine against a freshly
// read base.
func Put(tx Execer, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion string) error {
	var existingBefore string
	err := tx.QueryRow(
		`SELECT before_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, entityType, entityKey, field,
	).Scan(&existingBefore)

	now := time.Now().UTC().Format(time.RFC3339)

	if errors.Is(err, sql.ErrNoRows) {
		_, ierr := tx.Exec(
			`INSERT INTO pending_change
			   (profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, entityType, entityKey, field, currentVal, newValue, baseVersion, now,
		)
		if ierr != nil {
			return fmt.Errorf("insert pending change: %w", ierr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read existing pending: %w", err)
	}

	if _, uerr := tx.Exec(
		`UPDATE pending_change SET after_val = ?, created_at = ?
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		newValue, now, profileID, entityType, entityKey, field,
	); uerr != nil {
		return fmt.Errorf("update pending: %w", uerr)
	}
	return nil
}

// Audit appends one entry to the trail, stamped with the OS user.
func Audit(tx Execer, profileID, entityType, entityKey, action, field, beforeVal, afterVal, note string) error {
	if _, err := tx.Exec(
		`INSERT INTO audit_log
		   (profile_id, occurred_at, actor, entity_type, entity_key, action, field, before_val, after_val, note)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profileID, time.Now().UTC().Format(time.RFC3339),
		Actor(), entityType, entityKey, action, field, beforeVal, afterVal, note,
	); err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	return nil
}

// Actor returns the OS username for the audit trail, or "user" when it
// cannot be resolved.
func Actor() string {
	u, err := user.Current()
	if err != nil || u == nil || u.Username == "" {
		return "user"
	}
	return u.Username
}

const pendingColumns = `id, entity_type, entity_key, field, before_val, after_val, base_version, created_at`

func scanPending(rows *sql.Rows) ([]PendingChange, error) {
	defer rows.Close()
	out := []PendingChange{}
	for rows.Next() {
		var p PendingChange
		if err := rows.Scan(&p.ID, &p.EntityType, &p.EntityKey, &p.Field,
			&p.BeforeVal, &p.AfterVal, &p.BaseVersion, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// List returns every pending change for the profile, newest first.
func List(db Querier, profileID string) ([]PendingChange, error) {
	rows, err := db.Query(
		`SELECT `+pendingColumns+` FROM pending_change WHERE profile_id = ?
		 ORDER BY created_at DESC, id DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list pending changes: %w", err)
	}
	return scanPending(rows)
}

// ListForKey returns the pending changes of one entity key across entity
// types, oldest first, so a commit pass applies them in the order they were
// made. TAM keeps a draft's create row and its later edits under one key.
func ListForKey(db Querier, profileID, entityKey string) ([]PendingChange, error) {
	rows, err := db.Query(
		`SELECT `+pendingColumns+` FROM pending_change
		 WHERE profile_id = ? AND entity_key = ? ORDER BY id`,
		profileID, entityKey)
	if err != nil {
		return nil, fmt.Errorf("list pending changes for %s: %w", entityKey, err)
	}
	return scanPending(rows)
}

// Get returns one pending change by id, or ErrNotFound.
func Get(db Querier, profileID string, id int64) (PendingChange, error) {
	var p PendingChange
	err := db.QueryRow(
		`SELECT `+pendingColumns+` FROM pending_change WHERE profile_id = ? AND id = ?`, profileID, id,
	).Scan(&p.ID, &p.EntityType, &p.EntityKey, &p.Field, &p.BeforeVal, &p.AfterVal, &p.BaseVersion, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PendingChange{}, ErrNotFound
	}
	if err != nil {
		return PendingChange{}, fmt.Errorf("read pending change: %w", err)
	}
	return p, nil
}

// Delete removes the given pending changes of the profile.
func Delete(tx Execer, profileID string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	args := make([]any, 0, len(ids)+1)
	args = append(args, profileID)
	for _, id := range ids {
		args = append(args, id)
	}
	if _, err := tx.Exec(`DELETE FROM pending_change WHERE profile_id = ? AND id IN (`+marks+`)`, args...); err != nil {
		return fmt.Errorf("delete pending changes: %w", err)
	}
	return nil
}

// DeleteForKey removes every pending change of one entity key and returns
// how many went.
func DeleteForKey(tx Execer, profileID, entityKey string) (int, error) {
	res, err := tx.Exec(
		`DELETE FROM pending_change WHERE profile_id = ? AND entity_key = ?`,
		profileID, entityKey)
	if err != nil {
		return 0, fmt.Errorf("delete pending changes for %s: %w", entityKey, err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// SetBaseVersion rebases every pending change of one entity key onto
// version, which is what an override resolution does.
func SetBaseVersion(tx Execer, profileID, entityKey, version string) error {
	if _, err := tx.Exec(
		`UPDATE pending_change SET base_version = ? WHERE profile_id = ? AND entity_key = ?`,
		version, profileID, entityKey); err != nil {
		return fmt.Errorf("rebase pending changes for %s: %w", entityKey, err)
	}
	return nil
}

// Entries returns audit entries newest first, for one entity when entityKey
// is set or for the whole profile when it is empty. limit defaults to 200
// and caps at 1000.
func Entries(db Querier, profileID, entityKey string, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	where := `profile_id = ?`
	args := []any{profileID}
	if entityKey != "" {
		where += ` AND entity_key = ?`
		args = append(args, entityKey)
	}
	rows, err := db.Query(
		`SELECT id, occurred_at, actor, entity_type, entity_key, action, field, before_val, after_val, note
		 FROM audit_log WHERE `+where+` ORDER BY occurred_at DESC, id DESC LIMIT ?`,
		append(args, limit)...)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var a AuditEntry
		if err := rows.Scan(&a.ID, &a.OccurredAt, &a.Actor, &a.EntityType, &a.EntityKey,
			&a.Action, &a.Field, &a.BeforeVal, &a.AfterVal, &a.Note); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run the core tests**

Run (inside `core/`): `go vet ./journal/ && go test ./journal/ -v`
Expected: PASS, four tests.

- [ ] **Step 5: Point XTM's helpers at the package**

In `xtm/internal/testrepo/testrepo.go`, add the import `"agile-suite/core/journal"`. Replace the bodies of the four functions near the end of the file so each is a one-line delegator. Keep their signatures and doc comments; the doc comment on `putPendingChange` stays because it explains when XTM uses it.

```go
func upsertPendingChange(tx *sql.Tx, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion string) error {
	return journal.Upsert(tx, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion)
}
```

```go
func putPendingChange(tx *sql.Tx, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion string) error {
	return journal.Put(tx, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion)
}
```

```go
func writeAudit(tx *sql.Tx, profileID, entityType, entityKey, action, field, beforeVal, afterVal, note string) error {
	return journal.Audit(tx, profileID, entityType, entityKey, action, field, beforeVal, afterVal, note)
}
```

```go
// currentActor returns the OS username for the audit trail, falling back to
// "user" if it cannot be resolved.
func currentActor() string { return journal.Actor() }
```

Then run `go build ./...` inside `xtm/`. If it reports `"os/user" imported and not used` in `testrepo.go`, remove that import line; if `time` or `errors` become unused for the same reason, remove those too (only if the compiler says so). XTM's `pending_change` and `audit_log` DDL in `xtm/internal/store/store.go` stays where it is.

- [ ] **Step 6: Prove XTM did not move**

Run, inside `xtm/`:

```bash
go build ./... && go vet ./... && go test ./internal/... -count=1
```

Expected: every package `ok`. Then, from the repo root, `npm test --workspace xtm/frontend 2>&1 | grep "Tests "` still reports 159 passing (no frontend file changed).

- [ ] **Step 7: Commit and open the PR**

```bash
git add core/journal xtm/internal/testrepo/testrepo.go
git commit -m "refactor(core): lift the pending-change journal and audit helpers into core/journal"
```

Push the branch and open a PR titled "Lift the pending-change journal into core/journal" whose description says the four XTM helpers now delegate with their bodies and error strings unchanged, that XTM's tables stay in its own schema, and which XTM suites were run. The remaining tasks continue on a branch from this one and do not wait for the merge, but this PR merges first.

---

### Task 2: TAM store version 3, the pending and draft flags, and the draft-safe clear

Bring the journal tables into `tam.db`, let every issue read say whether it has pending changes or is a draft, and stop a full sync's clear from deleting draft rows.

**Files:**
- Modify: `tam/internal/tamstore/tamstore.go`, `tam/internal/tamstore/tamstore_test.go`
- Modify: `tam/internal/backend/backend.go` (the `Issue` struct)
- Modify: `tam/internal/issuerepo/issues.go` (`issueColumns`, `UpsertPage`'s clear, `scanIssue`)
- Create: `tam/internal/issuerepo/pending.go`, `tam/internal/issuerepo/pending_test.go`
- Test: `go test ./internal/tamstore/ ./internal/issuerepo/ ./internal/syncer/` inside `tam/`

**Interfaces:**
- Consumes: `journal.DDL`, `journal.Upsert`, `journal.List`, `journal.ListForKey`, `journal.Entries` from Task 1.
- Produces: `backend.Issue.Pending bool` and `backend.Issue.Draft bool` (JSON `pending`, `draft`); `issuerepo.DraftPrefix = "TAM-NEW-"`, `issuerepo.StatusDraft = "Draft"`, `issuerepo.EntityIssue = "issue"`, `issuerepo.EntityIssueCreate = "issue_create"`, `issuerepo.FieldCreate = "create"`; `(*Repository).PendingKeys(ctx, profileID) ([]string, error)`, `(*Repository).ListPendingChanges(ctx, profileID) ([]journal.PendingChange, error)`, `(*Repository).PendingForKey(ctx, profileID, key) ([]journal.PendingChange, error)`, `(*Repository).ListActivity(ctx, profileID, key string, limit int) ([]journal.AuditEntry, error)`.

- [ ] **Step 1: Store test**

Append to `tam/internal/tamstore/tamstore_test.go` (it already imports `path/filepath`, `testing`, and `tamstore`; add `"agile-suite/core/store"` to the imports):

```go
func TestSchemaVersionThreeAddsTheJournalTablesToAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Turn the fresh database into a version 2 one: drop the journal tables
	// and rewind the recorded version.
	for _, stmt := range []string{
		`DROP TABLE pending_change`, `DROP TABLE audit_log`,
		`UPDATE meta SET value = '2' WHERE key = 'schema_version'`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	_ = db.Close()

	db, err = tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"issue", "pending_change", "audit_log"} {
		var name string
		if err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Errorf("table %s after upgrade: %v", table, err)
		}
	}
	if v, _ := store.ReadSchemaVersion(db.DB()); v != 3 {
		t.Errorf("schema version = %d, want 3", v)
	}
}
```

- [ ] **Step 2: Run it**

Run (inside `tam/`): `go test ./internal/tamstore/ -run ThreeAdds`
Expected: FAIL at `DROP TABLE pending_change` (no such table).

- [ ] **Step 3: Schema**

In `tam/internal/tamstore/tamstore.go`, add the import `"agile-suite/core/journal"` and change the schema value:

```go
// Schema is TAM's local schema. Version 3 adds the shared journal tables;
// their statements are idempotent, so an older database picks them up on
// its next open without a migration step.
var Schema = store.Schema{
	Version: 3,
	Base:    baseDDL + journal.DDL,
	Indexes: indexDDL,
}
```

Run `go test ./internal/tamstore/` and expect PASS for all three tests.

- [ ] **Step 4: The flags on `backend.Issue`**

In `tam/internal/backend/backend.go`, add two fields at the end of the `Issue` struct, after `Updated`:

```go
	// Pending and Draft are computed by the repository's reads, never
	// stored: Pending says the journal holds a change for this key, Draft
	// says the key is a local placeholder Commit has not yet created.
	Pending bool `json:"pending"`
	Draft   bool `json:"draft"`
```

- [ ] **Step 5: Repository tests**

Create `tam/internal/issuerepo/pending_test.go`:

```go
package issuerepo_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

// newRepoDB is newRepo plus the raw handle, for tests that seed the journal
// directly.
func newRepoDB(t *testing.T) (*issuerepo.Repository, *sql.DB) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return issuerepo.New(db.DB()), db.DB()
}

func TestListIssuesFlagsPendingAndDraftRows(t *testing.T) {
	repo, db := newRepoDB(t)
	ctx := context.Background()
	rows := []backend.Issue{
		{Key: "PLAT-1", Type: backend.TypeTask, Summary: "one", Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-2", Type: backend.TypeTask, Summary: "two", Updated: "2026-09-01T00:00:00Z"},
		{Key: issuerepo.DraftPrefix + "1", Type: backend.TypeTask, Summary: "draft", Status: issuerepo.StatusDraft},
	}
	if err := repo.UpsertPage(ctx, "p1", rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := journal.Upsert(db, "p1", issuerepo.EntityIssue, "PLAT-1", "summary", "one", "uno", "2026-09-01T00:00:00Z"); err != nil {
		t.Fatalf("journal: %v", err)
	}
	page, err := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string][2]bool{}
	for _, iss := range page.Issues {
		got[iss.Key] = [2]bool{iss.Pending, iss.Draft}
	}
	want := map[string][2]bool{"PLAT-1": {true, false}, "PLAT-2": {false, false}, "TAM-NEW-1": {false, true}}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: pending/draft = %v, want %v", k, got[k], w)
		}
	}
	one, err := repo.GetIssue(ctx, "p1", "PLAT-1")
	if err != nil || !one.Pending {
		t.Errorf("GetIssue must carry the flag too: %+v %v", one, err)
	}
	keys, err := repo.PendingKeys(ctx, "p1")
	if err != nil || len(keys) != 1 || keys[0] != "PLAT-1" {
		t.Errorf("PendingKeys = %v %v", keys, err)
	}
	if other, _ := repo.PendingKeys(ctx, "p2"); len(other) != 0 {
		t.Errorf("profiles are isolated: %v", other)
	}
}

func TestFullSyncClearKeepsDraftRows(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed := []backend.Issue{
		{Key: "PLAT-1", Type: backend.TypeTask, Summary: "one"},
		{Key: issuerepo.DraftPrefix + "1", Type: backend.TypeTask, Summary: "draft", Status: issuerepo.StatusDraft},
	}
	if err := repo.UpsertPage(ctx, "p1", seed, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	remote := []backend.Issue{{Key: "PLAT-2", Type: backend.TypeTask, Summary: "two"}}
	if err := repo.UpsertPage(ctx, "p1", remote, time.Now(), true); err != nil {
		t.Fatalf("full: %v", err)
	}
	page, _ := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{})
	keys := []string{}
	for _, iss := range page.Issues {
		keys = append(keys, iss.Key)
	}
	if len(keys) != 2 || keys[0] != "PLAT-2" || keys[1] != "TAM-NEW-1" {
		t.Errorf("after a full clear the draft survives and PLAT-1 goes: %v", keys)
	}
}

func TestPendingReadsDelegateToTheJournal(t *testing.T) {
	repo, db := newRepoDB(t)
	ctx := context.Background()
	if err := journal.Upsert(db, "p1", issuerepo.EntityIssue, "PLAT-1", "summary", "a", "b", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := journal.Put(db, "p1", issuerepo.EntityIssueCreate, "TAM-NEW-1", issuerepo.FieldCreate, "", `{"summary":"x"}`, ""); err != nil {
		t.Fatal(err)
	}
	if err := journal.Audit(db, "p1", issuerepo.EntityIssue, "PLAT-1", "edit", "summary", "a", "b", ""); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListPendingChanges(ctx, "p1")
	if err != nil || len(all) != 2 {
		t.Fatalf("ListPendingChanges: %v %d", err, len(all))
	}
	one, err := repo.PendingForKey(ctx, "p1", "TAM-NEW-1")
	if err != nil || len(one) != 1 || one[0].EntityType != issuerepo.EntityIssueCreate {
		t.Errorf("PendingForKey: %+v %v", one, err)
	}
	act, err := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if err != nil || len(act) != 1 || act[0].Action != "edit" {
		t.Errorf("ListActivity: %+v %v", act, err)
	}
	if none, _ := repo.ListActivity(ctx, "p1", "PLAT-2", 0); len(none) != 0 {
		t.Errorf("activity is per key: %+v", none)
	}
}
```

- [ ] **Step 6: Run them**

Run (inside `tam/`): `go test ./internal/issuerepo/ -run 'Flags|KeepsDraft|Delegate'`
Expected: compile failure (undefined `issuerepo.DraftPrefix`, `PendingKeys`, and the rest).

- [ ] **Step 7: `pending.go`**

Create `tam/internal/issuerepo/pending.go`:

```go
package issuerepo

import (
	"context"
	"fmt"

	"agile-suite/core/journal"
)

const (
	// DraftPrefix starts the temporary key of an issue created locally and
	// not yet committed. Commit swaps it for Jira's key.
	DraftPrefix = "TAM-NEW-"
	// StatusDraft is the status a draft row shows until Commit creates it.
	StatusDraft = "Draft"
	// EntityIssue is the journal entity type of a field edit on an issue.
	EntityIssue = "issue"
	// EntityIssueCreate is the journal entity type of a draft's create row,
	// whose after_val is the draft as JSON.
	EntityIssueCreate = "issue_create"
	// FieldCreate is the field name on a create row.
	FieldCreate = "create"
)

// pendingFlag is the computed column every issue read carries.
const pendingFlag = `EXISTS (SELECT 1 FROM pending_change p WHERE p.profile_id = issue.profile_id AND p.entity_key = issue.key)`

// PendingKeys returns the keys with at least one pending change, sorted.
func (r *Repository) PendingKeys(ctx context.Context, profileID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT entity_key FROM pending_change WHERE profile_id = ? ORDER BY entity_key`, profileID)
	if err != nil {
		return nil, fmt.Errorf("pending keys: %w", err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListPendingChanges returns the profile's journal, newest first.
func (r *Repository) ListPendingChanges(ctx context.Context, profileID string) ([]journal.PendingChange, error) {
	return journal.List(r.db, profileID)
}

// PendingForKey returns one issue's pending changes, oldest first.
func (r *Repository) PendingForKey(ctx context.Context, profileID, key string) ([]journal.PendingChange, error) {
	return journal.ListForKey(r.db, profileID, key)
}

// ListActivity returns the audit trail for one issue, newest first.
func (r *Repository) ListActivity(ctx context.Context, profileID, key string, limit int) ([]journal.AuditEntry, error) {
	return journal.Entries(r.db, profileID, key, limit)
}
```

- [ ] **Step 8: The flags in every read and the draft-safe clear**

In `tam/internal/issuerepo/issues.go`:

Add `"strings"` is already imported. Change the column list so the computed flag rides along:

```go
const issueColumns = `key, id, project, type, summary, status, assignee, reporter, priority, labels,
	sprint_id, sprint_name, parent_key, story_points, rank, created, updated, ` + pendingFlag
```

In `UpsertPage`, replace the two clear statements so drafts survive:

```go
	if clearFirst {
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key NOT LIKE ?`, profileID, DraftPrefix+"%"); err != nil {
			return fmt.Errorf("clear links: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue WHERE profile_id = ? AND key NOT LIKE ?`, profileID, DraftPrefix+"%"); err != nil {
			return fmt.Errorf("clear issues: %w", err)
		}
	}
```

In `scanIssue`, scan the flag and derive the draft flag:

```go
func scanIssue(s scanner) (backend.Issue, error) {
	var (
		iss     backend.Issue
		labels  string
		points  sql.NullFloat64
		pending int
	)
	if err := s.Scan(&iss.Key, &iss.ID, &iss.Project, &iss.Type, &iss.Summary, &iss.Status, &iss.Assignee,
		&iss.Reporter, &iss.Priority, &labels, &iss.SprintID, &iss.SprintName, &iss.ParentKey, &points,
		&iss.Rank, &iss.Created, &iss.Updated, &pending); err != nil {
		return backend.Issue{}, err
	}
	if err := json.Unmarshal([]byte(labels), &iss.Labels); err != nil {
		return backend.Issue{}, fmt.Errorf("labels for %s: %w", iss.Key, err)
	}
	iss.Labels = nonNil(iss.Labels)
	if points.Valid {
		v := points.Float64
		iss.StoryPoints = &v
	}
	iss.Pending = pending != 0
	iss.Draft = strings.HasPrefix(iss.Key, DraftPrefix)
	return iss, nil
}
```

- [ ] **Step 9: Run the package suites**

Run (inside `tam/`):

```bash
go vet ./... && go test ./internal/tamstore/ ./internal/issuerepo/ ./internal/syncer/ -count=1
```

Expected: PASS. `TestClearFirstReplacesTheProfileOnly` in `issues_test.go` still passes because none of its keys carry the prefix. If `TestListIssuesFlagsPendingAndDraftRows` fails on ordering, note that `ListIssues` orders empty ranks last and then by key, so `PLAT-1`, `PLAT-2`, `TAM-NEW-1` is the order; the test reads by key and does not depend on it.

- [ ] **Step 10: Commit**

```bash
git add tam/internal/tamstore tam/internal/backend/backend.go tam/internal/issuerepo
git commit -m "feat(tam): schema v3 with the journal tables, pending and draft flags on issue reads"
```

---

### Task 3: The write operations on the store

Field edits, drafts, rekey, replace, discard, and the guards that keep a sync from overwriting a pending column.

**Files:**
- Create: `tam/internal/issuerepo/writes.go`, `tam/internal/issuerepo/writes_test.go`
- Modify: `tam/internal/backend/backend.go` (add `IssueDraft`), `tam/internal/issuerepo/issues.go` (`UpsertPage` re-applies pending columns), `tam/internal/issuerepo/detail.go` (`WriteDetail` keeps a pending description)
- Test: `go test ./internal/issuerepo/` inside `tam/`

**Interfaces:**
- Consumes: Task 2's constants and journal reads; `journal.Upsert`, `Put`, `Audit`, `Get`, `Delete`, `DeleteForKey`, `SetBaseVersion`.
- Produces: `backend.IssueDraft{Type, Summary, Description, Priority string; Labels []string; Assignee string; StoryPoints *float64; Extra map[string]string}`; `backend.SplitLabels(s string) []string`, `backend.ParsePoints(s string) (*float64, error)`, `backend.FormatPoints(p *float64) string`; `issuerepo.EditableFields []string`; `issuerepo.FieldValue(iss backend.Issue, description, field string) string`; on `*Repository`: `EditField(ctx, profileID, key, field, value string) error`, `CreateDraft(ctx, profileID, projectKey string, d backend.IssueDraft) (string, error)`, `Rekey(ctx, profileID, tempKey, realKey string) error`, `ReplaceRow(ctx, profileID string, iss backend.Issue) error`, `SetBaseVersion(ctx, profileID, key, version string) error`, `DiscardPendingChange(ctx, profileID string, id int64) error`, `DiscardAllPendingChanges(ctx, profileID string) (int, error)`, `MarkCommitted(ctx, profileID string, changes []journal.PendingChange) error`, `DiscardKey(ctx, profileID, key string) (int, error)`.

- [ ] **Step 1: `IssueDraft`**

In `tam/internal/backend/backend.go`, add after the `Issue` struct:

```go
// IssueDraft is a new issue as the form captured it. Type is the logical
// type (task, story, bug). Extra carries the create-meta required fields
// the form rendered as text, keyed by Jira field id; the backend sends each
// as a string, or as {"name": value} when the field takes an option.
type IssueDraft struct {
	Type        string            `json:"type"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Priority    string            `json:"priority"`
	Labels      []string          `json:"labels"`
	Assignee    string            `json:"assignee"`
	StoryPoints *float64          `json:"storyPoints"`
	Extra       map[string]string `json:"extra"`
}

// SplitLabels turns the comma list the form and the journal use back into
// Jira's label slice, trimming blanks.
func SplitLabels(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, ",") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// FormatPoints renders story points the way the journal and the forms
// show them: a plain number, or empty for none.
func FormatPoints(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

// ParsePoints reads the text form back; blank means none.
func ParsePoints(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, errors.New("story points must be a number")
	}
	return &v, nil
}
```

Add `"errors"`, `"strconv"`, and `"strings"` to the file's imports.

- [ ] **Step 2: The tests**

Create `tam/internal/issuerepo/writes_test.go`:

```go
package issuerepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

const v1 = "2026-09-01T00:00:00Z"

func seedOne(t *testing.T, repo *issuerepo.Repository, profileID string) {
	t.Helper()
	rows := []backend.Issue{{
		Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do",
		Priority: "Medium", Labels: []string{"a"}, StoryPoints: pts(3), Rank: "0|a", Updated: v1,
	}}
	if err := repo.UpsertPage(context.Background(), profileID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestEditFieldWritesTheRowAndJournalsAgainstUpdated(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Summary != "uno" || !iss.Pending {
		t.Errorf("row after edit: %+v", iss)
	}
	pend, _ := repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 1 || pend[0].Field != "summary" || pend[0].BeforeVal != "one" || pend[0].AfterVal != "uno" || pend[0].BaseVersion != v1 || pend[0].EntityType != issuerepo.EntityIssue {
		t.Errorf("journal: %+v", pend)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 1 || act[0].Action != "edit" || act[0].AfterVal != "uno" {
		t.Errorf("audit: %+v", act)
	}
	// A second edit coalesces; a return to the original drops the row.
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "eins"); err != nil {
		t.Fatal(err)
	}
	pend, _ = repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 1 || pend[0].BeforeVal != "one" || pend[0].AfterVal != "eins" {
		t.Errorf("coalesced: %+v", pend)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "one"); err != nil {
		t.Fatal(err)
	}
	pend, _ = repo.ListPendingChanges(ctx, "p1")
	iss, _ = repo.GetIssue(ctx, "p1", "PLAT-1")
	if len(pend) != 0 || iss.Pending || iss.Summary != "one" {
		t.Errorf("revert: pending=%+v row=%+v", pend, iss)
	}
}

func TestEditFieldHandlesEveryEditableField(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	if err := repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "old text"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	for field, value := range map[string]string{
		"labels": "b, c", "storyPoints": "8", "priority": "High", "assignee": "jdoe", "description": "new text",
	} {
		if err := repo.EditField(ctx, "p1", "PLAT-1", field, value); err != nil {
			t.Fatalf("edit %s: %v", field, err)
		}
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if strings.Join(iss.Labels, ",") != "b,c" || iss.StoryPoints == nil || *iss.StoryPoints != 8 || iss.Priority != "High" || iss.Assignee != "jdoe" {
		t.Errorf("row: %+v", iss)
	}
	d, _, ok, err := repo.ReadDetail(ctx, "p1", "PLAT-1")
	if err != nil || !ok || d.Description != "new text" {
		t.Errorf("description lives in the detail cache: %+v %v %v", d, ok, err)
	}
	pend, _ := repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 5 {
		t.Errorf("one journal row per field: %d", len(pend))
	}
	for _, p := range pend {
		switch p.Field {
		case "labels":
			if p.BeforeVal != "a" || p.AfterVal != "b, c" {
				t.Errorf("labels journal as a comma list: %+v", p)
			}
		case "storyPoints":
			if p.BeforeVal != "3" || p.AfterVal != "8" {
				t.Errorf("points journal as text: %+v", p)
			}
		case "description":
			if p.BeforeVal != "old text" {
				t.Errorf("description before: %+v", p)
			}
		}
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "storyPoints", ""); err != nil {
		t.Fatal(err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1"); iss.StoryPoints != nil {
		t.Errorf("empty points clears the column: %+v", iss)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "storyPoints", "eight"); err == nil {
		t.Error("non-numeric points must be rejected")
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "status", "Done"); err == nil {
		t.Error("status is not editable")
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "  "); err == nil {
		t.Error("summary cannot be blank")
	}
	if err := repo.EditField(ctx, "p1", "PLAT-9", "summary", "x"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("missing issue: %v", err)
	}
}

func TestCreateDraftNumbersPerProfileAndEditsUpdateItsJSON(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	d := backend.IssueDraft{Type: backend.TypeTask, Summary: "Add a retry", Priority: "Medium", Labels: []string{"payments"}, Assignee: "mortiz", StoryPoints: pts(3)}
	k1, err := repo.CreateDraft(ctx, "p1", "PLAT", d)
	if err != nil || k1 != "TAM-NEW-1" {
		t.Fatalf("first draft: %q %v", k1, err)
	}
	k2, _ := repo.CreateDraft(ctx, "p1", "PLAT", d)
	other, _ := repo.CreateDraft(ctx, "p2", "PLAT", d)
	if k2 != "TAM-NEW-2" || other != "TAM-NEW-1" {
		t.Errorf("numbering: %s %s", k2, other)
	}
	iss, err := repo.GetIssue(ctx, "p1", k1)
	if err != nil || !iss.Draft || !iss.Pending || iss.Status != issuerepo.StatusDraft || iss.Project != "PLAT" || iss.Summary != "Add a retry" || *iss.StoryPoints != 3 {
		t.Errorf("draft row: %+v %v", iss, err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", k1)
	if len(pend) != 1 || pend[0].EntityType != issuerepo.EntityIssueCreate || pend[0].Field != issuerepo.FieldCreate {
		t.Fatalf("create row: %+v", pend)
	}
	var stored backend.IssueDraft
	if err := json.Unmarshal([]byte(pend[0].AfterVal), &stored); err != nil || stored.Summary != "Add a retry" || stored.Type != backend.TypeTask {
		t.Errorf("create JSON: %s %v", pend[0].AfterVal, err)
	}
	if err := repo.EditField(ctx, "p1", k1, "summary", "Add a retry to the consumer"); err != nil {
		t.Fatal(err)
	}
	pend, _ = repo.PendingForKey(ctx, "p1", k1)
	_ = json.Unmarshal([]byte(pend[0].AfterVal), &stored)
	if len(pend) != 1 || stored.Summary != "Add a retry to the consumer" {
		t.Errorf("editing a draft rewrites its JSON instead of adding a row: %+v", pend)
	}
	if _, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "x"}); err == nil {
		t.Error("only task, story, and bug can be drafted in 1b")
	}
	if _, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask}); err == nil {
		t.Error("a draft needs a summary")
	}
	det, _, ok, _ := repo.ReadDetail(ctx, "p1", k1)
	if !ok || det.Description != "" {
		t.Errorf("a draft has a detail cache so the panel never asks the backend: %+v %v", det, ok)
	}
}

func TestSyncKeepsPendingColumnsAndADraft(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	if _, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "d"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine"); err != nil {
		t.Fatal(err)
	}
	if err := repo.EditField(ctx, "p1", "PLAT-1", "labels", "x, y"); err != nil {
		t.Fatal(err)
	}
	remote := []backend.Issue{{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "theirs", Status: "Done", Labels: []string{"z"}, Updated: "2026-09-02T00:00:00Z"}}
	for _, full := range []bool{false, true} {
		if err := repo.UpsertPage(ctx, "p1", remote, time.Now(), full); err != nil {
			t.Fatalf("sync full=%v: %v", full, err)
		}
		iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
		if iss.Summary != "mine" || strings.Join(iss.Labels, ",") != "x,y" {
			t.Errorf("full=%v: pending columns must win: %+v", full, iss)
		}
		if iss.Status != "Done" || iss.Updated != "2026-09-02T00:00:00Z" {
			t.Errorf("full=%v: untouched columns follow Jira: %+v", full, iss)
		}
		if _, err := repo.GetIssue(ctx, "p1", "TAM-NEW-1"); err != nil {
			t.Errorf("full=%v: the draft survives: %v", full, err)
		}
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	for _, p := range pend {
		if p.BaseVersion != v1 {
			t.Errorf("a sync never moves the base: %+v", p)
		}
	}
}

func TestWriteDetailKeepsAPendingDescription(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "old"}, time.Now())
	if err := repo.EditField(ctx, "p1", "PLAT-1", "description", "mine"); err != nil {
		t.Fatal(err)
	}
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "theirs", Links: []backend.Link{{Direction: "outward", Type: "Relates", Key: "PLAT-2"}}}, time.Now())
	d, _, _, _ := repo.ReadDetail(ctx, "p1", "PLAT-1")
	if d.Description != "mine" || len(d.Links) != 1 {
		t.Errorf("refresh keeps the pending description and takes the links: %+v", d)
	}
}

func TestRekeyMovesEveryTable(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	k, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "d"})
	if err := repo.Rekey(ctx, "p1", k, "PLAT-501"); err != nil {
		t.Fatalf("rekey: %v", err)
	}
	if _, err := repo.GetIssue(ctx, "p1", k); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("old key gone: %v", err)
	}
	iss, err := repo.GetIssue(ctx, "p1", "PLAT-501")
	if err != nil || iss.Draft || !iss.Pending {
		t.Errorf("new key carries the row and its create row: %+v %v", iss, err)
	}
	if pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-501"); len(pend) != 1 {
		t.Errorf("pending moved: %+v", pend)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-501", 0)
	if len(act) != 2 || act[0].Action != "created" || act[0].BeforeVal != k || act[1].Action != "create" {
		t.Errorf("audit moved and gained the created entry: %+v", act)
	}
}

func TestReplaceRowOverwritesAndClearsTheDetailCache(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.WriteDetail(ctx, "p1", "PLAT-1", backend.IssueDetail{Description: "cached"}, time.Now())
	fresh := backend.Issue{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "fresh", Status: "Done", Labels: []string{}, Updated: "2026-09-03T00:00:00Z"}
	if err := repo.ReplaceRow(ctx, "p1", fresh); err != nil {
		t.Fatalf("replace: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Summary != "fresh" || iss.Status != "Done" || iss.Updated != "2026-09-03T00:00:00Z" || iss.StoryPoints != nil {
		t.Errorf("row: %+v", iss)
	}
	if _, _, ok, _ := repo.ReadDetail(ctx, "p1", "PLAT-1"); ok {
		t.Error("the detail cache is dropped so the panel refetches")
	}
}

func TestDiscardRevertsAnEditOrDropsADraft(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "priority", "High")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "labels", "q")
	k, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "d"})
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	var prio journal.PendingChange
	for _, p := range pend {
		if p.Field == "priority" {
			prio = p
		}
	}
	if err := repo.DiscardPendingChange(ctx, "p1", prio.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Priority != "Medium" || strings.Join(iss.Labels, ",") != "q" || !iss.Pending {
		t.Errorf("only the discarded field reverts: %+v", iss)
	}
	if err := repo.DiscardPendingChange(ctx, "p1", 9999); !errors.Is(err, journal.ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
	n, err := repo.DiscardAllPendingChanges(ctx, "p1")
	if err != nil || n != 2 {
		t.Fatalf("discard all: %d %v", n, err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1"); iss.Pending || strings.Join(iss.Labels, ",") != "a" {
		t.Errorf("labels reverted: %+v", iss)
	}
	if _, err := repo.GetIssue(ctx, "p1", k); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("discarding a create row removes the draft: %v", err)
	}
	act, _ := repo.ListActivity(ctx, "p1", "", 0)
	discards := 0
	for _, a := range act {
		if a.Action == "discard" {
			discards++
		}
	}
	if discards != 3 {
		t.Errorf("every discard is audited: %d of %+v", discards, act)
	}
}

func TestMarkCommittedSetBaseVersionAndDiscardKey(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedOne(t, repo, "p1")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "assignee", "me")
	if err := repo.SetBaseVersion(ctx, "p1", "PLAT-1", "v9"); err != nil {
		t.Fatal(err)
	}
	pend, _ := repo.PendingForKey(ctx, "p1", "PLAT-1")
	for _, p := range pend {
		if p.BaseVersion != "v9" {
			t.Errorf("rebased: %+v", p)
		}
	}
	if err := repo.MarkCommitted(ctx, "p1", pend[:1]); err != nil {
		t.Fatal(err)
	}
	if rest, _ := repo.PendingForKey(ctx, "p1", "PLAT-1"); len(rest) != 1 {
		t.Errorf("one row committed, one left: %+v", rest)
	}
	n, err := repo.DiscardKey(ctx, "p1", "PLAT-1")
	if err != nil || n != 1 {
		t.Errorf("DiscardKey: %d %v", n, err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1"); iss.Pending {
		t.Errorf("nothing pending after DiscardKey: %+v", iss)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	seen := map[string]int{}
	for _, a := range act {
		seen[a.Action]++
	}
	if seen["override"] != 1 || seen["commit"] != 1 || seen["discard"] != 1 {
		t.Errorf("audit actions: %v", seen)
	}
}

func TestFieldValueAndSplitLabels(t *testing.T) {
	iss := backend.Issue{Summary: "s", Priority: "p", Assignee: "a", Labels: []string{"x", "y"}, StoryPoints: pts(2.5)}
	for field, want := range map[string]string{"summary": "s", "priority": "p", "assignee": "a", "labels": "x, y", "storyPoints": "2.5", "description": "text"} {
		if got := issuerepo.FieldValue(iss, "text", field); got != want {
			t.Errorf("FieldValue(%s) = %q, want %q", field, got, want)
		}
	}
	if got := issuerepo.FieldValue(backend.Issue{}, "", "storyPoints"); got != "" {
		t.Errorf("nil points = %q", got)
	}
	if got := backend.SplitLabels(" a ,, b,c "); strings.Join(got, "|") != "a|b|c" {
		t.Errorf("SplitLabels = %v", got)
	}
	if got := backend.SplitLabels(""); len(got) != 0 {
		t.Errorf("empty = %v", got)
	}
	if got := backend.FormatPoints(pts(2.5)); got != "2.5" {
		t.Errorf("FormatPoints = %q", got)
	}
	if _, err := backend.ParsePoints("eight"); err == nil {
		t.Error("ParsePoints must reject text")
	}
}
```

- [ ] **Step 3: Run them**

Run (inside `tam/`): `go test ./internal/issuerepo/ -run 'EditField|CreateDraft|SyncKeeps|WriteDetailKeeps|Rekey|ReplaceRow|Discard|MarkCommitted|FieldValue'`
Expected: compile failure on the undefined methods.

- [ ] **Step 4: `writes.go`**

Create `tam/internal/issuerepo/writes.go`:

```go
package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// EditableFields are the fields EditField accepts, in the order the panel
// shows them. Their names are the JSON names on backend.Issue, plus
// description, which lives in the detail cache.
var EditableFields = []string{"summary", "description", "priority", "labels", "storyPoints", "assignee"}

// fieldColumns maps a field name to its issue column; description has none.
var fieldColumns = map[string]string{
	"summary": "summary", "description": "", "priority": "priority",
	"labels": "labels", "storyPoints": "story_points", "assignee": "assignee",
}

// draftTypes are the logical types a draft may have in plan 1b.
var draftTypes = map[string]bool{backend.TypeTask: true, backend.TypeStory: true, backend.TypeBug: true}

// execer is the subset of *sql.Tx and *sql.DB the field helpers use.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// FieldValue renders one editable field of an issue as the journal and the
// conflict table show it: labels as a comma list, points as a plain
// number, description from the detail cache the caller passes.
func FieldValue(iss backend.Issue, description, field string) string {
	switch field {
	case "summary":
		return iss.Summary
	case "description":
		return description
	case "priority":
		return iss.Priority
	case "assignee":
		return iss.Assignee
	case "labels":
		return strings.Join(iss.Labels, ", ")
	case "storyPoints":
		return backend.FormatPoints(iss.StoryPoints)
	}
	return ""
}

func validateField(field, value string) error {
	if _, ok := fieldColumns[field]; !ok {
		return fmt.Errorf("field %q cannot be edited", field)
	}
	switch field {
	case "summary":
		if strings.TrimSpace(value) == "" {
			return errors.New("summary cannot be empty")
		}
	case "storyPoints":
		if _, err := backend.ParsePoints(value); err != nil {
			return err
		}
	}
	return nil
}

// readField returns the current text form of a field and the row's
// updated stamp, which is the base version of any edit made now.
func readField(ctx context.Context, q execer, profileID, key, field string) (string, string, error) {
	var (
		iss     backend.Issue
		labels  string
		points  sql.NullFloat64
		detail  sql.NullString
		updated string
	)
	err := q.QueryRowContext(ctx,
		`SELECT summary, priority, assignee, labels, story_points, detail_json, updated FROM issue WHERE profile_id = ? AND key = ?`,
		profileID, key).Scan(&iss.Summary, &iss.Priority, &iss.Assignee, &labels, &points, &detail, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", key, err)
	}
	_ = json.Unmarshal([]byte(labels), &iss.Labels)
	if points.Valid {
		v := points.Float64
		iss.StoryPoints = &v
	}
	description := ""
	if detail.Valid && detail.String != "" {
		var d backend.IssueDetail
		if err := json.Unmarshal([]byte(detail.String), &d); err == nil {
			description = d.Description
		}
	}
	return FieldValue(iss, description, field), updated, nil
}

// writeField stores the text form of a field on the row. Description goes
// into the detail cache, creating a minimal one when none exists so the
// panel can show the edit.
func writeField(ctx context.Context, q execer, profileID, key, field, value string) error {
	switch field {
	case "description":
		var (
			raw       sql.NullString
			fetchedAt sql.NullString
		)
		if err := q.QueryRowContext(ctx, `SELECT detail_json, detail_fetched_at FROM issue WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&raw, &fetchedAt); err != nil {
			return fmt.Errorf("read detail for %s: %w", key, err)
		}
		d := backend.IssueDetail{Key: key, Fields: map[string]any{}}
		if raw.Valid && raw.String != "" {
			_ = json.Unmarshal([]byte(raw.String), &d)
		}
		d.Description = value
		d.Links = nil
		encoded, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("encode detail for %s: %w", key, err)
		}
		at := fetchedAt.String
		if at == "" {
			at = time.Now().UTC().Format(time.RFC3339)
		}
		_, err = q.ExecContext(ctx, `UPDATE issue SET detail_json = ?, detail_fetched_at = ? WHERE profile_id = ? AND key = ?`, string(encoded), at, profileID, key)
		return err
	case "labels":
		encoded, _ := json.Marshal(backend.SplitLabels(value))
		_, err := q.ExecContext(ctx, `UPDATE issue SET labels = ? WHERE profile_id = ? AND key = ?`, string(encoded), profileID, key)
		return err
	case "storyPoints":
		p, err := backend.ParsePoints(value)
		if err != nil {
			return err
		}
		var points sql.NullFloat64
		if p != nil {
			points = sql.NullFloat64{Float64: *p, Valid: true}
		}
		_, err = q.ExecContext(ctx, `UPDATE issue SET story_points = ? WHERE profile_id = ? AND key = ?`, points, profileID, key)
		return err
	}
	col, ok := fieldColumns[field]
	if !ok || col == "" {
		return fmt.Errorf("field %q cannot be edited", field)
	}
	_, err := q.ExecContext(ctx, `UPDATE issue SET `+col+` = ? WHERE profile_id = ? AND key = ?`, value, profileID, key)
	return err
}

// reapplyPending rewrites the columns of every pending field edit for the
// given keys, so a sync that just refreshed those rows from Jira does not
// hide a local edit. The journal's base version is left alone: if Jira did
// change, the next Commit sees it.
func reapplyPending(ctx context.Context, q execer, profileID string, keys map[string]bool) error {
	if len(keys) == 0 {
		return nil
	}
	rows, err := q.QueryContext(ctx,
		`SELECT entity_key, field, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? ORDER BY id`,
		profileID, EntityIssue)
	if err != nil {
		return fmt.Errorf("pending fields: %w", err)
	}
	type edit struct{ key, field, value string }
	var edits []edit
	for rows.Next() {
		var e edit
		if err := rows.Scan(&e.key, &e.field, &e.value); err != nil {
			rows.Close()
			return err
		}
		if keys[e.key] {
			edits = append(edits, e)
		}
	}
	rows.Close()
	for _, e := range edits {
		if err := writeField(ctx, q, profileID, e.key, e.field, e.value); err != nil {
			return fmt.Errorf("reapply %s.%s: %w", e.key, e.field, err)
		}
	}
	return nil
}

// EditField changes one field on the row and journals it. The edit's base
// version is the row's updated stamp. On a draft the create row's JSON is
// rewritten instead of adding a second journal row.
func (r *Repository) EditField(ctx context.Context, profileID, key, field, value string) error {
	if err := validateField(field, value); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	current, updated, err := readField(ctx, tx, profileID, key, field)
	if err != nil {
		return err
	}
	if current == value {
		return nil
	}
	if err := writeField(ctx, tx, profileID, key, field, value); err != nil {
		return err
	}
	if strings.HasPrefix(key, DraftPrefix) {
		if err := updateDraftJSON(ctx, tx, profileID, key, field, value); err != nil {
			return err
		}
	} else if err := journal.Upsert(tx, profileID, EntityIssue, key, field, current, value, updated); err != nil {
		return err
	}
	if err := journal.Audit(tx, profileID, EntityIssue, key, "edit", field, current, value, ""); err != nil {
		return err
	}
	return tx.Commit()
}

func updateDraftJSON(ctx context.Context, tx *sql.Tx, profileID, key, field, value string) error {
	var raw string
	err := tx.QueryRowContext(ctx,
		`SELECT after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		profileID, EntityIssueCreate, key).Scan(&raw)
	if err != nil {
		return fmt.Errorf("draft %s has no create row: %w", key, err)
	}
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return fmt.Errorf("decode draft %s: %w", key, err)
	}
	switch field {
	case "summary":
		d.Summary = value
	case "description":
		d.Description = value
	case "priority":
		d.Priority = value
	case "assignee":
		d.Assignee = value
	case "labels":
		d.Labels = backend.SplitLabels(value)
	case "storyPoints":
		d.StoryPoints, _ = backend.ParsePoints(value)
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		string(encoded), profileID, EntityIssueCreate, key)
	return err
}

// CreateDraft inserts a placeholder row under the next temporary key and a
// create row holding the draft as JSON. It returns the temporary key.
func (r *Repository) CreateDraft(ctx context.Context, profileID, projectKey string, d backend.IssueDraft) (string, error) {
	if !draftTypes[d.Type] {
		return "", fmt.Errorf("type %q cannot be created here; plan 1b creates tasks, stories, and bugs", d.Type)
	}
	if strings.TrimSpace(d.Summary) == "" {
		return "", errors.New("summary cannot be empty")
	}
	d.Summary = strings.TrimSpace(d.Summary)
	if d.Labels == nil {
		d.Labels = []string{}
	}
	if d.Extra == nil {
		d.Extra = map[string]string{}
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("encode draft: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var last int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(CAST(SUBSTR(key, ?) AS INTEGER)), 0) FROM issue WHERE profile_id = ? AND key LIKE ?`,
		len(DraftPrefix)+1, profileID, DraftPrefix+"%").Scan(&last); err != nil {
		return "", fmt.Errorf("next draft key: %w", err)
	}
	key := fmt.Sprintf("%s%d", DraftPrefix, last+1)
	now := time.Now().UTC().Format(time.RFC3339)
	labels, _ := json.Marshal(d.Labels)
	var points sql.NullFloat64
	if d.StoryPoints != nil {
		points = sql.NullFloat64{Float64: *d.StoryPoints, Valid: true}
	}
	detail, _ := json.Marshal(backend.IssueDetail{Key: key, Description: d.Description, Fields: map[string]any{}})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO issue (profile_id, key, id, project, type, summary, status, assignee, reporter, priority, labels,
			sprint_id, sprint_name, parent_key, story_points, rank, created, updated, synced_at, detail_json, detail_fetched_at)
		VALUES (?, ?, '', ?, ?, ?, ?, ?, '', ?, ?, '', '', '', ?, '', ?, '', '', ?, ?)`,
		profileID, key, projectKey, d.Type, d.Summary, StatusDraft, d.Assignee, d.Priority, string(labels),
		points, now, string(detail), now); err != nil {
		return "", fmt.Errorf("insert draft: %w", err)
	}
	if err := journal.Put(tx, profileID, EntityIssueCreate, key, FieldCreate, "", string(encoded), ""); err != nil {
		return "", err
	}
	if err := journal.Audit(tx, profileID, EntityIssue, key, "create", "", "", d.Summary, ""); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return key, nil
}

// Rekey moves a draft to the key Jira assigned, across the row, its links,
// its journal rows, and its audit trail, and audits the creation.
func (r *Repository) Rekey(ctx context.Context, profileID, tempKey, realKey string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`UPDATE issue SET key = ? WHERE profile_id = ? AND key = ?`,
		`UPDATE issue_link SET from_key = ? WHERE profile_id = ? AND from_key = ?`,
		`UPDATE pending_change SET entity_key = ? WHERE profile_id = ? AND entity_key = ?`,
		`UPDATE audit_log SET entity_key = ? WHERE profile_id = ? AND entity_key = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, realKey, profileID, tempKey); err != nil {
			return fmt.Errorf("rekey %s to %s: %w", tempKey, realKey, err)
		}
	}
	if err := journal.Audit(tx, profileID, EntityIssue, realKey, "created", "", tempKey, realKey, "created in Jira"); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceRow overwrites every column of one issue from a fresh Jira read and
// drops its detail cache so the panel refetches. It does not touch the
// journal; callers delete the rows first when that is the intent.
func (r *Repository) ReplaceRow(ctx context.Context, profileID string, iss backend.Issue) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertIssue(ctx, tx, profileID, iss, time.Now()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE issue SET detail_json = NULL, detail_fetched_at = NULL WHERE profile_id = ? AND key = ?`,
		profileID, iss.Key); err != nil {
		return fmt.Errorf("clear detail for %s: %w", iss.Key, err)
	}
	return tx.Commit()
}

// SetBaseVersion rebases an issue's pending changes onto the remote version
// the user chose to override, and audits the choice.
func (r *Repository) SetBaseVersion(ctx context.Context, profileID, key, version string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := journal.SetBaseVersion(tx, profileID, key, version); err != nil {
		return err
	}
	if err := journal.Audit(tx, profileID, EntityIssue, key, "override", "", "", version, "pending edits rebased onto the remote version"); err != nil {
		return err
	}
	return tx.Commit()
}

// DiscardPendingChange reverts one journal row: a field edit goes back to
// its before value, a create row takes its draft row with it.
func (r *Repository) DiscardPendingChange(ctx context.Context, profileID string, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := journal.Get(tx, profileID, id)
	if err != nil {
		return err
	}
	if err := discardOne(ctx, tx, profileID, p); err != nil {
		return err
	}
	return tx.Commit()
}

// DiscardAllPendingChanges reverts every journal row of the profile and
// returns how many it reverted.
func (r *Repository) DiscardAllPendingChanges(ctx context.Context, profileID string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	all, err := journal.List(tx, profileID)
	if err != nil {
		return 0, err
	}
	for _, p := range all {
		if err := discardOne(ctx, tx, profileID, p); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(all), nil
}

// DiscardKey reverts every pending change of one issue, which is what a
// keep-remote resolution does before it replaces the row.
func (r *Repository) DiscardKey(ctx context.Context, profileID, key string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := journal.ListForKey(tx, profileID, key)
	if err != nil {
		return 0, err
	}
	for _, p := range rows {
		if err := discardOne(ctx, tx, profileID, p); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func discardOne(ctx context.Context, tx *sql.Tx, profileID string, p journal.PendingChange) error {
	if p.EntityType == EntityIssueCreate {
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key = ?`, profileID, p.EntityKey); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue WHERE profile_id = ? AND key = ?`, profileID, p.EntityKey); err != nil {
			return fmt.Errorf("drop draft %s: %w", p.EntityKey, err)
		}
	} else if err := writeField(ctx, tx, profileID, p.EntityKey, p.Field, p.BeforeVal); err != nil {
		return fmt.Errorf("revert %s.%s: %w", p.EntityKey, p.Field, err)
	}
	if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "discard", p.Field, p.AfterVal, p.BeforeVal, "")
}

// MarkCommitted deletes the journal rows a commit pushed and audits each.
func (r *Repository) MarkCommitted(ctx context.Context, profileID string, changes []journal.PendingChange) error {
	if len(changes) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := make([]int64, 0, len(changes))
	for _, p := range changes {
		ids = append(ids, p.ID)
		if err := journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "commit", p.Field, p.BeforeVal, p.AfterVal, ""); err != nil {
			return err
		}
	}
	if err := journal.Delete(tx, profileID, ids); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 5: Factor the single-row upsert out of `UpsertPage` and add the guard**

In `tam/internal/issuerepo/issues.go`, replace `UpsertPage` so the statement is shared with `ReplaceRow` and pending columns are re-applied after the page lands:

```go
const upsertIssueSQL = `
	INSERT INTO issue (profile_id, key, id, project, type, summary, status, assignee, reporter, priority, labels,
		sprint_id, sprint_name, parent_key, story_points, rank, created, updated, synced_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(profile_id, key) DO UPDATE SET
		id = excluded.id, project = excluded.project, type = excluded.type, summary = excluded.summary,
		status = excluded.status, assignee = excluded.assignee, reporter = excluded.reporter,
		priority = excluded.priority, labels = excluded.labels, sprint_id = excluded.sprint_id,
		sprint_name = excluded.sprint_name, parent_key = excluded.parent_key,
		story_points = excluded.story_points, rank = excluded.rank, created = excluded.created,
		updated = excluded.updated, synced_at = excluded.synced_at`

func upsertIssue(ctx context.Context, q execer, profileID string, iss backend.Issue, syncedAt time.Time) error {
	labels, err := json.Marshal(nonNil(iss.Labels))
	if err != nil {
		return fmt.Errorf("labels for %s: %w", iss.Key, err)
	}
	var points sql.NullFloat64
	if iss.StoryPoints != nil {
		points = sql.NullFloat64{Float64: *iss.StoryPoints, Valid: true}
	}
	if _, err := q.ExecContext(ctx, upsertIssueSQL, profileID, iss.Key, iss.ID, iss.Project, iss.Type, iss.Summary, iss.Status,
		iss.Assignee, iss.Reporter, iss.Priority, string(labels), iss.SprintID, iss.SprintName, iss.ParentKey,
		points, iss.Rank, iss.Created, iss.Updated, syncedAt.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("upsert %s: %w", iss.Key, err)
	}
	return nil
}

// UpsertPage lands one page from Jira. With clearFirst the profile's synced
// rows go first (drafts stay). Columns with a pending edit are written back
// from the journal afterwards, so a sync never hides a local change.
func (r *Repository) UpsertPage(ctx context.Context, profileID string, page []backend.Issue, syncedAt time.Time, clearFirst bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if clearFirst {
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key NOT LIKE ?`, profileID, DraftPrefix+"%"); err != nil {
			return fmt.Errorf("clear links: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue WHERE profile_id = ? AND key NOT LIKE ?`, profileID, DraftPrefix+"%"); err != nil {
			return fmt.Errorf("clear issues: %w", err)
		}
	}
	keys := make(map[string]bool, len(page))
	for _, iss := range page {
		if err := upsertIssue(ctx, tx, profileID, iss, syncedAt); err != nil {
			return err
		}
		keys[iss.Key] = true
	}
	if err := reapplyPending(ctx, tx, profileID, keys); err != nil {
		return err
	}
	return tx.Commit()
}
```

The prepared statement is gone; `modernc.org/sqlite` caches compiled statements per connection, and the page size is 50, so the per-row `ExecContext` costs nothing measurable. Remove the now unused `stmt` code entirely.

- [ ] **Step 6: `WriteDetail` keeps a pending description**

In `tam/internal/issuerepo/detail.go`, inside `WriteDetail` just before `return tx.Commit()` (after the links loop), add:

```go
	if err := reapplyPending(ctx, tx, profileID, map[string]bool{key: true}); err != nil {
		return err
	}
```

- [ ] **Step 7: Run the suites**

Run (inside `tam/`):

```bash
gofmt -l ./internal && go vet ./... && go test ./internal/... -count=1
```

Expected: `gofmt` prints nothing; every package PASS. The syncer suite runs unchanged against the new `UpsertPage`.

- [ ] **Step 8: Commit**

```bash
git add tam/internal/backend/backend.go tam/internal/issuerepo
git commit -m "feat(tam): journaled field edits, drafts, rekey, replace, and discard on the store"
```

---

### Task 4: The backend write methods, in Jira and in the demo

Four methods join `IssueBackend`: read one issue, update fields, create an issue, and list the create-meta fields a type requires. The Jira implementation maps the six editable fields to Jira's shapes; the demo keeps an in-memory overlay and stages one conflict.

**Files:**
- Modify: `tam/internal/backend/backend.go` (`FieldSpec`, `FieldOption`, the interface)
- Create: `tam/internal/backend/jira/writes.go`, `tam/internal/backend/jira/writes_test.go`
- Modify: `tam/internal/backend/jira/jira_test.go` (the fake's handler gains the write endpoints)
- Modify: `tam/internal/backend/demo/demo.go`, `tam/internal/backend/demo/demo_test.go`
- Modify: `tam/internal/syncer/syncer_test.go` (the `fake` gains the four methods)
- Test: `go test ./internal/backend/... ./internal/syncer/` inside `tam/`

**Interfaces:**
- Consumes: `backend.IssueDraft`, `backend.SplitLabels`, `backend.ParsePoints` from Task 3; `parseIssue`, `fieldIDs`, `baseFields`, `jiraTypeNames` already in the jira package.
- Produces: `backend.FieldSpec{ID, Name, Type string; Required bool; AllowedValues []FieldOption}`, `backend.FieldOption{ID, Value string}`; on `IssueBackend`: `GetIssue(ctx, key string) (Issue, error)`, `UpdateIssue(ctx, key string, fields map[string]string) error` (logical field name to text value, the six editable fields), `CreateIssue(ctx, projectKey string, d IssueDraft) (string, error)`, `CreateFields(ctx, projectKey, logicalType string) ([]FieldSpec, error)`; `demo.Backend.ConflictKey()` for tests.

- [ ] **Step 1: The types and the interface**

In `tam/internal/backend/backend.go`, add after `IssueDraft`:

```go
// FieldSpec is one create-meta field the New issue form must ask for
// because Jira requires it and the form does not already carry it. Type is
// string, option, number, date, or array; option and array fields list
// their AllowedValues.
type FieldSpec struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	Required      bool          `json:"required"`
	AllowedValues []FieldOption `json:"allowedValues"`
}

// FieldOption is one allowed value of an option field.
type FieldOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}
```

Extend the `IssueBackend` interface with four methods after `IssueTypes`:

```go
	// GetIssue reads one issue's row fields, for the version check before a
	// commit and the refresh after it.
	GetIssue(ctx context.Context, key string) (Issue, error)
	// UpdateIssue pushes edited fields, keyed by the logical field name
	// (summary, description, priority, labels, storyPoints, assignee) with
	// the text form the journal holds.
	UpdateIssue(ctx context.Context, key string, fields map[string]string) error
	// CreateIssue creates the draft and returns the key Jira assigned.
	CreateIssue(ctx context.Context, projectKey string, d IssueDraft) (string, error)
	// CreateFields lists the required create-meta fields of a logical type
	// that the New issue form does not already carry.
	CreateFields(ctx context.Context, projectKey, logicalType string) ([]FieldSpec, error)
```

- [ ] **Step 2: The syncer's fake grows the methods**

In `tam/internal/syncer/syncer_test.go`, after `IssueTypes` on `fake`, add:

```go
func (f *fake) GetIssue(context.Context, string) (backend.Issue, error) {
	return backend.Issue{}, errors.New("not used")
}
func (f *fake) UpdateIssue(context.Context, string, map[string]string) error { return errors.New("not used") }
func (f *fake) CreateIssue(context.Context, string, backend.IssueDraft) (string, error) {
	return "", errors.New("not used")
}
func (f *fake) CreateFields(context.Context, string, string) ([]backend.FieldSpec, error) {
	return nil, errors.New("not used")
}
```

Run `go build ./... && go vet ./...` inside `tam/`. Expected: the demo and jira backends fail to satisfy the interface wherever they are assigned to it (`app_issues.go`, the syncer test). That is the failure the rest of this task fixes.

- [ ] **Step 3: Jira tests**

Extend the fake server in `tam/internal/backend/jira/jira_test.go`. Add fields to `fakeJira`:

```go
type fakeJira struct {
	fieldCalls int32
	searches   []string
	fields     string // the /rest/api/2/field body
	writes     []string // "METHOD path body" for every PUT and POST
	createKey  string   // key the POST /issue answers with
	createFail bool     // POST /issue answers 400
}
```

And extend the handler's `switch` with these cases placed before the `strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/PLAT-412")` case, so writes and the createmeta endpoint are matched first:

```go
		case r.Method == http.MethodPut || r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			f.writes = append(f.writes, r.Method+" "+r.URL.Path+" "+string(body))
			if r.URL.Path == "/rest/api/2/issue" {
				if f.createFail {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"errorMessages":[],"errors":{"customfield_10050":"Severity is required."}}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"9001","key":"` + f.createKey + `"}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/2/issue/createmeta":
			f.searches = append(f.searches, "createmeta "+r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"projects":[{"key":"PLAT","issuetypes":[{"name":"Bug","fields":{
				"summary":{"required":true,"name":"Summary","schema":{"type":"string"}},
				"project":{"required":true,"name":"Project","schema":{"type":"project"}},
				"issuetype":{"required":true,"name":"Issue Type","schema":{"type":"issuetype"}},
				"customfield_10016":{"required":true,"name":"Story Points","schema":{"type":"number"}},
				"customfield_10050":{"required":true,"name":"Severity","schema":{"type":"option"},"allowedValues":[{"id":"1","value":"Minor"},{"id":"3","value":"Critical"}]},
				"components":{"required":true,"name":"Component/s","schema":{"type":"array","items":"component"},"allowedValues":[{"id":"100","name":"Checkout"}]},
				"environment":{"required":false,"name":"Environment","schema":{"type":"string"}}
			}}]}]}`))
```

Add `"io"` to the test file's imports. Then create `tam/internal/backend/jira/writes_test.go`:

```go
package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestGetIssueParsesLikeSearch(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	iss, err := b.GetIssue(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if iss.Key != "PLAT-412" || iss.StoryPoints == nil || *iss.StoryPoints != 5 {
		t.Errorf("issue = %+v", iss)
	}
}

func TestUpdateIssueMapsTheSixFields(t *testing.T) {
	b, f := newBackend(t, twoFields)
	err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{
		"summary": "New title", "description": "Body", "priority": "High", "assignee": "jdoe",
		"labels": "checkout, promo", "storyPoints": "8",
	})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if len(f.writes) != 1 || !strings.HasPrefix(f.writes[0], "PUT /rest/api/2/issue/PLAT-412 ") {
		t.Fatalf("writes = %v", f.writes)
	}
	body := f.writes[0]
	for _, want := range []string{
		`"summary":"New title"`, `"description":"Body"`, `"priority":{"name":"High"}`, `"assignee":{"name":"jdoe"}`,
		`"labels":["checkout","promo"]`, `"customfield_10016":8`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %s: %s", want, body)
		}
	}
	// Clearing assignee and points sends null; an unknown field is refused
	// before any request; points without the custom field are refused too.
	f.writes = nil
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"assignee": "", "storyPoints": ""}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.writes[0], `"assignee":null`) || !strings.Contains(f.writes[0], `"customfield_10016":null`) {
		t.Errorf("clears: %s", f.writes[0])
	}
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"status": "Done"}); err == nil {
		t.Error("status must be refused")
	}
	noPoints, f2 := newBackend(t, `[]`)
	if err := noPoints.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"storyPoints": "3"}); err == nil || len(f2.writes) != 0 {
		t.Errorf("points without the field: err=%v writes=%v", err, f2.writes)
	}
}

func TestCreateIssuePostsTheDraftAndReturnsTheKey(t *testing.T) {
	b, f := newBackend(t, twoFields)
	f.createKey = "PLAT-501"
	key, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeBug, Summary: "Promo field accepts spaces", Description: "Steps", Priority: "Low",
		Labels: []string{"promo"}, Assignee: "jdoe", StoryPoints: pts(1),
		Extra: map[string]string{"customfield_10050": "3", "components": "100", "customfield_10060": "free text"},
	})
	if err != nil || key != "PLAT-501" {
		t.Fatalf("CreateIssue: %q %v", key, err)
	}
	var post string
	for _, w := range f.writes {
		if strings.HasPrefix(w, "POST /rest/api/2/issue ") {
			post = w
		}
	}
	for _, want := range []string{
		`"project":{"key":"PLAT"}`, `"issuetype":{"name":"Bug"}`, `"summary":"Promo field accepts spaces"`,
		`"description":"Steps"`, `"priority":{"name":"Low"}`, `"labels":["promo"]`, `"assignee":{"name":"jdoe"}`,
		`"customfield_10016":1`, `"customfield_10050":{"id":"3"}`, `"components":[{"id":"100"}]`, `"customfield_10060":"free text"`,
	} {
		if !strings.Contains(post, want) {
			t.Errorf("POST lacks %s: %s", want, post)
		}
	}
	f.createFail = true
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "x"}); err == nil || !strings.Contains(err.Error(), "Severity is required") {
		t.Errorf("Jira's message must surface: %v", err)
	}
}

func TestCreateFieldsKeepsOnlyRequiredUnknownFields(t *testing.T) {
	b, f := newBackend(t, twoFields)
	specs, err := b.CreateFields(context.Background(), "PLAT", backend.TypeBug)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	var seen []string
	for _, s := range specs {
		seen = append(seen, s.ID+":"+s.Type)
	}
	if strings.Join(seen, ",") != "components:array,customfield_10050:option" {
		t.Errorf("specs = %v", seen)
	}
	if specs[1].Name != "Severity" || len(specs[1].AllowedValues) != 2 || specs[1].AllowedValues[1].Value != "Critical" {
		t.Errorf("severity = %+v", specs[1])
	}
	if specs[0].AllowedValues[0].Value != "Checkout" {
		t.Errorf("array options take name when value is empty: %+v", specs[0])
	}
	found := false
	for _, s := range f.searches {
		if strings.HasPrefix(s, "createmeta ") && strings.Contains(s, "projectKeys=PLAT") && strings.Contains(s, "issuetypeNames=Bug") && strings.Contains(s, "expand=projects.issuetypes.fields") {
			found = true
		}
	}
	if !found {
		t.Errorf("createmeta query: %v", f.searches)
	}
}

func pts(v float64) *float64 { return &v }
```

The jira test package has no `pts` helper yet, so this one stays.

- [ ] **Step 4: Run them**

Run (inside `tam/`): `go test ./internal/backend/jira/`
Expected: compile failure, the four methods do not exist.

- [ ] **Step 5: `writes.go` in the Jira backend**

Create `tam/internal/backend/jira/writes.go`:

```go
package jira

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"agile-suite/tam/internal/backend"
)

// GetIssue reads one issue with the same fields the search asks for, so it
// parses to the same row.
func (b *Backend) GetIssue(ctx context.Context, key string) (backend.Issue, error) {
	ids := b.discover(ctx)
	fields := append(append([]string{}, baseFields...), ids.list()...)
	raw, err := b.c.GetIssue(ctx, key, fields)
	if err != nil {
		return backend.Issue{}, err
	}
	return parseIssue(raw, ids, b.requirementType), nil
}

// jiraFields turns the journal's text values into Jira's field shapes. An
// empty priority, assignee, or points clears the field with null.
func jiraFields(fields map[string]string, ids fieldIDs) (map[string]any, error) {
	out := map[string]any{}
	for name, v := range fields {
		switch name {
		case "summary":
			out["summary"] = v
		case "description":
			out["description"] = v
		case "priority":
			out["priority"] = nameOrNull(v)
		case "assignee":
			out["assignee"] = nameOrNull(v)
		case "labels":
			out["labels"] = backend.SplitLabels(v)
		case "storyPoints":
			if ids.Points == "" {
				return nil, errors.New("this Jira has no Story Points field, so points cannot be pushed")
			}
			p, err := backend.ParsePoints(v)
			if err != nil {
				return nil, err
			}
			if p == nil {
				out[ids.Points] = nil
			} else {
				out[ids.Points] = *p
			}
		default:
			return nil, fmt.Errorf("field %q cannot be sent to Jira", name)
		}
	}
	return out, nil
}

func nameOrNull(v string) any {
	if v == "" {
		return nil
	}
	return map[string]string{"name": v}
}

// UpdateIssue PUTs the edited fields. Jira answers 204 on success and a
// 400 with a per-field message otherwise; the client's error carries it.
func (b *Backend) UpdateIssue(ctx context.Context, key string, fields map[string]string) error {
	ids := b.discover(ctx)
	jf, err := jiraFields(fields, ids)
	if err != nil {
		return err
	}
	return b.c.Put(ctx, "/rest/api/2/issue/"+url.PathEscape(key), map[string]any{"fields": jf})
}

// CreateIssue POSTs the draft. Extra values are shaped from the type's
// create-meta: option fields as {"id"}, arrays as [{"id"}], numbers as
// numbers, everything else as the text entered. If the meta cannot be read
// the values go as text and Jira's own validation decides.
func (b *Backend) CreateIssue(ctx context.Context, projectKey string, d backend.IssueDraft) (string, error) {
	ids := b.discover(ctx)
	names := jiraTypeNames([]string{d.Type}, b.requirementType)
	if len(names) == 0 {
		return "", fmt.Errorf("unknown issue type %q", d.Type)
	}
	fields := map[string]any{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"name": names[0]},
		"summary":   d.Summary,
	}
	if d.Description != "" {
		fields["description"] = d.Description
	}
	if d.Priority != "" {
		fields["priority"] = map[string]string{"name": d.Priority}
	}
	if d.Assignee != "" {
		fields["assignee"] = map[string]string{"name": d.Assignee}
	}
	if len(d.Labels) > 0 {
		fields["labels"] = d.Labels
	}
	if d.StoryPoints != nil && ids.Points != "" {
		fields[ids.Points] = *d.StoryPoints
	}
	if len(d.Extra) > 0 {
		kinds := map[string]string{}
		if specs, err := b.CreateFields(ctx, projectKey, d.Type); err == nil {
			for _, s := range specs {
				kinds[s.ID] = s.Type
			}
		}
		for id, v := range d.Extra {
			if v == "" {
				continue
			}
			fields[id] = shapeExtra(kinds[id], v)
		}
	}
	var resp struct {
		Key string `json:"key"`
	}
	if err := b.c.WriteJSONReturning(ctx, http.MethodPost, "/rest/api/2/issue", map[string]any{"fields": fields}, &resp); err != nil {
		return "", err
	}
	if resp.Key == "" {
		return "", errors.New("Jira created the issue but returned no key")
	}
	return resp.Key, nil
}

func shapeExtra(kind, v string) any {
	switch kind {
	case "option":
		return map[string]string{"id": v}
	case "array":
		return []map[string]string{{"id": v}}
	case "number":
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return v
}

// createMeta is the slice of Jira's createmeta answer the form needs.
type createMeta struct {
	Projects []struct {
		IssueTypes []struct {
			Name   string `json:"name"`
			Fields map[string]struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
				Schema   struct {
					Type  string `json:"type"`
					Items string `json:"items"`
				} `json:"schema"`
				AllowedValues []struct {
					ID    string `json:"id"`
					Value string `json:"value"`
					Name  string `json:"name"`
				} `json:"allowedValues"`
			} `json:"fields"`
		} `json:"issuetypes"`
	} `json:"projects"`
}

// formFields are the create-meta ids the New issue form already carries or
// sets itself, so they never come back as extra required fields.
var formFields = map[string]bool{
	"project": true, "issuetype": true, "summary": true, "description": true,
	"priority": true, "assignee": true, "labels": true, "reporter": true,
}

// CreateFields returns the required fields of the type beyond the form's
// own, sorted by name, with their options when they have any.
func (b *Backend) CreateFields(ctx context.Context, projectKey, logicalType string) ([]backend.FieldSpec, error) {
	ids := b.discover(ctx)
	names := jiraTypeNames([]string{logicalType}, b.requirementType)
	if len(names) == 0 {
		return nil, fmt.Errorf("unknown issue type %q", logicalType)
	}
	q := url.Values{}
	q.Set("projectKeys", projectKey)
	q.Set("issuetypeNames", names[0])
	q.Set("expand", "projects.issuetypes.fields")
	var meta createMeta
	if err := b.c.Get(ctx, "/rest/api/2/issue/createmeta?"+q.Encode(), &meta); err != nil {
		return nil, err
	}
	out := []backend.FieldSpec{}
	for _, p := range meta.Projects {
		for _, t := range p.IssueTypes {
			for id, f := range t.Fields {
				if !f.Required || formFields[id] || id == ids.Points {
					continue
				}
				spec := backend.FieldSpec{ID: id, Name: f.Name, Type: fieldKind(f.Schema.Type), Required: true, AllowedValues: []backend.FieldOption{}}
				for _, av := range f.AllowedValues {
					v := av.Value
					if v == "" {
						v = av.Name
					}
					spec.AllowedValues = append(spec.AllowedValues, backend.FieldOption{ID: av.ID, Value: v})
				}
				out = append(out, spec)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func fieldKind(schemaType string) string {
	switch schemaType {
	case "option", "number", "array":
		return schemaType
	case "date", "datetime":
		return "date"
	}
	return "string"
}
```

Run `go test ./internal/backend/jira/` inside `tam/`. Expected: PASS. If `TestCreateFieldsKeepsOnlyRequiredUnknownFields` reports `components:array,customfield_10050:option` in the other order, the sort is by `Name` and "Component/s" sorts before "Severity", so check the fake's field names rather than the sort.

- [ ] **Step 6: Demo tests**

Append to `tam/internal/backend/demo/demo_test.go` (it already imports `context`, `testing`, `backend`, and the demo backend package under whatever alias the file uses; match it):

```go
func TestDemoBackendWritesInMemoryAndStagesOneConflict(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	conflictKey := b.ConflictKey()
	if conflictKey != "ACME-412" {
		t.Fatalf("conflict key = %s", conflictKey)
	}
	before, err := b.GetIssue(ctx, "ACME-409")
	if err != nil || before.Summary != "Rotate payment gateway API keys" {
		t.Fatalf("GetIssue: %+v %v", before, err)
	}
	if err := b.UpdateIssue(ctx, "ACME-409", map[string]string{"summary": "Rotate keys", "labels": "security, ops", "storyPoints": "5", "description": "Body"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	after, _ := b.GetIssue(ctx, "ACME-409")
	if after.Summary != "Rotate keys" || len(after.Labels) != 2 || *after.StoryPoints != 5 || after.Updated <= before.Updated {
		t.Errorf("after update: %+v", after)
	}
	d, _ := b.GetIssueDetail(ctx, "ACME-409")
	if d.Description != "Body" {
		t.Errorf("description overlay: %+v", d)
	}
	page, _, _ := b.SearchIssuesPage(ctx, "ACME", "", "", nil, 0, 100)
	seen := false
	for _, iss := range page {
		if iss.Key == "ACME-409" && iss.Summary == "Rotate keys" {
			seen = true
		}
	}
	if !seen {
		t.Error("search must reflect the overlay")
	}
	if err := b.UpdateIssue(ctx, "ACME-999", map[string]string{"summary": "x"}); err == nil {
		t.Error("unknown key must fail")
	}

	key, err := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeTask, Summary: "New one", Labels: []string{"x"}, StoryPoints: pts(2)})
	if err != nil || key != "ACME-500" {
		t.Fatalf("CreateIssue: %q %v", key, err)
	}
	key2, _ := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeBug, Summary: "Second"})
	if key2 != "ACME-501" {
		t.Errorf("keys count up: %s", key2)
	}
	created, err := b.GetIssue(ctx, key)
	if err != nil || created.Type != backend.TypeTask || created.Status != "To Do" || created.Project != "ACME" || *created.StoryPoints != 2 {
		t.Errorf("created: %+v %v", created, err)
	}

	// The staged conflict fires once: the first GetIssue of the key reports
	// a version later than the search showed, and the next read agrees with
	// the first.
	var base backend.Issue
	for _, iss := range page {
		if iss.Key == conflictKey {
			base = iss
		}
	}
	first, _ := b.GetIssue(ctx, conflictKey)
	second, _ := b.GetIssue(ctx, conflictKey)
	if first.Updated <= base.Updated || second.Updated != first.Updated {
		t.Errorf("staged conflict: base=%s first=%s second=%s", base.Updated, first.Updated, second.Updated)
	}

	specs, err := b.CreateFields(ctx, "ACME", backend.TypeBug)
	if err != nil || len(specs) != 1 || specs[0].Type != "option" || len(specs[0].AllowedValues) != 3 {
		t.Errorf("bug create fields: %+v %v", specs, err)
	}
	if specs, _ := b.CreateFields(ctx, "ACME", backend.TypeTask); len(specs) != 0 {
		t.Errorf("tasks need nothing extra: %+v", specs)
	}
}

func pts(v float64) *float64 { return &v }
```

The file is package `demo_test` and imports the backend as `demobackend`; it has no `pts` helper yet, so the one above stays.

- [ ] **Step 7: The demo backend**

Replace `tam/internal/backend/demo/demo.go` with:

```go
// Package demo is the in-memory backend behind a demo profile: the curated
// dataset for reads, an overlay for writes, and one staged conflict so the
// resolution dialog can be exercised offline.
package demo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

// conflictKey is the curated story whose first version check after the app
// starts reports a later remote version. It is rekeyed to the profile's
// project.
const conflictKey = "PLAT-412"

// Backend serves the demo dataset with an overlay of writes made this run.
type Backend struct {
	project string

	mu       sync.Mutex
	over     map[string]backend.Issue
	desc     map[string]string
	nextKey  int
	conflict map[string]bool
}

// New returns a demo backend for the project key. An empty key uses the
// dataset's own.
func New(projectKey string) *Backend {
	if projectKey == "" {
		projectKey = demo.ProjectKey
	}
	b := &Backend{project: projectKey, over: map[string]backend.Issue{}, desc: map[string]string{}, nextKey: 500, conflict: map[string]bool{}}
	b.conflict[b.ConflictKey()] = true
	return b
}

// ConflictKey is the key whose first Commit conflicts, for tests and docs.
func (b *Backend) ConflictKey() string {
	return b.project + conflictKey[len(demo.ProjectKey):]
}

func (b *Backend) TestConnection(context.Context) (backend.User, error) {
	return backend.User{Name: "demo", DisplayName: "Demo User"}, nil
}

func (b *Backend) IsDemo() bool { return true }

// issues is the dataset with the overlay applied: rewritten rows replace
// their originals, created rows follow. Callers hold b.mu.
func (b *Backend) issues() []backend.Issue {
	all := demo.Issues(b.project)
	seen := map[string]bool{}
	for i, iss := range all {
		if o, ok := b.over[iss.Key]; ok {
			all[i] = o
		}
		seen[iss.Key] = true
	}
	for k, o := range b.over {
		if !seen[k] {
			all = append(all, o)
		}
	}
	return all
}

func (b *Backend) SearchIssuesPage(_ context.Context, _, _, _ string, types []string, startAt, maxResults int) ([]backend.Issue, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	want := map[string]bool{}
	for _, t := range types {
		want[t] = true
	}
	var all []backend.Issue
	for _, iss := range b.issues() {
		if len(want) == 0 || want[iss.Type] {
			all = append(all, iss)
		}
	}
	total := len(all)
	if startAt >= total || maxResults <= 0 {
		return []backend.Issue{}, total, nil
	}
	end := startAt + maxResults
	if end > total {
		end = total
	}
	return all[startAt:end], total, nil
}

func (b *Backend) GetIssueDetail(_ context.Context, key string) (backend.IssueDetail, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, ok := demo.Detail(b.project, key)
	if !ok {
		if o, created := b.over[key]; created {
			d = backend.IssueDetail{Key: key, Description: "", Links: []backend.Link{}, Fields: map[string]any{}}
			_ = o
		} else {
			return backend.IssueDetail{}, fmt.Errorf("demo: no issue %s", key)
		}
	}
	if desc, ok := b.desc[key]; ok {
		d.Description = desc
	}
	return d, nil
}

func (b *Backend) IssueTypes(context.Context, string) ([]backend.IssueType, error) {
	return []backend.IssueType{
		{ID: "1", Name: "Task"},
		{ID: "2", Name: "Epic"},
		{ID: "3", Name: "Story"},
		{ID: "4", Name: "Bug"},
		{ID: "5", Name: "Requirement"},
	}, nil
}

func (b *Backend) find(key string) (backend.Issue, bool) {
	for _, iss := range b.issues() {
		if iss.Key == key {
			return iss, true
		}
	}
	return backend.Issue{}, false
}

// GetIssue returns the row, once reporting the staged conflict's later
// version so a Commit of an edit to it is held back exactly one time.
func (b *Backend) GetIssue(_ context.Context, key string) (backend.Issue, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, ok := b.find(key)
	if !ok {
		return backend.Issue{}, fmt.Errorf("demo: no issue %s", key)
	}
	if b.conflict[key] {
		delete(b.conflict, key)
		if t, err := time.Parse(time.RFC3339, iss.Updated); err == nil {
			iss.Updated = t.Add(time.Hour).Format(time.RFC3339)
		}
		iss.StoryPoints = pts(13)
		b.over[key] = iss
	}
	return iss, nil
}

func pts(v float64) *float64 { return &v }

func (b *Backend) UpdateIssue(_ context.Context, key string, fields map[string]string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, ok := b.find(key)
	if !ok {
		return fmt.Errorf("demo: no issue %s", key)
	}
	for name, v := range fields {
		switch name {
		case "summary":
			iss.Summary = v
		case "priority":
			iss.Priority = v
		case "assignee":
			iss.Assignee = v
		case "labels":
			iss.Labels = backend.SplitLabels(v)
		case "storyPoints":
			p, err := backend.ParsePoints(v)
			if err != nil {
				return err
			}
			iss.StoryPoints = p
		case "description":
			b.desc[key] = v
		default:
			return fmt.Errorf("field %q cannot be sent to Jira", name)
		}
	}
	iss.Updated = time.Now().UTC().Format(time.RFC3339)
	b.over[key] = iss
	return nil
}

func (b *Backend) CreateIssue(_ context.Context, projectKey string, d backend.IssueDraft) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := fmt.Sprintf("%s-%d", projectKey, b.nextKey)
	b.nextKey++
	now := time.Now().UTC().Format(time.RFC3339)
	priority := d.Priority
	if priority == "" {
		priority = "Medium"
	}
	labels := d.Labels
	if labels == nil {
		labels = []string{}
	}
	b.over[key] = backend.Issue{
		Key: key, ID: fmt.Sprintf("%d", 30000+b.nextKey), Project: projectKey, Type: d.Type, Summary: d.Summary,
		Status: "To Do", Assignee: d.Assignee, Reporter: "Demo User", Priority: priority, Labels: labels,
		StoryPoints: d.StoryPoints, Created: now, Updated: now,
	}
	b.desc[key] = d.Description
	return key, nil
}

// CreateFields asks for one extra field on bugs, so the New issue form's
// create-meta section can be seen offline.
func (b *Backend) CreateFields(_ context.Context, _, logicalType string) ([]backend.FieldSpec, error) {
	if logicalType != backend.TypeBug {
		return []backend.FieldSpec{}, nil
	}
	return []backend.FieldSpec{{
		ID: "customfield_10050", Name: "Severity", Type: "option", Required: true,
		AllowedValues: []backend.FieldOption{{ID: "1", Value: "Minor"}, {ID: "2", Value: "Major"}, {ID: "3", Value: "Critical"}},
	}}, nil
}
```

The `GetIssueDetail` branch for created issues builds an empty detail; tidy the `_ = o` away by using `if _, created := b.over[key]; created {` instead.

- [ ] **Step 8: Run everything**

Run (inside `tam/`):

```bash
gofmt -l ./internal && go vet ./... && go test ./internal/... -count=1
```

Expected: PASS everywhere, including the existing demo tests (`TestDemoBackendPagesTheWholeDataset` still sees 60 rows because the overlay is empty on a fresh backend). `go vet ./...` at the module root still fails to build `app_issues.go` only if a backend no longer satisfies the interface; both now do, so it is clean.

- [ ] **Step 9: Commit**

```bash
git add tam/internal/backend tam/internal/syncer/syncer_test.go
git commit -m "feat(tam): backend get, update, create, and create-meta fields for Jira and the demo"
```

---

### Task 5: The commit pass

`tam/internal/committer` pushes the journal: creates first, then per-issue version checks, PUTs, refreshes, and the two resolutions.

**Files:**
- Create: `tam/internal/committer/committer.go`, `tam/internal/committer/committer_test.go`
- Test: `go test ./internal/committer/` inside `tam/`

**Interfaces:**
- Consumes: `backend.IssueBackend` (Task 4), `issuerepo` writes (Task 3), `issuerepo.FieldValue`, `journal.PendingChange`.
- Produces: `committer.Result{Committed []string; Created []Created; Conflicts []Conflict; Failures []Failure; Remaining int}`, `committer.Created{TempKey, Key string}`, `committer.Conflict{Key, Summary, RemoteVersion string; Fields []FieldConflict}`, `committer.FieldConflict{Field, Base, Mine, Remote string}`, `committer.Failure{Key, Error string}`, `committer.New(b backend.IssueBackend, repo *issuerepo.Repository) *Engine`, `(*Engine).Commit(ctx, profileID, projectKey string) (Result, error)`, `(*Engine).ResolveOverride(ctx, profileID, key, remoteVersion string) error`, `(*Engine).ResolveKeepRemote(ctx, profileID, key string) error`.

- [ ] **Step 1: Tests**

Create `tam/internal/committer/committer_test.go`:

```go
package committer_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

// fake is a Jira that remembers its rows and every write.
type fake struct {
	rows       map[string]backend.Issue
	desc       map[string]string
	updates    []string // "KEY field=value,..." in field order
	creates    []backend.IssueDraft
	nextKey    int
	updateErr  map[string]error
	createErr  error
	getErr     map[string]error
}

func newFake() *fake {
	return &fake{rows: map[string]backend.Issue{}, desc: map[string]string{}, nextKey: 501, updateErr: map[string]error{}, getErr: map[string]error{}}
}

func (f *fake) TestConnection(context.Context) (backend.User, error) { return backend.User{Name: "f"}, nil }
func (f *fake) IsDemo() bool                                            { return false }
func (f *fake) SearchIssuesPage(context.Context, string, string, string, []string, int, int) ([]backend.Issue, int, error) {
	return nil, 0, errors.New("not used")
}
func (f *fake) IssueTypes(context.Context, string) ([]backend.IssueType, error) { return nil, nil }
func (f *fake) CreateFields(context.Context, string, string) ([]backend.FieldSpec, error) {
	return []backend.FieldSpec{}, nil
}
func (f *fake) GetIssueDetail(_ context.Context, key string) (backend.IssueDetail, error) {
	return backend.IssueDetail{Key: key, Description: f.desc[key], Links: []backend.Link{}, Fields: map[string]any{}}, nil
}
func (f *fake) GetIssue(_ context.Context, key string) (backend.Issue, error) {
	if err := f.getErr[key]; err != nil {
		return backend.Issue{}, err
	}
	iss, ok := f.rows[key]
	if !ok {
		return backend.Issue{}, fmt.Errorf("no issue %s", key)
	}
	return iss, nil
}
func (f *fake) UpdateIssue(_ context.Context, key string, fields map[string]string) error {
	if err := f.updateErr[key]; err != nil {
		return err
	}
	iss := f.rows[key]
	parts := []string{}
	for _, name := range issuerepo.EditableFields {
		v, ok := fields[name]
		if !ok {
			continue
		}
		parts = append(parts, name+"="+v)
		switch name {
		case "summary":
			iss.Summary = v
		case "priority":
			iss.Priority = v
		case "assignee":
			iss.Assignee = v
		case "labels":
			iss.Labels = backend.SplitLabels(v)
		case "storyPoints":
			iss.StoryPoints, _ = backend.ParsePoints(v)
		case "description":
			f.desc[key] = v
		}
	}
	iss.Updated = "2026-09-07T00:00:00Z"
	f.rows[key] = iss
	f.updates = append(f.updates, key+" "+strings.Join(parts, ","))
	return nil
}
func (f *fake) CreateIssue(_ context.Context, projectKey string, d backend.IssueDraft) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.creates = append(f.creates, d)
	key := fmt.Sprintf("%s-%d", projectKey, f.nextKey)
	f.nextKey++
	f.rows[key] = backend.Issue{Key: key, ID: "9", Project: projectKey, Type: d.Type, Summary: d.Summary, Status: "To Do", Labels: d.Labels, StoryPoints: d.StoryPoints, Updated: "2026-09-07T00:00:00Z"}
	return key, nil
}

func setup(t *testing.T) (*committer.Engine, *issuerepo.Repository, *fake) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := issuerepo.New(db.DB())
	f := newFake()
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", Priority: "Medium", Labels: []string{"a"}, StoryPoints: pts(3), Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeStory, Summary: "two", Status: "To Do", Labels: []string{}, Updated: "2026-09-01T00:00:00Z"},
	}
	for _, r := range rows {
		f.rows[r.Key] = r
	}
	f.desc["PLAT-1"] = "remote text"
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	return committer.New(f, repo), repo, f
}

func pts(v float64) *float64 { return &v }

func TestCommitPushesEditsRefreshesRowsAndClearsTheJournal(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "labels", "a, b")
	_ = repo.EditField(ctx, "p1", "PLAT-2", "storyPoints", "5")
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if strings.Join(res.Committed, ",") != "PLAT-1,PLAT-2" || len(res.Conflicts) != 0 || len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
	if len(f.updates) != 2 || f.updates[0] != "PLAT-1 summary=uno,labels=a, b" || f.updates[1] != "PLAT-2 storyPoints=5" {
		t.Errorf("updates: %v", f.updates)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Pending || iss.Updated != "2026-09-07T00:00:00Z" || iss.Summary != "uno" {
		t.Errorf("row refreshed from the fake: %+v", iss)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	commits := 0
	for _, a := range act {
		if a.Action == "commit" {
			commits++
		}
	}
	if commits != 2 {
		t.Errorf("two commit entries: %+v", act)
	}
}

func TestCommitHoldsBackAConflictWithBaseMineRemote(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.EditField(ctx, "p1", "PLAT-1", "storyPoints", "8")
	_ = repo.EditField(ctx, "p1", "PLAT-1", "description", "mine text")
	_ = repo.EditField(ctx, "p1", "PLAT-2", "summary", "dos")
	remote := f.rows["PLAT-1"]
	remote.Updated = "2026-09-05T00:00:00Z"
	remote.StoryPoints = pts(13)
	f.rows["PLAT-1"] = remote

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Conflicts) != 1 || strings.Join(res.Committed, ",") != "PLAT-2" || res.Remaining != 2 {
		t.Fatalf("result: %+v", res)
	}
	c := res.Conflicts[0]
	if c.Key != "PLAT-1" || c.RemoteVersion != "2026-09-05T00:00:00Z" || c.Summary != "one" || len(c.Fields) != 2 {
		t.Errorf("conflict: %+v", c)
	}
	byField := map[string]committer.FieldConflict{}
	for _, fc := range c.Fields {
		byField[fc.Field] = fc
	}
	if p := byField["storyPoints"]; p.Base != "3" || p.Mine != "8" || p.Remote != "13" {
		t.Errorf("points: %+v", p)
	}
	if d := byField["description"]; d.Base != "" || d.Mine != "mine text" || d.Remote != "remote text" {
		t.Errorf("description base is what the cache held when edited: %+v", d)
	}
	for _, u := range f.updates {
		if strings.HasPrefix(u, "PLAT-1") {
			t.Errorf("a held issue must not be pushed: %v", f.updates)
		}
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !iss.Pending || *iss.StoryPoints != 8 {
		t.Errorf("local edit stays: %+v", iss)
	}

	// Override rebases and the next commit pushes.
	if err := eng.ResolveOverride(ctx, "p1", "PLAT-1", c.RemoteVersion); err != nil {
		t.Fatal(err)
	}
	res, _ = eng.Commit(ctx, "p1", "PLAT")
	if strings.Join(res.Committed, ",") != "PLAT-1" || res.Remaining != 0 {
		t.Errorf("after override: %+v", res)
	}
	if last := f.updates[len(f.updates)-1]; last != "PLAT-1 description=mine text,storyPoints=8" {
		t.Errorf("pushed both fields: %s", last)
	}
}

func TestKeepRemoteDropsTheEditsAndTakesJirasRow(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "mine")
	remote := f.rows["PLAT-1"]
	remote.Summary = "theirs"
	remote.Updated = "2026-09-05T00:00:00Z"
	f.rows["PLAT-1"] = remote
	if err := eng.ResolveKeepRemote(ctx, "p1", "PLAT-1"); err != nil {
		t.Fatal(err)
	}
	iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if iss.Pending || iss.Summary != "theirs" || iss.Updated != "2026-09-05T00:00:00Z" {
		t.Errorf("row: %+v", iss)
	}
	if len(f.updates) != 0 {
		t.Errorf("nothing pushed: %v", f.updates)
	}
}

func TestCommitCreatesDraftsFirstAndRekeysThem(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "New bug", Labels: []string{"x"}, StoryPoints: pts(2)})
	_ = repo.EditField(ctx, "p1", temp, "summary", "New bug, renamed")
	_ = repo.EditField(ctx, "p1", "PLAT-2", "summary", "dos")
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 || res.Created[0].TempKey != temp || res.Created[0].Key != "PLAT-501" || strings.Join(res.Committed, ",") != "PLAT-2" || res.Remaining != 0 {
		t.Errorf("result: %+v", res)
	}
	if len(f.creates) != 1 || f.creates[0].Summary != "New bug, renamed" || f.creates[0].Type != backend.TypeBug {
		t.Errorf("posted draft: %+v", f.creates)
	}
	if _, err := repo.GetIssue(ctx, "p1", temp); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("temp key gone: %v", err)
	}
	iss, err := repo.GetIssue(ctx, "p1", "PLAT-501")
	if err != nil || iss.Pending || iss.Draft || iss.Status != "To Do" || iss.Summary != "New bug, renamed" {
		t.Errorf("created row refreshed from Jira: %+v %v", iss, err)
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-501", 0)
	if len(act) < 3 || act[0].Action != "commit" {
		t.Errorf("audit trail followed the key: %+v", act)
	}
}

func TestFailuresKeepTheRowsForNextTime(t *testing.T) {
	eng, repo, f := setup(t)
	ctx := context.Background()
	_ = repo.EditField(ctx, "p1", "PLAT-1", "summary", "uno")
	_ = repo.EditField(ctx, "p1", "PLAT-2", "summary", "dos")
	temp, _ := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "d"})
	f.updateErr["PLAT-1"] = errors.New("PUT failed: 400 summary too long")
	f.createErr = errors.New("POST failed: 400 Severity is required")
	f.getErr["PLAT-2"] = errors.New("GET failed: 502")
	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 3 || len(res.Committed) != 0 || len(res.Created) != 0 || res.Remaining != 3 {
		t.Errorf("result: %+v", res)
	}
	keys := map[string]string{}
	for _, fl := range res.Failures {
		keys[fl.Key] = fl.Error
	}
	if !strings.Contains(keys["PLAT-1"], "summary too long") || !strings.Contains(keys[temp], "Severity") || !strings.Contains(keys["PLAT-2"], "502") {
		t.Errorf("failures: %v", keys)
	}
	if pend, _ := repo.ListPendingChanges(ctx, "p1"); len(pend) != 3 {
		t.Errorf("all rows kept: %+v", pend)
	}
	if _, err := repo.GetIssue(ctx, "p1", temp); err != nil {
		t.Errorf("draft kept: %v", err)
	}
}

func TestCommitWithNothingPendingIsEmpty(t *testing.T) {
	eng, _, f := setup(t)
	res, err := eng.Commit(context.Background(), "p1", "PLAT")
	if err != nil || len(res.Committed)+len(res.Created)+len(res.Conflicts)+len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("empty: %+v %v", res, err)
	}
	if res.Committed == nil || res.Created == nil || res.Conflicts == nil || res.Failures == nil {
		t.Error("slices are non-nil so the frontend sees [] not null")
	}
	if len(f.updates) != 0 {
		t.Error("no writes")
	}
}
```

- [ ] **Step 2: Run them**

Run (inside `tam/`): `go test ./internal/committer/`
Expected: compile failure, the package does not exist.

- [ ] **Step 3: The engine**

Create `tam/internal/committer/committer.go`:

```go
// Package committer pushes TAM's journal to Jira: drafts are created and
// rekeyed first, then each edited issue is version-checked, pushed, and
// refreshed. An issue whose remote version moved is held back as a conflict
// carrying base, mine, and remote for every pending field; the two
// resolutions rebase the edits or drop them.
package committer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Created pairs a draft's temporary key with the key Jira assigned.
type Created struct {
	TempKey string `json:"tempKey"`
	Key     string `json:"key"`
}

// FieldConflict is one pending field of a held issue: the value when the
// edit was made, the edit, and what Jira holds now.
type FieldConflict struct {
	Field  string `json:"field"`
	Base   string `json:"base"`
	Mine   string `json:"mine"`
	Remote string `json:"remote"`
}

// Conflict is an issue Commit held back. RemoteVersion is the updated stamp
// an override rebases onto.
type Conflict struct {
	Key           string          `json:"key"`
	Summary       string          `json:"summary"`
	RemoteVersion string          `json:"remoteVersion"`
	Fields        []FieldConflict `json:"fields"`
}

// Failure is an issue whose push or create failed; its rows stay.
type Failure struct {
	Key   string `json:"key"`
	Error string `json:"error"`
}

// Result is what one Commit did. Remaining counts the journal rows left.
type Result struct {
	Committed []string   `json:"committed"`
	Created   []Created  `json:"created"`
	Conflicts []Conflict `json:"conflicts"`
	Failures  []Failure  `json:"failures"`
	Remaining int        `json:"remaining"`
}

// Engine runs commits for one backend and repository pair.
type Engine struct {
	b    backend.IssueBackend
	repo *issuerepo.Repository
}

// New returns an engine over the backend and the store.
func New(b backend.IssueBackend, repo *issuerepo.Repository) *Engine {
	return &Engine{b: b, repo: repo}
}

// Commit pushes every pending change of the profile. Only a store failure
// returns an error; per-issue outcomes land in the Result.
func (e *Engine) Commit(ctx context.Context, profileID, projectKey string) (Result, error) {
	res := Result{Committed: []string{}, Created: []Created{}, Conflicts: []Conflict{}, Failures: []Failure{}}
	all, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return res, err
	}
	byKey := map[string][]journal.PendingChange{}
	var creates, edits []string
	for _, p := range all {
		if _, seen := byKey[p.EntityKey]; !seen {
			if p.EntityType == issuerepo.EntityIssueCreate {
				creates = append(creates, p.EntityKey)
			} else {
				edits = append(edits, p.EntityKey)
			}
		}
		byKey[p.EntityKey] = append(byKey[p.EntityKey], p)
	}
	sort.Strings(creates)
	sort.Strings(edits)
	// journal.List is newest first; apply each issue's rows oldest first.
	for k := range byKey {
		rows := byKey[k]
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	}

	for _, tempKey := range creates {
		e.commitCreate(ctx, profileID, projectKey, tempKey, byKey[tempKey], &res)
	}
	for _, key := range edits {
		e.commitEdit(ctx, profileID, key, byKey[key], &res)
	}

	left, err := e.repo.ListPendingChanges(ctx, profileID)
	if err != nil {
		return res, err
	}
	res.Remaining = len(left)
	return res, nil
}

func (e *Engine) commitCreate(ctx context.Context, profileID, projectKey, tempKey string, rows []journal.PendingChange, res *Result) {
	var createRow journal.PendingChange
	for _, p := range rows {
		if p.EntityType == issuerepo.EntityIssueCreate {
			createRow = p
		}
	}
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(createRow.AfterVal), &d); err != nil {
		res.Failures = append(res.Failures, Failure{Key: tempKey, Error: "the draft could not be decoded: " + err.Error()})
		return
	}
	realKey, err := e.b.CreateIssue(ctx, projectKey, d)
	if err != nil {
		res.Failures = append(res.Failures, Failure{Key: tempKey, Error: err.Error()})
		return
	}
	if err := e.repo.Rekey(ctx, profileID, tempKey, realKey); err != nil {
		// Jira has the issue. Report the real key so the user can find it;
		// the next full sync brings its row in and the draft can be
		// discarded by hand.
		res.Failures = append(res.Failures, Failure{Key: realKey, Error: fmt.Sprintf("created in Jira as %s but the local row could not be renamed: %v", realKey, err)})
		return
	}
	if err := e.repo.MarkCommitted(ctx, profileID, rows); err != nil {
		res.Failures = append(res.Failures, Failure{Key: realKey, Error: "created in Jira but the journal could not be cleared: " + err.Error()})
		return
	}
	e.refresh(ctx, profileID, realKey)
	res.Created = append(res.Created, Created{TempKey: tempKey, Key: realKey})
}

func (e *Engine) commitEdit(ctx context.Context, profileID, key string, rows []journal.PendingChange, res *Result) {
	remote, err := e.b.GetIssue(ctx, key)
	if err != nil {
		res.Failures = append(res.Failures, Failure{Key: key, Error: err.Error()})
		return
	}
	if remote.Updated != rows[0].BaseVersion {
		res.Conflicts = append(res.Conflicts, e.conflict(ctx, key, remote, rows))
		return
	}
	fields := make(map[string]string, len(rows))
	for _, p := range rows {
		fields[p.Field] = p.AfterVal
	}
	if err := e.b.UpdateIssue(ctx, key, fields); err != nil {
		res.Failures = append(res.Failures, Failure{Key: key, Error: err.Error()})
		return
	}
	if err := e.repo.MarkCommitted(ctx, profileID, rows); err != nil {
		res.Failures = append(res.Failures, Failure{Key: key, Error: "pushed to Jira but the journal could not be cleared: " + err.Error()})
		return
	}
	e.refresh(ctx, profileID, key)
	res.Committed = append(res.Committed, key)
}

// conflict builds the three-way view. The remote description is fetched
// only when a description edit is pending.
func (e *Engine) conflict(ctx context.Context, key string, remote backend.Issue, rows []journal.PendingChange) Conflict {
	c := Conflict{Key: key, Summary: remote.Summary, RemoteVersion: remote.Updated, Fields: []FieldConflict{}}
	remoteDesc := ""
	for _, p := range rows {
		if p.Field == "description" {
			if d, err := e.b.GetIssueDetail(ctx, key); err == nil {
				remoteDesc = d.Description
			}
			break
		}
	}
	for _, p := range rows {
		c.Fields = append(c.Fields, FieldConflict{
			Field: p.Field, Base: p.BeforeVal, Mine: p.AfterVal,
			Remote: issuerepo.FieldValue(remote, remoteDesc, p.Field),
		})
	}
	return c
}

// refresh replaces the row from Jira after a push. A failed read is logged,
// not reported: the push succeeded and the next sync refreshes the row.
func (e *Engine) refresh(ctx context.Context, profileID, key string) {
	fresh, err := e.b.GetIssue(ctx, key)
	if err != nil {
		log.Printf("tam: refresh %s after commit: %v", key, err)
		return
	}
	if err := e.repo.ReplaceRow(ctx, profileID, fresh); err != nil {
		log.Printf("tam: replace %s after commit: %v", key, err)
	}
}

// ResolveOverride rebases the held issue's edits onto the remote version so
// the next Commit pushes them over Jira's values.
func (e *Engine) ResolveOverride(ctx context.Context, profileID, key, remoteVersion string) error {
	return e.repo.SetBaseVersion(ctx, profileID, key, remoteVersion)
}

// ResolveKeepRemote drops the held issue's edits and takes Jira's row.
func (e *Engine) ResolveKeepRemote(ctx context.Context, profileID, key string) error {
	if _, err := e.repo.DiscardKey(ctx, profileID, key); err != nil {
		return err
	}
	fresh, err := e.b.GetIssue(ctx, key)
	if err != nil {
		return err
	}
	return e.repo.ReplaceRow(ctx, profileID, fresh)
}
```

- [ ] **Step 4: Run the suite**

Run (inside `tam/`):

```bash
gofmt -l ./internal && go vet ./... && go test ./internal/committer/ -count=1 -v
```

Expected: six tests PASS. Two details worth checking if one fails: in `TestCommitHoldsBackAConflictWithBaseMineRemote` the description's base is `""` because the edit was made before any detail was cached; and `TestCommitPushesEdits...` expects `labels=a, b` because the journal keeps the comma list and the fake prints it verbatim.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/committer
git commit -m "feat(tam): the commit pass with per-issue conflicts and the two resolutions"
```

---

### Task 6: The ten bound methods and the regenerated bindings

Expose the write path to the frontend, make commit and sync exclude each other, and regenerate the Wails bindings.

**Files:**
- Create: `tam/app_writes.go`
- Modify: `tam/app.go` (the in-flight map), `tam/app_issues.go` (`SyncIssues` uses the shared guard; `GetIssueDetail` never asks the backend about a draft)
- Regenerate: `tam/frontend/wailsjs/go/main/App.js`, `App.d.ts`, `tam/frontend/wailsjs/go/models.ts`
- Test: `go build ./... && go vet ./... && go test ./... ` inside `tam/`, then `wails generate module`

**Interfaces:**
- Consumes: everything from Tasks 2 to 5.
- Produces (on `*App`, bound to the frontend): `EditIssue(profileID, key, field, value string) error`, `CreateIssue(profileID string, draft backend.IssueDraft) (string, error)`, `GetCreateFields(profileID, typeName string) ([]backend.FieldSpec, error)`, `ListPendingChanges(profileID string) ([]journal.PendingChange, error)`, `DiscardPendingChange(profileID string, id int64) error`, `DiscardAllPendingChanges(profileID string) (int, error)`, `CommitPendingChanges(profileID string) (committer.Result, error)`, `ResolveConflictOverride(profileID, key, remoteVersion string) error`, `ResolveConflictKeepRemote(profileID, key string) error`, `ListActivity(profileID, key string, limit int) ([]journal.AuditEntry, error)`. Generated TypeScript namespaces `journal`, `committer`, plus `backend.IssueDraft` and `backend.FieldSpec`.

- [ ] **Step 1: One in-flight map for both operations**

In `tam/app.go`, change the field on `App` from `syncing map[string]bool` to:

```go
	// busy names the operation running for a profile ("sync" or "commit"),
	// so the two never overlap; the frontend reducer mirrors this.
	busy map[string]string
```

and in `initStore` change `a.syncing = map[string]bool{}` to `a.busy = map[string]string{}`.

In `tam/app_issues.go`, replace the guard block at the top of `SyncIssues` (from `a.backendMu.Lock()` through the deferred delete) with:

```go
	if err := a.acquire(p.ID, "sync"); err != nil {
		return syncer.Summary{}, err
	}
	defer a.release(p.ID)
```

Still in `app_issues.go`, in `GetIssueDetail`, change the cache check so a draft never reaches the backend (its detail cache is written at creation):

```go
	if ok && (strings.HasPrefix(key, issuerepo.DraftPrefix) || time.Since(fetchedAt) < detailFreshFor) {
		return cached, nil
	}
```

- [ ] **Step 2: `app_writes.go`**

Create `tam/app_writes.go`:

```go
package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
)

// acquire marks the profile as running what ("sync" or "commit") and
// refuses while either runs. Callers defer release.
func (a *App) acquire(profileID, what string) error {
	a.backendMu.Lock()
	defer a.backendMu.Unlock()
	if cur := a.busy[profileID]; cur != "" {
		return fmt.Errorf("a %s is already running for this profile", cur)
	}
	a.busy[profileID] = what
	return nil
}

func (a *App) release(profileID string) {
	a.backendMu.Lock()
	delete(a.busy, profileID)
	a.backendMu.Unlock()
}

// EditIssue journals one field change on an issue or a draft.
func (a *App) EditIssue(profileID, key, field, value string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if strings.TrimSpace(profileID) == "" {
		return errors.New("no profile selected")
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("issue key is empty")
	}
	return a.repo.EditField(a.ctx, profileID, key, field, value)
}

// CreateIssue stores a draft under a temporary key and returns that key.
func (a *App) CreateIssue(profileID string, draft backend.IssueDraft) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	return a.repo.CreateDraft(a.ctx, p.ID, p.ProjectKey, draft)
}

// GetCreateFields asks the backend which required fields the New issue
// form must add for the type.
func (a *App) GetCreateFields(profileID, typeName string) ([]backend.FieldSpec, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	b, err := a.backendFor(p)
	if err != nil {
		return nil, err
	}
	specs, err := b.CreateFields(a.ctx, p.ProjectKey, typeName)
	if err != nil {
		return nil, err
	}
	if specs == nil {
		specs = []backend.FieldSpec{}
	}
	return specs, nil
}

// ListPendingChanges returns the profile's journal, newest first.
func (a *App) ListPendingChanges(profileID string) ([]journal.PendingChange, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	rows, err := a.repo.ListPendingChanges(a.ctx, profileID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []journal.PendingChange{}
	}
	return rows, nil
}

// DiscardPendingChange reverts one journaled change.
func (a *App) DiscardPendingChange(profileID string, id int64) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.DiscardPendingChange(a.ctx, profileID, id)
}

// DiscardAllPendingChanges reverts every journaled change of the profile.
func (a *App) DiscardAllPendingChanges(profileID string) (int, error) {
	if err := a.requireStore(); err != nil {
		return 0, err
	}
	return a.repo.DiscardAllPendingChanges(a.ctx, profileID)
}

// CommitPendingChanges pushes the journal to Jira. It refuses while a sync
// runs for the profile, and a sync refuses while it runs.
func (a *App) CommitPendingChanges(profileID string) (committer.Result, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return committer.Result{}, err
	}
	if err := a.acquire(p.ID, "commit"); err != nil {
		return committer.Result{}, err
	}
	defer a.release(p.ID)
	b, err := a.backendFor(p)
	if err != nil {
		return committer.Result{}, err
	}
	res, err := committer.New(b, a.repo).Commit(a.ctx, p.ID, p.ProjectKey)
	if err != nil {
		log.Printf("tam: commit %s (%s) failed: %v", p.Name, p.ProjectKey, err)
		return res, err
	}
	log.Printf("tam: committed %s (%s): %d pushed, %d created, %d conflicts, %d failures, %d left",
		p.Name, p.ProjectKey, len(res.Committed), len(res.Created), len(res.Conflicts), len(res.Failures), res.Remaining)
	return res, nil
}

// ResolveConflictOverride rebases a held issue's edits so the next Commit
// pushes them over Jira's values.
func (a *App) ResolveConflictOverride(profileID, key, remoteVersion string) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	b, err := a.backendFor(p)
	if err != nil {
		return err
	}
	return committer.New(b, a.repo).ResolveOverride(a.ctx, p.ID, key, remoteVersion)
}

// ResolveConflictKeepRemote drops a held issue's edits and takes Jira's row.
func (a *App) ResolveConflictKeepRemote(profileID, key string) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	b, err := a.backendFor(p)
	if err != nil {
		return err
	}
	return committer.New(b, a.repo).ResolveKeepRemote(a.ctx, p.ID, key)
}

// ListActivity returns the local audit trail of one issue, newest first.
func (a *App) ListActivity(profileID, key string, limit int) ([]journal.AuditEntry, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	rows, err := a.repo.ListActivity(a.ctx, profileID, key, limit)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []journal.AuditEntry{}
	}
	return rows, nil
}
```

- [ ] **Step 3: Build, vet, test**

Run (inside `tam/`):

```bash
gofmt -l . ./internal && go build ./... && go vet ./... && go test ./... -count=1
```

Expected: clean and PASS. `app_issues.go` needs no new import for `issuerepo` (already imported) but does use `strings` (already imported).

- [ ] **Step 4: Regenerate the bindings**

Run (inside `tam/`): `wails generate module`

Then check the output:

```bash
grep -n "EditIssue\|CommitPendingChanges\|ListActivity\|ResolveConflictKeepRemote" frontend/wailsjs/go/main/App.d.ts
grep -n "^export namespace" frontend/wailsjs/go/models.ts
grep -n "class PendingChange\|class AuditEntry\|class Result\|class Conflict\|class IssueDraft\|class FieldSpec" frontend/wailsjs/go/models.ts
```

Expected: the ten methods in `App.d.ts`; namespaces `backend`, `committer`, `issuerepo`, `journal`, `main`, `profile`, `settings`, `syncer`; the six classes present. If `wails generate module` is not on the PATH, the `wails` binary lives at `%USERPROFILE%\go\bin\wails.exe`.

Wails also rewrites line endings under `frontend/wailsjs/runtime` and `frontend/package.json.md5`, and sometimes `go.mod`. Revert those:

```bash
git checkout -- frontend/wailsjs/runtime frontend/package.json.md5 go.mod
git status --short --untracked-files=no
```

Expected: only `app.go`, `app_issues.go`, `app_writes.go`, `frontend/wailsjs/go/main/App.d.ts`, `frontend/wailsjs/go/main/App.js`, and `frontend/wailsjs/go/models.ts` changed.

- [ ] **Step 5: The frontend still compiles**

From the repo root: `npm run typecheck --workspace tam/frontend && npm test --workspace tam/frontend 2>&1 | grep "Tests "`
Expected: clean, and the same passing count as before this task (no frontend source changed).

- [ ] **Step 6: Commit**

```bash
git add tam/app.go tam/app_issues.go tam/app_writes.go tam/frontend/wailsjs/go
git commit -m "feat(tam): bind the write path and make commit and sync exclude each other"
```

---

### Task 7: Frontend API, queries, the commit action, and the Pending changes dialog

The frontend learns the new bindings, the sync provider gains the commit action on the shared reducer, the status bar shows the Commit chip, and the Pending changes dialog lists, discards, and commits.

**Files:**
- Modify: `tam/frontend/src/api.ts`, `queries/keys.ts`, `queries/invalidate.ts`, `modals.ts`, `contexts/SyncContext.tsx`, `App.tsx`, `App.test.tsx`, `App.css`
- Create: `tam/frontend/src/queries/pending.ts`, `components/PendingChangesModal.tsx`, `components/PendingChangesModal.test.tsx`
- Test: `npm test --workspace tam/frontend` and `npm run typecheck --workspace tam/frontend` from the repo root

**Interfaces:**
- Consumes: the generated bindings from Task 6 (`App.EditIssue` and the other nine; `backend.IssueDraft` in `models.ts`).
- Produces (in `api.ts`): `Issue.pending?: boolean`, `Issue.draft?: boolean`, `DRAFT_PREFIX`, `EDITABLE_FIELDS`, `fieldLabel(field)`, types `PendingChange`, `AuditEntry`, `IssueDraft`, `FieldSpec`, `FieldOption`, `FieldConflict`, `Conflict`, `CommitResult`, and the ten bound functions. In `queries/keys.ts`: `keys.pending(profileId)`, `keys.activity(profileId, key)`, `keys.createFields(profileId, type)`. In `queries/invalidate.ts`: `invalidateWrites(qc, profileId, key?)`. In `queries/pending.ts`: `usePendingChanges`, `useActivity`, `useCreateFields`, `useEditIssue`, `useDiscardChange`, `useDiscardAll`, `groupPending(rows)`. In `SyncContext`: `canCommit`, `runCommit(): Promise<CommitResult | null>`, `lastCommit: CommitResult | null`. `ModalId` gains `"pending" | "newIssue"`.

- [ ] **Step 1: `api.ts`**

In `tam/frontend/src/api.ts`, add to the `Issue` interface after `updated`:

```ts
  // Computed by the store on every read. Optional so fixtures that predate
  // plan 1b still type-check; the backend always sends both.
  pending?: boolean;
  draft?: boolean;
```

After `LinkedTest`, add the write-path shapes:

```ts
// DRAFT_PREFIX starts the temporary key of an issue created locally and
// not yet committed. It matches issuerepo.DraftPrefix.
export const DRAFT_PREFIX = "TAM-NEW-";

export type EditableField = "summary" | "description" | "priority" | "labels" | "storyPoints" | "assignee";

export const EDITABLE_FIELDS: { id: EditableField; label: string }[] = [
  { id: "summary", label: "Summary" },
  { id: "description", label: "Description" },
  { id: "priority", label: "Priority" },
  { id: "labels", label: "Labels" },
  { id: "storyPoints", label: "Story points" },
  { id: "assignee", label: "Assignee" },
];

export function fieldLabel(field: string): string {
  return EDITABLE_FIELDS.find((f) => f.id === field)?.label ?? field;
}

export interface PendingChange {
  id: number;
  entityType: string;
  entityKey: string;
  field: string;
  beforeVal: string;
  afterVal: string;
  baseVersion: string;
  createdAt: string;
}

export interface AuditEntry {
  id: number;
  occurredAt: string;
  actor: string;
  entityType: string;
  entityKey: string;
  action: string;
  field: string;
  beforeVal: string;
  afterVal: string;
  note: string;
}

export interface IssueDraft {
  type: IssueType;
  summary: string;
  description: string;
  priority: string;
  labels: string[];
  assignee: string;
  storyPoints: number | null;
  extra: Record<string, string>;
}

export interface FieldOption {
  id: string;
  value: string;
}

export interface FieldSpec {
  id: string;
  name: string;
  type: "string" | "option" | "number" | "date" | "array" | string;
  required: boolean;
  allowedValues: FieldOption[];
}

export interface FieldConflict {
  field: string;
  base: string;
  mine: string;
  remote: string;
}

export interface Conflict {
  key: string;
  summary: string;
  remoteVersion: string;
  fields: FieldConflict[];
}

export interface CommitResult {
  committed: string[];
  created: { tempKey: string; key: string }[];
  conflicts: Conflict[];
  failures: { key: string; error: string }[];
  remaining: number;
}
```

After `SetProfileSetting`, add the bindings. `CreateIssue` wraps its argument the way `ListIssues` does, so Wails receives its generated class:

```ts
export const EditIssue: (profileId: string, key: string, field: string, value: string) => Promise<void> =
  App.EditIssue;
export const CreateIssue = (profileId: string, draft: IssueDraft): Promise<string> =>
  App.CreateIssue(profileId, backend.IssueDraft.createFrom(draft));
export const GetCreateFields: (profileId: string, typeName: string) => Promise<FieldSpec[]> =
  App.GetCreateFields;
export const ListPendingChanges: (profileId: string) => Promise<PendingChange[]> = App.ListPendingChanges;
export const DiscardPendingChange: (profileId: string, id: number) => Promise<void> =
  App.DiscardPendingChange;
export const DiscardAllPendingChanges: (profileId: string) => Promise<number> =
  App.DiscardAllPendingChanges;
export const CommitPendingChanges: (profileId: string) => Promise<CommitResult> =
  App.CommitPendingChanges;
export const ResolveConflictOverride: (profileId: string, key: string, remoteVersion: string) => Promise<void> =
  App.ResolveConflictOverride;
export const ResolveConflictKeepRemote: (profileId: string, key: string) => Promise<void> =
  App.ResolveConflictKeepRemote;
export const ListActivity: (profileId: string, key: string, limit: number) => Promise<AuditEntry[]> =
  App.ListActivity;
```

Add `backend` to the existing `models` import at the top of the file: the file imports `issuerepo` from `"../wailsjs/go/models"`; make it `import { backend, issuerepo } from "../wailsjs/go/models";`.

- [ ] **Step 2: Keys, invalidation, modal ids**

`tam/frontend/src/queries/keys.ts` gains three entries inside `keys`:

```ts
  pending: (profileId: string) => [profileId, "pending"] as const,
  activity: (profileId: string, key: string) => [profileId, "issue", key, "activity"] as const,
  createFields: (profileId: string, type: string) => [profileId, "createFields", type] as const,
```

`tam/frontend/src/queries/invalidate.ts` adds `keys.pending(profileId)` to the list in `invalidateProfileData` and gains:

```ts
// invalidateWrites refreshes what a local write can change: the Backlog
// rows, the pending list, and one issue's detail, tests, and activity when
// a key is given, or every issue's when it is not (a discard-all).
export function invalidateWrites(qc: QueryClient, profileId: string, key?: string) {
  if (!profileId) return;
  qc.invalidateQueries({ queryKey: [profileId, "issues"] });
  qc.invalidateQueries({ queryKey: keys.pending(profileId) });
  qc.invalidateQueries({ queryKey: key ? [profileId, "issue", key] : [profileId, "issue"] });
}
```

`tam/frontend/src/modals.ts`:

```ts
export type ModalId = "profiles" | "about" | "pending" | "newIssue";
```

- [ ] **Step 3: `queries/pending.ts`**

Create `tam/frontend/src/queries/pending.ts`:

```ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import {
  DiscardAllPendingChanges,
  DiscardPendingChange,
  EditIssue,
  GetCreateFields,
  ListActivity,
  ListPendingChanges,
} from "../api";
import type { IssueDraft, PendingChange } from "../api";
import { keys } from "./keys";
import { invalidateWrites } from "./invalidate";

const ACTIVITY_LIMIT = 200;

export function usePendingChanges(profileId: string) {
  return useQuery({
    queryKey: keys.pending(profileId),
    queryFn: () => call(() => ListPendingChanges(profileId)),
    enabled: !!profileId,
  });
}

export function useActivity(profileId: string, key: string) {
  return useQuery({
    queryKey: keys.activity(profileId, key),
    queryFn: () => call(() => ListActivity(profileId, key, ACTIVITY_LIMIT)),
    enabled: !!profileId && !!key,
    retry: false,
  });
}

export function useCreateFields(profileId: string, type: string) {
  return useQuery({
    queryKey: keys.createFields(profileId, type),
    queryFn: () => call(() => GetCreateFields(profileId, type)),
    enabled: !!profileId && !!type,
    retry: false,
  });
}

export function useEditIssue(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, field, value }: { key: string; field: string; value: string }) =>
      call(() => EditIssue(profileId, key, field, value)),
    onSuccess: (_, v) => invalidateWrites(qc, profileId, v.key),
  });
}

export function useDiscardChange(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (change: PendingChange) => call(() => DiscardPendingChange(profileId, change.id)),
    onSuccess: (_, change) => invalidateWrites(qc, profileId, change.entityKey),
  });
}

export function useDiscardAll(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => call(() => DiscardAllPendingChanges(profileId)),
    onSuccess: () => invalidateWrites(qc, profileId),
  });
}

// A PendingGroup is one issue's rows. A draft group carries its decoded
// draft; an edit group carries one row per field.
export interface PendingGroup {
  key: string;
  draft: IssueDraft | null;
  createRow: PendingChange | null;
  edits: PendingChange[];
}

// groupPending folds the journal (newest first) into one group per key,
// drafts first, then keys in the order they first appear.
export function groupPending(rows: PendingChange[]): PendingGroup[] {
  const byKey = new Map<string, PendingGroup>();
  for (const row of rows) {
    let g = byKey.get(row.entityKey);
    if (!g) {
      g = { key: row.entityKey, draft: null, createRow: null, edits: [] };
      byKey.set(row.entityKey, g);
    }
    if (row.entityType === "issue_create") {
      g.createRow = row;
      try {
        g.draft = JSON.parse(row.afterVal) as IssueDraft;
      } catch {
        g.draft = null;
      }
    } else {
      g.edits.push(row);
    }
  }
  const groups = [...byKey.values()];
  return [...groups.filter((g) => g.createRow), ...groups.filter((g) => !g.createRow)];
}
```

- [ ] **Step 4: The commit action on the sync provider**

Replace `tam/frontend/src/contexts/SyncContext.tsx` with:

```tsx
import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState } from "react";
import type { ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  syncReducer,
  initialSyncState,
  canSync as canSyncSel,
  canCommit as canCommitSel,
  canSwitchProfile as canSwitchProfileSel,
  useNotice,
  useProfile,
  call,
  errMsg,
} from "@agile-suite/core";
import type { SyncProgress, SyncStatus } from "@agile-suite/core";
import { CommitPendingChanges, EventsOn, SyncIssues } from "../api";
import type { CommitResult, Profile, Settings } from "../api";
import { invalidateProfileData, invalidateWrites } from "../queries/invalidate";

// SyncProvider owns the one reducer that keeps sync and commit from
// overlapping. Both actions gate on the selectors before dispatching and the
// reducer refuses a start in any state but idle, so a double click or a
// keyboard repeat cannot start a second run.

const PROGRESS_EVENT = "tam:sync-progress";

interface SyncApi {
  status: SyncStatus;
  progress: SyncProgress | null;
  syncError: string;
  canSync: boolean;
  canCommit: boolean;
  canSwitchProfile: boolean;
  runSync: (full: boolean) => Promise<void>;
  // runCommit resolves to the result, or null when nothing ran or the call
  // failed (the failure is shown as a notice).
  runCommit: () => Promise<CommitResult | null>;
  // lastCommit is the most recent result for the active profile, for the
  // Pending changes dialog's banner. It clears when the profile changes.
  lastCommit: CommitResult | null;
}

const SyncContext = createContext<SyncApi | null>(null);

export function useSync(): SyncApi {
  const ctx = useContext(SyncContext);
  if (!ctx) {
    throw new Error("useSync must be used within a SyncProvider");
  }
  return ctx;
}

export function SyncProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(syncReducer, initialSyncState);
  const { activeId } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const { notice } = useNotice();
  const statusRef = useRef<SyncStatus>("idle");
  const [lastCommit, setLastCommit] = useState<CommitResult | null>(null);

  useEffect(
    () =>
      EventsOn(PROGRESS_EVENT, (p: SyncProgress) =>
        dispatch({ type: "SYNC_PROGRESS", progress: p }),
      ),
    [],
  );

  useEffect(() => {
    setLastCommit(null);
  }, [activeId]);

  const runSync = useCallback(
    async (full: boolean) => {
      if (!activeId || statusRef.current !== "idle") return;
      statusRef.current = "syncing";
      dispatch({
        type: "SYNC_START",
        clearError: true,
        initialProgress: { phase: "issues", fetched: 0, total: 0, done: false, stage: "Starting" },
      });
      try {
        await call(() => SyncIssues(activeId, full));
      } catch (e) {
        const message = errMsg(e);
        dispatch({ type: "SYNC_ERROR", message });
        void notice({ title: "Sync failed", message, tone: "error" });
      } finally {
        statusRef.current = "idle";
        dispatch({ type: "SYNC_END" });
        invalidateProfileData(qc, activeId);
      }
    },
    [activeId, qc, notice],
  );

  const runCommit = useCallback(async (): Promise<CommitResult | null> => {
    if (!activeId || statusRef.current !== "idle") return null;
    statusRef.current = "committing";
    dispatch({ type: "COMMIT_START" });
    try {
      const res = await call(() => CommitPendingChanges(activeId));
      setLastCommit(res);
      return res;
    } catch (e) {
      void notice({ title: "Commit failed", message: errMsg(e), tone: "error" });
      return null;
    } finally {
      statusRef.current = "idle";
      dispatch({ type: "COMMIT_END" });
      invalidateWrites(qc, activeId);
      invalidateProfileData(qc, activeId);
    }
  }, [activeId, qc, notice]);

  const api = useMemo<SyncApi>(
    () => ({
      status: state.status,
      progress: state.progress,
      syncError: state.syncError,
      canSync: canSyncSel(state) && !!activeId,
      canCommit: canCommitSel(state) && !!activeId,
      canSwitchProfile: canSwitchProfileSel(state),
      runSync,
      runCommit,
      lastCommit,
    }),
    [state, activeId, runSync, runCommit, lastCommit],
  );

  return <SyncContext.Provider value={api}>{children}</SyncContext.Provider>;
}
```

- [ ] **Step 5: The dialog test**

Create `tam/frontend/src/components/PendingChangesModal.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { PendingChange } from "../api";
import { profileBackend } from "../profileBackend";
import { SyncProvider } from "../contexts/SyncContext";
import { PendingChangesModal } from "./PendingChangesModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    SyncIssues: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
    ListPendingChanges: vi.fn(),
    DiscardPendingChange: vi.fn(),
    DiscardAllPendingChanges: vi.fn(),
    CommitPendingChanges: vi.fn(),
    ResolveConflictOverride: vi.fn(),
    ResolveConflictKeepRemote: vi.fn(),
  };
});

const rows: PendingChange[] = [
  { id: 3, entityType: "issue_create", entityKey: "TAM-NEW-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "2026-09-06T10:00:00Z",
    afterVal: JSON.stringify({ type: "task", summary: "Add a retry to the payment webhook consumer", description: "", priority: "Medium", labels: [], assignee: "M. Ortiz", storyPoints: 3, extra: {} }) },
  { id: 2, entityType: "issue", entityKey: "PLAT-409", field: "priority", beforeVal: "Medium", afterVal: "High", baseVersion: "v1", createdAt: "2026-09-06T09:59:00Z" },
  { id: 1, entityType: "issue", entityKey: "PLAT-409", field: "assignee", beforeVal: "", afterVal: "M. Ortiz", baseVersion: "v1", createdAt: "2026-09-06T09:58:00Z" },
];

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderModal(onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <SyncProvider>
            <Loader />
            <PendingChangesModal onClose={onClose} />
          </SyncProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
  return onClose;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListPendingChanges).mockResolvedValue(rows);
  vi.mocked(api.DiscardPendingChange).mockResolvedValue();
  vi.mocked(api.DiscardAllPendingChanges).mockResolvedValue(3);
});

describe("PendingChangesModal", () => {
  it("groups the journal by issue, drafts first, with before and after per field", async () => {
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    expect(within(dialog).getByText("3 changes on 2 issues, 1 of them new")).toBeInTheDocument();
    const cards = within(dialog).getAllByRole("group");
    expect(cards).toHaveLength(2);
    expect(within(cards[0]).getByText("TAM-NEW-1")).toBeInTheDocument();
    expect(within(cards[0]).getByText("Draft")).toBeInTheDocument();
    expect(within(cards[0]).getByText("Add a retry to the payment webhook consumer")).toBeInTheDocument();
    expect(within(cards[0]).getByText("New Task in PLAT, priority Medium, assignee M. Ortiz, 3 points")).toBeInTheDocument();
    expect(within(cards[1]).getByText("PLAT-409")).toBeInTheDocument();
    const rowsOfCard = within(cards[1]).getAllByRole("listitem");
    expect(rowsOfCard[0]).toHaveTextContent("Priority Medium to High");
    expect(rowsOfCard[1]).toHaveTextContent("Assignee (none) to M. Ortiz");
    expect(within(dialog).getByRole("button", { name: "Commit (2)" })).toBeEnabled();
  });

  it("discards one row and all rows", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(within(dialog).getByRole("button", { name: "Discard priority on PLAT-409" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 2));
    expect(api.ListPendingChanges).toHaveBeenCalledTimes(2);

    await user.click(within(dialog).getByRole("button", { name: "Discard all" }));
    const confirm = await screen.findByRole("alertdialog");
    await user.click(within(confirm).getByRole("button", { name: "Discard all" }));
    await waitFor(() => expect(api.DiscardAllPendingChanges).toHaveBeenCalledWith("p1"));
  });

  it("commits and shows the result banner with the key mapping", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: ["PLAT-409"], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501" }], conflicts: [], failures: [], remaining: 0,
    });
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce(rows).mockResolvedValue([]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(within(dialog).getByRole("button", { name: "Commit (2)" }));
    await waitFor(() => expect(api.CommitPendingChanges).toHaveBeenCalledWith("p1"));
    expect(await within(dialog).findByText("Last commit: 1 issue pushed, 1 created (TAM-NEW-1 is now PLAT-501).")).toBeInTheDocument();
    expect(await within(dialog).findByText("Nothing pending. Edit an issue or create one and it shows up here.")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Commit (0)" })).toBeDisabled();
  });

  it("names failures in the banner and keeps their rows", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], conflicts: [],
      failures: [{ key: "PLAT-409", error: "PUT failed: 400 priority is invalid" }, { key: "TAM-NEW-1", error: "Severity is required" }],
      remaining: 3,
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(within(dialog).getByRole("button", { name: "Commit (2)" }));
    expect(await within(dialog).findByText("Last commit: nothing pushed, 2 failed.")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-409: PUT failed: 400 priority is invalid")).toBeInTheDocument();
    expect(within(dialog).getByText("TAM-NEW-1: Severity is required")).toBeInTheDocument();
    expect(within(dialog).getAllByRole("group")).toHaveLength(2);
  });
});
```

The profile and settings fixtures match `api.ts`'s `Profile` and `Settings` exactly, the same shapes `App.test.tsx` uses.

- [ ] **Step 6: Run it**

Run from the repo root: `npm test --workspace tam/frontend -- PendingChangesModal`
Expected: FAIL, the component does not exist.

- [ ] **Step 7: The dialog**

Create `tam/frontend/src/components/PendingChangesModal.tsx`:

```tsx
import { useMemo } from "react";
import { Modal, useConfirm, useProfile } from "@agile-suite/core";
import { fieldLabel } from "../api";
import type { CommitResult, IssueDraft, Profile, Settings } from "../api";
import { ISSUE_TYPES } from "../api";
import { groupPending, useDiscardAll, useDiscardChange, usePendingChanges } from "../queries/pending";
import type { PendingGroup } from "../queries/pending";
import { useSync } from "../contexts/SyncContext";
import { ConflictCard } from "./ConflictCard";

interface Props {
  onClose: () => void;
}

function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

// summaryLine is the dialog's subtitle: "3 changes on 2 issues, 1 of them new".
export function summaryLine(groups: PendingGroup[], rowCount: number): string {
  const drafts = groups.filter((g) => g.createRow).length;
  const base = `${plural(rowCount, "change", "changes")} on ${plural(groups.length, "issue", "issues")}`;
  return drafts > 0 ? `${base}, ${drafts} of them new` : base;
}

// bannerLine renders a commit result as one sentence.
export function bannerLine(r: CommitResult): string {
  const parts: string[] = [];
  if (r.committed.length) parts.push(plural(r.committed.length, "issue pushed", "issues pushed"));
  if (r.created.length) {
    const mapping = r.created.map((c) => `${c.tempKey} is now ${c.key}`).join(", ");
    parts.push(`${r.created.length} created (${mapping})`);
  }
  if (r.conflicts.length) parts.push(`${r.conflicts.length} held back`);
  if (r.failures.length) parts.push(`${r.failures.length} failed`);
  if (parts.length === 0) return "Last commit: nothing to push.";
  if (!r.committed.length && !r.created.length) return `Last commit: nothing pushed, ${parts.join(", ")}.`;
  return `Last commit: ${parts.join(", ")}.`;
}

function draftLine(d: IssueDraft, project: string): string {
  const type = ISSUE_TYPES.find((t) => t.id === d.type)?.label ?? d.type;
  const bits = [`New ${type} in ${project}`];
  if (d.priority) bits.push(`priority ${d.priority}`);
  if (d.assignee) bits.push(`assignee ${d.assignee}`);
  if (d.storyPoints !== null && d.storyPoints !== undefined) bits.push(`${d.storyPoints} points`);
  return bits.join(", ");
}

export function PendingChangesModal({ onClose }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const pending = usePendingChanges(activeId);
  const discardOne = useDiscardChange(activeId);
  const discardAll = useDiscardAll(activeId);
  const { confirm } = useConfirm();
  const { canCommit, runCommit, lastCommit, status } = useSync();

  const rows = pending.data ?? [];
  const groups = useMemo(() => groupPending(rows), [rows]);
  const conflictKeys = new Set((lastCommit?.conflicts ?? []).map((c) => c.key).filter((k) => groups.some((g) => g.key === k)));
  const pushable = groups.filter((g) => !conflictKeys.has(g.key)).length;
  const busy = status !== "idle" || discardOne.isPending || discardAll.isPending;

  async function onDiscardAll() {
    const ok = await confirm({
      title: "Discard all pending changes?",
      message: `${plural(rows.length, "change", "changes")} will be reverted locally. Jira is not touched.`,
      confirmLabel: "Discard all",
      danger: true,
    });
    if (ok) discardAll.mutate();
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="pending-title">
      <div className="pending-head">
        <h2 id="pending-title">Pending changes</h2>
        <span className="muted">{groups.length ? summaryLine(groups, rows.length) : ""}</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      {lastCommit && (
        <div className={`pending-banner${lastCommit.conflicts.length || lastCommit.failures.length ? " pending-banner-warn" : ""}`} role="status">
          <p className="b">{bannerLine(lastCommit)}</p>
          {lastCommit.conflicts.filter((c) => conflictKeys.has(c.key)).map((c) => (
            <p key={c.key} className="small">{c.key} changed in Jira since you edited it. Resolve it below, then commit again.</p>
          ))}
          {lastCommit.failures.map((f) => (
            <p key={f.key} className="small error-text">{f.key}: {f.error}</p>
          ))}
        </div>
      )}

      {pending.isError ? (
        <p className="error-text">Could not load the pending changes: {pending.error.message}</p>
      ) : pending.isPending ? (
        <p className="muted">Loading</p>
      ) : groups.length === 0 ? (
        <p className="muted pending-empty">Nothing pending. Edit an issue or create one and it shows up here.</p>
      ) : (
        <div className="pending-list">
          {groups.map((g) => {
            const conflict = lastCommit?.conflicts.find((c) => c.key === g.key && conflictKeys.has(c.key));
            if (conflict) {
              return <ConflictCard key={g.key} profileId={activeId} conflict={conflict} disabled={busy} />;
            }
            return (
              <section key={g.key} className="pending-card" role="group" aria-label={g.key}>
                <div className="pending-card-head">
                  <span className="b">{g.key}</span>
                  {g.createRow && <span className="chip chip-draft">Draft</span>}
                  {g.draft && <span className="pending-card-summary">{g.draft.summary}</span>}
                  {g.createRow && (
                    <button type="button" className="btn pending-discard" disabled={busy} aria-label={`Discard ${g.key}`} onClick={() => discardOne.mutate(g.createRow!)}>
                      Discard
                    </button>
                  )}
                </div>
                {g.draft ? (
                  <>
                    <p className="muted small">{draftLine(g.draft, activeProfile?.projectKey ?? "")}</p>
                    <p className="muted small">Commit creates it in Jira and swaps the temporary key for the real one.</p>
                  </>
                ) : (
                  <ul className="pending-rows">
                    {g.edits.map((row) => (
                      <li key={row.id} className="pending-row">
                        <span className="muted pending-field">{fieldLabel(row.field)}</span>
                        <span>{row.beforeVal || "(none)"}</span>
                        <span className="muted">to</span>
                        <span className="b">{row.afterVal || "(none)"}</span>
                        <button type="button" className="btn btn-ghost" disabled={busy} aria-label={`Discard ${row.field} on ${g.key}`} onClick={() => discardOne.mutate(row)}>
                          Discard
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            );
          })}
        </div>
      )}

      <div className="pending-footer">
        <span className="muted small">Edits are pushed with Jira's own field update; a conflict holds only that issue back.</span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" disabled={busy || rows.length === 0} onClick={() => void onDiscardAll()}>Discard all</button>
          <button type="button" className="btn btn-primary" disabled={!canCommit || busy || pushable === 0} onClick={() => void runCommit()}>
            {`Commit (${pushable})`}
          </button>
        </span>
      </div>
    </Modal>
  );
}
```

`ConflictCard` is Task 10's. Until then, create a placeholder `tam/frontend/src/components/ConflictCard.tsx` that Task 10 replaces:

```tsx
import type { Conflict } from "../api";

interface Props {
  profileId: string;
  conflict: Conflict;
  disabled: boolean;
}

// ConflictCard is filled in by the conflict task; until then a held issue
// shows its key and the note from the banner.
export function ConflictCard({ conflict }: Props) {
  return (
    <section className="pending-card pending-card-conflict" role="group" aria-label={conflict.key}>
      <div className="pending-card-head">
        <span className="b">{conflict.key}</span>
        <span className="chip chip-conflict">Conflict</span>
        <span className="pending-card-summary">{conflict.summary}</span>
      </div>
    </section>
  );
}
```

`Modal` comes from `@agile-suite/core` with `className` and `labelledBy` props (see `ProfilesModal.tsx` for the same usage); `.btn`, `.btn-primary`, `.btn-ghost`, and `.btn-danger` are in `frontend/core/styles/primitives.css`.

- [ ] **Step 8: The status bar chip and the modal in the shell**

In `tam/frontend/src/App.tsx`:

Add the imports:

```tsx
import { PendingChangesModal } from "./components/PendingChangesModal";
import { usePendingChanges } from "./queries/pending";
```

Inside `App()` after `const syncState = useSyncState(activeId);`:

```tsx
  const pending = usePendingChanges(activeId);
  const pendingCount = pending.data?.length ?? 0;
```

In the footer, after the sync progress chip block and before the sync error block, add:

```tsx
        {pendingCount > 0 && (
          <button type="button" className="chip chip-pending" onClick={() => openModal("pending")}>
            {`${pendingCount} pending ${pendingCount === 1 ? "change" : "changes"}: Commit`}
          </button>
        )}
```

At the bottom, after the About modal line:

```tsx
      {isOpen("pending") && <PendingChangesModal onClose={closeModal} />}
```

Also add the `EventsOn` menu wiring for a future menu entry only if `tam/main.go` registers one; it does not, so nothing else changes here.

- [ ] **Step 9: CSS**

Append to `tam/frontend/src/App.css`:

```css
/* Plan 1b: pending changes */
.chip-pending {
  background: var(--warn-bg, #fef3c7);
  color: var(--warn-fg, #92400e);
  border: 1px solid var(--warn-border, #f59e0b);
  cursor: pointer;
  font-weight: 600;
}
.chip-draft { background: var(--warn-bg, #fef3c7); color: var(--warn-fg, #92400e); border: 1px solid var(--warn-border, #f59e0b); }
.chip-conflict { background: #fee2e2; color: #991b1b; border: 1px solid #f87171; text-transform: uppercase; font-size: 10px; }
.pending-modal { width: min(700px, 92vw); max-height: 86vh; display: flex; flex-direction: column; }
.pending-head { display: flex; align-items: center; gap: 12px; }
.pending-head h2 { margin: 0; font-size: 15px; }
.pending-head .detail-close { margin-left: auto; }
.pending-banner { border: 1px solid var(--border); border-radius: 4px; padding: 8px 12px; margin: 12px 0 4px; }
.pending-banner p { margin: 2px 0; }
.pending-banner-warn { background: var(--warn-bg, #fef3c7); border-color: var(--warn-border, #f59e0b); }
.pending-list { display: flex; flex-direction: column; gap: 12px; overflow: auto; padding: 8px 0; }
.pending-card { border: 1px solid var(--border); border-radius: 6px; padding: 10px 12px; }
.pending-card-conflict { background: #fff7ed; border-color: #f59e0b; }
.pending-card-head { display: flex; align-items: center; gap: 10px; }
.pending-card-summary { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pending-discard { margin-left: auto; }
.pending-rows { list-style: none; margin: 8px 0 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
.pending-row { display: grid; grid-template-columns: 110px 1fr auto 1fr auto; gap: 8px; align-items: center; font-size: 13px; }
.pending-empty { padding: 24px 0; text-align: center; }
.pending-footer { display: flex; align-items: center; gap: 12px; border-top: 1px solid var(--border); padding-top: 12px; margin-top: 8px; }
.pending-footer-buttons { margin-left: auto; display: flex; gap: 8px; }
.b { font-weight: 600; }
```

If `--warn-bg` and friends are not tokens in `@agile-suite/core/styles/tokens.css`, the fallbacks above render the mockup's amber.

- [ ] **Step 10: Shell test**

In `tam/frontend/src/App.test.tsx`, add to the `vi.mock("./api", ...)` block:

```ts
    ListPendingChanges: vi.fn(),
    DiscardPendingChange: vi.fn(),
    DiscardAllPendingChanges: vi.fn(),
    CommitPendingChanges: vi.fn(),
```

and in the file's `beforeEach` (where `ListIssues` and the others get their default resolved values), add `vi.mocked(api.ListPendingChanges).mockResolvedValue([]);`. Then add a test next to the existing sync tests:

```tsx
  it("shows the Commit chip with the pending count and opens the dialog", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 1, entityType: "issue", entityKey: "PLAT-409", field: "priority", beforeVal: "Medium", afterVal: "High", baseVersion: "v1", createdAt: "" },
      { id: 2, entityType: "issue", entityKey: "PLAT-409", field: "assignee", beforeVal: "", afterVal: "M. Ortiz", baseVersion: "v1", createdAt: "" },
      { id: 3, entityType: "issue_create", entityKey: "TAM-NEW-1", field: "create", beforeVal: "", afterVal: '{"type":"task","summary":"x","description":"","priority":"","labels":[],"assignee":"","storyPoints":null,"extra":{}}', baseVersion: "", createdAt: "" },
    ]);
    renderApp();
    const chip = await screen.findByRole("button", { name: "3 pending changes: Commit" });
    await user.click(chip);
    expect(await screen.findByRole("dialog", { name: "Pending changes" })).toBeInTheDocument();
  });
```

Match how the existing shell tests wait for a profile to load (they call `renderApp()` and then `findBy...`); if they mock `Health` and `ListProfiles` in `beforeEach`, this test inherits that.

- [ ] **Step 11: Run everything**

From the repo root:

```bash
npm run typecheck --workspace tam/frontend && npm test --workspace tam/frontend 2>&1 | grep -E "Tests |FAIL"
```

Expected: typecheck clean; every test passes (the previous count plus five). The reducer test in `frontend/core` is untouched.

- [ ] **Step 12: Commit**

```bash
git add tam/frontend/src
git commit -m "feat(tam): commit action, Commit chip, and the Pending changes dialog"
```

---

### Task 8: The editable Details tab, the Activity tab, and the grid markers

The detail panel's Details tab gains inputs for the six fields and a Save edit button; a fourth tab lists the issue's local activity; grid rows show the pending dot and the Draft chip.

**Files:**
- Create: `tam/frontend/src/components/EditableFields.tsx`, `components/ActivityTab.tsx`
- Modify: `tam/frontend/src/components/IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx`, `IssueTable.tsx`, `BacklogView.test.tsx`, `App.css`
- Test: `npm test --workspace tam/frontend -- IssueDetailPanel BacklogView` from the repo root

**Interfaces:**
- Consumes: `useEditIssue`, `useActivity` (Task 7), `EDITABLE_FIELDS`, `fieldLabel`, `Issue.pending`, `Issue.draft`.
- Produces: `EditableFields({ profileId, issue, description, descriptionReady })`, `ActivityTab({ profileId, issueKey })`; CSS classes `.pending-dot`, `.edit-form`, `.edit-row`, `.edit-actions`, `.activity-list`, `.activity-row`.

- [ ] **Step 1: Panel tests**

In `tam/frontend/src/components/IssueDetailPanel.test.tsx`, extend the mock so the write bindings are mocked:

```ts
vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn(), EditIssue: vi.fn(), ListActivity: vi.fn() };
});
```

In `beforeEach`, add:

```ts
  vi.mocked(api.EditIssue).mockResolvedValue();
  vi.mocked(api.ListActivity).mockResolvedValue([
    { id: 2, occurredAt: "2026-09-06T10:05:00Z", actor: "araha", entityType: "issue", entityKey: "PLAT-412", action: "edit", field: "storyPoints", beforeVal: "5", afterVal: "8", note: "" },
    { id: 1, occurredAt: "2026-09-06T10:00:00Z", actor: "araha", entityType: "issue", entityKey: "PLAT-412", action: "edit", field: "summary", beforeVal: "Checkout: apply promo code", afterVal: "Checkout: apply promo code at payment step", note: "" },
  ]);
```

Then append a `describe` block:

```tsx
describe("IssueDetailPanel write path", () => {
  it("prefills the editable fields and saves only what changed, in field order", async () => {
    const user = userEvent.setup();
    renderPanel();
    const summary = await screen.findByLabelText("Summary");
    expect(summary).toHaveValue("Checkout: apply promo code at payment step");
    expect(screen.getByLabelText("Labels")).toHaveValue("checkout, promo");
    expect(screen.getByLabelText("Story points")).toHaveValue("5");
    const description = await screen.findByLabelText("Description");
    await waitFor(() => expect(description).toHaveValue("As a shopper I can enter a promo code on the payment step."));
    const save = screen.getByRole("button", { name: "Save edit" });
    expect(save).toBeDisabled();

    await user.clear(screen.getByLabelText("Story points"));
    await user.type(screen.getByLabelText("Story points"), "8");
    await user.clear(summary);
    await user.type(summary, "Checkout: promo code at payment");
    expect(save).toBeEnabled();
    await user.click(save);
    await waitFor(() => expect(api.EditIssue).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.EditIssue).mock.calls[0]).toEqual(["p1", "PLAT-412", "summary", "Checkout: promo code at payment"]);
    expect(vi.mocked(api.EditIssue).mock.calls[1]).toEqual(["p1", "PLAT-412", "storyPoints", "8"]);
    expect(await screen.findByText("Saved. Commit pushes it to Jira.")).toBeInTheDocument();
  });

  it("refuses a blank summary and non-numeric points before calling the backend", async () => {
    const user = userEvent.setup();
    renderPanel();
    const summary = await screen.findByLabelText("Summary");
    await user.clear(summary);
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText("Summary cannot be empty.")).toBeInTheDocument();
    await user.type(summary, "x");
    await user.clear(screen.getByLabelText("Story points"));
    await user.type(screen.getByLabelText("Story points"), "eight");
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText("Story points must be a number.")).toBeInTheDocument();
    expect(api.EditIssue).not.toHaveBeenCalled();
  });

  it("shows the backend's error and keeps the edits in the form", async () => {
    const user = userEvent.setup();
    vi.mocked(api.EditIssue).mockRejectedValueOnce(new Error("field \"priority\" cannot be edited"));
    renderPanel();
    const priority = await screen.findByLabelText("Priority");
    await user.clear(priority);
    await user.type(priority, "Highest");
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText(/cannot be edited/)).toBeInTheDocument();
    expect(priority).toHaveValue("Highest");
  });

  it("lists the activity newest first on the Activity tab", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(await screen.findByRole("tab", { name: "Activity" }));
    const items = await screen.findAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent("araha edited Story points: 5 to 8");
    expect(items[1]).toHaveTextContent("araha edited Summary");
    expect(api.ListActivity).toHaveBeenCalledWith("p1", "PLAT-412", 200);
  });

  it("marks a draft in the panel head", async () => {
    render(
      <QueryClientProvider client={createQueryClient()}>
        <IssueDetailPanel profileId="p1" issue={{ ...story, key: "TAM-NEW-1", status: "Draft", draft: true, pending: true }} onClose={vi.fn()} />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Draft")).toBeInTheDocument();
    expect(screen.getByText("Commit creates this issue in Jira and gives it a real key.")).toBeInTheDocument();
  });
});
```

If the file's `renderPanel` helper does not accept an issue override, the draft test renders inline as shown. `story` is the fixture already in the file.

- [ ] **Step 2: Grid tests**

In `tam/frontend/src/components/BacklogView.test.tsx`, append a test to the existing `describe`:

```tsx
  it("shows the pending dot and the Draft chip on rows", async () => {
    vi.mocked(api.ListIssues).mockResolvedValue({
      issues: [
        issue({ key: "TAM-NEW-1", summary: "Add a retry", status: "Draft", draft: true, pending: true }),
        issue({ key: "PLAT-412", summary: "Promo", status: "In Progress", pending: true }),
        issue({ key: "PLAT-409", summary: "Rotate keys" }),
      ],
      total: 3,
    });
    renderView();
    const rows = await screen.findAllByRole("row");
    // rows[0] is the header row.
    expect(within(rows[1]).getByLabelText("Pending changes")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Draft")).toBeInTheDocument();
    expect(within(rows[2]).getByLabelText("Pending changes")).toBeInTheDocument();
    expect(within(rows[3]).queryByLabelText("Pending changes")).not.toBeInTheDocument();
  });
```

- [ ] **Step 3: Run them**

Run from the repo root: `npm test --workspace tam/frontend -- IssueDetailPanel BacklogView`
Expected: the new tests FAIL (no inputs, no Activity tab, no dot).

- [ ] **Step 4: `EditableFields.tsx`**

Create `tam/frontend/src/components/EditableFields.tsx`:

```tsx
import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { errMsg } from "@agile-suite/core";
import { EDITABLE_FIELDS } from "../api";
import type { EditableField, Issue } from "../api";
import { useEditIssue } from "../queries/pending";

interface Props {
  profileId: string;
  issue: Issue;
  // description is the cached detail's text; descriptionReady says the
  // detail has loaded, so the textarea is enabled and its edit is genuine.
  description: string;
  descriptionReady: boolean;
}

type Values = Record<EditableField, string>;

function valuesOf(issue: Issue, description: string): Values {
  return {
    summary: issue.summary,
    description,
    priority: issue.priority,
    labels: issue.labels.join(", "),
    storyPoints: issue.storyPoints === null || issue.storyPoints === undefined ? "" : String(issue.storyPoints),
    assignee: issue.assignee,
  };
}

// EditableFields is the write half of the Details tab. Each field the user
// changes becomes one journal row when Save edit is pressed; unchanged
// fields are not sent. Validation mirrors the store's so the common
// mistakes never round-trip.
export function EditableFields({ profileId, issue, description, descriptionReady }: Props) {
  const base = valuesOf(issue, description);
  const [values, setValues] = useState<Values>(base);
  const [dirty, setDirty] = useState<Set<EditableField>>(new Set());
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const edit = useEditIssue(profileId);

  // A fresh row from the backend (after save, sync, or commit) resets the
  // fields the user has not touched; dirty ones keep their text.
  useEffect(() => {
    setValues((cur) => {
      const next = { ...base };
      for (const f of dirty) next[f] = cur[f];
      return next;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [issue, description]);

  function set(field: EditableField, v: string) {
    setValues((cur) => ({ ...cur, [field]: v }));
    setDirty((cur) => {
      const next = new Set(cur);
      if (v === base[field]) next.delete(field);
      else next.add(field);
      return next;
    });
    setSaved(false);
    setError("");
  }

  const changed = EDITABLE_FIELDS.map((f) => f.id).filter((f) => values[f] !== base[f]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (values.summary.trim() === "") {
      setError("Summary cannot be empty.");
      return;
    }
    if (values.storyPoints.trim() !== "" && Number.isNaN(Number(values.storyPoints.trim()))) {
      setError("Story points must be a number.");
      return;
    }
    setError("");
    try {
      for (const field of changed) {
        await edit.mutateAsync({ key: issue.key, field, value: values[field] });
      }
      setDirty(new Set());
      setSaved(true);
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <form className="edit-form" onSubmit={(e) => void onSubmit(e)} aria-label="Edit fields">
      {EDITABLE_FIELDS.map((f) => (
        <label key={f.id} className="edit-row">
          <span className="muted small">{f.label}</span>
          {f.id === "description" ? (
            <textarea
              className="detail-input"
              rows={5}
              value={values.description}
              disabled={!descriptionReady}
              placeholder={descriptionReady ? "" : "Loading the description"}
              onChange={(e) => set("description", e.target.value)}
            />
          ) : (
            <input
              className="detail-input"
              type="text"
              inputMode={f.id === "storyPoints" ? "decimal" : undefined}
              value={values[f.id]}
              onChange={(e) => set(f.id, e.target.value)}
            />
          )}
        </label>
      ))}
      <div className="edit-actions">
        <button type="submit" className="btn btn-primary" disabled={changed.length === 0 || edit.isPending}>
          {edit.isPending ? "Saving" : "Save edit"}
        </button>
        {error ? (
          <span className="error-text small" role="alert">{error}</span>
        ) : saved ? (
          <span className="muted small" role="status">Saved. Commit pushes it to Jira.</span>
        ) : (
          <span className="muted small">Labels are a comma list. Saving journals the change; nothing reaches Jira until Commit.</span>
        )}
      </div>
    </form>
  );
}
```

- [ ] **Step 5: `ActivityTab.tsx`**

Create `tam/frontend/src/components/ActivityTab.tsx`:

```tsx
import { fieldLabel } from "../api";
import type { AuditEntry } from "../api";
import { useActivity } from "../queries/pending";
import { formatWhen } from "../lib/format";

interface Props {
  profileId: string;
  issueKey: string;
}

// describe turns one audit entry into a sentence: who did what to which field.
export function describe(a: AuditEntry): string {
  const field = a.field ? fieldLabel(a.field) : "";
  const change = a.field ? `${field}: ${a.beforeVal || "(none)"} to ${a.afterVal || "(none)"}` : "";
  switch (a.action) {
    case "edit":
      return `${a.actor} edited ${change}`;
    case "create":
      return `${a.actor} drafted this issue: ${a.afterVal}`;
    case "created":
      return `${a.actor} created it in Jira as ${a.afterVal} (was ${a.beforeVal})`;
    case "commit":
      return a.field ? `${a.actor} pushed ${change}` : `${a.actor} pushed the draft to Jira`;
    case "discard":
      return a.field ? `${a.actor} discarded ${field}: back to ${a.afterVal || "(none)"}` : `${a.actor} discarded the draft`;
    case "override":
      return `${a.actor} chose to override Jira's version ${a.afterVal}`;
    default:
      return `${a.actor} ${a.action}${change ? " " + change : ""}`;
  }
}

export function ActivityTab({ profileId, issueKey }: Props) {
  const activity = useActivity(profileId, issueKey);
  return (
    <div role="tabpanel" id="panel-activity" aria-labelledby="tab-activity" className="tab-panel">
      <div className="detail-section-head">
        <h3>Local activity</h3>
        <button type="button" className="btn btn-ghost" onClick={() => void activity.refetch()} disabled={activity.isFetching}>
          {activity.isFetching ? "Refreshing" : "Refresh"}
        </button>
      </div>
      {activity.isPending ? (
        <p className="muted">Loading activity</p>
      ) : activity.isError ? (
        <p className="error-text" data-testid="activity-error">
          Could not load the activity: {activity.error.message}{" "}
          <button type="button" className="btn btn-ghost" onClick={() => void activity.refetch()}>Retry</button>
        </p>
      ) : activity.data.length === 0 ? (
        <p className="muted">No local activity yet. Edits, commits, and discards land here.</p>
      ) : (
        <ul className="activity-list">
          {activity.data.map((a) => (
            <li key={a.id} className="activity-row">
              <span className="muted small">{formatWhen(a.occurredAt)}</span>
              <span>{describe(a)}</span>
              {a.note && <span className="muted small">{a.note}</span>}
            </li>
          ))}
        </ul>
      )}
      <p className="muted small detail-note">This is TAM's own trail, not Jira's history.</p>
    </div>
  );
}
```

- [ ] **Step 6: The panel**

In `tam/frontend/src/components/IssueDetailPanel.tsx`:

Add the imports:

```tsx
import { EditableFields } from "./EditableFields";
import { ActivityTab } from "./ActivityTab";
```

Change the tab list:

```tsx
type Tab = "details" | "links" | "tests" | "activity";

const TABS: { id: Tab; label: string }[] = [
  { id: "details", label: "Details" },
  { id: "links", label: "Links" },
  { id: "tests", label: "Tests" },
  { id: "activity", label: "Activity" },
];
```

In the head, after `<TypeChip type={issue.type} />`, add the Draft chip:

```tsx
        {issue.draft && <span className="chip chip-draft">Draft</span>}
```

Replace the Details tab panel with the read-only fields, the editable form, and the draft note:

```tsx
      {tab === "details" && (
        <div role="tabpanel" id="panel-details" aria-labelledby="tab-details" className="tab-panel">
          {issue.draft && (
            <p className="muted small detail-note">Commit creates this issue in Jira and gives it a real key.</p>
          )}
          <dl className="field-list">
            <dt>Status</dt><dd>{issue.status || "-"}</dd>
            <dt>Sprint</dt><dd>{issue.sprintName || "-"}</dd>
            <dt>{issue.type === "epic" ? "Parent" : "Epic"}</dt><dd>{issue.parentKey ? <span className="accent-text">{issue.parentKey}</span> : "-"}</dd>
            <dt>Updated</dt><dd>{formatWhen(issue.updated) || "-"}</dd>
          </dl>
          <div className="detail-section-head">
            <h3>Fields</h3>
            <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()} disabled={detail.isFetching}>
              {detail.isFetching ? "Refreshing" : "Refresh"}
            </button>
          </div>
          {detail.isError && (
            <p className="error-text" data-testid="detail-error">
              Could not load the details: {detail.error.message}{" "}
              <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()}>Retry</button>
            </p>
          )}
          <EditableFields
            profileId={profileId}
            issue={issue}
            description={detail.data?.description ?? ""}
            descriptionReady={detail.isSuccess}
          />
        </div>
      )}
```

And add the Activity tab after the Tests tab panel:

```tsx
      {tab === "activity" && <ActivityTab profileId={profileId} issueKey={issue.key} />}
```

The old Details tab test that looked for the description paragraph and the note "Fields load from the cache first" now has to look for the Description textarea; update that assertion in `IssueDetailPanel.test.tsx` to `expect(await screen.findByLabelText("Description")).toHaveValue("As a shopper I can enter a promo code on the payment step.")`. The `detail-error` test id still exists for the error test.

- [ ] **Step 7: The grid markers**

In `tam/frontend/src/components/IssueTable.tsx`, inside the row, change the key cell and the status cell:

```tsx
                <span role="gridcell" className="issue-key">
                  {iss.key}
                  {iss.pending && <span className="pending-dot" role="img" aria-label="Pending changes" title="Has pending changes" />}
                </span>
```

```tsx
                <span role="gridcell">
                  {iss.draft
                    ? <span className="chip chip-draft">Draft</span>
                    : <span className={`chip chip-status chip-status-${statusClass(iss.status)}`}>{iss.status}</span>}
                </span>
```

- [ ] **Step 8: CSS**

Append to `tam/frontend/src/App.css`:

```css
/* Plan 1b: editable details, activity, grid markers */
.pending-dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; background: #f59e0b; margin-left: 6px; vertical-align: middle; }
.edit-form { display: flex; flex-direction: column; gap: 8px; margin-top: 8px; }
.edit-row { display: grid; grid-template-columns: 96px 1fr; gap: 8px; align-items: start; }
.edit-row .detail-input { width: 100%; box-sizing: border-box; }
.edit-actions { display: flex; align-items: center; gap: 10px; margin-top: 4px; }
.activity-list { list-style: none; margin: 8px 0 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.activity-row { display: grid; grid-template-columns: 110px 1fr; gap: 8px; font-size: 13px; }
.activity-row .small:last-child { grid-column: 2; }
```

- [ ] **Step 9: Run the suites**

From the repo root:

```bash
npm run typecheck --workspace tam/frontend && npm test --workspace tam/frontend 2>&1 | grep -E "Tests |FAIL"
```

Expected: clean; all tests pass. If the `useEffect` in `EditableFields` trips the `react-hooks/exhaustive-deps` lint in `npm run lint`, keep the disable comment: `base` is derived from `issue` and `description`, which are the effect's real inputs.

- [ ] **Step 10: Commit**

```bash
git add tam/frontend/src
git commit -m "feat(tam): editable Details tab, Activity tab, pending dot, and Draft chip"
```

---

### Task 9: The New issue dialog

A New button on the Backlog opens a dialog that captures the minimal form plus the type's required create-meta fields and stores a draft.

**Files:**
- Create: `tam/frontend/src/components/NewIssueModal.tsx`, `components/NewIssueModal.test.tsx`
- Modify: `tam/frontend/src/components/BacklogView.tsx`, `BacklogView.test.tsx`, `App.css`
- Test: `npm test --workspace tam/frontend -- NewIssueModal BacklogView` from the repo root

**Interfaces:**
- Consumes: `CreateIssue`, `useCreateFields` (Task 7), `useModal` with `"newIssue"`, `invalidateWrites`.
- Produces: `NewIssueModal({ onClose, onCreated })`; the Backlog's New button; CSS `.new-issue-modal`, `.meta-fields`.

- [ ] **Step 1: Dialog tests**

Create `tam/frontend/src/components/NewIssueModal.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { NewIssueModal } from "./NewIssueModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    GetCreateFields: vi.fn(),
    CreateIssue: vi.fn(),
  };
});

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderModal(onCreated = vi.fn(), onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <Loader />
          <NewIssueModal onClose={onClose} onCreated={onCreated} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onCreated, onClose };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetCreateFields).mockImplementation(async (_p, type) =>
    type === "bug"
      ? [{ id: "customfield_10050", name: "Severity", type: "option", required: true, allowedValues: [{ id: "1", value: "Minor" }, { id: "3", value: "Critical" }] }]
      : [],
  );
  vi.mocked(api.CreateIssue).mockResolvedValue("TAM-NEW-1");
});

describe("NewIssueModal", () => {
  it("creates a task from the minimal form", async () => {
    const user = userEvent.setup();
    const { onCreated, onClose } = renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    await user.type(within(dialog).getByLabelText("Summary"), "Add a retry to the payment webhook consumer");
    await user.type(within(dialog).getByLabelText("Labels"), "payments, webhooks");
    await user.type(within(dialog).getByLabelText("Story points"), "3");
    await user.type(within(dialog).getByLabelText("Assignee"), "mortiz");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    await waitFor(() => expect(api.CreateIssue).toHaveBeenCalledWith("p1", {
      type: "task", summary: "Add a retry to the payment webhook consumer", description: "", priority: "",
      labels: ["payments", "webhooks"], assignee: "mortiz", storyPoints: 3, extra: {},
    }));
    expect(onCreated).toHaveBeenCalledWith("TAM-NEW-1");
    expect(onClose).toHaveBeenCalled();
  });

  it("asks for the type's required create-meta fields and sends them as extra", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    await user.selectOptions(within(dialog).getByLabelText("Type"), "bug");
    const severity = await within(dialog).findByLabelText("Severity");
    await user.type(within(dialog).getByLabelText("Summary"), "Promo field accepts spaces");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    expect(await within(dialog).findByText("Severity is required.")).toBeInTheDocument();
    expect(api.CreateIssue).not.toHaveBeenCalled();
    await user.selectOptions(severity, "3");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    await waitFor(() => expect(api.CreateIssue).toHaveBeenCalled());
    expect(vi.mocked(api.CreateIssue).mock.calls[0][1].extra).toEqual({ customfield_10050: "3" });
    expect(vi.mocked(api.CreateIssue).mock.calls[0][1].type).toBe("bug");
  });

  it("degrades to the minimal form when create-meta cannot be read", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetCreateFields).mockRejectedValue(new Error("GET failed: 403"));
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    expect(await within(dialog).findByText(/Jira's required fields could not be read/)).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText("Summary"), "Still works");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    await waitFor(() => expect(api.CreateIssue).toHaveBeenCalled());
  });

  it("refuses a blank summary and shows the backend's error", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    expect(await within(dialog).findByText("Summary cannot be empty.")).toBeInTheDocument();
    vi.mocked(api.CreateIssue).mockRejectedValueOnce(new Error("summary cannot be empty"));
    await user.type(within(dialog).getByLabelText("Summary"), "x");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    expect(await within(dialog).findByText(/summary cannot be empty/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run them**

Run from the repo root: `npm test --workspace tam/frontend -- NewIssueModal`
Expected: FAIL, the component does not exist.

- [ ] **Step 3: The dialog**

Create `tam/frontend/src/components/NewIssueModal.tsx`:

```tsx
import { useState } from "react";
import type { FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, call, errMsg, useProfile } from "@agile-suite/core";
import { CreateIssue, ISSUE_TYPES } from "../api";
import type { FieldSpec, IssueDraft, IssueType, Profile, Settings } from "../api";
import { useCreateFields } from "../queries/pending";
import { invalidateWrites } from "../queries/invalidate";

interface Props {
  onClose: () => void;
  onCreated: (key: string) => void;
}

// CREATABLE are the types plan 1b can draft. Requirements and epics arrive
// with plan 1c.
const CREATABLE: IssueType[] = ["task", "story", "bug"];

// MetaField renders one create-meta field by its schema type: a select for
// option and array fields with values, a date or number input, or text.
function MetaField({ spec, value, onChange }: { spec: FieldSpec; value: string; onChange: (v: string) => void }) {
  const id = `meta-${spec.id}`;
  if ((spec.type === "option" || spec.type === "array") && spec.allowedValues.length > 0) {
    return (
      <label className="edit-row" htmlFor={id}>
        <span className="muted small">{spec.name}</span>
        <select id={id} className="detail-input" value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">Choose</option>
          {spec.allowedValues.map((o) => (
            <option key={o.id} value={o.id}>{o.value}</option>
          ))}
        </select>
      </label>
    );
  }
  return (
    <label className="edit-row" htmlFor={id}>
      <span className="muted small">{spec.name}</span>
      <input
        id={id}
        className="detail-input"
        type={spec.type === "date" ? "date" : "text"}
        inputMode={spec.type === "number" ? "decimal" : undefined}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  );
}

export function NewIssueModal({ onClose, onCreated }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const [type, setType] = useState<IssueType>("task");
  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("");
  const [labels, setLabels] = useState("");
  const [assignee, setAssignee] = useState("");
  const [points, setPoints] = useState("");
  const [extra, setExtra] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const meta = useCreateFields(activeId, type);
  const specs = meta.data ?? [];

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (summary.trim() === "") {
      setError("Summary cannot be empty.");
      return;
    }
    if (points.trim() !== "" && Number.isNaN(Number(points.trim()))) {
      setError("Story points must be a number.");
      return;
    }
    for (const s of specs) {
      if (s.required && !(extra[s.id] ?? "").trim()) {
        setError(`${s.name} is required.`);
        return;
      }
    }
    const draft: IssueDraft = {
      type,
      summary: summary.trim(),
      description,
      priority: priority.trim(),
      labels: labels.split(",").map((l) => l.trim()).filter(Boolean),
      assignee: assignee.trim(),
      storyPoints: points.trim() === "" ? null : Number(points.trim()),
      extra: Object.fromEntries(Object.entries(extra).filter(([, v]) => v.trim() !== "")),
    };
    setError("");
    setSaving(true);
    try {
      const key = await call(() => CreateIssue(activeId, draft));
      invalidateWrites(qc, activeId);
      onCreated(key);
      onClose();
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal onClose={onClose} className="modal new-issue-modal" labelledBy="new-issue-title">
      <div className="pending-head">
        <h2 id="new-issue-title">New issue</h2>
        <span className="muted">{activeProfile ? `in ${activeProfile.projectKey}` : ""}</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>
      <form className="edit-form" onSubmit={(e) => void onSubmit(e)}>
        <label className="edit-row" htmlFor="new-type">
          <span className="muted small">Type</span>
          <select id="new-type" className="detail-input" value={type} onChange={(e) => { setType(e.target.value as IssueType); setExtra({}); setError(""); }}>
            {ISSUE_TYPES.filter((t) => CREATABLE.includes(t.id)).map((t) => (
              <option key={t.id} value={t.id}>{t.label}</option>
            ))}
          </select>
        </label>
        <label className="edit-row" htmlFor="new-summary">
          <span className="muted small">Summary</span>
          <input id="new-summary" className="detail-input" type="text" value={summary} onChange={(e) => setSummary(e.target.value)} autoFocus />
        </label>
        <label className="edit-row" htmlFor="new-description">
          <span className="muted small">Description</span>
          <textarea id="new-description" className="detail-input" rows={4} value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-priority">
          <span className="muted small">Priority</span>
          <input id="new-priority" className="detail-input" type="text" placeholder="Jira's default when empty" value={priority} onChange={(e) => setPriority(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-labels">
          <span className="muted small">Labels</span>
          <input id="new-labels" className="detail-input" type="text" placeholder="comma separated" value={labels} onChange={(e) => setLabels(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-assignee">
          <span className="muted small">Assignee</span>
          <input id="new-assignee" className="detail-input" type="text" placeholder="Jira username" value={assignee} onChange={(e) => setAssignee(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-points">
          <span className="muted small">Story points</span>
          <input id="new-points" className="detail-input" type="text" inputMode="decimal" value={points} onChange={(e) => setPoints(e.target.value)} />
        </label>

        <div className="meta-fields">
          {meta.isError ? (
            <p className="muted small">Jira's required fields could not be read ({meta.error.message}). The form stays minimal; Jira validates the rest on Commit.</p>
          ) : meta.isPending ? (
            <p className="muted small">Checking which fields Jira requires</p>
          ) : specs.length > 0 ? (
            <>
              <p className="muted small">Jira requires these for a {ISSUE_TYPES.find((t) => t.id === type)?.label ?? type}:</p>
              {specs.map((s) => (
                <MetaField key={s.id} spec={s} value={extra[s.id] ?? ""} onChange={(v) => setExtra((cur) => ({ ...cur, [s.id]: v }))} />
              ))}
            </>
          ) : null}
        </div>

        <div className="edit-actions">
          <button type="submit" className="btn btn-primary" disabled={saving}>{saving ? "Creating" : "Create draft"}</button>
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          {error ? (
            <span className="error-text small" role="alert">{error}</span>
          ) : (
            <span className="muted small">The draft joins the Backlog now; Commit creates it in Jira.</span>
          )}
        </div>
      </form>
    </Modal>
  );
}
```

- [ ] **Step 4: The New button on the Backlog**

In `tam/frontend/src/components/BacklogView.tsx`:

Add the imports:

```tsx
import { useModal } from "../modals";
import { NewIssueModal } from "./NewIssueModal";
```

Inside `BacklogView()`, after `const sprints = useSprints(activeId);`:

```tsx
  const { isOpen, openModal, closeModal } = useModal();
```

Replace the filter bar's `<span className="muted small filter-note">Read-only until plan 1b</span>` with:

```tsx
        <button type="button" className="btn btn-primary filter-new" disabled={!activeId} onClick={() => openModal("newIssue")}>
          + New
        </button>
```

And render the dialog at the end of the section, after the detail panel block:

```tsx
      {isOpen("newIssue") && (
        <NewIssueModal
          onClose={closeModal}
          onCreated={(key) => {
            setPage(0);
            setSelectedKey(key);
          }}
        />
      )}
```

`BacklogView` is rendered inside `ModalProvider` in `main.tsx` and in `App.test.tsx`; the `BacklogView.test.tsx` render helper wraps only `ProfileProvider`, so wrap its tree in `ModalProvider` too (import it from `../modals`). Any existing Backlog test that asserted the text "Read-only until plan 1b" changes to assert the New button: `expect(screen.getByRole("button", { name: "+ New" })).toBeInTheDocument()`.

- [ ] **Step 5: Drafts sort first**

In `tam/internal/issuerepo/issues.go`, change the `ListIssues` ordering so drafts lead the Backlog, as the mockup shows:

```go
			` ORDER BY CASE WHEN key LIKE '`+DraftPrefix+`%' THEN 0 WHEN rank = '' THEN 2 ELSE 1 END, rank, key LIMIT ? OFFSET ?`,
```

and in `tam/internal/issuerepo/pending_test.go`, `TestFullSyncClearKeepsDraftRows` now expects `keys[0] == "TAM-NEW-1"` and `keys[1] == "PLAT-2"`. Run `go test ./internal/issuerepo/` inside `tam/` and expect PASS.

- [ ] **Step 6: CSS**

Append to `tam/frontend/src/App.css`:

```css
/* Plan 1b: new issue */
.filter-new { margin-left: auto; }
.new-issue-modal { width: min(640px, 92vw); max-height: 86vh; overflow: auto; }
.meta-fields { display: flex; flex-direction: column; gap: 8px; border-top: 1px solid var(--border); padding-top: 8px; }
```

Remove the now unused `.filter-note` rule if nothing else uses it.

- [ ] **Step 7: Run everything**

From the repo root and inside `tam/`:

```bash
npm run typecheck --workspace tam/frontend && npm test --workspace tam/frontend 2>&1 | grep -E "Tests |FAIL"
cd tam && go test ./internal/issuerepo/ -count=1
```

Expected: clean and green.

- [ ] **Step 8: Commit**

```bash
git add tam/frontend/src tam/internal/issuerepo
git commit -m "feat(tam): New issue dialog with create-meta fields, drafts lead the Backlog"
```

---

### Task 10: Conflict resolution, docs, and the whole-plan verification

The held issue's card in the Pending changes dialog gets its base, mine, remote table and the two resolutions; the docs record what shipped; the whole suite runs.

**Files:**
- Replace: `tam/frontend/src/components/ConflictCard.tsx` (the Task 7 placeholder)
- Modify: `tam/frontend/src/contexts/SyncContext.tsx` (`dismissConflict`), `components/PendingChangesModal.test.tsx`, `App.css`
- Modify: `tam/CLAUDE.md`, `README.md`, `docs/superpowers/specs/2026-09-05-tam-issues-design.md` (a 13.10 subsection)
- Test: every suite, then `wails build` inside `tam/`

**Interfaces:**
- Consumes: `ResolveConflictOverride`, `ResolveConflictKeepRemote`, `Conflict`, `invalidateWrites`, `useSync`.
- Produces: `ConflictCard({ profileId, conflict, disabled })`; `SyncApi.dismissConflict(key)`.

- [ ] **Step 1: Tests**

Append to the `describe` in `tam/frontend/src/components/PendingChangesModal.test.tsx`:

```tsx
  it("shows a held issue with base, mine, and remote and resolves it either way", async () => {
    const user = userEvent.setup();
    const conflictRows: PendingChange[] = [
      { id: 5, entityType: "issue", entityKey: "PLAT-412", field: "storyPoints", beforeVal: "5", afterVal: "8", baseVersion: "v1", createdAt: "" },
      { id: 4, entityType: "issue", entityKey: "PLAT-412", field: "labels", beforeVal: "checkout, promo", afterVal: "checkout, promo, q3", baseVersion: "v1", createdAt: "" },
      ...rows,
    ];
    // The first read shows every row; every read after the commit shows only
    // the held issue's rows, as the store would.
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce(conflictRows).mockResolvedValue(conflictRows.slice(0, 2));
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: ["PLAT-409"], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501" }],
      conflicts: [{ key: "PLAT-412", summary: "Checkout: apply promo code at payment step", remoteVersion: "2026-09-06T11:00:00Z", fields: [
        { field: "storyPoints", base: "5", mine: "8", remote: "13" },
        { field: "labels", base: "checkout, promo", mine: "checkout, promo, q3", remote: "checkout, promo" },
      ] }],
      failures: [], remaining: 2,
    });
    vi.mocked(api.ResolveConflictOverride).mockResolvedValue();
    vi.mocked(api.ResolveConflictKeepRemote).mockResolvedValue();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(within(dialog).getByRole("button", { name: "Commit (3)" }));
    expect(await within(dialog).findByText("Last commit: 1 issue pushed, 1 created (TAM-NEW-1 is now PLAT-501), 1 held back.")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-412 changed in Jira since you edited it. Resolve it below, then commit again.")).toBeInTheDocument();

    const card = await within(dialog).findByRole("group", { name: "PLAT-412" });
    expect(within(card).getByText("Conflict")).toBeInTheDocument();
    const table = within(card).getByRole("table");
    const bodyRows = within(table).getAllByRole("row").slice(1);
    expect(bodyRows[0]).toHaveTextContent("Story points 5 8 13");
    expect(bodyRows[1]).toHaveTextContent("Labels checkout, promo checkout, promo, q3 checkout, promo");
    expect(within(dialog).getByRole("button", { name: "Commit (0)" })).toBeDisabled();

    await user.click(within(card).getByRole("button", { name: "Override" }));
    await waitFor(() => expect(api.ResolveConflictOverride).toHaveBeenCalledWith("p1", "PLAT-412", "2026-09-06T11:00:00Z"));
    await waitFor(() => expect(within(dialog).queryByText("Conflict")).not.toBeInTheDocument());
    expect(await within(dialog).findByRole("button", { name: "Commit (1)" })).toBeEnabled();
  });

  it("keep remote drops the edits", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 5, entityType: "issue", entityKey: "PLAT-412", field: "storyPoints", beforeVal: "5", afterVal: "8", baseVersion: "v1", createdAt: "" },
    ]);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [],
      conflicts: [{ key: "PLAT-412", summary: "Promo", remoteVersion: "v2", fields: [{ field: "storyPoints", base: "5", mine: "8", remote: "13" }] }],
      failures: [], remaining: 1,
    });
    vi.mocked(api.ResolveConflictKeepRemote).mockResolvedValue();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(within(dialog).getByRole("button", { name: "Commit (1)" }));
    const card = await within(dialog).findByRole("group", { name: "PLAT-412" });
    vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
    await user.click(within(card).getByRole("button", { name: "Keep remote" }));
    await waitFor(() => expect(api.ResolveConflictKeepRemote).toHaveBeenCalledWith("p1", "PLAT-412"));
    expect(await within(dialog).findByText("Nothing pending. Edit an issue or create one and it shows up here.")).toBeInTheDocument();
  });
```

Run from the repo root: `npm test --workspace tam/frontend -- PendingChangesModal`
Expected: the two new tests FAIL (no table, no buttons).

- [ ] **Step 2: `dismissConflict` on the provider**

In `tam/frontend/src/contexts/SyncContext.tsx`, add to `SyncApi`:

```ts
  // dismissConflict drops one held issue from lastCommit once it has been
  // resolved, so the dialog shows it as an ordinary group (override) or
  // not at all (keep remote).
  dismissConflict: (key: string) => void;
```

Inside `SyncProvider`, after `runCommit`:

```ts
  const dismissConflict = useCallback((key: string) => {
    setLastCommit((cur) => (cur ? { ...cur, conflicts: cur.conflicts.filter((c) => c.key !== key) } : cur));
  }, []);
```

Add `dismissConflict` to the memoised `api` object and its dependency list.

- [ ] **Step 3: `ConflictCard.tsx`**

Replace `tam/frontend/src/components/ConflictCard.tsx` with:

```tsx
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { call, errMsg } from "@agile-suite/core";
import { ResolveConflictKeepRemote, ResolveConflictOverride, fieldLabel } from "../api";
import type { Conflict } from "../api";
import { invalidateWrites } from "../queries/invalidate";
import { useSync } from "../contexts/SyncContext";

interface Props {
  profileId: string;
  conflict: Conflict;
  disabled: boolean;
}

// ConflictCard is a held issue inside the Pending changes dialog: the
// three-way table and the two resolutions. Override rebases the edits so
// the next Commit pushes them; Keep remote drops them and takes Jira's row.
export function ConflictCard({ profileId, conflict, disabled }: Props) {
  const qc = useQueryClient();
  const { dismissConflict } = useSync();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function resolve(how: "override" | "keep") {
    setBusy(true);
    setError("");
    try {
      if (how === "override") {
        await call(() => ResolveConflictOverride(profileId, conflict.key, conflict.remoteVersion));
      } else {
        await call(() => ResolveConflictKeepRemote(profileId, conflict.key));
      }
      dismissConflict(conflict.key);
      invalidateWrites(qc, profileId, conflict.key);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="pending-card pending-card-conflict" role="group" aria-label={conflict.key}>
      <div className="pending-card-head">
        <span className="b">{conflict.key}</span>
        <span className="pending-card-summary">{conflict.summary}</span>
        <span className="chip chip-conflict">Conflict</span>
      </div>
      <table className="conflict-table">
        <thead>
          <tr><th scope="col">Field</th><th scope="col">Base</th><th scope="col">Mine</th><th scope="col">Remote</th></tr>
        </thead>
        <tbody>
          {conflict.fields.map((f) => (
            <tr key={f.field}>
              <td>{fieldLabel(f.field)}</td>
              <td>{f.base || "(none)"}</td>
              <td className="b">{f.mine || "(none)"}</td>
              <td className={f.remote !== f.base ? "danger-text" : ""}>{f.remote || "(none)"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="muted small">Base is the value when you edited. Remote is Jira now. Override pushes Mine over Remote; Keep remote drops your edits.</p>
      <div className="edit-actions">
        <button type="button" className="btn btn-primary" disabled={disabled || busy} onClick={() => void resolve("override")}>Override</button>
        <button type="button" className="btn" disabled={disabled || busy} onClick={() => void resolve("keep")}>Keep remote</button>
        {error && <span className="error-text small" role="alert">{error}</span>}
      </div>
    </section>
  );
}
```

- [ ] **Step 4: CSS**

Append to `tam/frontend/src/App.css`:

```css
/* Plan 1b: conflicts */
.conflict-table { width: 100%; border-collapse: collapse; margin-top: 8px; font-size: 13px; }
.conflict-table th { text-align: left; color: var(--muted); font-weight: 600; border-bottom: 1px solid var(--border); padding: 2px 8px 4px 0; }
.conflict-table td { padding: 4px 8px 4px 0; vertical-align: top; }
.danger-text { color: #b42318; }
```

- [ ] **Step 5: Run the frontend suite**

From the repo root: `npm run typecheck --workspace tam/frontend && npm test --workspace tam/frontend 2>&1 | grep -E "Tests |FAIL"`
Expected: clean and green. In the override test the `Commit (1)` count comes from the two PLAT-412 rows forming one pushable group once the conflict is dismissed.

- [ ] **Step 6: Docs**

In `tam/CLAUDE.md`, add a section after the one describing the read path (search for "detail panel" or "sync"; place it after that section):

```markdown
## The write path (plan 1b)

Edits and creates go through the journal in `tam.db` (`pending_change` and
`audit_log`, shared DDL and helpers in `core/journal`). `issuerepo.EditField`
writes the row and journals the change with the row's `updated` as the base
version; `CreateDraft` inserts a `TAM-NEW-n` row with status `Draft` and a
create row holding the draft as JSON. Sync never deletes a draft and never
overwrites a column with a pending edit. `internal/committer` pushes the
journal: drafts first (POST, then rekey), then per-issue version checks and
PUTs; an issue whose remote `updated` moved is held back with base, mine,
and remote per field, and the user picks Override (rebase, push next time)
or Keep remote (drop the edits, take Jira's row). Commit and sync exclude
each other through `App.busy` and the shared reducer's `committing` state.

The demo backend keeps writes in memory, hands out keys from 500, and
stages one conflict: the first Commit of an edit to the curated story
(`<project>-412`) is held back. Editable fields are summary, description,
priority, labels, story points, and assignee; drafts can be tasks, stories,
and bugs. Requirements, Excel import, and cross-project links are plan 1c.
```

In `README.md`, in the TAM paragraph or feature list, add one sentence: "Plan 1b adds local edits and drafts, a journal, Commit with conflict detection, and an Activity tab."

In `docs/superpowers/specs/2026-09-05-tam-issues-design.md`, append after 13.9:

```markdown
### 13.10 Implementation notes

Recorded when plan 1b was written. `core/journal` exposes `Get`, `Delete`, `DeleteForKey`, and `SetBaseVersion` keyed by entity key rather than a `Discard`, because the revert of an entity's columns belongs to each app. `IssueBackend` grew a fourth method, `GetIssue`, for the version check and the row refresh, and `UpdateIssue` takes the journal's text values keyed by logical field so the backend owns Jira's shapes. Description edits live in the cached detail, since the row has no description column. The conflict cards render inside the Pending changes dialog, as the mockup shows, rather than in a separate dialog. The demo's staged conflict is the curated story rekeyed to the profile's project, held back once.
```

- [ ] **Step 7: Whole-plan verification**

Run every gate:

```bash
cd core && go vet ./... && go test ./... -count=1 && cd ..
cd xtm && go vet ./... && go test ./internal/... -count=1 && cd ..
cd tam && gofmt -l . ./internal && go vet ./... && go test ./... -count=1 && cd ..
npm run typecheck --workspaces --if-present
npm test --workspaces --if-present 2>&1 | grep -E "Tests |FAIL"
cd tam && wails build && cd ..
git status --short --untracked-files=no
```

Expected: every Go package `ok`; `gofmt` silent; typecheck clean; every Vitest workspace green (XTM still 159); `wails build` produces `tam/build/bin/tam.exe`; the status shows only files this plan touched plus the wails line-ending churn under `tam/frontend/wailsjs/runtime`, `tam/frontend/package.json.md5`, and possibly `tam/go.mod`, which you revert with `git checkout --`.

Then the offline walk-through with `wails dev` inside `tam/` on the demo profile:

1. Open PLAT-412, change Story points to 8 on the Details tab, Save edit. The row gains the dot; the status bar shows "1 pending change: Commit"; the Activity tab lists the edit.
2. New, type Bug, summary "Promo field accepts spaces", Severity Critical, Create draft. `TAM-NEW-1` leads the Backlog with the Draft chip and the dot; the chip says 2.
3. Open the chip, Commit (2). The banner says 1 created (TAM-NEW-1 is now PLAT-500) and 1 held back; the PLAT-412 card shows Base 5, Mine 8, Remote 13.
4. Override, then Commit (1). The banner says 1 issue pushed; the dialog is empty; PLAT-412 shows 8 points and no dot.
5. Edit PLAT-409's priority, Commit, and confirm it pushes cleanly (no second staged conflict).
6. While a Full sync runs, the Commit chip's dialog button is disabled and the profile picker is disabled; while a Commit runs, the Sync menu items are disabled.
7. Discard all with two pending rows reverts both and clears the chip.

- [ ] **Step 8: Commit**

```bash
git add tam/frontend/src tam/CLAUDE.md README.md docs/superpowers/specs/2026-09-05-tam-issues-design.md
git commit -m "feat(tam): conflict resolution in the Pending changes dialog, plan 1b docs"
```

Then open the PR for the branch against `main` titled "Task Activity Manager issues, plan 1b: the write path", listing the ten tasks, the gates run, and the walk-through result. No AI attribution anywhere in it.

---

## Risky spots for the implementer

- **Task 1** must keep XTM's error strings byte-identical; the bodies are copied, not rewritten. Run XTM's Go suite before and after.
- **Task 3**'s `reapplyPending` runs inside the same transaction as the page upsert; it reads `pending_change` rows for the profile, so a full sync of a large project with many pending rows is still one pass.
- **Task 4**: `jiraFields` refuses `storyPoints` when the Story Points custom field was not discovered. The New issue form ignores points in that case (the create drops them silently, matching `SearchIssuesPage`'s empty column).
- **Task 6**: `wails generate module` must run after the Go build is clean, or `api.ts` will not compile in Task 7.
- **Task 7**: `groupPending` decides card order; the tests pin drafts first.
- **Task 8**: `EditableFields` keeps dirty fields across row refreshes; the effect's dependency list is deliberate.
- **Task 10**: the override test's `Commit (1)` depends on `dismissConflict` running before the queries refetch; both happen in `resolve`.
