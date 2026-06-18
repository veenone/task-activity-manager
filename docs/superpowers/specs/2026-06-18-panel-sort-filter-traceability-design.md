---
title: Panel sort/filter, scoped bug sync, and traceability improvements
date: 2026-06-18
status: approved
---

## Summary

Five independently shippable enhancements raised against the freshly-merged bug
feature and existing side panels / dashboard:

1. Sort controls on the Requirements, Preconditions, and Bugs side panels.
2. Sortable Test Set / Test Plan / Test Execution container lists and the
   execution member board.
3. Keyword + status filtering on the container lists.
4. Scope bug sync to the profile's `ScopeJQL` instead of every synced test.
5. Traceability Sankey: cascade the Execution filter from the selected Test
   Plan(s), and make the cross-project toggle surface cross-project executions
   **and** cross-project linked bugs.

Items 1–3 are pure frontend (client-side over already-loaded lists). Item 4 is
backend + a unit test. Item 5a is a backend helper + frontend wiring. Item 5b is
demo-seed data + a small dashboard addition.

## Context (current behavior)

- **Side panels** (`RequirementsView.tsx`, `PreconditionsView.tsx`,
  `BugsPanel.tsx`) each load their full list into memory and filter/paginate
  client-side. None expose a sort control; ordering is fixed by the backend
  (`ListRequirementsWithCoverage`, `ListPreconditionsWithUsage`,
  `ListBugsWithTests` — all numeric-aware key order).
- **Containers** render via `ContainerList.tsx` (a left nav list) fed by
  `ListContainers(profileID, kind)` with fixed `ORDER BY jira_key`. No sort, no
  filter. The execution member board (`GetContainerBoard`) paginates client-side
  with no sort.
- **`syncBugs`** (`internal/syncer/engine.go`) calls `AllTestKeys(profileID)` —
  every test ever synced — ignoring `profile.ScopeJQL`.
- **Traceability Sankey** (`Dashboard.tsx` + `internal/testrepo/sankey.go`):
  `planSel`, `execSel`, `crossProject` drive `GetTraceabilitySankey`; the
  Execution filter is independent of the Plan filter. `crossProjectOnly` keeps
  runs whose execution key prefix differs from the profile project. The demo
  seed contains no cross-project executions, so the toggle is always empty.
  Cross-project bugs are not surfaced on the dashboard at all.

The Test grid's sort (`TestTable.tsx` `toggleSort`, backend `keyNumericOrderExpr`)
is the pattern to mirror.

## Design

### Item 1 — Sort controls on side panels (frontend only)

Add a reusable `SortControl` component: a field `<select>` plus an asc/desc
toggle button, emitting `(field, desc)`. Each panel sorts its in-memory list
(after filtering, before paging) with a numeric-aware key comparator
(`keyCompare`) that parses the trailing digit run of an issue key, matching
`keyNumericOrderExpr`'s semantics.

Sort fields per panel:

| Panel | Fields (only those the DTO exposes) | Default |
|-------|--------|---------|
| Requirements | Key, Coverage, Tests (count), Status | Key, desc |
| Preconditions | Key, Type, Usage (count) | Key, desc |
| Bugs | Key, Status, Project, Priority | Key, desc |

(The projected DTOs — `RequirementCoverage`, `PreconditionUsage`,
`BugWithTests` — carry no `updated` timestamp, so "Updated" is intentionally not
offered here to avoid expanding the backend DTOs.)

Existing filter boxes and pagination are untouched; sort applies to the filtered
set so the page count reflects the active filter.

### Item 2 — Sortable container lists + execution board (frontend only)

- `ContainerList` gains the same `SortControl` (fields: Key, Summary, Status),
  sorting the `containers` array client-side. Default Key, asc (lists are short).
- The execution member board rows become sortable by test key, summary, and run
  status via clickable column headers reusing the grid's `toggleSort` pattern,
  applied client-side before the existing client-side paging.

### Item 3 — Filterable container lists (frontend only)

Add a filter bar above `ContainerList`, mirroring `RequirementsView`:

- **Keyword** box: case-insensitive match on container key or summary.
- **Status** `<select>`: options are the distinct statuses present in the loaded
  `containers` array, plus an "All" default.

Filtering is client-side over `containers` and recomputed with `useMemo`. Applies
to all three kinds (selected via the existing `kind` toggle). When the active
selection is filtered out, selection falls back to the first visible container
(same fallback `ContainersView` already uses on load).

### Item 4 — Scope bug sync to the synced (in-scope) tests (backend + test)

The defect is in the **demo client**: `demoBugs(testProjectKey)` ignores the
`testKeys` argument entirely (`engine.go` passes them; `jira/bugs.go` line 45
discards them with `_ = testKeys`), so bug sync returns every demo defect
regardless of which Tests are actually synced/in scope — the reported "global"
behavior.

`profile.ScopeJQL` is a Jira query evaluated **server-side at test-sync time**,
not something we can re-evaluate against SQLite. The local store therefore
already holds only the Tests the scoped test sync pulled, so the in-scope set is
exactly the cached Test keys (`AllTestKeys`). The fix is to make bug sync honor
those keys instead of returning the whole project's defects:

- `ListBugs` (demo branch) passes `testKeys` into `demoBugs`, which emits bugs
  and links only for Tests in that set; an empty set returns nothing.
- `syncBugs` keeps passing the cached in-scope keys (`AllTestKeys`) — no change
  there — but the result now actually narrows to them.
- Real `ListBugs` path already receives `testKeys`; its existing `TODO(xtm)`
  already specifies filtering linked issues to those Tests. Stays stubbed /
  `NOTE`-marked.

Test: demo `ListBugs` with a subset of `testKeys` returns only the bugs/links
for those Tests; an empty subset returns none; the full set reproduces today's
seed.

### Item 5a — Plan → Execution cascade (backend helper + frontend)

Add `Repository.ExecutionsForPlans(profileID string, planKeys []string)
([]Container, error)`: executions sharing at least one test with the given plans
(one SQL join over `test_container_test` for the plans' tests, then executions
containing those tests). Empty `planKeys` returns all executions.

`GetExecutionsForPlans` is exposed as an `App` binding. In `Dashboard.tsx`, when
`planSel` changes, the Execution multiselect's **options** are recomputed from
this method, and any `execSel` entries no longer valid are pruned (which
re-triggers the Sankey effect). The Requirement Sankey stays independent.

### Item 5b — Cross-project executions and bugs

Investigation correction: the demo generator **already** seeds two cross-project
executions (`XRAYINT-TE-1/2`, `demo.go:411-447`) that run this project's tests,
and `app.go:1659-1669` already threads the profile `projectKey` into
`GetTraceabilitySankey`, so the executions path should work. The remaining work
is therefore a regression test plus the genuinely-missing bug surfacing:

1. **Regression test (executions):** a `GetTraceabilitySankey` test in
   `internal/testrepo` asserting that with `crossProjectOnly=true` over seeded
   data, only executions whose key prefix differs from the profile project
   survive (and the in-project ones drop). Plus a `projectKeyOf` unit test for
   the real key shape (`RND_P_4TFINT_05-123` → `RND_P_4TFINT_05`) and the demo
   shape (`XRAYINT-TE-1` → `XRAYINT`). If the test reveals a threading/prefix
   bug, fix it; if it passes, the executions path is confirmed working.
2. **Cross-project bugs on the dashboard:** when `crossProject` is on, show a
   compact "cross-project bugs" list/count beside the Sankey, driven by the
   existing `ListBugsWithTests` data filtered to bugs whose `projectKey` differs
   from the profile project. Each entry is a hyperlink (same open-in-browser
   pattern already used on test detail). Bugs are **not** woven into the Sankey
   flow — they are not a run status and would break the balanced-flow invariant
   (Plan→Exec→Status each sum to the same total). This needs the profile project
   key on the client; add a tiny `GetProfileProjectKey(profileID)` binding (or
   reuse the existing profile fetch the dashboard can call).

## Out of scope (YAGNI)

- Server-side sort/filter for the side panels (lists are already fully loaded).
- Persisting sort/filter preferences across sessions.
- Weaving bugs into the Sankey graph itself.
- Any live-Jira REST wiring (`ListBugs` real path stays stubbed — Phase 7).

## Testing

- Go unit tests: demo `ListBugs` honors `testKeys` (subset / empty / full),
  `ExecutionsForPlans`, `projectKeyOf`, and demo-seed cross-project executions
  present.
- Frontend: `tsc` typecheck + `npm run build` (no frontend test runner). Manual
  demo-mode verification of each new control.

## Rollout

Five independent slices; each can be its own commit/PR. Suggested order: 1 → 2 →
3 (shared `SortControl`), then 4, then 5a, then 5b.
