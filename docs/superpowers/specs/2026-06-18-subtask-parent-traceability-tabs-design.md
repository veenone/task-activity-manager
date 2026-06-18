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
2. **A tabbed container** holding all three Sankeys — Requirement, Execution
   (the existing Plan→Exec→Status flow), and the new Sub-task flow — so only one
   shows at a time. The two existing Sankey panels are extracted from
   `Dashboard` into a focused `TraceabilityTabs` component.

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

## Feature C — Tabbed traceability container

Extract a new `frontend/src/components/TraceabilityTabs.tsx` that owns the
traceability area. The two Sankey blocks have grown `Dashboard`; moving the
state, effects, filters and JSX into a focused component keeps `Dashboard`
readable. `TraceabilityTabs` receives what it needs as props and self-contains
the rest:

- **Props:** `profileId`, `refreshKey`, `nonce`, `jiraUrl`, `projectKey`,
  `hasCoverage` (`stats.byCoverage.length > 0`), `hasExecutions`
  (`stats.testExecutions > 0`), and `reqOptions` (the requirement list for the
  requirement filter).
- **Owns:** the three Sankey datasets (`reqSankey`, `sankey`, `subSankey`), all
  filter state (`reqSel`; `planSel`/`execSel`/`crossProject` + the cascaded
  `execs` options and cross-project bug list; `parentSel` + the derived
  `parents` list), and the effects that fetch each Sankey (moved verbatim from
  `Dashboard`, plus a new effect for `GetSubTaskTraceability(profileId,
  parentSel)`).
- **Renders:** a tab bar with three always-visible tabs — **Requirement**,
  **Execution** (default selected), **Sub-task** — and, below it, the active
  tab's chart plus that tab's own filters:
  - Requirement → `RequirementSankey` + the requirement `MultiSelect`.
  - Execution → `SankeyChart` (default columns) + the plan/exec `MultiSelect`s,
    cross-project toggle, clear button, and the cross-project bug list (the
    current behavior, moved as-is).
  - Sub-task → `SankeyChart` with `columns={["Parent issues","Test
    Executions","Run Status"]}` and the parent-hint empty text, plus a Parent
    `MultiSelect` whose options are the distinct `parentKey`s of the synced
    sub-task executions.

`Dashboard` imports `TraceabilityTabs` and renders it where the two
`sankey-panel` blocks were, passing the props above; the sankey state, effects
and JSX are removed from `Dashboard`.

Tab state is a local `useState<"req" | "exec" | "subtask">("exec")`. Each tab's
filters keep their state across tab switches (the state lives in the component,
not per-tab-mount). The tab bar reuses the existing segmented-button styling
(e.g. the `seg-btn` pattern used by the Containers/Bugs toggle) so no new visual
language is introduced.

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

One slice (the three features are interdependent — the tab container needs the
new chart and the renderer params). Suggested commit order within it: backend +
binding, then `SankeyChart` param props, then `TraceabilityTabs` extraction +
wiring.
