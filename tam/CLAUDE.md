# CLAUDE.md

Task Activity Manager (TAM) is the agile task-management app of the suite:
Jira DC tasks, epics, stories, bugs, and requirements for scrum masters,
product owners, and team members. It shares connection profiles and the
Windows Credential Manager entries with Xray Test Manager through
`core/profile` and the shared `profiles.db`. The design lives in
`docs/superpowers/specs/2026-09-04-tam-foundation-design.md`; the Outline
collection "Task Activity Manager" mirrors it.

## Status

Plan 1a (issues, read path): sync by project into `tam.db`, the Backlog
grid, and a read-only detail panel, on the demo dataset or a live Jira DC.
Plan 1b adds the journal, create and edit, and Commit. Plan 1c adds Excel
import, cross-project links, and requirement creation. Phase 2 (this
branch) adds the epic and story hierarchy: the Epics view, `parentKey` as
the seventh editable field, and epic creation.

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
"imported from <file>"). Links: the Links tab's Add link form journals a
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
    internal/tamstore/   TAM's own SQLite file (schema version 4: issue, issue_link, sync_state,
                          profile_setting, jira_user, plus the shared journal tables pending_change
                          and audit_log)
    internal/backend/    IssueBackend seam and DTOs; backend/jira on core/jira, backend/demo on internal/demo
    internal/demo/       the Acme Platform (PLAT) dataset behind a "demo" profile
    internal/issuerepo/  the store layer: issue cache, detail cache, links, sync state, profile
                          settings, the pending-change journal, and drafts; tree.go groups the
                          cache into the Epics view's tree
    internal/committer/  pushes the journal to Jira and resolves conflicts
    internal/importer/   maps import columns to draft fields and validates rows
    internal/syncer/     the paging engine; emits tam:sync-progress through app_issues.go
    internal/suiteprofiles/  which shared profiles TAM shows, demo detection, validation
    frontend/            React app on @agile-suite/core (see ../frontend/core)
      src/api.ts         typed access to the bindings; plain shapes for fixtures
      src/lib/keyColumn.ts  the issue-key column width both tables share
      src/queries/       TanStack Query keys, hooks, and the post-sync invalidation
      src/contexts/      SyncContext on the shared sync reducer
      src/components/    BacklogView, IssueTable, IssueDetailPanel, EditableFields, ActivityTab,
                          AssigneePicker, PriorityPicker,
                          PendingChangesModal, ConflictCard, NewIssueModal, ProfilesModal,
                          ProfileForm, AboutModal, ImportIssuesModal, AddLinkForm, EpicsView,
                          EpicTree, EpicRow
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
