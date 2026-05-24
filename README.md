# Xray Test Manager

A lightweight Windows desktop application for managing **Xray test cases in
Jira Data Center** at scale — built for QA teams whose projects hold 10,000+
test cases and have outgrown the Jira browser UI.

## Why

The Jira/Xray web interface becomes slow and cumbersome with very large test
suites — navigation, filtering, and especially bulk edits. Xray Test Manager
synchronises test data into a fast local store and gives testers a dedicated,
bulk-first interface, writing changes back to Jira on commit.

## Status

🚧 **Phase 0 — Foundations.** Project scaffold; backend skeletons in place.
See the roadmap below.

## Stack

- **Go** + **Wails v2** + **React / TypeScript**
- **SQLite** local store (pure-Go `modernc.org/sqlite` — no cgo)
- Single Windows executable — only WebView2 is required (built into Windows 11)
- Targets **Jira DC 8.14+** and **Xray Server / DC 8.4.0**

## Development

Prerequisites: Go 1.25+, Node.js, and the Wails CLI
(`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

```powershell
wails dev      # run with live reload
wails build    # build build/bin/xray-test-manager.exe
go build ./... # compile-check the Go backend only
```

## Demo mode

No Jira instance handy? Create a profile with **Jira base URL `demo`** (any
project key, any token). The backend short-circuits the sync and serves
~5,000 deterministically-generated tests so the full UI — sync progress,
browse, search, filter, sort, detail — can be exercised end to end. The
header shows a yellow `DEMO` chip while a demo profile is active.

## Roadmap

| Phase | Theme |
| --- | --- |
| 0 | Foundations — scaffold, local store, profiles, PAT auth |
| 1 | MVP — fast browse / search / filter (read-only) |
| 2 | Local editing + on-commit sync |
| 3 | Bulk operations |
| 4 | Workflow & dashboard |
| 5 | XLSX / CSV import |
| 6 | pytest helper, advanced features |

Full planning, requirements (FR-1…FR-13) and design notes are maintained in the
project's Outline documentation collection.
