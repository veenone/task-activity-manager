# Task Activity Manager: issues (Phase 1) design

**Status:** proposed · **Date:** 2026-09-05
**Parent:** [`2026-09-04-tam-foundation-design.md`](2026-09-04-tam-foundation-design.md), which fixes the repository shape, the shared core, the data model, and the write model this spec builds on.
**Mockups:** [`assets/2026-09-05-tam-backlog-read-path.svg`](assets/2026-09-05-tam-backlog-read-path.svg) is what plan 1a ships: the Backlog with a read-only detail panel and a sync in progress. [`assets/2026-09-04-tam-shell-backlog.svg`](assets/2026-09-04-tam-shell-backlog.svg) is the same view once plan 1b adds the write controls. [`assets/2026-09-06-tam-pending-changes.svg`](assets/2026-09-06-tam-pending-changes.svg) is plan 1b's Pending changes dialog with one issue held back by a conflict. [`assets/2026-09-06-tam-import-and-links.svg`](assets/2026-09-06-tam-import-and-links.svg) is plan 1c's Import issues dialog and the Links tab's Add link form.

## 1. What this phase delivers

Phase 1 is the first feature subsystem: issues. Everything TAM does later is a view over issues, so this phase proves sync, the local cache, and the hybrid write model on real data. It is one spec and two plans, in the same shape as Phase 0.

**Plan 1a, the read path.** The shared Jira client lifted from XTM; TAM's issue backend with a Jira and a demo implementation; the `issue` and `issue_link` tables; sync by project; the Backlog grid with search, type chips, a sprint filter, and paging; a read-only detail panel with Details, Links, and Tests tabs; and the demo dataset that drives all of it offline. Nothing in 1a writes to Jira.

**Plan 1b, the write path.** The shared journal lifted from XTM (`pending_change`, `audit_log`, the coalescing upsert, and the audit writer); edit and create through the journal; Commit with the base-version conflict check and the override and keep-remote resolutions; the pending-change dot and the Commit chip; and the detail panel's Activity tab. Section 13 is its design.

**Plan 1c, the write features.** Excel import on XTM's importer, cross-project links, and requirement creation against Jira's create-meta fields, all riding on 1b's journal. Section 14 is its design.

## 2. Decisions

| Decision | Choice | Why |
|---|---|---|
| Sync scope | `project = KEY AND issuetype in (task, epic, story, bug, requirement)`, narrowed by the profile's scope JQL when set, incremental on `updated` | Mirrors XTM's rule, reuses the profile fields that already exist, and keeps the local table bounded to the five types TAM manages |
| Issue type names | Task, Epic, Story, and Bug are Jira's defaults and fixed. The requirement type name is a per-profile TAM setting, default `Requirement` | Only the requirement type varies between instances. XTM names its bug type per profile for the same reason |
| Where per-profile TAM settings live | A `profile_setting` table in `tam.db` | The shared `profiles` table stays untouched, so the schema drift guard and XTM's import keep their meaning |
| Backend seam | A narrow `IssueBackend` in `tam/internal/backend`, with a Jira implementation on the shared client and a demo implementation | The real reuse is the shared transport. `core/backend` waits until both apps share an interface |
| `core/jira` extraction | Transport only: auth, TLS, request helpers, the HTTP error type, generic JQL search with paging, get issue, issue types, custom-field discovery | XTM's Xray methods stay in XTM. Landed as its own PR with XTM's suites as the gate |
| Detail panel in 1a | Read-only, three tabs: Details, Links, Tests | The Tests tab makes the requirement-to-test seam visible from day one. Activity needs the audit log, which arrives with the journal |
| Demo data | A curated, deterministic Acme Platform (PLAT) dataset in `tam/internal/demo` | XTM's generator is entangled with tests and executions. `core/demo` waits until the two apps share generator code |
| Backlog grid | TAM's own `IssueTable`, following XTM's table patterns | XTM's `TestTable` is 990 lines tied to selection and review features TAM does not have |
| Frontend sync state | XTM's `syncMachine` reducer moves to `frontend/core`; XTM's `SyncProvider` stays in XTM | The reducer is pure and tested. The provider's API carries commit state, which 1a does not have; the plan decides between a small TAM-local provider (the default) and generalising XTM's |

## 3. Shared code changes

### 3.1 `core/jira`

A new package holding what both apps need to talk to Jira DC. Lifted from `xtm/internal/jira`, not rewritten.

- `Client`: base URL, PAT bearer auth, TLS options built from the profile's CA certificate and its allow-untrusted flag, and a configurable `http.Client`.
- Request helpers: `Get`, `Post`, `Put` returning decoded JSON, with one place that turns a non-2xx response into `*HTTPError` (status, method, path, and the response body's message list).
- `SearchIssues(ctx, jql string, fields []string, startAt, maxResults int) (SearchPage, error)`: one page of `/rest/api/2/search`. `SearchPage` carries `Issues []RawIssue` (key, id, and the `fields` object as `map[string]json.RawMessage`) and `Total`.
- `GetIssue(ctx, key string, fields []string) (RawIssue, error)`.
- `IssueTypes(ctx, projectKey string) ([]IssueType, error)`: the project's issue types by name and id, from `/rest/api/2/project/{key}`.
- `CustomFieldID(ctx, name string) (string, error)`: the `customfield_NNNNN` id for a field name, from `/rest/api/2/field`, cached per client. Returns `ErrFieldNotFound` when the instance has no such field.
- `Myself(ctx) (User, error)`: the connection test.

XTM's `jira.Client` embeds `*corejira.Client` and keeps every Xray, container, folder, precondition, and bug method as they are. XTM's demo short-circuits stay in XTM. The extraction PR changes no XTM behaviour; its proof is XTM's full Go suite and its Vitest suite staying green.

### 3.2 `frontend/core`

`syncMachine.ts` (the `SyncStatus` and `SyncMachineState` types, `syncReducer`, and the `canSync`, `canCommit`, and `canSwitchProfile` guards) moves in with its test, and XTM's path becomes a re-export shim. Nothing else moves.

## 4. TAM Go packages

### 4.1 `tam/internal/backend`

```go
type IssueBackend interface {
    TestConnection(ctx context.Context) (User, error)
    IsDemo() bool
    // SearchIssuesPage returns one page of issues in projectKey whose type is
    // in types, narrowed by scopeJQL when non-empty and by updated >= since
    // when non-empty, plus the total match count.
    SearchIssuesPage(ctx context.Context, projectKey, scopeJQL, since string, types []string, startAt, maxResults int) ([]Issue, int, error)
    // GetIssueDetail fetches the fields the grid does not carry: description,
    // links, and the discovered custom fields.
    GetIssueDetail(ctx context.Context, key string) (IssueDetail, error)
    IssueTypes(ctx context.Context, projectKey string) ([]IssueType, error)
}
```

`Issue` carries the grid columns: key, id, project, type (the logical type, one of `task`, `epic`, `story`, `bug`, `requirement`), summary, status, assignee, reporter, priority, labels, sprint id and name, parent key, story points, rank, and the remote created and updated times. `IssueDetail` adds description, the `Link` list (direction, Jira link type name, other key, other summary, other type), and the raw custom-field map. `IssueType` is name and id. `User` is name and display name.

### 4.2 `tam/internal/backend/jira`

Wraps `core/jira`. On construction it resolves the profile's requirement type name and discovers the custom-field ids for `Sprint`, `Story Points`, and `Epic Link` by name, tolerating absence: a missing field leaves that column empty and logs one warning per profile. The JQL it builds is the decision-table rule; type names are quoted. Field mapping:

- Logical type: the issue type name compared case-insensitively against `Task`, `Epic`, `Story`, `Bug`, and the profile's requirement type name. Anything else is skipped and counted in the sync summary. The JQL never returns such issues, so the guard only matters for the demo backend and for future type overrides.
- Sprint: Jira DC returns the Sprint field as opaque strings on older Jira Software versions (`...Sprint@1a2b[id=12,name=Sprint 12,state=ACTIVE,...]`) and as objects with `id`, `name`, and `state` on newer ones. The parser accepts both and takes the last entry as the current sprint. Marked `NOTE(tam)` for verification against a real instance.
- Parent key: `fields.parent.key` when present, else the Epic Link custom field's value.
- Story points: the custom field's number, empty when absent.

### 4.3 `tam/internal/backend/demo`

Serves the dataset from `tam/internal/demo` through the same interface. `SearchIssuesPage` ignores `since` and the scope, as XTM's demo does, so an incremental sync still returns the full set. `GetIssueDetail` returns the curated description and links.

### 4.4 `tam/internal/demo`

A deterministic Acme Platform (PLAT) dataset: four epics, about a dozen stories and requirements with hand-written summaries that match the mockup (checkout, promotions, guest checkout), and seeded filler up to roughly 60 issues across the five types with a spread of statuses, assignees, sprints 11 to 13, priorities, labels, and story points. A few stories and requirements carry `tested by` links to `XT-` keys so the Tests tab has content. Keys, ids, and timestamps are fixed so tests can assert on them.

### 4.5 `tam/internal/issuerepo`

The store layer over `tam.db`, schema version 2 (section 5). It offers upsert by profile and key, replace-all for a full sync, the paged query behind the grid, detail read and write with a fetch timestamp, link replace per issue, the distinct sprint list for the filter, sync-state read and write, and profile settings.

### 4.6 `tam/internal/syncer`

`Engine.Sync(ctx, profileID, projectKey, scopeJQL string, full bool, onProgress func(Progress)) (Summary, error)`. It tests the connection, computes `since` from the profile's last sync (empty for a full sync), pages through `SearchIssuesPage` upserting each page inside one transaction per page, and records the sync state at the end. `Progress` carries stage, fetched, and total; the app forwards each one as a Wails event. `Summary` carries fetched, upserted, skipped, and the elapsed time. A page failure after at least one page succeeded returns `*PartialSyncError` with the pages done and the underlying error; rows already upserted stay, and the sync state is not advanced.

### 4.7 Bound methods on `App`

- `SyncIssues(profileID string, full bool) (syncer.Summary, error)`: runs to completion, emitting `tam:sync-progress` events.
- `GetSyncState(profileID string) (issuerepo.SyncState, error)`.
- `ListIssues(profileID string, q issuerepo.IssueQuery) (issuerepo.IssuePage, error)`: `IssueQuery` is text, types, sprint id, offset, and limit; `IssuePage` is the rows and the total.
- `GetIssueDetail(profileID, key string) (issuerepo.IssueDetail, error)`: returns the cached detail immediately when fresh (fetched within the last ten minutes), otherwise fetches through the backend, caches, and returns. The frontend calls it once on panel open and again on an explicit refresh.
- `ListLinkedTests(profileID, key string) ([]issuerepo.LinkedTest, error)`: the issue's links whose type matches the shared requirement link type setting, each with key and summary.
- `ListSprints(profileID string) ([]issuerepo.SprintRef, error)`: the distinct sprints in the cached issues, for the filter.
- `GetProfileSetting(profileID, key string) (string, error)` and `SetProfileSetting(profileID, key, value string) error`. The one key in 1a is `requirement_issue_type`.

`app.go` stays thin: it validates ids, resolves the backend for the profile (demo when the URL is `demo`, Jira otherwise, with the PAT read from the credential store), and delegates.

## 5. Data model

Schema version 2 of `tam.db`. Every table carries `profile_id`.

| Table | Columns |
|---|---|
| `issue` | `profile_id`, `key`, `id`, `project`, `type`, `summary`, `status`, `assignee`, `reporter`, `priority`, `labels` (JSON array), `sprint_id`, `sprint_name`, `parent_key`, `story_points`, `rank`, `created`, `updated`, `synced_at`, `detail_json`, `detail_fetched_at`. Primary key `(profile_id, key)`; index on `(profile_id, type)` and `(profile_id, sprint_id)` |
| `issue_link` | `profile_id`, `from_key`, `to_key`, `link_type`, `direction` (`outward` or `inward`), `to_summary`, `to_type`. Primary key `(profile_id, from_key, to_key, link_type, direction)` |
| `sync_state` | `profile_id`, `last_synced`, `last_full`, `last_error`. Primary key `profile_id` |
| `profile_setting` | `profile_id`, `key`, `value`. Primary key `(profile_id, key)` |

`board`, `sprint`, and `ritual_doc` from the foundation spec arrive with their phases. `pending_change` and `audit_log` arrive with plan 1b.

## 6. Sync behaviour

- **Incremental** is the default. `since` is the profile's `last_synced`, so only issues updated after it are fetched and upserted. An issue that left the project or changed to a type outside the five stays cached until the next full sync, the same trade-off XTM makes.
- **Full** clears the profile's `issue` and `issue_link` rows inside the same transaction as the first page's upsert, then proceeds like an incremental sync with an empty `since`. This is how deletions in Jira are reconciled.
- **Custom-field discovery** runs once per backend construction and is cached for the app's lifetime. Absence degrades to empty columns, never to a failed sync.
- **Links** are not fetched during sync. They come with the detail fetch and are replaced per issue at that point, so the Tests tab reflects Jira as of the last time the panel opened. Plan 1b, which needs links for cross-project work, decides whether to add them to the bulk sync.
- **Progress** is one event per page plus a start and a finish event.

## 7. Frontend

TAM's app gains, all TAM-local under `tam/frontend/src`:

- `queries/`: TanStack Query keys and hooks for issues (keyed by profile and query), issue detail, linked tests, sprints, and sync state, following XTM's `queries/` layout. Invalidation after a sync clears the profile's issue, sprint, and sync-state keys.
- `components/BacklogView.tsx` and `components/IssueTable.tsx`: the mockup's seven columns on react-virtual, a search box that matches summary, key, and label, five type chips that toggle, a sprint dropdown built from `ListSprints`, and 25-row paging. Filter and page state live in the view component and reset on profile switch. Row click opens the panel; there is no selection column in 1a.
- `components/IssueDetailPanel.tsx`: header with key, type chip, and summary; Details (status, assignee, sprint, story points, epic, priority, labels, updated, and the description), Links (every link grouped by type and direction), and Tests (the linked tests, labelled "via XTM" with the link type name). Opening the panel calls `GetIssueDetail`; the panel shows the cached row's fields at once, a loading state for the detail, and an inline error with a retry when the fetch fails. A refresh control forces a fetch.
- Sync: the topbar Sync button calls `SyncIssues` (a menu offers Full sync), subscribes to `tam:sync-progress`, and drives the sync machine. The status bar shows the issue count and last sync time from `GetSyncState`, a progress chip while syncing, and the last error when there is one. The pending-changes cell arrives with 1b.
- Profile settings: the Profiles dialog gains a "Requirement issue type" field per profile, read and written through the profile-setting methods.

## 8. Errors

- A sync starts with the connection test; an auth or network failure stops before any page is fetched and is shown as a notice with the normalised message.
- A page failure mid-sync surfaces as a partial sync: the status bar shows the rows that landed and the error, the sync state is not advanced, and a notice offers to retry, which resumes from the same `since`.
- A failed detail fetch stays inside the panel with a retry; the cached fields remain visible.
- Every backend error passes through the shared `normalizeError`, so messages read the same in both apps.
- Custom-field discovery failure is a logged warning and empty columns, never an error the user sees during sync.

## 9. What 1a leaves in place for 1b

- `issue.detail_json` holds the raw field map, so the edit form can build on the same shape the detail panel reads.
- `issue.updated` is the base version the journal's conflict check compares against.
- `IssueBackend` grows `CreateIssue`, `UpdateIssue`, and `LinkIssues` in 1b; the interface is small enough now that adding them is one reviewable change.
- The sync machine's `committing` state and `canCommit` guard exist in the shared reducer already; 1b wires them.
- The grid's row model reserves a `pending` flag the table renders as the amber dot once the journal exists.

## 10. Verification

- Go tests per package against a temp SQLite file and the demo backend: upsert and replace semantics, the paged query with each filter, detail caching and staleness, link replacement, sync state, full versus incremental, and partial failure handling. `backend/jira` gets an `httptest` server test for the JQL it builds, the type mapping, both Sprint field shapes, and the parent-key fallback.
- The `core/jira` extraction PR keeps XTM's full Go suite and its 191 Vitest tests green; that is the proof nothing moved.
- Vitest in `tam/frontend` covers the grid's filtering and paging against a mocked `ListIssues`, the panel's three tabs plus its loading and error states, and the sync flow from button to progress chip to invalidation. `frontend/core` keeps `syncMachine`'s tests running from their new home.
- The demo profile walks the whole UI offline: sync, filter, page, open an issue, read its links and tests.

## 11. Risks

| Risk | Mitigation |
|---|---|
| Lifting the transport out of XTM's `jira` package changes behaviour | Embed, do not rewrite; XTM's suites gate the extraction PR; every XTM method keeps its body |
| Sprint and Epic Link shapes differ across Jira DC versions | Both known shapes handled and marked `NOTE(tam)`; a missing field is an empty column, not a failure |
| The grid becomes a second copy of `TestTable` | It is written for TAM's columns from the start and shares only the CSS primitives and the virtualiser; when XTM's chrome adopts the shared shell, the table is a candidate to unify |
| Incremental sync misses moved or deleted issues | Documented, same as XTM; Full sync is one menu item away |
| 1b's journal needs a shape 1a did not keep | Section 9 lists what 1a preserves on purpose; 1b's plan starts by checking it |

## 12. Out of scope for this phase

Sprint and board entities and the agile client (Phase 3), the epic tree (Phase 2), reports (Phase 4), Confluence (Phase 5), the cross-link views inside XTM (Phase 6), and any non-Jira backend for TAM.

## 13. Plan 1b design: the write path

Decided 2026-09-06 after 1a merged. Everything here rides on 1a's shapes; section 9's promises were checked first: `issue.updated` is the base version, `detail_json` holds the discovered custom fields (enough for the six editable fields), the `IssueBackend` grows by three methods, and the row shape gains the `pending` flag the grid reserved.

### 13.1 Scope

Edit and create through the journal, Commit with conflict detection, the pending markers, and the Activity tab. Editable fields are summary, description, priority, labels, story points, and assignee. Creatable types are task, story, and bug; requirement creation, Excel import, and cross-project links are plan 1c. Status and sprint moves are the live path and arrive with boards in Phase 3.

### 13.2 `core/journal`

Lifted from XTM's `testrepo`: the `pending_change` and `audit_log` tables and the two transaction-level helpers behind every XTM edit. `Upsert(tx, profileID, entityType, entityKey, field, before, after, baseVersion)` keeps one row per entity and field: a first edit inserts, a later edit updates `after_val`, and a value returning to its original deletes the row. `Audit(tx, profileID, entityType, entityKey, action, field, before, after, note)` appends to the trail. The package also exports the DDL both apps' stores include, the `PendingChange` and `AuditEntry` shapes, `List(db, profileID)`, `Discard(db, profileID, id)`, `DiscardAll`, `Delete(tx, ids)` for a committed batch, and `Entries(db, profileID, entityKey, limit)`. XTM's `upsertPendingChange` and `writeAudit` become one-line delegators, the same shape as the `jira` embedding; its list and discard methods keep their bodies. The extraction is its own PR, gated by XTM's full Go and Vitest suites.

### 13.3 TAM store, schema version 3

`tam.db` gains the two journal tables from the shared DDL. `issuerepo` gains:

- `EditField(ctx, profileID, key, field, value)`: in one transaction, read the row's current value and `updated`, write the new value to the row, journal it with `updated` as the base version, audit it. Unknown or read-only fields are rejected.
- `CreateDraft(ctx, profileID, draft)`: insert a row under the next temporary key (`TAM-NEW-1`, `TAM-NEW-2`, per profile) with status `Draft`, plus one pending row of entity type `issue_create` whose `after_val` is the draft as JSON, and an audit entry. Returns the temporary key.
- `PendingKeys(ctx, profileID)`: the keys with at least one pending row, for the grid's dot and the `pending` flag on `Issue`.
- `Rekey(ctx, profileID, tempKey, realKey)`: moves a draft to its real key across `issue`, `issue_link`, `pending_change`, and `audit_log` in one transaction.
- `ReplaceRow(ctx, profileID, issue)`: replaces a row from a fresh Jira read after a commit or a keep-remote.

Sync never touches drafts or pending rows: a full sync's clear skips keys with the temporary prefix, and an upsert of a key with pending rows does not overwrite the locally edited columns (the pending `after_val` wins until Commit).

### 13.4 Backend additions

`IssueBackend` grows `UpdateIssue(ctx, key string, fields map[string]any) error`, `CreateIssue(ctx, projectKey, typeName string, fields map[string]any) (string, error)`, and `CreateFields(ctx, projectKey, typeName string) ([]FieldSpec, error)`. The Jira implementation maps the six fields to Jira's shapes (priority and assignee as `{name}`, labels as a list, story points through the discovered custom field id) and PUTs `/rest/api/2/issue/{key}`; creates POST `/rest/api/2/issue`; `CreateFields` reads `createmeta` with the fields expansion and returns the required fields the form does not already carry as name, id, and schema type. The demo backend accepts all three in memory, hands out keys from `PLAT-500` upward, bumps `updated` on every write, and can stage a conflict (a demo issue whose remote `updated` moves after an edit) so the resolution dialog is testable offline.

### 13.5 The commit pass

`tam/internal/committer`, not XTM's engine. It groups the profile's pending rows by key and runs creates first: POST, then `Rekey`. For each edited issue it fetches the remote `updated`; a mismatch holds the issue back as a conflict carrying base, mine, and remote for every pending field, read from the fetched issue. Otherwise it PUTs the fields, replaces the row from Jira, deletes the pending rows, and audits `committed`. A failed PUT or POST keeps the rows and records the error. `Result` lists committed keys, created keys with their temporary ones, conflicts, and failures, plus counts. Override rewrites the held issue's base versions to the remote `updated` so the next Commit pushes; keep remote deletes its pending rows and replaces the row from Jira. Commit and sync exclude each other through the shared reducer's states and a per-profile in-flight guard on the Go side.

### 13.6 Bound methods

`EditIssue(profileID, key, field, value string) error`, `CreateIssue(profileID string, draft IssueDraft) (string, error)`, `GetCreateFields(profileID, typeName string) ([]backend.FieldSpec, error)`, `ListPendingChanges(profileID string) ([]journal.PendingChange, error)`, `DiscardPendingChange(profileID string, id int64) error`, `DiscardAllPendingChanges(profileID string) (int, error)`, `CommitPendingChanges(profileID string) (committer.Result, error)`, `ResolveConflictOverride(profileID, key, remoteVersion string) error`, `ResolveConflictKeepRemote(profileID, key string) error`, `ListActivity(profileID, key string, limit int) ([]journal.AuditEntry, error)`.

### 13.7 Frontend

- The Details tab becomes editable: inputs for the six fields and a Save edit button that journals each changed field; a Draft chip on rows and panels of uncommitted creates.
- A New button on the Backlog opens the New issue dialog: type, summary, description, priority, labels, assignee, then the create-meta required fields rendered as text inputs. The draft appears in the grid at once.
- Rows with pending rows show the amber dot from the mockup; `Issue.pending` drives it.
- The status bar's Commit chip shows the pending count and opens the Pending changes dialog: changes grouped by issue with before and after, per-row discard, discard all, and Commit. The commit result lands in a banner naming committed, created, conflicted, and failed keys.
- Conflicts open the resolution dialog: one card per held issue with a base, mine, remote table and Override and Keep remote buttons.
- The Activity tab lists the audit entries for the issue, newest first.
- The sync reducer's `committing` state is wired: Commit disables Sync and the profile picker, and the reverse.

### 13.8 Errors

Commit is per issue; one failure or conflict never blocks the others, and the rows of anything not committed stay for the next attempt. A create whose POST succeeded but whose rekey failed is reported with the real key and audited, so the next sync reconciles rather than duplicates it. A create-meta failure degrades the New issue dialog to the minimal form with a note. A partial commit shows the banner and leaves the chip's count at what remains. Every backend error passes through the shared normaliser.

### 13.9 Verification

Go: `core/journal` against a temp file (insert, update, revert-drops-row, audit, list, discard, delete); `issuerepo` edit, draft, rekey, replace, pending keys, and the sync guards for drafts and pending rows; `committer` against a fake backend for the clean path, a conflict with both resolutions, a create with rekey, and a mid-run failure; the demo backend's writes and its staged conflict. The `core/journal` PR keeps XTM's suites green. Vitest covers the editable panel, the New issue dialog, the pending dialog, the conflict dialog, the Activity tab, and the reducer wiring. The demo profile runs the loop offline: edit, create, see the dot and the chip, commit, force a conflict, resolve it both ways.

### 13.10 Implementation notes

Recorded when plan 1b was written. `core/journal` exposes `Get`, `Delete`, `DeleteForKey`, and `SetBaseVersion` keyed by entity key rather than a `Discard`, because the revert of an entity's columns belongs to each app. `IssueBackend` grew a fourth method, `GetIssue`, for the version check and the row refresh, and `UpdateIssue` takes the journal's text values keyed by logical field so the backend owns Jira's shapes. Description edits live in the cached detail, since the row has no description column. The conflict cards render inside the Pending changes dialog, as the mockup shows, rather than in a separate dialog. The demo's staged conflict is the curated story rekeyed to the profile's project, held back once.

## 14. Plan 1c design: the write features

Decided 2026-09-06 after 1b merged. Everything here rides on 1b's journal, drafts, and commit pass. The user chose drafts through the journal for import, link creation only (no removal, no links in the bulk sync), the requirement type in the existing New issue dialog, XTM's import flow trimmed to TAM's fields, and one plan with the file parser lifted into `core` first.

### 14.1 Scope

Three features. Excel or CSV import that turns rows into drafts. Link creation from the Links tab, journaled and pushed on Commit. Requirement creation as a fourth creatable type. Deferred: link removal (needs link ids the detail fetch does not keep), links in the bulk sync, a per-row preview table in the import dialog, epic creation, and subtask parents.

### 14.2 `core/importfile`

Lifted from XTM's `testrepo/importcsv.go`: `ParseRecords(data []byte, isXlsx bool) ([][]string, error)` (CSV with the UTF-8 BOM stripped, or the first worksheet of an XLSX through excelize), `Preview{Headers []string; RowCount int}` and `ParsePreview(records) (Preview, error)`. XTM's `ParseRecords`, `ParseImportPreview`, `readCSV`, `parseXLSX`, and `stripUTF8BOM` become delegators and its `ImportPreview` type an alias, so no caller changes; `core` gains the excelize dependency. Own PR, gated by XTM's Go and Vitest suites, the same shape as the `core/jira` and `core/journal` lifts.

### 14.3 Import

`tam/internal/importer`. A `Mapping` names the column header for each of eight fields: type, summary, description, priority, labels, assignee, story points, parent key; an empty name means unmapped. `AutoMap(headers)` matches headers case-insensitively with spaces and underscores ignored and a few synonyms ("Issue Type" for type, "Points" and "Story Points" for story points, "Epic", "Epic Link", and "Parent" for the parent key). Validation per row: summary required; type blank or unmapped means task, otherwise it must be one of task, story, bug, requirement, or the profile's requirement type name, case-insensitively; story points numeric when present; a parent key must exist in the profile's cache; labels split on commas. `Run(ctx, repo, profileID, projectKey, requirementType, records, mapping, fileName, dryRun) (Result, error)` returns `Result{Rows int; Created []string; Errors []RowError{Row int; Message string}}`. A dry run only validates. An import creates every valid row's draft in one transaction through a new `CreateDrafts(ctx, profileID, projectKey string, drafts []IssueDraft, note string) ([]string, error)` on the repository, whose audit entries carry "imported from <file name>"; rows with errors are skipped and listed. `IssueDraft` gains `ParentKey`; a draft row stores it in `parent_key`; the Jira backend sends it through the discovered Epic Link field when the type is not an epic and the field exists, and drops it with a log line otherwise. A template CSV (headers Type, Summary, Description, Priority, Labels, Assignee, Story Points, Parent, plus one example row) is saved through the Wails save dialog.

### 14.4 Links

A pending link is a journal row of entity type `link`: `entity_key` is the source issue, `field` is `<type name>|<direction>|<target key>` (so the journal's uniqueness rejects a duplicate), `after_val` is the link as JSON (`LinkDraft{Type, Direction, ToKey, ToSummary, ToType}`, direction `outward` or `inward`). There is no base version; links do not conflict.

`IssueBackend` grows `LinkTypes(ctx) ([]LinkType{Name, Inward, Outward}, error)` and `CreateLink(ctx, fromKey string, d LinkDraft) error`. The Jira implementation reads `/rest/api/2/issueLinkType` once per backend and POSTs `/rest/api/2/issueLink` with the source as `outwardIssue` for an outward link and as `inwardIssue` for an inward one, the shape XTM's `CreateBugLink` uses. The demo defines three types (Relates: "relates to" both ways; Blocks: "blocks" and "is blocked by"; Tested By: "tests" and "is tested by"), keeps created links in an overlay that `GetIssueDetail` merges, and answers `GetIssue` for the `XT-` keys its curated details reference with synthetic Test issues, so a cross-project link can be checked offline.

The repository gains `AddLink(ctx, profileID, key string, d LinkDraft) error` (journals with `Put`, audits `link`), `PendingLinks(ctx, profileID, key) ([]Link, error)`, and `ReadDetail` merges pending links into the detail's links with `Link.Pending = true`. Discarding a link row deletes it and audits; nothing to revert. The committer handles `link` rows after edits: `CreateLink`, `MarkCommitted`, then the detail cache for the source key is dropped so the panel refetches. Failures are recorded like edits.

### 14.5 Requirement creation

`requirement` joins the creatable types. The New issue dialog hides Story points when the type is requirement, and the demo's `CreateFields` asks for one required string field, Source, on requirements. Commit already maps the logical type to the profile's requirement type name.

### 14.6 Bound methods

`PreviewImport(contentB64 string, isXlsx bool) (importfile.Preview, error)`, `AutoMapImport(headers []string) importer.Mapping`, `ImportIssues(profileID, contentB64 string, isXlsx bool, fileName string, mapping importer.Mapping, dryRun bool) (importer.Result, error)`, `SaveImportTemplate() (string, error)`, `GetLinkTypes(profileID string) ([]backend.LinkType, error)`, `LookupIssue(profileID, key string) (backend.Issue, error)`, `AddLink(profileID, key string, link backend.LinkDraft) error`. File contents travel base64-encoded from a browser file input, as XTM's import does. Import and Commit exclude each other through the `busy` guard.

### 14.7 Frontend

- An Import button beside New on the Backlog opens the Import issues dialog: file input (CSV or XLSX), the header row and row count, one select per field pre-filled by `AutoMapImport`, a Dry run button whose result lists row errors, an Import button whose result says how many drafts were created and lists the skipped rows, and a Download template link. Drafts appear in the grid at once.
- The Links tab gains an Add link form: a select of link phrasings (each type's outward and inward wording), a target key input with a Check button that shows the target's summary and type through `LookupIssue`, and Add. Pending links list with the amber dot and a Discard action; the tab's refresh keeps them.
- The Pending changes dialog shows a link card ("PLAT-412 relates to XT-1018" with the target's summary) with Discard; it counts toward Commit (n).
- The Activity tab words link entries: added, pushed, discarded.
- The New issue dialog offers Requirement.

### 14.8 Errors

An import never partially fails: the dry run and the import validate every row first, skip the invalid ones, and create the rest in one transaction; a parse error (bad file, empty file) stops the dialog with the message. A link to a key `LookupIssue` cannot find is rejected at Check; a duplicate pending link is rejected with "already pending". Link pushes that fail keep their rows and land in the commit banner with Jira's message.

### 14.9 Verification

Go: `core/importfile` against a CSV with a BOM and an XLSX built with excelize in the test; `importer` for auto-mapping, every validation rule, dry run, and an import that creates drafts with parents; the repository's `CreateDrafts`, `AddLink`, pending link merge, and discard; both backends' link types and link creation (httptest for Jira, including the direction mapping); the committer pushing a link and recording a failure; XTM's suites for the parser lift. Vitest: the import dialog (mapping, dry run errors, import result), the Add link form (check, add, duplicate), the pending link card, the requirement type in the New dialog. The demo profile runs it offline: import the template, link PLAT-412 to XT-1018, create a requirement with a Source, Commit.

### 14.10 Mockup

[`assets/2026-09-06-tam-import-and-links.svg`](assets/2026-09-06-tam-import-and-links.svg): the Import issues dialog after a dry run, and the Links tab with the Add link form and a pending link.

### 14.11 Implementation notes

Recorded when plan 1c was written. `core/importfile` exports the parser pieces XTM used privately so the delegators are one-for-one. `CreateDrafts` is the one transaction; `CreateDraft` calls it with a single draft. The parent key reaches Jira through the discovered Epic Link field only. Link rows are pushed in their own pass, read fresh after creates so a link from a draft follows the real key; `Link` carries `pending` and `pendingId` so the Links tab discards with the existing `DiscardPendingChange`. `LookupIssue` is the backend's `GetIssue`; the demo answers it for foreign `XT-` keys with synthetic rows.
