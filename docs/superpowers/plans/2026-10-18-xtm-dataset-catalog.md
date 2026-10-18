# XTM dataset catalog implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a complete, profile-scoped catalog of Xray Test, Test Plan and Test Execution dataset definitions, with explicit sync and exact navigation links.

**Architecture:** A dedicated service enumerates current Jira Tests and container relationships, reads each definition without resolving inheritance, and publishes one generation to catalog-specific SQLite tables. React reads the active generation through paged queries; the existing `test_dataset` editor cache and pending journal remain independent.

**Tech stack:** Go, SQLite, Wails v2, React 19, TypeScript, TanStack Query, Vitest, Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-18-xtm-dataset-catalog-design.md`

## Global constraints

- Issue #300 owns this work; each implementation commit references `#300`.
- A dataset is scoped to a Test, Test Plan + Test, or Test Execution + Test. Do not create a project-level dataset entity.
- The active profile includes project-owned Tests and external Tests referenced by its Plans or Executions.
- One Test group lists all usages. Persist separate definitions only for real overrides.
- Catalog sync is explicit and never joins the normal profile sync.
- A failed or cancelled scan leaves the previous active generation readable.
- Catalog publication never writes `test_dataset` or `pending_change`.
- Test Plan and Test Execution overrides are read-only in this feature.
- Kiwi keeps dataset capability off and returns `backend.ErrUnsupported` at the runtime boundary.
- Use existing dependencies and design tokens. Add no dependency and no user-facing em dash.
- Keep new production files below 400 lines and add every new CSS class to a stylesheet.
- Regenerate Wails bindings; never edit generated bindings by hand.
- Follow P2: record an assertion failure against pre-change behavior before each implementation slice.
- Execute in an isolated worktree created with `superpowers:using-git-worktrees`; do not move, stash or include the current worktree's unrelated changes.

## Review focus

- One Execution linked to several Plans with different overrides: list every proven definition and link, but label none as the effective source.
- A Plan export that returns inherited content or ignores `resolved=false`: do not publish it as an override; the live transport evidence and adapter test must pin the response.
- Cancellation or a profile switch after some probes finish: publish no rows under either profile.
- Duplicate container links and a large relationship graph: de-duplicate probes, bound concurrency, and page Test groups without splitting one group.
- A Test default edited locally while catalog sync runs: keep the pending edit and present the catalog as the last Jira snapshot.

---

### Task 1: Unresolved Test Plan dataset transport

**Files:**
- Create: `docs/superpowers/plans/assets/2026-10-18-xray-plan-dataset-probe.md`
- Create: `xtm/internal/jira/dataset_plan_test.go`
- Modify: `xtm/internal/jira/dataset.go`
- Modify: `xtm/internal/backend/backend.go`
- Modify: `xtm/internal/backend/xray/adapter.go`
- Modify: `xtm/internal/backend/xray/dataset_test.go`

**Interfaces:**
- Produces: `backend.PlanDatasetReader.GetPlanDataset(ctx context.Context, planKey, testKey string) (backend.Dataset, error)`.
- Produces: `jira.Client.GetPlanDataset(ctx context.Context, planKey, testKey string) (jira.Dataset, error)`.
- Uses: `GET /rest/raven/2.0/api/dataset/export?contextIssueKey=...&resolved=false&testIssueKey=...` and `ParseDatasetCSV`.

- [ ] **Step 1: Capture the live Plan-context response**

Use a sandbox Test and Test Plan to record method, encoded query, status, content type and sanitized body for one real override and one inherited membership. Save the evidence in the asset file. Confirm specifically whether `resolved=false` returns an empty body or a dataset-not-found response when no Plan override exists.

- [ ] **Step 2: Write the failing Jira client tests**

Add `TestGetPlanDatasetRequestsOnlyThePlanDefinition`, `TestGetPlanDatasetReturnsEmptyForAnInheritedMembership`, `TestGetPlanDatasetDoesNotHideAMissingRoute`, and `TestGetPlanDatasetPadsShortCSVRows`. Assert the exact query values, the parsed table, absence behavior, route-error behavior, and rectangular rows.

- [ ] **Step 3: Run the Jira tests and preserve the red result**

Run from `xtm`:

```powershell
go test ./internal/jira -run 'TestGetPlanDataset' -count=1
```

Expected: FAIL because `GetPlanDataset` does not exist.

- [ ] **Step 4: Implement the client and optional backend interface**

Add `PlanDatasetReader` beside the dataset methods in `backend.go`. Implement the Jira call in `dataset.go`, including `resolved=false`, URL encoding, CSV parsing, the observed absence response, and a demo branch returning an empty dataset. Implement the optional interface on `xray.Adapter` through `toBackendDataset`. Do not add the method to the fat `backend.Backend` interface.

- [ ] **Step 5: Test the adapter mapping**

Extend `backend/xray/dataset_test.go` with `TestAdapterGetPlanDatasetMapsCSVMetadataAsIncomplete`: assert names, values and combinatorial flags survive, while list-only metadata remains empty instead of being guessed as `TEXT`.

- [ ] **Step 6: Run the focused Go tests**

Run from `xtm`:

```powershell
go test ./internal/jira ./internal/backend/xray -run 'Dataset|PlanDataset' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the transport**

```powershell
git add docs/superpowers/plans/assets/2026-10-18-xray-plan-dataset-probe.md xtm/internal/jira/dataset.go xtm/internal/jira/dataset_plan_test.go xtm/internal/backend/backend.go xtm/internal/backend/xray/adapter.go xtm/internal/backend/xray/dataset_test.go
git commit -m "feat(xtm): read plan dataset overrides (#300)"
```

### Task 2: Atomic catalog repository

**Files:**
- Create: `xtm/internal/testrepo/dataset_catalog.go`
- Create: `xtm/internal/testrepo/dataset_catalog_test.go`
- Create: `xtm/internal/store/migration_dataset_catalog_test.go`
- Modify: `xtm/internal/store/store.go`
- Modify: `xtm/internal/testrepo/purge.go`
- Modify: `xtm/internal/testrepo/purge_test.go`

**Interfaces:**
- Produces: scope constants `DatasetScopeTest`, `DatasetScopePlan`, `DatasetScopeExecution`.
- Produces: `DatasetCatalogSnapshot`, `DatasetCatalogTest`, `DatasetCatalogTestSummary`, `DatasetCatalogDefinition`, `DatasetCatalogUsage`, `DatasetCatalogExecPlan`, `DatasetCatalogState`, `DatasetCatalogQuery`, `DatasetCatalogPage`, and `DatasetCatalogDetail`. `DatasetCatalogTestSummary.InternalOpenable` is true only when the Test has a safe local Browse destination.
- Produces: `Repository.PublishDatasetCatalog(profileID string, snapshot DatasetCatalogSnapshot, syncedAt time.Time) error`.
- Produces: `Repository.ListDatasetCatalog(profileID string, q DatasetCatalogQuery) (DatasetCatalogPage, error)`.
- Produces: `Repository.GetDatasetCatalogDetail(profileID, testKey string) (DatasetCatalogDetail, bool, error)`.

- [ ] **Step 1: Write the failing migration and purge tests**

Raise the asserted schema floor to 54. In `migration_dataset_catalog_test.go`, open a database stamped at 53 and assert all five catalog tables accept a profile row. Extend the purge tests so `dataset_catalog_state`, `dataset_catalog_test`, `dataset_catalog_definition`, `dataset_catalog_usage`, and `dataset_catalog_exec_plan` are discovered and emptied.

- [ ] **Step 2: Run the schema tests and preserve the red result**

Run from `xtm`:

```powershell
go test ./internal/store ./internal/testrepo -run 'DatasetCatalog|PurgeProfile' -count=1
```

Expected: FAIL because schema 54 and the catalog tables do not exist.

- [ ] **Step 3: Add schema 54**

Add the five tables from the spec to `baseSchema`, add an idempotent v54 creation block to `applyMigrations`, add lookup indexes to `indexSchema`, and add every table to `profileScopedTables`. Store parameter and row arrays as JSON; include `generation` in every snapshot-table primary key.

- [ ] **Step 4: Write failing repository behavior tests**

Add tests named `TestPublishDatasetCatalogListsOneTestWithManyUsages`, `TestPublishDatasetCatalogKeepsAnOverrideOnlyTest`, `TestPublishDatasetCatalogDeduplicatesLinks`, `TestPublishDatasetCatalogRejectsMalformedDefinitions`, `TestPublishDatasetCatalogFailureKeepsActiveGeneration`, `TestListDatasetCatalogFiltersAndPagesWholeTestGroups`, `TestDatasetCatalogListsAmbiguousPlanLinksWithoutAnEffectiveClaim`, and `TestPublishDatasetCatalogLeavesEditorAndJournalUntouched`.

- [ ] **Step 5: Run the repository tests and preserve the red result**

Run from `xtm`:

```powershell
go test ./internal/testrepo -run 'DatasetCatalog' -count=1
```

Expected: FAIL because the catalog types and methods do not exist.

- [ ] **Step 6: Implement generation publication and queries**

In one transaction, choose the next generation, validate keys, kinds, JSON shape and row width, insert the snapshot, update `dataset_catalog_state.active_generation`, then remove older generations. `ListDatasetCatalog` validates `scope`, `ownership`, `offset` and `limit`, searches Test and container keys/summaries, and pages after grouping by Test. `GetDatasetCatalogDetail` reads only the active generation and returns all definitions, usages and Execution-to-Plan links for the Test.

- [ ] **Step 7: Run repository and instruction gates**

Run from `xtm`, then from the repository root:

```powershell
go test ./internal/store ./internal/testrepo -count=1
npm test --workspace @agile-suite/core -- --run src/instruction-gate.test.ts
```

Expected: PASS, including the profile-table gate.

- [ ] **Step 8: Commit the repository slice**

```powershell
git add xtm/internal/store/store.go xtm/internal/store/migration_dataset_catalog_test.go xtm/internal/testrepo/dataset_catalog.go xtm/internal/testrepo/dataset_catalog_test.go xtm/internal/testrepo/purge.go xtm/internal/testrepo/purge_test.go
git commit -m "feat(xtm): store dataset catalog snapshots (#300)"
```

### Task 3: Complete catalog discovery service

**Files:**
- Create: `xtm/internal/datasetcatalog/service.go`
- Create: `xtm/internal/datasetcatalog/service_test.go`

**Interfaces:**
- Consumes: Task 1's `backend.PlanDatasetReader` and Task 2's snapshot types and `PublishDatasetCatalog`.
- Produces: `datasetcatalog.Progress { Stage string; Fetched int; Total int }`.
- Produces: `datasetcatalog.New(b DiscoveryBackend, repo *testrepo.Repository) *Service`.
- Produces: `datasetcatalog.Service.Sync(ctx context.Context, profileID, projectKey string, onProgress func(Progress)) error`.
- Internal backend seam: `SearchTestsPage`, `ListTestsBasic`, `ListContainers`, `ExecPlans`, `GetDataset`, `GetRunDataset`, plus the optional `PlanDatasetReader` assertion.

- [ ] **Step 1: Write the failing enumeration tests**

With a deterministic fake backend, add `TestSyncCatalogEnumeratesOwnedAndReferencedTests`, `TestSyncCatalogProbesEachDefinitionOnce`, `TestSyncCatalogRecordsExecutionPlanLinks`, and `TestSyncCatalogPublishesOneTestGroupForInheritedUsages`. Assert the exact Test keys, memberships, probe calls and published snapshot.

- [ ] **Step 2: Run the service tests and preserve the red result**

Run from `xtm`:

```powershell
go test ./internal/datasetcatalog -count=1
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement enumeration and bounded probes**

Page `SearchTestsPage` with an empty incremental watermark, read Plans and Executions from `ListContainers`, union linked external keys, hydrate them with `ListTestsBasic`, and call `ExecPlans` once per Execution. Build unique jobs for every Test default and Plan/Execution membership. Run at most eight probe jobs concurrently, stop scheduling on cancellation, collect results in memory, and call `PublishDatasetCatalog` only after every job succeeds.

- [ ] **Step 4: Write the failing recovery tests**

Add `TestSyncCatalogFailureDoesNotPublish`, `TestSyncCatalogCancellationDoesNotPublish`, `TestSyncCatalogRejectsMissingPlanReader`, `TestSyncCatalogDoesNotTouchPendingDatasetEdits`, and `TestSyncCatalogProgressCountsUniqueProbes`. Make the cancellation test release already-running probes after cancelling so it covers the late-result race.

- [ ] **Step 5: Implement failure, cancellation and progress behavior**

Check `ctx.Err()` before enumeration, before scheduling, after workers finish, and immediately before publication. Treat a missing `PlanDatasetReader` as `backend.ErrUnsupported`. Emit discovery stages first, then a probe total equal to unique Test and relationship jobs. Return the first contextualized error without publishing.

- [ ] **Step 6: Run the package tests with the race detector**

Run from `xtm`:

```powershell
go test -race ./internal/datasetcatalog -count=1
```

Expected: PASS with no race report.

- [ ] **Step 7: Commit the service**

```powershell
git add xtm/internal/datasetcatalog
git commit -m "feat(xtm): discover complete dataset catalogs (#300)"
```

### Task 4: Wails bindings and frontend queries

**Files:**
- Create: `xtm/app_dataset_catalog.go`
- Create: `xtm/app_dataset_catalog_test.go`
- Create: `xtm/frontend/src/queries/datasets.ts`
- Create: `xtm/frontend/src/queries/datasets.test.tsx`
- Modify: `xtm/frontend/src/queries/keys.ts`
- Modify: `xtm/frontend/src/queries/invalidate.ts`
- Modify: `xtm/frontend/src/queries/invalidate.test.ts`
- Modify: `xtm/frontend/src/api.ts`
- Regenerate: `xtm/frontend/wailsjs/go/main/App.d.ts`
- Regenerate: `xtm/frontend/wailsjs/go/main/App.js`
- Regenerate: `xtm/frontend/wailsjs/go/models.ts`

**Interfaces:**
- Consumes: Task 3's `Service.Sync` and Task 2's repository queries.
- Produces: `App.SyncDatasetCatalog(profileID string) error`.
- Produces: `App.ListDatasetCatalog(profileID string, q testrepo.DatasetCatalogQuery) (testrepo.DatasetCatalogPage, error)`.
- Produces: `App.GetDatasetCatalogDetail(profileID, testKey string) (testrepo.DatasetCatalogDetail, error)`.
- Produces: `keys.datasets(profileID)`, `keys.datasetCatalog(profileID, query)`, and `keys.datasetDetail(profileID, testKey)`.
- Produces: `useDatasetCatalog(profileID, query)` and `useDatasetCatalogDetail(profileID, testKey, enabled)`.

- [ ] **Step 1: Write failing binding tests**

Add `TestListDatasetCatalogRejectsUnknownFilters`, `TestGetDatasetCatalogDetailRejectsEmptyKey`, and `TestSyncDatasetCatalogEmitsProgressAndPublishesDemoSnapshot`. Assert the returned errors, progress payload values and stored state.

- [ ] **Step 2: Run the binding tests and preserve the red result**

Run from `xtm`:

```powershell
go test . -run 'DatasetCatalog' -count=1
```

Expected: FAIL because the App methods do not exist.

- [ ] **Step 3: Implement the App boundary**

Keep `app_dataset_catalog.go` thin: require the store, load the profile and credentials, build the backend, construct the service, and map `datasetcatalog.Progress` to the existing `sync:progress` event. Validate query values and Test keys before delegating. Emit terminal `Done` on every exit, matching `runPartialSync`.

- [ ] **Step 4: Regenerate and expose the Wails API**

Run from `xtm`:

```powershell
wails generate module
```

Export the three methods from `frontend/src/api.ts` and add plain TypeScript interfaces matching the Go JSON shapes: nullable sync state, paged Test groups, definitions, usages and Execution-to-Plan links.

- [ ] **Step 5: Write failing query-hook tests**

In `datasets.test.tsx`, assert disabled queries make no call, query objects appear in the cache key, detail reads wait for an expanded Test, and invalidating the dataset family refreshes both list and detail.

- [ ] **Step 6: Implement the query keys and hooks**

Place every dataset query below `[profileID, "datasets"]`. Use `placeholderData` for catalog pages so the last successful snapshot stays visible during refetch. Add that prefix to `invalidateProfileData`.

- [ ] **Step 7: Run the boundary and query tests**

Run from `xtm`, then from the repository root:

```powershell
go test . -run 'DatasetCatalog' -count=1
npm test --workspace xtm/frontend -- --run src/queries/datasets.test.tsx src/queries/invalidate.test.ts
npm run typecheck --workspace xtm/frontend
```

Expected: PASS.

- [ ] **Step 8: Commit the API slice**

```powershell
git add xtm/app_dataset_catalog.go xtm/app_dataset_catalog_test.go xtm/frontend/wailsjs/go/main/App.d.ts xtm/frontend/wailsjs/go/main/App.js xtm/frontend/wailsjs/go/models.ts xtm/frontend/src/api.ts xtm/frontend/src/queries/keys.ts xtm/frontend/src/queries/invalidate.ts xtm/frontend/src/queries/invalidate.test.ts xtm/frontend/src/queries/datasets.ts xtm/frontend/src/queries/datasets.test.tsx
git commit -m "feat(xtm): expose dataset catalog queries (#300)"
```

### Task 5: Datasets inventory view

**Files:**
- Create: `xtm/frontend/src/components/datasets/DatasetsView.tsx`
- Create: `xtm/frontend/src/components/datasets/DatasetCatalogToolbar.tsx`
- Create: `xtm/frontend/src/components/datasets/DatasetCatalogTable.tsx`
- Create: `xtm/frontend/src/components/datasets/DatasetCatalogDetail.tsx`
- Create: `xtm/frontend/src/components/datasets/useDatasetCatalogState.ts`
- Create: `xtm/frontend/src/components/datasets/datasets.css`
- Create: `xtm/frontend/src/components/datasets/DatasetsView.test.tsx`
- Modify: `xtm/frontend/src/contexts/NavContext.tsx`
- Modify: `xtm/frontend/src/views.ts`
- Modify: `xtm/frontend/src/views.test.ts`
- Modify: `xtm/frontend/src/shell/useAppState.ts`
- Modify: `xtm/frontend/src/shell/MainView.tsx`
- Modify: `xtm/main.go`

**Interfaces:**
- Consumes: Task 4's hooks and DTOs.
- Produces: `DatasetsView({ onOpenTest, onOpenContainer, jiraUrl })`.
- Produces: `View` id `datasets`, shown when `supportsDatasets` is true.
- Produces: a read-only Test-group list and definition preview; it exposes callbacks but owns no cross-view navigation state.

- [ ] **Step 1: Write failing registry and state tests**

Extend `views.test.ts` to assert Datasets appears in `VIEWS`, disappears when `supportsDatasets` is false, and keeps a distinct tour id. Add component tests for never-synced, syncing with the previous page retained, successful empty, failed-with-previous-snapshot, and successful populated states.

- [ ] **Step 2: Run the view tests and preserve the red result**

Run from the repository root:

```powershell
npm test --workspace xtm/frontend -- --run src/views.test.ts src/components/datasets/DatasetsView.test.tsx
```

Expected: FAIL because the view and `datasets` id do not exist.

- [ ] **Step 3: Add the capability-gated view shell**

Add `supportsDatasets` to `ViewContext`, add Datasets after Containers in `VIEWS` and `menuViews`, and render it from `MainView`. Thread the active Jira URL and navigation callbacks as props. Keep the native-menu list and frontend list in the same order so the instruction gate passes.

- [ ] **Step 4: Implement filters, paging and explicit sync**

Persist search, scope, ownership, page and page size with `useViewState(profileID, "datasets", ...)`. The toolbar calls `SyncDatasetCatalog`, uses the shared `SyncContext` lock and progress, invalidates `keys.datasets(profileID)` on success, and renders the last successful time from the page state. Reset page to zero when a filter changes.

- [ ] **Step 5: Implement the accessible Test-group table**

Render one row per Test group. The key cell contains a real expand button with `aria-expanded` and `aria-controls`. Expanded content loads `useDatasetCatalogDetail`, groups default and override definitions, lists every Plan and Execution usage once, marks external Tests, labels incomplete Plan metadata as unknown, and never calls an inherited usage an override. For multiple linked Plan definitions, show all links without an effective-source badge.

- [ ] **Step 6: Add token-based styling and interaction tests**

Keep view-specific rules in `datasets.css`; import it from `DatasetsView.tsx`. Test search and filter values, paging by Test group, expansion, keyboard activation, external and override-only labels, unknown Plan metadata, sync failure retaining prior rows, and exact callback arguments for Test, Plan, Execution and Jira actions.

- [ ] **Step 7: Run frontend tests, typecheck and UI gates**

Run from the repository root:

```powershell
npm test --workspace xtm/frontend -- --run src/views.test.ts src/components/datasets/DatasetsView.test.tsx
npm run typecheck --workspace xtm/frontend
npm test --workspace @agile-suite/core -- --run src/instruction-gate.test.ts
```

Expected: PASS without increasing accessibility, class-name, color or font-size ratchets.

- [ ] **Step 8: Commit the view**

```powershell
git add xtm/main.go xtm/frontend/src/contexts/NavContext.tsx xtm/frontend/src/views.ts xtm/frontend/src/views.test.ts xtm/frontend/src/shell/useAppState.ts xtm/frontend/src/shell/MainView.tsx xtm/frontend/src/components/datasets
git commit -m "feat(xtm): add datasets inventory view (#300)"
```

### Task 6: Exact Test and container navigation

**Files:**
- Modify: `xtm/frontend/src/contexts/NavContext.tsx`
- Modify: `xtm/frontend/src/contexts/NavContext.test.tsx`
- Modify: `xtm/frontend/src/shell/MainView.tsx`
- Modify: `xtm/frontend/src/components/ContainersView.tsx`
- Modify: `xtm/frontend/src/components/ContainersView.test.tsx`
- Modify: `xtm/frontend/src/components/datasets/DatasetsView.test.tsx`

**Interfaces:**
- Produces: `ContainerNavigationTarget { kind: "testplan" | "testexec"; key: string }` in `NavContext`.
- Produces: `openContainer(target: ContainerNavigationTarget): void` and `consumeContainerTarget(): void`.
- Consumes: Task 5's `onOpenTest` and `onOpenContainer` callbacks.

- [ ] **Step 1: Write failing navigation tests**

Test that Open Test selects the exact owned Test and switches to Browse; an external Test without a safe local destination exposes only Jira navigation. Test that Open Container switches to Containers, sets mode to containers, changes kind, selects the exact key after its list loads, consumes the target once, and survives a different previously persisted kind.

- [ ] **Step 2: Run the navigation tests and preserve the red result**

Run from the repository root:

```powershell
npm test --workspace xtm/frontend -- --run src/contexts/NavContext.test.tsx src/components/ContainersView.test.tsx src/components/datasets/DatasetsView.test.tsx
```

Expected: FAIL because the navigation intent does not exist.

- [ ] **Step 3: Implement the navigation intent**

Add the nullable target and two actions to `NavContext`. `MainView` sets the target and opens `plans`. `ContainersView` consumes a target by selecting `mode="containers"`, the supplied kind and key through its existing `useViewState` setters. Clear the intent only after applying it; do not clear the selected container when the matching query result arrives.

- [ ] **Step 4: Guard internal Test navigation**

Carry an `internalOpenable` flag in catalog Test summaries. Owned Tests open Browse; external Tests only do so when the sync hydrated enough local detail to satisfy the existing Test detail path. Otherwise render the Jira action without a disabled or broken Open Test control.

- [ ] **Step 5: Run the navigation and shell tests**

Run from the repository root:

```powershell
npm test --workspace xtm/frontend -- --run src/contexts/NavContext.test.tsx src/components/ContainersView.test.tsx src/components/datasets/DatasetsView.test.tsx src/App.test.tsx
npm run typecheck --workspace xtm/frontend
```

Expected: PASS.

- [ ] **Step 6: Commit navigation**

```powershell
git add xtm/frontend/src/contexts/NavContext.tsx xtm/frontend/src/contexts/NavContext.test.tsx xtm/frontend/src/shell/MainView.tsx xtm/frontend/src/components/ContainersView.tsx xtm/frontend/src/components/ContainersView.test.tsx xtm/frontend/src/components/datasets/DatasetsView.test.tsx
git commit -m "feat(xtm): link dataset catalog targets (#300)"
```

### Task 7: Documentation and full verification

**Files:**
- Create: `docs/user-guide/xtm/13-datasets.md`
- Modify: `docs/user-guide/xtm/README.md`
- Modify: `docs/user-guide/xtm/00-getting-started.md`
- Modify: `docs/user-guide/SCREENSHOTS.md`
- Modify: `docs/superpowers/specs/2026-10-18-xtm-dataset-catalog-design.md`
- Modify when Task 1 adds a durable platform fact: `agents/project/domain-context.md`

**Interfaces:**
- Documents: the shipped App bindings, `datasetcatalog.Service.Sync`, repository generation publication, query hooks and container navigation intent.
- Documents: selected-profile scope, external Tests, explicit sync, inheritance, stale-snapshot recovery, filters and links.

- [ ] **Step 1: Write the user guide**

Add Datasets to the guide index and View menu description. Explain what one Test group represents, what an override is, why sync is explicit, how the last successful snapshot behaves, how external Tests are marked, and where each link opens. Add one screenshot checklist entry without inventing an image file.

- [ ] **Step 2: Update the developer call flow in the spec**

Add an implementation map using the exact shipped symbols. Trace `DatasetsView -> SyncDatasetCatalog -> datasetcatalog.Service.Sync -> Repository.PublishDatasetCatalog`, and the list/detail plus container-navigation paths. Cite rule IDs instead of copying process rules.

- [ ] **Step 3: Record the verified Plan response**

If Task 1 established a new or corrected Xray fact, add it to the dataset section of `domain-context.md` and cite the existing Task 1 commit hash. Run the instruction gate to verify that the cited commit exists.

- [ ] **Step 4: Run the full gates**

From Git Bash at the repository root, with Git's `usr/bin` on `PATH`:

```bash
make gates
```

Expected: all Go tests, frontend tests, typechecks, vet, instruction gate and ratchets pass. Read and fix any failure before rerunning its direct command.

- [ ] **Step 5: Run the focused race and production-build checks**

Run from `xtm`, then from the repository root:

```powershell
go test -race ./internal/datasetcatalog -count=1
npm run build --workspace xtm/frontend
```

Expected: PASS.

- [ ] **Step 6: Commit the documentation**

Run the humanizer skill in file mode over the changed prose, then commit:

```powershell
git add docs/user-guide/xtm docs/user-guide/SCREENSHOTS.md docs/superpowers/specs/2026-10-18-xtm-dataset-catalog-design.md agents/project/domain-context.md
git commit -m "docs(xtm): explain dataset catalog sync (#300)"
```

- [ ] **Step 7: Prepare the PR without merging**

Open an issue-linked PR whose body links the spec and quotes every acceptance line from #300. Attach the Task 1 live evidence and the gate output. Do not use a closing keyword until the user confirms the behavior, and do not merge without approval.
