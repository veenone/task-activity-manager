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
