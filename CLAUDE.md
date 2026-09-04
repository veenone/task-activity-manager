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
