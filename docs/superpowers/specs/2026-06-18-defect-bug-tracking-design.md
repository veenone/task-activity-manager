# Defect (Bug) tracking

**Status:** Approved (design) — 2026-06-18
**Area:** New cross-cutting feature spanning store, jira client, syncer, app bindings, and frontend. Mirrors the existing **requirements** feature.

## Problem

When a test is marked FAILED in a Test Execution, there is no way to file a
defect against it from the app, no overview of defects linked to the profile's
tests, and no way to see a test's linked defects (which may live in another
Jira project). QA has to leave the app, create a Bug in Jira by hand, and link
it manually.

## Goal

Three capabilities, delivered as one cohesive "defect tracking" feature:

1. **Create a Bug from a failed test** in a Test Execution container — a new
   Bug-type Jira issue, linked to that test.
2. **List all bugs** linked to the profile's tests, as a panel inside the
   Containers view.
3. **Show a test's linked bugs** on the test-case detail (including
   cross-project bugs) as hyperlinks that open the bug in the browser.

The feature mirrors the **requirements** feature end-to-end (cross-project Jira
issues, cached locally, discovered via `issuelinks`, shown on test detail + a
dedicated list), reusing its proven patterns.

Non-goals: editing bug fields locally; a bug workflow/transition UI; unlinking
bugs from the detail panel; a configurable bug-source project catalog.

## Locked decisions

- **Create flow: local-first, commit on sync.** Creating a bug makes a local
  placeholder (`NEW-BUG-N`) + a `bug_create` pending change; the issue and its
  link to the test are POSTed to Jira on the next Commit, exactly like new
  tests today. Works in demo mode; undoable via Discard; visible in Pending
  Changes.
- **Bugs list location: a panel inside the Containers view**, reached by a new
  top-level `[Containers | Bugs]` mode toggle (default Containers). Bugs are not
  a 4th container "kind" (they have no member-test board), so they get their own
  mode rather than joining the kind selector.
- **Create form: summary + prefilled-editable description + priority + labels.**
  Project defaults to the profile's project key.
- **Bug issue type defaults to `"Bug"`** for creation; discovery of existing
  linked bugs recognizes the set `{"Bug","Defect"}`.
- **Detail section is read-only**, showing each linked bug's key as a hyperlink
  (incl. cross-project); creation happens from the execution board.
- **The test↔bug relationship is a Jira issue link** (link type stubbed with a
  live-verification NOTE, exactly as the requirements feature does today).

## Architecture

### Data model (`internal/store/store.go`, bump `schemaVersion`)

Two new tables, mirroring `requirement` / `test_requirement`:

```sql
CREATE TABLE IF NOT EXISTS bug (
    profile_id  TEXT NOT NULL,
    jira_key    TEXT NOT NULL,
    project_key TEXT NOT NULL DEFAULT '',
    issue_type  TEXT NOT NULL DEFAULT '',
    summary     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT '',
    priority    TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (profile_id, jira_key)
);

CREATE TABLE IF NOT EXISTS test_bug (
    profile_id TEXT NOT NULL,
    test_key   TEXT NOT NULL,
    bug_key    TEXT NOT NULL,
    link_id    TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (profile_id, test_key, bug_key)
);
-- indexes: idx_test_bug_test (profile_id, test_key), idx_test_bug_bug (profile_id, bug_key)
```

New tables are auto-created on existing DBs by the `CREATE TABLE IF NOT EXISTS`
baseSchema path (no destructive migration), consistent with how `requirement`
was added at schema v17.

### Go types (`internal/testrepo/bugs.go`)

- `Bug{ Key, ProjectKey, IssueType, Summary, Status, Priority, Updated }`
- `BugLink{ TestKey, BugKey, LinkID }`
- `BugWithTests{ Key, ProjectKey, Summary, Status, Priority, TestKeys []string }`
  — for the Bugs panel.
- `TestBug{ Key, ProjectKey, Summary, Status, Priority }` — for the detail
  section.
- `BugDraft{ ProjectKey, Summary, Description, Priority, Labels }` — create
  payload.
- `bugLinkSnap{ Key, LinkID }` — discard snapshot (mirrors `reqLinkSnap`).

### Read / sync (inbound)

- `internal/jira/bugs.go`:
  - `ListBugs(ctx, profileProjectKey, testKeys, onProgress) ([]Bug, []BugLink, error)`
    — demo short-circuits to `demoBugs`; the real path reads tests' `issuelinks`,
    keeps issues whose issuetype ∈ `{"Bug","Defect"}`, batch-fetches them by key
    (`/rest/api/2/search?jql=key in (...)`) so cross-project bugs resolve, and
    parses links. Carries a `NOTE(xtm)` for live Xray Server 8.4.0 verification,
    mirroring `ListRequirements`.
  - `CreateBug(ctx, projectKey, summary, description, priority string, labels []string) (key string, err error)`
    — POST `/rest/api/2/issue` with `issuetype.name="Bug"`; demo returns a
    synthetic key; real path NOTE'd.
  - Issue-link creation reuses the same endpoint the requirements commit path
    uses (POST `/rest/api/2/issueLink`).
  - `demoBugs(testProjectKey)` — generates a handful of Bug issues, some in a
    separate `BUGS` project, linked to the demo profile's FAILED tests, so the
    panel and detail section have cross-project data.
- `internal/testrepo/bugs.go`: `ReplaceAllBugs`, `ReplaceAllBugLinks` (full
  replace on sync), `ListBugsWithTests(profileID)`, `GetTestBugs(profileID, testKey)`.
- `internal/syncer/engine.go`: a best-effort `syncBugs` pass after
  `syncRequirements` — `ListBugs` → `ReplaceAllBugs` + `ReplaceAllBugLinks`;
  failures log and continue (same as requirement sync).

### Create + link (outbound, local-first)

- `internal/testrepo/bugcrud.go`:
  - `CreateBugForTest(profileID, testKey, execKey string, draft BugDraft) (string, error)`
    — allocates a placeholder key `NEW-BUG-N`, inserts it into `bug` (status
    "(new)"), inserts the `test_bug` link, and queues a `bug_create` pending
    change. `before_val` is a snapshot for discard; `after_val` is the JSON
    draft payload (incl. `testKey`). Returns the placeholder key.
  - `RenameBug(profileID, oldKey, newKey string) error` — used at commit to
    repoint the cached bug + link from the placeholder to the real key.
- `internal/testrepo/testrepo.go`: new entity constant `entityBugCreate =
  "bug_create"`; a `DiscardPendingChange` case that deletes the placeholder bug,
  its `test_bug` link, and the pending row.
- `internal/syncer/commit.go`: collect `bug_create` rows; `commitBugCreates`
  mirrors `commitTestCreates` — `CreateBug` → real key → `RenameBug` →
  `CreateBugLink(testKey, realKey)` → clear the pending row. Reported under the
  test key.
- `app.go`: binding `CreateBugForTest(profileID, testKey, execKey, summary,
  description, priority string, labels []string) (string, error)`; plus read
  bindings `ListBugsWithTests`, `GetTestBugs`.

### Frontend

- `frontend/src/api.ts`: interfaces `Bug`, `BugWithTests`, `TestBug`; exports
  `CreateBugForTest`, `ListBugsWithTests`, `GetTestBugs`.
- **Create from failed test** (`ContainersView.tsx` execution board): rows whose
  `runStatus` is `FAIL`/`FAILED` show a `🐞 Bug` button beside the `✕` remove.
  It opens a new `CreateBugModal.tsx` (summary required; priority `<select>`;
  labels input; description `<textarea>` prefilled with `Found while executing
  {execKey}. Test {testKey} "{summary}" marked FAILED.` and editable). Submit →
  `CreateBugForTest` → bump refresh + reload pending.
- **Bugs panel** (`ContainersView.tsx`): a top-of-view `[Containers | Bugs]`
  toggle. In Bugs mode, render `BugsPanel` listing `ListBugsWithTests`: bug key
  (click → `BrowserOpenURL({jiraUrl}/browse/{key})`), project, summary, status,
  priority, and affected test keys (click → select that test / open detail).
  Text + project + status filters. The view receives `jiraUrl` so links work and
  are disabled for demo profiles (same guard as TestDetail's `openInJira`).
- **Test detail section** (`TestDetail.tsx`): a **Bugs** section after
  Requirements, listing `GetTestBugs`: each bug key as a hyperlink (reusing the
  existing `BrowserOpenURL` + `{jiraUrl}/browse/{key}` pattern, guarded for demo
  and `NEW-` keys), project (cross-project visible), summary, status pill.
  Read-only. Empty state: "No linked bugs."
- `PendingChangesModal.tsx`: `describeChange` case for `bug_create` →
  field "new bug", after = the draft summary.
- `App.tsx`: pass `jiraUrl` to `ContainersView` (TestDetail already receives it).

## Error handling

- All Jira write/discovery paths demo-no-op and carry `NOTE(xtm)` live-verify
  comments, exactly as the requirements feature does — demo mode is fully
  functional; real Xray calls are wired but flagged pending live verification.
- `CreateBugForTest` is a single local transaction (placeholder + link + pending
  row) — partial failure rolls back.
- `commitBugCreates`: if the issue is created but the link POST fails, the
  pending row is left in place for retry (mirrors `commitTestCreates`' step
  failure handling); a cache-rename hiccup after a successful create does not
  fail the commit.

## Testing

- Go (`internal/testrepo`, `internal/jira`, `internal/syncer`):
  - `CreateBugForTest` queues a `bug_create` and a placeholder bug + link;
    `DiscardPendingChange` restores (bug + link + row gone).
  - `demoBugs` generates cross-project bugs linked to FAILED demo tests.
  - `ReplaceAllBugs` / `ReplaceAllBugLinks` reconcile the cache; `GetTestBugs`
    and `ListBugsWithTests` return the expected joins.
  - `commitBugCreates` against a fake client: creates the issue, renames
    temp→real, creates the link, clears the pending row.
- Frontend: `npm run build` (tsc + vite) + `wails build`; demo click-through —
  file a bug on a failed demo test, confirm it appears in Pending Changes, in
  the Containers→Bugs panel, and on the test detail as a browser hyperlink.

## Build order (for the plan)

1. Store schema (tables + indexes + `schemaVersion` bump) and Go types.
2. `bugs.go` read/replace methods + `bugcrud.go` create/rename/discard + entity
   constant.
3. `jira/bugs.go` (demo `ListBugs`/`CreateBug`/`demoBugs` + real-path NOTE
   stubs); `syncer` sync + commit passes.
4. `app.go` bindings; `api.ts` exports/interfaces.
5. Test-detail Bugs section (part 3).
6. CreateBugModal + execution-board action (part 1).
7. Bugs panel + Containers mode toggle (part 2).
8. PendingChangesModal description; full build + demo verification.
