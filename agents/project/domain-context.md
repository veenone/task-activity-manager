# agile-suite domain context

Verified project facts for writing code: API quirks, platform behavior,
and approaches that already failed. Read before working on the subsystem
it covers. New entries arrive through the self-improvement protocol
(AGENTS.md M5) when a session learns a durable fact the hard way. Entries
carry no personal data, hostnames, or addresses. Each entry cites the
commit hash behind it; the instruction gate checks the hash exists.

## Local store and schema

- A profile-keyed table must join both purge lists, not one. `ritual_document`
  arrived at schema version 9 and reached neither, so deleting a profile left
  its documents on disk (39981c5) and removing a board orphaned that board's
  rows (303bd3a). `PurgeProfile` sweeps by profile, `RemoveBoards` by board;
  a table carrying both keys belongs in both.
- SQLite text columns are case-sensitive and `ritual_type` has no
  `COLLATE NOCASE`. A lookup with the caller's raw case returns a zero-value
  row and no error, so the write that follows blanks the real row's fields.
  Normalise the key once, before the first repository call, and use the same
  value for lookup and write (73bbefb).
- `Base` DDL runs on every open, before migrations. A new table therefore
  needs no migration entry, only a version bump; a new column on an existing
  table does need one, in the `AddColumnIfMissing` shape.

## Opening the local database

- Migrations run before indexes, or an old database fails to upgrade: an index
  built against a column a pending migration has not added yet aborts the open
  (0021930).
- The store opens in WAL mode with no single-connection cap, and retries a
  transient lock on startup rather than failing the launch (9633cdd, 4888574).

## Jira issue links

- Issue links are directional and the direction carries the meaning. Coverage
  reads as "tested by" from one side and something else from the other, so a
  link is resolved by direction, never by type name alone (6ef4b6d, 0218fca).

## Jira saved filters

- Data Center has no route that lists the filters a user owns.
  `/rest/api/2/filter/my` and `/rest/api/2/filter/search` are both Cloud only
  and 404 on Data Center; the 9.12 reference lists the whole filter resource
  as `POST /filter`, `GET/PUT/DELETE /filter/{id}`, `/filter/{id}/columns`,
  `/filter/{id}/permission`, `/filter/defaultShareScope` and
  `/filter/favourite`. A feature was built on `/filter/my` and shipped dead,
  saved from erroring only by its own fallback (22b6dd6).
- Which filters are a user's own is answered per filter, not per list:
  `/filter/favourite` carries `owner.name`, and `/rest/api/2/myself` answers
  with the same username, so comparing the two is the scope. Both names have
  to be non-empty. Cloud identifies users by account id and leaves `name`
  empty, so matching empty against empty would mark every filter as the
  reader's own (4c20722).
- A filter nobody starred is in no list Data Center will enumerate. Reaching
  one needs its id and `GET /filter/{id}`.
- The Data Center REST reference at developer.atlassian.com is rendered
  client-side and unreadable to a fetch tool. The static reference at
  `docs.atlassian.com/software/jira/docs/api/REST/<version>/` is plain HTML
  and answers these questions; it is large, so read it at an offset.

## Jira worklogs

- An issue's worklog comes from `GET /rest/api/2/issue/{key}/worklog`, which
  pages. A search's `fields=worklog` caps at 20 entries per issue and says
  nothing about the rest, so an issue worked on for a month would read as one
  worked on for a week (6400c42).
- `started` is an offset-carrying stamp, `2026-09-29T01:00:00.000+0700`, and
  Jira files the entry under the day that offset makes of it. Sending the same
  instant as UTC moves the work to the day before for anyone east of
  Greenwich, which is then wrong in every Jira report that groups by day, the
  main reason people log work at all. TAM formats it from the machine's own
  clock with `2006-01-02T15:04:05.000-0700` and sends it verbatim (6400c42).
- Jira parses the duration phrase itself and refuses a zero or unparseable
  one with a 400, which without a check of its own would surface at Commit
  rather than at the keystroke. A day and a week inside a duration are
  instance settings; the defaults are eight hours and five days, which is what
  `ParseWorkSeconds` assumes for the total TAM shows beside an entry it has not
  pushed yet. Jira's own seconds replace that reading once Commit has landed
  it, so a non-default instance reads a pending `1d` a little high rather than
  reporting a wrong number for ever (55129be).

## Jira issue types

- A project's issue types come from `GET /rest/api/2/project/{key}`, and the
  names are the instance's, not Jira's: the sub-task level is "Technical
  task" on one instance seen in the field and the task level "Todo" on the
  same one. Discover them; never hardcode a name.
- A sync scopes by what it excludes, not by what it names. Naming the six
  types TAM models fetched 38 issues of a project's 2,943 and reported it as
  a success (#68). Excluding leaves a type the project adds later in scope.
- Xray's types cannot be recognised by name either, for the same reason XTM
  discovers its own. The only signal in the project response is `iconUrl`:
  a plugin's types are drawn from its own bundled resources, so Xray's point
  at `com.xpandit.plugins.xray`. It is a heuristic with two failure modes,
  both in `TestXrayTypesAreRecognisedByTheirPluginIcon`: an Xray type given
  an uploaded avatar is synced anyway, and a type of the project's own given
  an Xray icon is left out.
- An issue whose type TAM has no logical type for keeps the project's own
  name for it, everywhere, and draws on `chip-type-none`. A
  `chip-type-Improvement` class no stylesheet defines renders as unstyled
  text and reports nothing.

## Jira create and edit screens

- Jira decides which fields an issue may be edited with per project and issue
  type, through the edit screen they carry, and
  `GET /rest/api/2/issue/{key}/editmeta` is the only call that says so. One
  instance seen in the field answers six fields for every type of a project,
  story and sub-task alike: summary, priority, reporter, description, labels
  and assignee. Story Points is not among them, although it reads fine on the
  issue and exists in the instance's field list (a0672d1). A field list is not
  a screen.
- The editmeta payload is the same `fields` map keyed by field id that the
  classic create-meta call returns, so `MetaField` and `MetaSchema` read both
  (09ab90a).
- The edit screen and the create screen are separate configurations and a
  field can be on one and not the other, so neither answer stands in for the
  other.
- TAM's own fields are the ones a screen check forgets. Story Points, the
  Epic Link and an Epic Name are set by the form rather than by the extras,
  so the create-meta filter in `applyExtras` never saw them and Jira refused
  the whole create when a screen lacked one. Both write paths now check them
  (bcbb486 for the edit, and the create beside it).
- Being on the screen is not the whole answer: each field carries an
  `operations` array, and a field listing add and remove but not `set`
  refuses the single value an edit sends. The instance behind #52 answers
  `["set"]` for everything and `["add","set","remove"]` for labels, so it
  does not hit this, but the array is the half of the payload that says what
  a write may do. An absent array is not an empty one; some payloads omit it,
  and that says nothing rather than no.
- A screen that cannot be read is not an empty screen. Treating a failed
  editmeta read as "nothing is editable" would refuse every field on a 403;
  the write goes and Jira's own refusal decides (a0672d1).

## Jira board configuration

- `/rest/agile/1.0/board/{id}/configuration` carries each column's `min` and
  `max` and the board's `columnConfig.constraintType`. Both limits are
  optional per column and a board that sets neither is the ordinary case, so
  every layer that holds one holds a pointer: as ints, an unset limit and a
  limit of zero are the same value, and every column on every board reads as
  over a limit of nothing (#101).
- `constraintType` says what a limit counts, `issueCount` or
  `issueCountExclSubs`. It is a board fact and Jira puts it beside the
  columns rather than on them, so TAM carries it on every `BoardColumn`: the
  boards sync reads nothing but the columns, and a count that ignored it
  would disagree with the number Jira's own board shows the same team.
- The endpoint answers the same shape for a scrum board as for a kanban one,
  so nothing reading a limit asks which kind of board it has. Not probed
  against a live instance: a scrum board that answered with no limits would
  read as a board with none rather than misreport one (#101).

## Confluence API

- A form post to Data Center needs `X-Atlassian-Token: nocheck`. Without it
  the instance rejects the request as XSRF, which is why attaching a file has
  its own request path in `core/confluence/client.go` rather than going
  through the JSON `send`.
- Posting an attachment whose filename is already on the page is a 400 on
  some Data Center versions and a second attachment with the same name on
  others. `AttachFile` therefore looks the filename up first and posts to the
  existing attachment's `/data` endpoint, so republishing a report replaces
  its chart. Atlassian documents that data update as a POST; the PUT on
  `/child/attachment/{id}` updates an attachment's properties, not its bytes.

## Modals

- Modal layering and backgrounds have cost four separate fixes: the grid header
  painted over every modal (db86b4f), modal cards had no background (425e65f),
  and two more reworked modal layout (a36b1dd, 8b46d9c). The dialog primitives
  in frontend/core exist because of this; the Modals contract is the rule.

## Demo mode

- Demo data must never fabricate state that only a real operation produces.
  A demo seed wrote a published status, a made-up page id and hand-written
  body HTML into real user databases, which would have made every later
  comparison against that body a permanent false difference (bb6799d).

## Generated bindings

- Wails generates TypeScript from the Go types. An anonymous Go struct
  degrades to `any` with a comment, but a qualified type inside one, such as
  `time.Time`, truncates mid-token and emits invalid TypeScript that stops
  the dev server. Types crossing the binding are built from named structs.
- A generated file that disagrees with the Go source means the generator has
  not run, not that the file needs editing.

## Git and remotes

- A PR whose base is another feature branch merges into that branch, not into
  the default branch, so a stack merged in order leaves every commit but the
  first somewhere the default branch cannot see. Merge the base down first and
  retarget, or open the last PR against the default branch once the stack is
  complete. This has already caught this repository once.
- `gofmt -l` flags almost every Go file on a Windows checkout because
  `core.autocrlf` rewrites line endings. The formatting is fine; the check
  belongs in CI, which checks out LF.

## Platform quirks

- The one revert in this repository's history is a UI polish change that
  bundled container filters, a docked detail panel, collapsible panels, a
  pager and copy changes into a single commit (5020d4f). Land UI changes
  narrow enough to revert one at a time.

## Xray datasets over REST

Verified against a live Xray Server/DC instance (Jersey 2.48) in October 2026,
while planning the dataset write feature (#277). The read path these entries
correct shipped in e0dbf0ce.

- There are two dataset APIs, not one. `/rest/raven/2.0/api/dataset/export` is
  the documented CSV export. `/rest/raven/1.0/dataset` is the JSON API the Xray
  UI itself drives, and it is the one a write needs. Both answered on the same
  instance.
- `/rest/raven/2.0/api/dataset/import` exists and accepts POST. A widely cited
  community thread reports it answering 404; that does not reproduce. Confirm
  with `OPTIONS`, which returns a WADL naming every query parameter: the import
  takes `testIssueId`, `testIssueKey`, `contextIssueId`, `contextIssueKey` and
  `testVersion`, and **no** overwrite or create-parameters flag.
- The two flags the import dialog shows live on a different route:
  `POST /rest/raven/1.0/dataset/parseDataset` takes `createNewParameters`,
  `overwriteValues`, `delimiter` and `encoding`. Alongside it,
  `POST /rest/raven/1.0/dataset` takes the whole dataset as JSON and answers
  with JSON, with no flags at all.
- `GET /rest/raven/1.0/dataset?entityId=<id>&type=TEST|TEST_RUN` returns three
  fields: `entityDataset`, what this level defines; `resolvedDataset`, what it
  would inherit from the Test; and `resolutionMessage`.
- **`resolutionMessage` names no level.** It reads "This is the resolved
  dataset, defined in the Test." at the test level, at a run with no override
  and at a run that does override, which was checked one case at a time on a
  live instance while building W3. An earlier entry here claimed the server
  says which level a dataset comes from. It does not, and anything built on
  that sentence would have been wrong in the one case that matters.
- The pair is what answers the question instead. At a run, `entityDataset`
  holds the override and is empty when there is none, while `resolvedDataset`
  holds the Test's table either way. Nearest level wins at execution time, so
  a non-empty `entityDataset` is what will run.
- `type` accepts `TEST` and `TEST_RUN`, uppercase, and nothing else. A Test
  Execution is not addressable: its `entityId` is the Test Run id, which
  `GET /rest/raven/1.0/api/testrun?testExecIssueKey=&testIssueKey=` returns,
  answering 400 when the test is not in that execution.
- Nothing lists the contexts that override a test's dataset.
  `/rest/raven/1.0/dataset/analytics/override` answers 405 to a GET,
  `/dataset/info` wants parameters this does not, and `/dataset/info/calledTest`
  answers 403 to an ordinary user. Finding the overrides of one test means
  asking its runs one at a time, which rules out a badge on a container's
  member list.
  Passing an execution's issue id with `type=TEST` answers 200 with an empty
  dataset, and passing a run id with `type=TEST` answers 500.
- `GET /rest/raven/1.0/dataset/info` reports the server's own limits:
  `maxParametersSize` (20) and `maxParametersValueSize` (100 characters per
  value). Read them rather than hardcoding them. The per-value cap is easy to
  miss and the server enforces it.
- `GET /rest/raven/1.0/dataset/hasDataset` answers `{"hasDataset":bool}` without
  transferring the table, which is the cheap way to ask whether a level
  overrides.
- A test with no dataset and an instance with no endpoint both answer 404, but
  they are distinguishable: a missing dataset is
  `application/json` `{"error":"Dataset not found"}`, while a missing route is
  Jersey's `application/xml` `<status><status-code>404</status-code>`. No
  version detection is needed to tell them apart.
- The CSV export answers `application/octet-stream`, not `text/csv`, and the
  body is raw CSV with no JSON wrapper. A combinatorial parameter carries an
  asterisk suffix on its column name. A short row is padded with empty cells.
- List parameters keep their allowed values in the JSON API
  (`info.type` is `LIST`, with `options`, `paramTestListScope` and `listId`) and
  lose them in the CSV export, which carries values only. A reader that only
  speaks CSV cannot tell a list parameter from a text one.
- The Universal Plugin Manager route that would report the Xray version
  (`/rest/plugins/1.0/...`) answers 403 to a non-administrator, so an ordinary
  user's client cannot read the plugin version.

### Iterations of a parameterized run

Answered against the same instance while planning #267 Phase 4, which the
issue's Phase 0 had left open. Fixtures in `xtm/internal/jira/testdata/`.

- `GET /rest/raven/2.0/api/testrun/{runId}` carries `iterations`: one entry
  per dataset row, each with its own `id`, `status`, `color` and the
  `parameters` it runs with. The 1.0 resource carries none of it, nor does
  `testexec/{key}/test?detailed=true`, which is what XTM syncs today.
- **A single-row dataset produces no iterations.** The run carries a flat
  `parameters` array instead. A reader that only looks for `iterations` shows
  such a run as if it were not parameterized.
- **A run freezes its dataset when it is created.** Replacing the test's
  dataset afterwards leaves the existing run reporting the parameters it was
  created with, under their old names. Only a run created after the change
  sees it.
- `GET /rest/raven/2.0/api/testrun/{runId}/iteration/{id}` reads one on its
  own, with its `steps`. There is no `/iteration` collection: asking for one
  answers 404 in both API versions, which is what makes the run GET the only
  way in.
- **A per-iteration write exists**, which #267 wrote as an open question
  against the JSON results import. `PUT` that same iteration resource with
  `{"status":"PASS"}` and it answers `{"id":<id>,"stepResults":[]}`: the id
  alone, not the updated iteration. The run's own status then moves to
  `EXECUTING` on its own, because Xray derives it from the iterations.
- `.../iteration/{id}/status?status=PASS` is not a route, despite the run
  having one of that shape. The iteration's status is written through the
  body.

The steps of an iteration, read while building #334 against the same instance:

- The run resource lists iterations **without their steps**. Steps come only
  from `GET /rest/raven/2.0/api/testrun/{runId}/iteration/{id}`, so reading a
  whole run costs one call per iteration on top of the two the run needs.
- A step carries **no id and no status of its own**. Only the iteration has a
  status, so a per-step result has nothing behind it to show.
- A step's fields are keyed by their **display name** (`Action`, `Data`,
  `Expected Result`), each `{type, value:{raw, rendered}}`. Those names are
  instance configuration, not API, so a step whose keys match none of them
  still has to keep its place or every step after it is renumbered.
- **Xray substitutes the row's values before it answers.** `Data` comes back
  resolved, so a reader that substitutes again is substituting twice.

Writing one back, confirmed while building #336 on the same instance:

- The `PUT` Phase 4 found does work: `{"status":"PASS"}` on the iteration
  resource is accepted and the next read shows the new status.
- **A closed Test Execution refuses it with 403**, `Current user does not have
  permissions to execute the Test Run`. The same write on an open execution
  from the same account succeeds, so the message names a permission but the
  cause is the execution's state. This is the first thing to check when a
  write is refused.
- The run's own status did **not** move to `EXECUTING` here: writing one of
  two iterations to PASS left the run at `TODO`. Phase 4 recorded a move to
  EXECUTING, so the derivation depends on more than one iteration changing.
  Either way it is Xray's rule, which is why the run is re-read after a write
  rather than recomputed.
- **Per-step results still cannot be written.** The read gives steps with no
  id, and the write answers `stepResults: []`, so nothing exposes a
  step-result id to address. `SetIterationStepStatus` stays unsupported.

### Writing a dataset

The same spike, continued against a throwaway Test. Every operation the
planning issue called impossible under merge semantics works, because the
write that matters is not the CSV import.

- `POST /rest/raven/1.0/dataset?entityId=<id>&type=TEST` takes
  `{"params":[{"param":{...},"values":[...]}]}` and **replaces** the whole
  table. Changing a value, dropping a row, dropping a parameter, renaming one
  and toggling `combinatorial` were each confirmed by writing and reading back.
  The wrapper shape the GET returns is not accepted; send the bare `params`.
- The JSON API addresses an entity by numeric id only. There is no key
  parameter, unlike the 2.0 export, so a caller holding a Jira key resolves it
  first with an ordinary issue GET.
- A new parameter is asked for with a **negative id**. Omitting `id` or
  sending `0` is refused with
  `{"error":"Parameters (0) not associated with the dataset."}`. Nothing
  documents this, and it is the first thing a client gets wrong.
- A parameter sent with the **id it already has** is updated in place: the id
  survives and a rename under it stays a rename rather than a removal and an
  addition. Sent as new instead, it is rebuilt, and a `LIST` parameter whose
  `info` was not sent back comes back as `TEXT` with its allowed values gone.
  Sending `type` and `options` keeps it a list but rebuilds the list itself
  with a new `listId` each write; sending the whole `info`, `listId` and
  `paramTestListScope` included, leaves it untouched. That is #283.
- `GET /rest/raven/1.0/dataset` answers 200 with an empty `params` array for a
  test that simply has no dataset, so on this route nothing has to read a 404
  as data. A nonexistent entity id answers 500 with a Java null-pointer
  message, which is another reason to resolve the key through the ordinary
  issue GET first.
- An empty `params` array clears the dataset: `hasDataset` goes false
  afterwards. Neither API publishes a `DELETE`, and both answer 405 to one, so
  this is how a dataset is removed.
- A successful write answers `Content-Type: application/json` with a body that
  is **not** JSON: the sentence `Dataset Updated with success.` A failure
  answers JSON. Branch on the status, not the body.
- Multipart writes to `raven/1.0` are refused with a plain-text 403
  `XSRF check failed` unless the request carries
  `X-Atlassian-Token: no-check`. The JSON write on the same prefix does not
  need it, so the check follows the content type rather than the API.
- `POST /rest/raven/1.0/dataset/parseDataset` requires `delimiter` and
  `encoding` in the query despite the WADL marking nothing required; without
  `encoding` it answers 500 `{"error":"Null charset name"}`.
- `POST /rest/raven/2.0/api/dataset/import` does create a dataset where none
  existed. It takes `multipart/form-data` with a `.csv` filename, and answers
  200 with an empty body. A non-CSV filename is refused with
  `{"error":"The file extension is not valid. Please use a CSV file."}`.
