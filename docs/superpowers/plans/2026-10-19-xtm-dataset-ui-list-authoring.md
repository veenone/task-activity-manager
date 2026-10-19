# XTM Dataset UI and List Authoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the dataset catalog and editor into XTM's design system and add safe Text and ad-hoc List parameter authoring.

**Architecture:** Keep catalog layout in its existing dataset feature folder. Split parameter configuration from the value grid through a shared Modal, while retaining the existing whole-table journal and Xray metadata round trip.

**Tech Stack:** React 19, TypeScript, Vitest, Testing Library, Go 1.25, Wails v2, Xray Server/DC JSON dataset API.

**Spec:** `docs/superpowers/specs/2026-10-19-xtm-parameterized-dataset-execution-design.md`

## Global Constraints

- Reuse XTM controls and tokens; add no dependency, hex color, or pixel font size.
- Keep files below 400 lines and define every class in an imported stylesheet.
- Use the shared `Modal`, `Menu`, confirmation hooks and focus treatment.
- Preserve parameter `id`, `listId`, `listScope`, `type` and `options` on every save.
- CSV keeps Xray's name/value format; unmatched imported columns become Text.
- Every behavior change starts with a named failing test and one logical commit referencing #301.

## Review Focus

- A long container summary wraps without hiding its navigation or source label; Task 1 pins this at a narrow viewport.
- A predefined List survives an unrelated cell edit byte-for-byte; Task 3 asserts the outbound JSON.
- Removing an option used in several rows reports every affected row; Task 2 covers the complete row list.
- Canceling the parameter Modal leaves the grid unchanged; Task 2 asserts the original table.
- A CSV column matched by name retains List metadata while a new column becomes Text; Task 3 covers both in one import.

---

### Task 1: Catalog detail hierarchy and usage table

**Files:**
- Modify: `xtm/frontend/src/components/datasets/DatasetCatalogDetail.tsx`
- Modify: `xtm/frontend/src/components/datasets/datasets.css`
- Modify: `xtm/frontend/src/components/datasets/DatasetsView.test.tsx`

**Interfaces:**
- Consumes: existing `DatasetCatalogDetail`, `DatasetCatalogDefinition`, `DatasetCatalogUsage`, and `ContainerNavigationTarget` types.
- Produces: semantic definition cards and a `table` named `Containers using this dataset`; callback signatures stay unchanged.

- [ ] **Step 1: Add failing catalog interaction tests**

Add tests named `renders_scope_cards_and_a_compact_usage_table` and `keeps_long_usage_content_reachable_at_narrow_width`. Assert Type, Container, Summary and Dataset headers; linked `PLAN-1`/`EXEC-1`; Override/Inherited default text; `aria-expanded`; and an overflow wrapper instead of clipped content.

- [ ] **Step 2: Run the focused tests and record the red assertion**

Run: `npm test -w xtm/frontend -- DatasetsView.test.tsx`

Expected: FAIL because Used by is still a list and the detail region lacks the new semantics.

- [ ] **Step 3: Implement the detail layout**

Replace `UsageLink` with semantic table rows. Give each definition scope a labeled card, keep parameter badges static, connect the master expand control to a stable detail region, and add only token-based responsive rules in `datasets.css`.

- [ ] **Step 4: Run the focused tests**

Run: `npm test -w xtm/frontend -- DatasetsView.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add xtm/frontend/src/components/datasets
git commit -m "feat(xtm): refine dataset catalog details (#301)"
```

### Task 2: Parameter Modal and editor shell

**Files:**
- Create: `xtm/frontend/src/components/test-detail/DatasetParameterDialog.tsx`
- Create: `xtm/frontend/src/components/test-detail/DatasetParameterDialog.test.tsx`
- Create: `xtm/frontend/src/components/test-detail/dataset.css`
- Modify: `xtm/frontend/src/components/test-detail/DatasetEditor.tsx`
- Modify: `xtm/frontend/src/components/test-detail/DatasetEditor.test.tsx`
- Modify: `xtm/frontend/src/components/test-detail/DatasetSection.tsx`
- Modify: `xtm/frontend/src/App.css`

**Interfaces:**
- Consumes: `DatasetParameter`, shared `Modal`, `Menu`, `useConfirm`.
- Produces: `DatasetParameterDialog({ parameter, usedValues, onSave, onCancel })`, returning one complete `DatasetParameter`; the editor continues to call `SaveTestDataset(profileId, testKey, table)`.

- [ ] **Step 1: Write failing dialog and grid tests**

Cover Add parameter opening the Modal; Text and List selection; ordered unique ad-hoc options; blocking removal of `gold` when rows 1 and 3 use it; cancel preservation; header Menu actions; semantic row numbers; List cells rendered as selects; grouped toolbar/footer actions; a predefined binding shown read-only; and confirmation before converting that binding.

- [ ] **Step 2: Run the editor tests to prove red**

Run: `npm test -w xtm/frontend -- DatasetParameterDialog.test.tsx DatasetEditor.test.tsx`

Expected: FAIL because the dialog and List selects do not exist.

- [ ] **Step 3: Implement the focused components and styles**

Keep table mutation helpers in `DatasetEditor.tsx`; move parameter form validation into `DatasetParameterDialog.tsx`. Import `dataset.css` from `DatasetSection.tsx`, move the old dataset rules out of `App.css`, and keep every new component under 400 lines.

- [ ] **Step 4: Run editor tests**

Run: `npm test -w xtm/frontend -- DatasetParameterDialog.test.tsx DatasetEditor.test.tsx`

Expected: PASS.

- [ ] **Step 5: Run the XTM frontend typecheck**

Run: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add xtm/frontend/src/components/test-detail xtm/frontend/src/App.css
git commit -m "feat(xtm): rebuild the dataset editor layout (#301)"
```

### Task 3: Validate and serialize Text and ad-hoc List parameters

**Files:**
- Modify: `xtm/internal/backend/dto.go`
- Modify: `xtm/internal/jira/dataset.go`
- Modify: `xtm/internal/jira/dataset_write_test.go`
- Modify: `xtm/internal/testrepo/dataset_validate.go`
- Modify: `xtm/internal/testrepo/dataset_test.go`
- Modify: `xtm/internal/testrepo/dataset_csv_test.go`

**Interfaces:**
- Consumes: existing `backend.DatasetParameter` and Xray column-major payload.
- Produces: constants `DatasetParameterText`, `DatasetParameterList`, and `DatasetListScopeAdHoc`; validation accepts a new List only with non-empty unique options and values drawn from them.

- [ ] **Step 1: Write failing Go contracts**

Add tests `TestValidateDatasetAcceptsAdHocList`, `TestValidateDatasetRejectsUnknownListValue`, `TestSetDatasetWritesNewAdHocListInfo`, `TestSetDatasetPreservesPredefinedListBinding`, and `TestMergeDatasetParametersKeepsMatchedListAndDefaultsNewColumnToText`.

- [ ] **Step 2: Run the focused Go tests to prove red**

Run from `xtm`: `go test ./internal/testrepo ./internal/jira -run 'Dataset|AdHocList|PredefinedList' -count=1`

Expected: FAIL on absent type/list validation and the new-list payload.

- [ ] **Step 3: Implement validation and payload constants**

Normalize an empty legacy type to Text. Serialize a new ad-hoc List as `type=LIST`, its ordered `options`, `paramTestListScope=PARAMETER_AD_HOC`, and no positive `listId`; retain every positive id and predefined scope read from Xray.

- [ ] **Step 4: Run the focused Go packages**

Run from `xtm`: `go test ./internal/testrepo ./internal/jira -count=1`

Expected: PASS.

- [ ] **Step 5: Run the dataset editor tests**

Run from repo root: `npm test -w xtm/frontend -- DatasetEditor.test.tsx`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add xtm/internal/backend/dto.go xtm/internal/jira/dataset.go xtm/internal/jira/dataset_write_test.go xtm/internal/testrepo
git commit -m "feat(xtm): author list dataset parameters (#301)"
```

### Task 4: Document and verify dataset authoring

**Files:**
- Modify: `docs/user-guide/xtm/01-browse.md`
- Modify: `docs/user-guide/xtm/13-datasets.md`
- Modify: `agents/project/xtm-architecture.md`

**Interfaces:**
- Consumes: Tasks 1-3.
- Produces: user instructions for Text/List creation and developer call flow for the whole-table journal.

- [ ] **Step 1: Update user and developer documentation**

Document ad-hoc options, predefined-list preservation, CSV limits, catalog scope cards and the compact usage table. Do not claim project-list administration support.

- [ ] **Step 2: Run the XTM frontend tests**

Run: `npm test -w xtm/frontend`

Expected: PASS.

- [ ] **Step 3: Run the XTM frontend typecheck**

Run: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 4: Run the XTM Go tests**

Run from `xtm`: `go test ./internal/... -count=1`

Expected: PASS.

- [ ] **Step 5: Run Go vet**

Run from `xtm`: `go vet ./...`

Expected: PASS.

- [ ] **Step 6: Run lint and the ratchet**

Run: `npm run lint`

Expected: lint report succeeds and the ratchet stays within baseline.

- [ ] **Step 7: Commit**

```bash
git add docs/user-guide/xtm/01-browse.md docs/user-guide/xtm/13-datasets.md agents/project/xtm-architecture.md
git commit -m "docs(xtm): explain dataset parameter types (#301)"
```

