# TAM Rituals, local first: Sync, a page tree TAM creates, and an editor

The Rituals view shipped with two models that never met. Associations linked a
Confluence page somebody else wrote, and the view fetched that page live every
time it was opened. Local drafts held a remark and a list of issues, built
through a wizard, and nothing ever published them. This design replaces both
with one model: the ritual page lives in `tam.db`, is edited in TAM, and
reaches Confluence only when the user presses Sync.

It supersedes `2026-09-13-tam-rituals-authoring-design.md` on three points,
each a decision taken during brainstorming on 2026-09-14:

1. **TAM owns the whole page.** The old design split a page into TAM-owned
   blocks between markers and human prose around them, and made a marker
   probe a prerequisite. That probe never ran. With an editor in TAM, people
   write prose here, so the split has nothing left to protect, and the
   markers, the splice, and their failure modes all go.
2. **Sync, not Commit.** Local edits reach Confluence through a Sync button in
   the Rituals view that pulls, creates, and pushes in one pass. They are not
   journaled.
3. **Jira issues are a Confluence macro.** Pages carry Jira Issues macros, not
   a table TAM renders, so the issue list never drifts and needs no
   republishing. The wizard, per-issue remarks, and manual associations are
   retired.

## What exists today

- `tam/app_rituals.go`: association CRUD, `GetRitualPage` (live first, falling
  back to `confluence_page_cache` in `profiles.db`), draft CRUD,
  `ScaffoldSprintRituals`, `ListSprintIssues`.
- `tam/app_confluence.go`: config get and set, `GetConfluencePage`,
  `ListConfluenceChildPages`, `confluenceClient`.
- `core/confluence`: read only. `GetPage`, `ListChildPages`, `DemoPages`.
- `tam/internal/ritualrepo`: `ritual_document` in `tam.db`, keyed
  `(profile_id, board_id, sprint_id, ritual_type)`, with `SeedDemo`.
- `tam/internal/ritualdefaults`: per-type issue selection for the wizard.
- `RitualsView.tsx` calls `GetConfluenceConfig` and then
  `ListConfluenceChildPages` on mount and discards the child list
  (`RitualsView.tsx:84`), and opens associations through `GetRitualPage`.
  `ProfileForm.tsx` also lists child pages live for its association picker.

## 1. Storage

### Schema version 11

`ritual_document` stays the one table. Its columns are redefined or added:

| column | meaning |
|---|---|
| `body` | **the local page body**, Confluence storage XHTML. Redefined; nothing read it before. |
| `base_body` (new) | the body as of `confluence_version`, the last remote TAM synced with |
| `confluence_page_id` | the page, empty until created or adopted |
| `confluence_version` | the base version, the remote version `base_body` came from |
| `conflict_body` (new) | a newer remote body a Sync found while local edits were pending |
| `conflict_version` (new) | that body's version |
| `status` | `local`, `synced`, `unsynced`, `conflict`, `gone` |
| `published_at` | the last time this row was synced |
| `title` | the page title; not editable, because Sync matches by it |
| `remark`, `issues_json` | left in place and no longer read |

The migration adds the three new columns with `AddColumnIfMissing`, the shape
versions 7, 8 and 10 use. It rewrites no rows: it has neither sprint names nor
the project to render a template from. Rows are upgraded by the ensure step
below. Test it the way version 10 is tested: rewind the recorded version,
reopen, assert the columns.

**Dirty is computed, never stored.** A row is dirty when
`confluence_page_id = ''` or `body <> base_body`. `status` is written by the
operations that change it, and `unsynced` is written by `SaveRitualBody` when
the saved body differs from `base_body`, but no decision in Sync reads
`status` to find dirty rows. A flag would drift from the bodies; the
comparison cannot.

`ritual_type` gains a fifth value, `_sprint`, for the sprint overview page.
The primary key already fits it. `knownRitualType` accepts it.

`ritual_document` is already in `PurgeProfile` (`boardrepo.go:85`), so the new
columns go with the profile without a change there.

### Creating pages starts locally

`EnsureSprintRituals(profileID, boardID, sprintID)` writes whichever of the
sprint's five rows are missing, each with its template rendered into `body`,
`base_body` empty, and status `local`. It is a local write, takes no lock, and
makes no network call, so a planning page can be written on a train and Sync
creates the page from it later. The view calls it when a sprint is selected.
It never touches a row that already has a body.

A row with an empty `body` and an empty `confluence_page_id` counts as
missing. That is the upgrade path for rows written before version 11: the
eight rows `SeedDemo` wrote and any the wizard saved. When such a row carries
a non-empty `remark` or an `issues_json` holding any issue with a remark, the
rendered body gains a final H2 section, **Earlier draft notes**, holding the
remark as a paragraph and a bullet per issue as `KEY: remark`. Text somebody
typed into the wizard survives the retirement of the wizard.

## 2. The Sync pass

### The binding

`SyncRituals(profileID string, boardID int) (ritualsync.Result, error)` in
`app_rituals.go`, calling `internal/ritualsync`. It takes
`a.acquire(p.ID, "rituals")`.

Refused before the pass starts, as a Go error run through `errtext`:
Confluence URL, space key, or root page id not configured; no token in the
credential store; the root page not readable (*"The Confluence root page
123456 could not be read: 403 Forbidden"*).

Everything that goes wrong on a single page is reported inside `Result`,
because Wails fills in either a bound method's value or its error and never
both, which is the rule `sprints.Completion` already works around:

```go
type Result struct {
    Created   int
    Pulled    int
    Pushed    int
    Conflicts int
    Gone      int
    Failed    []PageFailure // {SprintName, Title, Reason}
    SyncedAt  string
}
```

The last sync time per board is kept as the profile setting
`rituals_last_sync:<boardID>`, written at the end of every pass that started.

### Which sprints

Every active or future sprint of the board, read from the sprint cache, plus
every sprint that already has `ritual_document` rows on this board. Closed
sprints never get new pages; a closed sprint that already has pages keeps
syncing them.

### Titles

The sprint overview page is titled with the sprint name. Each ritual page is
`<sprint name> · <label>`, labels `Planning`, `Standup`, `Review`,
`Retrospective`. Confluence Data Center requires titles to be unique within a
space, so lookup is space wide, not a walk of the root's children.

### Per sprint, in order

1. **The sprint page.**
   - Row has a page id: go to step 3 for it.
   - No page id: `FindPageByTitle(space, sprintName)`.
     - Not found: `CreatePage(space, rootID, title, body)`. Store id, version,
       `base_body = body`, status `synced`. Counted in `Created`.
     - Found, and the root page id is among its ancestors: adopt (below).
     - Found anywhere else in the space: refuse, in `Failed`, with
       *"A page titled "Sprint 14" already exists outside the rituals root.
       Rename one of them."* The sprint's ritual pages are skipped for this
       pass, since they need their parent.
2. **The four ritual pages**, the same rules, with the sprint page as parent
   and the ancestor check against the sprint page.
3. **Rows with a page id.** `GetPageStorage(id)` returns storage body and
   version.

| remote | local | action |
|---|---|---|
| version = base | clean | nothing |
| version = base | dirty | `UpdatePage(id, title, body, base+1)`, then CAS store `base_body = body`, version, status `synced`. `Pushed`. |
| version > base | clean | CAS store `body = base_body = remote`, version, status `synced`. `Pulled`. |
| version > base | dirty | CAS store `conflict_body`, `conflict_version`, status `conflict`. `Conflicts`. |
| 404 | any | status `gone`. `Gone`. Never recreated silently. |

A 409 on `UpdatePage` means the version moved between the read and the
write: read the page again and record it as a conflict.

A row already in `conflict` is refreshed by the same read: if the remote moved
again, `conflict_body` and `conflict_version` are replaced with the newer one.
It is never pushed while in `conflict`.

### Adoption

Adopting a found page reads its storage body and version.

- The local row is still exactly its template, meaning `body` equals a fresh
  `ritualtemplate.Render` for this row: take the remote page, as a clean pull.
- The local row holds anything else: store the remote as `conflict_*`, set the
  page id and `confluence_version` to the remote's, leave `base_body` empty,
  and status `conflict`. The user decides.

This is why rendering has to be deterministic (section 3): the template
comparison is how Sync tells a page nobody touched from one somebody wrote in.

### Compare and set

Every write the pass makes to a row is conditional on `body` still holding
the value the pass read at its start for that row. `SaveRitualBody` takes no
lock, so a keystroke saved while a push is in flight changes `body`, the CAS
misses, and the pass writes only the remote facts it learned
(`confluence_version`, and `base_body` set to what it pushed) without touching
`body` or claiming `synced`. The row stays dirty and the next Sync pushes it.
The same rule protects a pull: a remote body never overwrites a body that
changed after the pass read it. This is `MarkMoveCommitted`'s argument from
Phase 3b applied to a page.

### Partial failure

Pages are independent. A transport failure on one page is recorded in
`Failed` and the pass continues with the next; a failure that makes every
request fail (the connection is gone) still ends the pass with what landed
recorded, since every write is per page and already committed locally.

The pass respects `a.ctx`. It has no cancel binding: a sprint is five pages,
and a board with three open sprints is fifteen reads and a handful of writes.

### Resolving a conflict

Both resolutions are local, take no lock, and make no network call.

- `ResolveRitualConflict(…, "mine")`: `base_body = conflict_body`,
  `confluence_version = conflict_version`, clear `conflict_*`, status
  `unsynced`. The next Sync pushes the local body over the newer version.
  This is `RebaseMoves` for a page.
- `ResolveRitualConflict(…, "theirs")`: `body = base_body = conflict_body`,
  version from `conflict_version`, clear `conflict_*`, status `synced`.

For a `gone` row, `ForgetRitualPage` clears the page id and version, leaving
`body`, so the next Sync creates the page again. Removing the local copy
deletes the row, and `EnsureSprintRituals` renders a fresh template the next
time the sprint opens.

### The demo

`core/confluence` gains a `Pages` interface that both the HTTP client and an
in-memory fake satisfy. The fake lives in `tam/internal/demo`, starts with a
root page and nothing under it, so the first Sync on a demo profile visibly
creates the tree, and stages one remote edit: the first Standup page it
creates gets its version bumped, with an extra paragraph, straight after the
Sync that created it, so a user who edits the Standup locally and syncs again meets
a conflict. That is the same teaching device the demo Commit uses on
`<project>-412`. `GetConfluenceConfig`'s demo answer stays.

## 3. Templates

`tam/internal/ritualtemplate`, pure Go, no I/O.

```go
type SprintInfo struct {
    ID        int
    Name      string
    Goal      string
    StartDate string // Jira Agile datetime, parsed through internal/sprintdate
    EndDate   string
    BoardName string
}

func Render(ritualType string, s SprintInfo, loc *time.Location) string
```

Rendering is deterministic: no clock, dates formatted as local dates from the
sprint's own fields, text escaped for XML. The same inputs produce the same
bytes, which adoption depends on. A sprint with no dates renders the date line
as "Dates not set".

### Jira Issues macros

Only these three JQL forms are emitted, because they are the forms TAM can
preview offline (section 4):

| JQL | offline preview |
|---|---|
| `sprint = N ORDER BY Rank` | cached issues with `sprint_id = N`, rank order |
| `sprint = N AND statusCategory = Done` | those where `backend.IsDone(status)` |
| `sprint = N AND statusCategory != Done` | those where `!backend.IsDone(status)` |

Parameters: `jqlQuery`, `columns`
(`key,summary,type,status,assignee`, plus the story points field on Planning),
`maximumIssues` = `50`. No `serverId` until the probe says one is needed; if
it is, a per-profile setting `confluence_jira_server_id` is read by `Render`
through `SprintInfo`.

### Pages

Section headings are H2. "Task list" means `ac:task-list`.

- **Sprint overview** (`_sprint`): board name and dates; **Sprint goal**, the
  cached goal or an empty paragraph; a `children` macro listing the rituals.
- **Planning**: **Sprint goal** (prefilled); **Capacity**, a table
  `Member | Days available | Notes` with three empty rows; **Committed scope**,
  macro all issues; **Risks and dependencies**; **Decisions**; **Action
  items**, task list.
- **Standup**: **Blockers and work in flight**, macro not done; **Daily log**,
  one H3 per day, newest first, each holding **Yesterday**, **Today**,
  **Blockers** (a task list). The template seeds one entry dated the sprint's
  start date, or no entry when the sprint has no start date.
- **Review**: **Sprint goal** with a line `Met / Partly met / Not met`;
  **Completed**, macro done; **Not completed**, macro not done; **Demo
  notes**; **Stakeholder feedback**, a table `Who | Feedback | Follow up`;
  **Follow ups**, task list.
- **Retrospective**: **What went well**, **What did not**, **What we will
  try**, three bullet lists; **Carried over**, macro not done; **Action
  items**, task list, owner and due date written inline.

The standup entry fragment is exported separately
(`StandupEntry(date time.Time) string`) so the editor's "Add today's entry"
and the template share one definition. The frontend receives it through a
binding rather than a second copy in TypeScript.

## 4. The editor

### Dependencies

TAM frontend only: `@tiptap/react`, `@tiptap/pm`, `@tiptap/starter-kit`,
`@tiptap/extension-table` (with row, header, cell), `@tiptap/extension-task-list`,
`@tiptap/extension-task-item`, `@tiptap/extension-link`. Nothing enters
`@agile-suite/core` until XTM needs an editor.

### `src/lib/storage/`

Pure TypeScript, no React, tested alone.

`parse(body: string): JSONContent | ParseFailure`

1. Replace HTML named entities with numeric references (`&nbsp;` to
   `&#160;`, and the rest of the HTML5 table), since XML defines only five.
2. Wrap in `<root xmlns:ac="…" xmlns:ri="…">`, because storage format uses
   both prefixes without declaring them.
3. `DOMParser` with `application/xml`. A `parsererror` document returns
   `ParseFailure`.
4. Walk. Mapped elements: `p`, `h1`–`h6` (h4 and below kept as their level),
   `ul`, `ol`, `li`, `table`, `tbody`, `tr`, `th`, `td` (`colspan`, `rowspan`),
   `strong`, `b`, `em`, `i`, `u`, `s`, `code`, `br`, `a[href]`, `blockquote`,
   `ac:task-list`, `ac:task` with `ac:task-status` and `ac:task-body`.
   Attributes a mapped element carries that the schema does not model go into
   an `extra` attribute bag on the node and are written back.
5. **Every other element becomes an opaque node** holding its raw XML, as
   produced by `XMLSerializer` from the parsed node: `opaqueBlock` where it
   appears between blocks, `opaqueInline` inside text. That covers
   `ac:structured-macro`, `ac:link`, `ac:image`, `ac:emoticon`,
   `ac:placeholder`, `ac:layout`, `span` with styles, and anything no one
   anticipated. There is no list of known unknowns; the fallback is the rule.

`serialize(doc: JSONContent): string` is the inverse. Opaque nodes print their
stored XML unchanged. Output uses the prefixes without declarations, as
Confluence sends them.

**Contract:** `normalize(serialize(parse(x))) === normalize(x)` for every
corpus page, where `normalize` collapses whitespace between block elements,
sorts attributes, and unifies self-closing form. The corpus is the five
rendered templates plus hand-written pages carrying: a page layout, a code
macro with CDATA, a `ri:user` mention, a coloured span, merged table cells,
`&nbsp;` and `&mdash;`, a nested task list, an image, and an emoticon. The
probe's round-tripped page joins the corpus.

**A parse failure opens the page read only**, rendered through
`sanitizeHtml`, under the sentence *"This page has content TAM cannot edit
safely. Edit it in Confluence; Sync will bring the changes back."* with an
Open in Confluence link. Sync pulls and pushes it as usual; there is nothing
local to push unless it was edited before it became unparseable.

### Opaque nodes in the editor

An opaque block renders as a locked tile labelled with what it is: the macro
name for `ac:structured-macro` (*Confluence: panel*), the element name
otherwise. It can be selected, moved, and deleted whole; its content cannot
be edited. An opaque inline renders as a small chip with the same label.

A `jira` macro whose `jqlQuery` matches one of the three forms in section 3
renders a compact issue table from the cache through
`RitualMacroIssues(profileID, jql) (MacroPreview, error)`, which parses only
those forms and answers `Supported: false` for anything else. Under the table:
*"From TAM's cache. Done here means the status name; Confluence shows the live
result."* A `jira` macro with other JQL shows its query and *"Rendered in
Confluence"*.

### Saving

- Saved locally, 800 ms after the last user edit, and immediately on Ctrl+S,
  on switching document or sprint, and on leaving the view.
- Only user transactions count. Loading content, `setContent`, and schema
  normalisation on load never trigger a save, so opening a page never makes it
  dirty.
- `SaveRitualBody(profileID, boardID, sprintID, ritualType, body)` takes no
  lock and makes no network call. It writes `body`, `updated_at`, and status:
  `local` while the row has no page id, otherwise `unsynced` when the body
  differs from `base_body` and `synced` when an edit was undone back to it.
  A row in `conflict` or `gone` keeps that status.
- The status line: *Saving…*, *Saved locally 14:03 · not synced*,
  *Synced 13:50*, *Conflict with Confluence*.

### The conflict banner

Above the editor when status is `conflict`:
*"Confluence has a newer version (v8). Your local edits are kept until you
choose."* Actions: **View theirs** (toggles a read-only render of
`conflict_body` in place of the editor), **Keep mine**, **Take theirs**. Take
theirs confirms through `useConfirm` first, because it discards local text.

When status is `gone`: *"This page was deleted or moved in Confluence."*
Actions: **Recreate on next Sync** (`ForgetRitualPage`) and **Remove local
copy** (confirmed).

Both resolutions and both gone actions call `announce()`.

### The view

`RitualsView` is rebuilt:

- **Toolbar**: Board picker (scrum boards), Sprint picker, **Sync rituals**,
  and a line *Last synced 14:02 · 3 unsynced · 1 conflict*.
- **Left**, a document list: Overview, Planning, Standup, Review,
  Retrospective, each with a status chip (Local, Synced, Unsynced, Conflict,
  Gone), styled with the shared `folder-item` classes.
- **Right**: the title, an **Open in Confluence** link when the row has a page
  id, the formatting toolbar (bold, italic, H2, H3, bullet list, numbered list,
  task list, table, link, undo, redo, and **Add today's entry** on Standup
  only), then the editor, which is the only scroller. Layout follows the
  `min-height: 0` chain in "Layout and scrolling".
- **Add today's entry** inserts the `StandupEntry` fragment for today's local
  date as the first child under the Daily log heading, and refuses with an
  announced sentence when an entry for today already exists or the heading is
  gone.
- **Confluence not configured**: editing works; Sync is disabled with
  *"Add Confluence in Profile settings to sync."*
- **No scrum board**: the empty state the Sprints view uses.
- Toolbar buttons carry `aria-label`s; the list is reachable by keyboard; the
  Sync result is announced.

## 5. Lock, bindings, removals

### Lock

`SyncRituals` holds `a.acquire(p.ID, "rituals")`, so sync, commit, boards
refresh, sprint operations and reports all refuse against it and it against
them. `SyncContext` gains `runRitualsSync`, which dispatches `SYNC_START` and
`SYNC_END` the way `runBoardsRefresh` does, and `running` gains `"rituals"`
so a refusal names it. It is not a quiet write: it is several requests and has
no modal holding focus, which is the case "One lock, both ends" was written
about. That section of `tam/CLAUDE.md` gets a paragraph saying so.

`EnsureSprintRituals`, `SaveRitualBody`, `ResolveRitualConflict`,
`ForgetRitualPage` and `DeleteRitualDocument` are local writes and take no
lock, for the reason board moves take none; the CAS in the pass is what makes
that safe.

### Bindings

Added, in `app_rituals.go`: `EnsureSprintRituals`, `ListRitualDocuments`,
`SaveRitualBody`, `ResolveRitualConflict`, `ForgetRitualPage`,
`DeleteRitualDocument`, `RitualMacroIssues`, `StandupEntry`, `SyncRituals`,
`LastRitualSync`.

Removed: `GetRitualPage`, `GetConfluencePage`, `ListConfluenceChildPages`,
`ListRitualAssociations`, `SetRitualAssociation`, `DeleteRitualAssociation`,
`ListRitualDrafts`, `GetRitualDraft`, `SaveRitualDraft`, `DeleteRitualDraft`,
`ScaffoldSprintRituals`, `ListSprintIssues`.

`GetConfluenceConfig` and `SetConfluenceConfig` stay.

### `core/confluence`

```go
type Pages interface {
    GetPage(ctx, id) (Page, error)                        // existing, root check
    GetPageStorage(ctx, id) (StoredPage, error)           // storage body + version
    FindPageByTitle(ctx, space, title) (Found, bool, error) // with ancestor ids
    CreatePage(ctx, space, parentID, title, body) (StoredPage, error)
    UpdatePage(ctx, id, title, body, version) (StoredPage, error)
}
```

`FindPageByTitle` calls
`/rest/api/content?spaceKey=…&title=…&expand=ancestors,version`.
`UpdatePage` sends `version.number = version` as given; the caller passes
base plus one. A 409 maps to `ErrVersionConflict`, a 404 to `ErrNotFound`,
both matchable with `errors.Is` alongside the existing `HTTPError`. The
package comment stops calling it read only. `ListChildPages` and `DemoPages`
are removed.

### Removed code

- Frontend: `RitualWizard.tsx` and its test, `RitualTemplate`, the association
  list and picker in `ProfileForm.tsx` (its Confluence connection fields
  stay), `parseRitualIssues` and `encodeRitualIssues` in `api.ts`.
- Go: `internal/ritualdefaults`, `ritualrepo.SeedDemo` and its two call sites
  in `app.go` and `app_profiles.go`, `ritualrepo.DecodeIssues` and
  `EncodeIssues` once the ensure step's carry-forward reads `issues_json`
  through a private helper, `profile.Manager`'s `CacheConfluencePage`,
  `CachedConfluencePage` and the three association methods.
- Kept: the `confluence_association` and `confluence_page_cache` tables in
  `profiles.db`, unwritten, still deleted with the profile.

## 6. Plan step 1: the probe

Before any write code, run a probe against the real instance and record the
answers in `docs/superpowers/plans/assets/2026-09-14-confluence-write-probe.md`.
Phase 4's probe was written and never run; this one gates the plan.

1. Create a page under the root carrying a Jira Issues macro with no
   `serverId`. Does it render in Confluence?
2. The same page carries an `ac:task-list`. Open it in the web editor, tick a
   task, type above and below, save. Read the storage body back: are the task
   list and the macro intact, and does `lib/storage` round-trip it?
3. Update with a stale version number. Is the answer 409, and what body?
4. Create a second page with the same title in the space. 400 or 409, and what
   message?

If answer 1 is no, `confluence_jira_server_id` becomes part of the plan. If
answer 2 shows the web editor rewriting task lists into a shape `lib/storage`
cannot map, that shape joins the schema or stays opaque, decided then.

## 7. Verification

**Go**

- `ritualtemplate`: each type renders identically twice; output is well
  formed XML once wrapped; the three JQL forms appear exactly; a sprint with no
  dates renders.
- `ritualsync`, against the fake, one test per row of the tables in section 2:
  create the tree in order (a ritual never created before its sprint page);
  adopt under the root; refuse a foreign title and skip that sprint's rituals;
  adopt over an untouched template pulls; adopt over edits conflicts; clean
  pull; push at base plus one; remote newer and dirty conflicts; 404 goes
  gone; 409 on push becomes a conflict; a save between read and push leaves the
  row dirty and does not overwrite it; a pull does not overwrite a body saved
  mid-pass; one failing page leaves the others synced; closed sprints get no
  new pages.
- Client: `httptest` for the four new calls, the auth header, the version
  number sent, 409 and 404 mapping.
- Repository: version 11 migration; the ensure step writes five rows, leaves
  existing bodies alone, upgrades an empty-body row, and carries remarks
  forward; both resolutions; `PurgeProfile` still clears the table.
- App: `SyncRituals` refused while another operation holds the lock; refused
  unconfigured; `ListRitualDocuments` makes no network call.

**Frontend**

- `lib/storage`: the corpus round-trips; opaque XML is preserved byte for
  byte; entities survive; a malformed body returns `ParseFailure`.
- Editor: opening a document does not call `SaveRitualBody`; typing calls it
  once after the debounce; switching documents flushes; Add today's entry
  inserts at the top and refuses a second time the same day; a jira macro with
  a supported form previews from `RitualMacroIssues` with its caveat.
- `RitualsView`: status chips for all five statuses; the conflict banner's
  three actions; the gone banner's two; Sync disabled when unconfigured; a
  parse failure renders read only.
- `SyncContext`: `runRitualsSync` sets `running` to `"rituals"` and a refused
  sync names it.
- The `vi.mock("./api")` allowlists in `App.test.tsx` and the view tests list
  every added binding and drop every removed one.

The full gate runs once at the end, plus `wails build`. Test counts are counts
to beat: the number removed with the wizard and `ritualdefaults` is reported,
not hidden.

## Out of scope

Automatic merging of a conflict. Deleting Confluence pages from TAM.
Comments, attachments, search. Inserting sprint report figures. A title
prefix setting. Per-day standup pages. New pages for closed sprints. Anything
in XTM.
