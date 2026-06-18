# Sub-task Test Executions & Jira color-macro rendering — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render Jira `{color:#hex}…{color}` macros in description/step text, and sync + surface Xray sub-task Test Executions (executions that hang off a parent issue) using a unified `testexec` model with a `parent_key`.

**Architecture:** Feature B is a frontend-only rehype plugin over a pure parser; no new deps, no raw-HTML. Feature A adds two additive columns to `test_container` (schema v23), carries `ParentKey`/`IssueType` through the jira → store → bindings → UI path, seeds sub-task executions in demo mode, and adds an execution-type filter + a clickable parent badge in the Containers view. Both reuse all existing execution logic.

**Tech Stack:** Go (`modernc.org/sqlite`), Wails v2, React + TypeScript (Vite), `react-markdown` + `remark-gfm`.

**Spec:** `docs/superpowers/specs/2026-06-18-subtask-execs-and-color-macros-design.md`

**Conventions / important notes:**
- Backend changes are TDD with Go tests. The **frontend has no test runner** (per CLAUDE.md) — frontend verification is `cd frontend && npm run build` (tsc + vite) plus a documented manual check. Feature B's parser is therefore verified by tsc + the explicit example table in Task B1, not a JS test runner. Do **not** add vitest.
- Ship order: **Feature B first** (frontend-only), then Feature A.
- Do NOT stage `CLAUDE.md`, `.remember/`, or local tooling/build-churn files (e.g. `frontend/package.json.md5`). Stage only the files each step names.
- Regenerate Wails bindings with `"$USERPROFILE/go/bin/wails.exe" generate module` after the `Container` struct changes (Task A4/A7).

---

## File map

**Feature B (create/modify):**
- Create `frontend/src/jiraColor.ts` — `splitColorSegments` + `rehypeJiraColor`.
- Modify `frontend/src/components/Markdown.tsx` — add the rehype plugin.

**Feature A (modify/create):**
- `internal/store/store.go` — `test_container` columns, `schemaVersion` 23, migration.
- `internal/store/store_test.go` (or a new `*_test.go`) — migration test.
- `internal/jira/containers.go` — `Container.ParentKey` / `IssueType`; real-path sub-task search (stub).
- `internal/jira/demo.go` — seed sub-task executions.
- `internal/jira/containers_subtask_test.go` (new) — demo seed test.
- `internal/testrepo/testrepo.go` — `Container`/`ContainerMembership` fields, `UpsertContainers`, `ListContainers`.
- `internal/testrepo/containers_parent_test.go` (new) — round-trip test.
- `internal/syncer/engine.go` — map `ParentKey`/`IssueType` in `syncContainers`.
- `app.go` — none (no new bindings; struct change flows through existing `ListContainers`).
- `frontend/wailsjs/**` + `frontend/src/components/ContainersView.tsx` — regen + exec-type filter + parent badge.

---

# PART 1 — Feature B: Jira color-macro rendering

## Task B1: `splitColorSegments` pure parser

**Files:**
- Create: `frontend/src/jiraColor.ts`

- [ ] **Step 1: Write the parser**

```ts
// Parses Jira color macros — {color:VALUE}TEXT{color} — out of a string into a
// flat list of segments. Each segment is plain text or text carrying a color.
// Nested macros resolve innermost-wins for any given character; adjacent and
// repeated macros are supported. An invalid color value keeps the inner text
// but drops the color (and the macro markers).
//
// Examples:
//   splitColorSegments("a {color:#f00}b{color} c")
//     => [{text:"a "}, {text:"b", color:"#f00"}, {text:" c"}]
//   splitColorSegments("{color:#ffbdad}00{color} {color:#57d9a3}00{color}")
//     => [{text:"00",color:"#ffbdad"}, {text:" "}, {text:"00",color:"#57d9a3"}]
//   splitColorSegments("{color:bogus!}x{color}")  => [{text:"x"}]
//   splitColorSegments("plain")                   => [{text:"plain"}]
export interface ColorSegment {
  text: string;
  color?: string;
}

const OPEN = /\{color:([^}]*)\}/;

// A conservative validator: 3/6-digit hex, or a small set of CSS color names
// Jira commonly emits. Anything else is treated as "no color".
const HEX = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
const NAMED = new Set([
  "red", "green", "blue", "black", "white", "gray", "grey", "orange",
  "yellow", "purple", "teal", "navy", "maroon", "olive", "silver", "lime",
]);

function validColor(raw: string): string | undefined {
  const v = raw.trim();
  if (HEX.test(v)) return v;
  if (NAMED.has(v.toLowerCase())) return v.toLowerCase();
  return undefined;
}

export function splitColorSegments(input: string): ColorSegment[] {
  const out: ColorSegment[] = [];
  // Stack of currently-open colors; the top is the active color.
  const stack: Array<string | undefined> = [];
  let rest = input;

  const push = (text: string) => {
    if (!text) return;
    const color = stack.length ? stack[stack.length - 1] : undefined;
    // Merge with the previous segment when the color matches, to keep output tidy.
    const prev = out[out.length - 1];
    if (prev && prev.color === color) prev.text += text;
    else out.push(color ? { text, color } : { text });
  };

  while (rest.length > 0) {
    const open = OPEN.exec(rest);
    const closeIdx = rest.indexOf("{color}");

    // Whichever marker comes first (or none) decides the next emit.
    const openIdx = open ? open.index : -1;
    if (openIdx === -1 && closeIdx === -1) {
      push(rest);
      break;
    }
    const nextIsOpen =
      openIdx !== -1 && (closeIdx === -1 || openIdx < closeIdx);

    if (nextIsOpen && open) {
      push(rest.slice(0, openIdx));
      stack.push(validColor(open[1]));
      rest = rest.slice(openIdx + open[0].length);
    } else {
      // a {color} close
      push(rest.slice(0, closeIdx));
      if (stack.length) stack.pop();
      rest = rest.slice(closeIdx + "{color}".length);
    }
  }
  return out;
}
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend && npm run build`
Expected: build succeeds (the file is exported but not yet imported — fine).

- [ ] **Step 3: Manual verification (no FE test runner)**

Eyeball the function against this table by reasoning through it (and, if desired, temporarily paste it into the browser console while running `wails dev`). It must produce exactly:

| Input | Output |
|-------|--------|
| `plain` | `[{text:"plain"}]` |
| `a {color:#f00}b{color} c` | `[{text:"a "},{text:"b",color:"#f00"},{text:" c"}]` |
| `{color:#ffbdad}00{color} {color:#57d9a3}00{color}` | `[{text:"00",color:"#ffbdad"},{text:" "},{text:"00",color:"#57d9a3"}]` |
| `{color:bogus!}x{color}` | `[{text:"x"}]` |
| `{color:#172b4d}cmd{color}` | `[{text:"cmd",color:"#172b4d"}]` |

- [ ] **Step 4: Commit**

```bash
git add frontend/src/jiraColor.ts
git commit -m "Add splitColorSegments parser for Jira color macros"
```

## Task B2: `rehypeJiraColor` plugin + wire into Markdown

**Files:**
- Modify: `frontend/src/jiraColor.ts` (append the plugin)
- Modify: `frontend/src/components/Markdown.tsx`

- [ ] **Step 1: Append the rehype plugin to `jiraColor.ts`**

```ts
// Minimal hast node shapes (avoids a hard dependency on @types/hast).
interface HastText {
  type: "text";
  value: string;
}
interface HastElement {
  type: "element";
  tagName: string;
  properties?: Record<string, unknown>;
  children: HastNode[];
}
type HastNode = HastText | HastElement | { type: string; children?: HastNode[] };

function colorSpan(seg: ColorSegment): HastNode {
  if (!seg.color) return { type: "text", value: seg.text } as HastText;
  return {
    type: "element",
    tagName: "span",
    // react-markdown parses this style string into a React style object.
    properties: { style: `color:${seg.color}` },
    children: [{ type: "text", value: seg.text } as HastText],
  } as HastElement;
}

// rehype plugin: replace text nodes containing color macros with a mix of text
// and styled <span> nodes. Builds element nodes programmatically (never enables
// raw HTML), so no XSS surface is opened.
export function rehypeJiraColor() {
  return (tree: { children?: HastNode[] }) => {
    const walk = (node: { children?: HastNode[] }) => {
      if (!node.children) return;
      const next: HastNode[] = [];
      for (const child of node.children) {
        if ((child as HastText).type === "text") {
          const value = (child as HastText).value;
          if (value.includes("{color")) {
            for (const seg of splitColorSegments(value)) next.push(colorSpan(seg));
          } else {
            next.push(child);
          }
        } else {
          walk(child as { children?: HastNode[] });
          next.push(child);
        }
      }
      node.children = next;
    };
    walk(tree);
  };
}
```

- [ ] **Step 2: Wire it into `Markdown.tsx`**

Change the import block and the `<ReactMarkdown>` element. The current file:
```tsx
import ReactMarkdown from "react-markdown";
import type { Components } from "react-markdown";
import remarkGfm from "remark-gfm";
```
Add:
```tsx
import { rehypeJiraColor } from "../jiraColor";
```
And change:
```tsx
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </ReactMarkdown>
```
to:
```tsx
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeJiraColor]}
        components={components}
      >
        {children}
      </ReactMarkdown>
```

- [ ] **Step 3: Build**

Run: `cd frontend && npm run build`
Expected: build succeeds.

- [ ] **Step 4: Manual verification in the app**

Run `wails dev`, open a demo profile, and in a test description or step field type:
```
Command : {color:#172b4d} *mycommand* {color}
Output : {color:#4c9aff}*01*{color} *{color:#ffbdad}00 00 00 00{color} {color:#57d9a3}00{color}*
```
Click away to render. Verify: the `{color}` text is gone and the wrapped text shows in the given colors. If the colors do NOT apply (react-markdown ignored the `style` string), add a `span` component override to `Markdown.tsx`'s `components` that maps the style string to a style object:
```tsx
  span({ node, style, ...props }) {
    return <span {...props} style={typeof style === "string" ? undefined : style} />;
  },
```
(react-markdown v10 normally parses the style string itself, so this fallback is usually unnecessary — only add it if the manual check shows uncolored text.)

- [ ] **Step 5: Commit**

```bash
git add frontend/src/jiraColor.ts frontend/src/components/Markdown.tsx
git commit -m "Render Jira {color} macros in the markdown read view"
```

---

# PART 2 — Feature A: Sub-task Test Executions

## Task A1: schema columns + migration v23

**Files:**
- Modify: `internal/store/store.go`
- Create: `internal/store/migration_v23_test.go`

- [ ] **Step 1: Write the failing test**

```go
package store

import "testing"

func TestMigrationV23AddsContainerParentColumns(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Insert a container using the new columns; fails if they don't exist.
	if _, err := db.DB().Exec(
		`INSERT INTO test_container
		   (profile_id, jira_key, kind, summary, status, parent_key, issue_type)
		 VALUES ('p1', 'DEMO-TE-1', 'testexec', 'Cycle', 'Open', 'DEMO-S-1', 'Sub Test Execution')`,
	); err != nil {
		t.Fatalf("insert with parent_key/issue_type: %v", err)
	}
	var parent, issueType string
	if err := db.DB().QueryRow(
		`SELECT parent_key, issue_type FROM test_container WHERE jira_key = 'DEMO-TE-1'`,
	).Scan(&parent, &issueType); err != nil {
		t.Fatalf("select: %v", err)
	}
	if parent != "DEMO-S-1" || issueType != "Sub Test Execution" {
		t.Fatalf("got parent=%q issueType=%q", parent, issueType)
	}
}
```

Note: confirm the store's open function and the accessor for the raw `*sql.DB`. Inspect `internal/store/store.go` and an existing `internal/store/*_test.go` to find the exact constructor (e.g. `Open`) and DB accessor (e.g. `db.DB()`); adjust the test's two calls to match the real API before running.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/store/ -run TestMigrationV23 -v`
Expected: FAIL — `no such column: parent_key`.

- [ ] **Step 3: Implement the schema change**

In `internal/store/store.go`:

a) Bump the version constant:
```go
const schemaVersion = 23
```

b) Add the columns to the `test_container` CREATE in `baseSchema`:
```go
CREATE TABLE IF NOT EXISTS test_container (
	profile_id TEXT NOT NULL,
	jira_key   TEXT NOT NULL,
	kind       TEXT NOT NULL,
	summary    TEXT NOT NULL DEFAULT '',
	status     TEXT NOT NULL DEFAULT '',
	parent_key TEXT NOT NULL DEFAULT '',
	issue_type TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, jira_key)
);
```

c) Add the migration at the end of `applyMigrations` (after the `current < 22` block):
```go
	// v23: sub-task Test Execution support — parent_key (the parent issue key
	// for a sub-task execution; empty for standalone) and issue_type (the Jira
	// issuetype name, informational). Fresh installs get these from the CREATE
	// above; these ALTERs catch pre-v23 databases.
	if current < 23 {
		for _, col := range []string{"parent_key", "issue_type"} {
			if _, err := db.Exec(
				fmt.Sprintf(`ALTER TABLE test_container ADD COLUMN %s TEXT NOT NULL DEFAULT ''`, col),
			); err != nil && !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("v23 add %s: %w", col, err)
			}
		}
	}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/store/ -run TestMigrationV23 -v`
Expected: PASS.

- [ ] **Step 5: Run the package**

Run: `go test ./internal/store/`
Expected: ok.

- [ ] **Step 6: Commit**

```bash
git add internal/store/store.go internal/store/migration_v23_test.go
git commit -m "Schema v23: add parent_key/issue_type to test_container (sub-task execs)"
```

## Task A2: jira.Container fields + demo sub-task executions

**Files:**
- Modify: `internal/jira/containers.go`
- Modify: `internal/jira/demo.go`
- Create: `internal/jira/containers_subtask_test.go`

- [ ] **Step 1: Write the failing test**

```go
package jira

import (
	"context"
	"strings"
	"testing"
)

func TestDemoSeedsSubTaskExecutions(t *testing.T) {
	c := NewClient("demo", "")
	containers, links, err := c.ListContainers(context.Background(), "DEMO", nil)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	var subs []Container
	for _, ct := range containers {
		if ct.Kind == KindTestExec && ct.ParentKey != "" {
			subs = append(subs, ct)
		}
	}
	if len(subs) < 2 {
		t.Fatalf("want >=2 sub-task executions, got %d", len(subs))
	}
	for _, s := range subs {
		if !strings.HasPrefix(s.ParentKey, "DEMO-S-") {
			t.Errorf("sub-task %s has unexpected parent %q", s.Key, s.ParentKey)
		}
		if s.IssueType == "" {
			t.Errorf("sub-task %s missing issue type", s.Key)
		}
	}
	// Sub-task executions carry run links like standalone ones.
	linked := map[string]bool{}
	for _, l := range links {
		linked[l.ContainerKey] = true
	}
	for _, s := range subs {
		if !linked[s.Key] {
			t.Errorf("sub-task execution %s has no run links", s.Key)
		}
	}
}
```

Note: confirm `ListContainers`'s real signature (the Explore showed `ListContainers(ctx, projectKey, onProgress)`); the `nil` here is the `onProgress` arg. Adjust if the signature differs.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/jira/ -run TestDemoSeedsSubTaskExecutions -v`
Expected: FAIL — `ParentKey`/`IssueType` undefined, or no sub-task execs.

- [ ] **Step 3a: Add the fields to `Container` in `internal/jira/containers.go`**

```go
type Container struct {
	Key       string
	Kind      string
	Summary   string
	Status    string
	ParentKey string // parent issue key for a sub-task Test Execution; else ""
	IssueType string // Jira issuetype name (e.g. "Sub Test Execution"); informational
}
```

- [ ] **Step 3b: Seed sub-task executions in `demoContainersAndLinks` (`internal/jira/demo.go`)**

After the existing standalone execution loop (the `for i := 0; i < execCount; i++` block that appends `KindTestExec` containers) and before the membership loop, add:
```go
	// Sub-task Test Executions: a couple of executions that are Jira sub-tasks of
	// a parent issue (a Story here), exercising the parent-linked execution path
	// offline. They are still Kind=testexec and behave like standalone ones.
	const subExecCount = 2
	subExecKeys := make([]string, subExecCount)
	for i := 0; i < subExecCount; i++ {
		key := fmt.Sprintf("%s-STE-%d", projectKey, i+1)
		subExecKeys[i] = key
		containers = append(containers, Container{
			Key:       key,
			Kind:      KindTestExec,
			Summary:   fmt.Sprintf("Sub-execution for story %d", i+1),
			Status:    demoExecStatuses[i%len(demoExecStatuses)],
			ParentKey: fmt.Sprintf("%s-S-%d", projectKey, i+1),
			IssueType: "Sub Test Execution",
		})
	}
```
Then, inside the existing membership loop (the `for i := 0; i < demoLinkedTests …` loop that links tests to `execKeys[i%execCount]`), add a link so the sub-task executions get members with run status. Right after the cross-project link block, add:
```go
		// Every 5th linked test also runs in a sub-task execution.
		if i%5 == 0 {
			links = append(links, ContainerLink{
				ContainerKey: subExecKeys[(i/5)%len(subExecKeys)],
				TestKey:      testKey,
				RunStatus:    demoRunStatuses[(i+1)%len(demoRunStatuses)],
			})
		}
```

- [ ] **Step 3c: Carry the fields through the real path of `ListContainers`**

In `internal/jira/containers.go`, the real (non-demo) path searches per kind. Leave standalone behavior intact and add a `TODO(xtm)` documenting the sub-task pass so it is explicit (the real path stays stubbed/empty as today). Above the `for _, kind := range …` loop add a comment:
```go
	// TODO(xtm): also search the sub-task Test Execution issuetype and set
	// Container.ParentKey from fields.parent.key + IssueType from the issuetype
	// name, so sub-task executions sync like standalone ones. Verify the
	// issuetype name and parent field on a live Xray Server 8.4.0 instance.
```
(No functional real-path change in this task — demo drives the feature, consistent with Phase 7.)

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/jira/ -run TestDemoSeedsSubTaskExecutions -v`
Expected: PASS.

- [ ] **Step 5: Run the package**

Run: `go test ./internal/jira/`
Expected: ok.

- [ ] **Step 6: Commit**

```bash
git add internal/jira/containers.go internal/jira/demo.go internal/jira/containers_subtask_test.go
git commit -m "Jira: Container parent/issue-type fields + demo sub-task executions"
```

## Task A3: testrepo Container fields + store round-trip

**Files:**
- Modify: `internal/testrepo/testrepo.go`
- Create: `internal/testrepo/containers_parent_test.go`

- [ ] **Step 1: Write the failing test**

```go
package testrepo

import "testing"

func TestUpsertAndListContainersCarryParent(t *testing.T) {
	r := newTestRepo(t) // shared helper in sankey_crossproject_test.go
	const p = "p1"

	in := []Container{
		{Key: "DEMO-TE-1", Kind: "testexec", Summary: "Standalone", Status: "Open"},
		{Key: "DEMO-STE-1", Kind: "testexec", Summary: "Sub", Status: "Open",
			ParentKey: "DEMO-S-1", IssueType: "Sub Test Execution"},
	}
	if err := r.UpsertContainers(p, in); err != nil {
		t.Fatalf("UpsertContainers: %v", err)
	}
	got, err := r.ListContainers(p, "testexec")
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	byKey := map[string]Container{}
	for _, c := range got {
		byKey[c.Key] = c
	}
	if byKey["DEMO-TE-1"].ParentKey != "" {
		t.Errorf("standalone exec should have empty ParentKey, got %q", byKey["DEMO-TE-1"].ParentKey)
	}
	sub := byKey["DEMO-STE-1"]
	if sub.ParentKey != "DEMO-S-1" || sub.IssueType != "Sub Test Execution" {
		t.Errorf("sub-task exec lost parent/issuetype: %+v", sub)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/testrepo/ -run TestUpsertAndListContainersCarryParent -v`
Expected: FAIL — `Container` has no field `ParentKey`.

- [ ] **Step 3a: Add fields to the structs (`internal/testrepo/testrepo.go`)**

```go
type Container struct {
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	ParentKey string `json:"parentKey"`
	IssueType string `json:"issueType"`
}
```
Leave `ContainerMembership` unchanged: nothing populates or reads a parent on the
per-test membership list (the parent badge reads `Container.parentKey` from
`ListContainers`), so adding fields there would be dead weight.

- [ ] **Step 3b: Update `UpsertContainers` INSERT (around line 1068)**

```go
	stmt, err := tx.Prepare(
		`INSERT INTO test_container (profile_id, jira_key, kind, summary, status, parent_key, issue_type)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, jira_key) DO UPDATE SET
		   kind       = excluded.kind,
		   summary    = excluded.summary,
		   status     = excluded.status,
		   parent_key = excluded.parent_key,
		   issue_type = excluded.issue_type`)
	if err != nil {
		return fmt.Errorf("prepare upsert container: %w", err)
	}
	defer stmt.Close()

	for _, c := range containers {
		if _, err := stmt.Exec(profileID, c.Key, c.Kind, c.Summary, c.Status, c.ParentKey, c.IssueType); err != nil {
			return fmt.Errorf("upsert container %s: %w", c.Key, err)
		}
	}
```

- [ ] **Step 3c: Update `ListContainers` SELECT + scan (around line 1421)**

```go
func (r *Repository) ListContainers(profileID, kind string) ([]Container, error) {
	rows, err := r.db.Query(
		`SELECT jira_key, kind, summary, status, parent_key, issue_type FROM test_container
		 WHERE profile_id = ? AND kind = ? ORDER BY jira_key`,
		profileID, kind)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	defer rows.Close()

	out := []Container{}
	for rows.Next() {
		var c Container
		if err := rows.Scan(&c.Key, &c.Kind, &c.Summary, &c.Status, &c.ParentKey, &c.IssueType); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/testrepo/ -run TestUpsertAndListContainersCarryParent -v`
Expected: PASS.

- [ ] **Step 5: Run the package**

Run: `go test ./internal/testrepo/`
Expected: ok. (The `ExecutionsForPlans` query selects `c.summary, c.status` explicitly, not `*`, so it is unaffected; verify the package is green.)

- [ ] **Step 6: Commit**

```bash
git add internal/testrepo/testrepo.go internal/testrepo/containers_parent_test.go
git commit -m "testrepo: carry parent_key/issue_type through container upsert + list"
```

## Task A4: map the fields in syncContainers + build

**Files:**
- Modify: `internal/syncer/engine.go`

- [ ] **Step 1: Update the mapping in `syncContainers` (around line 289)**

```go
	repoContainers := make([]testrepo.Container, len(containers))
	for i, c := range containers {
		repoContainers[i] = testrepo.Container{
			Key:       c.Key,
			Kind:      c.Kind,
			Summary:   c.Summary,
			Status:    c.Status,
			ParentKey: c.ParentKey,
			IssueType: c.IssueType,
		}
	}
```

- [ ] **Step 2: Build + test the syncer (and confirm demo flows parent through end to end)**

Run: `go build ./... && go test ./internal/syncer/ ./internal/testrepo/ ./internal/jira/`
Expected: build exit 0; all three packages ok.

- [ ] **Step 3: Commit**

```bash
git add internal/syncer/engine.go
git commit -m "syncer: map container ParentKey/IssueType into the store"
```

## Task A5: regenerate bindings + Containers UI (filter + parent badge)

**Files:**
- Regenerate: `frontend/wailsjs/go/models.ts` (+ App.d.ts/App.js if touched)
- Modify: `frontend/src/components/ContainersView.tsx`

- [ ] **Step 1: Regenerate Wails bindings**

Run: `"$USERPROFILE/go/bin/wails.exe" generate module`
Expected: `frontend/wailsjs/go/models.ts` `Container` now has `parentKey` and `issueType` fields.

- [ ] **Step 2: Add exec-type filter state to `ContainersView`**

After the existing `const [cStatus, setCStatus] = useState("");` line add:
```tsx
  // Execution-type filter (Test Execution kind only): "" = all, "standalone", "subtask".
  const [cExecType, setCExecType] = useState("");
```
And reset it when the kind changes — extend the existing kind-reset effect body (the one that does `setCFilter(""); setCStatus("");`) to also `setCExecType("");`.

- [ ] **Step 3: Apply the exec-type filter in the `viewContainers` memo**

In the `viewContainers` `useMemo`, change the `base` filter to also honor `cExecType` (only meaningful when `kind === "testexec"`):
```tsx
    const base = containers.filter(
      (c) =>
        (!cStatus || c.status === cStatus) &&
        (kind !== "testexec" ||
          !cExecType ||
          (cExecType === "subtask" ? !!c.parentKey : !c.parentKey)) &&
        (!f ||
          c.key.toLowerCase().includes(f) ||
          (c.summary ?? "").toLowerCase().includes(f)),
    );
```
Add `cExecType` to the memo's dependency array.

- [ ] **Step 4: Render the exec-type select in the filter bar**

Inside the `container-filter-bar`, immediately after the status `<select className="container-status-filter">…</select>`, add (only for executions):
```tsx
        {kind === "testexec" && (
          <select
            className="container-status-filter"
            value={cExecType}
            onChange={(e) => setCExecType(e.target.value)}
            title="Filter by execution type"
          >
            <option value="">All executions</option>
            <option value="standalone">Standalone</option>
            <option value="subtask">Sub-task</option>
          </select>
        )}
```

- [ ] **Step 5: Add the parent badge + open-in-Jira helper**

Add a helper inside the component (near the other handlers):
```tsx
  const isDemoUrl = /^(demo$|demo:|mock:)/i.test((jiraUrl ?? "").trim());
  function openParent(parentKey: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base && !isDemoUrl && !parentKey.startsWith("NEW-")) {
      BrowserOpenURL(`${base}/browse/${parentKey}`);
    }
  }
```
Add `BrowserOpenURL` to the `../api` import in this file if not already imported.

Then in the `container-card-top` block (the row with the kind badge / key / status / count), after the `container-card-key` span add:
```tsx
            {selectedContainer.parentKey && (
              <button
                className="mono container-parent-link"
                onClick={() => openParent(selectedContainer.parentKey)}
                title={`Open parent ${selectedContainer.parentKey} in Jira`}
              >
                ↳ {selectedContainer.parentKey}
              </button>
            )}
            {selectedContainer.issueType && selectedContainer.parentKey && (
              <span className="muted">{selectedContainer.issueType}</span>
            )}
```

- [ ] **Step 6: Add minimal styles to `frontend/src/App.css`**

```css
.container-parent-link {
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  color: var(--accent);
  font-size: 12px;
}
.container-parent-link:hover {
  text-decoration: underline;
}
```

- [ ] **Step 7: Build**

Run: `cd frontend && npm run build`
Expected: build succeeds.

- [ ] **Step 8: Manual verification**

`wails dev` → demo profile → Containers tab → Test Execution kind. Verify: the exec-type filter (All / Standalone / Sub-task) narrows the picker; selecting a sub-task execution (`DEMO-STE-1`) shows a "↳ DEMO-S-1" badge and its issue type; the board, run-status editing, and create-bug still work on it.

- [ ] **Step 9: Commit**

```bash
git add frontend/wailsjs frontend/src/components/ContainersView.tsx frontend/src/App.css
git commit -m "Containers UI: execution-type filter + sub-task parent badge"
```

## Task A6: full verification

- [ ] **Step 1:** `go build ./... && go test ./...` — build exit 0, all packages ok.
- [ ] **Step 2:** `cd frontend && npm run build` — succeeds.
- [ ] **Step 3:** Manual demo smoke of both features (color macros render; sub-task executions filter + badge work).

---

## Self-review notes (addressed)

- **No FE test runner:** Feature B's parser is verified via tsc + the explicit example table (Task B1 Step 3) and the in-app check (Task B2 Step 4); vitest is intentionally not added.
- **Write path:** the sync container write is `UpsertContainers` (not a `ReplaceAll*`); `SeedSampleContainers` and `CreateContainerAndAllocate` stay standalone (no parent), which is correct.
- **Board unaffected:** the parent badge reads `selectedContainer.parentKey` from `ListContainers`, so `GetContainerBoard` needs no change.
- **Type consistency:** `ParentKey`/`IssueType` (Go) ↔ `parentKey`/`issueType` (TS) used consistently across struct, SQL, mapping, and UI.
- **Security:** `rehypeJiraColor` builds nodes programmatically; no `rehype-raw`, so raw HTML stays inert.
