---
title: Sub-task parent traceability Sankey + tabbed traceability container
date: 2026-06-18
status: approved
---

## Summary

Two linked changes to the dashboard's traceability area:

1. **A third Sankey** tracing sub-task Test Executions: **Parent issue → Test
   Execution → Run result** (3 layers), scoped to sub-task executions only
   (those with a parent). It carries a Parent multi-select filter.
2. **A dedicated Traceability view** holding all three Sankeys in tabs —
   Requirement, Execution (the existing Plan→Exec→Status flow), and the new
   Sub-task flow — so only one shows at a time. The two Sankeys move out of
   `Dashboard` into a new top-level `TraceabilityTabs` view that gets its own
   menu entry: a native **View → Traceability** item, a top-bar tab, and a
   matching Outline "Supported Views" entry.

## Context (current behavior)

- `Dashboard.tsx` renders two Sankeys in two separate `stat-panel sankey-panel`
  blocks, each with its own filters and effects:
  - Requirement traceability — `GetRequirementTraceability(profileID, reqSel)`
    → `RequirementSankey` (an N-layer renderer: layers computed from the data,
    even column distribution, headers from a `COLUMNS` array).
  - Plan/Execution traceability — `GetTraceabilitySankey(profileID, planSel,
    execSel, crossProject)` → `SankeyChart` (a 3-layer renderer hardcoded to
    layers `[0,1,2]` with headers "Test Plans / Test Executions / Run Status",
    status colors on layer 2, a min-node-height + vertical-scroll layout, and a
    status legend). Plus a cross-project bug list beside it.
- Sub-task Test Executions are stored as `test_container` rows with
  `kind='testexec'` and a non-empty `parent_key` (schema v23). Their members and
  run statuses live in `test_container_test` (`container_key, test_key,
  run_status`). The traceability helpers live in `internal/testrepo/sankey.go`
  (e.g. `Sankey`, `SankeyNode`, `SankeyLink`, `nonEmptyKeys`, `sqlPlaceholders`,
  `flatten`, `orKey`).
- The demo seed includes sub-task executions (`DEMO-STE-1/2`, parents
  `DEMO-S-1/2`) with run links, so the new flow is exercisable offline.

## Feature A — Sub-task parent traceability Sankey (backend)

Add `GetSubTaskTraceability(profileID string, parentFilters []string) (Sankey,
error)` in `internal/testrepo/` (a new file `subtasksankey.go`, beside
`sankey.go`, to keep `sankey.go` focused).

Flow (3 layers), computed entirely from the local store:

- **Layer 0 — Parent:** node id `parent:<parentKey>`, label `<parentKey>`.
- **Layer 1 — Execution:** node id `exec:<execKey>`, label the execution summary
  or key (`orKey(summary, key)`).
- **Layer 2 — Run status:** node id `status:<status>`, label `<status>`;
  `(none)` when the membership has no run status.

The flow unit is a membership — a test's run in a sub-task execution. Each
membership adds 1 to the `parent→exec` link, the `exec→status` link, and to all
three node values, so the three layers sum to the same total and the diagram
balances (mirrors `GetTraceabilitySankey`). Each sub-task execution has exactly
one parent (`parent_key`), so there is no "multiple parents" bucket.

Query shape:

```sql
SELECT c.parent_key, l.container_key, l.test_key, l.run_status
FROM test_container_test l
JOIN test_container c
  ON c.profile_id = l.profile_id AND c.jira_key = l.container_key
WHERE l.profile_id = ? AND c.kind = 'testexec' AND c.parent_key != ''
-- optional: AND c.parent_key IN (<placeholders>)   when parentFilters non-empty
```

`parentFilters` (trimmed via `nonEmptyKeys`) narrows to chosen parents; empty
means all sub-task executions. Execution summaries are read once via the
existing `containerSummaries(profileID)` helper. Node/link assembly reuses the
same `note`/`flatten` pattern as `GetTraceabilitySankey`.

Expose it as a thin `App` method (guarded by `requireStore()`):

```go
func (a *App) GetSubTaskTraceability(profileID string, parentFilters []string) (testrepo.Sankey, error)
```

No new endpoint is needed for the parent-filter options — the dashboard derives
them client-side from the sub-task executions already returned by
`ListContainers(profileID, "testexec")` (those with a non-empty `parentKey`).

## Feature B — Renderer reuse

The new chart is the same 3-layer, status-colored, scrolling shape as the
Plan/Execution chart, so **reuse `SankeyChart`** rather than add a renderer.
Parameterize its currently-hardcoded bits with optional props that default to
today's values (so the Plan tab is unchanged):

- `columns?: [string, string, string]` — the three column headers. Default
  `["Test Plans", "Test Executions", "Run Status"]`. The Sub-task tab passes
  `["Parent issues", "Test Executions", "Run Status"]`.
- `emptyHint?: string` and `filteredHint?: string` — the two empty-state
  messages, so the Sub-task chart reads "No sub-task execution runs…" rather than
  "Test Plan / Execution". Defaults preserve the current text.

The hardcoded column-head `<span>`s and empty-state copy in `SankeyChart` read
from these props. `RequirementSankey` is untouched.

## Feature C — Dedicated Traceability view (tabs)

Create `frontend/src/components/TraceabilityTabs.tsx` as a **self-contained
top-level view** that owns the whole traceability area (the two Sankeys move out
of `Dashboard`, which keeps it focused on its stat tiles/bars/duplicates).

- **Props:** `profileId`, `refreshKey`, `jiraUrl`. The component fetches
  everything else itself.
- **Owns and fetches:** the three Sankey datasets (`reqSankey` via
  `GetRequirementTraceability`, `sankey` via `GetTraceabilitySankey`, `subSankey`
  via the new `GetSubTaskTraceability`); the gating it needs (`GetStatistics` for
  `byCoverage`/`testExecutions`, `GetProfileProjectKey` for the cross-project
  filter and bug list, `ListRequirementsWithCoverage` for the requirement filter
  options, `ListContainers(profileId, "testexec")` for both the cascaded
  execution options and the distinct sub-task `parentKey`s); and all filter state
  (`reqSel`; `planSel`/`execSel`/`crossProject` + cascaded `execs` + the
  cross-project bug list; `parentSel`). The plan→exec cascade, cross-project bug
  list, and requirement filter behave exactly as they do today — the logic moves
  over from `Dashboard` unchanged, with one new effect for
  `GetSubTaskTraceability(profileId, parentSel)`.
- **Renders:** a tab bar with three always-visible tabs — **Requirement**,
  **Execution** (default selected via `useState<"req"|"exec"|"subtask">("exec")`),
  **Sub-task** — and, below it, the active tab's chart plus that tab's own
  filters:
  - Requirement → `RequirementSankey` + the requirement `MultiSelect`.
  - Execution → `SankeyChart` (default columns) + the plan/exec `MultiSelect`s,
    cross-project toggle, clear button, and the cross-project bug list.
  - Sub-task → `SankeyChart` with `columns={["Parent issues","Test
    Executions","Run Status"]}` and the parent-hint empty text, plus a Parent
    `MultiSelect` whose options are the distinct `parentKey`s of the synced
    sub-task executions.

  Filter state persists across tab switches (it lives in the component, not per
  tab mount). The tab bar reuses the existing segmented-button styling
  (`seg-btn`, as in the Containers/Bugs toggle).

`Dashboard` loses the two `sankey-panel` blocks and all their state/effects (and
the now-unused imports). The traceability `MultiSelect`s it imported move with it.

## Feature D — Menu entry and navigation

Add **Traceability** as a top-level view, distinct from Dashboard:

- `frontend/src/App.tsx`:
  - Add the view to the `view` union/state and render
    `<main className="content content-dashboard"><TraceabilityTabs profileId={…}
    refreshKey={…} jiraUrl={…} /></main>` for `view === "traceability"`.
  - Add a top-bar `view-tab` button "Traceability" (placed after Dashboard).
  - Add `"menu:view-traceability": () => setView("traceability")` to
    `menuActions.current`.
- `main.go` `appMenu`: add `view.AddText("Traceability", nil,
  emit("menu:view-traceability"))` to the View submenu (after "Dashboard").

No backend or binding change for the menu itself — it reuses the existing
`menu:view-*` event pattern.

**Outline docs** (the documentation "menu"): add a Traceability entry to the XTM
**Supported Views** doc and mention the sub-task traceability Sankey + the new
view in the **Feature List**. These doc edits happen at the end, after the code
ships, and are not part of the Go/TS test surface.

## Out of scope (YAGNI)

- A per-test-case layer (the user chose the compact 3-layer flow).
- Migrating `RequirementSankey` or the Plan chart onto a shared renderer.
- Live Jira changes — this is all computed from the local store.

## Testing

- **Go:** `GetSubTaskTraceability` over seeded data builds a balanced 3-layer
  flow (parent / exec / status totals equal); the parent filter narrows to the
  chosen parent; standalone executions (`parent_key = ''`) are excluded; an
  empty result yields an empty `Sankey` (no nodes), not an error. Reuse the
  shared `newTestRepo` / `seedContainer` / `seedContainerTest` helpers.
- **Frontend:** `go build ./...` + regenerate Wails bindings (new `App` method);
  `cd frontend && npm run build` (tsc + vite). No FE test runner — verify the
  tab switching, the parent filter, and the three charts manually in demo mode.

## Rollout

One slice (the features are interdependent — the view needs the new chart and
the renderer params). Suggested commit order within it: backend +
binding; `SankeyChart` param props; `TraceabilityTabs` component (moving the
sankey logic out of `Dashboard`); the `Traceability` view + menu wiring (App.tsx
tab/menuActions + `main.go` View menu); then the Outline doc updates (Supported
Views + Feature List) after the build is green.
