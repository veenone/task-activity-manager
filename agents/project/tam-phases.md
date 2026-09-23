# TAM feature history, phase by phase

Moved verbatim from tam/CLAUDE.md, which loaded this into every session at
15,765 words. Read it for work on a TAM feature; it is the design record of
how each phase was built and why. Rules extracted from it live in
AGENTS.project.md; this file is history and detail, not rules.


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
`sprint`. Charts those numbers would draw into came later, in their own change;
see the Phase 4 section below. Here ship figures, method printed under
them, plain statement of what
reconstruction cannot see.

## Rich text: descriptions, summaries, comments

The renderer lives in `@agile-suite/core`'s `richtext/`, not in TAM: a small
AST (`ast.ts`, one node per thing the renderer draws differently, nothing
either format merely happens to have), a hand-written Jira wiki parser
(`wiki.ts`, blocks in `parseBlockLines` then inline marks in `lineToInline`,
character scan only, escapes and all), Markdown on `marked`'s own lexer
(`markdown.ts`, `marked.lexer`, never `marked.parse`, so there is no HTML
renderer's output to sanitise), and per-field detection (`detect.ts`).
`RichText.tsx` walks the AST into React elements; `RichTextField.tsx` is the
Write/Preview field wherever a long text field is typed.

Jira's own `expand=renderedFields` was considered and dropped: it is
server-rendered HTML, exact for wiki markup and wrong for Markdown text,
needs its own sanitiser, and cannot render a draft or a pending edit that
has never touched Jira. One local parser instead covers drafts, offline
reading and both syntaxes with one code path, at the cost of writing the
parser.

**Detection scores distinct signal kinds, not occurrences**, so thirty `- `
bullets cannot outvote one `{code}`: each format's scanner records at most
one match per kind, keyed by the label of the line that found it, and
Markdown needs a strict majority of kinds to win; a tie, including 0-0,
reads as Jira markup, the safer default for a field that turns out to hold
neither. A line beginning `#` is the one character both formats claim as
their own marker (Jira's ordered list, Markdown's heading): `hashIsStacked`
in `detect.ts` reads it as Markdown only when neither neighbouring line also
starts with `#`, since two or more stacked `#` lines are a Jira numbered
list far more often than back-to-back headings, and both scanners call the
same function so one line can never register as both at once.

**Character scan only, with a fail-position cache so a wall of unclosed
delimiters cannot go quadratic.** `wiki.ts` never runs a nested-quantifier
regex over user text; every construct is found by scanning forward from the
current position. A bare `[`, `!`, `{{` or an unrecognised `{name:attrs}`
that never closes would otherwise search to the end of the string once per
occurrence one of many; `dbraceFailFrom`, `bracketFailFrom`,
`imageFailFrom`, and `BraceAttrsCache`'s `failFrom` each remember the
earliest position a scan is already known to fail from, so a field carrying
thousands of one bare delimiter still pays for one failed scan, not one per
character. `{code:...}`'s own close search needs no such cache: on failure
it advances straight to the end of the text, which ends the parse loop
outright.

**Three wiki delimiters are also ordinary punctuation, and prose wins each
time.** `!...!` is an image only when what it wraps names a file or an
address (no blanks, and either an extension or a scheme), so "Deploy
failed! Check the logs!" stays a sentence instead of a paragraph ending in
an image named " Check the logs", which is what it used to draw; the
paragraph that joins consecutive lines means the two marks need not even
share a line. `[...]` with no `|` becomes a link only when the inner text
is an allowed link, so `[WIP] rework the step` and `[~jdoe]` keep their
brackets rather than losing them to a link whose scheme the renderer then
refuses; with a `|` the right half is the address whatever it says. A bare
autolink gives back the trailing `.,;:!?` and any closing bracket it did
not open, so "See http://x.com/a. Then go" links the address and leaves the
full stop in the sentence.

**Entities decode in ordinary Markdown text only, never in a code span, a
code block, or the raw-HTML fallback.** This is CommonMark's own rule, not
a TAM invention: a character reference is prose that stands for a
character, while a code span is the author's own bytes. `marked`'s lexer
pre-escapes nothing, measured twice against `marked@18.0.13`: `a < b & c`
comes back exactly as typed, and `` `a<b &amp;` `` comes back as a
`codespan` still holding the five characters `&amp;`. So the decoding is
TAM's to do and TAM's to withhold, and `markdown.ts` calls its decode step
only from the `text`/`escape` case, which is what keeps a code sample's
literal `&amp;` from being corrupted into `&`. The plan's outside-voice
fix 5 assumed the opposite, that the lexer escaped every token's text; the
plan file records the correction.

**React elements only.** No `innerHTML`, no `dangerouslySetInnerHTML`, no
`<img>`, no `href`, anywhere under `richtext/`; `noHtml.test.ts` reads the
module sources as text and fails if any of the three spellings appears,
which is why this paragraph cannot even name them without tripping its own
guard in a differently-quoted way. A link renders as
`<button role="link">` with no `href`, guarded by `isAllowedLink` (moved
from the ritual editor's own `sanitizeHtml.ts` to core's `lib/links.ts`, one
definition of an allowed scheme shared by both) and calling `onOpenLink` on
click, never navigating the WebView; a refused scheme renders its label as
plain text instead of a button. An image macro is a named placeholder
(`Image: <file>`), never fetched or drawn. Issue keys are not an AST node:
`RichText` splits a text leaf on `\b<projectKey>-\d+\b` at render time, so
the parsers and `toPlainText` stay project-blind and the panel's own
`issue.key` is the only source of the project key a bare mention is matched
against.

**Comments ride `IssueDetail` in the detail cache, not a new table or a
separate fetch.** The detail cache already stores the whole `IssueDetail`
as JSON, so adding `Comments`, `CommentTotal` and `CommentsTruncated` to
that struct means comments are cached, read offline, and refreshed exactly
when the description already is, with no schema change. They are read
newest first, `GET /issue/{key}/comment?orderBy=-created`, paged by what
actually came back rather than by arithmetic on `startAt`, kept to the
newest 500. `CommentsTruncated` true with an empty `Comments` means the
paged read failed, never that the issue has none (an issue nobody has
commented on answers with a total of 0 and truncated false); a panel must
tell those two apart; the detail itself is not failed by a comment page
that failed, since Wails' either/or return means an error here would also
drop the description just read. A comment's `author` can be null (anonymous
or a deleted user), read as "Unknown user"; a comment carrying a
`visibility` restriction is marked with the restriction's `value` (a role
or group name), never labelled "role" or "group", since only the value is
known.

**The description is on the issue row, the links and the comments are in
the detail cache** (issue #63, schema 17). The sync asks Jira for
`description` with the rest of the row, so the panel reads
`issue.description` and draws it with no call of its own, online or off.
The links and the comments stay in the detail cache, because they are a
round trip per issue and have no business in a search payload, so
`GetIssueDetail` is still what the panel opens with, just not for the
description. `issue.description` is the one nullable column on `issue` and
`backend.Issue.Description` the one pointer: NULL is "no sync has carried a
description for this row", which is a different fact from an issue that has
none, and the panel says so in its own words rather than drawing a blank.
Migration 17 clears every sync watermark so the next sync fills in the rows
already cached, the way migrations 5 and 14 did for the status id and the
assignee name. An edit writes the column and `reapplyPending` puts it back
after every sync, so a pending local edit still wins; `WriteDetail` keeps no
second copy of the description in its JSON.

**The size cost of that was accepted rather than bounded.** A description
now rides in the search payload, in a column on every row, and across the
Wails bridge in every list the frontend reads. A page of 50 issues carrying
2KB descriptions is about 100KB on top of roughly 25KB of row fields, so a
2000-issue project pays a few megabytes on a full sync and the same again in
SQLite, once; incremental syncs pay it only for what changed. Some of that is
not new, because the description was already stored for every issue whose
panel had been opened, in `detail_json`, and that copy is gone. What is new
is paying for issues nobody opens, and the Epics tree, which carries up to
`treeCap` (5000) rows in one call for a panel that reads one of them.

Truncating the stored value was considered and refused: the editor would open
on a prefix, Save would journal that prefix as the whole field, and Commit
would delete the rest in Jira (I2). A "truncated" flag that disabled editing
would put back exactly the unknown/empty ambiguity the nullable column exists
to remove. The levers, if a payload ever justifies one, are `syncer.PageSize`
and `treeCap`, and dropping the column from the tree and board reads only:
the panel already falls back to the detail read's description when the row it
is given carries none, so those two views would go back to a round trip per
selection while the Backlog kept its local read.

**The raw string is always what is saved.** Nothing anywhere in this
feature converts wiki markup to Markdown or back; the journal, `EditField`,
`CreateDraft` and Commit all see exactly the characters typed, the same
guarantee every other TAM edit already gives.

**Offline reading is a fallback with a setting, not a special mode.**
`GetIssueDetail` in `app_issues.go` used to drop the cached detail whenever
the backend read failed, so a panel open past `detail_cache_minutes` with
no connection showed an error instead of the description, links and
comments it already had a moment before; it now serves the cached detail
with a logged line and a nil error whenever one exists (the description no
longer depends on any of this: it is on the row), and the panel
prints `cached <when>` beside the Comments heading so a stale offline read
never passes as current. `detail_cache_minutes` is a per-profile setting,
default 10, where 0 does not mean "always stale" but "never expires": a
profile set that way reads every detail out of the cache and needs no Jira
connection at all to open one. The one way past that, short of the window
elapsing, is the shell's own **Refresh** button beside Sync: it clears the
profile's cached details (`issuerepo.ClearDetails`, one statement) under
the same `"sync"` lock a sync or a commit takes, through
`SyncContext.runRefresh`, so it refuses exactly when those would and the
next read of whatever is on screen goes back to Jira. No section of the
detail panel carries a Refresh of its own: one reading through
`GetIssueDetail` would do nothing while the cached detail is fresh and
nothing at all at 0. The Retry offered after a failed read is a different
control, since a read that failed left nothing cached to serve.

**The summary renders inline code and links only, never marks**, and every
surface that shows a summary (grids, cards, the Epics tree, dialogs) shares
that one scope: Jira DC does not wiki- or Markdown-render the summary
field, so `2*3*4 items` must read as typed in the heading and everywhere
else, and only `{{code}}`/`` `code` `` and `[text|url]`/`[text](url)`
unwrap. `toPlainText`'s `scope` argument (`"full"` for grids, tree rows,
sort and search values; `"summary"` for the summary's own narrower rule)
is what the panel heading and every list share, the ritual editor's Jira
Issues macro preview included. `RichText`'s own `inline` mode keeps the
inline children of paragraphs and headings and drops every other block, so
a summary that parses as a list, a table or a rule would leave it with
nothing to draw; it falls back to the capped raw text there rather than
render an empty heading beside a grid row that shows the words.
`ConflictCard`, `PendingChangesModal`, and a journaled link's own stored
summary render raw text on purpose: they show the exact value being pushed
or compared, and
stripping markup there would make `*bold*` and `bold` look identical in the
one place that distinction matters most.

**The toggle's memory is a module-level `Map`, not state, and lives for the
app run.** `EditableFields.tsx` keys it `profileId:key:description`; picking
Markdown for one field on one issue is remembered the next time that same
field is opened, in the same TAM run, and forgotten on restart, which is
what the spec's "session" means.

Add to Layout: `frontend/core/src/richtext/` (`ast.ts`, `wiki.ts`,
`markdown.ts`, `detect.ts`, `plain.ts`, `RichText.tsx`, `RichTextField.tsx`,
which also exports `SyntaxToggle` and the `FORMAT_LABEL` every surface that
names a syntax reads) and `frontend/core/src/lib/links.ts` (`isAllowedLink`,
`compactUrl`, shared with the ritual editor's sanitizer). The one new TAM
component is `Comments` (in `IssueDetailPanel.tsx`), the read-only comment
thread with its own "Show all" and restriction chip.

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

**Charts came after, in their own change.** Phase 4 shipped the numbers and
left `Series.Days` undrawn, so the figures got trusted before anything drawn
from them. The charts landed later in `tam/frontend/src/components/charts`,
hand built in SVG the way XTM draws its Sankey, with the scale maths in
`lib/chartScale.ts`. Every chart states its numbers in text beside the marks
and ships a hidden data table, except where a visible table already carries
the same rows: the velocity chart drops its table beside `VelocityTable` so
a screen reader reads the figures once.

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
(`setEditable` on same instance, toolbar disabled not hidden, never a
rebuild): lock alone release before reload remount editor, and keystroke in
that gap would be saved against replaced version and refused. Open in
Confluence built only from http or https base URL.

**Editor toolbar = shared `EditorToolbar` in `@agile-suite/core`, and it know
nothing about TipTap.** Items = plain data (toggle, action, link: label, icon,
shortcut, active, disabled, what to run); `useRitualToolbar` in
`ritual-editor/` map TipTap state onto them through `useEditorState`, so
toolbar re-render on transaction and offer only what schema carry (Underline
shown because StarterKit 3 bring it and `lib/storage` write `<u>`). Keyboard =
WAI-ARIA toolbar: one tab stop, Left/Right with wrap, Home/End, tab stop kept
by item id not position. Item that cannot run here = `aria-disabled`, still
focusable; whole toolbar locked = native `disabled` on every button. **Locked
for Sync = disabled, not hidden**, so page not jump when Sync start; editor
itself still `setEditable(false)` on same instance. Read-only page still show
no toolbar. Link popover: Enter apply, Escape close and hand focus back to
link button, refusal (`LINK_REFUSED`, from `isAllowedLink` in
`lib/sanitizeHtml.ts`: absolute http, https, mailto only, scheme read after
dropping control characters) shown under box, never silent close. Button
mouse press `preventDefault`, or selection gone before command run.

**Root page gone = offer, not dead end.** `ritualsync.Run` read root first; 404
come back as `Result.RootMissing` (page id, space, `CanCreate`,
`SuggestedTitle` = `<project key> Rituals`), nil error, nothing written, no
sync time recorded. Any other root failure still Go error. `CanCreate` =
`confluence.SpaceProbe` answer (content listing then space `operations`),
unknown and transport with no probe both read as yes, create own 403 the
fallback. Rituals view open `RitualRootDialog`: placement stated first (top of
space, id saved to profile), Create page and sync, or forbidden sentence plus
Open Profile settings when probe say no. `App.CreateRitualRoot(profile, board,
title, adopt)` run under `"rituals"` lock, reached only through
`SyncContext.runRitualRoot`: `ritualsync.CreateRoot` call `CreatePage` with
empty parent (now omit `ancestors`), 403 = `forbidden`, 400 or 409 = look title
up, `titleTaken` with `TopLevel`, never read Confluence message; adopt (second
confirmation) take only page with no ancestors. On `created`/`adopted`
binding write new id onto stored Confluence config (`saveRitualRoot`, stored
row not stand-in demo config) then run same pass (`syncRitualsLocked`) and
return it in `RitualRootResult`; pass refusal = `SyncError`, not Go error,
since root already saved. Pages under vanished old root go Gone as before;
Recreate on next Sync put them under new root. Demo space asked for root id
`1` (`demo.StagedMissingRootID`) start without it, `DenyCreate` stage token
that cannot create. Profile form refuse non-numeric root id (except Confluence
URL `demo`) and read `pageId` out of pasted address (`lib/confluenceRoot.ts`).
Real-instance answers = `docs/superpowers/plans/assets/2026-09-15-confluence-root-page-probe.md`,
non-blocking.

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
- **Active and future sprints have cards fetched every pass; a closed sprint
  once.** This used to read "only active and future sprints have cards
  fetched", and issue #62 reversed it. A closed sprint's membership cannot
  change, so `readBoard` asks `boardrepo.SprintIssues` for it first and
  re-supplies what the cache holds as this pass's answer, which is what
  carries it through `ReplaceBoard`'s wholesale delete of the board's rows.
  Only a closed sprint that has never been asked costs a request, at most
  `closedMembershipBudget` (12) of them per pass, walked newest first by
  start date so a board backfills from the top of the Sprints view downward.
  "Has been asked" is `sprint.membership_synced`, not the presence of
  `board_issue` rows; a read that fails leaves it unset and is retried next
  pass, and it does not drop the board.
  The sprint picker is unchanged and still offers active and future sprints
  only: what a sprint holds and what a card can be moved into are different
  questions.

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
(`Moved`, `MovedTo`, `Note`, `Message`) not as Go error, because
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

## The sprint row

Issue #62 redesigned it. Six cells: the caret, the sprint's name over its
goal, the state badge over the sprint's dates, the timeline and its bars, the
scope, and the actions menu.

- **The badge shouts in CSS.** `StatusBadge` in `@agile-suite/core` takes a
  tone and a mixed-case label; the capitals are `text-transform: uppercase`
  in `primitives.css`, the way `.chip-conflict` already does it. The same
  string is the row's `aria-label`, so an uppercased one would make a screen
  reader spell the state out and would need a second field to keep the
  accessible name readable. The badge also carries a glyph per tone, so the
  four states are never told apart by colour alone.
- **The row's name carries what the row draws.** A `treeitem` with an
  explicit `aria-label` is announced by that name and nothing else, so a
  reader arrowing the tree never reaches the cells inside it. The name is
  the sprint, its state, the timing label, the trailing note and what it
  holds: "Sprint 10, Closed, Closed 12 Sep, 15 days, cards not read yet".
  `sprintTimelineText` builds the second half and the cell renders the same
  pieces, so the two cannot drift.
- **There is no separate scope cell.** One existed and printed "14 cards"
  and "8 done" an inch from the timeline's "8 of 14 done, 21 of 34 pts".
  Its second line called `total - done` "carried over" while `done` comes
  from each card's status *today*, so work that was unfinished at close and
  finished afterwards, which is the normal life of carried-over work,
  counted as done and the figure read 0 for almost every closed sprint. The
  close-time answer is the sprint report's, built from the sprint's own
  history. The timeline's line carries the count for a row with no calendar
  too, which is how the board backlog still says what it holds.
- **Two bars, each naming itself.** `ProgressBar` is a track, a fill and an
  optional marker. The time bar is named "Time elapsed in <sprint>" and its
  `aria-valuetext` is the relative label, "Day 6 of 15" or "Closed 11 Sep";
  it carries a today marker on a running sprint only, since a finished one
  has no today inside it, and it is greyed for a closed sprint. The points
  bar is named "Points done in <sprint>", counts `donePoints` of `points`,
  and reads "0 of 21 pts planned" on a future sprint. Neither takes a colour:
  the twelve `--state-*` and `--bar-*` tokens in `tokens.css` are aliases
  over the chip palette #61 measured, except `--today-marker`.
- **The goal is on the row now.** It was kept off it because every cell clips
  and the detail panel narrows the pane; the name column has the widest track
  and the goal clips with a `title` the way the name already does. It is
  still drawn under an expanded sprint, because that copy handles the sprint
  with no goal at all and the row does not. A test querying the goal after a
  sprint is expanded finds it twice, so scope with `within`.
- **`sprintRelative` is the one place the relative wording lives**
  (`lib/format.ts`), including the Sprints view's own summary line.
  `dayOfSprint` was a second formatter for the same fact and is gone; two of
  them is how the row and that line came to print different lengths for one
  sprint. It takes `now` as a defaulted parameter, the way
  `formatWhen` and `dayOfSprint` do, so no clock is threaded through the
  tree; component tests pin it with `vi.useFakeTimers({ toFake: ["Date"] })`,
  and `toFake: ["Date"]` is not decoration, faking every timer breaks
  `userEvent`.
- **Below 900px the timeline stacks under the sprint name**, in the media
  block #61 opened for the report charts rather than a second one at the same
  width.

Two `lib/format.ts` bugs the row would have drawn, both fixed with it and
both the kind that come back:

- **`dayOfSprint` counted its length exclusively.** It measured the bare
  difference between the two dates, so a 12 Sep to 25 Sep sprint read "day 13
  of 13" on the day it ended, could not express a one-day sprint at all, and
  disagreed with the row on the same screen, since the Sprints view prints
  `dayOfSprint` in its own summary line. It counts both end days now.
- **A future sprint nobody started said "Starts today" for ever.** A sprint
  stays future until somebody starts it, so its planned start slides into
  the past and stays there. It reads "3 days late", the mirror of the
  "2 days over" the active branch already got right.
- **`day()` moved a sprint by a day.** It put Jira's stamp through
  `new Date()` and `toLocaleDateString`, which is the bug `dayInput`'s own
  comment warns about: a sprint starting at 09:00 UTC read as the day before
  for a reader west of it. It reads the leading date and renders it through
  the `calendarDay` reader #61 added for the report's bare days. One function
  in that file turns a date into a `Date` afterwards, `civilDay`, and
  everything counted in days is counted from those rather than from the
  instants Jira sent. The countdown to a future sprint was measured in
  instants and its own boundary test caught it.

**`civilDay` reads the literal date off the stamp, and that is a decision
nobody has taken yet.** It takes the `YYYY-MM-DD` prefix, so a sprint Jira
stamps `2026-09-12T23:00:00.000+0000` renders as 12 Sep even for a team in
UTC+2, for whom that instant is 13 Sep and for whom Jira's own UI says 13
Sep. The prefix reading predates this work: `dayInput` introduced it so an
edit dialog nobody touched would stop moving a sprint by a day, and reading
the day as an instant is a bug in the other direction. Which of the two is
right depends on whose calendar is authoritative, and the honest answer is
the Jira instance's zone, which TAM does not know and does not ask for.
Guessing moves every sprint date by a day for the teams the current reading
serves, so this needs a decision and a place to keep the instance's zone
before it needs code. The sprint row widened its blast radius from one
dialog to every row, which is why it is written down here.

**Clock-pinned fixtures are built with the local-time `Date` constructor.**
A UTC instant is a different calendar day either side of about eleven hours
from UTC, and these rows count civil days, so a runner far enough out read
them a day wrong. `tam/frontend/vite.config.ts` also pins `TZ` to UTC, but
that is the second line of defence and not the first: Node on Windows
ignores `TZ` entirely, so `TZ=Asia/Tokyo npx vitest` proves nothing there.
`SprintTimeline.test.tsx` renders the same row one second after local
midnight and one second before the next and asserts they read the same,
which reproduces the zone bug without needing a zone.

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

**Every sprint write is journaled now.** Phase 3c made create, edit, delete,
start and complete reach Jira at once, for cost reasons: TAM journal was
issue machinery (pending change keyed by issue key, conflict by `updated`
stamp), and sprint have no cached version to rebase edit on and no conflict
card. Bundle 01 journaled create (`sprint_create` + draft row under negative
id), bundle 04 Task 7 edit and delete, Task 8 start and complete (see Sprint
writes, journaled). The fence test that named the writes reaching Jira at
once went with the last two: an empty
list is the point, and a fence guarding nothing is worse than none. Membership
stay journaled everywhere in this view exactly as on board: detail panel
Sprint field and tree's own multi-select move both go through same journaled
`MoveManyToSprint` path board selection use, with same conflict story and
same Discard case, because reaching sprint without first picking its board is
whole reason this view exist, not reason to grow second write path.

**Closed sprint membership is read once and kept.** This entry used to say
the opposite, and #62 reversed it. The old reasoning was that a chart drawing
from closed sprints should not depend on a mostly-idle poll of history nobody
asked for, and that reasoning was sound about a *poll*. It is wrong about a
*row*: the redesigned row promises a progress bar, most rows in any real
Sprints view are closed, and a bar drawn from structurally zero numbers is a
lie the row cannot tell apart from an empty sprint. The reversal is narrow.
A closed sprint's membership cannot change, so it is read once and then
re-supplied from the cache for ever after, which is not a poll.

A lazy fetch on expand was considered and rejected: `ReplaceBoard` deletes
every `board_issue` row of a board before it writes what the pass read, so a
scope written through `ReplaceSprintIssues` outside the pass would be wiped by
the next boards sync and re-fetched after every one. The lazy option therefore
cost a new binding, a second holder of the per-profile lock for a read, a
frontend query with its own loading, error and offline states, *and* a change
to that delete, for a worse bound.

**The bit lives on the sprint row, because `board_issue` cannot carry it.**
The first version of this counted a closed sprint as read when `board_issue`
held a row for it, and deferred a `membership_synced` column as a migration
for one bit, to be spent if the case ever showed up. It shows up on the first
board whose filter spans more than one project, which is the case `ownBoards`
exists for and the one it documents with 8,485 cards against the project's
38: most of that board's closed sprints hold nothing of *this* project, so
they answer empty, write no rows, and look unread for ever. They spent a unit
of every pass and the older sprints behind them were never reached at all.
Every way of avoiding the column either re-asks each empty sprint once per
rotation for ever or keeps the same bit somewhere worse than a column, so
schema version 17 spends it.

The flag only ever goes from unread to read. `writeSprints` carries it across
the delete-and-reinsert both `ReplaceBoard` and `ReplaceSprints` do, so
starting one sprint does not cost a board its backfill, and only a purge or
the sprint leaving Jira's list resets it.

**A failed historical read is skipped, not fatal.** The backfill is
best-effort work about sprints nobody is waiting on. Returning its error
dropped the whole board, so one 403 on a sprint from 2023 cost the running
sprint its refresh on every pass, silently and for ever (I2). The sprint is
logged, left unread and tried again next pass; the active and future loop
keeps its hard failure, because that work is what the view is for.

**Changing a profile's project key purges the board tables too.**
`UpdateProfile` purged issuerepo's half only, which was survivable while
every boards sync deleted and rewrote a board's membership on the way past.
Once the sync started reading that membership back and treating it as an
answer, the old project's keys were re-supplied for ever and the sprint
reported itself read while holding another project's cards.

`MembershipCached` keeps its name, its type and its place on the wire, and
changes meaning for a closed sprint only: from "the sync tries to fetch this
scope" to "this scope has been read and kept". Active and future sprints keep
the derivation from state, because the sync asks for them on every pass, so an
empty scope there is as likely to be a sync that failed partway through. Until
a closed sprint has been read the row says "cards not read yet" and draws no
points bar. A closed sprint that *was* read and holds nothing of this project
reports itself read and counts 0, which is the distinction the flag buys.

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
`frontend/src/nav.ts` by hand; native menu cannot read frontend's. Bundle 03
add Assigned to me between Backlog and Epics in both lists, which is why its
accelerator is Ctrl+2 and every view after it shifted one key down.

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

**New issue dialog offer project's own types (issue #65).** Six logical types
stay: grid chip, filter bar, epic tree, sync scope all still key off them.
What change is the dialog's list. Sync call `IssueBackend.IssueTypes` and
leave answer in `profile_setting` key `project_types`, JSON of
`backend.IssueType` where each carry project's own `name`, Jira's `subtask`
flag, and `logical`, which `jira.logicalType` fill in and which is `""` for
type TAM have no concept of. `App.ListProjectTypes` read that setting and
nothing else: dialog must not reach Jira when user press New, and instance
behind #65 answer per-type create-meta with error anyway (#51). Setting not
new table on purpose: `profile_setting` already profile-keyed and already in
both `PurgeProfile` list, so no migration and no new name to keep in step.

Dialog list project's non-sub-task types under project's own names, value =
`logical` when there is one else the Jira name. So project that call task
level "Todo" show "Todo" and still draft `task`; "Improvement", which map to
nothing, draft as `"Improvement"`. `jira.typeNameIn` is where draft type
become Jira name: TAM's six through `jiraTypeNames`, else project's own name
verbatim **only when project type list carry it**, else error naming the
type. Type store never heard of must not become task. `TypeChip` draw such
type with its own name on neutral `chip-type-none`, because
`chip-type-Improvement` is class no stylesheet define. Profile that never
sync have nothing recorded: dialog fall back to TAM's five creatable types
and say so under the select, so create still work.

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
return `CreateFieldSet`: fields plus `ScreenKnown` (= answer came from
per-type endpoint). Never base field (`isBaseField`: summary, description,
priority, labels, assignee, reporter, parent, project, issuetype, discovered
Story Points / Epic Link / Epic Name / Sprint / Rank, greenhopper custom
types by suffix, and any field whose `schema.system` = `parent`), never
optional field no text form fill (`KindOther`). Value shape = `ShapeValue`:
option id when Jira listed values, `{"value"}` when not, comma list to array
(`{"id"}`, `{"value"}`, `{"name"}` by items), `{"name"}` for user, ISO day
for date, midnight for datetime. Dialog show required at once, optional
behind **More fields (n)**; `MetaField.tsx` `META_INPUTS` = per-type input
table (bundle 06 replace `textarea` entry). Draft carry `screenFields`, ids
dialog offered; `CreateIssue` `applyExtras` never let extra overwrite key
already in payload or base field, drop extra outside `screenFields` (nil =
legacy draft, no check), drop extra per-type metadata no longer list, log
each drop.

**A classic answer is not the screen.** It list field the screen do not
carry on some DC version, so `screenFields` built from one cannot catch
anything: dialog build that list from the same answer. So when
`Source == MetaClassic`, `CreateFields` offer only what Jira mark required
and `applyExtras` send only those, logging each drop; **More fields** then
short or empty, and dialog say so ("This Jira version does not report which
fields are on the create screen, so only required ones are offered.").
Required still offered and sent: leaving one out only move failure to Jira
400. Per-type path unchanged, and must not narrow.

**An unreadable answer is not a licence to send.** `metaErr != nil` used to
skip both source checks, so every extra went as text and Jira refused the
whole create; that was the path a real instance hit. Now nothing confirmed =
not sent. A draft's own `screenFields` do not override it: they record a
screen read from an earlier session, and a create refused today is exactly
where that reading went stale. Create still go through: one unconfirmable
field is not reason to refuse an issue. What it leave out come back from
`CreateIssue` as second result, named for a person, ride `committer.Created.LeftOut`,
and `CommitBanner` say which field the issue was created without. Dialog say
the matching thing when read fail ("Jira's create fields could not be read
(reason). TAM cannot offer the extra fields this issue type has, so a create
will carry only the fields above."), which is different state from unknown
screen line and stay apart from it.

**Field error get named.** `core/jira` `writeStatusError` return `*WriteError`
carrying `Messages` and `Fields` (Jira's errors map, by field id) beside the
flattened `Message`; `Client.FieldName` answer id to name off the same cached
`/rest/api/2/field` list `CustomFieldID` load. `backend/jira`
`humanizeFieldError` (create and edit) rewrite a refusal that name field into
one that name them, drop the REST prefix (this string is what commit failure
line show verbatim), name every field in the map, keep `errorMessages` as
they are, leave a fieldless error alone, and fall back to the bare id rather
than invent a name. Create add one sentence saying why TAM sent the field.

That = fix for `customfield_10253 ... not on the appropriate screen`.
`parent: data was not an object` was the duplicate Parent input, fixed in
1cbc847 by `baseFieldIDs` carrying `parent`; note Jira key its `errors` map
by **field id**, so a `parent:` key mean the payload key was literally
`parent`, which no build since that commit can send. A parent reported under
a custom id is a different failure, named by that id, and `applyExtras`
filtering with `isBaseField` (not `isBaseFieldID`) is what stop it.
Sub-task drafted from draft parent allowed:
Commit create parent first. XTM stays on its own reader for now
(`xtm/internal/jira.GetBugCreateFields`); moving it onto
`core/jira/createmeta.go` is a separate change.

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
frontend keep `runQuietLock`. Edit + delete of a real sprint are journaled
too; see Sprint writes, journaled. Rituals skip drafts. `RemoveBoards` unchanged:
draft on board that leave cache go with it, journal row stay in Pending
changes.

## Sprint writes, journaled

Bundle 04 Task 7. Edit and delete of a sprint Jira holds (positive id) used to
reach Jira at once through `sprints.Service`. Now journal rows, pushed on
Commit, and both bindings work offline.

- **Entities** (`issuerepo/sprintwrites.go`): `sprint_edit` and
  `sprint_delete`, key = sprint id text, one fixed field each (`edit`,
  `delete`), so a second edit replaces the first and keeps its before value.
  `sprint_edit` after_val = `SprintEdit{boardId, name, goal, startDate,
  endDate, clearGoal}`, before_val = cached `{name, goal, startDate,
  endDate}`. `sprint_delete` after_val = `{boardId, name}`, before_val = name.
- **Local effect.** `JournalSprintEdit` updates the cached `sprint` row on
  every board holding it and renames the cards (`rewriteSprintID`), in the
  same transaction. Goal follows Jira's partial update: cleared with
  `clearGoal`, kept when the new goal is empty, else replaced. A later edit
  with an empty goal keeps an earlier `clearGoal`, because the dialog reopens
  on the cleared goal and cannot ask again. `JournalSprintDelete` changes
  nothing locally: Jira holds the sprint until Commit.
- **Local refusals** (nothing journaled): edit of a closed sprint, of one the
  cache does not hold, or of one with a pending delete; delete of anything
  but a cached future sprint, or while cards in it have pending changes
  (`sprints.Service.CheckDelete`, which the push runs too).
- **Delete supersedes edit.** A pending edit of the sprint is reverted and
  dropped (through `discardOne`) in the delete's transaction, so the delete
  row names the sprint as Jira has it.
- **Discard.** `discardOne` has a case for each: an edit restores the row and
  card names from before_val, a delete just drops its row.
- **Push.** Commit's `sprint changes` phase (its own phase after
  `sprints`, so it reads the ids the creates rewrote) runs
  `pushSprintWrites`: edits, then starts, then completions, then deletes,
  oldest first within a kind (`sprintWriteOrder`), through `Engine.Sprints`
  (`committer.SprintWriter`), which `app_writes.go` wires to
  `sprints.ForCommit(a.sprintService(p, b))`. Every write to Jira in
  `internal/sprints` hangs off `sprints.Committed`; `Service` exports none.
  The push keeps the guards (`requireEditable`, `requireDeletable`,
  `CheckDelete`, `CheckComplete`, `requireCompletable`), the audit
  row and the cache work. A guard refusal wraps `sprints.ErrRefused` and
  becomes a non-retryable `Failure` naming the row; any other error is
  retryable; both keep the row. A nil seam fails each row with "this
  connection cannot manage sprints". A pushed write is listed in
  `Result.SprintsChanged` ("Sprint 12 edited"); a note is logged.
- **Frontend.** `groupPending` keys groups by kind and key (`sprint:12`,
  `board:-1`), so a draft sprint and a draft board with the same negative id
  get separate cards; `PendingGroup.id` is that identity, `key` the entity
  key. Pending changes reads an edit as "Edit sprint <name>: <what changed>"
  and a delete as "Delete sprint <name>", each with Discard; a draft board
  as "New board <name>". The Sprints view marks a sprint with a pending
  delete "Deleting on Commit". Known edge: a boards refresh replaces the
  edited sprint row with Jira's until Commit (TODOS.md).

Bundle 04 Task 8 did the same for start and complete, the last two writes
that reached Jira at once.

- **Entities** (`issuerepo/sprintceremonies.go`): `sprint_start` (field
  `start`, after_val `SprintStart{boardId, name, goal, startDate,
  endDate}`, dates already in Agile format) and `sprint_complete` (field
  `complete`, after_val `SprintComplete{boardId, name, moveTo, moveToName}`,
  `moveTo` empty for the backlog). No before_val. Neither
  changes the cache until Commit; the Sprints view shows "Starting on
  Commit" / "Completing on Commit" through the chip the delete uses
  (`sprintWaiting` in `queries/pending.ts`).
- **Local refusals.** `JournalSprintStart`: a real sprint unless cached
  `future`, or while its delete waits. `CompleteSprint` (the binding) runs
  `sprints.Service.CheckComplete`, the check the push runs too: a draft, a
  cached `future` sprint, a draft or self destination, pending changes on
  cards staying in it;
  `JournalSprintComplete` then refuses a sprint the cache does not hold and
  one whose delete waits. `JournalSprintDelete` now also refuses while a
  start or a completion waits (`refuseQueued`).
- **Draft sprints can be started.** `sprint_start` on a negative id takes
  the draft's own board, which is always real. `RekeySprint` moves the row
  to the real id (`rekeySprintStart`); `discardDraftSprint` drops it. In the push, a
  start still keyed negative is held (`deps.blockedBy` / `hold`) when its
  create was blocked, and fails for good when the create row is gone.
  Start is offered on a draft in the Sprints menu and the Boards toolbar;
  Complete stays held back.
- **Complete recomputes.** The row stores intent only. At push `Committed.Complete` reads the sprint from
  Jira, moves what is unfinished by the board's last column, closes it, and
  `SprintsChanged` reports the real count ("Sprint 12 completed, 45
  unfinished cards moved to Sprint 13"). A completion that moved cards and
  stopped (a failed chunk, a refused close) comes back with a `Message`
  naming the moved keys; the committer turns that into a retryable failure
  and keeps the row. A retry moves whatever is still unfinished and closes;
  a sprint the refreshed cache holds as `closed` is treated as done and
  moves nothing.
- **Bindings.** `StartSprint` and `CompleteSprint(profileID, boardID,
  sprintID, moveTo)` return only an error (as do `EditSprint` and
  `DeleteSprint`), need no backend, and take
  the `"sprint"` lock; the frontend calls both through `runQuietLock`
  (`runSprintCeremony` and `ImmediateWriteChip` are gone).
- **Dialogs.** Start says "Saved locally. Commit starts the sprint in
  Jira." Complete lists the cached unfinished cards and says "About N cards
  are not finished. The exact set is worked out again on Commit, and the
  Commit result reports how many moved."; it never promises an exact count.
- **Release note.** Starting and completing a sprint now wait for Commit,
  like every other change in TAM. The sprint stays future or active on
  screen, marked Starting or Completing on Commit, until you commit; a
  completion moves the cards that are unfinished when Commit runs, which
  can differ from the number the dialog showed, and the Commit result says
  how many moved.

## Phased Commit

`committer.Commit` = `phases()` in order: boards, board adds, sprints, sprint changes, epics, issues
(task, story, bug, requirement), sub-tasks, edits, board moves, links;
journal re-read (`commitRun.reload`) after each, so next phase see ids last
one rewrote. Inside create phase, drafts by `draftOrdinal` (n of TAM-NEW-n),
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
(`firewall.go`) check, right before backend call, only reference values each
call site hand it (parentKey, sprintId, issue key, link from/to key, rank
neighbour), never free text: `TAM-NEW-` string, or negative whole number under
key naming sprint, fail that one write with internal error, never 400 from
Jira. New phase must pass references only, not whole payload. Pending changes dialog show draft sprint
as own card first, held rows with Waiting chip + reason; banner count
"n waiting". The boards and board adds phases, first in `phases()`, are that later addition;
see Draft boards below. Demo: epic whose summary contain "refused" refused
once per run; demo refuse placeholder parent.

## Draft boards

A board created in TAM is a draft, the same local-first shape as a draft
sprint (Draft sprints, above). `issuerepo.CreateDraftBoard` writes, in one
transaction, a `board` row under a negative id with `draft = 1` (schema
version 15 adds the column, both to `baseDDL` and as a migration) and a
`board_create` journal row (`EntityBoardCreate = "board_create"`) whose
`after_val` is `DraftBoard{Name, Type, FilterName, JQL}` as JSON. The id
comes from `nextDraftID`, one below the lowest of a stored
`draft_board_seq` setting and any negative id already in the table, so a
discarded or committed draft's id is never handed to a new one; draft
sprints use the same function over their own table and setting. `assertNoPlaceholders` (the
firewall that used to check only for a stray `sprintId`) now trips on a
negative `boardId` too, string or number, under its renamed `negRef`
parameter, so a board id that skipped a rekey fails loud, inside TAM, rather
than reaching Jira as a nonsense number.

This buys what every journaled create in TAM buys: a board can be made with
no connection at all, it shows in Pending changes like any other pending
write, and nothing about it has reached Jira until Commit runs. It costs an
extra id to rekey: the negative id stands in for a board that does not exist
yet, and everything that named it while it was still a draft (its own
columns and membership once synced, any card queued onto it, any rank
dropped on it) has to be rewritten the moment Jira hands back the real one.

`RekeyBoard(profileID, draftID, realID)` is that rewrite, run inside the
phase that creates the board (below), in one transaction: it clears any
stale row Jira's id might already hold in `board` (a boards refresh between
the create and the rekey could have cached one), moves the draft row onto
the real id and turns off its `draft` flag, then repoints every table keyed
by `board_id`: `board_column`, `board_issue`, `sprint`. It also repoints two
things that are not tables, both through `repointRows`: an `issue_rank`
journal row whose packed value names the draft board, and every
`issue_board` journal row queued onto it.

That last one is worth being exact about, because it was the fix-round-1
defect that broke the bundle's own headline flow. `AddToBoard`
(`boardwrites.go`) does not give a board add a fixed field the way a
transition or a sprint move gets one; it packs the board id into the field
itself, `BoardField(boardID) = "boardId:" + boardID`. An issue can be
queued onto more than one board at once, unlike a status or a sprint move,
which each have exactly one pending destination, and the journal is unique
on `(profile_id, entity_type, entity_key, field)`: a fixed field would have
let a second board's add silently collapse onto the first board's row
instead of sitting beside it. So `RekeyBoard` has to rewrite the row's
field, not just a value inside it, a different operation from every other
rekey in this codebase, and was what `RekeyBoard`'s first version missed;
its repoint rewrites both the field (`BoardField`) and the value's id half, leaving the scope half (backlog or a sprint id) exactly as queued.

**Phase order.** The `boards` phase runs first in `phases()`, then `board
adds`, because a card queued onto a board names the board by id before
Commit ever runs, and cannot be sent while that id is still a negative
placeholder. `createBoards` creates and rekeys every drafted board; the
journal re-read between the two phases hands `pushBoardAdds` the rows
`RekeyBoard` already rewrote, so an add queued onto a board created in the
same Commit is pushed with the real id. A sprint is never drafted onto a
draft board: `CreateDraftSprint` refuses `BoardID <= 0`.

A board create Jira refuses (or a connection that cannot create boards, or a
create Jira accepts but answers with no id, or a rekey that fails locally
after Jira already made the board) blocks the draft's negative id
(`r.deps.block`) rather than sending anything with a placeholder in it, so
an add queued onto it is held with a reason naming the board. The
`board_create` row survives the Commit, and since nothing about the failed
attempt left a second row or a stray Jira board, the next Commit creates the
board exactly once.

**The filter check after Commit.** Once `pushBoardAdds` lands a board's
queued issues (backlog scope batched through
`BoardCreator.AddToBoardBacklog`, sprint scope through the same
`MoveIssuesToSprint` a board move already uses, since putting a card on a
sprint already puts it on that sprint's board), their journal rows are
removed regardless of what happens next: Jira accepted the write, so
retrying it would only repeat it. `checkBoardFilters` is a courtesy read
after that, once per board that received at least one add this Commit:
which of the pushed keys the board's own filter actually kept. A key it
dropped becomes a non-retryable result line (`"PLAT-1 is outside board 1's
filter, so it will not show on that board."`), naming the board by id (the
committer owns no board-name lookup). A backend that cannot answer the check, or a read that fails,
changes nothing about the Commit: it is a read after a write that already
landed, logged and otherwise ignored, never a reason to fail it.

**The drawing change, as a release note.** Boards used to draw every draft,
on every board, all the time: `composeBoard` appended `DraftIssues`
unfiltered. That was never true, a draft belongs to no board in Jira, and
this bundle stops doing it. A non-subtask draft, or a real issue reached
only through local state (a pending board add), now draws on a board only
when one of four ties holds: a pending `issue_board` add naming this exact
board, a pending sprint move whose target is one of this board's own
sprints, a `SprintID` the card already carries that is one of this board's
own real sprints, or one that is one of this board's own draft sprints.
With none of the four, nothing is drawn. Users who currently see a draft
sitting on every board they open will notice it disappear from boards it
has no actual tie to; that is the fix, not a regression, but it will look
like one at a glance.

**Known edges.**

- The add-issues search caps at 25 results with no pager. A large backlog
  may need a narrower search text to surface the issue meant.
- The "not already on this board" exclusion it filters against is itself
  capped by the board read's shared per-view card limit; on a very large
  board an issue past that cap is not known as a member and could be
  offered again.
- `withMovedIn`'s board-add branch, shared with the Sprints view's own read
  (`sprintlist.go`), now also surfaces a pending board add in that view's
  matching sprint node. This is additive and untested there; the Sprints
  view's own test suite passed unchanged because none of its fixtures set a
  board add.

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

## Assigned to me

Fourth tab (`assigned` in `nav.ts` / `menuViews`), between Backlog and
Epics: same grid, filters, sort, pager, detail panel, narrowed to the
connected Jira user, read entirely from `tam.db` so it works offline and
includes the user's own uncommitted drafts.

Knowing "me": profile settings `jira_username`, `jira_display_name`
(`app_issues.go`), written by `TestProfileConnection` (`app_profiles.go`)
and at the start of every sync (`internal/syncer/syncer.go`), off the one
`GET /rest/api/2/myself` the connection check already makes and, until now,
threw away. An answer with no username is refused at both write sites
rather than written: overwriting a good stored value with "" would leave
the filter matching nothing, and unlike a write failure it would leave
nothing in the log to find the cause by (fixed 915c431; proven red with the
guard disabled, which recorded `jira_username = ""` over a seeded
"rahmad").

Caching the assignee key: schema 14 adds `issue.assignee_name`, written by
the sync normaliser off `fields.assignee.name` (falling back to the key
when name is empty, Data Center's GDPR mode), and by `EditField` and
`CreateDraft` off `AssigneePicker`, which already stores the username, not
the display name. The migration clears every profile's sync watermark
(`last_synced = ''`), the same reason version 5 did for `status_id`: an
incremental sync only touches rows whose fields changed since that
watermark, so a column added by a migration alone would stay empty on
every already-cached row forever without a full resync forcing it.

Match rule: `IssueQuery.AssigneeName` matches `assignee_name`
case-insensitively (`COLLATE NOCASE`, the same collation `sortColumns` uses
for the assignee sort). A row whose `assignee_name` is already populated
never falls through to the display-name check, even when the display name
would also match: the fallback exists only for a row cached before schema
14, whose `assignee_name` is still empty, not as a second chance for a
populated row, which would risk pulling in a different person who happens
to share a display name.

The limitation, plainly: `issue.assignee` holds two different kinds of
value, the display name sync writes (`fields.go`'s `displayName()`) and the
username a local edit writes (`AssigneePicker`, through `writes.go`). The
fallback above cannot match a row that was edited locally before the
migration ran, because that row's `assignee` already holds a username, not
the display name the fallback is matching against. Such a row does carry a
pending journal entry, though, and the pending-edit overlay covers it the
way it covers every uncommitted change, so it still shows up correctly,
just not through the fallback. Full write-up in `TODOS.md` ("The assignee
column holds two different kinds of value").

A pending reassignment needs no replay to be seen by this filter:
`EditField` writes `assignee_name` straight into the `issue` row
(`writes.go`), so the SQL `WHERE` already sees it, the same reason a
pending summary edit already shows in the Backlog grid with no merge step
of its own.

Reuse: `IssueListView` (extracted out of `BacklogView` this bundle) owns
the toolbar, grid, pager, detail panel and resize grip, parameterised by
`viewId`, `label`, `baseQuery: Partial<IssueQuery>`, which toolbar actions
show, `emptyNote`, and `onPage` (fires with the page's total and rows after
every load, so a caller can build its own summary off the same query
instead of firing a second one). `BacklogView` passes no `baseQuery`
(project-wide) with `showCreate` and `showImport`; `AssignedToMeView`
passes `baseQuery={{ assigneeName, assigneeDisplayName }}` with neither,
plus its own header line and stale-rows note (both built off `onPage`) and
its own empty state for no username yet, with Sync and Test connection
both reachable from it.

**Filter, sort and page state do not survive a tab switch, and that is a
deliberate deviation from the design.** The design asked that switching tabs
never reset the other tab's state, by way of the query key carrying the view
id. That mechanism would not have produced the effect: the filters live in
`useState` inside `IssueListView`, not in the query key, and `App.tsx` renders
one view at a time, so a switch unmounts the view and takes its state with it.
Backlog has always behaved this way, and matching it is the consistent choice.
Real persistence needs either every view mounted at once or the filter state
lifted above the view switch, which is a navigation-model change and not a
tab's to make. `viewId` survives as the namespace for the pager's element ids,
which is precautionary: nothing mounts two `IssueListView`s at once today.

**Waiting for the profile is two conditions, not one.** With no active profile
both settings queries are disabled, and a disabled query stays pending for
ever, so waiting on `isPending` alone left the whole pane blank on a fresh
install. Waiting only on the active id instead flashed the no-username empty
state on mount, before the settings had been asked for. The view waits for the
profile list's own load and, separately, for the settings of a profile that is
actually there.

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
holding focus. `runRitualRoot` share same private `runRituals` path, name and
banner, because `CreateRitualRoot` acquire `"rituals"` and end in full Sync.
Editor save, resolve, forget, delete take no lock at all.

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
                          of the destination's name, plus CreateDraftBoard and AddIssuesToBoard,
                          the two local board writes
    app_sprints.go       the two sprint ceremonies, SuggestSprintDates, and PendingInSprint
    app_sprintmanage.go  Create (a draft), Edit and Delete sprint (local for a draft), and
                          ListBoardSprintDetails for the Sprints view's tree, all under the
                          "sprint" lock name the ceremonies use
    app_reports.go       GetSprintReport, the one binding the Reports view calls, and
                          CancelSprintReport, which is how a view that has been left cancels a read
                          still holding the profile lock; both under the "report" lock name
    app_rituals.go       the ritual bindings: ensure, list, save, resolve, forget, delete, macro
                          preview, standup entry, last sync, Sync, CreateRitualRoot; all but Sync and
                          CreateRitualRoot under no lock, those two under the "rituals" lock name
    internal/tamstore/   TAM's own SQLite file (schema version 15: issue (with status_id and,
                          added at version 14, assignee_name, whose migration clears every
                          profile's sync watermark the way version 5's did), issue_link,
                          sync_state, profile_setting, jira_user, board (with draft, added at
                          version 15), board_column, board_issue, sprint (with goal, added at
                          version 7, and complete_date, added at version 8, and draft, added at
                          version 13), sprint_report (a sprint's saved report, added at version 8),
                          ritual_document (added at version 9, with base_body, conflict_body and
                          conflict_version added at version 12), plus the shared journal tables
                          pending_change and audit_log)
    internal/backend/    IssueBackend and BoardBackend seams and DTOs, plus BoardCreator
                          (CreateBoard, AddToBoardBacklog) and BoardFilterChecker
                          (BoardFilterCheck), the optional seams a board create and its
                          post-Commit filter check go through; backend/jira on core/jira,
                          backend/demo on internal/demo (BoardCreator only, no BoardFilterChecker:
                          demo boards have no real filter to check against); core/jira/createmeta.go
                          is the createmeta reader and value shaper backend/jira builds the create
                          dialog's fields from
    internal/demo/       the Acme Platform (PLAT) dataset behind a "demo" profile;
                          confluence.go is the in-memory Confluence space rituals sync against
                          on a demo profile, rebuilt from stored pages after a restart and
                          staging one conflict on the first Standup it creates, a missing root
                          for root id 1, and with DenyCreate a token that cannot create pages
    internal/issuerepo/  the store layer: issue cache, detail cache, links, sync state, profile
                          settings, the pending-change journal, and drafts; tree.go groups the
                          cache into the Epics view's tree; boardwrites.go, movevalue.go,
                          movecolumns.go, and rebasemoves.go are the three board moves, their
                          before_val/after_val packing, and what Override does to a held one, plus
                          boardwrites.go's fourth write, AddToBoard, packed onto BoardField(boardID)
                          rather than a fixed field so one issue can queue onto more than one board
                          at once; boarddrafts.go is the draft board's own CreateDraftBoard and
                          RekeyBoard, the board twin of sprintdrafts.go/rekeysprint.go below;
                          sprintdrafts.go is the draft sprint's create, edit, discard and
                          rewriteSprintID, and rekeysprint.go RekeySprint,
                          MarkSprintCreatedWithoutRekey and rewriteParentKey
    internal/boardrepo/  the store layer over board, board_column, board_issue, and sprint; view.go
                          composes the Boards view's data over the issue cache through IssueSource,
                          both in one deferred read transaction (tx.go); pendingmoves.go's
                          tiedToBoard is the three ties that let a draft, or a real issue reached
                          only through a pending board add, draw on a board Jira never put it on;
                          cellorder.go is the board's final local order the commit pass ranks
                          against, on the same kind of transaction; sprintlength.go is the
                          median-of-three-closed-sprints read the start dialog's date suggestion is
                          built from; sprintlist.go is the Sprints view's own read, one board's
                          sprints with their issues and the computed unassigned node; boardrepo.go's
                          Board.Draft labels a drafted board the way a drafted sprint already is;
                          deletesprint.go is DeleteSprintEverywhere, the two-repository delete's
                          first transaction, across every board that holds a copy of the sprint
    internal/sprints/    the push half of every journaled sprint write, for Commit alone:
                          ForCommit's Committed carries Edit, Delete (manage.go), Start and
                          Complete (sprints.go), and Service exports no write; draft.go is
                          DraftSprint, the check a drafted sprint gets, and errDraftSprint;
                          guards.go is what a push refuses before it reaches Jira, with
                          CheckComplete and CheckDelete, which the app also runs before it
                          journals, cache.go the board
                          cache's bookkeeping after it has, and suggest.go the start and create
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
                          firewall.go is assertNoPlaceholders, which trips on a negative sprint or
                          board id alike (negRef); boards.go, ranks.go, and boardvalues.go are the
                          board pass, after the edits and before the links; boardcreate.go is the
                          boards and board adds phases, first in phases(): createBoards and
                          RekeyBoard, then pushBoardAdds (backlog batched through AddToBoardBacklog, sprint scope
                          through the same MoveIssuesToSprint the board pass already uses), and
                          checkBoardFilters, the post-Commit filter courtesy read
    internal/importer/   maps import columns to draft fields and validates rows
    internal/syncer/     the paging engine; emits tam:sync-progress through app_issues.go; boards.go
                          is the boards pass, reached through backend.BoardBackend
    internal/errtext/    reduces an error to one readable line (strips HTML tags, collapses
                          whitespace) for sync summaries and dropped-board reasons
    internal/ritualtemplate/  renders a sprint's five ritual pages (pure, no clock, no I/O), the
                          body of a root page TAM creates (RootBody), and reads a Jira Issues
                          macro's JQL back (JQL, ParseJQL), the three written forms and nothing
                          else
    internal/ritualrepo/ the store layer over ritual_document; documents.go is the CRUD, the
                          dirty and status computation, and Apply{Created,Pulled,Pushed,Conflict},
                          MarkGone, the writes a Sync pass makes
    internal/ritualsync/ Ensure (write missing pages from templates, local, no lock), Run
                          (the Sync pass under the "rituals" lock: title match, adopt, create,
                          pull, push, conflict, gone, and a 404 root reported as RootMissing), and
                          root.go: CreateRoot, the top-level root page create or adoption
    internal/suiteprofiles/  which shared profiles TAM shows, demo detection, validation
    frontend/            React app on @agile-suite/core (see ../frontend/core); EditorToolbar
                          lives there
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
      src/lib/confluenceRoot.ts  the Profile settings root page id field: a number, or the pageId
                          read out of a pasted page address
      src/lib/standupLog.ts  finds where today's dated Yesterday/Today/Blockers section belongs
                          in a Standup page's Daily log, and whether it is already there
      src/lib/sprintOptions.ts  draftLabel(name, draft) is the "(draft)" suffix a sprint or board
                          picker option carries; sprintOptionLabel, sprintOption, and the new
                          boardOption all call it
      src/components/    BacklogView (a thin wrapper), IssueListView (the extracted toolbar, grid,
                          pager and detail panel both it and AssignedToMeView mount),
                          AssignedToMeView, IssueTable, IssueDetailPanel, EditableFields, ActivityTab,
                          AssigneePicker, PriorityPicker,
                          PendingChangesModal, ConflictCard, NewIssueModal,
                          MetaField (a create-meta field's input, by type, through META_INPUTS),
                          ProfilesModal,
                          ProfileForm, AboutModal, DiagnosticsModal (the paths, the build, the tail
                          of tam.log through ReadLog, and ExportDiagnostics, in the Help menu beside
                          About; the status bar stopped naming tam.db and profiles.db when it
                          landed), ImportIssuesModal, AddLinkForm, EpicsView,
                          EpicTree, EpicRow, BoardsView, BoardsToolbar, BoardBody, BoardGrid,
                          BoardCard, BoardNotes, NewBoardModal (the four-field draft board create,
                          reached from BoardsToolbar beside New sprint), AddIssuesModal (the
                          board's own issue picker: search over useIssues, membership exclusion
                          over useBoardSprintDetails, destination one of the board's open sprints
                          or its backlog), useBoardMoves (the three writes, the drag state,
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
                          XHTML, its own save state), ritual-editor/useRitualToolbar (TipTap state
                          mapped onto @agile-suite/core's EditorToolbar), RitualRootDialog (the
                          missing root page: create, adopt, or Profile settings), MacroPreview (a
                          Jira Issues macro rendered from cache), OpaqueViews (read-only rendering
                          of an opaque node)
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