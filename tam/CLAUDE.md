# CLAUDE.md

Task Activity Manager (TAM) is the agile task-management app of the suite:
Jira DC tasks, epics, stories, bugs, and requirements for scrum masters,
product owners, and team members. It shares connection profiles and the
Windows Credential Manager entries with Xray Test Manager through
`core/profile` and the shared `profiles.db`. The design lives in
`docs/superpowers/specs/2026-09-04-tam-foundation-design.md`.

Outline mirrors this file for readers who are not in the repository, under
**Tools → Task Activity Manager (TAM)**: a User Guide (end users), Feature
List, Supported Views, Architecture, Code Structure, Developer User Guide,
and Change Log, alongside XTM's own pages in the same collection. Keep the
Outline pages in step when a change here alters what the app does; the User
Guide is the one written for people who will never read this file.

## Status

Plan 1a (issues, read path): sync by project into `tam.db`, the Backlog
grid, and a read-only detail panel, on the demo dataset or a live Jira DC.
Plan 1b adds the journal, create and edit, and Commit. Plan 1c adds Excel
import, cross-project links, and requirement creation. Phase 2 adds the
epic and story hierarchy: the Epics view, `parentKey` as the seventh
editable field, and epic creation. Phase 3a adds the Boards view: boards,
columns, and sprints synced from Jira's Agile API, read only. Phase 3b
(this branch) makes the board writable: a card dragged or keyboard-moved
across columns, within a column, or into another sprint journals the same
way every other TAM write does, and Commit pushes it.

## Phase 3a: boards

`core/jira/agile.go` is the Agile 1.0 transport: `Boards`, `BoardConfiguration`,
`Sprints`, and `BoardIssueKeys`, each paged to exhaustion and returning
Jira's raw shape. A Data Center with no Jira Software answers `Boards`
with 404, mapped to `ErrNoAgile`; a kanban board's sprint call answers 400,
mapped to `ErrNoSprints`. TAM's `internal/backend.BoardBackend` is the
read-only capability those four calls back, kept off `IssueBackend` so
only a backend that can speak Jira's Agile API has to answer for it; the
demo backend implements it too, with one scrum board (three sprints, one
closed) and one kanban board.

Schema version 5 adds four tables to `tam.db`, all keyed by profile:
`board`, `board_column`, `sprint`, and `board_issue` (a board's membership
of one scope, `sprint_id` empty for the board's own list and a sprint id
otherwise). It also adds a `status_id` column to `issue`, which is how a
card is matched to a column. Since `CREATE TABLE IF NOT EXISTS` cannot add
a column to a table that already exists, this is the plan's first store
migration (`tamstore.Schema.Migrations`): it adds the column, then clears
every profile's sync watermark. **An incremental sync only re-reads issues
Jira reports changed since the watermark, so a row cached before version 5
would never get a status id filled in on its own; clearing the watermark
is what makes the next sync for each profile re-read every issue and fill
the column in, without purging anything first.**

Schema version 6 re-keys `sprint` from `(profile_id, id)` to
`(profile_id, board_id, id)`. Jira Data Center hands the same sprint to
every board whose filter reaches it, and the sync clears and writes
sprints one board at a time, so two scrum boards over one project used to
collide on the second board's insert and take the whole pass down. SQLite
cannot change a primary key in place, so the migration drops the table and
recreates it: it is a cache the next sync refills, and nothing joins to
its rows.

`internal/boardrepo` is the store layer over the four tables, beside
`issuerepo` since boards are their own concern. It never imports
`issuerepo`: what it needs from the issue cache is the two-method
`IssueSource` interface (`IssuesByKeys`, `DraftIssues`), which `app.go`
satisfies with the issue repository it already holds. `boardrepo.Board`
composes the view a board draws: columns in board order, cards bucketed
into them by status id (a draft goes to the first column that collects
any status, since Jira has never assigned it one), and the lanes the
chosen swimlane (none, assignee, epic) asks for. A card whose status is
in no column is counted into `Unmapped`, and its status name into
`UnmappedStatuses`, not listed card by card. A cell caps at 200 cards and
the whole view at 2,000; past either cap a card is only counted, in
`Overflow` and `Capped`. `DonePoints` is summed in the same walk, over
every mapped card rather than the ones a capped cell drew, by
`backend.IsDone`: the definition lives in `internal/backend` because both
`boardrepo` and `issuerepo` count by it and `boardrepo` may not import
`issuerepo`. `NeedsStatusSync` is true when every cached card
still carries an empty status id, which is the state right after the
version 5 migration and before the next sync; the view tells the user to
sync rather than drawing an empty board and blaming them for it.

`internal/syncer/boards.go` is `Engine.SyncBoards`, reached through a type
assertion on `backend.BoardBackend` so a backend without it is skipped,
not failed. It runs after the issues pass in a regular sync, and alone
from the Boards view's Refresh. For each board it reads columns, sprints,
its own issue list, and the issue keys of its active and future sprints
only, then writes all of it for that board in one transaction with
`boardrepo.ReplaceBoard`; a board whose read fails at any point is
recorded in the summary's `Dropped` with a one-line reason from
`internal/errtext` (added during review: it strips HTML tags and
collapses whitespace, since a 403 answered with an HTML login page hands
the transport a kilobyte of markup) and is left exactly as it was. The
four read methods are `ListBoards`, `ListBoardSprints`, `GetBoard`, and
`SyncBoards`, all in `app_boards.go`. Phase 3a wrote nothing to Jira;
Phase 3b, directly below, adds the writes.

Two facts worth knowing before they cost you a debugging session:

- **An incremental sync cannot backfill `status_id`.** That is the whole
  reason version 5's migration clears the sync watermark rather than just
  adding the column: without the reset, every issue cached before this
  branch would carry an empty status id forever, since nothing would ever
  ask Jira for it again.
- **A board's membership is whatever Jira's board endpoint returned at
  sync time.** TAM does not compute board membership from status; it
  caches the key list `/board/{id}/issue` and `/board/{id}/sprint/{id}/issue`
  answered with. A card moved on the web board moves in TAM only after
  the next sync, boards sync included.
- **Only the active and future sprints have their cards fetched.** A
  closed sprint stays in the sprint list, for history, but its membership
  is never pulled, so the sprint picker offers only active and future
  sprints and no others.

## Phase 3b: board writes

A card dropped or keyboard-moved on the board makes one of three moves,
each its own journal entity beside `issue`, `issue_create`, and `link`:
`issue_transition` (field `statusId`) for a drag across columns,
`issue_sprint` (field `sprintId`) for a move to another sprint or to the
backlog, and `issue_rank` (field `rank`) for a reorder within a column.
The three writes, the packing, and the reverts are `internal/issuerepo`'s
`boardwrites.go`, `movevalue.go`, and `movecolumns.go`; `rebasemoves.go`
is what Override does to a held one. Nothing here talks to Jira directly;
the journal is what Commit pushes, exactly as it does for a field edit.

A transition is journaled by the target status id, never by a transition
id: which transitions Jira offers depends on the issue's status at that
exact moment, so an id read at drag time would be stale before Commit
ever runs. `internal/backend/jira/transitions.go`'s `Transition` is where
the target status id becomes a real transition at push time: it lists the
issue's transitions, picks the one whose `to.id` matches the journaled
target (the lowest transition id when two reach it, so the same drop
resolves the same way on every run), and fills whatever that transition's
own screen requires. Almost every Data Center workflow's way into Done
puts a resolution on that screen; TAM reads it from the transition's own
`fields.resolution.allowedValues` and fills it from the profile's
`transition_resolution` setting when the transition allows that value,
from the transition's first allowed value otherwise, and refuses (naming
every required field by name) when the screen asks for anything else.

A rank is never journaled as a LexoRank: Jira owns that value, and a
client-made one would be a second, wrong source of truth the next sync
would silently overwrite. It is journaled as a neighbour, a side, and the
board the drop was made on (`before|KEY|BOARD` or `after|KEY|BOARD`), and
the commit pass re-derives the neighbour from that board's final local
order at push time (`boardrepo.CellOrder`) rather than trusting the key
that was journaled at drop time, which a busy board can easily have moved
on from. The board rides along because one key can sit on two boards
whose orders disagree, and nothing else says which board's order a rank
was measured against. Every card but the one at the very top of the
board's order anchors with `rankAfterIssue` against the card that landed
above it; the top card has nothing above it, so it anchors with
`rankBeforeIssue` against the card below it instead. "X before Y" and "X
after W" place X in exactly the same spot in Jira's one global rank, so
two cards ranked against each other can never disagree about the pair,
and only the first card of the board's first non-empty column can ever
take the `before` branch.

A board write is classified against the issue's remote status or sprint
id at Commit, never against its `updated` stamp: a comment left on the
issue in Jira bumps `updated` without moving the card, and reading that
as a conflict would hold back a card nobody touched. Remote already at
the journaled target is satisfaction, not a conflict, and the row is
dropped as a `Moved{Satisfied: true}` rather than raised as one; remote
at the journaled before value is pushed; anything else holds the whole
issue back, both board rows together if both are pending, so half an
intent is never committed against a card that is not where the user left
it. `internal/committer/boards.go` (the sprint and transition passes),
`ranks.go` (the rank pass, last, since it is the one write that can be
redone harmlessly), and `boardvalues.go` (the three-way classification
and the labels a conflict card prints) are the board pass, which runs
after the edits and before the links, because a transition on a draft has
to wait for the create that gives the draft a real key.

`Commit`'s own loop and `regroupEdits` both name the three board entity
types through `boardRow`; missing either spot sorts a board row into
`commitEdit`, which sends `statusId` to Jira as an ordinary field, fails
on it, and fails the issue's real edits along with it. Overriding a held
board row is not the same operation as overriding a held edit: an edit is
held on the issue's `updated` stamp, which `ResolveOverride` rebases by
writing a new `base_version`, but a board row is held on its `before_val`
against the remote status or sprint id, which has no base version to
rewrite. `RebaseMoves` is the other half: it rewrites the held row's
`before_val` to what Jira holds now and leaves `after_val`, the user's
move, untouched. Skip it and Override on a board conflict would meet the
identical conflict on every following Commit; a key with only edits
pending still overrides with no network call.

A journal row for a board move is deleted only while its `after_val` is
still the value that was pushed (`MarkMoveCommitted`), because none of
the three move bindings take TAM's busy guard the way editing a field
does: a card dragged again while Commit is mid-push updates that row in
place, and a delete by row id would throw away an intent Jira was never
told about. The commit is still audited, since the push did happen; the
newer intent stays in the journal for the next Commit to find.

Jira's Agile bulk endpoints (`PUT /issue/rank`, `POST /sprint/{id}/issue`,
`POST /backlog/issue`) answer 207 Multi-Status when at least one issue in
the request was rejected and 204 when every one landed, so
`core/jira/bulkwrite.go` treats a 207 as a failure by its HTTP status
alone, never by trying to recognise a body schema: there is no successful
Multi-Status to accommodate. The body is decoded only to say why, trying
Atlassian's documented `entries` array first, then the shapes
`jiraErrorMessage` already knows, then a raw excerpt as a last resort.

The keyboard path is not a convenience beside the drag; it is the
accessible path a screen reader user and a trackpad-averse user actually
get the feature through. Ctrl with an arrow moves the card that already
holds focus (left and right transition a column, up and down rank within
the cell), refusing with an announced sentence at either edge, and every
card's "Move to" menu reaches a sprint move without a pointer at all. A
drag draws two different cues for the same reason the two moves are
different writes: a drop line at the cursor inside the card's own cell,
exactly where a rank will land, and a full-cell outline for a drop on
another column, never a line there, since a transition lands the card by
its own rank rather than at the cursor.

## The write path (plan 1b)

Edits and creates go through the journal in `tam.db` (`pending_change` and
`audit_log`, shared DDL and helpers in `core/journal`). `issuerepo.EditField`
writes the row and journals the change with the row's `updated` as the base
version; `CreateDraft` inserts a `TAM-NEW-n` row with status `Draft` and a
create row holding the draft as JSON. Sync never deletes a draft and never
overwrites a column with a pending edit. `internal/committer` pushes the
journal: drafts first (POST, then rekey), then per-issue version checks and
PUTs; an issue whose remote `updated` moved is held back with base, mine,
and remote per field, and the user picks Override (rebase, push next time)
or Keep remote (drop the edits, take Jira's row). Commit and sync exclude
each other through `App.busy` and the shared reducer's `committing` state.

The demo backend keeps writes in memory, hands out keys from 500, and
stages one conflict: the first Commit of an edit to the curated story
(`<project>-412`) is held back. Editable fields are summary, description,
priority, labels, story points, and assignee; drafts can be tasks, stories,
bugs, and requirements. Excel import and cross-project links are plan 1c.

## The write features (plan 1c)

Import: the Backlog's Import button takes a CSV or XLSX (parsed by
`core/importfile`, XTM's parser lifted out), maps columns to the eight draft
fields (`internal/importer`), validates rows with file row numbers, and
creates the valid rows as drafts in one transaction (`CreateDrafts`, audited
"imported from <file>").

A ninth column, Key, decides what a row does. Empty, the row creates a
draft. Filled with an issue key or a `.../browse/KEY` URL, it journals edits
to that cached issue instead (`EditFields`, one transaction, all or
nothing), so a sheet exported from Jira round-trips rather than duplicating
every row. On such a row the Type cell is ignored (an issue's type is not
editable) and an empty cell means "leave this field alone", never "clear
it"; a value that already matches is not journaled, so re-importing an
unchanged file leaves nothing pending. Summary is required only when no Key
column is mapped. `SaveImportTemplate` writes a real workbook
(`internal/importer/template.go`, excelize): an Issues sheet with the nine
columns, a Type dropdown carrying the profile's own requirement type name,
five examples, and a "How to use" sheet; naming the file `.csv` in the save
dialog writes the same columns as CSV. Assignee is the Jira username, not
the display name.

Links: the Links tab's Add link form journals a
link (entity type `link`, field `<type>|<direction>|<target>`); the
repository merges pending links into the cached detail; the committer pushes
link rows after edits with `POST /rest/api/2/issueLink` and drops the
source's detail cache. Requirements are creatable; the demo asks for a
Source field on them and answers lookups for the `XT-` keys its curated
details reference. Link removal, links in the bulk sync, epics, and
subtask parents are not in scope.

## Phase 2: epics

The Epics view groups the cache by parent: `issuerepo.EpicTree` reads a
profile's epics and their children from `tam.db` in one call, in rank
order, with per-epic progress counts (done count, points, done points),
truncating past a 5,000-row cap. `parentKey` (label "Epic") is the seventh
editable field, riding the same edit, journal, conflict, and commit
machinery as the other six; the Jira backend maps it to the discovered
Epic Link field. Creating an epic defaults its Epic Name to the summary
when the draft leaves the field blank, so the user never has to know Epic
Name exists. The two bound methods are `GetEpicTree` and `ListEpics`, both
in `app_writes.go`. The tree's own styling is XTM's folder tree, reused
class for class (`folder-tree`, `folder-item`, `folder-caret`, and the
rest) out of `frontend/core/styles/primitives.css` rather than a second
tree style.

An incremental sync does not remove an epic that was deleted in Jira, so a
stale epic keeps showing its children in the tree until a full sync clears
it.

A CSV import's epic rows must come before the rows of any child that
names them: a child's row is checked against the epics the file has
defined so far, in the order the rows appear, not against the whole file
or the cache.

## Navigation and the menu bar

The view tabs under the topbar are the visible navigation, mirrored from
XTM's (`references/xtm-main-layout.png`). The View menu and the optional rail
reach the same places.

No view renders a title bar, the way none of XTM's do: the active tab already
names the view and the topbar's profile select already names the project, so a
heading repeating both only cost the content height. Each view names its own
landmark (`aria-label` on its section) rather than borrowing an id from a
heading that no longer exists, so the views are still distinguishable to a
screen reader and to a test.

The native menu bar carries the same list, the way XTM's View menu does:
`menuViews` in `main.go` lists the views and each item emits `menu:view` with
the view id, which `App.tsx` routes on. That list has to stay in step with
`VIEWS` in `frontend/src/nav.ts` by hand; a native menu cannot read the
frontend's.

The left nav rail is a second, optional way to reach the same places, off by
default and toggled from View → Navigation Rail (Ctrl+B) or from its own close
button. The preference is `show_nav_rail` in the shared settings, whose zero
value is the hidden default. Wails renders a checkbox's tick from the value the
item was built with, so `App.refreshMenu` rebuilds the whole menu rather than
mutating it, and `startup` rebuilds once more because `main()` builds the first
menu before the store exists.

The window icon and the executable's icon come from `build/windows/icon.ico`,
not from `build/appicon.png`: Wails only generates the `.ico` when it is
missing. After changing `appicon.png`, regenerate the `.ico` over the same six
sizes Wails uses (256, 128, 64, 48, 32, 16) and copy the PNG to
`frontend/src/assets/images/appicon.png`, which is what the About dialog shows.

## Issue types and sub-tasks

The six logical types are task, epic, story, bug, requirement, and sub-task.
Epic, story, and bug map to fixed Jira names; the requirement's is the
per-profile setting `requirement_issue_type`; **the task and sub-task levels
are discovered from the project**, because the instance names them.
`jira.Backend.resolveTypes` reads the project's issue types once, caches them,
and takes the first with `subtask: true` as the sub-task level and the first
matching `taskAliases` ("task", "todo", "to do") as the task level. Jira's
defaults are "Sub-task" and "Task"; an instance in the field calls them
"Technical task" and "Todo", and asking that instance for "Task" found
nothing. A level the project does not define is dropped from the sync scope
rather than quoted into it: Jira rejects an entire query that names an
issuetype the instance lacks, so asking would fail the sync instead of
returning nothing.

The sub-task chip shows the instance's own word, passed down as
`TypeChip`'s `subtaskLabel` by whoever holds the profile rather than looked up
in the chip, so the chip stays a pure render. A reader who sees "Technical
task" in Jira should not see "Sub" here.

`parentKey` means two different things by level. For a sub-task it is Jira's
own `parent` field and an ordinary issue, never an epic or another sub-task,
and it cannot be blank. For everything else it is the Epic Link and must be an
epic. `validateParent` and the Jira create both branch on that.

A sub-task is not in `CREATABLE`: it cannot exist without a parent, so it is
only drafted from the issue it belongs to, through the detail panel's
"+ <sub-task type>" button, which opens the create dialog with the type locked
and the parent stated rather than chosen.

## The detail sidebar

The panel is XTM's (`references/xtm-detail-sidebar.png`): a dark instrument
bar carrying the key and its status chip over a padded scrolling body, then
the read-only facts in an 84px label grid, then collapsible sections under
uppercase headings. It used to hide Links, Tests, and Activity behind tabs;
XTM stacks them so a reader scrolls one column instead of hunting three.
Fields is the only section open on mount, so the panel still starts short.

## The create dialog

A draft's parent comes from context. `parentKey` is fixed and stated (a
sub-task's parent, from the issue it was drafted from); `initialEpic` is a
default the picker may change, seeded from the epic on screen — the selected
row when it is an epic, else the epic it hangs off. Starting at "(none)" made
every draft begun with an epic open an orphan that had to be reparented
afterwards. A fixed parent wins over a seeded epic.


`NewIssueModal` drafts one issue and says so: it is titled after the type it
is about to create, and its subtitle carries the offline-first sentence at
full strength, in a slot no error message can take (it used to share the
footer with the error, so the explanation vanished exactly when a confused
user needed it). Success calls `announce()` naming the `TAM-NEW-n` key, since
the created row may be hidden by the active filters.

Submit waits for `GetCreateFields`. Drafting before the required-field list
lands skipped every one of them and deferred the failure to a Jira 400 at
Commit; a *failed* read is different and still lets the user draft, which is
the intended degrade. The list is cached for ten minutes, so a type toggle no
longer re-flickers it.

`parentKey` is the seventh draft field here as well as in the detail panel,
so a story can be born under its epic rather than reparented afterwards. An
epic never carries one, and switching the type to epic drops it.

The Epics view opens the dialog with `lockType`, which fixes the draft to
`initialType` and drops the type select: "+ New epic" is a statement, not an
opening question, and the dialog's own title states the type. The Backlog's
"+ New" leaves the select in place, so everything else is drafted there.

A create-meta value's JSON shape comes from its own create-meta, in
`shapeExtra`: an option id when Jira listed allowed values, the typed text as
`{"value": …}` when it did not, and a comma list split into the array Jira
wants. That is why an array field renders as a multi-select: a Jira array
takes more than one value, and the form joins the chosen ids with a comma.

## The grids' columns

Every track in both grids is a fixed width, including SUMMARY. A `1fr` summary
made the whole table reflow the moment a row was selected, because the detail
panel took 352px out of the pane: the one time the reader is looking closely
at it is the one time it moved. The rows are `width: max-content;
min-width: 100%`, so they size to their tracks, still fill a wider pane, and
leave the slack at the right instead of redistributing it. When the pane is
narrower than the tracks, the scroller takes over; in the Backlog that is
`.issue-body`, which holds the header too, so a sideways scroll carries the
column labels with the rows.

The table and the detail panel meet on a single 1px border with no gutter and
no rounded seam, the way XTM's do, and the panel is dragged to width from a
grip on its left edge (bounded 300-900px, remembered in the WebView's own
localStorage since it is a per-machine reading preference, not a shared
setting).

The detail panel's title is the one issue key that links out to the instance
(`IssueKeyLink`, opened in the user's own browser). The grids' key cells are
plain text on purpose: a row selects on click, so a link inside one would put
two actions on the same pixels.

## People and priorities

The assignee field is a picker, not a text box, because the two halves of a
user never matched: sync writes the *display name* into the issue row, and the
write path sends `{"assignee": {"name": …}}`, so free text could only produce
a value Jira accepts when the two happened to be identical. `AssigneePicker`
stores the username and shows the display name.

`App.SearchUsers` asks Jira's *assignable* endpoint (the plain user search
answers with people who hold no permission on the project, and picking one of
those fails at Commit), caches the answer in `jira_user` in `tam.db`, and
falls back to that cache when Jira cannot be reached. A blank query goes out
as the wildcard Jira DC's assignable search wants, and is what seeds the
cache. `CacheUsers` is additive: a narrow search must not empty the picker for
the next one, so rows go when the profile is purged, not when a search misses
them.

Priority is the instance's own list through `App.ListPriorities`. Both
pickers degrade to the text input they replaced when their lookup fails, the
same shape the create dialog uses for a failed create-meta read: a lookup that
cannot reach its list must not be the reason an issue cannot be assigned.

## One lock, both ends

Go holds a single per-profile lock (`App.acquire`) for a sync, a commit, an
import, and a boards refresh alike, so whichever starts second is refused.
The frontend models the same invariant in the shared sync reducer, and the two
have to agree: the Boards view's Refresh used to be a plain mutation outside
the reducer, so the shell stayed `idle`, kept offering Sync, and Go refused it
with "a sync is already running for this profile" on a profile whose status
still read "not synced yet". It runs through `SyncContext.runBoardsRefresh`
now, which takes the same lock the sync does. **Anything new that calls a
bound method taking `acquire` has to go through the reducer too.**

`SyncBoards` acquires under its own name, so a refusal says which operation is
actually running. The boards pass is also given the progress sink now: it
walks every board's columns, sprints, and each sprint's issue keys, which
takes minutes on a real project, and a standalone Refresh used to pass `nil`
and report nothing anywhere.

`SyncIssues` and `SyncBoards` log on the way in as well as out. A run that
never returns used to leave no trace at all, so the log could not say whether
a call had even reached Go.

## The boards pass and the board's shape

The pass is one request per board's column config, per page of its sprint
list, and per page of the issue keys of every scope it holds: the board's own
list plus each active or future sprint. Two things keep that from dominating
the sync, and both were measured against a real instance where one board took
66 seconds on its own:

- **The Agile page size** (`pageAgileIssues`, 500). At 50 a page a board's own
  issue list, which is every issue on the board, cost one round trip per 50
  keys. Both paging loops advance by what actually came back, so an instance
  that clamps `maxResults` lower is handled by the same arithmetic.
- **The project narrowing.** A board's filter is not bounded by a project: the
  board above held 8,485 cards while the project being synced had 38. Every
  scope is read with `jql=project = "KEY"`, which the Agile endpoints AND with
  the board's own filter. Only that project's issues are ever in the cache, so
  a key outside it could not be drawn anyway.

A board that still takes over five seconds names itself in the log with its
sprint and card counts.

The third saving is **whose boards get synced at all**. Jira's board list
answers with every board whose *filter* mentions the project, which includes
boards another team owns: the 8,485-card board above belongs to a different
project entirely. `ownBoards` keeps only the boards whose own
`location.projectKey` is this project, counts the rest in the summary's
`Foreign`, and names them in the log. A board whose home the instance did not
report is kept, so an instance that sends no location does not lose every
board. The per-profile setting `boards_all_projects` keeps them all, for a
profile that genuinely works across a programme board.

The columns are Jira's own: `BoardColumns` reads the board's configuration and
keeps each column's status ids, and `placeCard` puts a card in the column
whose ids contain the card's `status_id` (schema 5 added that column; matching
on the status *name* would break on any instance that renames one). A status
no column collects is counted as `Unmapped` rather than drawn, which is what
the board's unmapped line reports. Re-syncing a board therefore picks up a
column added or renamed in Jira with no further work.

## A drop asks for a column, not a status

A Jira board column collects several statuses: a Done column commonly holds
Resolved and Closed, and an instance that has migrated a workflow holds the
old status beside the new one. Only one of them is usually reachable from
where a card is now.

So the push is given the whole column, not the one id the drop journaled.
`BoardOrder.ColumnStatuses` returns every status sharing the dropped-on
status's column, that one first, and `Transition` walks them in order and
fires the first the issue's workflow offers. Keeping the journaled status
first means a reachable target is still preferred over its siblings, and a
refusal still names the status the user actually dropped on.

The journal format is untouched: it still holds one `id|Name`, and the
resolution happens at commit time, which is the only moment the workflow is
knowable. `App.CanTransition` builds the same candidate set, so the drop's
optimistic check and the push agree about what the drop meant.

## The board grid

The header row anchors as a **row** (`.board-columns-head`, sticky with its
own background), not as individual sticky cells: sticking the cells alone left
the 12px gaps between them transparent and cards scrolled through the header.

The toolbar's filter is a reading aid over the drawn board, not a query
(`lib/boardFilter.ts`): it matches a card's key, assignee, or issue type, and
never refetches, so clearing it costs nothing. It keeps every column and lane
so a filter never reads as a lost column, recomputes the column counts from
what survives, and zeroes overflow rather than guessing at cards a cap left
out. The board's own totals (unmapped, notSynced, donePoints) describe the
board and are left alone. `BoardBody` takes the filtered view beside the query
result: the query still says whether the board loaded, failed, or is empty.


The column header row and each lane's row are separate elements, so they only
line up because both are laid out on **one grid template**:
`repeat(var(--board-cols), minmax(--board-col-w, 1fr))`, with the column count
set on `.board-scroll` from the view. A flex row cannot do this, since each
row sizes its own items and a lane holding a long card grew wider than the
header above it. The cells carry `min-width: 0` so a wide card cannot push its
track open. `minmax` is also what makes the columns share the pane's width and
stop at a floor, past which `.board-scroll` scrolls sideways.

## Layout and scrolling

The window never scrolls as a whole. `.app` is one flex chain down to the
panes, and only three boxes scroll: the Epics tree, the Backlog's table body,
and the detail panel, each inside its own bounded card. `.main` is
`overflow: hidden`, so every level between it and a scroller carries
`min-height: 0` (a flex child defaults to `min-height: auto` and will not
shrink below its content, so one missing declaration lets the table push the
pager off the bottom of the window). Nothing sizes itself off
`calc(100vh - <a guess>)` any more; that was only ever right at one window
height. `references/xtm-main-layout.png` is the reference for this
arrangement, and the nav rail stays TAM's own in place of XTM's view tabs.

Anything rendered directly in `.main` has to say how it behaves in a bounded
box, which is why the placeholder and the startup error carry their own
`min-height: 0; overflow: auto`.

The Backlog's pager is pinned to the floor of the table card: rows per page,
first / previous / a typed page number / next / last, and the range on the
right. The page box holds a draft string separate from the page itself, with
`null` meaning "not editing", because binding it straight to `page + 1` made
clearing it snap back to "1" and typing "23" over it produce "123".

## The tables

Both grids size their key column from the keys on screen rather than from a
fixed pixel track (`frontend/src/lib/keyColumn.ts`, handed to the table as
`--issue-key-w` / `--epic-key-w`). A fixed track held about eleven characters
and Jira DC allows a ten-character project key, so a legal key overflowed: in
the Backlog it wrapped inside a 34px row and the type chip painted over it, in
the Epics tree it ran straight over the summary. One width per table, not per
row: each row is its own grid container, so a `max-content` track would size
every row to its own key and the columns would stop lining up. Every cell in
both grids now clips with an ellipsis and carries a `title`, so a shortened
value never reads as a complete one.

The Backlog is sortable. `IssueQuery` carries `sort` and `desc`, and
`issuerepo.sortColumns` maps a sort key to its SQL; a name outside that
whitelist falls back to rank order, so nothing from the frontend reaches the
query. Sorting is server-side because the grid is paged: the frontend holds 25
rows out of a project's thousands. A header click goes ascending, descending,
then back to rank; drafts stay pinned to the top under every sort, and blanks
sort last in both directions. The grid states its order in a line above the
rows, since rank order is the point of a backlog and was otherwise invisible.

The Epics tree's row kinds all emit the same seven cells now. An epic header
used to fold key and summary into one spanning cell while a child split them,
and with the status and points tracks sized `auto` in each row's own grid,
column 5 started up to 110px apart between the two; those tracks are fixed for
that reason, since a grid cannot see its siblings without `subgrid`.

## Profiles

Manage Profiles is XTM's dialog, feature for feature: the profile list on the
left with the launch-default star, the selected profile's `ProfileForm` on the
right, Create and Import above the list, Export and Delete in the open
profile's footer, and the start state that opens on nothing so the active
connection is never edited by accident. The markup and the styles are the
shared ones in `frontend/core/styles/primitives.css`, not a second copy.

The form drops the fields that only mean something to Xray: the Kiwi/Xray
backend selector, the bug issue type, the bug project, and the cross-project
sources. `App.UpdateProfile` reads those off the saved row and writes them
back unchanged, so editing a profile in TAM never resets what XTM configured
on it. In their place the form carries TAM's own requirement issue type,
which lives in `tam.db` as the per-profile setting `requirement_issue_type`
and so is loaded and saved around the profile write, not with it.

Export writes the same credential-free JSON shape XTM's exporter does, so a
file from either app imports into the other; an imported profile has no token
until one is entered. A Kiwi profile file is refused.

## Layout

    main.go              Wails entry point, window, menu
    app.go               App struct: startup, health, diagnostics, settings
    app_profiles.go      the profile methods: CRUD, connection test, export and import
    app_issues.go        the issue methods: sync, list, detail, per-profile settings
    app_writes.go        the write methods: edit, create, commit, and conflict resolution
    app_imports.go       the import methods: preview, mapping, and creating drafts from a file
    app_boards.go        the board methods: list boards, list sprints, get a board's view, sync boards
    internal/tamstore/   TAM's own SQLite file (schema version 6: issue (with status_id), issue_link,
                          sync_state, profile_setting, jira_user, board, board_column, board_issue,
                          sprint, plus the shared journal tables pending_change and audit_log)
    internal/backend/    IssueBackend and BoardBackend seams and DTOs; backend/jira on core/jira,
                          backend/demo on internal/demo
    internal/demo/       the Acme Platform (PLAT) dataset behind a "demo" profile
    internal/issuerepo/  the store layer: issue cache, detail cache, links, sync state, profile
                          settings, the pending-change journal, and drafts; tree.go groups the
                          cache into the Epics view's tree; boardwrites.go, movevalue.go,
                          movecolumns.go, and rebasemoves.go are the three board moves, their
                          before_val/after_val packing, and what Override does to a held one
    internal/boardrepo/  the store layer over board, board_column, board_issue, and sprint; view.go
                          composes the Boards view's data over the issue cache through IssueSource;
                          cellorder.go is the board's final local order the commit pass ranks against
    internal/committer/  pushes the journal to Jira and resolves conflicts; boards.go, ranks.go,
                          and boardvalues.go are the board pass, after the edits and before the links
    internal/importer/   maps import columns to draft fields and validates rows
    internal/syncer/     the paging engine; emits tam:sync-progress through app_issues.go; boards.go
                          is the boards pass, reached through backend.BoardBackend
    internal/errtext/    reduces an error to one readable line (strips HTML tags, collapses
                          whitespace) for sync summaries and dropped-board reasons
    internal/suiteprofiles/  which shared profiles TAM shows, demo detection, validation
    frontend/            React app on @agile-suite/core (see ../frontend/core)
      src/api.ts         typed access to the bindings; plain shapes for fixtures
      src/lib/keyColumn.ts  the issue-key column width both tables share
      src/queries/       TanStack Query keys, hooks, and the post-sync invalidation
      src/contexts/      SyncContext on the shared sync reducer
      src/lib/boardCells.ts  the board's position arithmetic: keyboard focus and navigation
                          over the lane/column/index grid
      src/lib/cardMove.ts  the drag/keyboard arithmetic a board move shares: where a drop lands
                          in a cell, which column a key press steps to, whether either changes
                          anything; cardMoveState.ts folds a card's pending row, warnings, and
                          commit failures into one state; moveValue.ts reads a journaled board
                          value back for the Pending changes dialog and the Activity tab
      src/components/    BacklogView, IssueTable, IssueDetailPanel, EditableFields, ActivityTab,
                          AssigneePicker, PriorityPicker,
                          PendingChangesModal, ConflictCard, NewIssueModal, ProfilesModal,
                          ProfileForm, AboutModal, ImportIssuesModal, AddLinkForm, EpicsView,
                          EpicTree, EpicRow, BoardsView, BoardsToolbar, BoardBody, BoardGrid,
                          BoardCard, BoardNotes, useBoardMoves (the three writes, the drag state,
                          the keyboard moves), useMovedCard, CardMoveMenu, BoardMoveBanner,
                          PendingMoveRow, CommitBanner
      wailsjs/           GENERATED bindings, do not hand-edit

## Commands

    wails dev                      # run with hot reload
    wails build                    # build/bin/task-activity-manager.exe
    go test ./internal/...         # Go tests
    cd frontend; npx vitest run    # frontend tests
    cd frontend; npm run build     # tsc + vite build

`npm install` runs at the repo root (npm workspaces). `frontend:install` in
wails.json does that for you.

## Conventions

Same as XTM's: logic in `internal/`, `app.go` only adapts it to Wails; Jira
is the system of record; credentials go to the OS credential manager only;
`TODO(tam): desc` marks planned work. TAM creates Jira profiles, which core
stores with backend `xray`; Kiwi profiles from XTM are hidden. UI text uses
no em dashes.

Sync scope is `project = KEY AND issuetype in (Task, Epic, Story, Bug, <requirement
type>)` plus the profile's scope JQL; incremental syncs add `updated >=` the last
sync minus an hour. The requirement type name is the per-profile setting
`requirement_issue_type`. The Sprint and Epic Link field shapes are marked
`NOTE(tam)` in `internal/backend/jira/fields.go` until verified on a real
instance.
