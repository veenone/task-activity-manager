# XTM architecture and demo mode

Moved verbatim from xtm/CLAUDE.md. Read it for work in xtm/. Rules extracted
from it live in AGENTS.project.md; this file is architecture and detail.


This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**Xray Test Manager** — a lightweight Windows desktop application for managing
**Xray test cases in Jira Data Center** at scale (10k+ test cases, ceiling 50k).
It exists because the Jira browser UI is too slow for bulk test-case work.

Local-first: test data is synced into a local SQLite cache for instant
browse / search / filter; edits are tracked locally and pushed back to Jira
**on commit**. Jira is always the system of record.

Full planning and requirements live in the Outline collection **"Xray Test
Manager"** — Overview, Architecture, Functional Requirements (FR-1…FR-13),
Non-Functional Requirements, Roadmap (7 phases), Risks & Open Questions.
Requirement IDs (`FR-1.4`, `FR-13.5`, …) are referenced throughout the code and
in commit messages; keep using them.

## Stack

- **Go** backend + **Wails v2** desktop shell + **React + TypeScript** frontend.
- **SQLite** local store via the pure-Go `modernc.org/sqlite` driver (no cgo).
- Single distributable **Windows** executable; WebView2 is the only runtime
  prerequisite.
- Target server: **Jira DC 8.14+** (Personal Access Tokens) and **Xray
  Server / DC 8.4.0**.

## Layout

This directory is the `xtm/` module of the agile-suite monorepo (Go module
`agile-suite/xtm`; `go.work` at the repo root ties it to the shared `core`
module in `../core/`). Profiles, connections, settings, and the OS credential
stores live in `core`, not here.

```
main.go              Wails entry point, window options, bound App
app.go               App struct — backend wired to the React frontend
internal/
  jira/              Jira DC + Xray Server REST client (/rest/api/2/ + /rest/raven/2.0/)
  store/             SQLite local store, schema, and migrations
  testrepo/          Local Test repository — browse/search/filter + all local mutations
  syncer/            Pull-sync engine (Jira -> store) and commit engine (store -> Jira)
../core/             Shared suite module (agile-suite/core)
  store/             SQLite open + ordered-migration runner both apps use
  shareddb/          The shared profile database: schema, Open, DefaultPath
  profile/           Connection profiles + OS credential storage
  connection/        Per-workspace connections (backend, URL, project)
  settings/          Global app preferences
frontend/            React + TypeScript (Vite), rendered in WebView2
  src/api.ts         The frontend's typed API. It only re-exports src/api/, one module per
                     domain; import from "./api", never a module, so one vi.mock covers all
  src/contexts/, components/   several files are re-exports from @agile-suite/core (frontend/core)
  wailsjs/           GENERATED bindings — do not hand-edit
build/windows/installer/installer.iss  Inno Setup script; its AppId is XTM's own and never changes
```

## Architecture — the big picture

**The bound `App` is the only boundary.** Every exported method on `App` in
`app.go` is callable from React (Wails generates `frontend/wailsjs` bindings from
them). `app.go` is a thin adapter: it validates, loads the profile + PAT, and
delegates to `internal/`. Keep real logic out of it. Adding/renaming an exported
`App` method changes the frontend API surface. `frontend/src/api/bindings.ts`
re-exports the generated bindings, and the domain modules beside it wrap the ones
that take a model class, so regenerate (`wails dev`/`wails build`) and update the
module that owns the method.

**`requireStore()` guards every store-dependent method.** The Windows GUI has no
console, so a backend init failure must surface as a returned error, not a
nil-pointer panic. `startup` captures any failure into `startupErr`; `Health()`
and `GetDiagnostics()` stay callable even when the store failed to open so the
frontend can show what actually broke.

**Profiles, connections, and settings live in the shared suite database**,
`<user config dir>/agile-suite/profiles.db` (`core/shareddb`), which
`initStore` opens next to `xtm.db`. On the first launch after the upgrade,
`internal/sharedmigrate` copies the rows from `xtm.db` across once and records
a marker so it never runs again. XTM's own store still declares its old
`profiles`, `connection`, and `app_setting` tables, but only as a downgrade
backup: nothing reads or writes them any more, and `testrepo` gets a profile's
defect settings through a lookup the app injects from the profile manager
instead of querying the table.

**Local edits are a journal, committed later — not written through.** Mutating
methods (`EditTestField`, `TransitionTest`, `BulkEditTests`, step/precondition/
container/folder ops, review verdicts) do **not** call Jira. They write to the
`pending_change` table (coalesced by `UNIQUE(profile_id, entity_type,
entity_key, field)`, so repeated edits collapse and reverting to the original
drops the row) plus an `audit_log` entry. `CommitPendingChanges` is the single
place that pushes to Jira, via `syncer.Engine.CommitChanges`, grouping pending
rows by entity and applying field updates → transitions → step CRUD per Test.
Failed/conflicted Tests keep their pending rows so the user can retry or discard.

**Conflict detection is optimistic.** Each pending change records the remote
`updated` it was based on (`base_version`). At commit, the engine re-fetches the
remote `updated`; if it moved, the Test is reported as a `Conflict` and held
back (FR-1.4). The user syncs, then either overrides (rebase onto remote) or
keeps remote (discard local).

**A dataset is read from Xray's JSON model, never the CSV export.** The export
carries values alone, so a parameter read through it loses its id and, for a
list parameter, its allowed values; writing that back rebuilds every parameter
and flattens the list (#283). The model is addressed by numeric id, so a
test-level read costs an issue GET first and an execution-level read costs a
run lookup. Both ask for `entityDataset`, what that level defines: an empty
answer at an execution means the test's own dataset is what runs there, and
the panel says which it is showing. The CSV parser stays for the files users
bring back from a spreadsheet, and the export route is gone.

**The dataset CSV is Xray's format, so its reader and writer live in
`internal/jira` beside the rest of it.** A CSV carries names, the
combinatorial asterisk and values, and has nowhere to put a parameter's id or
the list behind it, so an import matches those back by name from the table it
replaces (`testrepo.MergeDatasetParameters`). Skipping that would rebuild
every parameter on the next write, which is #283 again through a different
door. A column with no match is normalized to `TEXT`.

**Dataset parameter configuration is separate from cell editing.**
`DatasetParameterDialog` returns one complete parameter to `DatasetEditor`.
Text parameters accept free-form cells. An ad-hoc List carries `type=LIST`,
ordered unique `options` and `paramTestListScope=PARAMETER_AD_HOC`; a new List
has no positive `listId`. Existing predefined bindings stay read-only and are
sent back with their ids and scope unless the user confirms conversion. XTM
does not administer project lists. `testrepo.ValidateDataset` checks this
contract before the whole table can enter the journal.

**Iterations come from the run, never from the cached dataset.** How each
dataset row went in an execution comes from the 2.0 run resource. It is then
stored: `app.GetParameterizedRun` writes what the backend returned through
`ReplaceParameterizedRun` and reads it back before the view sees any of it,
because a status edit needs the stored row to journal the value it replaced.
A run the store refuses therefore reaches nothing at all, and the member row
shows no iterations with no error anywhere, which is how #362 hid every live
iteration for two releases. The gate for that is
`xray.TestLiveParameterizedRunSurvivesTheStore`, which puts the live reader's
own output through the store rather than asserting the reader in isolation.

What must never happen is rendering iterations from the cached dataset. They
are a property of the run: a run keeps the parameters it was created with, so
a dataset edited afterwards does not change a run that already exists, and
drawing one from the other would show rows that run never had (#267 Phase 4).

**Renaming a dataset parameter is two changes, not one.** The dataset write
carries the rename by sending the parameter's existing id, but every
`${placeholder}` in the test's steps and definition still names the old
parameter and would resolve to nothing. `RenameDatasetPlaceholder` rewrites
those through the ordinary step and field edits, so each rewritten field is
its own pending change. The editor offers it after a rename and does nothing
unless the user agrees: a reference may be deliberate.

**A dataset is journalled whole, and its conflict check is its own.** A
dataset edit queues one `test_dataset` row holding the entire table as JSON in
`before_val` and `after_val`, because the write replaces the table rather than
patching cells. It has no `updated` watermark either, so the commit pass in
`internal/syncer/commit_dataset.go` reads the table Jira holds and compares it
with `before_val` instead of using `base_version`. Keeping "mine" on such a
conflict rewrites `before_val` to the remote table, which is what lets the next
commit through. The comparison looks at names, combinatorial flags and rows
only: Xray reissues parameter and list ids whenever it rebuilds a parameter,
and a conflict over bookkeeping nobody changed would be a dead end. The write
takes its ids from the table Jira holds at commit time for the same reason.
The frontend call flow remains `DatasetEditor` to `SaveTestDataset`, then
`testrepo.EditTestDataset` validates, caches and journals the full table in one
transaction. Commit later maps the stored parameter metadata to Xray's
column-major JSON payload.

**Parameterized Test Runs are cached and committed by stable result ids.** A
Test remains one execution member; `test_run_iteration` and
`test_run_iteration_step` hold its nested result tree. The frontend loads that
tree lazily with the full profile, execution and test identity in its query
key. Status edits journal `test_run_iteration` or
`test_run_iteration_step` rows. Before a write, `commit_iteration.go` reads the
remote tree and compares the targeted stable id with `before_val`: a different
remote status becomes a conflict, an already-applied value clears locally, and
a backend refusal keeps only that row pending. Detail and write capabilities
are separate. Demo2 advertises both for its deterministic fixture; live Xray
and Kiwi keep both false until fixture-backed endpoints prove their contracts.

**A JUnit result for a newly created execution resolves after allocation.**
The preview matches a testcase's complete ordered property signature against
the cached test-level dataset. Existing parameterized Tests retain that
signature; a newly created Test remains an overall run because it has no
dataset. Apply journals the retained result as
`test_run_iteration_import`, keyed by temporary execution key, Test key and a
signature hash. The container-create rename rewrites that pending key. After
membership commit, `commit_junit_iteration.go` fetches the new parameterized
run, verifies the dataset source, requires exactly one signature match, and
writes by the stable iteration ID returned by the backend. Source drift and
zero or multiple matches produce a conflict that keeps the pending row. The
deferred payload does not treat a preview-only signature hash as a server ID.

**Created entities use temporary keys.** New Preconditions/Containers/Steps get
a placeholder key locally; the commit path creates them in Jira first, then
rewrites dependent pending rows (e.g. associations) to the real key before the
per-Test pass runs.

**Steps and custom-field values load lazily.** The bulk sync pulls Test fields
only — fetching steps/custom fields per Test would be thousands of extra round
trips. `GetTestSteps` / `GetTestCustomFields` fetch on first detail-panel open
and cache; `forceRefresh` re-pulls.

**Schema migrations are ordered.** `store.go` defines `baseSchema` →
`applyMigrations` → `indexSchema`, so an old DB gets missing columns added
before any index references them. Bump the `schemaVersion` constant on any
schema change (it surfaces in diagnostics).

## Demo mode — how to run without Jira

Create a profile with **Jira base URL `demo`** (any project key, any token).
`jira.isDemoURL` short-circuits every REST call to a deterministic generator
serving ~5,000 tests plus sample Test Sets/Plans/Executions, folders, and
preconditions, so the whole UI works end to end offline. The header shows a
yellow `DEMO` chip. This is the default way to exercise the app today.

A write in demo mode answers success without sending anything, so the pending
change clears on a demo commit and the feature can be walked through end to
end. A write that refuses instead leaves the change stuck in the journal with
no way to clear it but a discard, which is what the dataset write did until
#295. What the store holds is what the user keeps seeing; a refresh reads the
generator again, the same way a refresh on a real profile reads the server.

## Conventions

- Go: standard `gofmt`; package names lowercase; document exported identifiers.
- Backend logic lives in `internal/`; `app.go` only adapts it to Wails bindings.
- `internal/` is import-private to this module — keep it that way.
- Jira is the system of record. The local store is a cache plus a pending-change
  journal — never authoritative.
- Credentials (PAT) go to the Windows Credential Manager — never the database,
  never plaintext, never logs.
- Code markers: `TODO(xtm): desc` for planned work; reference the FR / phase.
  Live-Jira REST calls not yet verified against a real instance are stubbed and
  marked `NOTE`/`TODO(xtm)` in `internal/jira/`.

## Building, running, and testing

```powershell
wails dev                       # run with hot reload (Go + Vite)
wails build                     # produce build/bin/xray-test-manager.exe
go build ./...                  # compile-check the Go backend only
go test ./...                   # run all Go tests (the backend has broad coverage)
cd ../core; go test ./...       # the shared core's tests (profiles, settings, store runner)
go test ./internal/syncer/...   # one package
go test ./internal/testrepo/ -run TestImportTests   # one test by name
gofmt -w .                      # format

# Frontend (usually run via wails)
cd frontend; npm run build      # tsc typecheck + vite build
cd frontend; npx vitest run     # Vitest unit tests (query hooks + contexts)
cd frontend; npx tsc --noEmit   # typecheck only
```

Prerequisites: Go 1.25+, Node.js, and the Wails CLI
(`go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`).

`npm install` runs at the repo root, not in `frontend/`, since this is an npm
workspace now. The dialog system, `Modal`, `Menu`, `LiveRegion`, the API call
helpers, and the query client factory come from `@agile-suite/core`
(`frontend/core` at the repo root); XTM's own files re-export them.

Most backend behavior is verified by Go unit tests against the store and the
demo client. When changing backend logic, add or update the `_test.go` beside
it. The frontend has a Vitest suite (`frontend/src/**/*.test.ts{,x}`) covering
the TanStack Query hooks and the React contexts (`src/contexts/`); when changing
that logic, add or update the `.test.tsx` beside it.

## Releasing

Version is single-sourced in `wails.json` (`info.productVersion`).
`scripts/release-app.ps1 -App xtm -Version X.Y.Z`, run from the repository root,
stamps it, builds the portable exe, compiles the installer from
`build/windows/installer/installer.iss` with Inno Setup (needs `ISCC.exe`),
bundles the user guide, and writes `SHA256SUMS.txt` into `xtm/dist/`
(`-NoInstaller` skips the installer). The script serves every app in the
repository and reads the product and binary names from the app's `wails.json`,
so it carries no XTM literals; `-App tam` releases TAM the same way.

Pushing an `xtm/vX.Y.Z` tag triggers `.github/workflows/release.yml`, whose
`resolve` job reads the app out of the tag's prefix and hands the build jobs the
directory, product name and binary name. A `tam/vX.Y.Z` tag runs the same two
jobs for TAM. The instruction gate fails when an app carrying a `wails.json`
has no matching prefix, so a new app cannot be left unreleasable without
anything saying so. See README for the artifact table.

## Current status

**1.9.0 released; 1.10.0 in development.** Feature-complete for Xray on Jira DC
and in daily use: sync, fast browse/search/filter/sort, saved views,
configurable columns, local field/step/custom-field editing with on-commit sync
and conflict resolution, workflow transitions (single + bulk), all bulk
operations, Test Sets/Plans/Executions, Test Repository folders, preconditions,
requirements and coverage, defect tracking, duplicates, test calls,
misspellings, test review, CSV/XLSX import + export, a pytest scaffold
generator, a statistics dashboard with traceability Sankeys, Gap Analysis,
diagnostics, sync history, light/dark themes, and profile management.

**Live Xray/Jira REST is wired.** Every call in `internal/jira/` has a real
implementation behind the demo short-circuit. Instance-specific shapes (link
types and direction, custom-field names and value shapes, mandatory create
fields) are marked `NOTE(xtm)` where they still want verification against
another instance.

**More than one backend since 1.9.0.** The local store is a neutral hub rather
than an Xray-only cache. Kiwi TCMS is supported alongside Xray via the
`backend.Backend` interface, each backend advertises its capabilities, and a
migration bridge moves a dataset between connections.

**Zephyr Essential is the third backend, and it is partial.**
`internal/backend/zephyr` connects, declares its capabilities, and reads tests,
steps, cycles and run results. Writing anything Zephyr owns, and both custom
field namespaces, answer `ErrUnsupported` until the phase that fills them
(#371); `unsupported.go` groups them by phase and `absent.go` holds what
Essential does not have at all. It is Data Center only.

It is shaped differently from the other two: a Zephyr test is an ordinary Jira
issue, so the adapter holds a Jira client beside its Zephyr one and most reads
go through the Jira path XTM already has. Only cycles, executions, steps and
step results sit behind `/rest/zapi/latest`.

That Jira half is delegated to the Xray adapter method by method, in
`jiraside.go`, rather than restating its conversions. Delegation is explicit
and never by embedding, because the Xray adapter also answers Xray's own
endpoints under `/rest/raven`, which a Zephyr instance does not serve, and an
embedded adapter would satisfy the interface with those too. The boundary is a
test rather than a convention: `TestTheDelegatedMethodsOnlyTalkToJira` drives
every delegated method against a recording server and fails if one asks for
anything outside `/rest/api`.

Two things about it are worth knowing before touching it. The product names
collide, because SmartBear renamed both apps in April 2025 and Essential is
the former Squad, not the former Scale; `agents/project/glossary.md` has it.
And statuses are numeric ids configured per instance, so they are read at
connect time and never compiled in: the probed instance defines a sixth status
SmartBear does not document, and a hardcoded map would have written the wrong
result without failing.

See `CHANGELOG.md` for what has shipped and what is pending release.
