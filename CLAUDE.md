# CLAUDE.md

This repository is the **agile-suite** monorepo: two desktop apps for Jira DC
that share a Go core.

- `xtm/`: Xray Test Manager. Read `xtm/CLAUDE.md` for everything about it;
  run Wails, Go tests, and the frontend from inside `xtm/`.
- `core/`: the shared Go spine (store runner, profiles, connections,
  settings, credentials, and the Jira transport in `core/jira`). Added by
  packages only when an app needs them.
- `tam/`: Task Activity Manager. Syncs a project's issues and shows them in
  the Backlog with a detail panel; read `tam/CLAUDE.md` for everything
  about it.
- `frontend/core`: the shared React package (`@agile-suite/core`) both
  frontends build on: dialog primitives, contexts, API helpers.
- `docs/superpowers/`: design specs and implementation plans for the suite.

`go.work` at the root ties the modules together, so `go build ./...` and
`go test ./...` work from any module directory.

## Remotes

Two remotes hold this repository: `origin`
(github.com/veenone/task-activity-manager) and `gitea` (the home Gitea,
`achmarah/xray-test-manager`), which mirrors it. `origin` pushes to both, so a
plain `git push` lands on each; `.\scripts\sync-remotes.ps1 -Setup` writes that
configuration on a fresh clone.

A pull request merged on GitHub lands on that one remote only. Run
`.\scripts\sync-remotes.ps1` afterwards: it fast-forwards `main` and tags on
whichever remote is behind. It stops before pushing anything if the two `main`
branches have diverged, and stops before touching tags if they disagree on one.
Feature branches are not synchronised; they are pushed through the fan-out and
deleted where they were merged.

**xray-testcase-manager is a separate project now, and this repository does not
push to it.** The two shared a history up to Phase 3a and diverged after it:
this one became the Task Activity Manager monorepo, and that one carried on
with its own Xray work, its own releases and its own `main`. Trying to keep the
two `main` branches in step stopped being bookkeeping and started being a
question about whose feature work wins, which is not a thing a sync script
should answer. The `xtm-origin` remote is still configured, for fetching and
for reading history; it is not in the push fan-out and the sync script ignores
it. If a change genuinely belongs to both, carry it across deliberately.

**Stacked pull requests need care on this repository.** A PR whose base is
another feature branch merges into that branch, not into `main`, so a stack
merged in order leaves every commit but the first sitting somewhere `main`
cannot see. Either merge the base down to `main` first and retarget each PR as
it comes up, or open the last one against `main` once the stack is complete.
This has already caught us once.

Release tags carry the app name: `xtm/v1.10.0` for XTM, `tam/v0.1.0` for TAM
once it ships. The release workflow filters on `xtm/v*`.

## Frontends

The three React packages are npm workspaces. Run `npm install` once at the
repo root; `npm test --workspaces --if-present` runs every Vitest suite and
`npm run typecheck --workspaces --if-present` type-checks them. Wails does the
root install itself through each app's `frontend:install`.

## gstack (recommended)

This project uses [gstack](https://github.com/garrytan/gstack) for AI-assisted workflows.
Install it for the best experience:

```bash
git clone --depth 1 https://github.com/garrytan/gstack.git ~/.claude/skills/gstack
cd ~/.claude/skills/gstack && ./setup --team
```

Skills like /qa, /ship, /review, /investigate, and /browse become available after install.
Use /browse for all web browsing. Use ~/.claude/skills/gstack/... for gstack file paths.
