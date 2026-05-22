# CLAUDE.md

Guidance for Claude Code when working in this repository.

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

## Stack

- **Go** backend + **Wails v2** desktop shell + **React + TypeScript** frontend.
- **SQLite** local store via the pure-Go `modernc.org/sqlite` driver (no cgo).
- Single distributable **Windows** executable; WebView2 is the only runtime
  prerequisite.
- Target server: **Jira DC 8.14+** (Personal Access Tokens) and **Xray
  Server / DC 8.4.0**.

## Layout

```
main.go              Wails entry point, window options, bound App
app.go               App struct — backend wired to the React frontend
internal/
  jira/              Jira DC + Xray Server REST client
  store/             SQLite local store and schema
  profile/           Connection profiles + OS credential storage
  testrepo/          Local Test repository — browse / search / filter queries
  syncer/            Pull-sync engine — Jira -> local store
frontend/            React + TypeScript (Vite), rendered in WebView2
build/               Wails build assets and output
```

## Conventions

- Go: standard `gofmt`; package names lowercase; document exported identifiers.
- Backend logic lives in `internal/`; `app.go` only adapts it to Wails bindings.
- `internal/` is import-private to this module — keep it that way.
- Jira is the system of record. The local store is a cache plus a pending-change
  journal — never authoritative.
- Credentials (PAT) go to the Windows Credential Manager — never the database,
  never plaintext, never logs.
- Code markers: `TODO(xtm): desc` for planned work; reference the FR / phase.

## Building and running

```powershell
wails dev      # run with hot reload
wails build    # produce build/bin/xray-test-manager.exe
go build ./... # compile-check the Go backend only
```

## Current status

Phase 1 backend. Profiles and Windows credential storage are implemented; the
local store caches Test data; `jira.Client` does paginated Test search;
`syncer.Engine` runs a full pull sync; `testrepo.Repository` serves the
browse / search / filter queries. The React browse grid is the next slice.
