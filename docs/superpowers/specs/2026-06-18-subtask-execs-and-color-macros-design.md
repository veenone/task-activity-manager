---
title: Sub-task Test Executions and Jira color-macro rendering
date: 2026-06-18
status: approved
---

## Summary

Two independent Jira-fidelity improvements:

1. **Sub-task Test Executions** — Xray has a sub-task issue type for Test
   Executions that hangs off a parent issue (often a Story, but not limited to
   it). The tool currently has no notion of a parent or issue type on a
   container. Add a unified model so sub-task executions sync locally and reuse
   every existing Test Execution feature, distinguished only by a parent
   reference.
2. **Jira color-macro rendering** — description and step text can contain Jira
   wiki color macros (`{color:#hex}…{color}`). They currently render as literal
   text. Render them with the proper color in the read view while keeping the
   edit view raw so the macros round-trip back to Jira unchanged.

These are independent and ship as separate slices.

## Context (current behavior)

- Containers are modeled by `jira.Container` and `testrepo.Container` with only
  `Key / Kind / Summary / Status` (`internal/jira/containers.go:23`,
  `internal/testrepo/testrepo.go:60`). `Kind` is one of `testset / testplan /
  testexec`. There is no parent, issue-type, or subtask field anywhere.
- The `test_container` table (`internal/store/store.go`) has columns
  `profile_id, jira_key, kind, summary, status`. `schemaVersion` is **22**.
- `ListContainers` (real path, stubbed for Phase 7) searches by issue type per
  kind; demo mode (`demoContainersAndLinks`) seeds `PROJECT-TE-N` executions
  with no parent.
- All description / step / precondition text renders through `MarkdownField`
  (idle = rendered via the `Markdown` component, edit = raw textarea). `Markdown`
  uses `react-markdown` + `remark-gfm` and does no Jira-macro parsing
  (`frontend/src/components/Markdown.tsx`). Eight render sites across
  `TestDetail.tsx`, `NewTestPanel.tsx`, and `PreconditionsView.tsx` share it.

## Feature A — Sub-task Test Executions

### Data model

Add two columns to `test_container`:

- `parent_key TEXT NOT NULL DEFAULT ''` — the parent issue key for a sub-task
  execution; empty for standalone executions and all other kinds.
- `issue_type TEXT NOT NULL DEFAULT ''` — the Jira issue-type name (e.g.
  "Test Execution" vs "Sub Test Execution"), informational and used to label.

`Kind` stays `testexec` for both standalone and sub-task executions. A non-empty
`parent_key` is the sole discriminator for "is a sub-task". Bump `schemaVersion`
to **23** and add an ordered migration that `ALTER TABLE test_container ADD
COLUMN` for each (additive, so old DBs upgrade cleanly before any index runs).

Carry the fields through the types and the read/write paths:

- `jira.Container`: add `ParentKey string`, `IssueType string`.
- `testrepo.Container` and `testrepo.ContainerMembership`: add `ParentKey` and
  `IssueType` (json `parentKey`, `issueType`).
- `ReplaceAllContainers` (insert), `ListContainers` (select), and
  `GetContainerBoard` (the selected container's header info) read/write the new
  columns. Container CRUD snapshots (`containercrud.go`) keep `parent_key` /
  `issue_type` untouched on edit/delete (they are sync-owned, not user-edited).

### Sync

- Real `ListContainers` (`internal/jira/containers.go`) is extended to also
  search the sub-task Test Execution issue type and read `fields.parent.key`
  into `ParentKey` and the issue type name into `IssueType`. This path stays
  stubbed / `TODO(xtm)`-marked, consistent with the rest of Phase 7.
- Demo (`demoContainersAndLinks`): seed two or three sub-task executions with
  `Kind=testexec`, `ParentKey` set to a synthetic parent key (e.g. `DEMO-S-1`,
  `DEMO-S-2` — Story-shaped, distinct from the test number range), `IssueType =
  "Sub Test Execution"`, plus run links so they appear on the board with run
  status like standalone executions.

### Behavior and UI

- Sub-task executions appear in the same Test Execution picker (`Kind=testexec`)
  and reuse all execution behavior unchanged: the member board, run-status
  editing, bulk run-status, create-bug-from-failed-row, and the traceability
  Sankey.
- The container filter bar (only when `kind === "testexec"`) gains an
  **execution-type** select: `All / Standalone / Sub-task`. `Standalone` keeps
  containers with empty `parentKey`; `Sub-task` keeps those with a non-empty
  `parentKey`. It composes with the existing keyword + status filters and the
  sort control.
- The selected execution's `container-card` shows, when `parentKey` is set, a
  clickable **"↳ <parentKey>"** badge that opens the parent issue in Jira via
  `BrowserOpenURL(<base>/browse/<parentKey>)` (suppressed for demo profiles and
  `NEW-` draft keys, mirroring the existing test-key link pattern). The
  `issueType` label is shown as muted text beside it when present.

### Out of scope

- Creating sub-task executions locally (would require choosing a parent issue);
  standalone container creation is unchanged.
- Syncing the parent issue itself into the store — the badge only needs the key
  and opens it in the browser.

## Feature B — Jira color-macro rendering

### Approach

A custom rehype plugin, `rehypeJiraColor`, runs in the `Markdown` render
pipeline (idle view only). It walks the HAST text nodes and, for each
`{color:VALUE}TEXT{color}` occurrence, replaces the text with a sequence of
text nodes and `span` element nodes carrying `style: "color: VALUE"`. Because the
plugin constructs element nodes programmatically (it never enables raw HTML
parsing), it adds no XSS surface: any literal `<script>`/`<img onerror>` in the
Jira text remains escaped exactly as today. No `rehype-raw`, no new runtime
dependency (the tree walk is hand-written; no `unist-util-visit` needed).

The core is a pure function:

```ts
// splitColorSegments("a {color:#f00}b{color} c")
//   => [{text:"a "}, {text:"b", color:"#f00"}, {text:" c"}]
export function splitColorSegments(text: string): Array<{ text: string; color?: string }>
```

It handles multiple and adjacent macros and nested macros (innermost wins for a
given character). Color `VALUE` is validated against a hex pattern
(`#rgb` / `#rrggbb`) or a conservative CSS color-name allowlist; an invalid value
yields a plain text segment (the macro markers are dropped but the inner text is
kept). The plugin maps each returned segment to a text node or a styled `span`.

Nested/adjacent example from the request renders correctly because each complete
`{color}…{color}` pair sits within a single markdown-produced text node:

```
*{color:#ffbdad}00 00 00 00{color} {color:#57d9a3}00{color}*
```

markdown emits one emphasis node whose text child is split by the plugin into two
colored spans.

**Known limitation (documented, accepted):** a macro whose open/close straddle a
markdown delimiter boundary (e.g. `{color:#f00}*bold{color} more*`) will not be
colored, because the macro is split across HAST nodes. Jira's own output keeps
color macros self-contained within formatting, matching the request's examples,
so this is acceptable. Such a case degrades to plain (uncolored) text, never to
broken markup.

### Wiring

`Markdown.tsx` adds `rehypePlugins={[rehypeJiraColor]}` to the existing
`<ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>`. This single
change applies to all eight render sites. `MarkdownField`'s edit view is
untouched, so editing shows and saves the raw `{color}` macro text.

Scope is color macros only — other Jira markup (`*bold*`, `{code}`, `{noformat}`)
is intentionally not translated, to avoid changing how existing markdown already
renders.

## Testing

- **Go:** schema migration v22 → v23 adds the columns (a fresh DB and an
  upgraded old DB both end with `parent_key` / `issue_type`); `ReplaceAllContainers`
  → `ListContainers` round-trips `ParentKey` / `IssueType`; the demo seed includes
  sub-task executions with non-empty `ParentKey` and run links.
- **TypeScript:** unit tests for `splitColorSegments` — single macro, multiple,
  adjacent, nested, invalid hex (passthrough of inner text), and no-macro
  passthrough.
- **Builds:** `go build ./...`, `go test ./...`, `cd frontend && npm run build`
  (regenerate Wails bindings after the `Container` struct change).

## Rollout

Two independent slices, each its own commit/PR:

1. Feature B (color macros) — frontend-only, no schema/binding churn; ship first.
2. Feature A (sub-task executions) — schema migration + bindings + sync + UI.
