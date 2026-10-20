# XTM Parameterized Test Runs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show dataset iterations nested under one Test Run and support iteration-step result edits only where the backend proves it can.

**Architecture:** Introduce neutral parameterized-run DTOs and separate read/write capabilities. Cache complete iteration trees atomically, load them lazily on row expansion, and keep live Xray unsupported until fixture-backed endpoints exist; Demo2 supplies the complete working path.

**Tech Stack:** Go 1.25, SQLite, Wails v2, React 19, TanStack Query, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-19-xtm-parameterized-dataset-execution-design.md`

## Global Constraints

- One Test member remains one Test Run; iterations are children, never members.
- Nearest dataset scope wins: Execution, Plan, then Test.
- Stable server ids own live changes; row indexes never identify results.
- Live Xray detail/write capabilities stay false until endpoint fixtures and contract tests exist.
- Malformed or partial responses do not replace the last complete cached tree.
- New profile-scoped tables join purge and a schema-version migration.
- Test first, keep App bindings thin, and commit each task with #301.

## Review Focus

- Duplicate combinatorial values do not create duplicate iterations; Task 1 asserts stable de-duplication.
- An expansion above 1,000 iterations fails before allocation; Task 1 tests the limit boundary.
- A late expanded-row response cannot attach to another Execution/Test; Task 4 tests the full query identity.
- A read-capable but write-incapable backend shows statuses without enabled controls; Task 4 covers both capability combinations.
- Replacing one malformed tree leaves the prior complete tree and another Test's tree untouched; Task 2 proves transaction isolation.

---

### Task 1: Neutral iteration model and deterministic expansion

**Files:**
- Create: `xtm/internal/backend/parameterized_run.go`
- Create: `xtm/internal/backend/parameterized_run_test.go`
- Modify: `xtm/internal/backend/backend.go`
- Modify: `xtm/internal/backend/xray/adapter.go`
- Modify: `xtm/internal/backend/kiwi/adapter.go`
- Modify: `xtm/internal/backend/xray/dataset_test.go`

**Interfaces:**
- Produces: `ParameterizedRun`, `RunIteration`, `IterationParameter`, `IterationStep`; `ExpandDataset(dataset Dataset, max int) ([]RunIteration, error)`; capabilities `SupportsParameterizedRunDetail` and `SupportsParameterizedRunWrites`; Backend methods `GetParameterizedRun(ctx, execKey, testKey)`, `SetIterationStatus(ctx, execKey, testKey, iterationID, status)`, and `SetIterationStepStatus(ctx, execKey, testKey, iterationID, stepResultID, status)`.

- [ ] **Step 1: Write failing model and expansion tests**

Cover seed rows times ordered distinct combinatorial values, multiple combinatorial columns, no seed columns, duplicate values, rectangular output, exact 1,000 success and 1,001 refusal.

- [ ] **Step 2: Run tests to prove red**

Run from `xtm`: `go test ./internal/backend/... -run 'Parameterized|ExpandDataset' -count=1`

Expected: FAIL with missing types/functions.

- [ ] **Step 3: Implement the model and backend contract**

Use ordered slices, non-null arrays and explicit `DatasetSourceKind`/`DatasetSourceKey`. Kiwi returns `backend.ErrUnsupported`; live Xray returns it and advertises false until Task 3 has verified transport evidence.

- [ ] **Step 4: Run backend tests**

Run from `xtm`: `go test ./internal/backend/... -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add xtm/internal/backend
git commit -m "feat(xtm): model parameterized test runs (#301)"
```

### Task 2: Atomic iteration cache and local journal

**Files:**
- Modify: `xtm/internal/store/store.go`
- Modify: `xtm/internal/store/store_testrun_test.go`
- Create: `xtm/internal/testrepo/parameterizedrun.go`
- Create: `xtm/internal/testrepo/parameterizedrun_test.go`
- Modify: `xtm/internal/testrepo/purge.go`
- Modify: `xtm/internal/testrepo/purge_test.go`

**Interfaces:**
- Produces: `ReplaceParameterizedRun(profileID string, run backend.ParameterizedRun) error`, `ParameterizedRun(profileID, execKey, testKey string) (backend.ParameterizedRun, bool, error)`, `SetIterationStatus(profileID, execKey, testKey, iterationID, status string) error`, and `SetIterationStepStatus(profileID, execKey, testKey, iterationID, stepResultID, status string) error`.
- Pending entities: `test_run_iteration` and `test_run_iteration_step`, keyed by stable run/iteration and step ids; before/after values are statuses.

- [ ] **Step 1: Write failing migration, repository and purge tests**

Assert full tree round trip, non-null ordering, replacement removes stale children, invalid duplicate ids roll back, another Test remains, status edit coalesces/reverts, missing membership fails, and purge removes every new table.

- [ ] **Step 2: Run focused tests to prove red**

Run from `xtm`: `go test ./internal/store ./internal/testrepo -run 'ParameterizedRun|Iteration|PurgeProfile' -count=1`

Expected: FAIL because schema and repository methods are absent.

- [ ] **Step 3: Add schema version 55 and repository methods**

Create `test_run_iteration(profile_id, exec_key, test_key, iteration_id, rank, status, dataset_source_kind, dataset_source_key, parameters_json)` and `test_run_iteration_step(profile_id, exec_key, test_key, iteration_id, step_result_id, step_definition_id, rank, status, action, data, expected)`. Their primary keys use the profile, Execution, Test and stable result identities. Validate the full input before beginning replacement, then delete/insert in one transaction.

- [ ] **Step 4: Run store and repository packages**

Run from `xtm`: `go test ./internal/store ./internal/testrepo -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add xtm/internal/store xtm/internal/testrepo
git commit -m "feat(xtm): store parameterized run trees (#301)"
```

### Task 3: Demo2 parameterized run service and bindings

**Files:**
- Create: `xtm/internal/jira/parameterized_runs.go`
- Create: `xtm/internal/jira/parameterized_runs_test.go`
- Modify: `xtm/internal/jira/demo.go`
- Modify: `xtm/internal/backend/xray/adapter.go`
- Create: `xtm/app_parameterized_run.go`
- Create: `xtm/app_parameterized_run_test.go`
- Modify: `xtm/frontend/src/api.ts`
- Regenerate: `xtm/frontend/wailsjs/` through `wails generate module`

**Interfaces:**
- Produces: App methods `GetParameterizedRun(profileID, execKey, testKey string, forceRefresh bool)`, `SetIterationStatus(profileID, execKey, testKey, iterationID, status string)`, and `SetIterationStepStatus(profileID, execKey, testKey, iterationID, stepResultID, status string)`.
- Demo2 returns stable ids and ordered iterations for DEMO2-1 in DEMO2-STE-1; live Xray remains `ErrUnsupported` without a captured fixture.

- [ ] **Step 1: Write failing Demo2 and binding tests**

Assert resolved USERNAME/REGION/PRIORITY values, substituted Action/Data/Expected, at least one PASS/FAIL/TODO iteration, explanatory step results, repeatable ids/order, invalid binding inputs, cache reuse, and force refresh.

- [ ] **Step 2: Run tests to prove red**

Run from `xtm`: `go test ./internal/jira . -run 'ParameterizedRun|IterationStep' -count=1`

Expected: FAIL on missing service and Demo2 result.

- [ ] **Step 3: Implement demo generation and thin bindings**

Update DEMO2-1 step text to use all three macros. Use `ExpandDataset` plus deterministic ids and native Demo status roll-up. The App resolves the backend, checks capabilities, delegates, validates, and publishes complete results to the repository.

- [ ] **Step 4: Regenerate Wails bindings**

Run from `xtm`: `wails generate module`

Expected: generated bindings match the Go surface without manual edits.

- [ ] **Step 5: Run Jira and App tests**

Run from `xtm`: `go test ./internal/jira . -count=1`

Expected: PASS.

- [ ] **Step 6: Run the XTM frontend typecheck**

Run from repo root: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add xtm/internal/jira xtm/internal/backend/xray xtm/app_parameterized_run.go xtm/app_parameterized_run_test.go xtm/frontend/src/api.ts xtm/frontend/wailsjs
git commit -m "feat(xtm): expose demo parameterized runs (#301)"
```

### Task 4: Nested Test Execution result UI

**Files:**
- Create: `xtm/frontend/src/queries/parameterizedRuns.ts`
- Create: `xtm/frontend/src/queries/parameterizedRuns.test.tsx`
- Create: `xtm/frontend/src/components/containers/ParameterizedRunRows.tsx`
- Create: `xtm/frontend/src/components/containers/ParameterizedRunRows.test.tsx`
- Create: `xtm/frontend/src/components/containers/MemberTable.test.tsx`
- Create: `xtm/frontend/src/components/containers/parameterized-runs.css`
- Modify: `xtm/frontend/src/components/containers/MemberTable.tsx`
- Modify: `xtm/frontend/src/components/containers/useContainerState.ts`

**Interfaces:**
- Consumes: Task 3 bindings and capability fields.
- Produces: `useParameterizedRun(profileId, execKey, testKey, expanded)` and nested rows controlled by `(execKey, testKey)`.

- [ ] **Step 1: Write failing query and UI tests**

Cover lazy loading, exact query identity, stale response isolation, iteration progress, parameter values, substituted steps, read/write capability states, keyboard expansion, write errors, and narrow horizontal overflow.

- [ ] **Step 2: Run tests to prove red**

Run: `npm test -w xtm/frontend -- parameterizedRuns.test.tsx ParameterizedRunRows.test.tsx MemberTable.test.tsx`

Expected: FAIL because queries and nested rows do not exist.

- [ ] **Step 3: Implement nested rows**

Render each member plus an optional sibling detail `<tr>` inside a keyed Fragment. Disable the overall run selector when iteration statuses own the aggregate. Patch the exact query after a local step-status edit and show unsupported writes as read-only text.

- [ ] **Step 4: Run XTM frontend tests**

Run: `npm test -w xtm/frontend`

Expected: PASS.

- [ ] **Step 5: Run the XTM frontend typecheck**

Run: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add xtm/frontend/src/queries xtm/frontend/src/components/containers
git commit -m "feat(xtm): show nested dataset iterations (#301)"
```

### Task 5: Commit iteration results and document capability behavior

**Files:**
- Create: `xtm/internal/syncer/commit_iteration.go`
- Create: `xtm/internal/syncer/commit_iteration_test.go`
- Modify: `xtm/internal/syncer/commit.go`
- Modify: `docs/user-guide/xtm/09-containers.md`
- Modify: `agents/project/xtm-architecture.md`

**Interfaces:**
- Consumes: pending entity from Task 2 and backend write method from Task 1.
- Produces: commit handling that refuses unsupported live writes without deleting pending data; Demo2 commit succeeds through its adapter.

- [ ] **Step 1: Write failing commit tests**

Assert demo success clears one change, a backend refusal retains it, missing write capability cannot create the change, a remote mismatch conflicts, and one failed iteration does not delete sibling changes.

- [ ] **Step 2: Run syncer tests to prove red**

Run from `xtm`: `go test ./internal/syncer -run Iteration -count=1`

Expected: FAIL because the commit entity is unhandled.

- [ ] **Step 3: Implement the commit handler and documentation**

Route by stable ids, preserve conflicts for retry/discard, and document read-only live behavior. Do not add a live Xray route or capability claim.

- [ ] **Step 4: Run XTM Go tests**

Expected: PASS.

- [ ] **Step 5: Run Go vet**

Run from `xtm`: `go vet ./...`

Expected: PASS.

- [ ] **Step 6: Run XTM frontend tests**

Run from repo root: `npm test -w xtm/frontend`

Expected: PASS.

- [ ] **Step 7: Run the XTM frontend typecheck**

Run from repo root: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 8: Run lint and the ratchet**

Run from repo root: `npm run lint`

Expected: lint report succeeds and the ratchet stays within baseline.

- [ ] **Step 9: Commit**

```bash
git add xtm/internal/syncer docs/user-guide/xtm/09-containers.md agents/project/xtm-architecture.md
git commit -m "feat(xtm): commit supported iteration results (#301)"
```

