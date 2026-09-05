# CLAUDE.md

This repository is the **agile-suite** monorepo: two desktop apps for Jira DC
that share a Go core.

- `xtm/`: Xray Test Manager. Read `xtm/CLAUDE.md` for everything about it;
  run Wails, Go tests, and the frontend from inside `xtm/`.
- `core/`: the shared Go spine (store runner, profiles, connections,
  settings, credentials). Added by packages only when an app needs them.
- `tam/`: Task Activity Manager (arrives with plan 0b).
- `docs/superpowers/`: design specs and implementation plans for the suite.

`go.work` at the root ties the modules together, so `go build ./...` and
`go test ./...` work from any module directory.

## Keeping xtm/ in step with its upstream

XTM is still developed in `veenone/xray-testcase-manager`. Pull its commits
in with `.\scripts\sync-xtm-upstream.ps1`, which merges upstream `main`
into `xtm/` with a subtree-shifted merge and leaves the result uncommitted.
Resolve conflicts (the shared-profile wiring in `xtm/app.go` is the usual
spot), run XTM's Go and Vitest suites, then commit. Changes upstream made
under its `docs/` or `.github/` land under `xtm/` and have to be moved to
the root by hand; the script warns when that happens.
