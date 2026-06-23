# Test Run Info, View Sessions & Bug Detail Breakdown — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface real Xray test-run information across Test Cases / Sets / Plans / Executions, keep each view's state when switching views, and break down a bug's affected tests by run/execution/plan/version/project with an in-view read-only test detail.

**Architecture:** Four ordered phases. **A** adds an in-memory `useViewState` hook so unmounted views restore their state. **B** adds `test_run` + `exec_plan` tables fed by an Xray fetch during sync, plus repo queries and run-history UI. **C** enriches the bug affected-tests list (reusing B's run-history query). **D** adds a `readOnly` mode to `TestDetail` and a session-preserving bug→test navigation plus an in-view read-only sidebar.

**Tech Stack:** Go (backend, `modernc.org/sqlite`, no cgo) + Wails v2 + React/TypeScript (Vite). Backend logic in `internal/`; `app.go` is a thin Wails adapter. Demo mode short-circuits Jira. Spec: `docs/superpowers/specs/2026-06-23-test-run-info-sessions-bug-detail-design.md`.

**Conventions to follow:**
- Go: `gofmt`; document exported identifiers; no em dashes in comments (project rule).
- Schema: edit `baseSchema` (new installs) AND add an ordered `applyMigrations` block (existing DBs), then bump `schemaVersion`. Guard each `ALTER`/`CREATE` with the existing duplicate-column / `IF NOT EXISTS` pattern.
- Tests: add/extend `_test.go` beside backend changes. Verify with `go build ./...` and `go test ./...` (LSP diagnostics are often stale — trust the compiler/tests).
- Frontend: no test runner; verify with `cd frontend; npm run build` (tsc typecheck) and `wails dev` on a demo profile. Regenerate Wails bindings with `wails build` (or `wails dev`) after changing exported `App` methods or returned structs, then re-export in `frontend/src/api.ts`.
- Commits: no AI co-author trailer. Never stage `CLAUDE.md` or local tooling files.

---

## File Structure

**Create:**
- `frontend/src/lib/viewState.ts` — `useViewState` hook + module-level Map + `clearViewState`.
- `internal/jira/testruns.go` — `GetTestRuns` + exec→plan association (live + demo).
- `internal/testrepo/testruns.go` — `TestRunEntry`, `RunRollup`, `GetTestRunHistory`, `GetRunRollup`, `GetExecutionMembersWithRuns`.
- `internal/testrepo/testruns_test.go`, `internal/jira/testruns_test.go`, `internal/store/store_testrun_test.go`.

**Modify:**
- `internal/store/store.go` — `test_run`, `exec_plan` tables; `schemaVersion` 27 → 28.
- `internal/store/` store helpers (whichever file holds container upserts) — `UpsertTestRun`, `UpsertExecPlan`, `ReplaceRunsForExec`.
- `internal/syncer/` (the container-sync pass) — fetch runs + plan links and upsert.
- `internal/testrepo/bugs.go` — `BugTest.Project` populated in `ListTestsForBug`.
- `app.go` — `GetTestRunHistory`, `GetRunRollup`, `GetExecutionMembersWithRuns` adapters.
- `frontend/src/api.ts` — re-export new bindings.
- `frontend/src/App.tsx` — `clearViewState` on profile change; bug→test session-preserving nav; adopt `useViewState` for Browse-owned state.
- `frontend/src/components/TestDetail.tsx` — `readOnly` prop + Run history section.
- `frontend/src/components/BugsPanel.tsx` — Project column, expandable breakdown, open-detail action, read-only sidebar.
- `frontend/src/components/ContainersView.tsx` — execution run columns + plan/set roll-up; adopt `useViewState`.
- `frontend/src/components/{PreconditionsView,RequirementsView,DuplicatesView,GapAnalysisView,TestCallsView,Dashboard,TraceabilityTabs}.tsx` — adopt `useViewState` for their preserved fields.

---

# Phase A — Per-view session state

### Task A1: `useViewState` hook

**Files:**
- Create: `frontend/src/lib/viewState.ts`

- [ ] **Step 1: Write the hook + store**

```ts
// frontend/src/lib/viewState.ts
import { useCallback, useState } from "react";

// Module-level store: survives component unmount (so a view restores its state
// when you switch away and back), but NOT a full app reload. Keyed by
// "<profileId>:<viewKey>:<fieldKey>".
const store = new Map<string, unknown>();

function k(profileId: string, viewKey: string, fieldKey: string): string {
  return `${profileId}:${viewKey}:${fieldKey}`;
}

// useViewState behaves like useState but persists the value in the module store,
// so leaving and returning to a view restores the field. Pass the active
// profileId, a stable viewKey (e.g. "bugs"), and a fieldKey (e.g. "selected").
export function useViewState<T>(
  profileId: string,
  viewKey: string,
  fieldKey: string,
  initial: T,
): [T, (next: T | ((prev: T) => T)) => void] {
  const key = k(profileId, viewKey, fieldKey);
  const [value, setValue] = useState<T>(() =>
    store.has(key) ? (store.get(key) as T) : initial,
  );
  const set = useCallback(
    (next: T | ((prev: T) => T)) => {
      setValue((prev) => {
        const resolved =
          typeof next === "function" ? (next as (p: T) => T)(prev) : next;
        store.set(key, resolved);
        return resolved;
      });
    },
    [key],
  );
  return [value, set];
}

// clearViewState drops all stored state for a profile. Call this when the active
// profile changes so a new profile does not inherit stale selections.
export function clearViewState(profileId: string): void {
  const prefix = `${profileId}:`;
  for (const key of [...store.keys()]) {
    if (key.startsWith(prefix)) store.delete(key);
  }
}
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend; npm run build`
Expected: tsc passes (no usages yet; the module compiles).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/viewState.ts
git commit -m "feat(frontend): add useViewState hook for per-view session state"
```

### Task A2: Clear view state on profile change

**Files:**
- Modify: `frontend/src/App.tsx` (the effect that clears per-profile state when `profileId` changes; survey ~lines 318-324)

- [ ] **Step 1: Import and call clearViewState**

In `App.tsx`, add `import { clearViewState } from "./lib/viewState";` and, in the existing profile-change effect that resets `selectedKey`/sidebar selections, call `clearViewState(previousProfileId)` before switching. If the effect only has the new `profileId`, store the previous id in a ref to pass to `clearViewState`. Concrete:

```ts
// near other refs
const prevProfileRef = useRef<string>("");
useEffect(() => {
  if (prevProfileRef.current && prevProfileRef.current !== profileId) {
    clearViewState(prevProfileRef.current);
  }
  prevProfileRef.current = profileId;
  // ...existing per-profile resets stay here...
}, [profileId]);
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend; npm run build`
Expected: tsc passes.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat(frontend): clear view session state on profile change"
```

### Task A3: Adopt useViewState in the Containers/Bugs view

**Files:**
- Modify: `frontend/src/components/ContainersView.tsx`

This view backs both the Containers board and the Bugs panel toggle, and is the primary target for the bug workflow in Phase D, so do it first and in full.

- [ ] **Step 1: Replace preserved useState with useViewState**

For each field listed in the spec table for Containers (`kind`, selected container, status/execType/env filters, sort, `Bugs` toggle, selected bug), change `const [x, setX] = useState(init)` to `const [x, setX] = useViewState(profileId, "containers", "x", init)`. Import the hook. Leave ephemeral/in-flight state (busy flags, draft inputs) as plain `useState`.

- [ ] **Step 2: Typecheck + manual verify**

Run: `cd frontend; npm run build` (expect pass). Then `wails dev` on a demo profile: in Containers pick a Test Plan and a status filter, switch to Browse, switch back to Containers; the kind, selection, and filter are retained.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ContainersView.tsx
git commit -m "feat(frontend): preserve Containers/Bugs view state across view switches"
```

### Task A4: Adopt useViewState in the remaining views

**Files:**
- Modify: `PreconditionsView.tsx`, `RequirementsView.tsx`, `DuplicatesView.tsx`, `GapAnalysisView.tsx`, `TestCallsView.tsx`, `Dashboard.tsx`, `TraceabilityTabs.tsx`, and Browse-owned state in `App.tsx`/`TestTable.tsx`

Apply the same mechanical swap (`useState` → `useViewState(profileId, "<view>", "<field>", init)`) for the fields in the spec's per-view table. One commit per view keeps diffs reviewable.

- [ ] **Step 1: Per view** — swap the preserved fields, import the hook, `npm run build`, and `wails dev` smoke-check (set a filter/selection, leave, return, confirm retained).

- [ ] **Step 2: Commit per view**

```bash
git add frontend/src/components/RequirementsView.tsx
git commit -m "feat(frontend): preserve Requirements view state across view switches"
# ...repeat per view...
```

---

# Phase B — Test run information from Xray

### Task B1: Schema v28 — `test_run` and `exec_plan`

**Files:**
- Modify: `internal/store/store.go`
- Test: `internal/store/store_testrun_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/store/store_testrun_test.go
package store

import "testing"

func TestSchemaHasTestRunAndExecPlan(t *testing.T) {
	s := openTestStore(t) // follow the existing test-store helper used in this package
	for _, tbl := range []string{"test_run", "exec_plan"} {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil || name != tbl {
			t.Fatalf("expected table %q to exist, err=%v name=%q", tbl, err, name)
		}
	}
	if schemaVersion < 28 {
		t.Fatalf("schemaVersion = %d, want >= 28", schemaVersion)
	}
}
```

If `openTestStore` does not exist, use the same store-construction the other `_test.go` files in `internal/store` use (open an in-memory or temp-file DB through the package's normal `Open`).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/store/ -run TestSchemaHasTestRunAndExecPlan`
Expected: FAIL (tables missing / version < 28).

- [ ] **Step 3: Add tables to `baseSchema` and a migration**

In `baseSchema` add:

```sql
CREATE TABLE IF NOT EXISTS test_run (
  profile_id  TEXT NOT NULL,
  exec_key    TEXT NOT NULL,
  test_key    TEXT NOT NULL,
  run_status  TEXT DEFAULT '',
  started_at  TEXT DEFAULT '',
  finished_at TEXT DEFAULT '',
  executed_by TEXT DEFAULT '',
  environment TEXT DEFAULT '',
  defects     TEXT DEFAULT '',
  PRIMARY KEY (profile_id, exec_key, test_key)
);
CREATE TABLE IF NOT EXISTS exec_plan (
  profile_id TEXT NOT NULL,
  exec_key   TEXT NOT NULL,
  plan_key   TEXT NOT NULL,
  PRIMARY KEY (profile_id, exec_key, plan_key)
);
```

In `applyMigrations`, append an ordered block (matching the existing `if current < N { ... }` style, with the duplicate-object guard the other blocks use):

```go
if current < 28 {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS test_run ( ... same columns as above ... )`,
		`CREATE TABLE IF NOT EXISTS exec_plan ( ... same columns as above ... )`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil &&
			!strings.Contains(err.Error(), "already exists") {
			return err
		}
	}
}
```

Bump the constant: `schemaVersion = 28`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/store/ -run TestSchemaHasTestRunAndExecPlan`
Expected: PASS. Then `go build ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/store/store.go internal/store/store_testrun_test.go
git commit -m "feat(store): add test_run and exec_plan tables (schema v28)"
```

### Task B2: Store upsert/replace helpers

**Files:**
- Modify: the `internal/store` file holding container link upserts (the one with `ReplaceAllContainerLinks`)
- Test: extend `internal/store/store_testrun_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestReplaceRunsForExecAndRead(t *testing.T) {
	s := openTestStore(t)
	runs := []TestRunRow{{
		ExecKey: "DEMO-EXEC-1", TestKey: "DEMO-1", RunStatus: "FAIL",
		StartedAt: "2026-06-01T10:00:00Z", FinishedAt: "2026-06-01T10:05:00Z",
		ExecutedBy: "achmarah", Environment: "Staging", Defects: `["DEMO-9"]`,
	}}
	if err := s.ReplaceRunsForExec("p1", "DEMO-EXEC-1", runs); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertExecPlan("p1", "DEMO-EXEC-1", "DEMO-TP-1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.RunsForTest("p1", "DEMO-1")
	if err != nil || len(got) != 1 || got[0].RunStatus != "FAIL" {
		t.Fatalf("RunsForTest = %+v, err=%v", got, err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/store/ -run TestReplaceRunsForExecAndRead`
Expected: FAIL (undefined `TestRunRow`, `ReplaceRunsForExec`, etc.).

- [ ] **Step 3: Implement the row type + helpers**

Add a `TestRunRow` struct (the columns above) and methods on the store:
- `ReplaceRunsForExec(profileID, execKey string, runs []TestRunRow) error` — delete `test_run` rows for `(profile_id, exec_key)` then insert the given runs in one transaction (mirror the transaction style of `ReplaceAllContainerLinks`).
- `UpsertExecPlan(profileID, execKey, planKey string) error` — `INSERT OR IGNORE` into `exec_plan`.
- `ReplaceExecPlans(profileID, execKey string, planKeys []string) error` — delete then insert (used by sync).
- `RunsForTest(profileID, testKey string) ([]TestRunRow, error)` — `SELECT ... FROM test_run WHERE profile_id=? AND test_key=? ORDER BY finished_at DESC, exec_key`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/store/ -run TestReplaceRunsForExecAndRead`
Expected: PASS. Then `go build ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): test_run/exec_plan upsert and read helpers"
```

### Task B3: Jira `GetTestRuns` + exec→plan (live stub + demo)

**Files:**
- Create: `internal/jira/testruns.go`
- Test: `internal/jira/testruns_test.go`

- [ ] **Step 1: Write the failing test (demo path)**

```go
// internal/jira/testruns_test.go
package jira

import (
	"context"
	"testing"
)

func TestGetTestRunsDemo(t *testing.T) {
	c := NewClient("demo", "token") // demo short-circuit
	runs, err := c.GetTestRuns(context.Background(), "DEMO-EXEC-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 {
		t.Fatal("expected demo test runs, got none")
	}
	for _, r := range runs {
		if r.TestKey == "" || r.Status == "" {
			t.Fatalf("incomplete demo run: %+v", r)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/jira/ -run TestGetTestRunsDemo`
Expected: FAIL (undefined `GetTestRuns`).

- [ ] **Step 3: Implement**

```go
// internal/jira/testruns.go
package jira

import "context"

// TestRun is one test's run within a Test Execution (Xray test run).
type TestRun struct {
	TestKey     string
	Status      string
	StartedAt   string
	FinishedAt  string
	ExecutedBy  string
	Environment string
	Defects     []string
}

// GetTestRuns returns the test runs of one Test Execution. Demo mode synthesizes
// runs deterministically; the live path calls Xray.
func (c *Client) GetTestRuns(ctx context.Context, execKey string) ([]TestRun, error) {
	if isDemoURL(c.baseURL) {
		return demoTestRuns(execKey), nil
	}
	// NOTE(xtm): verify against Xray Server/DC 8.4.0. Likely
	// GET /rest/raven/2.0/api/testruns?testExecKey=<execKey> returning per-test
	// run records (status, startedOn/finishedOn, executedBy, testEnvironments,
	// defects). Decode into []TestRun. Reuse c.getBytes + a tolerant parse like
	// parseStepsResponse since shapes vary between Xray versions.
	body, err := c.getBytes(ctx, "/rest/raven/2.0/api/testruns?testExecKey="+execKey)
	if err != nil {
		return nil, err
	}
	return parseTestRuns(body)
}

// ExecPlans returns the Test Plan keys a Test Execution is associated with.
func (c *Client) ExecPlans(ctx context.Context, execKey string) ([]string, error) {
	if isDemoURL(c.baseURL) {
		return demoExecPlans(execKey), nil
	}
	// NOTE(xtm): verify. The execution's associated Test Plans come from the
	// Xray test-exec association (or a Test Plan custom field on the execution
	// issue). Return the plan keys.
	return nil, nil
}
```

Add `parseTestRuns([]byte) ([]TestRun, error)` (tolerant decode), `demoTestRuns(execKey)` and `demoExecPlans(execKey)` (seed runs for the execution's member tests, with a date, an executed-by, an environment drawn from the exec's demo environments, and a defect key for FAIL runs — keep consistent with the existing demo container/bug seeding so a FAILed run lines up with a demo bug).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/jira/ -run TestGetTestRunsDemo`
Expected: PASS. Then `go build ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/jira/testruns.go internal/jira/testruns_test.go
git commit -m "feat(jira): GetTestRuns and ExecPlans (demo + live stub)"
```

### Task B4: Sync — fetch and upsert runs + plan links

**Files:**
- Modify: the `internal/syncer` function that syncs containers/executions (the pass that calls the jira container fetch and `ReplaceAllContainerLinks`)

- [ ] **Step 1: Extend the execution sync**

In the execution-sync loop, for each execution key, call `client.GetTestRuns(ctx, execKey)` → map to `[]store.TestRunRow` → `store.ReplaceRunsForExec(profileID, execKey, rows)`, and `client.ExecPlans(ctx, execKey)` → `store.ReplaceExecPlans(profileID, execKey, planKeys)`. Reuse the existing progress reporting (the per-view sync progress shown in the status bar) so runs appear as part of container sync. Keep run status in `test_container_test` consistent (the run's status is the membership status).

- [ ] **Step 2: Verify**

Run: `go build ./...` then `go test ./internal/syncer/...`
Expected: build + existing syncer tests pass. Add a focused test if the syncer package has a demo-client harness: after a container sync, `store.RunsForTest` returns rows for a known demo test.

- [ ] **Step 3: Commit**

```bash
git add internal/syncer/
git commit -m "feat(syncer): fetch and store test runs and exec-plan links during container sync"
```

### Task B5: Repo `GetTestRunHistory` + types

**Files:**
- Create: `internal/testrepo/testruns.go`
- Test: `internal/testrepo/testruns_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/testrepo/testruns_test.go
package testrepo

import "testing"

func TestGetTestRunHistory(t *testing.T) {
	r := newDemoRepo(t) // follow the existing testrepo test harness (demo-seeded store)
	hist, err := r.GetTestRunHistory("p1", "DEMO-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) == 0 {
		t.Fatal("expected run history for a demo test, got none")
	}
	e := hist[0]
	if e.ExecKey == "" || e.RunStatus == "" {
		t.Fatalf("incomplete entry: %+v", e)
	}
}
```

Use whatever demo-seeded repo helper the existing `internal/testrepo/*_test.go` files use; seed `test_run`/`exec_plan` in the test if the harness does not (via the store helpers from B2).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/testrepo/ -run TestGetTestRunHistory`
Expected: FAIL (undefined `GetTestRunHistory`).

- [ ] **Step 3: Implement types + query**

```go
// internal/testrepo/testruns.go
package testrepo

// TestRunEntry is one execution-run of a test, with the execution's context.
type TestRunEntry struct {
	ExecKey     string   `json:"execKey"`
	ExecSummary string   `json:"execSummary"`
	PlanKeys    []string `json:"planKeys"`
	Environment string   `json:"environment"`
	FixVersions []string `json:"fixVersions"`
	RunStatus   string   `json:"runStatus"`
	StartedAt   string   `json:"startedAt"`
	FinishedAt  string   `json:"finishedAt"`
	ExecutedBy  string   `json:"executedBy"`
	Defects     []string `json:"defects"`
}

// GetTestRunHistory returns every execution-run of a test, newest finished
// first, with the execution summary, environments, fix versions, and associated
// Test Plans.
func (r *Repository) GetTestRunHistory(profileID, testKey string) ([]TestRunEntry, error) {
	rows, err := r.db.Query(`
		SELECT tr.exec_key, COALESCE(c.summary,''), tr.run_status,
		       tr.started_at, tr.finished_at, tr.executed_by, tr.environment,
		       tr.defects, COALESCE(c.fix_versions,'')
		FROM test_run tr
		LEFT JOIN test_container c
		  ON c.profile_id = tr.profile_id AND c.jira_key = tr.exec_key
		WHERE tr.profile_id = ? AND tr.test_key = ?
		ORDER BY tr.finished_at DESC, tr.exec_key`, profileID, testKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestRunEntry
	for rows.Next() {
		var e TestRunEntry
		var defectsJSON, fixJSON string
		if err := rows.Scan(&e.ExecKey, &e.ExecSummary, &e.RunStatus,
			&e.StartedAt, &e.FinishedAt, &e.ExecutedBy, &e.Environment,
			&defectsJSON, &fixJSON); err != nil {
			return nil, err
		}
		e.Defects = decodeJSONStrings(defectsJSON)   // reuse the existing JSON-array decoder used for environments/fix_versions
		e.FixVersions = decodeJSONStrings(fixJSON)
		e.PlanKeys = r.planKeysForExec(profileID, e.ExecKey)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) planKeysForExec(profileID, execKey string) []string {
	rows, err := r.db.Query(
		`SELECT plan_key FROM exec_plan WHERE profile_id=? AND exec_key=? ORDER BY plan_key`,
		profileID, execKey)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ks []string
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			ks = append(ks, k)
		}
	}
	return ks
}
```

If a shared JSON-array decoder does not exist under a reusable name, factor the one already used to read `test_container.environments` / `fix_versions` into `decodeJSONStrings`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/testrepo/ -run TestGetTestRunHistory`
Expected: PASS. Then `go build ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/testrepo/testruns.go internal/testrepo/testruns_test.go
git commit -m "feat(testrepo): GetTestRunHistory across executions with plan/env/version context"
```

### Task B6: Repo `GetRunRollup` + `GetExecutionMembersWithRuns`

**Files:**
- Modify: `internal/testrepo/testruns.go`
- Test: extend `internal/testrepo/testruns_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestGetRunRollup(t *testing.T) {
	r := newDemoRepo(t)
	roll, err := r.GetRunRollup("p1", "DEMO-TP-1") // a demo Test Plan
	if err != nil {
		t.Fatal(err)
	}
	if roll.Total == 0 {
		t.Fatal("expected a non-zero rollup total for a demo plan")
	}
}

func TestGetExecutionMembersWithRuns(t *testing.T) {
	r := newDemoRepo(t)
	rows, err := r.GetExecutionMembersWithRuns("p1", "DEMO-EXEC-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected execution members")
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/testrepo/ -run 'TestGetRunRollup|TestGetExecutionMembersWithRuns'`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement**

Add:

```go
// RunRollup summarizes run results for a Test Plan or Test Set across the
// executions that ran its member tests.
type RunRollup struct {
	Passed    int `json:"passed"`
	Failed    int `json:"failed"`
	NotRun    int `json:"notRun"`
	Executing int `json:"executing"`
	Aborted   int `json:"aborted"`
	Blocked   int `json:"blocked"`
	Total     int `json:"total"`
	ExecCount int `json:"execCount"`
}

// ExecMemberRun is one member test of an execution with its run details.
type ExecMemberRun struct {
	TestKey     string `json:"testKey"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	RunStatus   string `json:"runStatus"`
	StartedAt   string `json:"startedAt"`
	FinishedAt  string `json:"finishedAt"`
	ExecutedBy  string `json:"executedBy"`
	Environment string `json:"environment"`
}
```

Implement `GetRunRollup(profileID, containerKey)` by reusing the consolidation already in `GetContainerBoard` (`consolidateRunStatus`): for a plan/set, gather member tests, consolidate each member's status across executions (worst-wins), and bucket the counts; `ExecCount` = distinct executions touching the members. Implement `GetExecutionMembersWithRuns(profileID, execKey)` by joining `test_container_test` (members of the exec) `LEFT JOIN test_run` on `(exec_key, test_key)` and `test_case`/`external_test` for summary/status.

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/testrepo/ -run 'TestGetRunRollup|TestGetExecutionMembersWithRuns'`
Expected: PASS. Then `go test ./internal/testrepo/...` (no regressions) and `go build ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/testrepo/testruns.go internal/testrepo/testruns_test.go
git commit -m "feat(testrepo): GetRunRollup and GetExecutionMembersWithRuns"
```

### Task B7: App bindings + regenerate + api.ts

**Files:**
- Modify: `app.go`, `frontend/src/api.ts`

- [ ] **Step 1: Add the three adapters**

```go
// app.go
func (a *App) GetTestRunHistory(profileID, testKey string) ([]testrepo.TestRunEntry, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.GetTestRunHistory(profileID, testKey)
}

func (a *App) GetRunRollup(profileID, containerKey string) (testrepo.RunRollup, error) {
	if err := a.requireStore(); err != nil {
		return testrepo.RunRollup{}, err
	}
	return a.repo.GetRunRollup(profileID, containerKey)
}

func (a *App) GetExecutionMembersWithRuns(profileID, execKey string) ([]testrepo.ExecMemberRun, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.GetExecutionMembersWithRuns(profileID, execKey)
}
```

- [ ] **Step 2: Regenerate bindings**

Run: `wails build` (or start `wails dev`) to regenerate `frontend/wailsjs/go/main/App.*` and `models.ts`. Then add re-exports in `frontend/src/api.ts` for `GetTestRunHistory`, `GetRunRollup`, `GetExecutionMembersWithRuns` and the new model types, following how the file already re-exports bindings.

- [ ] **Step 3: Verify**

Run: `go build ./...` then `cd frontend; npm run build`
Expected: both pass; new bindings importable.

- [ ] **Step 4: Commit**

```bash
git add app.go frontend/wailsjs/ frontend/src/api.ts
git commit -m "feat(app): bind GetTestRunHistory, GetRunRollup, GetExecutionMembersWithRuns"
```

### Task B8: Run history section in `TestDetail`

**Files:**
- Modify: `frontend/src/components/TestDetail.tsx`

- [ ] **Step 1: Add a lazily-loaded Run history section**

Add state `const [runs, setRuns] = useState<TestRunEntry[] | null>(null);` and a collapsible section after the existing detail content. On first expand (or on first open, matching how steps/custom fields lazy-load), call `GetTestRunHistory(profileId, testKey)` and render a table: Execution (key links to Jira if `jiraUrl` set), Result (run-status badge reusing the existing run-status badge style), Date (`finishedAt` or `startedAt`), By (`executedBy`), Environment, Plan(s) (`planKeys`), Fix Version(s), Defects (keys link to Jira). Empty state: "No run history." Cache per `testKey` + `version` like other lazy sections.

- [ ] **Step 2: Verify**

Run: `cd frontend; npm run build` (expect pass). `wails dev` on demo: open a test that is in an execution; the Run history table populates.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/TestDetail.tsx
git commit -m "feat(frontend): run history section on the test detail panel"
```

### Task B9: Execution run columns + plan/set roll-up in Containers

**Files:**
- Modify: `frontend/src/components/ContainersView.tsx`

- [ ] **Step 1: Execution member run details**

When kind is Test Execution, fetch `GetExecutionMembersWithRuns(profileId, execKey)` for the selected execution and add columns for Date (`finishedAt`), By (`executedBy`), Environment to the member table (keep the existing inline run-result control).

- [ ] **Step 2: Plan/Set roll-up**

When kind is Test Plan or Test Set, fetch `GetRunRollup(profileId, key)` for the selected container and render a compact summary bar above the board (passed / failed / not-run / executing / aborted / blocked, with `execCount`), reusing the run-status badge colors.

- [ ] **Step 3: Verify**

Run: `cd frontend; npm run build` (expect pass). `wails dev`: execution members show date/by/env; a plan/set shows the roll-up bar.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/ContainersView.tsx
git commit -m "feat(frontend): execution run columns and plan/set run roll-up"
```

---

# Phase C — Bug detail affected-tests breakdown

### Task C1: `BugTest.Project` in `ListTestsForBug`

**Files:**
- Modify: `internal/testrepo/bugs.go`
- Test: extend the existing bugs test (`internal/testrepo/bugs_test.go` if present, else add one)

- [ ] **Step 1: Write the failing test**

```go
func TestListTestsForBugHasProject(t *testing.T) {
	r := newDemoRepo(t)
	bugs, err := r.ListBugsWithTests("p1")
	if err != nil || len(bugs) == 0 {
		t.Fatalf("need a demo bug, err=%v n=%d", err, len(bugs))
	}
	tests, err := r.ListTestsForBug("p1", bugs[0].Key)
	if err != nil || len(tests) == 0 {
		t.Fatalf("need affected tests, err=%v n=%d", err, len(tests))
	}
	if tests[0].Project == "" {
		t.Fatal("expected BugTest.Project to be populated")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/testrepo/ -run TestListTestsForBugHasProject`
Expected: FAIL (no `Project` field / empty).

- [ ] **Step 3: Add `Project` to `BugTest` and populate it**

Add `Project string \`json:"project"\`` to `BugTest`. In `ListTestsForBug`, set `Project` from `external_test.project_key` for cross-project members; otherwise from the test key's project prefix (the part before the last `-`) or the profile's project key, consistent with how `BugWithTests.ProjectKey` is derived elsewhere.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/testrepo/ -run TestListTestsForBugHasProject`
Expected: PASS. Then `go build ./...`. Regenerate bindings (`wails build`) so `BugTest.project` appears in `models.ts`.

- [ ] **Step 5: Commit**

```bash
git add internal/testrepo/bugs.go internal/testrepo/bugs_test.go frontend/wailsjs/
git commit -m "feat(testrepo): include project on a bug's affected tests"
```

### Task C2: Project column + expandable breakdown in BugsPanel

**Files:**
- Modify: `frontend/src/components/BugsPanel.tsx`

- [ ] **Step 1: Add Project column + expand control**

Add a Project column to the affected-tests table. Add a per-row expand toggle that, on first expand, calls `GetTestRunHistory(profileId, t.key)` and renders the breakdown inline beneath the row: a small table of Execution, Result, Fix Version(s), Plan(s), Environment, Date, By, Defects (the same columns as the Run history section, scoped to this test). Cache fetched history per test key in component state. Empty state: "No run history for this test."

- [ ] **Step 2: Verify**

Run: `cd frontend; npm run build` (expect pass). `wails dev`: Containers → Bugs → select a bug → expand an affected test → the run breakdown shows.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/BugsPanel.tsx
git commit -m "feat(frontend): project column and per-test run breakdown in bug detail"
```

---

# Phase D — Read-only TestDetail + bug navigation

### Task D1: `readOnly` mode on `TestDetail`

**Files:**
- Modify: `frontend/src/components/TestDetail.tsx`

- [ ] **Step 1: Add the prop and gate all edits**

Add `readOnly?: boolean` to `Props`. When true:
- Render field values as static text (or disabled inputs) instead of editable controls: summary, description, priority, labels, components, custom fields.
- Hide/disable the step editors and the add/reorder/delete/duplicate/clone-from controls; render steps read-only.
- Hide/disable precondition and requirement add/replace/unlink controls (still list them).
- Hide the workflow transition control and the Clone action.
- As a safety net, make each mutating handler `if (readOnly) return;` at its top.
Read paths (fields, steps, preconditions, requirements, custom fields, linked bugs, Run history from B8) render normally.

- [ ] **Step 2: Verify**

Run: `cd frontend; npm run build`
Expected: tsc passes. (Exercised in D2.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/TestDetail.tsx
git commit -m "feat(frontend): read-only mode for the test detail panel"
```

### Task D2: Bug→test navigation (session-preserving) + read-only sidebar

**Files:**
- Modify: `frontend/src/components/BugsPanel.tsx`, `frontend/src/App.tsx`

- [ ] **Step 1: Session-preserving navigation**

The Bugs view already uses `useViewState` (Task A3), so its selected bug / filters / page are retained when leaving. Keep the existing test-key link calling `onOpenTest(key)` → `setSelectedKey(key); setView("browse")` in `App.tsx`; no change needed beyond A3 for restoration. Confirm returning to Containers→Bugs restores the selected bug.

- [ ] **Step 2: Add an open-detail action + read-only sidebar**

In `BugsPanel.tsx`, add a small "open detail" icon button beside each affected test's key. Store `const [detailKey, setDetailKey] = useViewState<string|null>(profileId, "bugs", "detailKey", null);` and a `detailVersion`. Clicking the icon sets `detailKey`. Mount, as a right sidebar within the Bugs view (mirror DuplicatesView's TestDetail usage), a `TestDetail` with `readOnly`, no `onCloned`, `onClose={() => setDetailKey(null)}`:

```tsx
{detailKey && (
  <TestDetail
    profileId={profileId}
    testKey={detailKey}
    version={detailVersion}
    pendingForTest={[]}
    folders={[]}
    readOnly
    onClose={() => setDetailKey(null)}
    onEdited={() => {}}
  />
)}
```

- [ ] **Step 3: Verify**

Run: `cd frontend; npm run build` (expect pass). `wails dev`: in Bugs, click the test key → Browse opens with the test selected; go back to Bugs → the bug/filters are restored. Click the open-detail icon → a read-only TestDetail opens in a sidebar in the Bugs view with no editable controls.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/BugsPanel.tsx frontend/src/App.tsx
git commit -m "feat(frontend): in-view read-only test detail and session-preserving bug navigation"
```

---

## Final verification

- [ ] `go build ./...` and `go test ./...` pass.
- [ ] `cd frontend; npm run build` passes (tsc).
- [ ] `wails dev` on a demo profile: Run history on a test; execution run columns; plan/set roll-up; bug affected-tests Project column + expandable breakdown; bug test-key navigation restores Bugs state; open-detail read-only sidebar in Bugs.
- [ ] Dispatch a final code-reviewer over the whole branch (per subagent-driven-development), then use `superpowers:finishing-a-development-branch`.

## Self-review notes (plan vs. spec)

- Phase A covers T2 (hook A1, profile-clear A2, adoption A3/A4) — all spec views listed.
- Phase B covers T1: schema (B1), store helpers (B2), jira (B3), sync (B4), repo queries (B5/B6), bindings (B7), UI on test/execution/plan/set (B8/B9). Run date, executed-by, environment, plan, fix versions, defects all flow through.
- Phase C covers T3: project (C1) + expandable breakdown reusing run history (C2).
- Phase D covers T4: readOnly mode (D1) + session-preserving nav and in-view read-only sidebar (D2).
- Type consistency: `TestRun` (jira) → `TestRunRow` (store) → `TestRunEntry` (repo/UI); `RunRollup`, `ExecMemberRun`, `BugTest.Project` used consistently across tasks.

---

# Phase E — Follow-up from new Jira issues (post-review)

After the Phase A-D work, a Jira sweep of component **Xray-test-management** found new requests. Three are already delivered by Phase A-D and need no work: **RND_P_4TFINT_05-238** (tab session persistence -> Phase A), **-239** (read-only side panel from a bug's affected test -> Phase D), **-240** (fix version / tester / test plan on the affected-tests table -> Phase C). Two more are in-domain and added below. Three are unrelated macOS platform bugs, listed under "Out of scope" for a separate effort.

### Task E1: Read-only test detail side panel in the Containers Test Execution view (RND_P_4TFINT_05-245)

Mirror the Phase-D bug sidebar in the Containers Test Execution member list: clicking (or an open-detail icon on) a member test opens the read-only `TestDetail` to the right of the board, instead of only showing run columns.

**Files:**
- Modify: `frontend/src/components/ContainersView.tsx`
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Open-detail action + session state.** When `kind === "testexec"`, add a small open-detail icon button to each member row (beside the test key). Add `const [detailKey, setDetailKey] = useViewState<string | null>(profileId, "containers", "detailKey", null);` and a plain `detailVersion`. Clicking sets `detailKey` and bumps `detailVersion`. (ContainersView already imports `useViewState`.)

- [ ] **Step 2: Mount the read-only sidebar.** Import `TestDetail`. At the end of the container board layout, render `{detailKey && <TestDetail profileId={profileId} testKey={detailKey} version={detailVersion} pendingForTest={[]} folders={[]} jiraUrl={jiraUrl} readOnly onClose={() => setDetailKey(null)} onEdited={() => {}} />}` (omit `onCloned`). Thread `jiraUrl` from `App.tsx` if ContainersView does not already receive it.

- [ ] **Step 3: Right-side layout.** Add a CSS class that, when the detail is open, lays the board container and the `.detail` aside side by side (mirror `.bugs-md-with-detail`: a grid/flex row with the board taking `1fr` and `.detail` sized `auto` on the right). Apply it conditionally on the board wrapper when `detailKey` is set.

- [ ] **Step 4:** `cd frontend && npm run build` passes. Commit `feat(frontend): read-only test detail side panel in the Containers execution view (RND_P_4TFINT_05-245)`.

### Task E2: Add tests to an existing Test Execution (RND_P_4TFINT_05-242)

Today a user can create a NEW Test Execution from selected bugs. This adds the ability to allocate the affected tests (or selected bugs' tests) to an EXISTING Test Execution.

**Files:**
- Modify: `frontend/src/components/BugsPanel.tsx` (the action + a picker modal)
- Possibly Modify: `app.go` / `internal/testrepo` only if no suitable allocate binding exists

- [ ] **Step 1: Confirm the binding.** Check `frontend/src/api.ts` / `app.go` for an existing allocate method (e.g. `AllocateTests(profileID, containerKey, testKeys)` used by the bulk "Allocate" action and the create-execution-from-bugs flow). Reuse it; only add a new binding if none fits.

- [ ] **Step 2: Action + picker.** In `BugsPanel.tsx`, next to the existing "Create Test Execution" (from selected bugs) action, add **"Add tests to execution..."**. It opens a modal listing existing Test Executions (reuse the container picker / `ListContainers(profileID, "testexec")`), with a type-to-filter search. On confirm, call the allocate binding with the chosen execution key and the affected tests of the selected bug(s) (or the checked bugs' tests), then refresh.

- [ ] **Step 3:** `cd frontend && npm run build` passes (and `go test ./...` if a backend method was added). Commit `feat(frontend): add affected tests to an existing test execution (RND_P_4TFINT_05-242)`.

### Out of scope (separate macOS platform effort, NOT in this plan)

These are unrelated platform bugs; they need their own plan and verification on a Mac, not this feature branch:
- **RND_P_4TFINT_05-241** macOS copy/paste and right-click context menu not working in text inputs (Apple Silicon / Tahoe 26.3).
- **RND_P_4TFINT_05-243** macOS connection test fails with TLS `x509: certificate signed by unknown authority`.
- **RND_P_4TFINT_05-244** macOS PAT field missing a show/hide visibility toggle.
