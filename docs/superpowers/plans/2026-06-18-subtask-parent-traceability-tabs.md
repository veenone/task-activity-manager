# Sub-task parent traceability Sankey + Traceability view — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a third traceability Sankey (Parent issue → Test Execution → Run result, for sub-task executions) and move all three Sankeys into a dedicated, tabbed Traceability top-level view with its own menu entry.

**Architecture:** A new `GetSubTaskTraceability` store method mirrors the existing balanced-flow `GetTraceabilitySankey`. The existing 3-layer `SankeyChart` is reused for the new chart via two new optional props (column headers + empty-state hints). A new self-contained `TraceabilityTabs` view component holds the three Sankeys (the logic moves out of `Dashboard`), and a new `traceability` view is wired into the top-bar tabs, the `menuActions` map, and the native `main.go` View menu.

**Tech Stack:** Go (`modernc.org/sqlite`), Wails v2, React + TypeScript (Vite).

**Spec:** `docs/superpowers/specs/2026-06-18-subtask-parent-traceability-tabs-design.md`

**Conventions:**
- Backend is TDD with Go tests. The frontend has **no test runner** (CLAUDE.md) — verify with `cd frontend && npm run build` (tsc + vite) plus manual demo checks.
- Regenerate Wails bindings with `"$USERPROFILE/go/bin/wails.exe" generate module` after the new `App` method (Task 2).
- Do NOT stage `CLAUDE.md` or build-churn (`frontend/package.json.md5`). Stage only the files each step names.
- The Outline doc updates are a controller (non-subagent) step after the build is green — see the final note.

---

## File map

**Create:**
- `internal/testrepo/subtasksankey.go` — `GetSubTaskTraceability`.
- `internal/testrepo/subtasksankey_test.go` — its test.
- `frontend/src/components/TraceabilityTabs.tsx` — the tabbed traceability view.

**Modify:**
- `app.go` — `GetSubTaskTraceability` binding.
- `frontend/src/api.ts` + `frontend/wailsjs/**` — regenerated.
- `frontend/src/components/SankeyChart.tsx` — `columns` + `emptyHint`/`filteredHint` props.
- `frontend/src/components/Dashboard.tsx` — remove the two Sankey panels + their state/effects/imports.
- `frontend/src/App.tsx` — `traceability` view: tab, render branch, `menuActions` entry, import.
- `main.go` — native View → Traceability menu item.

---

## Task 1: `GetSubTaskTraceability` store method

**Files:**
- Create: `internal/testrepo/subtasksankey.go`
- Create: `internal/testrepo/subtasksankey_test.go`

- [ ] **Step 1: Write the failing test**

```go
package testrepo

import "testing"

func TestGetSubTaskTraceability(t *testing.T) {
	r := newTestRepo(t) // shared helper in sankey_crossproject_test.go
	const p = "p1"

	// Two sub-task execs under one parent, one standalone exec (excluded).
	seedContainer(t, r, p, "DEMO-STE-1", "testexec", "Sub 1", "Open")
	seedContainer(t, r, p, "DEMO-STE-2", "testexec", "Sub 2", "Open")
	seedContainer(t, r, p, "DEMO-TE-9", "testexec", "Standalone", "Open")
	setContainerParent(t, r, p, "DEMO-STE-1", "DEMO-S-1")
	setContainerParent(t, r, p, "DEMO-STE-2", "DEMO-S-1")
	// DEMO-TE-9 keeps parent_key = '' (standalone).

	seedContainerTest(t, r, p, "DEMO-STE-1", "DEMO-1", "PASS")
	seedContainerTest(t, r, p, "DEMO-STE-1", "DEMO-2", "FAIL")
	seedContainerTest(t, r, p, "DEMO-STE-2", "DEMO-3", "PASS")
	seedContainerTest(t, r, p, "DEMO-TE-9", "DEMO-4", "PASS") // standalone, excluded

	sk, err := r.GetSubTaskTraceability(p, nil)
	if err != nil {
		t.Fatalf("GetSubTaskTraceability: %v", err)
	}

	// 3 memberships under sub-task execs (the standalone one is excluded).
	sumLayer := func(layer int) int {
		n := 0
		for _, nd := range sk.Nodes {
			if nd.Layer == layer {
				n += nd.Value
			}
		}
		return n
	}
	if sumLayer(0) != 3 || sumLayer(1) != 3 || sumLayer(2) != 3 {
		t.Fatalf("layers should each total 3 memberships, got %d/%d/%d", sumLayer(0), sumLayer(1), sumLayer(2))
	}
	if !hasNode(sk, "parent:DEMO-S-1") {
		t.Errorf("missing parent node")
	}
	if hasNode(sk, "exec:DEMO-TE-9") {
		t.Errorf("standalone execution must be excluded")
	}

	// Parent filter to a non-existent parent yields an empty (not error) result.
	empty, err := r.GetSubTaskTraceability(p, []string{"NOPE-1"})
	if err != nil {
		t.Fatalf("filtered: %v", err)
	}
	if len(empty.Nodes) != 0 {
		t.Errorf("unknown parent filter should yield no nodes, got %d", len(empty.Nodes))
	}
}

// setContainerParent sets parent_key on a seeded container.
func setContainerParent(t *testing.T, r *Repository, profileID, key, parent string) {
	t.Helper()
	if _, err := r.db.Exec(
		`UPDATE test_container SET parent_key = ? WHERE profile_id = ? AND jira_key = ?`,
		parent, profileID, key); err != nil {
		t.Fatalf("set parent on %s: %v", key, err)
	}
}
```

Note: `hasNode` is the helper already defined in `sankey_crossproject_test.go` (same package), so do not redefine it. `newTestRepo`, `seedContainer`, `seedContainerTest` are also from there.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/testrepo/ -run TestGetSubTaskTraceability -v`
Expected: FAIL — `GetSubTaskTraceability` undefined.

- [ ] **Step 3: Implement `internal/testrepo/subtasksankey.go`**

```go
package testrepo

import (
	"fmt"
	"sort"
)

// GetSubTaskTraceability builds a Parent issue -> Test Execution -> run status
// flow over sub-task Test Executions only (kind = 'testexec' with a non-empty
// parent_key). Each sub-task execution has exactly one parent, so the parent is
// layer 0, the execution layer 1, the run status layer 2. The flow unit is a
// membership (a test's run in a sub-task execution); each adds 1 across the
// three layers, so the diagram balances. parentFilters narrows to chosen
// parents; empty means all. Computed entirely from the local store.
func (r *Repository) GetSubTaskTraceability(profileID string, parentFilters []string) (Sankey, error) {
	out := Sankey{Nodes: []SankeyNode{}, Links: []SankeyLink{}}

	summaryByKey, err := r.containerSummaries(profileID)
	if err != nil {
		return out, err
	}

	q := `SELECT c.parent_key, l.container_key, l.run_status
		 FROM test_container_test l
		 JOIN test_container c
		   ON c.profile_id = l.profile_id AND c.jira_key = l.container_key
		 WHERE l.profile_id = ? AND c.kind = 'testexec' AND c.parent_key != ''`
	args := []any{profileID}
	if parents := nonEmptyKeys(parentFilters); len(parents) > 0 {
		q += " AND c.parent_key IN (" + sqlPlaceholders(len(parents)) + ")"
		for _, p := range parents {
			args = append(args, p)
		}
	}
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return out, fmt.Errorf("read sub-task runs: %w", err)
	}
	defer rows.Close()

	parentExec := map[[2]string]int{}
	execStatus := map[[2]string]int{}
	value := map[string]int{}
	label := map[string]string{}
	layer := map[string]int{}
	note := func(id, lbl string, lyr, add int) {
		value[id] += add
		label[id] = lbl
		layer[id] = lyr
	}

	for rows.Next() {
		var parentKey, execKey, runStatus string
		if err := rows.Scan(&parentKey, &execKey, &runStatus); err != nil {
			return out, err
		}
		status := runStatus
		if status == "" {
			status = "(none)"
		}
		parentID := "parent:" + parentKey
		execID := "exec:" + execKey
		statusID := "status:" + status

		note(parentID, parentKey, 0, 1)
		note(execID, orKey(summaryByKey[execKey], execKey), 1, 1)
		note(statusID, status, 2, 1)
		parentExec[[2]string{parentID, execID}]++
		execStatus[[2]string{execID, statusID}]++
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	for id, lbl := range label {
		out.Nodes = append(out.Nodes, SankeyNode{ID: id, Label: lbl, Layer: layer[id], Value: value[id]})
	}
	sort.Slice(out.Nodes, func(i, j int) bool {
		if out.Nodes[i].Layer != out.Nodes[j].Layer {
			return out.Nodes[i].Layer < out.Nodes[j].Layer
		}
		if out.Nodes[i].Value != out.Nodes[j].Value {
			return out.Nodes[i].Value > out.Nodes[j].Value
		}
		return out.Nodes[i].ID < out.Nodes[j].ID
	})
	out.Links = append(out.Links, flatten(parentExec)...)
	out.Links = append(out.Links, flatten(execStatus)...)
	return out, nil
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/testrepo/ -run TestGetSubTaskTraceability -v`
Expected: PASS.

- [ ] **Step 5: Run the package**

Run: `go test ./internal/testrepo/`
Expected: ok.

- [ ] **Step 6: Commit**

```bash
git add internal/testrepo/subtasksankey.go internal/testrepo/subtasksankey_test.go
git commit -m "testrepo: GetSubTaskTraceability (parent -> exec -> status)"
```

## Task 2: App binding + regenerate bindings

**Files:**
- Modify: `app.go`
- Regenerate: `frontend/wailsjs/**`, `frontend/src/api.ts`

- [ ] **Step 1: Add the binding in `app.go`** (place right after `GetTraceabilitySankey`)

```go
// GetSubTaskTraceability returns the Parent -> Execution -> run-status flow over
// sub-task Test Executions (FR-9). parentFilters narrows to chosen parent
// issues; empty includes all.
func (a *App) GetSubTaskTraceability(profileID string, parentFilters []string) (testrepo.Sankey, error) {
	if err := a.requireStore(); err != nil {
		return testrepo.Sankey{}, err
	}
	return a.repo.GetSubTaskTraceability(profileID, parentFilters)
}
```

- [ ] **Step 2: Build the backend**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Regenerate Wails bindings**

Run: `"$USERPROFILE/go/bin/wails.exe" generate module`
Expected: `frontend/wailsjs/go/main/App.d.ts` declares
`GetSubTaskTraceability(arg1:string,arg2:Array<string>):Promise<testrepo.Sankey>`.

- [ ] **Step 4: Re-export in `frontend/src/api.ts`** — add `GetSubTaskTraceability` to the named re-export block, right after `GetTraceabilitySankey` (mirror that line exactly).

- [ ] **Step 5: Typecheck**

Run: `cd frontend && npm run build`
Expected: build succeeds.

- [ ] **Step 6: Commit**

```bash
git add app.go frontend/src/api.ts frontend/wailsjs
git commit -m "Add GetSubTaskTraceability binding"
```

## Task 3: `SankeyChart` column + empty-state props

**Files:**
- Modify: `frontend/src/components/SankeyChart.tsx`

- [ ] **Step 1: Extend the `Props` interface**

```tsx
interface Props {
  data: Sankey;
  // filtered tells the empty state to read as "filter matched nothing" rather
  // than "no data synced yet" — the two look identical otherwise.
  filtered?: boolean;
  onClearFilter?: () => void;
  // Column headers (layer 0/1/2). Defaults to the Plan/Execution/Status flow.
  columns?: [string, string, string];
  // Empty-state copy, so a reused chart (e.g. sub-task) reads correctly.
  emptyHint?: string;
  filteredHint?: string;
}
```

- [ ] **Step 2: Read the props with defaults** — in the component signature and body:

Change the signature:
```tsx
export function SankeyChart({
  data,
  filtered,
  onClearFilter,
  columns,
  emptyHint,
  filteredHint,
}: Props) {
```
Add right after the existing `const empty = !data || data.nodes.length === 0;` line:
```tsx
  const cols = columns ?? ["Test Plans", "Test Executions", "Run Status"];
  const emptyMsg =
    emptyHint ??
    "No execution data to trace yet — sync test executions to populate the traceability flow.";
  const filteredMsg =
    filteredHint ??
    "No execution runs match the selected Test Plan / Execution. The chosen plan's tests may not be in any execution yet.";
```

- [ ] **Step 3: Use the props in the empty state** — replace the two hardcoded empty-state paragraphs:

```tsx
      {empty ? (
        filtered ? (
          <p className="muted sankey-empty">
            {filteredMsg}{" "}
            {onClearFilter && (
              <button className="btn btn-ghost sankey-clear" onClick={onClearFilter}>
                Clear filter
              </button>
            )}
          </p>
        ) : (
          <p className="muted sankey-empty">{emptyMsg}</p>
        )
      ) : !layout ? (
```

- [ ] **Step 4: Use the props in the column heads** — replace the three `sankey-col-head` spans' text:

```tsx
            <span
              className="sankey-col-head"
              style={{ left: 0, width: layout.planX - 4, textAlign: "right" }}
            >
              {cols[0]}
            </span>
            <span className="sankey-col-head" style={{ left: layout.execX }}>
              {cols[1]}
            </span>
            <span className="sankey-col-head" style={{ left: layout.statusX }}>
              {cols[2]}
            </span>
```

- [ ] **Step 5: Typecheck**

Run: `cd frontend && npm run build`
Expected: build succeeds (existing callers omit the new optional props — unchanged behavior).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/SankeyChart.tsx
git commit -m "SankeyChart: parameterize column headers and empty-state copy"
```

## Task 4: `TraceabilityTabs` view component

**Files:**
- Create: `frontend/src/components/TraceabilityTabs.tsx`

- [ ] **Step 1: Create the component (full file)**

```tsx
import { useEffect, useState } from "react";
import {
  GetStatistics,
  GetTraceabilitySankey,
  GetRequirementTraceability,
  GetSubTaskTraceability,
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
  Sankey,
  Container,
  RequirementCoverage,
  BugWithTests,
} from "../api";
import { SankeyChart } from "./SankeyChart";
import { RequirementSankey } from "./RequirementSankey";
import { MultiSelect } from "./MultiSelect";

interface Props {
  profileId: string;
  refreshKey: number;
  jiraUrl?: string;
}

type Tab = "req" | "exec" | "subtask";

// TraceabilityTabs is the dedicated Traceability view: three Sankeys
// (requirement coverage, plan -> execution -> status, and sub-task
// parent -> execution -> status) behind a tab bar, each with its own filters.
// Computed entirely from the local store; recomputes on refreshKey.
export function TraceabilityTabs({ profileId, refreshKey, jiraUrl }: Props) {
  const [tab, setTab] = useState<Tab>("exec");
  const [stats, setStats] = useState<Statistics | null>(null);

  // Requirement traceability.
  const [reqSankey, setReqSankey] = useState<Sankey | null>(null);
  const [reqSankeyErr, setReqSankeyErr] = useState("");
  const [reqSel, setReqSel] = useState<string[]>([]);
  const [reqOptions, setReqOptions] = useState<RequirementCoverage[]>([]);

  // Plan/Execution traceability + cross-project bugs.
  const [sankey, setSankey] = useState<Sankey | null>(null);
  const [sankeyErr, setSankeyErr] = useState("");
  const [plans, setPlans] = useState<Container[]>([]);
  const [execs, setExecs] = useState<Container[]>([]);
  const [planSel, setPlanSel] = useState<string[]>([]);
  const [execSel, setExecSel] = useState<string[]>([]);
  const [crossProject, setCrossProject] = useState(false);
  const [projectKey, setProjectKey] = useState("");
  const [crossBugs, setCrossBugs] = useState<BugWithTests[]>([]);

  // Sub-task (parent) traceability.
  const [subSankey, setSubSankey] = useState<Sankey | null>(null);
  const [subSankeyErr, setSubSankeyErr] = useState("");
  const [parents, setParents] = useState<string[]>([]);
  const [parentSel, setParentSel] = useState<string[]>([]);

  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    GetStatistics(profileId)
      .then((s) => {
        if (!cancelled) setStats(s);
      })
      .catch(() => {
        if (!cancelled) setStats(null);
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey]);

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

  // Requirement filter options.
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    ListRequirementsWithCoverage(profileId)
      .then((rs) => {
        if (!cancelled) setReqOptions(rs ?? []);
      })
      .catch(() => {
        if (!cancelled) setReqOptions([]);
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey]);

  // Test Plan options.
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
  }, [profileId, refreshKey]);

  // Parent options: distinct parent keys of the synced sub-task executions.
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    setParentSel([]);
    ListContainers(profileId, "testexec")
      .then((te) => {
        if (cancelled) return;
        const ps = Array.from(
          new Set((te ?? []).filter((c) => c.parentKey).map((c) => c.parentKey)),
        ).sort();
        setParents(ps);
      })
      .catch((e) => console.error("list executions:", errMsg(e)));
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey]);

  // Execution options cascade from the selected plans; prune stale execSel.
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
  }, [profileId, refreshKey, planSel]);

  // Cross-project bugs (only when the toggle is on).
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
  }, [profileId, refreshKey, crossProject, projectKey]);

  // Requirement Sankey.
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    setReqSankeyErr("");
    GetRequirementTraceability(profileId, reqSel)
      .then((sk) => {
        if (!cancelled) setReqSankey(sk);
      })
      .catch((e) => {
        if (cancelled) return;
        setReqSankeyErr(errMsg(e));
        console.error("requirement traceability:", errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey, reqSel]);

  // Plan/Execution Sankey.
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    setSankeyErr("");
    GetTraceabilitySankey(profileId, planSel, execSel, crossProject)
      .then((sk) => {
        if (!cancelled) setSankey(sk);
      })
      .catch((e) => {
        if (cancelled) return;
        setSankeyErr(errMsg(e));
        console.error("traceability:", errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey, planSel, execSel, crossProject]);

  // Sub-task (parent) Sankey.
  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    setSubSankeyErr("");
    GetSubTaskTraceability(profileId, parentSel)
      .then((sk) => {
        if (!cancelled) setSubSankey(sk);
      })
      .catch((e) => {
        if (cancelled) return;
        setSubSankeyErr(errMsg(e));
        console.error("sub-task traceability:", errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey, parentSel]);

  function openCrossBug(key: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    const isDemo = /^(demo$|demo:|mock:)/i.test((jiraUrl ?? "").trim());
    if (base && !isDemo && !key.startsWith("NEW-")) {
      BrowserOpenURL(`${base}/browse/${key}`);
    }
  }

  if (!stats) {
    return <div className="dashboard muted">Loading…</div>;
  }
  if (stats.total === 0) {
    return (
      <div className="dashboard">
        <p className="muted">
          No tests cached yet. Run a sync to populate traceability.
        </p>
      </div>
    );
  }

  return (
    <div className="dashboard">
      <div className="containers-mode trace-tabs">
        <button
          className={`seg-btn${tab === "req" ? " seg-btn-active" : ""}`}
          onClick={() => setTab("req")}
        >
          Requirement
        </button>
        <button
          className={`seg-btn${tab === "exec" ? " seg-btn-active" : ""}`}
          onClick={() => setTab("exec")}
        >
          Execution
        </button>
        <button
          className={`seg-btn${tab === "subtask" ? " seg-btn-active" : ""}`}
          onClick={() => setTab("subtask")}
        >
          Sub-task
        </button>
      </div>

      {tab === "req" && (
        <div className="stat-panel sankey-panel">
          <div className="sankey-head">
            <h4>
              Requirement traceability
              <span className="stat-panel-sub">
                how each requirement flows through coverage and Test plans to run
                results
              </span>
            </h4>
            {reqOptions.length > 0 && (
              <label className="sankey-filter">
                <span className="muted">Requirements</span>
                <MultiSelect
                  allLabel="All requirements"
                  title="Filter by one or more requirements"
                  selected={reqSel}
                  onChange={setReqSel}
                  options={reqOptions.map((r) => ({
                    value: r.key,
                    label: r.summary ? `${r.key} — ${r.summary}` : r.key,
                  }))}
                />
              </label>
            )}
          </div>
          {reqSankeyErr ? (
            <p className="error-text sankey-empty">
              Couldn&apos;t build the requirement traceability flow: {reqSankeyErr}
            </p>
          ) : stats.byCoverage.length === 0 ? (
            <p className="muted sankey-empty">
              No requirement coverage yet. Add a requirement source (Requirements
              tab → Sources), link requirements to tests, then sync — the flow
              from requirement → coverage → Test plan → test result appears here.
            </p>
          ) : (
            <RequirementSankey data={reqSankey ?? { nodes: [], links: [] }} />
          )}
        </div>
      )}

      {tab === "exec" && (
        <div className="stat-panel sankey-panel">
          <div className="sankey-head">
            <h4>
              Execution traceability
              <span className="stat-panel-sub">
                how test runs flow from plans through executions to outcomes
              </span>
            </h4>
            <div className="sankey-filters">
              <MultiSelect
                allLabel="All plans"
                title="Filter by one or more Test Plans"
                selected={planSel}
                onChange={setPlanSel}
                options={plans.map((p) => ({
                  value: p.key,
                  label: p.summary ? `${p.key} — ${p.summary}` : p.key,
                }))}
              />
              <MultiSelect
                allLabel={`All executions (${execs.length})`}
                title="Filter by one or more Test Executions"
                selected={execSel}
                onChange={setExecSel}
                options={execs.map((x) => ({
                  value: x.key,
                  label: x.summary ? `${x.key} — ${x.summary}` : x.key,
                }))}
              />
              <label
                className="sankey-crossproject"
                title="Show only Test Plans in this project whose runs are in a different project"
              >
                <input
                  type="checkbox"
                  checked={crossProject}
                  onChange={(e) => setCrossProject(e.target.checked)}
                />
                Cross-project only
              </label>
              {(planSel.length > 0 || execSel.length > 0 || crossProject) && (
                <button
                  className="btn btn-ghost sankey-clear"
                  onClick={() => {
                    setPlanSel([]);
                    setExecSel([]);
                    setCrossProject(false);
                  }}
                  title="Clear filters"
                >
                  ✕ Clear
                </button>
              )}
            </div>
          </div>
          {sankeyErr ? (
            <p className="error-text sankey-empty">
              Couldn&apos;t build the traceability flow: {sankeyErr}
            </p>
          ) : (
            <SankeyChart
              data={sankey ?? { nodes: [], links: [] }}
              filtered={planSel.length > 0 || execSel.length > 0 || crossProject}
              onClearFilter={() => {
                setPlanSel([]);
                setExecSel([]);
                setCrossProject(false);
              }}
            />
          )}
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
        </div>
      )}

      {tab === "subtask" && (
        <div className="stat-panel sankey-panel">
          <div className="sankey-head">
            <h4>
              Sub-task traceability
              <span className="stat-panel-sub">
                how sub-task executions flow from their parent issue through
                executions to run results
              </span>
            </h4>
            {parents.length > 0 && (
              <label className="sankey-filter">
                <span className="muted">Parents</span>
                <MultiSelect
                  allLabel={`All parents (${parents.length})`}
                  title="Filter by one or more parent issues"
                  selected={parentSel}
                  onChange={setParentSel}
                  options={parents.map((p) => ({ value: p, label: p }))}
                />
              </label>
            )}
          </div>
          {subSankeyErr ? (
            <p className="error-text sankey-empty">
              Couldn&apos;t build the sub-task traceability flow: {subSankeyErr}
            </p>
          ) : (
            <SankeyChart
              data={subSankey ?? { nodes: [], links: [] }}
              filtered={parentSel.length > 0}
              onClearFilter={() => setParentSel([])}
              columns={["Parent issues", "Test Executions", "Run Status"]}
              emptyHint="No sub-task executions to trace yet — sync a project that has sub-task Test Executions (or a demo profile)."
              filteredHint="No sub-task execution runs match the selected parent."
            />
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Add a small style for the tab bar spacing** in `frontend/src/App.css`:

```css
.trace-tabs {
  margin-bottom: 14px;
}
```

- [ ] **Step 3: Typecheck**

Run: `cd frontend && npm run build`
Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/TraceabilityTabs.tsx frontend/src/App.css
git commit -m "Add TraceabilityTabs view (requirement / execution / sub-task)"
```

## Task 5: Strip the Sankeys from Dashboard + wire the Traceability view & menu

**Files:**
- Modify: `frontend/src/components/Dashboard.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `main.go`

- [ ] **Step 1: Remove the two Sankey panels from `Dashboard.tsx`**

Delete the entire **Requirement traceability** `stat-panel sankey-panel` block (the `<div className="stat-panel sankey-panel"> … </div>` that begins with `<h4>Requirement traceability` ) and the entire **Traceability** block (the `{stats.testExecutions > 0 && ( <div className="stat-panel sankey-panel"> … </div> )}` that follows, including the cross-project bugs list). They sit between the "Requirement coverage" `BarPanel` and `<TrendPanel … />`.

- [ ] **Step 2: Remove the now-unused state, effects, helper, and imports from `Dashboard.tsx`**

Delete these state hooks: `sankey`, `reqSankey`, `reqSankeyErr`, `reqSel`, `reqOptions`, `plans`, `execs`, `planSel`, `execSel`, `crossProject`, `sankeyErr`, `projectKey`, `crossBugs`. Keep `stats`, `error`, `loading`, `nonce`.

Delete these effects (each is a `useEffect` block): the project-key effect, the test-plan-options effect, the execution-cascade effect, the cross-project-bugs effect, the plan/exec Sankey effect, the requirement Sankey effect, and the requirement-list effect. Keep the `GetStatistics` effect.

Delete the `openCrossBug` function.

In the import block, drop the now-unused names: `GetTraceabilitySankey`, `GetRequirementTraceability`, `ListRequirementsWithCoverage`, `ListContainers`, `GetExecutionsForPlans`, `GetProfileProjectKey`, `ListBugsWithTests`, `BrowserOpenURL` (from `../api`); `Sankey`, `Container`, `RequirementCoverage`, `BugWithTests` (from the type import); and the `SankeyChart`, `RequirementSankey`, `MultiSelect` component imports. Keep `GetStatistics`, `errMsg`, `Statistics`, `Bucket`, `DuplicatesCard`.

Note: the `jiraUrl` prop on `Dashboard` is now only used by the cross-project bugs (removed). Leave the `Props` interface as-is (App.tsx still passes `jiraUrl`); an unused optional prop is harmless and avoids touching the call site. After the edits, run `cd frontend && npm run build` and fix any "declared but never used" tsc errors by removing exactly those leftover symbols.

- [ ] **Step 3: Add the `traceability` view to `App.tsx`**

a) Import the component near the other component imports:
```tsx
import { TraceabilityTabs } from "./components/TraceabilityTabs";
```

b) Add the view to the `view` state's union type. Find the `useState` for `view` (it lists `"browse" | "preconditions" | … | "plans"`) and add `| "traceability"`.

c) Add the `menuActions` entry — in the `menuActions.current = { … }` object, after `"menu:view-dashboard"`:
```tsx
    "menu:view-traceability": () => setView("traceability"),
```

d) Add the render branch — directly before the `) : view === "plans" ? (` branch:
```tsx
      ) : view === "traceability" ? (
        <main className="content content-dashboard">
          <TraceabilityTabs
            profileId={activeId}
            refreshKey={refreshKey}
            jiraUrl={activeProfile?.jiraUrl ?? ""}
          />
        </main>
```
(Use the same `activeId` / `refreshKey` / `activeProfile` identifiers the sibling branches use — confirm by reading the `view === "dashboard"` branch just above.)

e) Add the top-bar tab — directly after the Dashboard `<button … >Dashboard</button>` in the `view-tabs` nav:
```tsx
          <button
            className={`view-tab${view === "traceability" ? " view-tab-active" : ""}`}
            onClick={() => setView("traceability")}
          >
            Traceability
          </button>
```

- [ ] **Step 4: Add the native menu item in `main.go`**

In `appMenu`, in the `view` submenu, after the Dashboard line, add:
```go
	view.AddText("Traceability", nil, emit("menu:view-traceability"))
```

- [ ] **Step 5: Build everything**

Run: `go build ./... && cd frontend && npm run build`
Expected: backend exit 0; frontend tsc + vite succeed.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/Dashboard.tsx frontend/src/App.tsx main.go
git commit -m "Move traceability into its own view with a menu entry"
```

## Task 6: Full verification

- [ ] **Step 1:** `go build ./... && go test ./...` — build exit 0, all packages ok.
- [ ] **Step 2:** `cd frontend && npm run build` — succeeds.
- [ ] **Step 3:** Manual demo smoke (`wails dev`): the top bar and native View menu both show **Traceability**; the view opens on the **Execution** tab; switching to **Requirement** and **Sub-task** shows each chart; the Sub-task chart shows Parent → Execution → Run Status columns with a Parent filter; the Dashboard no longer shows the two Sankeys but keeps its tiles/bars/duplicates.

---

## Self-review notes (addressed)

- **No FE test runner:** frontend tasks verify via tsc/vite build + manual demo.
- **Renderer reuse:** `SankeyChart` gains only optional props (existing callers unchanged); `RequirementSankey` untouched.
- **Move, not rewrite:** the requirement + plan/exec + cross-project-bug logic moves verbatim into `TraceabilityTabs`; `nonce` is dropped (the view recomputes on `refreshKey`).
- **Shared test helpers:** the new Go test reuses `newTestRepo` / `seedContainer` / `seedContainerTest` / `hasNode`; only `setContainerParent` is new.
- **Type consistency:** `GetSubTaskTraceability(profileID, parentFilters)` ↔ `Promise<Sankey>`; the `columns` tuple is `[string,string,string]` everywhere.

## Controller follow-up (not a subagent task)

After the branch is merged and verified, update the Outline docs (done via the Outline MCP, not in-repo): add a **Traceability** entry to the XTM **Supported Views** doc, and note the sub-task traceability Sankey + the new top-level view in the **Feature List**.
