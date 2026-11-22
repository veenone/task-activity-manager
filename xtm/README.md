# Xray Test Manager

A Windows and macOS desktop application for managing Xray test cases in Jira
Data Center. It is for QA teams whose projects hold 10,000+ test cases and have
outgrown the Jira browser UI.

## Why

With very large test suites the Jira/Xray web interface gets slow to navigate
and filter, and slowest at bulk edits. Xray Test Manager syncs test data into a
local store, gives testers an interface built around bulk edits, and writes
changes back to Jira on commit.

## Status

1.10.0 is released. It has every planned feature for Xray on Jira DC and is in
daily use. Sync, fast browse/search/filter/sort, saved
views, configurable columns, local field/step/custom-field editing with
on-commit sync and conflict resolution, workflow transitions, all bulk
operations, Test Sets / Plans / Executions, Test Repository folders,
preconditions, requirements and coverage, defect tracking, duplicates, test
calls, misspellings, a statistics dashboard with traceability Sankeys, Gap
Analysis, CSV/XLSX import and export, a pytest scaffold generator, diagnostics,
sync history, light/dark themes, and profile management.

Live Xray/Jira REST is wired. Every call in `internal/jira/` has a real
implementation behind the demo short-circuit. Instance-specific shapes (link
types and direction, custom-field names and value shapes, mandatory create
fields) are marked `NOTE(xtm)` where they still want verification against
another instance. Every feature also works in demo mode.

Since 1.9.0 the local store holds data for more than one backend, where it used
to be an Xray-only cache. Kiwi TCMS is supported alongside Xray, and each
backend advertises what it can do. A migration bridge moves a dataset from one
connection to another, after a pre-flight check that lists what the target
backend cannot hold.

See [CHANGELOG.md](CHANGELOG.md) for what has shipped and what is pending
release.

## Stack

- Go, Wails v2, React and TypeScript
- SQLite local store (pure-Go `modernc.org/sqlite`, no cgo)
- **Windows:** a single `.exe`; only WebView2 is required (built into Windows 11)
- **macOS:** a universal `.app` (Apple Silicon + Intel); uses the built-in WKWebView
- Secrets in the OS-native store: Windows Credential Manager or macOS Keychain
- Targets **Jira DC 8.14+** and **Xray Server / DC 8.4.0**

## Development

Prerequisites: Go 1.25+, Node.js, and the Wails CLI
(`go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`).

```sh
wails dev                              # run with live reload
wails build                            # Windows: build/bin/xray-test-manager.exe
wails build -platform darwin/universal # macOS:   build/bin/xray-test-manager.app
go build ./...                         # compile-check the Go backend only
```

## Releasing & distribution

Releases ship Windows and macOS artifacts plus checksums:

| Artifact | Use |
| --- | --- |
| `xray-test-manager-<ver>-windows-amd64.exe` | Windows portable; run it directly, no install |
| `xray-test-manager-<ver>-windows-amd64-installer.exe` | Windows Inno Setup installer (Start-menu entry, uninstaller) |
| `xray-test-manager-<ver>-macos-universal.zip` | macOS universal `.app` (Apple Silicon + Intel) |
| `xray-test-manager-<ver>-user-guide.zip` | User guide (markdown + screenshots) |
| `SHA256SUMS.txt` / `SHA256SUMS-macos.txt` | Integrity check for the above |

> **macOS Gatekeeper:** by default the `.app` is unsigned, so on first launch
> macOS may block it. Right-click the app → **Open** (then confirm), or run
> `xattr -dr com.apple.quarantine xray-test-manager.app` to clear the quarantine flag.
>
> **Signing & notarization (optional):** the `release-macos` job code-signs and
> notarizes the `.app` automatically when these repo secrets are set. Without
> them it publishes the unsigned build. Set `MACOS_CERTIFICATE_P12` (base64 of
> a *Developer ID Application* `.p12`), `MACOS_CERTIFICATE_PASSWORD`,
> `MACOS_SIGNING_IDENTITY`, and for notarization `MACOS_NOTARY_APPLE_ID`,
> `MACOS_NOTARY_TEAM_ID`, and `MACOS_NOTARY_PASSWORD` (an app-specific password).

**Build locally** (`scripts/release-app.ps1` builds, version-stamps, bundles
into `xtm/dist/`, and writes checksums). Run it from the repository root; it
serves every app here and takes the app as a parameter:

```powershell
# Stamp + build the portable exe and the installer (needs Inno Setup / ISCC.exe)
./scripts/release-app.ps1 -App xtm -Version 0.2.0

# Portable exe only (skip the installer)
./scripts/release-app.ps1 -App xtm -Version 0.2.0 -NoInstaller
```

The version is set in one place, `wails.json` (`info.productVersion`).
The script stamps it and Wails bakes it into the installer and the exe metadata.

**Cut a GitHub release**: push an `xtm/v*` tag and CI (`.github/workflows/release.yml`
at the repository root) runs two jobs: `release-windows` (on `windows-latest`)
builds the installer and portable exe, and `release-macos` (on `macos-latest`)
builds the universal `.app`. Both publish to the same GitHub Release. The
prefix says which app the tag belongs to, and the workflow's `resolve` job
turns it into the directory, product name and binary name both jobs use; a bare
`v1.10.0` builds nothing. (`scripts/release-app.ps1` is Windows-only; the macOS
`.app` is built directly with `wails build`.)

```powershell
git tag xtm/v1.10.0
git push origin xtm/v1.10.0
```

## Demo mode

To try the app without Jira, create a profile with **Jira base URL `demo`** (any
project key, any token). The backend skips the sync and serves about 5,000
deterministically generated tests, plus sample Test Sets, Plans and Executions,
folders and preconditions, so you can use the whole UI end to end. The header shows a yellow `DEMO` chip while a demo profile is active.

## Roadmap

| Phase | Theme | Status |
| --- | --- | --- |
| 0 | Foundations: scaffold, local store, profiles, PAT auth | ✅ |
| 1 | MVP: fast browse / search / filter / sort, saved views, columns | ✅ |
| 2 | Local editing (fields / steps / custom fields) + on-commit sync, conflict resolution | ✅ |
| 3 | Bulk operations (edit / transition / allocate / move / preconditions / review) | ✅ |
| 4 | Workflow transitions, statistics dashboard, traceability Sankey | ✅ |
| 5 | XLSX / CSV import + export | ✅ |
| 6 | pytest scaffold, containers & folder CRUD, test review, themes, diagnostics | ✅ |
| 7 | Live Xray/Jira REST wiring (wired; verify against a real instance) | ✅ |
| 8 | Coverage module, requirements suite, Gap Analysis | ✅ |
| 9 | Backend-agnostic core, Kiwi TCMS, migration bridge | ✅ |

Beyond the phase plan, current work is tracked in
[CHANGELOG.md](CHANGELOG.md) and in Jira (`RND_P_4TFINT_05`, component
*Xray-test-management*).

Full planning, requirements (FR-1…FR-13) and design notes are maintained in the
project's Outline documentation collection.

## License

This project is licensed under the **GNU General Public License v3.0** (GPL-3.0).
See the [LICENSE](LICENSE) file for the full text.

Copyright (C) 2025-2026 Achmad Fienan Rahardianto.

This program is free software: you can redistribute it and/or modify it under the
terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version. It is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.
