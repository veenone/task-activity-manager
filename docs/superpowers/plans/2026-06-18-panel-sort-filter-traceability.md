# Panel sort/filter, scoped bug sync, traceability improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add sort controls to the Requirements/Preconditions/Bugs panels and the container picker, keyword+status filtering on the container picker, scope bug sync to the synced tests, cascade the Sankey Execution filter from the selected Plan(s), and surface cross-project bugs on the dashboard.

**Architecture:** Items 1–3 are pure frontend over already-loaded lists (a shared `SortControl` + a numeric-aware `keyCompare`). Item 4 fixes the demo `ListBugs` to honor the `testKeys` it is passed. Item 5a adds a backend `ExecutionsForPlans` query + `App` binding and wires the dashboard to cascade. Item 5b adds a regression test for cross-project executions and a small cross-project bug list on the dashboard.

**Tech Stack:** Go (`modernc.org/sqlite`), Wails v2 bindings, React + TypeScript (Vite). Backend has Go unit tests; the frontend has no test runner, so frontend tasks verify with `cd frontend; npm run build` (tsc typecheck) plus manual demo-mode checks.

**Spec:** `docs/superpowers/specs/2026-06-18-panel-sort-filter-traceability-design.md`

**Conventions:**
- Each backend change is TDD: write the failing Go test, run it red, implement, run it green, commit.
- Frontend: implement, typecheck/build, commit. Keep `frontend/wailsjs/` generated bindings regenerated via `wails generate module` when an `App` method is added/changed (Tasks 5 and 7).
- Do NOT stage `CLAUDE.md` or local tooling files. Commit only the files each step names.

---

## File map

**Create:**
- `frontend/src/sort.ts` — `keyCompare`, `cmpStr`, `applyDir` helpers.
- `frontend/src/components/SortControl.tsx` — reusable field-select + asc/desc toggle.
- `internal/testrepo/executionsforplans.go` — `ExecutionsForPlans` query.
- `internal/testrepo/executionsforplans_test.go` — its test.
- `internal/jira/bugs_scope_test.go` — demo `ListBugs` honors `testKeys`.
- `internal/testrepo/sankey_crossproject_test.go` — cross-project executions + `projectKeyOf` tests.

**Modify:**
- `frontend/src/components/BugsPanel.tsx` — sort control (Item 1).
- `frontend/src/components/RequirementsView.tsx` — sort control (Item 1).
- `frontend/src/components/PreconditionsView.tsx` — sort control (Item 1).
- `frontend/src/components/ContainersView.tsx` — container keyword/status filter + sort (Items 2 & 3).
- `frontend/src/components/Dashboard.tsx` — Plan→Exec cascade (5a) + cross-project bug list (5b).
- `frontend/src/App.css` — styles for the new controls.
- `internal/jira/bugs.go` — pass `testKeys` into `demoBugs`; filter the seed (Item 4).
- `app.go` — `ExecutionsForPlans` + `GetProfileProjectKey` bindings.
- `frontend/src/api.ts` + `frontend/wailsjs/**` — regenerate after the new bindings.

---

## Task 1: Shared sort helpers (`keyCompare`)

**Files:**
- Create: `frontend/src/sort.ts`

- [ ] **Step 1: Write the helper**

```ts
// Numeric-aware comparison for Jira-style issue keys, mirroring the backend
// keyNumericOrderExpr: the trailing digit run sorts numerically (so "DEMO-9"
// precedes "DEMO-10"); the leading non-numeric part breaks ties lexically.
export function keyCompare(a: string, b: string): number {
  const pa = splitKey(a);
  const pb = splitKey(b);
  if (pa.prefix !== pb.prefix) return pa.prefix < pb.prefix ? -1 : 1;
  if (pa.num !== pb.num) return pa.num - pb.num;
  return a < b ? -1 : a > b ? 1 : 0;
}

function splitKey(k: string): { prefix: string; num: number } {
  const m = /^(.*?)(\d+)\s*$/.exec(k ?? "");
  if (!m) return { prefix: k ?? "", num: -1 };
  return { prefix: m[1], num: parseInt(m[2], 10) };
}

// Case-insensitive string compare; empty strings sort last so blanks don't lead.
export function cmpStr(a: string, b: string): number {
  const x = (a ?? "").toLowerCase();
  const y = (b ?? "").toLowerCase();
  if (!x && y) return 1;
  if (x && !y) return -1;
  return x < y ? -1 : x > y ? 1 : 0;
}

// Flip a comparison result when descending.
export function applyDir(cmp: number, desc: boolean): number {
  return desc ? -cmp : cmp;
}
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend; npm run build`
Expected: build succeeds (no new file errors).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/sort.ts
git commit -m "Add numeric-aware key sort helpers for side panels"
```

---

## Task 2: Reusable `SortControl` component

**Files:**
- Create: `frontend/src/components/SortControl.tsx`
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Write the component**

```tsx
interface SortField {
  value: string;
  label: string;
}

interface Props {
  fields: SortField[];
  field: string;
  desc: boolean;
  onChange: (field: string, desc: boolean) => void;
}

// SortControl is a compact field-select plus an ascending/descending toggle,
// used by the Requirements / Preconditions / Bugs panels and the container
// picker. It owns no state — the parent holds (field, desc) and re-sorts.
export function SortControl({ fields, field, desc, onChange }: Props) {
  return (
    <div className="sort-control">
      <span className="sort-control-label muted">Sort</span>
      <select
        className="sort-field"
        value={field}
        onChange={(e) => onChange(e.target.value, desc)}
        title="Sort field"
      >
        {fields.map((f) => (
          <option key={f.value} value={f.value}>
            {f.label}
          </option>
        ))}
      </select>
      <button
        className="btn sort-dir"
        onClick={() => onChange(field, !desc)}
        title={desc ? "Descending — click for ascending" : "Ascending — click for descending"}
        aria-label="Toggle sort direction"
      >
        {desc ? "↓" : "↑"}
      </button>
    </div>
  );
}
```

- [ ] **Step 2: Add styles to `frontend/src/App.css`** (append near the other panel styles)

```css
.sort-control {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 4px 0;
}
.sort-control-label {
  font-size: 12px;
}
.sort-control .sort-field {
  padding: 2px 6px;
}
.sort-control .sort-dir {
  padding: 2px 8px;
  line-height: 1;
}
```

- [ ] **Step 3: Typecheck**

Run: `cd frontend; npm run build`
Expected: build succeeds. (The component is unused until Task 3 — that's fine; tsc allows unused exports.)

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/SortControl.tsx frontend/src/App.css
git commit -m "Add reusable SortControl (field select + asc/desc toggle)"
```

---

## Task 3: Sort controls on the three side panels (Item 1)

**Files:**
- Modify: `frontend/src/components/BugsPanel.tsx`
- Modify: `frontend/src/components/RequirementsView.tsx`
- Modify: `frontend/src/components/PreconditionsView.tsx`

### 3a — BugsPanel

- [ ] **Step 1: Add imports** (top of `BugsPanel.tsx`, alongside existing imports)

```tsx
import { SortControl } from "./SortControl";
import { keyCompare, cmpStr, applyDir } from "../sort";
```

- [ ] **Step 2: Add sort state** (after the existing `const [pageSize, setPageSize] = useState(15);`)

```tsx
  const [sortField, setSortField] = useState("key");
  const [sortDesc, setSortDesc] = useState(true);
```

- [ ] **Step 3: Add the comparator** (module scope, below the `Props` interface)

```tsx
function cmpBug(a: BugWithTests, b: BugWithTests, field: string): number {
  switch (field) {
    case "status":
      return cmpStr(a.status, b.status) || keyCompare(a.key, b.key);
    case "project":
      return cmpStr(a.projectKey, b.projectKey) || keyCompare(a.key, b.key);
    case "priority":
      return cmpStr(a.priority, b.priority) || keyCompare(a.key, b.key);
    default:
      return keyCompare(a.key, b.key);
  }
}
```

- [ ] **Step 4: Sort inside the `shown` memo** — replace the existing `shown` useMemo body with:

```tsx
  const shown = useMemo(() => {
    const f = filter.trim().toLowerCase();
    const base = !f
      ? bugs
      : bugs.filter(
          (b) =>
            b.key.toLowerCase().includes(f) ||
            b.summary.toLowerCase().includes(f) ||
            b.projectKey.toLowerCase().includes(f) ||
            b.status.toLowerCase().includes(f),
        );
    return [...base].sort((a, b) => applyDir(cmpBug(a, b, sortField), sortDesc));
  }, [bugs, filter, sortField, sortDesc]);
```

- [ ] **Step 5: Render the control** — directly after the filter `<input className="search bugs-md-filter" .../>` element, add:

```tsx
        <SortControl
          fields={[
            { value: "key", label: "Key" },
            { value: "status", label: "Status" },
            { value: "project", label: "Project" },
            { value: "priority", label: "Priority" },
          ]}
          field={sortField}
          desc={sortDesc}
          onChange={(f, d) => {
            setSortField(f);
            setSortDesc(d);
          }}
        />
```

- [ ] **Step 6: Reset to page 0 on sort change** — extend the existing page-reset effect dependency array:

Change `}, [profileId, refreshKey, filter]);` (the `setPage(0)` effect) to
`}, [profileId, refreshKey, filter, sortField, sortDesc]);`

### 3b — RequirementsView

- [ ] **Step 7: Add imports** (alongside existing imports in `RequirementsView.tsx`)

```tsx
import { SortControl } from "./SortControl";
import { keyCompare, cmpStr, applyDir } from "../sort";
```

- [ ] **Step 8: Add sort state** (after `const [covFilter, setCovFilter] = useState("");`)

```tsx
  const [sortField, setSortField] = useState("key");
  const [sortDesc, setSortDesc] = useState(true);
```

- [ ] **Step 9: Add the comparator** (module scope, below `COVERAGE_LABEL`)

```tsx
const COVERAGE_RANK: Record<string, number> = {
  FAILED: 0,
  NOTRUN: 1,
  PASSED: 2,
  UNCOVERED: 3,
};

function cmpReq(
  a: RequirementCoverage,
  b: RequirementCoverage,
  field: string,
): number {
  switch (field) {
    case "coverage":
      return (
        (COVERAGE_RANK[a.coverage] ?? 9) - (COVERAGE_RANK[b.coverage] ?? 9) ||
        keyCompare(a.key, b.key)
      );
    case "tests":
      return a.testCount - b.testCount || keyCompare(a.key, b.key);
    case "status":
      return cmpStr(a.status, b.status) || keyCompare(a.key, b.key);
    default:
      return keyCompare(a.key, b.key);
  }
}
```

- [ ] **Step 10: Sort inside the `filtered` memo** — replace the existing `filtered` useMemo with:

```tsx
  const filtered = useMemo(() => {
    const f = filter.trim().toLowerCase();
    const base = list.filter(
      (r) =>
        (!covFilter || r.coverage === covFilter) &&
        (!f ||
          r.key.toLowerCase().includes(f) ||
          r.summary.toLowerCase().includes(f)),
    );
    return [...base].sort((a, b) => applyDir(cmpReq(a, b, sortField), sortDesc));
  }, [list, filter, covFilter, sortField, sortDesc]);
```

- [ ] **Step 11: Reset page on sort change** — change the `setPage(0)` effect dependency from `}, [filter, covFilter]);` to `}, [filter, covFilter, sortField, sortDesc]);`

- [ ] **Step 12: Render the control** — directly after the coverage-summary `</div>` (the `reqs-coverage-summary` block), before the `{loading ? (` ternary, add:

```tsx
        <SortControl
          fields={[
            { value: "key", label: "Key" },
            { value: "coverage", label: "Coverage" },
            { value: "tests", label: "Tests" },
            { value: "status", label: "Status" },
          ]}
          field={sortField}
          desc={sortDesc}
          onChange={(f, d) => {
            setSortField(f);
            setSortDesc(d);
          }}
        />
```

### 3c — PreconditionsView

- [ ] **Step 13: Add imports** (alongside existing imports in `PreconditionsView.tsx`)

```tsx
import { SortControl } from "./SortControl";
import { keyCompare, cmpStr, applyDir } from "../sort";
```

- [ ] **Step 14: Add sort state** (after `const [filter, setFilter] = useState("");`)

```tsx
  const [sortField, setSortField] = useState("key");
  const [sortDesc, setSortDesc] = useState(true);
```

- [ ] **Step 15: Add the comparator** (module scope, below `PRECOND_TYPES`)

```tsx
function cmpPre(
  a: PreconditionUsage,
  b: PreconditionUsage,
  field: string,
): number {
  switch (field) {
    case "type":
      return cmpStr(a.type, b.type) || keyCompare(a.key, b.key);
    case "usage":
      return a.testCount - b.testCount || keyCompare(a.key, b.key);
    default:
      return keyCompare(a.key, b.key);
  }
}
```

- [ ] **Step 16: Sort inside the `filtered` memo** — replace the existing `filtered` useMemo with:

```tsx
  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const base = !q
      ? list
      : list.filter(
          (p) =>
            p.key.toLowerCase().includes(q) ||
            p.summary.toLowerCase().includes(q) ||
            p.type.toLowerCase().includes(q),
        );
    return [...base].sort((a, b) => applyDir(cmpPre(a, b, sortField), sortDesc));
  }, [list, filter, sortField, sortDesc]);
```

- [ ] **Step 17: Reset page on sort change** — change the `setListPage(0)` effect dependency from `}, [filter]);` to `}, [filter, sortField, sortDesc]);`

- [ ] **Step 18: Render the control** — the precondition list filter input is rendered in the JSX (search `className="search` within the file). Place the control immediately after that filter `<input>`:

```tsx
        <SortControl
          fields={[
            { value: "key", label: "Key" },
            { value: "type", label: "Type" },
            { value: "usage", label: "Usage" },
          ]}
          field={sortField}
          desc={sortDesc}
          onChange={(f, d) => {
            setSortField(f);
            setSortDesc(d);
          }}
        />
```

- [ ] **Step 19: Typecheck**

Run: `cd frontend; npm run build`
Expected: build succeeds.

- [ ] **Step 20: Commit**

```bash
git add frontend/src/components/BugsPanel.tsx frontend/src/components/RequirementsView.tsx frontend/src/components/PreconditionsView.tsx
git commit -m "Add sort controls to the Requirements, Preconditions and Bugs panels"
```

---

## Task 4: Container picker keyword + status filter and sort (Items 2 & 3)

**Files:**
- Modify: `frontend/src/components/ContainersView.tsx`

The container picker is the inline `<select>` in `ContainersView` fed by the
`containers` array. We add a keyword box, a status `<select>`, and a `SortControl`
above it; all three operate client-side over `containers` to produce `viewContainers`,
which feeds the picker `<option>`s. The board member table also becomes sortable.

- [ ] **Step 1: Add imports** (alongside existing imports)

```tsx
import { SortControl } from "./SortControl";
import { keyCompare, cmpStr, applyDir } from "../sort";
```

- [ ] **Step 2: Add filter/sort state** (after `const [kind, setKind] = useState("testplan");`)

```tsx
  const [cFilter, setCFilter] = useState("");
  const [cStatus, setCStatus] = useState("");
  const [cSortField, setCSortField] = useState("key");
  const [cSortDesc, setCSortDesc] = useState(false);
  const [rowSortField, setRowSortField] = useState("key");
  const [rowSortDesc, setRowSortDesc] = useState(false);
```

- [ ] **Step 3: Reset container filters when the kind changes** — add this effect right after the existing `useEffect(() => { setBugFor(null); }, [mode, selected, kind]);`:

```tsx
  useEffect(() => {
    setCFilter("");
    setCStatus("");
  }, [kind]);
```

- [ ] **Step 4: Compute distinct statuses + the filtered/sorted view** — add right after `const selectedContainer = containers.find((c) => c.key === selected) ?? null;`:

```tsx
  const statusOptions = useMemo(() => {
    const s = new Set<string>();
    for (const c of containers) if (c.status) s.add(c.status);
    return [...s].sort();
  }, [containers]);

  const viewContainers = useMemo(() => {
    const f = cFilter.trim().toLowerCase();
    const base = containers.filter(
      (c) =>
        (!cStatus || c.status === cStatus) &&
        (!f ||
          c.key.toLowerCase().includes(f) ||
          (c.summary ?? "").toLowerCase().includes(f)),
    );
    return [...base].sort((a, b) => {
      let cmp: number;
      switch (cSortField) {
        case "summary":
          cmp = cmpStr(a.summary, b.summary) || keyCompare(a.key, b.key);
          break;
        case "status":
          cmp = cmpStr(a.status, b.status) || keyCompare(a.key, b.key);
          break;
        default:
          cmp = keyCompare(a.key, b.key);
      }
      return applyDir(cmp, cSortDesc);
    });
  }, [containers, cFilter, cStatus, cSortField, cSortDesc]);
```

Add `useMemo` to the React import at the top of the file (currently `import { useEffect, useState } from "react";`):

```tsx
import { useEffect, useMemo, useState } from "react";
```

- [ ] **Step 5: Keep the selection valid against the filtered view** — add after the `viewContainers` memo:

```tsx
  useEffect(() => {
    if (viewContainers.length === 0) return;
    if (!viewContainers.some((c) => c.key === selected)) {
      setSelected(viewContainers[0].key);
    }
  }, [viewContainers, selected]);
```

- [ ] **Step 6: Feed the picker from `viewContainers`** — in the `board-picker` for the container, change the options source and the disabled/empty checks from `containers` to `viewContainers`:

Replace:
```tsx
            <select
              value={selected}
              onChange={(e) => setSelected(e.target.value)}
              disabled={containers.length === 0}
            >
              {containers.length === 0 && <option value="">None</option>}
              {containers.map((c) => (
                <option key={c.key} value={c.key}>
                  {c.key} — {c.summary}
                </option>
              ))}
            </select>
```
with:
```tsx
            <select
              value={selected}
              onChange={(e) => setSelected(e.target.value)}
              disabled={viewContainers.length === 0}
            >
              {viewContainers.length === 0 && <option value="">None</option>}
              {viewContainers.map((c) => (
                <option key={c.key} value={c.key}>
                  {c.key} — {c.summary}
                </option>
              ))}
            </select>
```

- [ ] **Step 7: Add the filter/sort bar** — directly after the `board-head` closing `</div>` (the one that wraps Type/picker/actions), and before `{error && <div className="error-text">{error}</div>}`, insert:

```tsx
      <div className="container-filter-bar">
        <input
          className="search container-filter"
          placeholder={`Filter ${kindLabel}s by key or name…`}
          value={cFilter}
          onChange={(e) => setCFilter(e.target.value)}
        />
        <select
          className="container-status-filter"
          value={cStatus}
          onChange={(e) => setCStatus(e.target.value)}
          title="Filter by status"
        >
          <option value="">All statuses</option>
          {statusOptions.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
        <SortControl
          fields={[
            { value: "key", label: "Key" },
            { value: "summary", label: "Name" },
            { value: "status", label: "Status" },
          ]}
          field={cSortField}
          desc={cSortDesc}
          onChange={(f, d) => {
            setCSortField(f);
            setCSortDesc(d);
          }}
        />
        <span className="muted container-filter-count">
          {viewContainers.length} of {containers.length}
        </span>
      </div>
```

- [ ] **Step 8: Make the board rows sortable** — replace the `const allRows = board?.rows ?? [];` line with a sorted view:

```tsx
  const allRows = useMemo(() => {
    const rows = board?.rows ?? [];
    return [...rows].sort((a, b) => {
      let cmp: number;
      switch (rowSortField) {
        case "summary":
          cmp = cmpStr(a.summary, b.summary) || keyCompare(a.testKey, b.testKey);
          break;
        case "status":
          cmp = cmpStr(a.status, b.status) || keyCompare(a.testKey, b.testKey);
          break;
        case "result":
          cmp =
            cmpStr(a.runStatus, b.runStatus) ||
            keyCompare(a.testKey, b.testKey);
          break;
        default:
          cmp = keyCompare(a.testKey, b.testKey);
      }
      return applyDir(cmp, rowSortDesc);
    });
  }, [board, rowSortField, rowSortDesc]);
```

- [ ] **Step 9: Make the board headers clickable** — replace the board table `<thead>` block's sortable headers. Change:

```tsx
              <th>Test</th>
              <th>Summary</th>
              <th>Status</th>
              <th>Execution</th>
              <th aria-label="Remove" />
```
to:
```tsx
              <th
                className="board-sort-th"
                onClick={() => toggleRowSort("key")}
                title="Sort by test key"
              >
                Test{rowSortIndicator("key")}
              </th>
              <th
                className="board-sort-th"
                onClick={() => toggleRowSort("summary")}
                title="Sort by summary"
              >
                Summary{rowSortIndicator("summary")}
              </th>
              <th
                className="board-sort-th"
                onClick={() => toggleRowSort("status")}
                title="Sort by status"
              >
                Status{rowSortIndicator("status")}
              </th>
              <th
                className="board-sort-th"
                onClick={() => toggleRowSort("result")}
                title="Sort by run result"
              >
                Execution{rowSortIndicator("result")}
              </th>
              <th aria-label="Remove" />
```

- [ ] **Step 10: Add the row-sort helpers** — inside the component, near the other handlers (e.g. after `toggleRun`):

```tsx
  function toggleRowSort(field: string) {
    if (rowSortField === field) {
      setRowSortDesc((d) => !d);
    } else {
      setRowSortField(field);
      setRowSortDesc(false);
    }
  }
  function rowSortIndicator(field: string): string {
    if (rowSortField !== field) return "";
    return rowSortDesc ? " ↓" : " ↑";
  }
```

- [ ] **Step 11: Add styles to `frontend/src/App.css`**

```css
.container-filter-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin: 6px 0;
}
.container-filter-bar .container-filter {
  flex: 1 1 220px;
  min-width: 160px;
}
.container-filter-count {
  font-size: 12px;
  white-space: nowrap;
}
.board-sort-th {
  cursor: pointer;
  user-select: none;
}
.board-sort-th:hover {
  text-decoration: underline;
}
```

- [ ] **Step 12: Typecheck**

Run: `cd frontend; npm run build`
Expected: build succeeds.

- [ ] **Step 13: Commit**

```bash
git add frontend/src/components/ContainersView.tsx frontend/src/App.css
git commit -m "Filter (keyword + status) and sort the container picker and board rows"
```

---

## Task 5: Scope bug sync to the synced tests (Item 4)

**Files:**
- Modify: `internal/jira/bugs.go`
- Create: `internal/jira/bugs_scope_test.go`

- [ ] **Step 1: Write the failing test**

```go
package jira

import (
	"context"
	"testing"
)

func TestListBugsHonorsTestKeys(t *testing.T) {
	c := NewClient("demo", "")
	ctx := context.Background()

	// Full set reproduces the seed: at least one bug, with links.
	full, fullLinks, err := c.ListBugs(ctx, "DEMO", nil, "Bug", nil)
	if err != nil {
		t.Fatalf("full ListBugs: %v", err)
	}
	if len(full) == 0 || len(fullLinks) == 0 {
		t.Fatalf("expected seeded bugs and links, got %d bugs, %d links", len(full), len(fullLinks))
	}

	// Restrict to a single linked test key: every returned link must reference it,
	// and every returned bug must be referenced by some surviving link.
	target := fullLinks[0].TestKey
	bugs, links, err := c.ListBugs(ctx, "DEMO", []string{target}, "Bug", nil)
	if err != nil {
		t.Fatalf("scoped ListBugs: %v", err)
	}
	if len(links) == 0 {
		t.Fatalf("expected at least one link for %s", target)
	}
	for _, l := range links {
		if l.TestKey != target {
			t.Errorf("link references out-of-scope test %s (want only %s)", l.TestKey, target)
		}
	}
	bugKeys := map[string]bool{}
	for _, b := range bugs {
		bugKeys[b.Key] = true
	}
	for _, l := range links {
		if !bugKeys[l.BugKey] {
			t.Errorf("link to %s has no matching bug in the result", l.BugKey)
		}
	}

	// Empty (no in-scope tests) returns nothing.
	noBugs, noLinks, err := c.ListBugs(ctx, "DEMO", []string{}, "Bug", nil)
	if err != nil {
		t.Fatalf("empty ListBugs: %v", err)
	}
	if len(noBugs) != 0 || len(noLinks) != 0 {
		t.Errorf("empty testKeys should yield nothing, got %d bugs, %d links", len(noBugs), len(noLinks))
	}
}
```

Note: `[]string{}` (non-nil empty) means "no in-scope tests → nothing"; `nil`
means "unfiltered" (preserves today's sync call and the seed). The test relies
on `demoFailedTestNums` producing ≥3 failed tests for the seed; that is already
guaranteed by the existing demo data.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/jira/ -run TestListBugsHonorsTestKeys -v`
Expected: FAIL — the scoped call currently returns all seed links (out-of-scope test keys present), and the empty call returns the full seed.

- [ ] **Step 3: Implement — thread `testKeys` into the demo path**

In `internal/jira/bugs.go`, change the demo branch of `ListBugs`:

```go
	if isDemoURL(c.baseURL) {
		bugs, links := demoBugs(testProjectKey, testKeys)
		if onProgress != nil {
			onProgress(len(bugs), len(bugs))
		}
		return bugs, links, nil
	}
```

Change `demoBugs` to accept and honor the keys. Replace its signature and add an
early filter set; keep all existing seeding logic but only emit links/bugs for
in-scope tests:

```go
// demoBugs seeds defect issues across two non-test projects, each linked to a
// demo Test that is actually marked FAILED, plus a test with two defects and a
// defect spanning two tests. When scope is non-nil it is the set of in-scope
// Test keys (the synced/ScopeJQL-narrowed tests); only bugs linked to those
// Tests are returned. A nil scope means unfiltered (full seed); an empty,
// non-nil scope means no in-scope tests, so nothing is returned.
func demoBugs(testProjectKey string, scope []string) ([]Bug, []BugLink) {
	if testProjectKey == "" {
		testProjectKey = "DEMO"
	}
	var inScope map[string]bool
	if scope != nil {
		inScope = make(map[string]bool, len(scope))
		for _, k := range scope {
			inScope[k] = true
		}
	}
	testInScope := func(testNum int) bool {
		if inScope == nil {
			return true
		}
		return inScope[fmt.Sprintf("%s-%d", testProjectKey, testNum)]
	}

	failed := demoFailedTestNums(10)
	if len(failed) < 3 {
		return []Bug{}, []BugLink{}
	}

	projects := []string{demoBugProject, demoBugProject2}
	bugs := []Bug{}
	links := []BugLink{}

	addBug := func(testNum int) string {
		n := len(bugs)
		project := projects[n%len(projects)]
		key := fmt.Sprintf("%s-%d", project, 100+n)
		bugs = append(bugs, Bug{
			Key:        key,
			ProjectKey: project,
			IssueType:  "Bug",
			Summary:    fmt.Sprintf("%s-%d %s", testProjectKey, testNum, demoBugSummaries[n%len(demoBugSummaries)]),
			Status:     demoBugStatuses[n%len(demoBugStatuses)],
			Priority:   demoBugPriorities[n%len(demoBugPriorities)],
		})
		return key
	}
	link := func(testNum int, bugKey string) {
		links = append(links, BugLink{
			TestKey: fmt.Sprintf("%s-%d", testProjectKey, testNum),
			BugKey:  bugKey,
			LinkID:  fmt.Sprintf("bl-%d", len(links)+1),
		})
	}

	// One defect per failed, in-scope test.
	for _, n := range failed {
		if testInScope(n) {
			link(n, addBug(n))
		}
	}
	// The first failed test carries a second defect.
	if testInScope(failed[0]) {
		link(failed[0], addBug(failed[0]))
	}
	// One defect spans two failed tests; keep links only for in-scope tests, and
	// only emit the bug if at least one endpoint is in scope.
	if testInScope(failed[1]) || testInScope(failed[2]) {
		spanKey := addBug(failed[1])
		if testInScope(failed[1]) {
			link(failed[1], spanKey)
		}
		if testInScope(failed[2]) {
			link(failed[2], spanKey)
		}
	}

	return bugs, links
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/jira/ -run TestListBugsHonorsTestKeys -v`
Expected: PASS.

- [ ] **Step 5: Run the package + the syncer (which calls `syncBugs`)**

Run: `go test ./internal/jira/ ./internal/syncer/`
Expected: ok for both. `syncBugs` already passes `AllTestKeys(profileID)` (the
synced, in-scope set) as `testKeys`, so it now narrows correctly with no change
to `engine.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/jira/bugs.go internal/jira/bugs_scope_test.go
git commit -m "Scope demo bug sync to the synced tests (honor testKeys) (item 4)"
```

---

## Task 6: Cross-project regression test + projectKeyOf test (Item 5b part 1)

**Files:**
- Create: `internal/testrepo/sankey_crossproject_test.go`

This task pins the cross-project behavior. The demo seed already provides
cross-project executions (`XRAYINT-TE-*`), so we assert the filter works through
the store. If the assertion fails, fix `projectKeyOf` / the threading in
`internal/testrepo/sankey.go` until it passes.

- [ ] **Step 1: Write the test**

```go
package testrepo

import "testing"

func TestProjectKeyOf(t *testing.T) {
	cases := map[string]string{
		"RND_P_4TFINT_05-123": "RND_P_4TFINT_05",
		"XRAYINT-TE-1":         "XRAYINT",
		"DEMO-9":               "DEMO",
		"NODASH":               "NODASH",
	}
	for in, want := range cases {
		if got := projectKeyOf(in); got != want {
			t.Errorf("projectKeyOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTraceabilityCrossProjectOnly(t *testing.T) {
	r := newTestRepo(t) // see helper note below
	const profileID = "p1"
	const projectKey = "DEMO"

	// Two executions: one in-project, one cross-project, each running one test.
	seedContainer(t, r, profileID, "DEMO-TE-1", "testexec", "Cycle 1", "Open")
	seedContainer(t, r, profileID, "XRAYINT-TE-1", "testexec", "Integration", "Open")
	seedContainerTest(t, r, profileID, "DEMO-TE-1", "DEMO-1", "PASS")
	seedContainerTest(t, r, profileID, "XRAYINT-TE-1", "DEMO-1", "FAIL")

	all, err := r.GetTraceabilitySankey(profileID, projectKey, nil, nil, false)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if !hasNode(all, "exec:DEMO-TE-1") || !hasNode(all, "exec:XRAYINT-TE-1") {
		t.Fatalf("unfiltered flow should contain both executions: %+v", all.Nodes)
	}

	cross, err := r.GetTraceabilitySankey(profileID, projectKey, nil, nil, true)
	if err != nil {
		t.Fatalf("cross: %v", err)
	}
	if hasNode(cross, "exec:DEMO-TE-1") {
		t.Errorf("cross-project-only should drop in-project execution DEMO-TE-1")
	}
	if !hasNode(cross, "exec:XRAYINT-TE-1") {
		t.Errorf("cross-project-only should keep cross-project execution XRAYINT-TE-1")
	}
}

func hasNode(s Sankey, id string) bool {
	for _, n := range s.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}
```

**Helper note:** reuse the package's existing test scaffolding. Before writing
the seed helpers, inspect a sibling test in `internal/testrepo` (e.g.
`grep -l "func.*Repository" internal/testrepo/*_test.go`) to find the established
`newTestRepo` / store-open helper and the direct-insert pattern. Implement
`seedContainer` and `seedContainerTest` as thin `r.db.Exec(...)` inserts into
`test_container` (`profile_id, jira_key, kind, summary, status`) and
`test_container_test` (`profile_id, container_key, test_key, run_status`),
matching the column names used in `sankey.go`. If a helper already exists that
seeds containers, prefer it over new inserts.

- [ ] **Step 2: Run it**

Run: `go test ./internal/testrepo/ -run 'TestProjectKeyOf|TestTraceabilityCrossProjectOnly' -v`
Expected: `TestProjectKeyOf` PASS. `TestTraceabilityCrossProjectOnly` should PASS
if the cross-project path works; if it FAILS, the test has caught the real bug —
fix `projectKeyOf` or the `crossProjectOnly` condition in `sankey.go` (line ~100)
until green, then note the fix in the commit.

- [ ] **Step 3: Run the full package**

Run: `go test ./internal/testrepo/`
Expected: ok.

- [ ] **Step 4: Commit**

```bash
git add internal/testrepo/sankey_crossproject_test.go
# include internal/testrepo/sankey.go too IF a fix was needed
git commit -m "Test (and confirm) cross-project execution traceability (item 5b)"
```

---

## Task 7: ExecutionsForPlans backend + binding (Item 5a part 1)

**Files:**
- Create: `internal/testrepo/executionsforplans.go`
- Create: `internal/testrepo/executionsforplans_test.go`
- Modify: `app.go`
- Modify (regenerate): `frontend/src/api.ts`, `frontend/wailsjs/**`

- [ ] **Step 1: Write the failing test**

```go
package testrepo

import "testing"

func TestExecutionsForPlans(t *testing.T) {
	r := newTestRepo(t)
	const p = "p1"

	seedContainer(t, r, p, "DEMO-TP-1", "testplan", "Plan 1", "Open")
	seedContainer(t, r, p, "DEMO-TP-2", "testplan", "Plan 2", "Open")
	seedContainer(t, r, p, "DEMO-TE-1", "testexec", "Exec 1", "Open")
	seedContainer(t, r, p, "DEMO-TE-2", "testexec", "Exec 2", "Open")

	// Plan 1 holds DEMO-1; Exec 1 runs DEMO-1; Exec 2 runs DEMO-2 (in Plan 2).
	seedContainerTest(t, r, p, "DEMO-TP-1", "DEMO-1", "")
	seedContainerTest(t, r, p, "DEMO-TP-2", "DEMO-2", "")
	seedContainerTest(t, r, p, "DEMO-TE-1", "DEMO-1", "PASS")
	seedContainerTest(t, r, p, "DEMO-TE-2", "DEMO-2", "FAIL")

	// For Plan 1 only Exec 1 shares a test.
	got, err := r.ExecutionsForPlans(p, []string{"DEMO-TP-1"})
	if err != nil {
		t.Fatalf("ExecutionsForPlans: %v", err)
	}
	if len(got) != 1 || got[0].Key != "DEMO-TE-1" {
		t.Fatalf("want [DEMO-TE-1], got %+v", got)
	}

	// Empty plan list returns all executions.
	all, err := r.ExecutionsForPlans(p, nil)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 executions, got %d", len(all))
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/testrepo/ -run TestExecutionsForPlans -v`
Expected: FAIL — `ExecutionsForPlans` undefined.

- [ ] **Step 3: Implement `internal/testrepo/executionsforplans.go`**

```go
package testrepo

import "fmt"

// ExecutionsForPlans returns the Test Executions that share at least one Test
// with the given Test Plans, ordered by key. An empty planKeys returns every
// Execution (same as ListContainers(profileID, "testexec")). Used to cascade the
// dashboard's Execution filter from the selected Plan(s) (FR-9, #5a).
func (r *Repository) ExecutionsForPlans(profileID string, planKeys []string) ([]Container, error) {
	keys := nonEmptyKeys(planKeys)
	if len(keys) == 0 {
		return r.ListContainers(profileID, "testexec")
	}

	q := `SELECT DISTINCT c.jira_key, c.kind, c.summary, c.status
	      FROM test_container c
	      JOIN test_container_test e
	        ON e.profile_id = c.profile_id AND e.container_key = c.jira_key
	      WHERE c.profile_id = ? AND c.kind = 'testexec'
	        AND e.test_key IN (
	          SELECT test_key FROM test_container_test
	          WHERE profile_id = ? AND container_key IN (` + sqlPlaceholders(len(keys)) + `))
	      ORDER BY c.jira_key`
	args := []any{profileID, profileID}
	for _, k := range keys {
		args = append(args, k)
	}

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("executions for plans: %w", err)
	}
	defer rows.Close()
	out := []Container{}
	for rows.Next() {
		var c Container
		if err := rows.Scan(&c.Key, &c.Kind, &c.Summary, &c.Status); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
```

Note: confirm `Container`'s scan order matches `ListContainers` in
`internal/testrepo/testrepo.go` (`jira_key, kind, summary, status`). If the
struct field order differs, scan into named locals instead.

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/testrepo/ -run TestExecutionsForPlans -v`
Expected: PASS.

- [ ] **Step 5: Add the `App` bindings in `app.go`** (place near `GetTraceabilitySankey`)

```go
// GetExecutionsForPlans returns the Test Executions sharing a Test with the
// given Test Plans, to cascade the dashboard's Execution filter (#5a). Empty
// planKeys returns all executions.
func (a *App) GetExecutionsForPlans(profileID string, planKeys []string) ([]testrepo.Container, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ExecutionsForPlans(profileID, planKeys)
}

// GetProfileProjectKey returns the active profile's Jira project key, used by the
// dashboard to flag cross-project bugs (#5b).
func (a *App) GetProfileProjectKey(profileID string) (string, error) {
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return "", err
	}
	return p.ProjectKey, nil
}
```

- [ ] **Step 6: Compile the backend**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 7: Regenerate Wails bindings**

Run: `"$USERPROFILE/go/bin/wails.exe" generate module`
Expected: `frontend/wailsjs/go/main/App.d.ts`, `App.js`, and `models.ts` updated
to include `GetExecutionsForPlans` and `GetProfileProjectKey`.

- [ ] **Step 8: Re-export in `frontend/src/api.ts`** — add the two new names to the
existing re-export block (follow the file's existing pattern; if it re-exports
`* from "../wailsjs/go/main/App"` no edit is needed — verify by grepping
`api.ts` for `GetTraceabilitySankey` and mirroring however that one is exported).

- [ ] **Step 9: Typecheck**

Run: `cd frontend; npm run build`
Expected: build succeeds.

- [ ] **Step 10: Commit**

```bash
git add internal/testrepo/executionsforplans.go internal/testrepo/executionsforplans_test.go app.go frontend/src/api.ts frontend/wailsjs
git commit -m "Add ExecutionsForPlans + GetProfileProjectKey bindings (items 5a/5b)"
```

---

## Task 8: Dashboard — Plan→Exec cascade + cross-project bug list (Items 5a & 5b part 2)

**Files:**
- Modify: `frontend/src/components/Dashboard.tsx`
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Add imports** — extend the api import block and the type import block:

```tsx
import {
  GetStatistics,
  GetTraceabilitySankey,
  GetRequirementTraceability,
  ListRequirementsWithCoverage,
  ListContainers,
  GetExecutionsForPlans,
  GetProfileProjectKey,
  ListBugsWithTests,
  BrowserOpenURL,
  errMsg,
} from "../api";
import type {
  Statistics,
  Bucket,
  Sankey,
  Container,
  RequirementCoverage,
  BugWithTests,
} from "../api";
```

- [ ] **Step 2: Add cross-project bug + project-key state** (after `const [crossProject, setCrossProject] = useState(false);`)

```tsx
  const [projectKey, setProjectKey] = useState("");
  const [crossBugs, setCrossBugs] = useState<BugWithTests[]>([]);
```

- [ ] **Step 3: Load the project key once per profile** — add an effect after the stats effect:

```tsx
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    GetProfileProjectKey(profileId)
      .then((k) => {
        if (!cancelled) setProjectKey(k ?? "");
      })
      .catch(() => {
        if (!cancelled) setProjectKey("");
      });
    return () => {
      cancelled = true;
    };
  }, [profileId]);
```

- [ ] **Step 4: Cascade the Execution options from the selected plans** — replace the existing "Filter options" effect (the one calling `ListContainers(profileId, "testplan")` and `ListContainers(profileId, "testexec")`) with two effects: plans load once; executions recompute from `planSel`.

```tsx
  // Test Plan options load with the profile.
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    setPlanSel([]);
    ListContainers(profileId, "testplan")
      .then((tp) => {
        if (!cancelled) setPlans(tp ?? []);
      })
      .catch((e) => console.error("list plans:", errMsg(e)));
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey, nonce]);

  // Execution options cascade from the selected plans (#5a): when plans are
  // chosen, only executions sharing a test with them are offered. Stale
  // execSel entries are pruned so the Sankey filter stays valid.
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    GetExecutionsForPlans(profileId, planSel)
      .then((te) => {
        if (cancelled) return;
        const opts = te ?? [];
        setExecs(opts);
        setExecSel((cur) => cur.filter((k) => opts.some((c) => c.key === k)));
      })
      .catch((e) => console.error("executions for plans:", errMsg(e)));
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey, nonce, planSel]);
```

- [ ] **Step 5: Load cross-project bugs when the toggle is on** — add an effect:

```tsx
  // Cross-project bugs (#5b): defects linked to this profile's tests but filed
  // in a different Jira project. Only fetched when the cross-project toggle is on.
  useEffect(() => {
    if (!profileId || !crossProject) {
      setCrossBugs([]);
      return;
    }
    let cancelled = false;
    ListBugsWithTests(profileId)
      .then((bs) => {
        if (cancelled) return;
        const pk = projectKey.trim();
        setCrossBugs(
          (bs ?? []).filter((b) => pk && b.projectKey && b.projectKey !== pk),
        );
      })
      .catch((e) => console.error("cross-project bugs:", errMsg(e)));
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey, nonce, crossProject, projectKey]);
```

- [ ] **Step 6: Render the cross-project bug list** — inside the traceability `stat-panel` (the `stats.testExecutions > 0` block), immediately after the `<SankeyChart .../>`'s closing parenthesis/ternary (after the `)}` that ends the `sankeyErr ? ... : ...` block) and before the panel's closing `</div>`, add:

```tsx
          {crossProject && (
            <div className="crossproject-bugs">
              <h5>
                Cross-project bugs
                <span className="stat-panel-sub">
                  defects filed outside {projectKey || "this project"} but linked
                  to its tests
                </span>
              </h5>
              {crossBugs.length === 0 ? (
                <p className="muted">No cross-project bugs linked.</p>
              ) : (
                <ul className="crossproject-bug-list">
                  {crossBugs.map((b) => (
                    <li key={b.key}>
                      <button
                        className="mono bug-link-key"
                        onClick={() => openCrossBug(b.key)}
                        title={`Open ${b.key} in Jira`}
                      >
                        {b.key}
                      </button>
                      <span className="muted">{b.projectKey}</span>
                      {b.status && <span className="status-pill">{b.status}</span>}
                      <span className="crossproject-bug-summary">
                        {b.summary || "(no summary)"}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
```

- [ ] **Step 7: Add the `openCrossBug` helper** — inside the `Dashboard` component, before the `return`:

```tsx
  function openCrossBug(key: string) {
    if (key.startsWith("NEW-")) return;
    GetProfileProjectKey(profileId).catch(() => {});
    BrowserOpenURL(`/browse/${key}`); // placeholder; replaced below
  }
```

Then replace that placeholder body — the dashboard needs the Jira base URL. The
simplest robust approach mirrors `BugsPanel`: accept the base URL from the page
that renders `Dashboard`. Check how `Dashboard` is instantiated in
`frontend/src/App.tsx`; if a `jiraUrl` prop is already passed to sibling views
(it is, e.g. to `BugsPanel`/`ContainersView`), add an optional `jiraUrl?: string`
to `Props` and use:

```tsx
  function openCrossBug(key: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    const isDemo = /^(demo$|demo:|mock:)/i.test((jiraUrl ?? "").trim());
    if (base && !isDemo && !key.startsWith("NEW-")) {
      BrowserOpenURL(`${base}/browse/${key}`);
    }
  }
```

Update `interface Props` to include `jiraUrl?: string;`, destructure it in the
component signature, and pass it from `App.tsx` where `<Dashboard .../>` is
rendered (mirror the `jiraUrl={...}` already passed to `ContainersView`).

- [ ] **Step 8: Add styles to `frontend/src/App.css`**

```css
.crossproject-bugs {
  margin-top: 12px;
  border-top: 1px solid var(--border, #2a2f3a);
  padding-top: 10px;
}
.crossproject-bugs h5 {
  margin: 0 0 6px;
}
.crossproject-bug-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.crossproject-bug-list li {
  display: flex;
  align-items: center;
  gap: 8px;
}
.crossproject-bug-summary {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

- [ ] **Step 9: Typecheck**

Run: `cd frontend; npm run build`
Expected: build succeeds.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/components/Dashboard.tsx frontend/src/App.tsx frontend/src/App.css
git commit -m "Dashboard: cascade Execution filter from plans; list cross-project bugs (items 5a/5b)"
```

---

## Task 9: Full verification

- [ ] **Step 1: Backend tests + build**

Run: `go build ./... && go test ./...`
Expected: build exit 0; all packages ok.

- [ ] **Step 2: Frontend build**

Run: `cd frontend; npm run build`
Expected: tsc + vite build succeed.

- [ ] **Step 3: Manual demo-mode smoke (wails dev)**

Run: `wails dev`
Verify each:
- Requirements / Preconditions / Bugs panels show a Sort control; toggling field + direction reorders the list; key sort is numeric (…-9 before …-10).
- Containers view: keyword + status filters narrow the picker; sort reorders it; the "N of M" count updates; board column headers sort the rows.
- Sankey: selecting a Test Plan narrows the Execution dropdown to related executions; clearing restores all.
- Sankey "Cross-project only": shows only cross-project executions and reveals the "Cross-project bugs" list with working hyperlinks (open in browser).
- Sync a demo profile with a ScopeJQL that narrows tests: bug count tracks scope (full scope reproduces the seed).

- [ ] **Step 4: Final commit if any manual fixes were needed**

```bash
git add -A -- ':!CLAUDE.md'
git commit -m "Polish from demo-mode verification"
```

---

## Self-review notes (addressed)

- **DTO fields:** sort fields are limited to those the projected DTOs actually
  expose (no `updated` on `RequirementCoverage`/`PreconditionUsage`/`BugWithTests`).
- **Container picker:** `ContainersView` uses an inline `<select>` (not
  `ContainerList.tsx`); Tasks 4 targets that select.
- **Item 4:** the real defect is demo `ListBugs` discarding `testKeys`;
  `syncBugs` already passes the in-scope `AllTestKeys`, so no `engine.go` change.
- **Item 5b executions:** demo cross-project executions already exist; Task 6 is
  a regression test that fixes `sankey.go` only if it proves broken.
- **Bindings:** Tasks 7 adds `App` methods → regenerate `wailsjs` + `api.ts`.
