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
