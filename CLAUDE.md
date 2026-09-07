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

Three remotes hold this repository and all three are equal:
`origin` (github.com/veenone/task-activity-manager), `xtm-origin`
(github.com/veenone/xray-testcase-manager, XTM's original home) and `gitea`
(the home Gitea, `achmarah/xray-test-manager`). `origin` pushes to all three,
so a plain `git push` lands everywhere; `.\scripts\sync-remotes.ps1 -Setup`
writes that configuration on a fresh clone (it also points `origin`'s fetch
URL at task-activity-manager, whichever repository the clone came from).

A pull request merged on GitHub lands on that one remote only. Run
`.\scripts\sync-remotes.ps1` afterwards: it fast-forwards `main` and tags on
whichever remotes are behind. It stops before pushing anything if the `main`
branches have diverged, and stops before touching tags if two remotes disagree
on one. Feature branches are not synchronised; they are pushed through the
fan-out and deleted where they were merged. Dependabot opens the same bump on
both GitHub repositories: merge it on task-activity-manager only and close the
copy on xray-testcase-manager, so the two `main` branches never diverge.

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
