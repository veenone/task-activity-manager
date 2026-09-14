# TAM Phase 5 gaps: what Confluence and Rituals left open

An audit of the branch on 2026-09-13, written as a work list. Phase 5 shipped
past its own design document, and the things it skipped are the ones that only
show up when a profile is deleted or a spec is read as current.

## Where this starts

- Branch `feat/phase-4-reports-v2`, 21 commits ahead of `main`, HEAD at
  `1941b9b feat: complete reports rituals and sprint workflows`.
- Every suite is green as of this writing: `core` and `tam` vet and tests,
  46 tests in `@agile-suite/core`, 159 in `xtm` frontend, 485 in `tam`
  frontend, typecheck clean across all three workspaces.
- Six views are live. `tam/frontend/src/components/Placeholder.tsx` is no
  longer reachable through the nav rail, because `App.tsx` now branches on all
  six ids before falling through to it.

One repair is already in, so do not go looking for it: `App.test.tsx` was
failing on committed code. Its `vi.mock("./api")` factory is an allowlist, and
Phase 5 added four bindings nobody added to it, so `GetConfluenceConfig` fell
through to the real Wails binding and threw on `window.go` being undefined.
The same test also still asserted that Rituals says "arrives in Phase 5",
which stopped being true when the view got built. Both are fixed.

## Gap 1: deleting a profile leaves its Confluence data behind

This is the one worth doing first, and it is a data retention problem rather
than an untidiness problem.

Phase 5 added three tables and a second credential. The delete path knows about
none of them.

`core/profile/profile.go:346`, `Manager.Delete`, removes the profile row and
its connection and nothing else:

```go
res, err := m.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
```

`tam/app_profiles.go:160` deletes the Jira credential, keyed on the bare
profile id:

```go
if err := a.creds.Delete(id); err != nil {
```

The Confluence token is keyed differently. From
`core/profile/credentials.go:19`:

```go
func ConfluenceCredentialID(profileID string) string { return profileID + ":confluence" }
```

Nothing deletes that key. Nothing deletes the three tables either. They are
declared in `core/shareddb/shareddb.go` at lines 60, 67 and 77, all keyed by
`profile_id`, none carrying a foreign key or `ON DELETE CASCADE`:

- `confluence_profile`, the base URL, space key and root page id
- `confluence_association`, the ritual-to-page mapping
- `confluence_page_cache`, which holds `payload TEXT NOT NULL`, the rendered
  HTML of internal Confluence pages

So a user who deletes a profile still has that profile's Confluence token in
the OS credential manager and the text of its ritual pages on disk in
`profiles.db`.

This is the failure the Phase 4 plan predicted in writing. Its storage task
required any new table to join both purge lists, and noted that
`PurgeProfile`'s own comment "predicts this exact failure when a fifth table
arrives". Phase 4 obeyed the rule: `sprint_report` is in the list at
`tam/internal/boardrepo/boardrepo.go:84`. Phase 5 did not.

### What to build

Delete the Confluence rows and the Confluence credential when a profile goes.
Put the row deletion where the rows live, which is the profile manager rather
than `boardrepo`, since these tables are in the shared database and
`boardrepo`'s list covers `tam.db`. Take the three deletes in the same
transaction as the profile row, so a half-deleted profile is not reachable.

The credential belongs in `tam/app_profiles.go` beside the existing
`a.creds.Delete(id)` call, and should be tolerant of a missing key the way the
connection delete already tolerates `connection.ErrNotFound`, because a profile
that never configured Confluence has no key to remove.

### Acceptance

- Deleting a profile that had Confluence configured leaves no rows in
  `confluence_profile`, `confluence_association` or `confluence_page_cache` for
  that profile id, and leaves another profile's rows untouched.
- The Confluence credential for that profile is gone, and deleting a profile
  that never configured Confluence does not error.
- Follow the shape of `TestPurgeProfileClearsTheFourBoardTables` in
  `tam/internal/boardrepo/boardrepo_test.go:259`, which seeds two profiles and
  asserts only one is swept.

### Do not fix while you are in here

`core/profile/profile.go:358` carries `TODO(xtm): cascade-delete this profile's
test_case / sync_state rows (FR-5.3)`. That is older, belongs to XTM, and is
not this work.

## Gap 2: the Phase 5 verification list was never satisfied

`docs/superpowers/specs/2026-09-13-tam-phase-5-confluence-foundation-design.md`
ends with a verification section. Most of it does not exist yet.

The spec asks the client tests to cover auth headers, URL joining, page
decoding, child-page pagination, and status and error mapping.
`core/confluence/client_test.go` has two tests:
`TestClientGetsPageWithBearerAndDecodesBody` and `TestClientMapsHTTPError`.
Pagination through `ListChildPages` is the notable hole, since `start` and
`limit` are threaded from the app seam and nothing exercises them.

The spec asks for app tests covering missing configuration, missing
credentials, and a successful page read. There is no `tam/app_confluence_test.go`
at all. The three failure branches already exist in `confluenceClient` at
`tam/app_confluence.go:68` and return distinct messages, so they are cheap to
pin.

The spec asks for profile tests that round-trip the optional Confluence fields
and prove exported profile configuration excludes the token. Neither exists.
The export assertion is the one that matters most, because it is what stops a
token reaching an exported file.

## Gap 3: the spec no longer describes the code

The Phase 5 design says, in its own words, that the slice "does not render
pages or add caching". The code does both. `RitualsView.tsx` renders pages, and
`confluence_page_cache` plus `CacheConfluencePage` and `CachedConfluencePage`
in `core/profile/profile.go:76` are a cache.

Either bring the spec up to what shipped, the way `024bb45 docs: bring the
Phase 4 plan up to what shipped` did, or say plainly that the slice grew. A
document that reads as current while describing a system that no longer exists
sends the next reader looking in the wrong place.

Related: Phase 5 is the only phase with a spec and no plan. Every other phase
has a matching file in `docs/superpowers/plans/`.

## Gap 4: loose ends

- `Placeholder.tsx` is unreachable through the nav rail. Keep it as a defensive
  default for a view id added without a branch, or remove it, but decide rather
  than leave it ambiguous.
- `tam/frontend/src/App.tsx:233` renders a disabled button promising that the
  launcher "arrives in Phase 6". No Phase 6 spec or plan exists. Either write
  one or stop promising it on screen, the same way the Phase 4 plan required
  `nav.ts` to stop promising charts it did not build.
- Phase 4's own leftovers are unchanged and honestly recorded: the four wire
  probes were never run against a live Data Center, which
  `docs/superpowers/plans/assets/2026-09-11-report-wire-probe.md` says in its
  first paragraph, and the plan's final human walk-through has not happened.

## The gate

Run these once at the end rather than per change.

```bash
cd core && go vet ./... && go test ./... -count=1 && cd ..
cd xtm && go vet ./... && go test ./internal/... -count=1 && cd ..
cd tam && go vet ./... && go test ./... -count=1 && cd ..
npm run typecheck --workspaces --if-present
npm test --workspaces --if-present
cd tam && wails build && cd ..
git status --short --untracked-files=no
```

Counts to beat, not to lower: 46 in `frontend/core`, 159 in `xtm`, 485 in
`tam`. If a number drops, something was deleted rather than fixed.

## One warning about this branch

Another session has been editing this working tree throughout the audit. Files
moved under inspection twice, and work was committed mid-review. Check
`git status` and `git log` before starting rather than trusting the state
described here, and expect the Confluence area in particular to have moved.
