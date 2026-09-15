# CLAUDE.md

Task Activity Manager (TAM) = agile task app of suite: Jira DC tasks,
epics, stories, bugs, requirements. For scrum masters, product owners,
team. Share connection profiles + Windows Credential Manager entries with
Xray Test Manager via `core/profile` + shared `profiles.db`. Design live in
`docs/superpowers/specs/2026-09-04-tam-foundation-design.md`; Outline
collection "Task Activity Manager" mirror it.

## Status

Plan 1a (issues, read path): sync by project into `tam.db`, Backlog grid,
read-only detail panel, on demo dataset or live Jira DC. Plan 1b add
journal, create, edit, Commit. Plan 1c add Excel import, cross-project
links, requirement creation. Phase 2 add epic+story hierarchy: Epics view,
`parentKey` as seventh editable field, epic creation. Phase 3a add Boards
view: boards, columns, sprints from Jira Agile API, read only. Phase 3b
make board writable: card dragged or keyboard-moved across columns, within
column, or into another sprint journal same way every other TAM write do,
Commit push it. Phase 3c close Phase 3 with sprint ceremonies: start +
complete sprint from Boards toolbar, move many selected cards into sprint
at once, sprint field in detail panel, plus board read now take one
transaction so reader never see board mid-write. This branch put sprint
choice everywhere issue appear, not only on its board: Backlog, Epics tree,
New issue dialog, Sprint column in spreadsheet importer, all read same
profile-wide list of open sprints board already draw from. Also add fourth
view, Sprints: board picker, that board's sprints as two-level tree with
board's unassigned work folded in, detail panel beside it. Create, edit,
delete sprint live here; edit + delete of sprint Jira hold reach Jira moment
pressed, same exception Phase 3c carve out for start + complete, while
create journal draft sprint (bundle 01, below). Fill
sprint = ordinary journaled move. Schema version 7 add sprint goal, which
exist on wire since Phase 3a and nowhere in TAM until now. Phase 4 fill
fifth view, Reports: numbers sprint review start with, committed, added,
removed, completed, carried over, rebuilt from Jira's own changelog not its
internal chart endpoints, with last six closed sprints as velocity table
beside them. Schema version 8 add `sprint_report`, where closed sprint's
reconstruction kept so view readable offline, and `complete_date` on
`sprint`. Charts those numbers would draw into = deliberately next plan;
here ship figures, method printed under them, plain statement of what
reconstruction cannot see.

## Phase 4: the sprint report, the reconstruction

`internal/reports` turn sprint's issues + changelogs into numbers:
committed, added, removed, completed, carried over, and day by day line
behind them. `Build` take one sprint, rule for what count as finished,
issues with history, clock, location. `VelocitySprints` = which closed
sprints velocity table cover, last `Depth` (six), oldest first; `Row` = what
one of them say, own unit per row; `internal/sprintreport` call both,
because half a table usually read back out of store, not rebuilt. Design =
`docs/superpowers/specs/2026-09-09-tam-reports-design.md`, sections 3 and 7.

**It run backwards before it run forwards, everything else here detail
beside that.** Changelog = today's field values plus list of deltas, so
status, estimate, sprint membership *at sprint start* not given: derive by
undo every change dated after start, take each one's "from" side, then
replay forward one local day at a time. Forward replay seeded from today's
values draw sprint that never happen, and look entirely plausible doing it:
issue reopened, re-estimated, moved to next sprint a week after this one
closed would start reconstruction already finished, with estimate nobody
agreed to. `rewound` in `series.go` = that pass, and walk never replay
change dated after sprint end, which keep two halves consistent.

Three smaller things that each look like one line simplification and are
not:

- **Every timestamp go through `internal/sprintdate`.** Jira offsets carry
  no colon, so `time.RFC3339` reject real thing outright while fixture
  written with `Z` pass. Fixtures in `internal/reports/*_test.go` written in
  Jira format for that reason.
- **Day = local day in location `Build` handed.** Day one = local date of
  sprint start, boundaries = local midnight. Bucket in UTC give team ten
  hours ahead a day one that start previous afternoon.
- **Sprint field changelog values = comma separated lists.** Card sit in two
  sprints at once during rollover, so `"12, 13"` to `"13"` mean it left 12
  and stay in 13; membership = set test per change, never toggle. Both id
  and name matched, because backend normaliser keep whichever half of Jira's
  parallel id/name pair the field populated.

What reconstruction cannot see = section 3 of design, not footnote: issues
come from `sprint = N` search, which answer with whoever in sprint now, so
card dragged out day four and left out never fetched. Committed = floor,
Removed can only hold cards that left and came back. Every surface printing
either number must say so.

**`internal/donerule` = one definition of done, not new one.** Board's last
column rule was unexported on `sprints.Service`, and report need same answer
sprint completion act on. Moved to own package, `sprints.completeStatuses`
call it, and `donerule.Done` return nil not silent false when columns cannot
answer, since board never synced is not board where nothing finished; caller
word that refusal, `donerule.LastColumn` = what it word it from.

**Name based rule still exist and still right where it is.**
`backend.IsDone` match on status *name* and power Backlog grid chip, Epics
tree counts, board done points, Sprints view per-sprint numbers, with
frontend `statusClass` mirroring that list for chip. Answer question about
one issue with no board in hand, from name cache already carry. Frontend
`lib/unfinished.ts` = other rule, column one, written again in TypeScript for
views that decide what to draw. So name rule and column rule can disagree,
on board whose last column collect status named something else, and sprint
report = surface that make it visible: user comparing Sprints view done
count against report completed figure look at two different questions.
`donerule` package comment = where that written down.

Jira `completeDate` now reach TAM: `core/jira.RawSprint`, `backend.Sprint`,
`boardrepo.Sprint`, `sprint` table (schema version 8) all carry it, written
through `writeSprints`, the one seam both `ReplaceBoard` and
`ReplaceSprints` call, so sprint completed through either path keep field.
`Build` own walk not read it yet: still stop at `endDate` (or at `now`, for
sprint still running), so wiring real close date into reconstruction =
separate later work.

## Phase 4: where the report is kept, asked for, and read

**What report cannot see = most important paragraph in this section.**
Issues come from `sprint = N` search, only membership JQL Jira offer, which
answer with whoever in sprint **now**. Card dragged out day four no longer
carry sprint N, so never fetched, changelog never read, removal leave no
trace. Jira own report get this right only because it keep private
sprint-change records public API not expose. So `Removed` can only hold
cards that left and came back, `Committed` = floor not total, and every
surface printing either one carry qualification: `SprintSummary` under the
sentence, `VelocityTable` under its own Committed column, which is not
redundancy but same rule applied twice, because Phase 5 Rituals publish
those figures out of `lib/reportText.ts` where no neighbouring paragraph to
borrow caveat from. Query whole project to recover removed cards = recorded
upgrade, not built.

**Method printed with numbers, and that = product decision not
documentation one.** TAM figures and Jira figures differ, in public, during
review, because different sources: Jira from private sprint records, TAM
from public changelog, with own done rule and blind spot above. One line
under summary say what done mean here, that history rebuilt from public
changelog not Jira stored sprint records, and that removals only visible for
cards that came back. Cost one sentence, turn argument in front of team into
footnote. Nobody read documentation during sprint review.

**Where report kept.** `sprint_report` arrive at schema version 8, keyed
`(profile_id, board_id, sprint_id)`. Board in that key for same reason
version 6 put it in `sprint`: Jira hand one sprint to every board whose
filter reach it, and report done rule come from own board's last column, so
key without board would serve board B a series built for board A and neither
board ever find out. Row carry `algo_version`, stamped on write from
`reports.AlgoVersion` and compared on read, so first reconstruction bug not
baked into every user database with no way out but delete file. Table in both
purge lists, `PurgeProfile` and `RemoveBoards`.

**Closed sprint series stored and served from store; live one neither
stored nor served.** Live sprint changed an hour ago, so stored copy =
confident wrong answer. Closed sprint numbers steady enough to keep, which
is not same as fixed, and two things that can still move them worth knowing
because neither obvious: membership, since series built from `sprint = N`
and card can move into or out of closed sprint, and board own done rule,
since column rearranged in Jira change what `donerule` answer without
touching single issue. Status and estimate changes after close move nothing,
which is what rewind for. `refresh` = how caller ask for series again, and
only invalidation path short of bump `AlgoVersion`: without it, report built
while Jira return cut-short changelogs keep its truncation marker forever.

**One binding, not two.** First draft had view ask for burndown and velocity
separately. `App.acquire` refuse not wait, and TanStack Query fire every
`useQuery` on component mount, so two bindings would fail one of them with
"a report is already running for this profile" every time view open.
`GetSprintReport` return both under one lock, share one fetch, and velocity
table reuse stored reports for sprints already built rather than refetch
six. `CancelSprintReport` = other half: Wails hand bound method no per-call
context, so cancel func live on `App` under same mutex `busy` do, and view
call it on unmount and on every board or sprint switch. Without it, report
nobody look at hold profile lock for minutes.

**Cost of open view is real and is reported.** Page = 25 issues against
sync's 50, because changelog expansion make each issue payload several times
size of row grid sync. **That = considered default not measured one**: step
4 of this phase wire probe, which would time 50 against 25 on real instance,
not run, so nobody know where real knee is. Progress go on
`tam:report-progress` with own frame type not on `tam:sync-progress`,
because shell banner read the latter and read that is not sync must not
announce one.

**`reports.Velocity` gone, and shape it had could not survive reusing
store.** Took map of every sprint fetched history and return whole table,
right shape when every row rebuilt and wrong one when five of six read back
out of `sprint_report`. Split in two = what let store in: `VelocitySprints`
pick which closed sprints table cover and in what order, `Row` turn one
series into one row, `internal/sprintreport` call both, so sprint built just
now and same sprint read back from store give identical row by construction
not by two code paths agreeing. `VelocitySprints` parse both dates not just
end, which stop sprint it already know it cannot rebuild from costing full
changelog fetch first; end-before-start case it cannot see caught later by
`ErrNoDates`, and each layer comment say which fault is whose.

`internal/sprintreport` = orchestration, `internal/reports` stay pure. Split
not tidiness: `reports` take issues + clock and touch no I/O, which make its
tests cheap, and everything that cost something (done rule, paged fetch,
store, progress frames) live other side of it. `app_reports.go` do what every
other `app*.go` do and no more.

**Condition view must render beside report travel in result, not as Go
error.** Wails fill in either bound method value or its error, never both,
limit `sprints.Completion` already built around. So board never synced,
sprint cache not hold, sprint with no readable dates, board that never closed
a sprint come back as `Report` carrying `Unavailable` reason and nothing
else, while refused lock, transport failure, database that will not answer
stay Go errors. Reasons = constants because frontend word them, in
`lib/reportText.ts`, one home for every sentence report print, tested without
rendering anything.

**Sprint id of 0 mean board's most recent closed sprint**, the call view make
before its picker have anything in it, and what make `noClosedSprint`
reachable at all. Negative id not that, answer `sprintNotFound`. `api.ts`
refuse sprint id that is not non-negative whole number before call, because
Wails marshal arguments with `JSON.stringify` and both `undefined` and `NaN`
arrive in Go as `0`: uninitialised picker would otherwise get newest closed
sprint report under whatever heading happened to be on screen. Heading drawn
from `series.sprintName`, the sprint backend answered about, never from
picker state = second line of defence for same failure.

**Charts not here, are next plan.** `Series.Days` carry day by day line and
nothing draw it. No SVG anywhere in this frontend, so axis ticks, label
collision, empty ranges, single day sprints, colour tokens, screen reader
access all new surface, and splitting mean numbers get trusted before
anything drawn from them.

## Rituals, local first

Ritual page live in `tam.db` (`ritual_document`), edited in TAM, reach
Confluence only on Rituals view **Sync rituals**. View make no Confluence
call on open, sprint pick, or edit. Design =
`docs/superpowers/specs/2026-09-14-tam-rituals-local-first-design.md`.

TAM own whole page. No markers, no splice: `body` = local page in storage
XHTML, `base_body` = page as of `confluence_version`, `conflict_body` +
`conflict_version` = newer remote Sync found while local edits pending
(schema version 12). Dirty computed (`page id empty or body != base_body`),
never stored; `status` only what view draw.

`internal/ritualtemplate` render five pages per sprint (`_sprint` overview,
planning, standup, review, retro), pure, no clock. **Deterministic on
purpose**: adoption compare stored body against fresh render to tell
untouched template from page somebody wrote in. Golden files in `testdata/`
are also frontend round-trip corpus, and `.gitattributes` keep them LF.
Jira issues = Jira Issues macro carrying one of three written JQL forms
(`JQL` write them, `ParseJQL` read them back): sprint's whole list, done
only, not done only. Only those three preview from cache; a bare
`sprint = N` is not one of them and opens unpreviewed.

`ritualsync.Ensure` write missing pages from templates, local, no lock, so
planning page writable offline; closed sprint get none. `ritualsync.Run` =
Sync pass under `a.acquire(p.ID, "rituals")`: title match space wide (titles
unique per space), adopt under root, refuse elsewhere by name, create, pull,
push base+1, remote-newer-and-dirty = conflict, 404 = gone and never
silently recreated. 409 on push re-read: newer version = conflict, 404 =
gone, same version = push failed (title clash, say) and row left as is,
since conflict at base version claim newer page that is not newer and Keep
mine loop on it. Sprint page gone or refused still let rituals that already
have page reconcile; only placing new ones wait. **Every row write blind to body or compare-and-set on body
read at pass start**, so save landing mid-push stay unsynced and mid-pull
become conflict, never overwrite. Base after push = body pushed, never
Confluence answer: Confluence normalise storage on save, and base from its
answer leave page dirty forever. Per-page trouble travel in `Result.Failed`,
not Go error (Wails either/or).

Local writes take no lock (`SaveRitualBody`, resolve, forget, delete, ensure),
same as board moves; compare-and-set make that safe. Other direction need own
guard: `SaveBody` take version and page id editor was opened on and update
only while row still hold both, else `ErrChangedUnderEditor` ("This page
changed while you were editing..."), shown on editor status line with text
left pending. Without it, text typed over page Sync pulled or created
overwrite it locally and next Sync push it over newer remote, no conflict.
`RemoveBoards` leave `ritual_document` alone: text people wrote, maybe never
pushed, and board leave Jira list for reasons that say nothing about it
(setting off, location change, lost permission). Only `PurgeProfile` remove
it. Demo rebuild restore conflict row from `conflict_body`/`conflict_version`,
since adopted page never had base. Demo Confluence URL
"demo" = `internal/demo.Confluence`, in memory, rebuilt from stored pages on
first use after restart (a page whose status is gone skipped, so it never
comes back under its old title and a later adopt sees a genuinely new page,
not a resurrected one), and stage one conflict on first Standup it create.

Rituals view Sync capture the board and sprint it started on and check both
still current before applying anything the pass read back, so switching
board or sprint mid-sync cannot overwrite what is on screen with another
sprint's result; both pickers disable for the run (`running === "rituals"`).
`onSaved`, the editor's own save landing after such a switch, replace a
document only when board, sprint and ritual type all match what the current
list holds, and drop one that matches nothing rather than graft it in. Same
captured-id check guard Sync error and reload after Keep mine, Take theirs,
Recreate, Remove. Editor `locked` from Sync press until its reload land
(`setEditable` on same instance, toolbar hidden, never a rebuild): lock alone
release before reload remount editor, and keystroke in that gap would be
saved against replaced version and refused. Open in Confluence built only
from http or https base URL.

Frontend `lib/storage` convert storage XHTML to TipTap JSON and back.
Everything not modelled = opaque node carrying raw XML, written back byte for
byte; fallback is rule, not list, and it also reaches shapes TAM recognises
but cannot hold exactly: a task list carrying an attribute anywhere in it, an
empty task id (the one field allowed to be empty), a task status that is not
exactly "complete" or "incomplete", a colspan/rowspan that is not a plain
integer greater than one, an empty `<strong>` or `<a>` with no text to carry
the mark. Namespace declarations strip only inside start tags, never out of
text or CDATA, so a code sample quoting one is left alone. Page that will not
parse open read only. Link click never navigate WebView (that take whole app
away from TAM): absolute http or https open in browser from read-only surface
or on Ctrl/Cmd click while editing, anything else go nowhere. `sanitizeHtml`
allow only http, https, mailto, relative in `href`, `src`, `xlink:href`,
`action`, scheme read after dropping control characters and whitespace
(`java&#9;script:` is javascript to browser), and drop style with `url(` or
`expression(`. Whole storage corpus (`lib/storage/corpus.ts`, test only) go
through editor real schema in `schemaRoundTrip.test.ts`, not just templates.

Editor saves when the serialized document differs from what was last saved
(the baseline serialized the moment it is created), never on a `touched`
flag: a checkbox click, a context-menu Cut, a spellcheck fix change the
document exactly as a keystroke does and none reliably raise one. Its
`useEditor` dependencies are `[editable]` only and its identity comes from
the parent's React `key`, so a local save never tears it down; 800 ms
debounce, Ctrl+S, flush on Sync and unmount trigger a save. Saves run one at
a time: after a success it re-serializes and queues another save if the
screen has moved on since, and after a failure it leaves the text pending and
waits for the next edit, flush, Ctrl+S or unmount rather than retry itself.
Editor keyed by board, sprint, ritual type, version and page id, so Sync
remount it and a local save do not. `lib/ritualText.ts` = every sentence.

Retired: wizard, `ritualdefaults`, page associations, `GetRitualPage` live
read. `confluence_association` and `confluence_page_cache` tables left in
`profiles.db`, unwritten, still purged with profile.

## Phase 3a: boards

`core/jira/agile.go` = Agile 1.0 transport: `Boards`, `BoardConfiguration`,
`Sprints`, `BoardIssueKeys`, each paged to exhaustion, return Jira raw
shape. Data Center with no Jira Software answer `Boards` with 404, mapped to
`ErrNoAgile`; kanban board sprint call answer 400, mapped to `ErrNoSprints`.
TAM `internal/backend.BoardBackend` = read-only capability those four calls
back, kept off `IssueBackend` so only backend that can speak Jira Agile API
answer for it; demo backend implement it too, with one scrum board (three
sprints, one closed) and one kanban board.

Schema version 5 add four tables to `tam.db`, all keyed by profile: `board`,
`board_column`, `sprint`, `board_issue` (board membership of one scope,
`sprint_id` empty for board's own list and sprint id otherwise). Also add
`status_id` column to `issue`, how card matched to column. Since
`CREATE TABLE IF NOT EXISTS` cannot add column to table that already exist,
this = plan's first store migration (`tamstore.Schema.Migrations`): add
column, then clear every profile sync watermark. **Incremental sync only
re-read issues Jira report changed since watermark, so row cached before
version 5 would never get status id filled in on its own; clearing watermark
= what make next sync for each profile re-read every issue and fill column
in, without purging anything first.**

Schema version 6 re-key `sprint` from `(profile_id, id)` to
`(profile_id, board_id, id)`. Jira Data Center hand same sprint to every
board whose filter reach it, and sync clear and write sprints one board at a
time, so two scrum boards over one project used to collide on second board
insert and take whole pass down. SQLite cannot change primary key in place,
so migration drop table and recreate it: it is cache next sync refill, and
nothing join to its rows.

`internal/boardrepo` = store layer over the four tables, beside `issuerepo`
since boards are own concern. Never import `issuerepo`: what it need from
issue cache = three-method `IssueSource` interface (`IssuesByKeys`,
`DraftIssues`, `PendingMoves`), which `app.go` satisfy with issue repository
it already hold. Every method take the `dbtx.Querier` board read running on,
so cards and moves come from same transaction as board's own columns and
membership rather than from later moment on handle. `boardrepo.Board`
compose view board draw: columns in board order, cards bucketed into them by
status id (draft go to first column that collect any status, since Jira
never assign it one), and lanes chosen swimlane (none, assignee, epic) ask
for. Card whose status in no column counted into `Unmapped`, and its status
name into `UnmappedStatuses`, not listed card by card. Cell cap at 200 cards
and whole view at 2,000; past either cap card only counted, in `Overflow`
and `Capped`. `DonePoints` summed in same walk, over every mapped card not
ones capped cell drew, by `backend.IsDone`: definition live in
`internal/backend` because both `boardrepo` and `issuerepo` count by it and
`boardrepo` may not import `issuerepo`. `NeedsStatusSync` true when every
cached card still carry empty status id, state right after version 5
migration and before next sync; view tell user to sync rather than draw
empty board and blame them for it.

`internal/syncer/boards.go` = `Engine.SyncBoards`, reached through type
assertion on `backend.BoardBackend` so backend without it skipped, not
failed. Run after issues pass in regular sync, and alone from Boards view
Refresh. For each board read columns, sprints, own issue list, and issue
keys of its active and future sprints only, then write all of it for that
board in one transaction with `boardrepo.ReplaceBoard`; board whose read
fail at any point recorded in summary `Dropped` with one-line reason from
`internal/errtext` (added during review: strip HTML tags and collapse
whitespace, since 403 answered with HTML login page hand transport a
kilobyte of markup) and left exactly as it was. Four read methods =
`ListBoards`, `ListBoardSprints`, `GetBoard`, `SyncBoards`, all in
`app_boards.go`. Phase 3a wrote nothing to Jira; Phase 3b, directly below,
add writes.

Two facts worth knowing before they cost you debugging session:

- **Incremental sync cannot backfill `status_id`.** Whole reason version 5
  migration clear sync watermark rather than just add column: without reset,
  every issue cached before this branch carry empty status id forever, since
  nothing would ever ask Jira for it again.
- **Board membership = whatever Jira board endpoint returned at sync time.**
  TAM not compute board membership from status; it cache key list
  `/board/{id}/issue` and `/board/{id}/sprint/{id}/issue` answered with. Card
  moved on web board move in TAM only after next sync, boards sync included.
- **Only active and future sprints have cards fetched.** Closed sprint stay
  in sprint list, for history, but membership never pulled, so sprint picker
  offer only active and future sprints and no others.

## Phase 3b: board writes

Card dropped or keyboard-moved on board make one of three moves, each own
journal entity beside `issue`, `issue_create`, `link`: `issue_transition`
(field `statusId`) for drag across columns, `issue_sprint` (field
`sprintId`) for move to another sprint or backlog, `issue_rank` (field
`rank`) for reorder within column. Three writes, packing, reverts =
`internal/issuerepo` `boardwrites.go`, `movevalue.go`, `movecolumns.go`;
`rebasemoves.go` = what Override do to held one. Nothing here talk to Jira
directly; journal = what Commit push, exactly as for field edit.

Transition journaled by target status id, never by transition id: which
transitions Jira offer depend on issue status at that exact moment, so id
read at drag time stale before Commit run.
`internal/backend/jira/transitions.go` `Transition` = where target status id
become real transition at push time: list issue transitions, pick one whose
`to.id` match journaled target (lowest transition id when two reach it, so
same drop resolve same way every run), and fill whatever that transition's
own screen require. Almost every Data Center workflow way into Done put
resolution on that screen; TAM read it from transition's own
`fields.resolution.allowedValues` and fill from profile
`transition_resolution` setting when transition allow that value, from
transition's first allowed value otherwise, and refuse (naming every
required field by name) when screen ask for anything else.

Rank never journaled as LexoRank: Jira own that value, and client-made one
would be second wrong source of truth next sync silently overwrite.
Journaled as neighbour, side, and board drop made on (`before|KEY|BOARD` or
`after|KEY|BOARD`), and commit pass re-derive neighbour from that board's
final local order at push time (`boardrepo.CellOrder`) rather than trust key
journaled at drop time, which busy board can easily have moved on from.
Board ride along because one key can sit on two boards whose orders
disagree, and nothing else say which board order rank measured against.
Every card but one at very top of board order anchor with `rankAfterIssue`
against card that landed above it; top card have nothing above it, so anchor
with `rankBeforeIssue` against card below instead. "X before Y" and "X after
W" place X in exactly same spot in Jira's one global rank, so two cards
ranked against each other can never disagree about pair, and only first card
of board's first non-empty column can ever take `before` branch.

Board write classified against issue remote status or sprint id at Commit,
never against its `updated` stamp: comment left on issue in Jira bump
`updated` without moving card, and read that as conflict would hold back
card nobody touched. Remote already at journaled target = satisfaction not
conflict, and row dropped as `Moved{Satisfied: true}` not raised as one;
remote at journaled before value pushed; anything else hold whole issue
back, both board rows together if both pending, so half an intent never
committed against card that is not where user left it.
`internal/committer/boards.go` (sprint + transition passes), `ranks.go` (rank
pass, last, since it is one write that can be redone harmlessly), and
`boardvalues.go` (three-way classification + labels conflict card print) =
board pass, which run after edits and before links, because transition on
draft must wait for create that give draft real key.

Edits phase's own `pushEdits` take only `EntityIssue` rows under a non-draft
key, never board rows: board row, `sprint_create` row, and link row each go
through its own phase (`boardRows`, sprint create, `commitLinks`), so
nothing routes a board move through `commitEdit`, which would send
`statusId` to Jira as ordinary field, fail on it, and fail issue's real
edits along with it. Override held board row not same operation as override
held edit: edit held on issue `updated` stamp, which `ResolveOverride`
rebase by writing new `base_version`, but board row held on its `before_val`
against remote status or sprint id, which have no base version to rewrite.
`RebaseMoves` = other half: rewrite held row `before_val` to what Jira hold
now and leave `after_val`, user move, untouched. Skip it and Override on
board conflict meet identical conflict on every following Commit; key with
only edits pending still override with no network call.

Journal row for board move deleted only while its `after_val` still the
value that was pushed (`MarkMoveCommitted`), because none of three move
bindings take TAM busy guard the way editing field do: card dragged again
while Commit mid-push update that row in place, and delete by row id would
throw away intent Jira never told about. Commit still audited, since push
did happen; newer intent stay in journal for next Commit to find.

Jira Agile bulk endpoints (`PUT /issue/rank`, `POST /sprint/{id}/issue`,
`POST /backlog/issue`) answer 207 Multi-Status when at least one issue in
request rejected and 204 when every one landed, so `core/jira/bulkwrite.go`
treat 207 as failure by HTTP status alone, never by trying to recognise body
schema: there is no successful Multi-Status to accommodate. Body decoded
only to say why, trying Atlassian documented `entries` array first, then
shapes `jiraErrorMessage` already know, then raw excerpt as last resort.

Keyboard path not convenience beside drag; it is accessible path screen
reader user and trackpad-averse user actually get feature through. Ctrl with
arrow move card that already hold focus (left and right transition column,
up and down rank within cell), refusing with announced sentence at either
edge, and every card "Move to" menu reach sprint move without pointer at
all. Drag draw two different cues for same reason two moves are different
writes: drop line at cursor inside card's own cell, exactly where rank will
land, and full-cell outline for drop on another column, never line there,
since transition land card by its own rank not at cursor.

## Phase 3c: the sprint lifecycle

Start sprint and complete one = only writes in TAM that reach Jira outside
Commit. Everything else in this app journaled and wait for user to push it;
these two do not, for reasons that do not apply to card move. Sprint start =
timestamped fact whole team read moment it happen, and Phase 4 burndown
computed from it, so journaling it would mean TAM decide when sprint started
and tell Jira an hour later. Completion = harder case: what happen to issues
that did not finish depend on sprint contents at exact moment it close, not
at whatever moment Commit next happen to run, and nothing to reconcile the
way held transition or rank can be, a sprint someone else already started
cannot be started again. `internal/sprints` own both ceremonies
(`Service.Start`, `Service.Complete`) so exception have one home and one
place to test; nothing in that package touch journal. Bound methods,
`StartSprint` and `CompleteSprint` in `app_sprints.go`, take same per-profile
lock (`a.acquire(p.ID, "sprint")`) sync, commit, boards refresh take, and
frontend reach them through `SyncContext.runSprintCeremony`, same reducer
path `runBoardsRefresh` use. Three sprint management writes in
`app_sprintmanage.go` take same Go lock but reach it through `runQuietLock`
instead, for reason sync section below give. Neither button have offline
state: TAM have no connectivity signal to disable one from, so both stay
enabled, call attempted, and transport failure or Jira refusal (second
active sprint, missing Manage Sprints permission) reported in dialog, which
stay open with what user typed still in it.

Completion move sprint's unfinished issues before it close sprint, never
after: reverse would leave closed sprint whose cards went nowhere, which
nobody can undo from TAM. "Unfinished" = one definition used everywhere it
matter: issue whose status id not in board's last `board_column`
`status_ids`, same mapping board itself draw with and same one `DonePoints`
count by. Sprint's own membership re-read from Jira through issue search
rather than from cache or from `BoardIssueKeys`, because cache can be
minutes stale and key list carry no status; search come back with both in one
paged call. Push itself move in chunks of twenty (`sprints.pushBatch`,
matching committer's own `sprintBatch`), because Jira bulk endpoints answer
partial refusal with 207 that name issues by numeric id, which cannot map
back to key, so whole batch fail together and smaller batch limit how much
of completion one refusal can take down.

Completion that reached Jira then failed reported inside `sprints.Completion`
(`Moved`, `MovedTo`, `Failed`, `Note`, `Message`) not as Go error, because
Wails discard bound method return value whenever method also return non-nil
error: dispatcher fill in either result or error, never both. Error would
therefore deliver sentence and drop counts and keys it is about, which is
what dialog need at that moment. Both failures travel that way, push that
stopped partway and close Jira refused once every card already moved, and
for second reason too: dialog render `Message` as outcome and Go error as
refusal, so refused close used to print its accurate sentence directly above
list still headed "47 cards are not finished and will move out of the
sprint" and footer still promising move. `CompleteSprint` return real error
only for refusals that happen before anything moves, and those go through
`internal/errtext` first, as do `Message` a failed push or refused close
carry: Jira words come straight off wire, and Data Center answering 403 with
HTML login page would otherwise put kilobyte of markup inline beside start
dialog buttons.

`Note` = opposite case: ceremony worked and bookkeeping after it did not.
`refreshSprints` answer with own failure now rather than only logging it,
because cached row still say `future` for sprint that is now running, so
toolbar offer Start for it and Jira answer that second start with 400. Both
ceremonies carry it back as one line telling user to press Refresh, beside
own success; `StartSprint` return it as string, and board ceremony banner =
where both read.

Completion refuse sprint cache call `future`, and only `future`. It move
cards out before it ask Jira to close sprint, so aimed at sprint that never
started it empty that sprint then fail close, and TAM can undo neither half.
Sprint started on web an hour ago still read as future in cache nobody
refreshed, and refusing that cost a Refresh where emptying it cost the
sprint, so no other state refused here.

Cards that move written back into both scopes of board cache, sprint they
left and destination they were sent to, second of which used to be missed:
banner said twelve cards moved to Sprint 15 and picker switched to Sprint
15, which drew exactly what it drew before, and nothing else would correct
it, since ceremony write no journal row for view to fold in. Empty
destination = board's own list, scope like any other.

Demo backend narrow its search to `sprint = N`. That = one scope it honour,
and not decoration: completion's own read is that query, and service keep
issue backend report no sprint for on grounds query already narrowed it.
Against backend that ignored scope, every card in project walked past that
guard, so completing sprint on demo profile moved whole backlog and reported
success.

Board read now run inside one deferred read transaction (`boardrepo.Board`,
`Order.CellOrder`, both through `Repository.inReadTx`), and every read issue
cache do on way, `IssuesByKeys`, `DraftIssues`, `PendingMoves`, take
`dbtx.Querier` that transaction opened rather than bare handle.
`ReplaceBoard` write board row, columns, sprints, membership together in one
transaction and was already correct; gap was on read side, where `Board` used
to issue four separate statements on handle and could land between two of
`ReplaceBoard` writes, drawing cards into columns already replaced or
membership list not yet written. That = flake two sessions chased before this
plan named it: reader on another connection see consistent snapshot only
inside transaction, and read spread across handle never had one. Sprint
lifecycle itself not write empty sprint list back over board it just acted
on: `sprints.Service.refreshSprints` refuse to persist what `BoardSprints`
answer with when answer empty, because single 400 on sprint endpoint
indistinguishable from "no sprints" the way `core/jira` map it, and ceremony
just proved board have at least one sprint. Overwriting cache with that empty
answer would delete every sprint row of board and drop sprint length start
dialog suggest from, with nothing reported anywhere since call itself did not
fail; `boardrepo.ReplaceSprints` do write once caller decided list is real.

Board multi-selection (`lib/boardSelection.ts`, `useBoardSelection`) = set of
issue keys, never set of positions, because board redraw on every refetch and
position-based selection would silently select whatever cards happen to land
in those slots afterwards, same class of bug 3b hit with keyboard focus. Also
different thing from detail panel's one selected card: panel selection stay
`.board-card-selected` and open on plain click or Enter; multi-selection
paint `.board-card-checked`, grow with control-click or shift-click range,
and more than one checked card close detail panel and replace it with
`BoardSelectionBar`, because panel describing one card while three checked
would lie about what next action touch. Its one action, "Move N cards",
journal through same `issuerepo.MoveManyToSprint` (`moveToSprintTx` shared
with single-card move) every other board write use, so take no guard, have
conflict story already reviewed in 3b, and need no new case in Discard,
pending dialog, or commit pass.

Detail panel Sprint field and start dialog suggested end date both read cache
not Jira: `SuggestSprintDates` combine `boardrepo.SprintLength` (median
whole-day length of board's last three closed sprints, zero when none exist)
with board cached sprint list, so dialog open with plausible date before any
network call and say plainly whether date came from history or from two-week
default.

## A sprint from anywhere an issue appears

Sprint list away from board = profile-wide. `boardrepo.OpenSprints` read
every active or future sprint across every board profile synced, each
carrying its board name, and detail panel offer that list in Backlog and
Epics tree. User in Backlog think about issue, not about board, so making
them pick board first to reach sprint would be app's own model leaking into
their task. Two boards can hold sprint of same name, which is why choice
carry board name when name alone would be guess.

Draft sprint pushed after its create, not sent with it. Sprint field missing
from most Data Center create screens, and create path already send none of
board state for that reason. `journalDraftSprint` run inside `Rekey`, in same
transaction that repoint draft's other rows, and write `issue_sprint` row
under real key with empty before value, so board pass of same Commit classify
it as push not conflict and land it right after create. One path that lose it
= `MarkCreatedWithoutRekey`, where Jira accepted create and local rename
failed: drop sprint same way it drop rank repointing, and say so in its audit
note.

Importer Sprint column matched by name against profile open sprints, case
insensitively. Empty cell = backlog; name that match nothing fail row and
list what was available; name two different sprints share refused not
guessed, and for that reason left out of template dropdown entirely. On row
that carry Key, cell ignored, and result now say how many rows that happened
to.

Writing draft sprint onto its row not change what board draw. `CreateDrafts`
write `sprint_id` and `sprint_name` onto draft row, same two columns
`moveDraft` already write when draft dragged into sprint, but `composeBoard`
append `DraftIssues` unfiltered, so draft already drawn in every board and
every sprint of profile, and `applyMoves` only drop card carrying pending
sprint-move journal row, which draft never have. What those two columns
change instead = Backlog sprint filter, Backlog grid and detail panel, and
`ListSprints`, whose `DISTINCT sprint_id` over cached rows can now surface
sprint id contributed only by draft.

## The Sprints view

Phase 3c gave TAM sprints on board: draw one, drag cards through it, start
it, close it. Could not make one, rename one, fix wrong date, delete one
created by mistake, or look at sprint contents without first choosing board
that happen to carry it. This view do those things, between Boards and
Reports in tab order, and it is always present: hiding it for project with no
scrum board synced would need cross-cutting navigation mechanism for one
consumer, and would show brand new profile nothing at all on first launch,
since condition that would hide it read cache that profile has not filled
yet. Project with no scrum board see empty state that say so and point at
Boards view instead.

**Edit and delete of sprint Jira hold reach Jira immediately; create does
not any more.** Reason for immediate edit + delete = cost not principle: TAM
journal is issue machinery (pending change keyed by issue key, conflict by
`updated` stamp), and sprint have no cached version to rebase edit on and no
conflict card. Create was same exception until bundle 01 paid its cost: plan
starting with new sprint could not be drafted offline. Now sprint create =
`sprint_create` journal row + draft row in `sprint` under negative id, and
Commit phase 1 create it. Full argument for edit + delete still on
`UpdateSprint` in `core/jira/sprintwrite.go`.

Fence structural rather than sentence in spec, because previous version of
this rule lived in one sentence in boards design and lasted one phase.
`internal/sprints/exceptions_test.go` assert, by name, that `sprints.Service`
exported method set is exactly `Complete`, `Delete`, `Edit`, `Start`;
growing it mean editing failing test whose message say what list is
for. Fence deliberately service's own methods and not `lifecycle` interface
ceremony use internally: that interface also carry `BoardSprints`, a read,
and `MoveIssuesToSprint`, whose other caller (multi-select move) journal it
like every other membership change, so asserting `lifecycle` as
immediate-write list would have been false day it was written. Membership
stay journaled everywhere in this view exactly as on board: detail panel
Sprint field and tree's own multi-select move both go through same journaled
`MoveManyToSprint` path board selection use, with same conflict story and
same Discard case, because reaching sprint without first picking its board is
whole reason this view exist, not reason to grow second write path.

Closed sprint carry no cached membership, by design not accident: boards sync
never fetch closed sprint issue keys, on reasoning that chart Phase 4 draw
from it should not depend on mostly-idle poll of history nobody asked for.
`SprintDetail.Issues` therefore empty for closed sprint for same reason it
would be empty right after version 5 migration and before next sync, and view
show closed sprint contents as unavailable, with reason, not as empty sprint,
which would be lie row cannot tell apart from truth.

**Delete span two repositories in two transactions, board rows first.**
`sprints.Service.Delete` call Jira, then `boardrepo.DeleteSprintEverywhere`
to remove sprint row and its membership from every board of profile that hold
copy (Jira hand same sprint to every board whose filter reach it, so delete
scoped to one board would leave second board copy in `OpenSprints`, still
offering sprint Jira already destroyed to New issue dialog, detail panel, and
importer Sprint column), then `issuerepo.ClearSprint` to blank sprint name
off issues that carried it. Two are separate transactions in separate
repositories on purpose: `dbtx.In` open its own transaction from handle, so
nesting one repository helper inside other's take second pooled connection,
block on first's write lock, and die on driver busy timeout, same failure
`MoveManyToSprint` already document. Shared transaction helper spanning both
= real work, recorded as deferred. `boardrepo` own comment on
`DeleteSprintEverywhere` carry rest of argument and what crash between two
transactions leave: board rows first mean crash leave issues whose
`sprint_id` and `sprint_name` still name sprint that is gone, stale text on
cards that already held it; other order would leave sprint alive in
`OpenSprints`, offering it as pickable destination everywhere issue Sprint
field appear. Narrow stale text beat dead sprint that can still be chosen,
and only full issue sync repair either.

**Scope `""` in `board_issue` = board's own list, not backlog.** It is every
issue on board, sprint issues included, which is what TAM's own code call
board's own list; rendering it as-is would list every sprint issues second
time and offer fill bar work already in sprint. So tree unassigned node,
`UnassignedSprintName` ("Board backlog", deliberately not "Unassigned": that
word already name assignee group two rows up in same tree), computed not
read: `boardrepo.sprintDetails` build it as board's own list minus every key
a sprint's journal-replayed scope hold, once journal replayed over every
sprint ahead of it in same pass.

**Delete confirmation issue count say "at least" for three different reasons,
never for one.** `notSynced` count keys sprint hold that issue cache do not,
so true count short by exactly them. `truncated` mean shared per-view card
budget stopped this sprint's own list short, so count cannot be checked
against what is on screen. Third = this view's own doing: pending sprint move
replayed over scope before total counted, so card journaled out of sprint but
not yet committed already left count while Jira still hold it, and reverse
hold too, card journaled in count here before Commit pushed it. Any one of
three turn sentence into "at least N issues" with line naming which; quoting
exact count that turn out low cost a sprint nobody can get back.

Schema version 7 add `goal` to `sprint` table, `RawSprint`, `backend.Sprint`,
`boardrepo.Sprint`. Not back-fill: unlike version 5 `status_id`, nothing here
clear sync watermark, because sprint not read by issue sync and its only
refresh = Boards view's own Refresh button. Sprint cached before version 7
keep empty goal until next boards refresh rewrite it. Clearing goal sent as
explicit empty string on edit and never on create, because partial-update
rule that stop empty box from wiping real goal on `UpdateSprint` is also what
make goal impossible to clear otherwise; `clearGoal` = how
`sprints.Service.Edit` tell the two apart.

Three management writes reach their lock same way two ceremonies do,
`a.acquire(p.ID, "sprint")` in Go, but frontend reach them through
`SyncContext.runQuietLock` not `runSprintCeremony`: that = exception the "One
lock, both ends" section already document, and what it cost written there,
not repeated here.

## The write path (plan 1b)

Edits and creates go through journal in `tam.db` (`pending_change` and
`audit_log`, shared DDL and helpers in `core/journal`). `issuerepo.EditField`
write row and journal change with row `updated` as base version;
`CreateDraft` insert `TAM-NEW-n` row with status `Draft` and create row
holding draft as JSON. Sync never delete draft and never overwrite column
with pending edit. `internal/committer` push journal: drafts first (POST,
then rekey), then per-issue version checks and PUTs; issue whose remote
`updated` moved held back with base, mine, remote per field, and user pick
Override (rebase, push next time) or Keep remote (drop edits, take Jira row).
Commit and sync exclude each other through `App.busy` and shared reducer
`committing` state.

Demo backend keep writes in memory, hand out keys from 500, and stage one
conflict: first Commit of edit to curated story (`<project>-412`) held back.
Editable fields = summary, description, priority, labels, story points,
assignee; drafts can be tasks, stories, bugs, requirements. Excel import and
cross-project links = plan 1c.

## The write features (plan 1c)

Import: Backlog Import button take CSV or XLSX (parsed by `core/importfile`,
XTM parser lifted out), map columns to nine draft fields
(`internal/importer`), validate rows with file row numbers, and create valid
rows as drafts in one transaction (`CreateDrafts`, audited "imported from
<file>").

Tenth column, Key, decide what row do. Empty, row create draft. Filled with
issue key or `.../browse/KEY` URL, it journal edits to that cached issue
instead (`EditFields`, one transaction, all or nothing), so sheet exported
from Jira round-trip rather than duplicate every row. On such row Type and
Sprint cells ignored (issue type not editable, and sprint is board write
EditFields cannot carry) and empty cell mean "leave this field alone", never
"clear it"; value that already match not journaled, so re-import unchanged
file leave nothing pending. Summary required only when no Key column mapped.
`SaveImportTemplate` write real workbook (`internal/importer/template.go`,
excelize): Issues sheet with ten columns, Type dropdown carrying profile's
own requirement type name, Sprint dropdown carrying profile open sprints,
five examples, and "How to use" sheet; naming file `.csv` in save dialog
write same columns as CSV. Assignee = Jira username, not display name.

Links: Links tab Add link form journal link (entity type `link`, field
`<type>|<direction>|<target>`); repository merge pending links into cached
detail; committer push link rows after edits with
`POST /rest/api/2/issueLink` and drop source detail cache. Requirements
creatable; demo ask for Source field on them and answer lookups for `XT-`
keys its curated details reference. Link removal, links in bulk sync, epics,
subtask parents not in scope.

## Phase 2: epics

Epics view group cache by parent: `issuerepo.EpicTree` read profile epics and
their children from `tam.db` in one call, in rank order, with per-epic
progress counts (done count, points, done points), truncating past 5,000-row
cap. `parentKey` (label "Epic") = seventh editable field, riding same edit,
journal, conflict, commit machinery as other six; Jira backend map it to
discovered Epic Link field. Creating epic default its Epic Name to summary
when draft leave field blank, so user never have to know Epic Name exist. Two
bound methods = `GetEpicTree` and `ListEpics`, both in `app_writes.go`. Tree's
own styling = XTM folder tree, reused class for class (`folder-tree`,
`folder-item`, `folder-caret`, rest) out of
`frontend/core/styles/primitives.css` rather than second tree style.

Incremental sync not remove epic deleted in Jira, so stale epic keep showing
its children in tree until full sync clear it.

CSV import epic rows must come before rows of any child that name them:
child row checked against epics file defined so far, in order rows appear,
not against whole file or cache.

## Navigation and the menu bar

View tabs under topbar = visible navigation, mirrored from XTM
(`references/xtm-main-layout.png`). View menu and optional rail reach same
places.

No view render title bar, way none of XTM's do: active tab already name view
and topbar profile select already name project, so heading repeating both
only cost content height. Each view name its own landmark (`aria-label` on
its section) rather than borrow id from heading that no longer exist, so
views still distinguishable to screen reader and to test.

Native menu bar carry same list, way XTM View menu do: `menuViews` in
`main.go` list views and each item emit `menu:view` with view id, which
`App.tsx` route on. That list must stay in step with `VIEWS` in
`frontend/src/nav.ts` by hand; native menu cannot read frontend's.

Left nav rail = second, optional way to reach same places, off by default and
toggled from View → Navigation Rail (Ctrl+B) or from own close button.
Preference = `show_nav_rail` in shared settings, whose zero value is hidden
default. Wails render checkbox tick from value item was built with, so
`App.refreshMenu` rebuild whole menu rather than mutate it, and `startup`
rebuild once more because `main()` build first menu before store exist.

Window icon and executable icon come from `build/windows/icon.ico`, not from
`build/appicon.png`: Wails only generate the `.ico` when missing. After
changing `appicon.png`, regenerate the `.ico` over same six sizes Wails use
(256, 128, 64, 48, 32, 16) and copy PNG to
`frontend/src/assets/images/appicon.png`, which is what About dialog show.

## Issue types and sub-tasks

Six logical types = task, epic, story, bug, requirement, sub-task. Epic,
story, bug map to fixed Jira names; requirement's = per-profile setting
`requirement_issue_type`; **task and sub-task levels discovered from
project**, because instance name them. `jira.Backend.resolveTypes` read
project issue types once, cache them, and take first with `subtask: true` as
sub-task level and first matching `taskAliases` ("task", "todo", "to do") as
task level. Jira defaults = "Sub-task" and "Task"; instance in field call
them "Technical task" and "Todo", and asking that instance for "Task" found
nothing. Level project not define dropped from sync scope rather than quoted
into it: Jira reject entire query that name issuetype instance lack, so
asking would fail sync instead of return nothing.

Sub-task chip show instance's own word, passed down as `TypeChip`
`subtaskLabel` by whoever hold profile rather than looked up in chip, so chip
stay pure render. Reader who see "Technical task" in Jira should not see
"Sub" here.

`parentKey` mean two different things by level. For sub-task it is Jira's own
`parent` field and ordinary issue, never epic or another sub-task, and cannot
be blank. For everything else it is Epic Link and must be epic.
`validateParent` and Jira create both branch on that.

Sub-task not in `CREATABLE`: cannot exist without parent, so only drafted
from issue it belong to, through detail panel "+ <sub-task type>" button,
which open create dialog with type locked and parent stated rather than
chosen.

## The detail sidebar

Panel = XTM's (`references/xtm-detail-sidebar.png`): dark instrument bar
carrying key and its status chip over padded scrolling body, then read-only
facts in 84px label grid, then collapsible sections under uppercase
headings. Used to hide Links, Tests, Activity behind tabs; XTM stack them so
reader scroll one column instead of hunting three. Fields = only section open
on mount, so panel still start short.

## The create dialog

Draft parent come from context. `parentKey` fixed and stated (sub-task
parent, from issue it was drafted from); `initialEpic` = default picker may
change, seeded from epic on screen: selected row when it is epic, else epic
it hang off. Starting at "(none)" made every draft begun with epic open an
orphan that had to be reparented afterwards. Fixed parent win over seeded
epic.

`NewIssueModal` draft one issue and say so: titled after type it is about to
create, and subtitle carry offline-first sentence at full strength, in slot
no error message can take (used to share footer with error, so explanation
vanished exactly when confused user needed it). Success call `announce()`
naming `TAM-NEW-n` key, since created row may be hidden by active filters.

Submit wait for `GetCreateFields`. Drafting before required-field list land
skipped every one of them and deferred failure to Jira 400 at Commit;
*failed* read is different and still let user draft, which is intended
degrade. List cached for ten minutes, so type toggle no longer re-flicker it.

`parentKey` = seventh draft field here as well as in detail panel, so story
can be born under its epic rather than reparented afterwards. Epic never
carry one, and switching type to epic drop it.

Epics view open dialog with `lockType`, which fix draft to `initialType` and
drop type select: "+ New epic" = statement not opening question, and dialog's
own title state type. Backlog "+ New" leave select in place, so everything
else drafted there.

Create fields come from `core/jira/createmeta.go` `CreateMeta`: per-type
endpoint `GET /rest/api/2/issue/createmeta/{project}/issuetypes/{typeId}`,
paged, classic expand call only on 404 (or when type has no id in project
list). Field absent from per-type answer = not on screen. `CreateFields`
return required and optional, never base field (`isBaseField`: summary,
description, priority, labels, assignee, reporter, parent, project,
issuetype, discovered Story Points / Epic Link / Epic Name / Sprint / Rank,
and greenhopper custom types by suffix), never optional field no text form
fill (`KindOther`). Value shape = `ShapeValue`: option id when Jira listed
values, `{"value"}` when not, comma list to array (`{"id"}`, `{"value"}`,
`{"name"}` by items), `{"name"}` for user, ISO day for date, midnight for
datetime. Dialog show required at once, optional behind **More fields (n)**;
`MetaField.tsx` `META_INPUTS` = per-type input table (bundle 06 replace
`textarea` entry). Draft carry `screenFields`, ids dialog offered;
`CreateIssue` `applyExtras` never let extra overwrite key already in payload
or base field, drop extra outside `screenFields` (nil = legacy draft, no
check), drop extra per-type metadata no longer list, log each drop. That =
fix for `parent: data was not an object` (duplicate Parent input) and
`customfield_10253 ... not on the appropriate screen` (classic answer
listing off-screen field). Sub-task drafted from draft parent allowed:
Commit create parent first.

## Draft sprints

Sprint created in TAM = draft. `issuerepo.CreateDraftSprint` write, in one
transaction, `sprint_create` journal row (key = negative id text, after_val
= `DraftSprint` JSON with board name) and `sprint` row with `draft = 1`,
state future (schema version 13 add `draft`). Id from profile setting
`draft_sprint_seq`, lowest of it and every negative id minus one: never
reused, so stale reference to discarded draft never attach to new one.
issuerepo write `sprint` table only for draft rows and `RekeySprint`, because
row must land and go with its journal row; every row Jira sent stay
boardrepo's, and `writeSprints` delete only `draft = 0`, so boards refresh
keep drafts. Every picker (`OpenSprints`, board sprint list, Sprints view,
detail panel Sprint field, New issue dialog, importer Sprint column) offer
draft, labelled "(draft)" / Draft chip. Card moved into draft = ordinary
`issue_sprint` row naming negative id. Start + Complete disabled with
"Commit this sprint first", and refused in Go (`errDraftSprint`) for negative
id; completion cannot move cards into draft. Edit + delete of draft local
(`EditDraftSprint` rename everywhere through `rewriteSprintID`;
`DiscardDraftSprint` = discard of `sprint_create` row: revert every move into
it, clear it off draft issues, drop row). Bindings keep `"sprint"` lock,
frontend keep `runQuietLock`. Rituals skip drafts. `RemoveBoards` unchanged:
draft on board that leave cache go with it, journal row stay in Pending
changes.

## Phased Commit

`committer.Commit` = `phases()` in order: sprints, epics, issues (task,
story, bug, requirement), sub-tasks, edits, board moves, links; journal
re-read (`commitRun.reload`) after each, so next phase see ids last one
rewrote. Inside create phase, drafts by `draftOrdinal` (n of TAM-NEW-n),
never string order (`TAM-NEW-10` used to go before `TAM-NEW-2`). Phase 1
`CreateSprint` then `RekeySprint`: draft row real, `rewriteSprintID` across
issue columns, both halves of `issue_sprint` rows, draft JSON; create row
gone. Epic/issue create `Rekey` now also `rewriteParentKey` in every other
draft JSON. Created sprint stay created if later phase fail
(`Result.CreatedSprints`). Refused create block its placeholder
(`dependencies.block`); any draft (parent or sprint), edit (after_val), board
row (key or move target), link (source or target) naming blocked placeholder
held (`Result.Held`, reason `waits for TAM-NEW-2, which Jira refused`, chain
`which is waiting for ...`), stay in journal, retried next Commit; held draft
block own key. Held != failure: nothing sent. `assertNoPlaceholders`
(`firewall.go`) walk every payload right before backend call: `TAM-NEW-`
string anywhere, or negative whole number under key naming sprint, fail that
one write with internal error, never 400 from Jira (label reading
`TAM-NEW-9` trip it too, accepted). Pending changes dialog show draft sprint
as own card first, held rows with Waiting chip + reason; banner count
"n waiting". Later bundle add commit step = one entry in `phases()`.
Demo: epic whose summary contain "refused" refused once per run; demo refuse
placeholder parent.

## The grids' columns

Every track in both grids = fixed width, including SUMMARY. A `1fr` summary
made whole table reflow moment row selected, because detail panel took 352px
out of pane: one time reader look closely at it = one time it moved. Rows are
`width: max-content;
min-width: 100%`, so they size to their tracks, still
fill wider pane, and leave slack at right instead of redistributing it. When
pane narrower than tracks, scroller take over; in Backlog that is
`.issue-body`, which hold header too, so sideways scroll carry column labels
with rows.

Table and detail panel meet on single 1px border with no gutter and no
rounded seam, way XTM's do, and panel dragged to width from grip on its left
edge (bounded 300-900px, remembered in WebView's own localStorage since it is
per-machine reading preference, not shared setting).

Detail panel title = one issue key that link out to instance
(`IssueKeyLink`, opened in user's own browser). Grid key cells = plain text
on purpose: row select on click, so link inside one would put two actions on
same pixels.

## People and priorities

Assignee field = picker not text box, because two halves of user never
matched: sync write *display name* into issue row, and write path send
`{"assignee": {"name": …}}`, so free text could only produce value Jira
accept when the two happened identical. `AssigneePicker` store username and
show display name.

`App.SearchUsers` ask Jira *assignable* endpoint (plain user search answer
with people who hold no permission on project, and picking one of those fail
at Commit), cache answer in `jira_user` in `tam.db`, and fall back to that
cache when Jira cannot be reached. Blank query go out as wildcard Jira DC
assignable search want, and is what seed cache. `CacheUsers` additive: narrow
search must not empty picker for next one, so rows go when profile purged,
not when search miss them.

Priority = instance's own list through `App.ListPriorities`. Both pickers
degrade to text input they replaced when lookup fail, same shape create
dialog use for failed create-meta read: lookup that cannot reach its list
must not be reason issue cannot be assigned.

## One lock, both ends

Go hold single per-profile lock (`App.acquire`) for sync, commit, import,
boards refresh, sprint operation and sprint report alike, so whichever start
second refused. Frontend model same invariant in shared sync reducer, and the
two must agree: Boards view Refresh used to be plain mutation outside
reducer, so shell stayed `idle`, kept offering Sync, and Go refused it with
"a sync is already running for this profile" on profile whose status still
read "not synced yet". Run through `SyncContext.runBoardsRefresh` now, which
take same lock sync do. **Anything new that call bound method taking
`acquire` must take frontend lock too.**

One deliberate exception, narrower than it look. `SyncContext.runQuietLock`
take same `statusRef` guard every other `run*` take, so sync, commit, boards
refresh and ceremony all still refuse against it, but it dispatch no progress
actions, so reducer stay `idle`. Create, edit, delete sprint go through it:
those are one short call each, and flashing whole app sync banner for rename
would say something untrue about what is happening.

What that cost worth knowing, because it is same shape as bug rule above was
written about. For length of call, shell Sync and Commit buttons stay enabled
and do nothing when pressed, and profile picker stay enabled, because all
three read `state.status` rather than ref. What keep user away from them =
dialog holding focus, which is why those dialogs refuse Escape while their
write in flight rather than merely disabling own buttons. Fourth quiet write
would have to earn same treatment.

**Sprint report not quiet write and not go through `runQuietLock`.**
`SyncContext.runReport` dispatch `SYNC_START` and `SYNC_END` way
`runBoardsRefresh` do, so shell genuinely enter running state and Sync,
Commit, profile picker all disabled. Three sprint management writes get away
with staying quiet only because each is one short call behind modal dialog
holding focus; report is minutes long, have no modal, and user look straight
at window, so leaving Sync enabled and inert would have been exact bug rule
above was written about. What status bar say while report run come from
report's own frames through `reportText.progressStage`, so it read "Reading
Sprint 10 for the velocity table" rather than borrow sync sentence. Reducer
internal field still called `syncing`; nothing render that word.

**`SyncContext` carry `running` value beside `statusRef`, and the two not
redundant.** `statusRef` is ref because it is synchronous guard: state update
would be render too late and two operations could both pass it. But it only
ever hold `idle`, `syncing` or `committing`, and every one of `runSync`,
`runBoardsRefresh`, `runSprintCeremony`, `runQuietLock` and `runReport` set
`syncing`, so frontend could not say what was actually holding its own lock.
Told user sync was running when holder was their own previous report, and
Reports view rendered whichever progress frame was in context as if it were
its own, so sync's issue counter appeared under report's copy. `running` hold
`acquire` names spelled Go's way, though only five this client can take:
`import` is Go's sixth and import dialog not run through this context, so
putting it in type would claim state that can never be set. Set and cleared
in same two places ref move, and is state rather than ref because view
re-render on it: refusal word itself from it, and Reports view show progress
frame only while `running` is `"report"`.

`SyncBoards` acquire under its own name, so refusal say which operation
actually running. Boards pass also given progress sink now: it walk every
board columns, sprints, and each sprint issue keys, which take minutes on
real project, and standalone Refresh used to pass `nil` and report nothing
anywhere.

`SyncIssues` and `SyncBoards` log on way in as well as out. Run that never
return used to leave no trace at all, so log could not say whether call had
even reached Go.

**Rituals Sync not quiet either.** `SyncContext.runRitualsSync` drive banner
like `runBoardsRefresh`, `running` = `"rituals"`: several requests, no modal
holding focus. Editor save, resolve, forget, delete take no lock at all.

## The boards pass and the board's shape

Pass = one request per board column config, per page of its sprint list, and
per page of issue keys of every scope it hold: board's own list plus each
active or future sprint. Two things keep that from dominating sync, both
measured against real instance where one board took 66 seconds on its own:

- **Agile page size** (`pageAgileIssues`, 500). At 50 a page, board's own
  issue list, which is every issue on board, cost one round trip per 50 keys.
  Both paging loops advance by what actually came back, so instance that
  clamp `maxResults` lower handled by same arithmetic.
- **Project narrowing.** Board filter not bounded by project: board above
  held 8,485 cards while project being synced had 38. Every scope read with
  `jql=project = "KEY"`, which Agile endpoints AND with board's own filter.
  Only that project issues ever in cache, so key outside it could not be
  drawn anyway.

Board that still take over five seconds name itself in log with its sprint
and card counts.

Third saving = **whose boards get synced at all**. Jira board list answer
with every board whose *filter* mention project, which include boards another
team own: 8,485-card board above belong to different project entirely.
`ownBoards` keep only boards whose own `location.projectKey` is this project,
count rest in summary `Foreign`, and name them in log. Board whose home
instance did not report is kept, so instance that send no location not lose
every board. Per-profile setting `boards_all_projects` keep them all, for
profile that genuinely work across programme board.

Columns = Jira's own: `BoardColumns` read board configuration and keep each
column status ids, and `placeCard` put card in column whose ids contain
card's `status_id` (schema 5 added that column; matching on status *name*
would break on any instance that rename one). Status no column collect
counted as `Unmapped` rather than drawn, which is what board unmapped line
report. Re-syncing board therefore pick up column added or renamed in Jira
with no further work.

## A drop asks for a column, not a status

Jira board column collect several statuses: Done column commonly hold
Resolved and Closed, and instance that migrated workflow hold old status
beside new one. Only one of them usually reachable from where card is now.

So push given whole column, not one id drop journaled.
`BoardOrder.ColumnStatuses` return every status sharing dropped-on status
column, that one first, and `Transition` walk them in order and fire first
the issue workflow offer. Keeping journaled status first mean reachable
target still preferred over its siblings, and refusal still name status user
actually dropped on.

Journal format untouched: still hold one `id|Name`, and resolution happen at
commit time, only moment workflow is knowable. `App.CanTransition` build same
candidate set, so drop optimistic check and push agree about what drop meant.

## The board grid

Header row anchor as **row** (`.board-columns-head`, sticky with own
background), not as individual sticky cells: sticking cells alone left 12px
gaps between them transparent and cards scrolled through header.

Toolbar filter = reading aid over drawn board, not query
(`lib/boardFilter.ts`): match card key, assignee, or issue type, and never
refetch, so clearing it cost nothing. Keep every column and lane so filter
never read as lost column, recompute column counts from what survive, and
zero overflow rather than guess at cards a cap left out. Board's own totals
(unmapped, notSynced, donePoints) describe board and left alone. `BoardBody`
take filtered view beside query result: query still say whether board loaded,
failed, or is empty.

Column header row and each lane row = separate elements, so they only line up
because both laid out on **one grid template**:
`repeat(var(--board-cols), minmax(--board-col-w, 1fr))`, with column count
set on `.board-scroll` from view. Flex row cannot do this, since each row
size its own items and lane holding long card grew wider than header above
it. Cells carry `min-width: 0` so wide card cannot push its track open.
`minmax` also what make columns share pane width and stop at floor, past
which `.board-scroll` scroll sideways.

## Layout and scrolling

Window never scroll as whole. `.app` = one flex chain down to panes, and only
three boxes scroll: Epics tree, Backlog table body, detail panel, each inside
own bounded card. `.main` is `overflow: hidden`, so every level between it
and scroller carry `min-height: 0` (flex child default to
`min-height: auto` and will not shrink below its content, so one missing
declaration let table push pager off bottom of window). Nothing size itself
off `calc(100vh - <a guess>)` any more; that was only ever right at one
window height. `references/xtm-main-layout.png` = reference for this
arrangement, and nav rail stay TAM's own in place of XTM view tabs.

Anything rendered directly in `.main` must say how it behave in bounded box,
which is why placeholder and startup error carry own
`min-height: 0; overflow: auto`.

Backlog pager pinned to floor of table card: rows per page, first / previous
/ typed page number / next / last, and range on right. Page box hold draft
string separate from page itself, with `null` meaning "not editing", because
binding it straight to `page + 1` made clearing it snap back to "1" and
typing "23" over it produce "123".

## The tables

Both grids size key column from keys on screen rather than from fixed pixel
track (`frontend/src/lib/keyColumn.ts`, handed to table as `--issue-key-w` /
`--epic-key-w`). Fixed track held about eleven characters and Jira DC allow
ten-character project key, so legal key overflowed: in Backlog it wrapped
inside 34px row and type chip painted over it, in Epics tree it ran straight
over summary. One width per table, not per row: each row is own grid
container, so `max-content` track would size every row to its own key and
columns would stop lining up. Every cell in both grids now clip with ellipsis
and carry `title`, so shortened value never read as complete one.

Backlog sortable. `IssueQuery` carry `sort` and `desc`, and
`issuerepo.sortColumns` map sort key to its SQL; name outside that whitelist
fall back to rank order, so nothing from frontend reach query. Sorting
server-side because grid paged: frontend hold 25 rows out of project
thousands. Header click go ascending, descending, then back to rank; drafts
stay pinned to top under every sort, and blanks sort last in both directions.
Grid state its order in line above rows, since rank order is point of backlog
and was otherwise invisible.

Epics tree row kinds all emit same seven cells now. Epic header used to fold
key and summary into one spanning cell while child split them, and with
status and points tracks sized `auto` in each row's own grid, column 5
started up to 110px apart between the two; those tracks fixed for that
reason, since grid cannot see its siblings without `subgrid`.

## Profiles

Manage Profiles = XTM dialog, feature for feature: profile list on left with
launch-default star, selected profile `ProfileForm` on right, Create and
Import above list, Export and Delete in open profile footer, and start state
that open on nothing so active connection never edited by accident. Markup
and styles = shared ones in `frontend/core/styles/primitives.css`, not second
copy.

Form drop fields that only mean something to Xray: Kiwi/Xray backend
selector, bug issue type, bug project, cross-project sources.
`App.UpdateProfile` read those off saved row and write them back unchanged,
so editing profile in TAM never reset what XTM configured on it. In their
place form carry TAM's own requirement issue type, which live in `tam.db` as
per-profile setting `requirement_issue_type` and so is loaded and saved
around profile write, not with it.

Export write same credential-free JSON shape XTM exporter do, so file from
either app import into other; imported profile have no token until one
entered. Kiwi profile file refused.

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
    app_sprintmanage.go  Create (a draft), Edit and Delete sprint (local for a draft), and
                          ListBoardSprintDetails for the Sprints view's tree, all under the
                          "sprint" lock name the ceremonies use
    app_reports.go       GetSprintReport, the one binding the Reports view calls, and
                          CancelSprintReport, which is how a view that has been left cancels a read
                          still holding the profile lock; both under the "report" lock name
    app_rituals.go       the ritual bindings: ensure, list, save, resolve, forget, delete, macro
                          preview, standup entry, last sync, Sync, all but Sync under no lock,
                          Sync under the "rituals" lock name
    internal/tamstore/   TAM's own SQLite file (schema version 13: issue (with status_id), issue_link,
                          sync_state, profile_setting, jira_user, board, board_column, board_issue,
                          sprint (with goal, added at version 7, and complete_date, added at
                          version 8, and draft, added at version 13), sprint_report (a sprint's
                          saved report, added at version 8), ritual_document (added at version 9,
                          with base_body, conflict_body and conflict_version added at version 12),
                          plus the shared journal tables pending_change and audit_log)
    internal/backend/    IssueBackend and BoardBackend seams and DTOs; backend/jira on core/jira,
                          backend/demo on internal/demo; core/jira/createmeta.go is the createmeta
                          reader and value shaper backend/jira builds the create dialog's fields from
    internal/demo/       the Acme Platform (PLAT) dataset behind a "demo" profile;
                          confluence.go is the in-memory Confluence space rituals sync against
                          on a demo profile, rebuilt from stored pages after a restart and
                          staging one conflict on the first Standup it creates
    internal/issuerepo/  the store layer: issue cache, detail cache, links, sync state, profile
                          settings, the pending-change journal, and drafts; tree.go groups the
                          cache into the Epics view's tree; boardwrites.go, movevalue.go,
                          movecolumns.go, and rebasemoves.go are the three board moves, their
                          before_val/after_val packing, and what Override does to a held one;
                          sprintdrafts.go is the draft sprint's create, edit, discard and
                          rewriteSprintID, and rekeysprint.go RekeySprint,
                          MarkSprintCreatedWithoutRekey and rewriteParentKey
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
                          the ceremonies, and Edit and Delete of a sprint Jira holds;
                          exceptions_test.go fences the package's exported method set to
                          exactly those four; draft.go is DraftSprint, the check a drafted
                          sprint gets, and errDraftSprint; guards.go is what a write refuses before it reaches
                          Jira, cache.go the board cache's bookkeeping after it has, manage.go
                          Edit and Delete themselves, and suggest.go the start and create
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
                          which six closed sprints a velocity table covers and what one row of
                          it says
    internal/sprintreport/  the orchestration around internal/reports, kept out of it so that
                          package stays pure: build.go is the one exported entry point, series.go
                          the store-or-fetch rule a closed sprint is served by, fetch.go the paged
                          changelog search with its progress frames, velocity.go the table
                          assembled from stored series plus whatever has to be read
    internal/dbtx/       the one transaction helper issuerepo and boardrepo share: In for a write,
                          InRead for a deferred read-only transaction, and the Querier interface a
                          read helper takes so it can run on the handle or inside either kind
    internal/committer/  pushes the journal to Jira in phases and resolves conflicts; phases.go is
                          the phase list, the draft sprint and draft issue creates, and the edits;
                          held.go is what a refused create blocks and the rows held for it;
                          firewall.go is assertNoPlaceholders; boards.go, ranks.go, and
                          boardvalues.go are the board pass, after the edits and before the links
    internal/importer/   maps import columns to draft fields and validates rows
    internal/syncer/     the paging engine; emits tam:sync-progress through app_issues.go; boards.go
                          is the boards pass, reached through backend.BoardBackend
    internal/errtext/    reduces an error to one readable line (strips HTML tags, collapses
                          whitespace) for sync summaries and dropped-board reasons
    internal/ritualtemplate/  renders a sprint's five ritual pages (pure, no clock, no I/O) and
                          reads a Jira Issues macro's JQL back (JQL, ParseJQL), the three
                          written forms and nothing else
    internal/ritualrepo/ the store layer over ritual_document; documents.go is the CRUD, the
                          dirty and status computation, and Apply{Created,Pulled,Pushed,Conflict},
                          MarkGone, the writes a Sync pass makes
    internal/ritualsync/ Ensure (write missing pages from templates, local, no lock) and Run
                          (the Sync pass under the "rituals" lock: title match, adopt, create,
                          pull, push, conflict, gone)
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
                          sprint management writes; runReport is the sprint report's path, which
                          does show the banner, and `running` is what lets a refusal name the
                          operation actually holding the lock
      src/lib/reportText.ts  every sentence the sprint report prints: the summary, the floor and
                          removal qualifications, the method line, the four unavailable reasons,
                          the unit and its reason, and the progress wording, all testable without
                          rendering anything and all reused by Phase 5's Rituals
      src/lib/boardCells.ts  the board's position arithmetic: keyboard focus and navigation
                          over the lane/column/index grid
      src/lib/cardMove.ts  the drag/keyboard arithmetic a board move shares: where a drop lands
                          in a cell, which column a key press steps to, whether either changes
                          anything; cardMoveState.ts folds a card's pending row, warnings, and
                          commit failures into one state; moveValue.ts reads a journaled board
                          value back for the Pending changes dialog and the Activity tab
      src/lib/storage/   converts a ritual page between storage XHTML and TipTap JSON and back;
                          parse.ts is the read side (BLOCK/INLINE element sets, the opaque-node
                          fallback for anything not modelled), serialize.ts the write side,
                          xml.ts the XML parse and the start-tag-only namespace stripping,
                          normalize.ts the byte-for-byte round-trip check
      src/lib/ritualText.ts  every sentence the Rituals view prints: chip and status labels, the
                          conflict and gone banners, the sync summary and pending line, the
                          unconfigured and no-scrum-board sentences
      src/lib/standupLog.ts  finds where today's dated Yesterday/Today/Blockers section belongs
                          in a Standup page's Daily log, and whether it is already there
      src/components/    BacklogView, IssueTable, IssueDetailPanel, EditableFields, ActivityTab,
                          AssigneePicker, PriorityPicker,
                          PendingChangesModal, ConflictCard, NewIssueModal,
                          MetaField (a create-meta field's input, by type, through META_INPUTS),
                          ProfilesModal,
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
                          multi-select, the board's under a different name), ReportsView (the two
                          pickers, the eight states a report can be in, and the rebuild),
                          SprintSummary (the sentence, its qualification, and the method line),
                          VelocityTable (the last six closed sprints, each row with its own unit),
                          RitualsView (the board and sprint pickers, the five-page nav, Sync
                          rituals and its result banner, the conflict and gone banners),
                          ritual-editor/RitualEditor (the TipTap editor over one page's storage
                          XHTML, its own save state), MacroPreview (a Jira Issues macro rendered
                          from cache), OpaqueViews (read-only rendering of an opaque node)
      wailsjs/           GENERATED bindings, do not hand-edit

## Commands

    wails dev                      # run with hot reload
    wails build                    # build/bin/task-activity-manager.exe
    go test ./internal/...         # Go tests
    cd frontend; npx vitest run    # frontend tests
    cd frontend; npm run build     # tsc + vite build

`npm install` run at repo root (npm workspaces). `frontend:install` in
wails.json do that for you.

## Conventions

Same as XTM's: logic in `internal/`, `app.go` only adapt it to Wails; Jira =
system of record; credentials go to OS credential manager only;
`TODO(tam): desc` mark planned work. TAM create Jira profiles, which core
store with backend `xray`; Kiwi profiles from XTM hidden. UI text use no em
dashes.

Sync scope = `project = KEY AND issuetype in (Task, Epic, Story, Bug, <requirement
type>)` plus profile scope JQL; incremental syncs add `updated >=` last sync
minus an hour. Requirement type name = per-profile setting
`requirement_issue_type`. Sprint and Epic Link field shapes marked
`NOTE(tam)` in `internal/backend/jira/fields.go` until verified on real
instance.