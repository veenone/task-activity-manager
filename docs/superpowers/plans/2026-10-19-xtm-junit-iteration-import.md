# XTM JUnit Iteration Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Map JUnit results into the correct dataset iteration with an explicit preview whenever XML properties cannot prove the target.

**Architecture:** Extend the parser with testcase-level properties and make import targets discriminated as Test Run or iteration. Analyze against the cached parameterized-run tree, require manual mapping for incomplete/ambiguous cases, and revalidate the exact signature at Apply time.

**Tech Stack:** Go 1.25 encoding/xml, SQLite, React 19, TypeScript, Vitest, Wails v2.

**Spec:** `docs/superpowers/specs/2026-10-19-xtm-parameterized-dataset-execution-design.md`

## Global Constraints

- Requires `2026-10-19-xtm-parameterized-test-runs.md` Tasks 1-3.
- Property names match dataset parameter names exactly after XML whitespace trimming.
- Every resolved parameter must be present; unrelated properties are ignored.
- Never infer an iteration from report order or a partial signature.
- Apply revalidates membership, dataset source and stable iteration identity.
- Non-parameterized and skipped-case behavior stays backward compatible.
- Test first and commit each task with #301.

## Review Focus

- Duplicate XML properties with the same name are ambiguous, not last-wins; Task 1 tests this.
- XML entity decoding and Unicode values compare as decoded strings; Task 1 covers both.
- Two included rows cannot target one iteration even if their results agree; Task 2 rejects the duplicate.
- A dataset changed after preview cannot receive stale results; Task 2 revalidates its source/signature.
- Creating a new Execution cannot use temporary row order as identity; Task 4 stores a signature and resolves after creation.

---

### Task 1: Parse and match complete parameter signatures

**Files:**
- Modify: `xtm/internal/testrepo/junitimport.go`
- Modify: `xtm/internal/testrepo/junitimport_test.go`

**Interfaces:**
- Produces: `JUnitProperty{Name, Value string}`, retained on `junitTestCase`; `JUnitTargetKind` constants `run` and `iteration`; `JUnitMatch` gains `TargetKind`, `IterationID`, and `Parameters []backend.IterationParameter`.
- Produces helper `matchIteration(properties []JUnitProperty, run backend.ParameterizedRun) (iterationID string, signature []backend.IterationParameter, reason string)`.

- [ ] **Step 1: Write failing parser and matcher tests**

Cover complete exact match, partial signature, unknown value, duplicate property, duplicate iteration signature, unrelated property, whitespace trimming, XML entity decoding, Unicode, skipped case and non-parameterized compatibility.

- [ ] **Step 2: Run focused tests to prove red**

Run from `xtm`: `go test ./internal/testrepo -run JUnit -count=1`

Expected: FAIL because properties and iteration targets are absent.

- [ ] **Step 3: Implement parsing and exact matching**

Preserve property order for display, but compare a complete name/value map against each iteration. Return a reason instead of selecting when zero or multiple matches exist.

- [ ] **Step 4: Run JUnit tests**

Run from `xtm`: `go test ./internal/testrepo -run JUnit -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add xtm/internal/testrepo/junitimport.go xtm/internal/testrepo/junitimport_test.go
git commit -m "feat(xtm): match junit dataset signatures (#301)"
```

### Task 2: Model unresolved mappings and revalidate Apply

**Files:**
- Modify: `xtm/internal/testrepo/junitimport.go`
- Modify: `xtm/internal/testrepo/junitimport_test.go`
- Modify: `xtm/app.go`
- Modify: `xtm/frontend/src/api.ts`
- Regenerate: `xtm/frontend/wailsjs/` through `wails generate module`

**Interfaces:**
- Produces: `JUnitMapping` rows with `Included`, `Reason`, `CandidateIterations`, and optional resolved `JUnitMatch`; preview returns `Mappings` instead of treating unresolved parameterized cases as ordinary skips.
- `ApplyJUnitImport(profileID, execKey string, matches []JUnitMatch)` accepts only resolved included targets.

- [ ] **Step 1: Write failing analyzer and Apply tests**

Assert an incomplete signature becomes Needs mapping; explicit iteration choice succeeds; excluded rows do nothing; duplicate targets fail; changed dataset/source fails; removed membership fails; unsupported iteration writes never fall back to overall status.

- [ ] **Step 2: Run repository and App tests to prove red**

Run from `xtm`: `go test ./internal/testrepo . -run JUnit -count=1`

Expected: FAIL on missing mapping state and revalidation.

- [ ] **Step 3: Implement discriminated preview and Apply validation**

Load the cached parameterized run for matched members. At Apply, compare stable iteration id plus the complete name/value signature and dataset source before calling `SetIterationStatus`.

- [ ] **Step 4: Regenerate Wails bindings**

Run from `xtm`: `wails generate module`

Expected: generated bindings match the Go surface without manual edits.

- [ ] **Step 5: Run repository and App JUnit tests**

Run from `xtm`: `go test ./internal/testrepo . -run JUnit -count=1`

Expected: PASS.

- [ ] **Step 6: Run the XTM frontend typecheck**

Run from repo root: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add xtm/internal/testrepo/junitimport* xtm/app.go xtm/frontend/src/api.ts xtm/frontend/wailsjs
git commit -m "feat(xtm): validate junit iteration targets (#301)"
```

### Task 3: Manual mapping preview

**Files:**
- Modify: `xtm/frontend/src/components/JUnitImportModal.tsx`
- Create: `xtm/frontend/src/components/JUnitImportModal.test.tsx`
- Modify: the stylesheet already owning JUnit modal classes.

**Interfaces:**
- Consumes: `JUnitImportPreview.mappings` and candidate iteration signatures.
- Produces: one editable preview row per testcase; included rows must resolve to a unique target before Apply enables.

- [ ] **Step 1: Write failing preview tests**

Cover automatic mapping display, accessible manual select with full signatures, exclusion, duplicate-target error, disabled Apply, current/new statuses, long-value disclosure, and successful mixed run/iteration submission.

- [ ] **Step 2: Run tests to prove red**

Run: `npm test -w xtm/frontend -- JUnitImportModal.test.tsx`

Expected: FAIL because the current preview has only Matched and Skipped tables.

- [ ] **Step 3: Implement grouped mapping rows**

Remove inline styles while touching the modal and replace them with token-based classes. Group by Test, keep unresolved rows visible, and build the final `JUnitMatch[]` only from included resolved rows.

- [ ] **Step 4: Run frontend tests**

Run: `npm test -w xtm/frontend -- JUnitImportModal.test.tsx`

Expected: PASS.

- [ ] **Step 5: Run the XTM frontend typecheck**

Run: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add xtm/frontend/src/components/JUnitImportModal*
git commit -m "feat(xtm): map junit results to iterations (#301)"
```

### Task 4: New-Execution imports and documentation

**Files:**
- Modify: `xtm/internal/testrepo/junitimport.go`
- Modify: `xtm/internal/testrepo/junitimport_test.go`
- Modify: `xtm/frontend/src/components/JUnitNewExecModal.tsx`
- Create: `xtm/frontend/src/components/JUnitNewExecModal.test.tsx`
- Modify: `docs/user-guide/xtm/09-containers.md`
- Modify: `agents/project/xtm-architecture.md`

**Interfaces:**
- Consumes: complete parameter signatures from Tasks 1-2.
- Produces: pending entity `test_run_iteration_import`, keyed by temporary Execution/Test plus a stable signature hash. Its `after_val` is a named `DeferredIterationResult{TestKey, Parameters, Result, DatasetSourceKind, DatasetSourceKey}` JSON value; commit resolves stable ids only after creating the Execution and its Test Run.

- [ ] **Step 1: Write failing new-Execution tests**

Assert existing parameterized Tests retain signatures, new Tests use overall results, temporary row order is ignored, post-create resolution targets the matching stable iteration, and a changed default dataset conflicts instead of misapplying.

- [ ] **Step 2: Run focused Go tests to prove red**

Run from `xtm`: `go test ./internal/testrepo ./internal/syncer -run 'JUnit.*NewExec|NewExec.*Iteration' -count=1`

Expected: FAIL on absent deferred signature resolution.

- [ ] **Step 3: Run the focused frontend test to prove red**

Run from repo root: `npm test -w xtm/frontend -- JUnitNewExecModal.test.tsx`

Expected: FAIL because the new-Execution preview does not expose iteration mappings.

- [ ] **Step 4: Implement deferred resolution and update docs**

Store the complete signature with the pending create workflow. After the real Execution and Test Run exist, fetch the parameterized run, resolve exactly once, then write. Retain the pending row on unsupported transport, ambiguity or conflict.

- [ ] **Step 5: Run full XTM Go tests**

Expected: PASS.

- [ ] **Step 6: Run Go vet**

Run from `xtm`: `go vet ./...`

Expected: PASS.

- [ ] **Step 7: Run XTM frontend tests**

Run from repo root: `npm test -w xtm/frontend`

Expected: PASS.

- [ ] **Step 8: Run the XTM frontend typecheck**

Run from repo root: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 9: Run lint and the ratchet**

Run from repo root: `npm run lint`

Expected: lint report succeeds and the ratchet stays within baseline.

- [ ] **Step 10: Commit**

```bash
git add xtm/internal/testrepo xtm/internal/syncer xtm/frontend/src/components/JUnitNewExecModal* docs/user-guide/xtm/09-containers.md agents/project/xtm-architecture.md
git commit -m "feat(xtm): import junit results into new run iterations (#301)"
```

