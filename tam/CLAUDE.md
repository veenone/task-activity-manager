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
import, cross-project links, and requirement creation. Phase 2 adds the
epic and story hierarchy: the Epics view, `parentKey` as the seventh
editable field, and epic creation. Phase 3a adds the Boards view: boards,
columns, and sprints synced from Jira's Agile API, read only. Phase 3b
makes the board writable: a card dragged or keyboard-moved across
columns, within a column, or into another sprint journals the same way
every other TAM write does, and Commit pushes it. Phase 3c
closes Phase 3 with the sprint ceremonies: starting and completing a
sprint from the Boards toolbar, moving several selected cards into a
sprint at once, and the sprint field in the detail panel, plus the board
read now taking one transaction so a reader can never observe a board
mid-write. This branch puts a sprint choice everywhere an issue appears
rather than only on its board: the Backlog and the Epics tree, the New
issue dialog, and a Sprint column in the spreadsheet importer, all
reading the same profile-wide list of open sprints the board already
drew from. It also adds the fourth view, Sprints: a board picker, that
board's sprints as a two-level tree with the board's unassigned work
folded in, and a detail panel beside it. Creating, editing and deleting a
sprint live here, and reach Jira the moment they are pressed rather than
waiting for Commit, the same exception Phase 3c carved out for starting
and completing one. Filling a sprint, by contrast, is an ordinary
journaled move. Schema version 7 adds the sprint's goal, which existed on
the wire since Phase 3a and nowhere in TAM until now.

## Phase 4: the sprint report, the reconstruction

`internal/reports` turns a sprint's issues and their changelogs into its
numbers: committed, added, removed, completed, carried over, and the day by
day line behind them. `Build` takes one sprint, a rule for what counts as
finished, the issues with their history, a clock, and a location;
`Velocity` runs the same reconstruction over the last `Depth` (six) closed
sprints and gives each row its own unit. Nothing stores or draws either yet;
the design is
`docs/superpowers/specs/2026-09-09-tam-reports-design.md`, sections 3 and 7.

**It runs backwards before it runs forwards, and everything else here is
detail beside that.** A changelog is today's field values plus a list of
deltas, so status, estimate and sprint membership *at the sprint's start*
are not given: they are derived by undoing every change dated after the
start, taking each one's "from" side, and only then replayed forward one
local day at a time. A forward replay seeded from today's values draws a
sprint that never happened, and it looks entirely plausible while doing it:
an issue reopened, re-estimated and moved to the next sprint a week after
this one closed would start the reconstruction already finished, with an
estimate nobody had agreed to. `rewound` in `series.go` is that pass, and
the walk never replays a change dated after the sprint's end, which is what
keeps the two halves consistent.

Three smaller things that each look like a one line simplification and are
not:

- **Every timestamp goes through `internal/sprintdate`.** Jira's offsets
  carry no colon, so `time.RFC3339` rejects the real thing outright while a
  fixture written with a `Z` passes. The fixtures in
  `internal/reports/*_test.go` are written in Jira's format for that reason.
- **A day is a local day in the location `Build` was handed.** Day one is
  the local date of the sprint's start and boundaries are local midnight.
  Bucketing in UTC gives a team ten hours ahead a day one that begins the
  previous afternoon.
- **The Sprint field's changelog values are comma separated lists.** A card
  sits in two sprints at once during a rollover, so `"12, 13"` to `"13"`
  means it left 12 and stayed in 13; membership is a set test per change,
  never a toggle. Both the id and the name are matched, because the
  backend's normaliser keeps whichever half of Jira's parallel id/name pair
  the field populated.

What the reconstruction cannot see is section 3 of the design and is not a
footnote: the issues come from a `sprint = N` search, which answers with
whoever is in the sprint now, so a card dragged out on day four and left out
is never fetched. Committed is a floor, and Removed can only ever hold cards
that left and came back. Every surface that prints either number has to say
so.

**`internal/donerule` is one definition of done, not a new one.** The
board's last column rule was unexported on `sprints.Service`, and the report
needs the same answer the sprint completion acts on. It moved to its own
package, `sprints.completeStatuses` calls it, and `donerule.Done` returns
nil rather than a silent false when the columns cannot answer, since a board
that was never synced is not a board where nothing is finished; the caller
words that refusal, and `donerule.LastColumn` is what it words it from.

**The name based rule still exists and is still right where it is.**
`backend.IsDone` matches on the status *name* and powers the Backlog grid's
chip, the Epics tree's counts, the board's done points and the Sprints
view's per-sprint numbers, with the frontend's `statusClass` mirroring that
list for the chip. It answers a question about one issue with no board in
hand, from a name the cache already carries. The frontend's
`lib/unfinished.ts` is the other rule, the column one, written again in
TypeScript for the views that decide what to draw. So the name rule and the
column rule can disagree, on a board whose last column collects a status
named something else, and the sprint report is the surface that makes it
visible: a user comparing the Sprints view's done count against a report's
completed figure is looking at two different questions. `donerule`'s
package comment is where that is written down.

Jira's `completeDate` now reaches TAM: `core/jira.RawSprint`, `backend.Sprint`,
`boardrepo.Sprint`, and the `sprint` table (schema version 8) all carry it,
written through `writeSprints`, the one seam both `ReplaceBoard` and
`ReplaceSprints` call, so a sprint completed through either path keeps the
field. `Build`'s own walk does not read it yet: it still stops at `endDate`
(or at `now`, for a sprint still running), so wiring the actual close date
into the reconstruction is separate, later work.

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
`issuerepo`: what it needs from the issue cache is the three-method
`IssueSource` interface (`IssuesByKeys`, `DraftIssues`, `PendingMoves`),
which `app.go` satisfies with the issue repository it already holds. Every
method takes the `dbtx.Querier` the board read is running on, so the cards
and moves come from the same transaction as the board's own columns and
membership rather than from a later moment on the handle. `boardrepo.Board`
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

## Phase 3c: the sprint lifecycle

Starting a sprint and completing one are the only writes in TAM that reach
Jira outside a Commit. Everything else in this app is journaled and waits
for the user to push it; these two do not, for reasons that do not apply
to a card move. A sprint's start is a timestamped fact a whole team reads
the moment it happens, and Phase 4's burndown will be computed from it, so
journaling it would mean TAM decides when the sprint started and tells
Jira an hour later. A completion is the harder case: what happens to the
issues that did not finish depends on the sprint's contents at the exact
moment it closes, not at whatever moment a Commit next happens to run, and
there is nothing to reconcile the way a held transition or rank can be, a
sprint someone else already started cannot be started again. `internal/sprints`
owns both ceremonies (`Service.Start`, `Service.Complete`) so the exception
has one home and one place to test; nothing in that package touches the
journal. The bound methods, `StartSprint` and `CompleteSprint` in
`app_sprints.go`, take the same per-profile lock (`a.acquire(p.ID, "sprint")`)
a sync, a commit, and a boards refresh take, and the frontend reaches them
through `SyncContext.runSprintCeremony`, which is the same reducer path
`runBoardsRefresh` uses. The three sprint management writes in
`app_sprintmanage.go` take that same Go lock but reach it through
`runQuietLock` instead, for the reason the sync section below gives. Neither button has an offline state: TAM has no
connectivity signal to disable one from, so both stay enabled, the call is
attempted, and a transport failure or a Jira refusal (a second active
sprint, a missing Manage Sprints permission) is reported in the dialog,
which stays open with what the user typed still in it.

A completion moves the sprint's unfinished issues before it closes the
sprint, never after: the reverse would leave a closed sprint whose cards
went nowhere, which nobody can undo from TAM. "Unfinished" is one
definition used everywhere it matters: an issue whose status id is not in
the board's last `board_column`'s `status_ids`, the same mapping the board
itself draws with and the same one `DonePoints` counts by. The sprint's
own membership is re-read from Jira through the issue search rather than
from the cache or from `BoardIssueKeys`, because the cache can be minutes
stale and a key list carries no status; the search comes back with both in
one paged call. The push itself moves in chunks of twenty
(`sprints.pushBatch`, matching the committer's own `sprintBatch`), because
Jira's bulk endpoints answer a partial refusal with a 207 that names
issues by numeric id, which cannot be mapped back to a key, so the whole
batch fails together and a smaller batch limits how much of a completion
one refusal can take down.

A completion that reached Jira and then failed is reported inside
`sprints.Completion` (`Moved`, `MovedTo`, `Failed`, `Note`, `Message`)
rather than as a Go error, because Wails discards a bound method's return
value whenever the method also returns a non-nil error: the dispatcher
fills in either the result or the error and never both. An error would
therefore deliver the sentence and drop the counts and keys it is about,
which is what the dialog needs at that moment. Both failures travel that
way, a push that stopped partway and a close Jira refused once every card
had already moved, and for a second reason as well: the dialog renders a
`Message` as an outcome and a Go error as a refusal, so the refused close
used to print its accurate sentence directly above a list still headed "47
cards are not finished and will move out of the sprint" and a footer still
promising the move. `CompleteSprint` returns a real error only for the
refusals that happen before anything moves, and those go through
`internal/errtext` first, as does the `Message` a failed push or a refused
close carries: Jira's words come straight off the wire, and a Data Center
answering 403 with an HTML login page would otherwise put a kilobyte of
markup inline beside the start dialog's buttons.

`Note` is the opposite case: the ceremony worked and the bookkeeping after
it did not. `refreshSprints` answers with its own failure now rather than
only logging it, because the cached row still says `future` for the sprint
that is now running, so the toolbar offers Start for it and Jira answers
that second start with a 400. Both ceremonies carry it back as one line
telling the user to press Refresh, beside their own success; `StartSprint`
returns it as a string, and the board's ceremony banner is where both are
read.

A completion refuses a sprint the cache calls `future`, and only `future`.
It moves the cards out before it asks Jira to close the sprint, so aimed at
a sprint that never started it empties that sprint and then fails the
close, and TAM can undo neither half. A sprint started on the web an hour
ago still reads as future in a cache nobody has refreshed since, and
refusing that costs a Refresh where emptying it costs the sprint, so no
other state is refused here.

The cards that move are written back into both scopes of the board cache,
the sprint they left and the destination they were sent to, the second of
which used to be missed: the banner said twelve cards moved to Sprint 15
and the picker switched to Sprint 15, which drew exactly what it drew
before, and nothing else would have corrected it, since a ceremony writes
no journal row for the view to fold in. An empty destination is the board's
own list, which is a scope like any other.

The demo backend narrows its search to `sprint = N`. That is the one scope
it honours, and it is not decoration: the completion's own read is that
query, and the service keeps an issue the backend reports no sprint for on
the grounds that the query already narrowed it. Against a backend that
ignored the scope, every card in the project walked past that guard, so
completing a sprint on the demo profile moved the whole backlog and
reported success.

A board read now runs inside one deferred read transaction
(`boardrepo.Board`, `Order.CellOrder`, both through `Repository.inReadTx`),
and every read the issue cache does on the way, `IssuesByKeys`,
`DraftIssues`, `PendingMoves`, takes the `dbtx.Querier` that transaction
opened rather than the bare handle. `ReplaceBoard` writes a board's row,
columns, sprints, and membership together in one transaction and was
already correct; the gap was on the read side, where `Board` used to issue
four separate statements on the handle and could land between two of
`ReplaceBoard`'s writes, drawing cards into columns that had already been
replaced or a membership list that had not been written yet. That is the
flake two sessions chased before this plan named it: a reader on another
connection sees a consistent snapshot only inside a transaction, and a
read spread across the handle never had one. The sprint lifecycle itself
does not write an empty sprint list back over a board it has just acted
on: `sprints.Service.refreshSprints` refuses to persist what
`BoardSprints` answers with when the answer is empty, because a single 400
on the sprint endpoint is indistinguishable from "no sprints" the way
`core/jira` maps it, and a ceremony has just proven the board has at least
one sprint. Overwriting the cache with that empty answer would delete
every sprint row of the board and drop the sprint length the start dialog
suggests from, with nothing reported anywhere since the call itself did
not fail; `boardrepo.ReplaceSprints` does the write once the caller has
decided the list is real.

The board's multi-selection (`lib/boardSelection.ts`, `useBoardSelection`)
is a set of issue keys, never a set of positions, because the board
redraws on every refetch and a position-based selection would silently
select whatever cards happen to land in those slots afterwards, the same
class of bug 3b hit with keyboard focus. It is also a different thing from
the detail panel's one selected card: the panel's selection stays
`.board-card-selected` and opens on a plain click or Enter; the
multi-selection paints `.board-card-checked`, grows with a control-click
or a shift-click range, and more than one checked card closes the detail
panel and replaces it with `BoardSelectionBar`, because a panel describing
one card while three are checked would be lying about what the next
action touches. Its one action, "Move N cards", journals through the same
`issuerepo.MoveManyToSprint` (`moveToSprintTx` shared with the single-card
move) that every other board write uses, so it takes no guard, has a
conflict story already reviewed in 3b, and needs no new case in Discard,
the pending dialog, or the commit pass.

The detail panel's Sprint field, and the start dialog's suggested end
date, both read the cache rather than Jira: `SuggestSprintDates` combines
`boardrepo.SprintLength` (the median whole-day length of the board's last
three closed sprints, zero when none exist) with the board's cached
sprint list, so the dialog can open with a plausible date before any
network call and say plainly whether the date came from history or from a
two-week default.

## A sprint from anywhere an issue appears

The sprint list away from a board is profile-wide. `boardrepo.OpenSprints`
reads every active or future sprint across every board the profile has
synced, each carrying its board's name, and the detail panel offers that
list in the Backlog and the Epics tree. A user in the Backlog is thinking
about an issue, not about a board, so making them pick a board first to
reach a sprint would be the app's own model leaking into their task. Two
boards can hold a sprint of the same name, which is why a choice carries
its board's name when the name alone would be a guess.

A draft's sprint is pushed after its create, not sent with it. The Sprint
field is missing from most Data Center create screens, and the create
path already sends none of the board state for that reason.
`journalDraftSprint` runs inside `Rekey`, in the same transaction that
repoints the draft's other rows, and writes an `issue_sprint` row under
the real key with an empty before value, so the board pass of the same
Commit classifies it as a push rather than a conflict and lands it right
after the create. The one path that loses it is
`MarkCreatedWithoutRekey`, where Jira accepted the create and the local
rename failed: it drops the sprint the same way it drops the rank
repointing, and says so in its audit note.

The importer's Sprint column is matched by name against the profile's
open sprints, case insensitively. An empty cell is the backlog; a name
that matches nothing fails the row and lists what was available; a name
two different sprints share is refused rather than guessed at, and for
that reason is left out of the template's dropdown entirely. On a row
that carries a Key the cell is ignored, and the result now says how many
rows that happened to.

Writing a draft's sprint onto its row does not change what the board
draws. `CreateDrafts` writes `sprint_id` and `sprint_name` onto the draft
row, the same two columns `moveDraft` already writes when a draft is
dragged into a sprint, but `composeBoard` appends `DraftIssues`
unfiltered, so a draft was already drawn in every board and every sprint
of the profile, and `applyMoves` only drops a card carrying a pending
sprint-move journal row, which a draft never has. What those two columns
change instead is the Backlog's sprint filter, the Backlog grid and
detail panel, and `ListSprints`, whose `DISTINCT sprint_id` over cached
rows can now surface a sprint id contributed only by a draft.

## The Sprints view

Phase 3c gave TAM sprints on a board: draw one, drag cards through it,
start it, close it. It could not make one, rename one, fix a wrong date,
delete one created by mistake, or look at a sprint's contents without
first choosing the board that happens to carry it. This view is what does
those things, between Boards and Reports in the tab order, and it is
always present: hiding it for a project with no scrum board synced would
have needed a cross-cutting navigation mechanism for one consumer, and
would have shown a brand new profile nothing at all on its first launch,
since the condition that would hide it reads a cache that profile has not
filled yet. A project with no scrum board sees an empty state that says
so and points at the Boards view instead.

**Create, edit and delete reach Jira immediately, and the honest reason
is a cost, not a principle.** The tempting explanation is that a sprint
is more of a shared Jira object than an issue is, and it is not: a new
issue is every bit as much a thing a whole team plans around, and TAM
journals it behind a `TAM-NEW-n` placeholder and pushes it on Commit like
everything else. The real reason is that TAM's journal is issue
machinery. A pending change is keyed by issue key, a conflict is decided
by comparing an issue's `updated` stamp, and Commit walks issues. A
sprint has none of that: no cached version to rebase an edit on, no
conflict card, no rekey path for an id Jira has not handed out yet.
Journaling these three writes would mean building a second journal for a
second kind of entity, with its own placeholder ids for create, its own
conflict story for edit, and a queue holding a destructive intent for
delete, for three calls a user makes a handful of times per sprint. The
full argument is written on `UpdateSprint` in `core/jira/sprintwrite.go`,
which is where a future exception should start reading rather than
re-deriving the point from scratch.

The fence is structural rather than a sentence in a spec, because the
previous version of this rule lived in one sentence in the boards design
and lasted one phase. `internal/sprints/exceptions_test.go` asserts, by
name, that `sprints.Service`'s exported method set is exactly `Complete`,
`Create`, `Delete`, `Edit`, `Start`; growing it means editing a failing
test whose message says what the list is for. The fence is deliberately
the service's own methods and not the `lifecycle` interface a ceremony
uses internally: that interface also carries `BoardSprints`, a read, and
`MoveIssuesToSprint`, whose other caller (the multi-select move) journals
it like every other membership change, so asserting `lifecycle` as the
immediate-write list would have been false the day it was written.
Membership stays journaled everywhere in this view exactly as it does on
the board: the detail panel's Sprint field and the tree's own multi-select
move both go through the same journaled `MoveManyToSprint` path the
board's selection uses, with the same conflict story and the same
Discard case, because reaching a sprint without first picking its board
is the whole reason this view exists, not a reason to grow a second write
path.

A closed sprint carries no cached membership, by design and not by
accident: the boards sync never fetches a closed sprint's issue keys, on
the reasoning that a chart Phase 4 draws from it should not depend on a
mostly-idle poll of history nobody asked for. `SprintDetail.Issues` is
therefore empty for a closed sprint for the same reason it would be
empty right after a version 5 migration and before the next sync, and the
view shows a closed sprint's contents as unavailable, with the reason,
rather than as an empty sprint, which would be a lie the row cannot tell
apart from the truth.

**Delete spans two repositories in two transactions, board rows first.**
`sprints.Service.Delete` calls Jira, then `boardrepo.DeleteSprintEverywhere`
to remove the sprint's row and its membership from every board of the
profile that holds a copy (Jira hands the same sprint to every board
whose filter reaches it, so a delete scoped to one board would leave a
second board's copy in `OpenSprints`, still offering a sprint Jira has
already destroyed to the New issue dialog, the detail panel, and the
importer's Sprint column), then `issuerepo.ClearSprint` to blank the
sprint's name off the issues that carried it. The two are separate
transactions in separate repositories on purpose: `dbtx.In` opens its own
transaction from the handle, so nesting one repository's helper inside
the other's takes a second pooled connection, blocks on the first's write
lock, and dies on the driver's busy timeout, the same failure
`MoveManyToSprint` already documents. A shared transaction helper
spanning both is real work and is recorded as deferred. `boardrepo`'s own
comment on `DeleteSprintEverywhere` carries the rest of the argument and
what a crash between the two transactions leaves: board rows first means
a crash leaves issues whose `sprint_id` and `sprint_name` still name a
sprint that is gone, stale text on cards that already held it; the other
order would leave the sprint alive in `OpenSprints`, offering it as a
pickable destination everywhere an issue's Sprint field appears. Narrow
stale text beats a dead sprint that can still be chosen, and only a full
issue sync repairs either.

**Scope `""` in `board_issue` is the board's own list, not a backlog.**
It is every issue on the board, sprint issues included, which is what
TAM's own code calls the board's own list; rendering it as-is would list
every sprint's issues a second time and offer the fill bar work that is
already in a sprint. So the tree's unassigned node, `UnassignedSprintName`
("Board backlog", deliberately not "Unassigned": that word already names
an assignee group two rows up in the same tree), is computed rather than
read: `boardrepo.sprintDetails` builds it as the board's own list minus
every key a sprint's journal-replayed scope holds, once the journal has
been replayed over every sprint ahead of it in the same pass.

**The delete confirmation's issue count says "at least" for three
different reasons, never for one.** `notSynced` counts keys the sprint
holds that the issue cache does not, so the true count is short by
exactly them. `truncated` means the shared per-view card budget stopped
this sprint's own list short, so the count cannot be checked against what
is on screen. And the third is this view's own doing: a pending sprint
move is replayed over the scope before the total is counted, so a card
journaled out of the sprint but not yet committed has already left the
count while Jira still holds it, and the reverse holds too, a card
journaled in counts here before Commit has pushed it. Any one of the
three turns the sentence into "at least N issues" with a line naming
which; quoting an exact count that turns out to be low costs a sprint
nobody can get back.

Schema version 7 adds `goal` to the `sprint` table, `RawSprint`,
`backend.Sprint`, and `boardrepo.Sprint`. It does not back-fill: unlike
version 5's `status_id`, nothing here clears a sync watermark, because a
sprint is not read by an issue sync and its only refresh is the Boards
view's own Refresh button. A sprint cached before version 7 keeps an
empty goal until the next boards refresh rewrites it. Clearing a goal is
sent as an explicit empty string on edit and never on create, because the
partial-update rule that stops an empty box from wiping a real goal on
`UpdateSprint` is also what makes a goal impossible to clear otherwise;
`clearGoal` is how `sprints.Service.Edit` tells the two apart.

The three management writes reach their lock the same way the two
ceremonies do, `a.acquire(p.ID, "sprint")` in Go, but the frontend reaches
them through `SyncContext.runQuietLock` rather than `runSprintCeremony`:
that is the exception the "One lock, both ends" section already
documents, and what it costs is written there, not repeated here.

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
`core/importfile`, XTM's parser lifted out), maps columns to the nine draft
fields (`internal/importer`), validates rows with file row numbers, and
creates the valid rows as drafts in one transaction (`CreateDrafts`, audited
"imported from <file>").

A tenth column, Key, decides what a row does. Empty, the row creates a
draft. Filled with an issue key or a `.../browse/KEY` URL, it journals edits
to that cached issue instead (`EditFields`, one transaction, all or
nothing), so a sheet exported from Jira round-trips rather than duplicating
every row. On such a row the Type and Sprint cells are ignored (an issue's
type is not editable, and a sprint is a board write EditFields cannot
carry) and an empty cell means "leave this field alone", never "clear it";
a value that already matches is not journaled, so re-importing an unchanged
file leaves nothing pending. Summary is required only when no Key column is
mapped. `SaveImportTemplate` writes a real workbook
(`internal/importer/template.go`, excelize): an Issues sheet with the ten
columns, a Type dropdown carrying the profile's own requirement type name,
a Sprint dropdown carrying the profile's open sprints, five examples, and a
"How to use" sheet; naming the file `.csv` in the save dialog writes the
same columns as CSV. Assignee is the Jira username, not the display name.

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
default the picker may change, seeded from the epic on screen: the selected
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
bound method taking `acquire` has to take the frontend's lock too.**

There is one deliberate exception, and it is narrower than it looks.
`SyncContext.runQuietLock` takes the same `statusRef` guard every other `run*`
takes, so a sync, a commit, a boards refresh and a ceremony all still refuse
against it, but it dispatches no progress actions, so the reducer stays
`idle`. Creating, editing and deleting a sprint go through it: those are one
short call each, and flashing the whole app's sync banner for a rename would
say something untrue about what is happening.

What that costs is worth knowing, because it is the same shape as the bug the
rule above was written about. For the length of the call the shell's Sync and
Commit buttons stay enabled and do nothing when pressed, and the profile
picker stays enabled, because all three read `state.status` rather than the
ref. What keeps a user away from them is the dialog holding focus, which is
why those dialogs refuse Escape while their write is in flight rather than
merely disabling their own buttons. A fourth quiet write would have to earn
the same treatment.

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
    app_boards.go        the board methods: list boards, list sprints, get a board's view, sync
                          boards, the three journaled board moves, CanTransition, and
                          JournalSprintMoves, the selection's bulk move, with its guarded lookup
                          of the destination's name
    app_sprints.go       the two sprint ceremonies, SuggestSprintDates, and PendingInSprint
    app_sprintmanage.go  Create, Edit and Delete sprint, and ListBoardSprintDetails for the
                          Sprints view's tree, all under the "sprint" lock name the ceremonies use
    internal/tamstore/   TAM's own SQLite file (schema version 8: issue (with status_id), issue_link,
                          sync_state, profile_setting, jira_user, board, board_column, board_issue,
                          sprint (with goal, added at version 7, and complete_date, added at
                          version 8), sprint_report (a sprint's saved report, added at version 8),
                          plus the shared journal tables pending_change and audit_log)
    internal/backend/    IssueBackend and BoardBackend seams and DTOs; backend/jira on core/jira,
                          backend/demo on internal/demo
    internal/demo/       the Acme Platform (PLAT) dataset behind a "demo" profile
    internal/issuerepo/  the store layer: issue cache, detail cache, links, sync state, profile
                          settings, the pending-change journal, and drafts; tree.go groups the
                          cache into the Epics view's tree; boardwrites.go, movevalue.go,
                          movecolumns.go, and rebasemoves.go are the three board moves, their
                          before_val/after_val packing, and what Override does to a held one
    internal/boardrepo/  the store layer over board, board_column, board_issue, and sprint; view.go
                          composes the Boards view's data over the issue cache through IssueSource,
                          both in one deferred read transaction (tx.go); cellorder.go is the board's
                          final local order the commit pass ranks against, on the same kind of
                          transaction; sprintlength.go is the median-of-three-closed-sprints read
                          the start dialog's date suggestion is built from; sprintlist.go is the
                          Sprints view's own read, one board's sprints with their issues and the
                          computed unassigned node; deletesprint.go is DeleteSprintEverywhere, the
                          two-repository delete's first transaction, across every board that holds
                          a copy of the sprint
    internal/sprints/    the sprint writes that reach Jira outside a Commit: Start and Complete,
                          the ceremonies, and Create, Edit and Delete, the management writes this
                          view adds; exceptions_test.go fences the package's exported method set to
                          exactly those five; guards.go is what a write refuses before it reaches
                          Jira, cache.go the board cache's bookkeeping after it has, manage.go
                          Create, Edit and Delete themselves, and suggest.go the start and create
                          dialogs' suggested name and dates
    internal/sprintdate/ the one place a sprint date is parsed and written in Jira's Agile
                          datetime format, shared by the ceremonies, the suggestion, and every
                          timestamp the sprint report reads off a changelog
    internal/donerule/   the board's own definition of finished, a status its last column
                          collects, shared by the sprint completion and the sprint report;
                          backend.IsDone, the status-name rule, is a different question and
                          stays where it is
    internal/reports/    the sprint report's reconstruction: reports.go is Build and the unit
                          it counts in, series.go the rewind and the day by day walk, velocity.go
                          the last six closed sprints with each row's own unit
    internal/dbtx/       the one transaction helper issuerepo and boardrepo share: In for a write,
                          InRead for a deferred read-only transaction, and the Querier interface a
                          read helper takes so it can run on the handle or inside either kind
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
      src/lib/boardSelection.ts  the board's multi-selection: a set of keys and the three
                          gestures over an ordered key list, no React and no DOM
      src/queries/       TanStack Query keys, hooks, and the post-sync invalidation
      src/contexts/      SyncContext on the shared sync reducer; runSprintCeremony is the reducer
                          path StartSprint and CompleteSprint run through, beside runBoardsRefresh;
                          runQuietLock takes the same lock without the banner, for the three
                          sprint management writes
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
                          PendingMoveRow, CommitBanner, BoardCeremonies (the two sprint dialogs'
                          shared context), StartSprintModal, CompleteSprintModal,
                          BoardSelectionBar (the multi-selection's count, destination, and Move
                          action), useBoardSelection (the selection as React state), SprintsView
                          (the board picker and the tree), SprintList, SprintRow, SprintField (the
                          goal, shown only while a sprint is expanded), SprintFillBar,
                          CreateSprintModal, EditSprintModal, SprintDraftForm (the four fields the
                          create and edit dialogs share), useSprintSelection (the tree's own
                          multi-select, the board's under a different name)
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
