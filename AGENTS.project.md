# agile-suite Project Instructions

Read `AGENTS.md` first. Contracts, project rules, verification, playbooks.
The sanctioned path is the only path; a bypass is a bug even when it works.

Monorepo: three Go modules (`core`, `tam`, `xtm`) tied by `go.work`, three npm
workspaces (`frontend/core`, `tam/frontend`, `xtm/frontend`).

## Architecture contracts

### Credentials
Owns: storage and retrieval of every backend token (Jira PAT, Confluence PAT, Kiwi).
Path: the credential store reached through `core/profile`; ids from `ConfluenceCredentialID` and its siblings.
Never: a token in SQLite, in profile JSON, in an exported profile, or in a log line.
Gate: `frontend/core/src/instruction-gate.test.ts` (no token literal written to a database or log call); review for the rest.

### Store schema and purge
Owns: SQLite schema, migration order, and everything a profile owns locally.
Path: migrations in `core/store`; per-profile deletion in the two `PurgeProfile` implementations (`tam/internal/issuerepo/state.go` sweeps issue data, `tam/internal/boardrepo/boardrepo.go` sweeps board data); per-board deletion in `RemoveBoards`.
Never: a profile-keyed table absent from both purge lists; a column added to an existing table without a migration entry.
Gate: `frontend/core/src/instruction-gate.test.ts` (every profile-keyed table name appears in both purge lists).

### Generated bindings
Owns: the Wails binding layer, `tam/frontend/wailsjs` and `xtm/frontend/wailsjs`.
Path: regenerate with wails generate module from the app directory.
Never: hand-edit a file under a `wailsjs` directory.
Gate: CI regenerates and runs git diff --exit-code; three of the six most fix-churned files in this repo live here.

### Module privacy
Owns: what one module may import from another.
Path: shared code goes through `core`; an app module's `internal/` is private to it.
Never: importing `tam/internal` from `xtm`, or `xtm/internal` from `tam` or `core`.
Gate: `frontend/core/src/instruction-gate.test.ts` (zero cross-module internal imports).

### Frontend logging
Owns: diagnostics reaching a developer from the frontends.
Path: surface failures through component state and the error text the view already renders; the Go side logs through `log`.
Never: `console` calls in a workspace `src` tree.
Gate: the ratchet holds eslint_no_console.

### UI copy
Owns: user-visible strings in both frontends.
Path: plain text in the component that renders it.
Never: an em dash in a user-visible string.
Gate: the ratchet holds ui_em_dashes; comments and docs are exempt.

### Modals
Owns: dialogs, overlays, and their stacking against the app chrome.
Path: the primitives in `frontend/core/src/components/Modal.tsx` and the `useConfirm`, `usePrompt`, `useNotice` hooks beside it.
Never: bespoke modal, overlay, or backdrop markup in an app workspace.
Gate: the ratchet holds bespoke_modals; four fixes have been spent on modal layering and backgrounds.

### Family rows
Owns: how a row shows that it is a subtask of another row, in every table and tree.
Path: `RowLead` in `frontend/core/src/components`, the `--subtask-indent` token, and `familyPlace` in `tam/frontend/src/lib/issueFamilies.ts` for deciding whether a row is a root, a child, or a child whose parent is not drawn.
Never: an indent, a branch glyph, or a parent marker written into one table's own rules or markup.
Gate: `tam/frontend/src/styles.test.ts` (no table indents a subtask with a literal of its own, and the toggle does not size itself). Four tables each invented an indent, and the toggle was wider than it, so a subtask rendered less indented than its parent.

### Class names
Owns: the classes components set on elements.
Path: a rule in a stylesheet for every class, or no class.
Never: a class name no stylesheet defines. It renders unstyled and nothing reports it.
Gate: `frontend/core/src/instruction-gate.test.ts` (every class a component sets is defined in a stylesheet). `.row` left the rituals conflict buttons touching, and `.ritual-page-body` left an error fallback as bare HTML.

## Project rules

- Run Go commands from the module directory (`core`, `tam`, `xtm`); run npm commands from the repo root. Run `git config core.hooksPath .githooks` once per clone so the pre-commit gates exist.
- Prose people read (docs, commit bodies, PR and issue bodies, review comments) is written with the slop-mop skill; prose reviews run its detect mode on Opus or newer.
- Jira is the system of record. The local store is a cache plus a pending-change journal, never authoritative. Gate: review.
- Backend logic lives in `internal/`; `app.go` only adapts it to Wails bindings. Gate: review.
- Planned work is marked `TODO(tam)` or `TODO(xtm)` and names its phase or FR. Gate: the ratchet holds unscoped_todos.
- Go source is gofmt-clean. Gate: CI only — `core.autocrlf` makes `gofmt -l` flag every file on a Windows checkout.
- Release tags carry the app name: `xtm/v1.10.0`, `tam/v0.1.0`. The release workflow filters `xtm/v*`. Gate: review.
- `origin` pushes to GitHub and the Gitea mirror together. After a PR merges on GitHub, run `.\scripts\sync-remotes.ps1`. Gate: review.
- A `feat` PR links its spec or says in its `## Spec` section why it needs none. Gate: the `pr-acceptance` CI job.

## Verification

```
make gates                                  # tests, typecheck, vet, instruction gate, ratchet
go test ./... -count=1                      # from core, tam, or xtm
npm test --workspaces --if-present          # from the repo root
npm run typecheck --workspaces --if-present
npm run lint                                # writes eslint-report.json for the ratchet
go vet ./...                                # from each module
```

Per commit, run what the change touches; the full set before push or PR.
Ratchet baselines lower with `bash scripts/ratchet.sh --update`; raising one
needs a reason in the commit message (C7). CI also runs proven red (P2) and
`pr-acceptance`. State completed checks in handoff.

## Playbooks

Read each listed playbook before work in that area.

| Work | Read first |
|---|---|
| Naming, briefs, docs, proposing work | `agents/project/glossary.md`, `agents/project/out-of-scope.md` |
| Tests, UI, or platform checks | `agents/project/testing.md` |
| Developer or user documentation | `agents/project/documentation.md` |
| Jira and Confluence APIs and quirks | `agents/project/domain-context.md` |
| TAM feature history, phase by phase | `agents/project/tam-phases.md` |
| XTM architecture and demo mode | `agents/project/xtm-architecture.md` |

Portable playbooks live in `agents/generic/`; project ones in
`agents/project/`.
