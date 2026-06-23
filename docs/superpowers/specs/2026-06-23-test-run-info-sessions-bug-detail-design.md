# Test Run Info, View Sessions & Bug Detail Breakdown — Design

**Goal:** Surface real Xray test-run information across the app, preserve each
view's state when switching views, and enrich the Bugs detail so affected tests
break down by run/execution/plan/version/project — with an in-view read-only
test detail.

**Status:** Approved design. Built as one spec in four ordered phases.

## Background (current state)

From a codebase survey (`internal/store`, `internal/testrepo`, `internal/jira`,
`frontend/src`):

- **Run results** live only in `test_container_test.run_status`, populated for
  **Test Executions** only (empty for Test Sets / Test Plans). There is no run
  timestamp, no per-test environment, and no executed-by. There is no query that
  returns a test's run history across executions, and the Test Plan ↔ Test
  Execution relationship is implicit (shared membership), not stored.
- **Views unmount on switch.** `App.tsx` renders the active view with conditional
  JSX, so leaving a view destroys its component and all `useState` (selection,
  filters, search, sort, pagination, expanded groups, active sub-tab). Only a few
  things persist via `localStorage`: grid columns (`xtm.gridColumns`), sidebar
  width (`xtm.sidebarWidth`), detail width (`xtm.detailWidth`), reviewer name.
- **Bug detail** (`BugsPanel.tsx`) shows affected tests as `BugTest`
  `{key, summary, status, runStatus}` where `runStatus` is a consolidated
  worst-wins value. Clicking a test key calls `onOpenTest(key)` →
  `setSelectedKey(key); setView("browse")` in `App.tsx`, which navigates away and
  loses the Bugs view state.
- **Duplicates** opens a member's content by mounting a full `TestDetail` in a
  side panel (Clone hidden by omitting `onCloned`), but `TestDetail` has no
  read-only mode — inline edits stay enabled.

## Locked decisions

1. **One spec, phased plan**, ordered A → B → C → D.
2. **Pull run date/details from Xray** (richer than cached-only): a new test-run
   table fed by a Jira/Xray fetch.
3. **View sessions survive view switches only** (in-memory; reset on app restart
   and profile change).
4. **Add a `readOnly` prop to `TestDetail`** (reuse, not a new component).

## Architecture & build order

```
A. View sessions (foundation, used by D)
B. Test run info  (data model + query, used by C)
C. Bug affected-tests breakdown (reuses B's run-history query)
D. Bug test-key navigation + read-only TestDetail sidebar (uses A + readOnly)
```

Each phase is independently shippable and testable. Backend changes carry Go unit
tests beside them and demo-mode data; `schemaVersion` bumps to **28**.

---

## Phase A — Per-view session state (T2)

**Mechanism.** A custom hook `useViewState<T>(scopeKey, fieldKey, initial)` that
behaves like `useState` but stores the value in a **module-level `Map`** so it
survives the component unmount that happens on a view switch. The `Map` is keyed
`"<profileId>:<viewKey>:<fieldKey>"`. It resets on app restart (module reload)
and is cleared for a profile when the profile changes (same hook exposes a
`clearViewState(profileId)` helper called where `App.tsx` already clears
per-profile state).

- **File:** `frontend/src/lib/viewState.ts` (the Map + hook + clear helper).
- **Adoption:** each view swaps `useState` → `useViewState` for the fields worth
  preserving. The hook signature mirrors `useState` so the diff is mechanical.

**Per-view fields to preserve** (selection, filters, search, sort, pagination,
expansion, sub-tab):

| View (component) | Preserved fields |
| --- | --- |
| Browse (`App.tsx` + `TestTable.tsx`) | `selectedKey`, sidebar `groupBy` + selected folder/container/component, grid search/status/execType filters, sort, page/pageSize |
| Preconditions (`PreconditionsView`) | `filter`, `sortField/sortDesc`, `selected`, `listPage/testsPage` |
| Requirements (`RequirementsView`) | `filter`, `covFilter`, sort, `selected`, `page/pageSize`, slide-over `detailKey` |
| Duplicates (`DuplicatesView`) | `filter`, `expanded` set, `page/pageSize`, `selectedKey` |
| Gap Analysis (`GapAnalysisView`) | `refSource`, `compareBy`, `threeWay`, `templateKind`, last `result` + per-list `page` |
| Test Calls (`TestCallsView`) | `page/pageSize`, sort, `detailKey` |
| Dashboard (`Dashboard`) | `folder/component/status` filters |
| Traceability (`TraceabilityTabs`) | active `tab` + each tab's filter selections |
| Containers (`ContainersView`) | `kind`, `selected` container, status/execType/env filters, sort, `Bugs` toggle + selected bug |

Scroll position is restored where it matters (Browse grid, long lists) via a ref
on the scroll container that reads/writes a numeric session field on
mount/unmount. Not every view needs it.

**Non-goal:** persisting across app restarts (localStorage) — explicitly out of
scope per decision 3.

---

## Phase B — Test run information from Xray (T1)

### Data model (migration v28)

Two new tables in `internal/store/store.go` (`baseSchema` + an ordered
`applyMigrations` block guarded by `current < 28` and the duplicate-column guard):

```sql
CREATE TABLE test_run (
  profile_id   TEXT NOT NULL,
  exec_key     TEXT NOT NULL,   -- Test Execution key
  test_key     TEXT NOT NULL,   -- Test key
  run_status   TEXT DEFAULT '', -- PASS/FAIL/TODO/EXECUTING/ABORTED/BLOCKED
  started_at   TEXT DEFAULT '', -- ISO; '' when unknown
  finished_at  TEXT DEFAULT '',
  executed_by  TEXT DEFAULT '', -- Jira username/display
  environment  TEXT DEFAULT '', -- the run's environment (subset of the exec's)
  defects      TEXT DEFAULT '', -- JSON array of defect keys
  PRIMARY KEY (profile_id, exec_key, test_key)
);

CREATE TABLE exec_plan (
  profile_id TEXT NOT NULL,
  exec_key   TEXT NOT NULL,   -- Test Execution key
  plan_key   TEXT NOT NULL,   -- associated Test Plan key
  PRIMARY KEY (profile_id, exec_key, plan_key)
);
```

`test_run` is the authoritative per-run record; `test_container_test.run_status`
stays as-is (the membership's current status) and is kept consistent with the run
on sync. `exec_plan` makes the implicit plan↔execution link explicit so a run can
name its Test Plan(s).

### Jira / Xray layer (`internal/jira`)

- `GetTestRuns(ctx, execKey) ([]TestRun, error)` — one Xray raven call per
  execution returning all its test runs (status, started/finished, executedBy,
  environment, defects). Behind the demo short-circuit; live shape marked
  `NOTE(xtm)` for verification against Xray Server/DC 8.4.0.
- Execution → Test Plan association: read the execution's associated Test Plans
  (Xray test-exec association or the Test Plan custom field) during the existing
  container fetch; mark `NOTE(xtm)`.
- Demo mode synthesizes plausible run dates, executed-by, and an environment
  drawn from the execution's environments, plus defect keys for failed runs.

### Sync (`internal/syncer`)

The container-sync pass for executions already runs; extend it to, per execution,
fetch runs and upsert `test_run`, and upsert `exec_plan` from the association.
Reuses the per-view sync progress already shown in the status bar. Partial
container sync refreshes runs for the synced executions.

### Repository (`internal/testrepo`)

- `GetTestRunHistory(profileID, testKey) ([]TestRunEntry, error)` —
  `test_run JOIN test_container` (exec details: summary, environments JSON,
  fix_versions JSON) `LEFT JOIN exec_plan` → ordered entries
  `{execKey, execSummary, planKeys[], environment, fixVersions[], runStatus,
  startedAt, finishedAt, executedBy, defects[]}`, newest run first.
- `GetRunRollup(profileID, containerKey) (RunRollup, error)` — for a Test Plan /
  Test Set: counts by run status across the executions that ran its member tests
  (built on the existing `GetContainerBoard` consolidation), e.g.
  `{passed, failed, notRun, executing, aborted, blocked, total, execCount}`.
- `GetExecutionMembersWithRuns(profileID, execKey)` — execution board enriched
  with each member's `test_run` fields (date/executedBy/environment).

### App bindings (`app.go`)

New thin adapters: `GetTestRunHistory`, `GetRunRollup`,
`GetExecutionMembersWithRuns`. `requireStore()` guards each.

### Generated models (`frontend/wailsjs/...` via `wails build`)

New `TestRunEntry`, `RunRollup`; regenerate bindings and re-export in `api.ts`.

### UI

- **Test Case** detail (`TestDetail.tsx`): a new collapsible **Run history**
  section under the existing detail content — a table of execution, status, date,
  executed-by, environment, plan, fix versions, defects (defect keys link to
  Jira). Lazily loaded on first open via `GetTestRunHistory`, cached per test
  like steps/custom fields.
- **Test Execution** detail (`ContainersView.tsx`): enrich each member row with
  run date / executed-by / environment from `GetExecutionMembersWithRuns`.
- **Test Plan / Test Set** detail (`ContainersView.tsx`): a run roll-up summary
  bar (passed/failed/not-run across executions) above the board, from
  `GetRunRollup`.

---

## Phase C — Bug detail: affected-tests breakdown (T3)

The affected-tests list stays lean but gains a **Project** column; `BugTest`
(`internal/testrepo/bugs.go` + models) gains `project` (from `external_test`
.project_key for cross-project members, otherwise the profile's project). The
existing consolidated `runStatus` stays.

Each affected-test row is **expandable inline** to show its full breakdown —
**fix version, run info, execution info, test plan info, project** — by reusing
**`GetTestRunHistory`** from Phase B, fetched lazily on expand. (This inline
breakdown is distinct from the Phase-D action, which opens the whole test's
read-only detail in a sidebar.) No new bug-specific query for the breakdown: the
run-history entries already carry execution, plan, environment, fix versions,
status, date, and defects, so the breakdown is a presentation of that data scoped
to the selected test.

- **Backend:** extend `ListTestsForBug` to populate `BugTest.project`. The
  breakdown reuses `GetTestRunHistory(testKey)`.
- **UI (`BugsPanel.tsx`):** Project column; expandable row (or sidebar) rendering
  the run-history table grouped/labelled as fix version / execution / plan /
  run-result / project.

---

## Phase D — Bug test-key navigation + read-only TestDetail (T4)

### `readOnly` mode on `TestDetail` (`TestDetail.tsx`)

Add `readOnly?: boolean`. When true: all inline field edits, the description /
step editors, step add/reorder/delete/duplicate/clone-from, precondition /
requirement add/replace/unlink, custom-field edits, workflow transition, and the
Clone action are disabled (controls hidden or rendered as static text / disabled
inputs). Read paths (fields, steps, preconditions, requirements, custom fields,
linked bugs, and the new Run history) render normally. Each edit handler early
-returns under `readOnly` as a safety net. Duplicates / Requirements / Test Calls
can adopt `readOnly` later (not required here).

### Two actions on an affected-test row (`BugsPanel.tsx` + `App.tsx`)

1. **Test key link → Browse** (kept), now **session-preserving**: because the
   Bugs view (inside Containers) uses Phase-A `useViewState`, its selected bug /
   filters / page are retained, so returning to the Bugs view restores them.
   Browse still selects the test via `selectedKey`.
2. **New "open detail" action** (a small icon/button beside the key) → mounts a
   **read-only `TestDetail`** as a right sidebar **within the Bugs view**,
   mirroring the Duplicates description/steps panel. `TestDetail` is rendered with
   `readOnly` and without `onCloned`; `onClose` clears the local
   `detailKey`/`detailVersion` session field.

---

## Testing

- **Go:** unit tests beside each store/repo/jira change — `test_run` / `exec_plan`
  schema + upserts, `GetTestRunHistory`, `GetRunRollup`,
  `GetExecutionMembersWithRuns`, `ListTestsForBug` project enrichment, and the
  demo run-data generator. Follow the existing `_test.go` patterns against the
  store and demo client.
- **Frontend:** no test runner (per repo conventions); verify via `wails dev` +
  the demo profile, and `cd frontend; npm run build` (tsc typecheck).

## Demo mode

The demo client seeds `test_run` rows (dates, executed-by, environments, defects)
and `exec_plan` links so Run history, the run roll-up, and the bug breakdown are
populated offline. The yellow `DEMO` chip behavior is unchanged.

## Out of scope / non-goals

- Persisting view state across app restarts (localStorage) — sessions are
  in-memory only.
- Editing run results from the new Run history view (the execution board remains
  the place to set results).
- Writing run dates / executed-by back to Xray (read-only enrichment).
- A separate read-only viewer component — `TestDetail` gains a `readOnly` prop
  instead.

## Files touched (summary)

- `internal/store/store.go` — `test_run`, `exec_plan`, `schemaVersion` 28.
- `internal/jira/` — `GetTestRuns`, exec→plan association, demo run data.
- `internal/syncer/` — fetch + upsert runs/plan links during container sync.
- `internal/testrepo/` — `GetTestRunHistory`, `GetRunRollup`,
  `GetExecutionMembersWithRuns`, `ListTestsForBug` project; types `TestRunEntry`,
  `RunRollup`.
- `app.go` — new bound adapters.
- `frontend/src/lib/viewState.ts` — `useViewState` + clear helper.
- `frontend/src/App.tsx` and each view component — adopt `useViewState`; Bugs
  navigation; mount read-only `TestDetail`.
- `frontend/src/components/TestDetail.tsx` — `readOnly` mode + Run history.
- `frontend/src/components/BugsPanel.tsx` — Project column, breakdown, open-detail
  action.
- `frontend/src/components/ContainersView.tsx` — execution run columns, plan/set
  roll-up.
- `frontend/src/api.ts` — re-export new bindings.
