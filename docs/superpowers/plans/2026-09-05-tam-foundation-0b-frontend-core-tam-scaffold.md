# Task Activity Manager foundation 0b: shared frontend core and the TAM scaffold

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up Task Activity Manager as a second Wails app in the monorepo, with its own database, the shared profile store, and a shell built from a new `frontend/core` package that XTM also imports, plus the housekeeping the split needs: a way to pull XTM's upstream commits in, a guard against the shared schema drifting from XTM's tables, and per-app settings that cannot collide.

**Architecture:** `frontend/core` is an npm workspace package (`@agile-suite/core`) holding the dialog system, the modal and view state factories, a profile provider with an injected backend adapter, the theme helper, the query client factory, and the design tokens plus primitive styles. XTM keeps every file it has today; the moved ones become one-line re-exports so no consumer changes. `tam/` is a Go module with `main.go`, a thin `app.go`, a `tamstore` package (schema version 1, no app tables yet), and a `suiteprofiles` package that decides which shared profiles TAM shows. `tam/frontend` renders the shell from the mockup with placeholder views and a working Profiles dialog. A PowerShell script performs the subtree-shifted merge that keeps `xtm/` following its upstream repository.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4, npm workspaces (npm 10, Node 20+).

**Mockups:** [`../specs/assets/2026-09-05-tam-scaffold-shell.svg`](../specs/assets/2026-09-05-tam-scaffold-shell.svg) shows exactly what this plan ships: the shell with the Backlog placeholder and the Profiles dialog open. [`../specs/assets/2026-09-04-tam-shell-backlog.svg`](../specs/assets/2026-09-04-tam-shell-backlog.svg) is where the shell is heading.

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, and (new) `agile-suite/tam`, each app module carrying `replace agile-suite/core => ../core`. See decision 1 below.
- XTM is never edited for TAM's sake beyond import paths, except the edits this plan names: the re-export shims in Task 5, the `errMsg` and `applyTheme` imports in Task 5, `xtm/wails.json`'s install command in Task 5, and the `sharedmigrate` refactor in Task 2.
- Every task leaves XTM's Go suite (`go test ./internal/...` inside `xtm/`) and the Vitest suites green. After Task 5, the XTM and core Vitest suites together must pass at least the 196 tests XTM passes today, and every test file this plan moves must run from its new home.
- `core` (Go) and `frontend/core` hold only what a task in this plan needs. No speculative packages, hooks, or components. `SelectionContext`, `SyncContext`, `syncMachine`, `viewState`, and the query-key helpers stay in XTM until Phase 1 reaches for them.
- The Windows Credential Manager prefix `xray-test-manager:` and the keyring service name `Xray Test Manager` stay as they are. TAM stores tokens through the same `core/profile` credential store, so a PAT saved in either app is readable by both.
- Each app keeps its own SQLite file. TAM's is `<user config dir>/task-activity-manager/tam.db`. Only `profiles.db` is shared, at `<user config dir>/agile-suite/profiles.db`.
- Frontends live inside their module (`xtm/frontend/`, `tam/frontend/`) because Wails embeds `frontend/dist` relative to `main.go`.
- UI text uses no em dashes.
- No AI attribution or mentions in any commit message or PR. Run the humanizer pass over prose, including code comments.
- Commit messages use the repo's conventional prefixes (`chore:`, `feat:`, `refactor:`, `docs:`, `ci:`, `test:`) with no trailers.

## Decisions

1. **Module paths stay `agile-suite/*`.** Renaming to `github.com/veenone/task-activity-manager/...` was considered as a way to drop the `replace` directive. It would not: without `replace`, `go mod tidy` runs in module mode (it ignores `go.work`) and would try to fetch `core` from GitHub. The `replace` is what keeps `tidy` and non-workspace builds working, so it stays, and the rename waits until the repository name is final.
2. **npm workspaces at the repo root.** A root `package.json` lists `frontend/core`, `xtm/frontend`, and `tam/frontend`; npm hoists one copy of React, Vite, and Vitest to the root `node_modules`. `@agile-suite/core` is source only (its `exports` point at `.ts` files); Vite serves and bundles it like app code, and each app's `tsc` type-checks it as part of its program.
3. **Pull-based, with shims.** Only what TAM's shell uses moves. Every XTM file that moves is replaced by a re-export of the same names from `@agile-suite/core`, so XTM's fifty-odd consumers keep their imports. XTM's `ProfileContext` and `NavContext` stay in XTM: they carry XTM-only state (`showCoverage`, the browse selections, the New Test panel).
4. **Styles are copied, not cut.** `frontend/core` ships `styles/tokens.css` (the two token blocks from XTM's `App.css`) and `styles/primitives.css` (the rules for the moved components). XTM's `App.css` keeps its copies for now: extracting rules from a 7,800-line cascade is XTM-side work with no TAM benefit, and it belongs to the change where XTM's chrome adopts the shared shell.
5. **Profiles across apps.** TAM lists every shared profile except those with backend `kiwi` (Kiwi TCMS is not Jira), and creates profiles with backend `jira`. XTM's `newBackend` routes any value other than `kiwi` to its Jira/Xray client, so a TAM-created profile works in XTM against a Jira DC with Xray without any XTM change.
6. **Settings scopes.** Keys both apps read (`theme`, `default_profile_id`, `requirement_link_type`) stay unprefixed on `settings.Manager`. App-specific keys go through `settings.Scope`, which stores `<app>.<key>`. XTM's existing app-specific keys (`show_coverage`, `spellcheck_ignore_words`, `tour_seen_version`) stay unprefixed; renaming rows XTM already wrote is an XTM-side migration for later, and TAM never uses those names.
7. **Upstream sync by subtree-shifted merge.** The monorepo's history contains every XTM commit up to the restructure, so `git merge -X subtree=xtm xtm-upstream/main` gives Git a real merge base and lines the trees up. A script wraps it and warns when upstream touched `docs/` or `.github/`, which live at the root here.
8. **TAM schema version 1 has no app tables.** `tamstore` exists so the app has a database, a version, and diagnostics from day one. The `issue` table arrives with the Phase 1 spec.

---

## File structure

**Created**

- `scripts/sync-xtm-upstream.ps1` (repo root) - the upstream merge helper (Task 1).
- `xtm/internal/sharedmigrate/schema_drift_test.go` - the guard test (Task 2).
- `core/settings/scope.go`, `core/settings/scope_test.go` - per-app settings (Task 3).
- `tam/go.mod`, `tam/main.go`, `tam/app.go`, `tam/wails.json`, `tam/internal/tamstore/`, `tam/internal/suiteprofiles/` (Task 4).
- `package.json` (repo root), `frontend/core/**` (Task 5).
- `tam/frontend/**` (Task 6).
- `tam/CLAUDE.md` (Task 7).

**Moved** (Task 5, with `git mv` so history follows; each old path becomes a re-export shim)

- `xtm/frontend/src/contexts/DialogContext.tsx` and `DialogContext.test.tsx` → `frontend/core/src/contexts/`.
- `xtm/frontend/src/components/Modal.tsx`, `LiveRegion.tsx`, `Menu.tsx`, `useNotice.tsx`, `useConfirm.tsx`, `usePrompt.tsx` → `frontend/core/src/components/`.
- `xtm/frontend/src/lib/apiCall.ts`, `apiError.ts`, `apiError.test.ts` → `frontend/core/src/lib/`.

**Modified**

- `go.work` (adds `./tam`), `.gitignore`, `.github/workflows/build.yml`, `.github/dependabot.yml`.
- `xtm/internal/sharedmigrate/sharedmigrate.go` (column lists become data, Task 2).
- `xtm/frontend/package.json`, `xtm/wails.json`, `xtm/frontend/src/api.ts`, `xtm/frontend/src/contexts/ProfileContext.tsx`, `xtm/frontend/src/contexts/ModalContext.tsx`, `xtm/frontend/src/lib/queryClient.ts` (Task 5).
- `CLAUDE.md`, `README.md` (root), `xtm/CLAUDE.md`, `xtm/CHANGELOG.md` (Task 7).

---

### Task 1: The upstream XTM sync script

XTM keeps developing in `veenone/xray-testcase-manager`. This task gives the monorepo a repeatable way to pull those commits into `xtm/` and documents the procedure. It ships the tool only; the first real sync is its own PR.

**Files:**
- Create: `scripts/sync-xtm-upstream.ps1`
- Modify: `CLAUDE.md` (root), `README.md` (root)

**Interfaces:**
- Produces: nothing other tasks consume.

- [ ] **Step 1: Write the script**

Create `scripts/sync-xtm-upstream.ps1`:

```powershell
<#
Pulls the latest Xray Test Manager commits from its own repository into xtm/.

XTM is still developed in veenone/xray-testcase-manager. This repository
carries a copy under xtm/ that has to follow it. Both histories share every
commit up to the monorepo restructure, so a subtree-shifted merge gives Git a
real merge base and lines the two trees up.

Usage:
  .\scripts\sync-xtm-upstream.ps1            # merge upstream main, leave it uncommitted
  .\scripts\sync-xtm-upstream.ps1 -Branch dev

Afterwards: resolve any conflicts, run the XTM suites, then commit.
#>
param(
    [string]$Remote = "xtm-upstream",
    [string]$Url = "git@github.com:veenone/xray-testcase-manager.git",
    [string]$Branch = "main"
)

$ErrorActionPreference = "Stop"
$root = (git rev-parse --show-toplevel).Trim()
Set-Location $root

if ((git status --porcelain).Length -ne 0) {
    throw "The working tree has uncommitted changes. Commit or stash them first."
}

$remotes = git remote
if ($remotes -notcontains $Remote) {
    git remote add $Remote $Url
}
git fetch $Remote $Branch

git merge -X subtree=xtm --no-commit --no-ff "$Remote/$Branch"
$mergeExit = $LASTEXITCODE

# Files that sit at the root of XTM's repository (docs/, .github/) get shifted
# under xtm/ by the subtree merge. This repository keeps them at the root.
foreach ($dir in @("docs", ".github")) {
    $shifted = Join-Path "xtm" $dir
    if (Test-Path $shifted) {
        Write-Warning "Upstream changed $dir and the merge put it under $shifted. Move those changes into $dir before committing."
    }
}

if ($mergeExit -ne 0) {
    Write-Host "The merge stopped on conflicts. Resolve them, then run the suites and commit."
} else {
    Write-Host "Merged without conflicts. Nothing is committed yet."
}
Write-Host "Check: cd xtm; go test ./internal/...; cd frontend; npx vitest run"
```

- [ ] **Step 2: Dry-run it against the real upstream**

Run from the repo root in PowerShell:

```powershell
.\scripts\sync-xtm-upstream.ps1
git status --short | Select-Object -First 20
```

Expected: either "Already up to date" from Git (upstream has nothing new) or a set of staged changes under `xtm/`. This task does not ship a sync, so undo whatever the dry run staged:

```powershell
git merge --abort
git status --short
```

Expected: an empty status. If `git merge --abort` reports there is no merge to abort, the dry run was already clean.

- [ ] **Step 3: Document the procedure**

In the root `CLAUDE.md`, add after the bullet list:

```markdown
## Keeping xtm/ in step with its upstream

XTM is still developed in `veenone/xray-testcase-manager`. Pull its commits
in with `.\scripts\sync-xtm-upstream.ps1`, which merges upstream `main`
into `xtm/` with a subtree-shifted merge and leaves the result uncommitted.
Resolve conflicts (the shared-profile wiring in `xtm/app.go` is the usual
spot), run XTM's Go and Vitest suites, then commit. Changes upstream made
under its `docs/` or `.github/` land under `xtm/` and have to be moved to
the root by hand; the script warns when that happens.
```

In the root `README.md`, add a matching short section titled "Syncing XTM from upstream" with the same command and the two follow-up steps (resolve, run the suites).

- [ ] **Step 4: Commit**

```bash
git add scripts/sync-xtm-upstream.ps1 CLAUDE.md README.md
git commit -m "chore: add the script that merges XTM's upstream repository into xtm/"
```

---

### Task 2: Guard the shared schema against drift from XTM's tables

`core/shareddb` was lifted from XTM's `profiles`, `connection`, and `app_setting` tables, and the one-time import copies them column by column. XTM upstream keeps evolving those tables. This task turns the import's column lists into data and adds a test that fails when XTM's table has a column the shared schema or the import does not know.

**Files:**
- Modify: `xtm/internal/sharedmigrate/sharedmigrate.go`
- Create: `xtm/internal/sharedmigrate/schema_drift_test.go`
- Test: `xtm/internal/sharedmigrate/sharedmigrate_test.go` (existing, must keep passing)

**Interfaces:**
- Produces: `sharedmigrate.Tables` (`[]string`, the import order) and `sharedmigrate.Columns` (`map[string][]string`), read by the new test.

- [ ] **Step 1: Write the failing test**

Create `xtm/internal/sharedmigrate/schema_drift_test.go`:

```go
package sharedmigrate

import (
	"database/sql"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"agile-suite/core/shareddb"
	xtmstore "agile-suite/xtm/internal/store"
)

// The shared database was lifted from XTM's own tables, and XTM upstream keeps
// changing them. This test fails as soon as XTM's copy of a table has a column
// the shared schema lacks, or the import stops naming every column, so the
// drift shows up in CI instead of as a setting that silently never reaches the
// shared file.
func TestSharedSchemaTracksXTMTables(t *testing.T) {
	dir := t.TempDir()
	src, err := xtmstore.Open(filepath.Join(dir, "xtm.db"))
	if err != nil {
		t.Fatalf("open xtm store: %v", err)
	}
	defer src.Close()
	dst, err := shareddb.Open(filepath.Join(dir, "profiles.db"))
	if err != nil {
		t.Fatalf("open shared db: %v", err)
	}
	defer dst.Close()

	for _, table := range Tables {
		xtmCols := tableColumns(t, src.DB(), table)
		sharedCols := tableColumns(t, dst.DB(), table)
		for _, c := range xtmCols {
			if !contains(sharedCols, c) {
				t.Errorf("%s: XTM has column %q but core/shareddb does not; add it to shareddb.Schema and to sharedmigrate.Columns", table, c)
			}
		}
		imported := append([]string(nil), Columns[table]...)
		sort.Strings(imported)
		want := append([]string(nil), xtmCols...)
		sort.Strings(want)
		if strings.Join(imported, ",") != strings.Join(want, ",") {
			t.Errorf("%s: import copies %v but XTM's table has %v", table, imported, want)
		}
	}
}

func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info %s: %v", table, err)
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}
	return cols
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run it to see it fail to compile**

Run (inside `xtm/`): `go test ./internal/sharedmigrate/ -run TestSharedSchemaTracksXTMTables`
Expected: FAIL, `undefined: Tables` and `undefined: Columns`.

- [ ] **Step 3: Make the column lists data**

In `xtm/internal/sharedmigrate/sharedmigrate.go`, add below `markerKey`:

```go
// Tables lists the shared tables in import order. Columns names every column
// the import copies for each; the drift test compares it with XTM's own
// table definitions.
var Tables = []string{"profiles", "connection", "app_setting"}

var Columns = map[string][]string{
	"profiles": {
		"id", "name", "jira_url", "project_key", "created_at", "scope_jql",
		"bug_issue_type", "bug_project_mode", "bug_project_key", "ca_cert",
		"allow_untrusted_tls", "backend", "cross_project_sources",
	},
	"connection": {
		"id", "workspace_id", "name", "backend", "url", "project_key", "scope_jql",
		"bug_issue_type", "bug_project_mode", "bug_project_key", "ca_cert",
		"allow_untrusted_tls", "role", "created_at",
	},
	"app_setting": {"key", "value"},
}

// copySQL builds the SELECT and INSERT OR IGNORE statements for one table from
// its column list.
func copySQL(table string) (selectSQL, insertSQL string, n int) {
	cols := Columns[table]
	list := strings.Join(cols, ", ")
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	return "SELECT " + list + " FROM " + table,
		"INSERT OR IGNORE INTO " + table + " (" + list + ") VALUES (" + marks + ")",
		len(cols)
}
```

Add `"strings"` to the imports. Replace the three `copyRows` calls in `ImportFromStore` with one loop:

```go
	for _, table := range Tables {
		selectSQL, insertSQL, n := copySQL(table)
		if err := copyRows(tx, src, selectSQL, insertSQL, n); err != nil {
			return fmt.Errorf("copy %s: %w", table, err)
		}
	}
```

- [ ] **Step 4: Run the package tests**

Run (inside `xtm/`): `go test ./internal/sharedmigrate/ -v`
Expected: PASS for the existing import tests and for `TestSharedSchemaTracksXTMTables`.

- [ ] **Step 5: Prove the guard bites**

Temporarily remove `"cross_project_sources"` from `Columns["profiles"]`, run `go test ./internal/sharedmigrate/ -run TestSharedSchemaTracksXTMTables`, and confirm it fails with the "import copies ... but XTM's table has ..." message. Put the column back and rerun to green. Do not commit the temporary edit.

- [ ] **Step 6: Commit**

```bash
git add xtm/internal/sharedmigrate/
git commit -m "test(xtm): fail when the shared profile schema drifts from XTM's tables"
```

---
### Task 3: Per-app settings scopes in `core/settings`

Both apps write to the same `app_setting` table. This task adds `settings.Scope`, which prefixes app-specific keys with the app id, so TAM's preferences never overwrite XTM's. The shared keys stay where they are.

**Files:**
- Create: `core/settings/scope.go`, `core/settings/scope_test.go`
- Test: `core/settings/settings_test.go` (existing helpers `newManager`, must keep passing)

**Interfaces:**
- Produces: `func (m *Manager) Scope(app string) (*Scope, error)`, `func (s *Scope) Get(key string) (string, error)`, `func (s *Scope) Set(key, value string) error`. TAM's `app.go` (Task 4) does not call these yet; the first TAM-only preference will. Nothing in this plan writes a scoped key, which is why the type stays this small.

- [ ] **Step 1: Write the failing tests**

Create `core/settings/scope_test.go`:

```go
package settings_test

import "testing"

func TestScopeKeysDoNotCollideAcrossApps(t *testing.T) {
	m := newManager(t)
	tam, err := m.Scope("tam")
	if err != nil {
		t.Fatalf("scope tam: %v", err)
	}
	xtm, err := m.Scope("xtm")
	if err != nil {
		t.Fatalf("scope xtm: %v", err)
	}
	if err := tam.Set("tour_seen_version", "3"); err != nil {
		t.Fatalf("tam set: %v", err)
	}
	if err := xtm.Set("tour_seen_version", "1"); err != nil {
		t.Fatalf("xtm set: %v", err)
	}
	if got, _ := tam.Get("tour_seen_version"); got != "3" {
		t.Errorf("tam value = %q, want 3", got)
	}
	if got, _ := xtm.Get("tour_seen_version"); got != "1" {
		t.Errorf("xtm value = %q, want 1", got)
	}
	// The unprefixed key XTM's Manager reads today is untouched.
	s, err := m.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.TourSeenVersion != 0 {
		t.Errorf("unprefixed tour_seen_version = %d, want 0", s.TourSeenVersion)
	}
}

func TestScopeUnsetKeyReadsEmpty(t *testing.T) {
	m := newManager(t)
	sc, err := m.Scope("tam")
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	got, err := sc.Get("missing")
	if err != nil || got != "" {
		t.Errorf("Get(missing) = %q, %v; want \"\", nil", got, err)
	}
}

func TestScopeRejectsBadAppIDs(t *testing.T) {
	m := newManager(t)
	for _, app := range []string{"", "a.b", "."} {
		if _, err := m.Scope(app); err == nil {
			t.Errorf("Scope(%q) accepted, want error", app)
		}
	}
}
```

- [ ] **Step 2: Run them to see the compile failure**

Run (inside `core/`): `go test ./settings/ -run 'TestScope'`
Expected: FAIL, `m.Scope undefined`.

- [ ] **Step 3: Implement `Scope`**

Create `core/settings/scope.go`:

```go
package settings

import (
	"errors"
	"strings"
)

// Scope reads and writes the settings one app owns. Two apps share the same
// app_setting table, so an app-specific key is stored as "<app>.<key>" and
// cannot overwrite the other app's preference of the same name. Settings both
// apps read (the theme, the default profile, the requirement link type) stay
// on Manager without a prefix.
type Scope struct {
	m   *Manager
	app string
}

// Scope returns the settings namespace for app, a short fixed identifier such
// as "tam". It must be non-empty and must not contain a dot.
func (m *Manager) Scope(app string) (*Scope, error) {
	if app == "" || strings.Contains(app, ".") {
		return nil, errors.New("settings scope: app id must be non-empty and contain no dot")
	}
	return &Scope{m: m, app: app}, nil
}

// Get returns the value stored for key in this scope, "" when unset.
func (s *Scope) Get(key string) (string, error) {
	return s.m.value(s.app + "." + key)
}

// Set stores value for key in this scope.
func (s *Scope) Set(key, value string) error {
	return s.m.setValue(s.app+"."+key, value)
}
```

- [ ] **Step 4: Run the package tests**

Run (inside `core/`): `go test ./settings/ -v`
Expected: PASS, including the three new tests and every existing one.

- [ ] **Step 5: Commit**

```bash
git add core/settings/scope.go core/settings/scope_test.go
git commit -m "feat(core): give each app its own settings scope in the shared table"
```

---

### Task 4: The TAM Go application

Create the `tam` module: its own database, the shared profiles, a thin bound `App`, and the two internal packages the app needs. The frontend arrives in Task 6; this task verifies the Go side with a placeholder `frontend/dist` so `go build` can embed something.

**Files:**
- Create: `tam/go.mod`, `tam/main.go`, `tam/app.go`, `tam/wails.json`, `tam/internal/tamstore/tamstore.go`, `tam/internal/tamstore/tamstore_test.go`, `tam/internal/suiteprofiles/suiteprofiles.go`, `tam/internal/suiteprofiles/suiteprofiles_test.go`
- Modify: `go.work`, `.gitignore`

**Interfaces:**
- Consumes: `core/store.Open`, `store.Schema`, `store.ReadSchemaVersion`; `core/shareddb.Open`, `shareddb.DefaultPath`; `core/profile.NewManager`, `Manager.List/Create/Delete`, `profile.NewCredentialStore`; `core/settings.NewManager`, `Manager.Get/SetTheme/SetDefaultProfileID`.
- Produces (bound methods the frontend calls in Task 6, exact signatures): `Health() HealthInfo`, `GetDiagnostics() Diagnostics`, `ListProfiles() ([]profile.Profile, error)`, `CreateProfile(name, jiraURL, projectKey, token string, makeDefault bool) (profile.Profile, error)`, `DeleteProfile(id string) error`, `GetSettings() (settings.Settings, error)`, `SetTheme(theme string) error`, `SetDefaultProfile(id string) error`.

- [ ] **Step 1: Module, workspace, ignore rules**

Create `tam/go.mod`:

```
module agile-suite/tam

go 1.25.0

require (
	agile-suite/core v0.0.0
	github.com/wailsapp/wails/v2 v2.15.0
)

replace agile-suite/core => ../core
```

Edit `go.work` so the `use` block reads:

```
use (
	./core
	./tam
	./xtm
)
```

Append to `.gitignore` under the Wails section:

```
tam/build/bin
tam/frontend/dist
```

- [ ] **Step 2: Write the failing store test**

Create `tam/internal/tamstore/tamstore_test.go`:

```go
package tamstore_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/tamstore"
)

func TestOpenRecordsVersionOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	v, err := store.ReadSchemaVersion(db.DB())
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if v != tamstore.Schema.Version {
		t.Errorf("version = %d, want %d", v, tamstore.Schema.Version)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Reopening a current database is a no-op.
	again, err := tamstore.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_ = again.Close()
}
```

- [ ] **Step 3: Implement `tamstore`**

Create `tam/internal/tamstore/tamstore.go`:

```go
// Package tamstore is Task Activity Manager's own database: everything that
// is not a profile, connection, or global setting, because those live in the
// shared profiles.db. Version 1 carries no app tables. The issue tables arrive
// with the Phase 1 spec as migration 2.
package tamstore

import (
	"os"
	"path/filepath"

	"agile-suite/core/store"
)

// Schema is TAM's database layout.
var Schema = store.Schema{Version: 1}

// Open opens (or creates) TAM's database at path.
func Open(path string) (*store.DB, error) { return store.Open(path, Schema) }

// DefaultDir is <user config dir>/task-activity-manager, created if missing.
// The log file lives there too.
func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "task-activity-manager")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	return appDir, nil
}

// DefaultPath is DefaultDir joined with tam.db.
func DefaultPath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tam.db"), nil
}
```

Run (inside `tam/`): `go test ./internal/tamstore/ -v`
Expected: PASS.

- [ ] **Step 4: Write the failing profile-rule tests**

Create `tam/internal/suiteprofiles/suiteprofiles_test.go`:

```go
package suiteprofiles_test

import (
	"testing"

	"agile-suite/core/profile"
	"agile-suite/tam/internal/suiteprofiles"
)

func TestVisibleDropsKiwiProfiles(t *testing.T) {
	in := []profile.Profile{
		{ID: "a", Backend: "xray"},
		{ID: "b", Backend: "kiwi"},
		{ID: "c", Backend: "Kiwi"},
		{ID: "d", Backend: ""},
		{ID: "e", Backend: "jira"},
	}
	got := suiteprofiles.Visible(in)
	want := []string{"a", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("got %d profiles, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("got[%d] = %s, want %s", i, got[i].ID, id)
		}
	}
}

func TestIsDemoURL(t *testing.T) {
	cases := map[string]bool{
		"demo":                  true,
		" DEMO ":                true,
		"demo:pkcs":             true,
		"demo-agile":            true,
		"https://jira.acme.example": false,
		"":                      false,
		"kiwi-demo":             false,
	}
	for url, want := range cases {
		if got := suiteprofiles.IsDemoURL(url); got != want {
			t.Errorf("IsDemoURL(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestValidateNew(t *testing.T) {
	if err := suiteprofiles.ValidateNew("Team", "demo", "DEMO", ""); err != nil {
		t.Errorf("demo profile without a token rejected: %v", err)
	}
	if err := suiteprofiles.ValidateNew("Team", "https://jira.acme.example", "PLAT", ""); err == nil {
		t.Error("live profile without a token accepted")
	}
	if err := suiteprofiles.ValidateNew("", "demo", "DEMO", ""); err == nil {
		t.Error("blank name accepted")
	}
	if err := suiteprofiles.ValidateNew("Team", "", "DEMO", "t"); err == nil {
		t.Error("blank URL accepted")
	}
	if err := suiteprofiles.ValidateNew("Team", "demo", "", "t"); err == nil {
		t.Error("blank project key accepted")
	}
}
```

- [ ] **Step 5: Implement `suiteprofiles`**

Create `tam/internal/suiteprofiles/suiteprofiles.go`:

```go
// Package suiteprofiles holds the rules for how Task Activity Manager sees the
// profiles it shares with Xray Test Manager.
package suiteprofiles

import (
	"errors"
	"strings"

	"agile-suite/core/profile"
)

// Backend is the backend value TAM writes on the profiles it creates. XTM
// treats any value other than "kiwi" as a Jira connection, so the same row
// works in both apps.
const Backend = "jira"

// Visible drops the profiles TAM cannot use. Kiwi TCMS is not Jira.
func Visible(ps []profile.Profile) []profile.Profile {
	out := make([]profile.Profile, 0, len(ps))
	for _, p := range ps {
		if strings.EqualFold(p.Backend, "kiwi") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// IsDemoURL reports whether a Jira URL selects the offline demo dataset:
// "demo" on its own, or a "demo:" or "demo-" variant. It mirrors XTM's rule so
// a demo profile made in either app is a demo profile in both.
func IsDemoURL(url string) bool {
	u := strings.ToLower(strings.TrimSpace(url))
	return u == "demo" || strings.HasPrefix(u, "demo:") || strings.HasPrefix(u, "demo-")
}

// ValidateNew checks the fields of a profile about to be created. A demo
// profile needs no token; a live one does.
func ValidateNew(name, jiraURL, projectKey, token string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("a profile needs a name")
	}
	if strings.TrimSpace(jiraURL) == "" {
		return errors.New("a profile needs a Jira URL (or \"demo\")")
	}
	if strings.TrimSpace(projectKey) == "" {
		return errors.New("a profile needs a project key")
	}
	if !IsDemoURL(jiraURL) && strings.TrimSpace(token) == "" {
		return errors.New("a live Jira profile needs a personal access token")
	}
	return nil
}
```

Run (inside `tam/`): `go test ./internal/... -v`
Expected: PASS for both packages.

- [ ] **Step 6: The bound app**

Create `tam/app.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"agile-suite/core/profile"
	"agile-suite/core/settings"
	"agile-suite/core/shareddb"
	"agile-suite/core/store"
	"agile-suite/tam/internal/suiteprofiles"
	"agile-suite/tam/internal/tamstore"
)

// App is the backend bound to the React frontend. Every exported method here
// is callable from JavaScript, so it only validates and delegates; the rules
// live in internal/.
type App struct {
	ctx        context.Context
	local      *store.DB
	shared     *store.DB
	profiles   *profile.Manager
	creds      profile.CredentialStore
	settings   *settings.Manager
	dbPath     string
	sharedPath string
	logPath    string
	startupErr string
}

// HealthInfo tells the frontend whether startup succeeded. The Windows build
// has no console, so a failure has to reach the UI as data.
type HealthInfo struct {
	OK         bool   `json:"ok"`
	Error      string `json:"error"`
	DBPath     string `json:"dbPath"`
	SharedPath string `json:"sharedPath"`
	LogPath    string `json:"logPath"`
}

// Diagnostics is the environment summary shown in the About dialog.
type Diagnostics struct {
	Version       string `json:"version"`
	DBPath        string `json:"dbPath"`
	SharedPath    string `json:"sharedPath"`
	LogPath       string `json:"logPath"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"goVersion"`
	SchemaVersion int    `json:"schemaVersion"`
	ProfileCount  int    `json:"profileCount"`
	StartupError  string `json:"startupError"`
}

// NewApp creates the application struct.
func NewApp() *App { return &App{} }

// startup wires the stores. Failures are recorded in startupErr rather than
// returned, so Health can report them.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if path, err := setupFileLogging(); err == nil {
		a.logPath = path
		log.Printf("tam: starting up, log at %s", path)
	}
	if err := a.initStore(); err != nil {
		a.startupErr = err.Error()
		log.Printf("tam: startup failed: %v", err)
	}
}

func (a *App) initStore() error {
	dbPath, err := tamstore.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	local, err := tamstore.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open local store at %s: %w", dbPath, err)
	}
	a.local = local
	a.dbPath = dbPath

	sharedPath, err := shareddb.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve shared profile database path: %w", err)
	}
	shared, err := shareddb.Open(sharedPath)
	if err != nil {
		return fmt.Errorf("open shared profile database at %s: %w", sharedPath, err)
	}
	a.shared = shared
	a.sharedPath = sharedPath

	a.profiles = profile.NewManager(shared.DB())
	a.creds = profile.NewCredentialStore()
	a.settings = settings.NewManager(shared.DB())
	log.Printf("tam: local store ready at %s; shared profiles at %s", dbPath, sharedPath)
	return nil
}

func (a *App) shutdown(ctx context.Context) {
	if a.shared != nil {
		if err := a.shared.Close(); err != nil {
			log.Printf("tam: close shared profile database: %v", err)
		}
	}
	if a.local != nil {
		if err := a.local.Close(); err != nil {
			log.Printf("tam: close local store: %v", err)
		}
	}
}

func (a *App) requireStore() error {
	if a.startupErr != "" {
		return fmt.Errorf("local store unavailable: %s", a.startupErr)
	}
	if a.profiles == nil {
		return errors.New("local store not initialised")
	}
	return nil
}

// Health reports startup status. Always safe to call.
func (a *App) Health() HealthInfo {
	return HealthInfo{
		OK:         a.startupErr == "" && a.profiles != nil,
		Error:      a.startupErr,
		DBPath:     a.dbPath,
		SharedPath: a.sharedPath,
		LogPath:    a.logPath,
	}
}

// GetDiagnostics returns the environment summary. Safe to call even when the
// store failed to open.
func (a *App) GetDiagnostics() Diagnostics {
	d := Diagnostics{
		Version:      productVersion(),
		DBPath:       a.dbPath,
		SharedPath:   a.sharedPath,
		LogPath:      a.logPath,
		OS:           goruntime.GOOS,
		Arch:         goruntime.GOARCH,
		GoVersion:    goruntime.Version(),
		StartupError: a.startupErr,
	}
	if a.local != nil {
		if v, err := store.ReadSchemaVersion(a.local.DB()); err == nil {
			d.SchemaVersion = v
		}
	}
	if a.profiles != nil {
		if ps, err := a.ListProfiles(); err == nil {
			d.ProfileCount = len(ps)
		}
	}
	return d
}

// ListProfiles returns the shared profiles TAM can use.
func (a *App) ListProfiles() ([]profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	ps, err := a.profiles.List()
	if err != nil {
		return nil, err
	}
	return suiteprofiles.Visible(ps), nil
}

// CreateProfile adds a profile to the shared store and saves its token in the
// OS credential manager. A demo profile ("demo" as the URL) needs no token.
func (a *App) CreateProfile(name, jiraURL, projectKey, token string, makeDefault bool) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	if err := suiteprofiles.ValidateNew(name, jiraURL, projectKey, token); err != nil {
		return profile.Profile{}, err
	}
	p, err := a.profiles.Create(
		strings.TrimSpace(name), strings.TrimSpace(jiraURL), strings.TrimSpace(projectKey),
		"", "", "", "", "", false, suiteprofiles.Backend,
	)
	if err != nil {
		return profile.Profile{}, err
	}
	if strings.TrimSpace(token) != "" {
		if err := a.creds.Save(p.ID, strings.TrimSpace(token)); err != nil {
			_ = a.profiles.Delete(p.ID)
			return profile.Profile{}, fmt.Errorf("save credentials: %w", err)
		}
	}
	if makeDefault {
		if err := a.settings.SetDefaultProfileID(p.ID); err != nil {
			log.Printf("tam: set default profile after create: %v", err)
		}
	}
	return p, nil
}

// DeleteProfile removes a profile from the shared store, so it disappears
// from XTM as well, and drops its credential.
func (a *App) DeleteProfile(id string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if err := a.profiles.Delete(id); err != nil {
		return err
	}
	if err := a.creds.Delete(id); err != nil {
		log.Printf("tam: delete credentials for %s: %v", id, err)
	}
	if s, err := a.settings.Get(); err == nil && s.DefaultProfileID == id {
		if err := a.settings.SetDefaultProfileID(""); err != nil {
			log.Printf("tam: clear default profile after delete: %v", err)
		}
	}
	return nil
}

// GetSettings returns the shared preferences.
func (a *App) GetSettings() (settings.Settings, error) {
	if err := a.requireStore(); err != nil {
		return settings.Settings{}, err
	}
	return a.settings.Get()
}

// SetTheme stores the colour theme: "light", "dark", or "system". Both apps
// read it.
func (a *App) SetTheme(theme string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	switch theme {
	case "light", "dark", "system":
		return a.settings.SetTheme(theme)
	}
	return fmt.Errorf("unknown theme %q", theme)
}

// SetDefaultProfile records which profile opens on launch. An empty id clears
// it.
func (a *App) SetDefaultProfile(id string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.settings.SetDefaultProfileID(id)
}

// setupFileLogging sends the standard logger to tam.log in the app data
// directory so startup output survives without a console.
func setupFileLogging() (string, error) {
	dir, err := tamstore.DefaultDir()
	if err != nil {
		return "", err
	}
	logPath := filepath.Join(dir, "tam.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return logPath, nil
}
```

- [ ] **Step 7: The Wails entry point and config**

Create `tam/main.go`:

```go
package main

import (
	"embed"
	"encoding/json"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// wailsConfig is the embedded wails.json, the single source of the product
// version.
//
//go:embed wails.json
var wailsConfig []byte

// productVersion returns info.productVersion from the embedded wails.json, or
// "" if it cannot be read.
func productVersion() string {
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsConfig, &cfg); err != nil {
		return ""
	}
	return cfg.Info.ProductVersion
}

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:  "Task Activity Manager",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:         &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:                app.startup,
		OnShutdown:               app.shutdown,
		Menu:                     appMenu(app),
		EnableDefaultContextMenu: true,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

// appMenu is the native menu bar. Items emit events the frontend listens for,
// so the menu and the in-app buttons share one code path.
func appMenu(app *App) *menu.Menu {
	emit := func(event string) func(*menu.CallbackData) {
		return func(*menu.CallbackData) {
			if app.ctx != nil {
				runtime.EventsEmit(app.ctx, event)
			}
		}
	}
	m := menu.NewMenu()
	file := m.AddSubmenu("File")
	file.AddText("Profiles…", nil, emit("menu:profiles"))
	file.AddSeparator()
	file.AddText("Quit", keys.CmdOrCtrl("q"), func(*menu.CallbackData) {
		if app.ctx != nil {
			runtime.Quit(app.ctx)
		}
	})
	help := m.AddSubmenu("Help")
	help.AddText("About Task Activity Manager", nil, emit("menu:about"))
	return m
}
```

Create `tam/wails.json`:

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "task-activity-manager",
  "outputfilename": "task-activity-manager",
  "frontend:install": "npm install --prefix ../..",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto",
  "author": {
    "name": "Achmad Fienan Rahardianto",
    "email": "veenone@gmail.com"
  },
  "info": {
    "companyName": "Achmad Fienan Rahardianto",
    "productName": "Task Activity Manager",
    "productVersion": "0.1.0",
    "copyright": "Copyright (c) 2026 Achmad Fienan Rahardianto",
    "comments": "Agile task management for Jira Data Center"
  }
}
```

`frontend:install` runs from `tam/frontend`; `--prefix ../..` points npm at the workspace root so one install serves every package (decision 2).

- [ ] **Step 8: Build the Go side**

The main package embeds `frontend/dist`, which Task 6 produces. For now give the embed something to find:

```bash
mkdir -p tam/frontend/dist && echo '<!doctype html><title>placeholder</title>' > tam/frontend/dist/index.html
```

(`tam/frontend/dist` is git-ignored.) Then, inside `tam/`:

```bash
go mod tidy
go build ./...
go vet ./...
go test ./internal/...
```

Expected: `go mod tidy` fills `go.sum` and the indirect requirements; build and vet are clean; the tests pass. Then, inside `xtm/`, `go build ./... && go test ./internal/...` still pass (the workspace now has three modules).

- [ ] **Step 9: Commit**

```bash
git add go.work .gitignore tam/go.mod tam/go.sum tam/main.go tam/app.go tam/wails.json tam/internal/
git commit -m "feat(tam): scaffold the Task Activity Manager backend on the shared core"
```

---
### Task 5: `frontend/core`, and XTM pointed at it

Create the shared npm package, move the dialog system and primitives into it, add the generic contexts TAM's shell needs, and replace each moved XTM file with a re-export. XTM's Vitest suite is the proof nothing moved.

**Files:**
- Create: `package.json` (root), `frontend/core/package.json`, `frontend/core/tsconfig.json`, `frontend/core/tsconfig.node.json`, `frontend/core/vite.config.ts`, `frontend/core/src/test/setup.ts`, `frontend/core/src/index.ts`, `frontend/core/src/lib/errMsg.ts`, `frontend/core/src/lib/theme.ts`, `frontend/core/src/lib/queryClient.ts`, `frontend/core/src/contexts/createModalContext.tsx`, `frontend/core/src/contexts/createModalContext.test.tsx`, `frontend/core/src/contexts/createViewContext.tsx`, `frontend/core/src/contexts/createViewContext.test.tsx`, `frontend/core/src/contexts/ProfileContext.tsx`, `frontend/core/src/contexts/ProfileContext.test.tsx`, `frontend/core/styles/tokens.css`, `frontend/core/styles/primitives.css`
- Move (git mv): listed under "Moved" in the file structure.
- Modify: `xtm/frontend/package.json`, `xtm/wails.json`, `xtm/frontend/src/api.ts`, `xtm/frontend/src/contexts/ProfileContext.tsx`, `xtm/frontend/src/contexts/ModalContext.tsx`, `xtm/frontend/src/lib/queryClient.ts`, and the shim files at every moved path.
- Delete: `xtm/frontend/package-lock.json` (the lock moves to the root).

**Interfaces:**
- Produces (the package's public surface, all from `@agile-suite/core`): `DialogProvider`, `useDialogs`, `createModalContext<Id extends string>(providerName?)` returning `{ ModalProvider, useModal }`, `createViewContext<V extends string>(initial: V)` returning `{ ViewProvider, useView }`, `ProfileProvider<P, S>({ backend, children })`, `useProfile<P, S>()`, types `ProfileBackend<P, S>` and `ProfileState<P, S>`, `Modal`, `Menu`, `MenuItem`, `LiveRegion`, `announce`, `useNotice`, `useConfirm`, `usePrompt` and their option types, `call`, `ApiError`, `normalizeError`, `errMsg`, `applyTheme`, `createQueryClient`. Stylesheets at `@agile-suite/core/styles/tokens.css` and `@agile-suite/core/styles/primitives.css`.

- [ ] **Step 1: The workspace root and the package manifest**

Create `package.json` at the repo root:

```json
{
  "name": "agile-suite",
  "private": true,
  "workspaces": [
    "frontend/core",
    "xtm/frontend"
  ],
  "scripts": {
    "test": "npm test --workspaces --if-present",
    "typecheck": "npm run typecheck --workspaces --if-present"
  }
}
```

Create `frontend/core/package.json`. The dev dependency versions are copied from `xtm/frontend/package.json` so npm hoists one copy of each:

```json
{
  "name": "@agile-suite/core",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "exports": {
    ".": "./src/index.ts",
    "./styles/tokens.css": "./styles/tokens.css",
    "./styles/primitives.css": "./styles/primitives.css"
  },
  "scripts": {
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit"
  },
  "peerDependencies": {
    "@tanstack/react-query": "^5.102.3",
    "react": "^19.2.7",
    "react-dom": "^19.2.7"
  },
  "devDependencies": {
    "@tanstack/react-query": "^5.102.3",
    "@testing-library/dom": "^10.4.1",
    "@testing-library/jest-dom": "^7.0.1",
    "@testing-library/react": "^16.3.2",
    "@testing-library/user-event": "^14.6.7",
    "@types/react": "^19.2.17",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^6.0.3",
    "jsdom": "^29.1.1",
    "react": "^19.2.7",
    "react-dom": "^19.2.7",
    "typescript": "^6.0.3",
    "vite": "^8.1.3",
    "vitest": "^4.1.11"
  }
}
```

Copy `xtm/frontend/tsconfig.json`, `xtm/frontend/tsconfig.node.json`, `xtm/frontend/vite.config.ts`, and `xtm/frontend/src/test/setup.ts` to the same relative paths under `frontend/core/`, unchanged.

- [ ] **Step 2: Move the files that need no change**

From the repo root:

```bash
mkdir -p frontend/core/src/contexts frontend/core/src/components frontend/core/src/lib frontend/core/styles
git mv xtm/frontend/src/contexts/DialogContext.tsx      frontend/core/src/contexts/DialogContext.tsx
git mv xtm/frontend/src/contexts/DialogContext.test.tsx frontend/core/src/contexts/DialogContext.test.tsx
git mv xtm/frontend/src/components/Modal.tsx      frontend/core/src/components/Modal.tsx
git mv xtm/frontend/src/components/LiveRegion.tsx frontend/core/src/components/LiveRegion.tsx
git mv xtm/frontend/src/components/Menu.tsx       frontend/core/src/components/Menu.tsx
git mv xtm/frontend/src/components/useNotice.tsx  frontend/core/src/components/useNotice.tsx
git mv xtm/frontend/src/components/useConfirm.tsx frontend/core/src/components/useConfirm.tsx
git mv xtm/frontend/src/components/usePrompt.tsx  frontend/core/src/components/usePrompt.tsx
git mv xtm/frontend/src/lib/apiCall.ts       frontend/core/src/lib/apiCall.ts
git mv xtm/frontend/src/lib/apiError.ts      frontend/core/src/lib/apiError.ts
git mv xtm/frontend/src/lib/apiError.test.ts frontend/core/src/lib/apiError.test.ts
```

The relative imports inside these files (`../components/Modal`, `../contexts/DialogContext`, `./apiError`) still resolve because the directory shape is the same. One import does not: `apiError.ts` reads `errMsg` from `../api`. Create `frontend/core/src/lib/errMsg.ts` with the function moved out of XTM's `api.ts`:

```ts
// errMsg turns whatever a rejected promise carries into a readable string.
export function errMsg(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === "string" ? e : String(e);
}
```

and in `frontend/core/src/lib/apiError.ts` change `import { errMsg } from "../api";` to `import { errMsg } from "./errMsg";`.

- [ ] **Step 3: The theme helper and the query client factory**

Create `frontend/core/src/lib/theme.ts`, lifted from XTM's `ProfileContext.tsx`:

```ts
// applyTheme resolves the preference ("system" follows the OS) and sets the
// data-theme attribute the CSS tokens key off.
export function applyTheme(theme: string) {
  const dark =
    theme === "dark" ||
    (theme === "system" &&
      window.matchMedia?.("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
}
```

Create `frontend/core/src/lib/queryClient.ts`:

```ts
import { QueryClient } from "@tanstack/react-query";

// Tuned for a local Go process behind Wails rather than a network: nothing
// retries and nothing refetches in the background. Freshness comes from
// explicit invalidation after mutations.
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: 0,
        refetchOnWindowFocus: false,
        staleTime: 30_000,
      },
    },
  });
}
```

- [ ] **Step 4: Failing tests for the two factories**

Create `frontend/core/src/contexts/createModalContext.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { createModalContext } from "./createModalContext";

const { ModalProvider, useModal } = createModalContext<"profiles" | "about">();

function wrapper({ children }: { children: React.ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

describe("createModalContext", () => {
  it("opens one modal at a time and closes it", () => {
    const { result } = renderHook(() => useModal(), { wrapper });
    expect(result.current.current).toBeNull();
    act(() => result.current.openModal("profiles"));
    expect(result.current.isOpen("profiles")).toBe(true);
    act(() => result.current.openModal("about"));
    expect(result.current.isOpen("profiles")).toBe(false);
    expect(result.current.current).toBe("about");
    act(() => result.current.closeModal());
    expect(result.current.current).toBeNull();
  });

  it("throws outside its provider", () => {
    expect(() => renderHook(() => useModal())).toThrow(
      "useModal must be used within a ModalProvider",
    );
  });
});
```

Create `frontend/core/src/contexts/createViewContext.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { createViewContext } from "./createViewContext";

const { ViewProvider, useView } = createViewContext<"backlog" | "boards">("backlog");

function wrapper({ children }: { children: React.ReactNode }) {
  return <ViewProvider>{children}</ViewProvider>;
}

describe("createViewContext", () => {
  it("starts on the initial view and switches", () => {
    const { result } = renderHook(() => useView(), { wrapper });
    expect(result.current.view).toBe("backlog");
    act(() => result.current.setView("boards"));
    expect(result.current.view).toBe("boards");
  });

  it("throws outside its provider", () => {
    expect(() => renderHook(() => useView())).toThrow(
      "useView must be used within a ViewProvider",
    );
  });
});
```

Run (inside `frontend/core/`, after `npm install` at the root): `npx vitest run src/contexts`
Expected: FAIL, the two factory modules do not exist.

- [ ] **Step 5: Implement the factories**

Create `frontend/core/src/contexts/createModalContext.tsx`:

```tsx
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useReducer,
} from "react";
import type { ReactNode } from "react";

export interface ModalApi<Id extends string> {
  current: Id | null;
  isOpen: (id: Id) => boolean;
  openModal: (id: Id) => void;
  closeModal: () => void;
}

// One root-level overlay is open at a time, so a single `current` id replaces
// a pile of booleans. Each app names its own modal ids and gets a typed
// provider and hook back.
export function createModalContext<Id extends string>(
  providerName = "ModalProvider",
) {
  const Ctx = createContext<ModalApi<Id> | null>(null);
  type Action = { type: "OPEN"; id: Id } | { type: "CLOSE" };

  function reducer(state: Id | null, action: Action): Id | null {
    switch (action.type) {
      case "OPEN":
        return action.id;
      case "CLOSE":
        return null;
      default:
        return state;
    }
  }

  function ModalProvider({ children }: { children: ReactNode }) {
    const [current, dispatch] = useReducer(reducer, null);
    const isOpen = useCallback((id: Id) => current === id, [current]);
    const openModal = useCallback(
      (id: Id) => dispatch({ type: "OPEN", id }),
      [],
    );
    const closeModal = useCallback(() => dispatch({ type: "CLOSE" }), []);
    const api = useMemo<ModalApi<Id>>(
      () => ({ current, isOpen, openModal, closeModal }),
      [current, isOpen, openModal, closeModal],
    );
    return <Ctx.Provider value={api}>{children}</Ctx.Provider>;
  }

  function useModal(): ModalApi<Id> {
    const ctx = useContext(Ctx);
    if (!ctx) {
      throw new Error(`useModal must be used within a ${providerName}`);
    }
    return ctx;
  }

  return { ModalProvider, useModal };
}
```

Create `frontend/core/src/contexts/createViewContext.tsx`:

```tsx
import { createContext, useContext, useMemo, useState } from "react";
import type { Dispatch, ReactNode, SetStateAction } from "react";

export interface ViewApi<V extends string> {
  view: V;
  setView: Dispatch<SetStateAction<V>>;
}

// The active top-level view. Each app names its own views; anything else it
// wants to route on lives in its own context.
export function createViewContext<V extends string>(initial: V) {
  const Ctx = createContext<ViewApi<V> | null>(null);

  function ViewProvider({ children }: { children: ReactNode }) {
    const [view, setView] = useState<V>(initial);
    const api = useMemo<ViewApi<V>>(() => ({ view, setView }), [view]);
    return <Ctx.Provider value={api}>{children}</Ctx.Provider>;
  }

  function useView(): ViewApi<V> {
    const ctx = useContext(Ctx);
    if (!ctx) {
      throw new Error("useView must be used within a ViewProvider");
    }
    return ctx;
  }

  return { ViewProvider, useView };
}
```

Run: `npx vitest run src/contexts` inside `frontend/core/`.
Expected: PASS for both factory tests and for the moved `DialogContext.test.tsx`.

- [ ] **Step 6: Failing test for the profile provider**

Create `frontend/core/src/contexts/ProfileContext.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { ProfileProvider, useProfile } from "./ProfileContext";
import type { ProfileBackend } from "./ProfileContext";

interface P { id: string; name: string }
interface S { defaultProfileId: string; theme: string }

const backend: ProfileBackend<P, S> = {
  listProfiles: vi.fn(),
  getSettings: vi.fn(),
  setTheme: vi.fn(),
  setDefaultProfile: vi.fn(),
};

function wrapper({ children }: { children: React.ReactNode }) {
  return <ProfileProvider backend={backend}>{children}</ProfileProvider>;
}

beforeEach(() => {
  vi.mocked(backend.listProfiles).mockResolvedValue([
    { id: "a", name: "A" },
    { id: "b", name: "B" },
  ]);
  vi.mocked(backend.getSettings).mockResolvedValue({
    defaultProfileId: "b",
    theme: "dark",
  });
  vi.mocked(backend.setTheme).mockResolvedValue();
  vi.mocked(backend.setDefaultProfile).mockResolvedValue();
  document.documentElement.dataset.theme = "";
});

describe("ProfileProvider", () => {
  it("loads profiles, picks the default, and applies the theme", async () => {
    const { result } = renderHook(() => useProfile<P, S>(), { wrapper });
    await act(async () => {
      await result.current.reload();
    });
    expect(result.current.profiles.map((p) => p.id)).toEqual(["a", "b"]);
    expect(result.current.activeId).toBe("b");
    expect(result.current.activeProfile?.name).toBe("B");
    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  it("falls back to the first profile when the default is gone", async () => {
    vi.mocked(backend.getSettings).mockResolvedValue({
      defaultProfileId: "zzz",
      theme: "light",
    });
    const { result } = renderHook(() => useProfile<P, S>(), { wrapper });
    await act(async () => {
      await result.current.reload();
    });
    expect(result.current.activeId).toBe("a");
  });

  it("setDefault toggles and persists", async () => {
    const { result } = renderHook(() => useProfile<P, S>(), { wrapper });
    await act(async () => {
      await result.current.reload();
    });
    await act(async () => {
      await result.current.setDefault("b");
    });
    expect(backend.setDefaultProfile).toHaveBeenCalledWith("");
    expect(result.current.defaultProfileId).toBe("");
  });

  it("setTheme applies immediately and persists", async () => {
    const { result } = renderHook(() => useProfile<P, S>(), { wrapper });
    await act(async () => {
      await result.current.setTheme("light");
    });
    await waitFor(() => expect(backend.setTheme).toHaveBeenCalledWith("light"));
    expect(document.documentElement.dataset.theme).toBe("light");
  });
});
```

Run: `npx vitest run src/contexts/ProfileContext` inside `frontend/core/`.
Expected: FAIL, module not found.

- [ ] **Step 7: Implement the profile provider**

Create `frontend/core/src/contexts/ProfileContext.tsx`:

```tsx
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";
import type { Dispatch, ReactNode, SetStateAction } from "react";
import { applyTheme } from "../lib/theme";
import { errMsg } from "../lib/errMsg";

// The calls the provider needs from an app's backend. Each app builds one
// from its own generated Wails bindings, so this package never imports them.
export interface ProfileBackend<
  P extends { id: string },
  S extends { defaultProfileId?: string; theme?: string },
> {
  listProfiles: () => Promise<P[]>;
  getSettings: () => Promise<S>;
  setTheme: (theme: string) => Promise<void>;
  setDefaultProfile: (id: string) => Promise<void>;
}

export interface ProfileState<
  P extends { id: string },
  S extends { defaultProfileId?: string; theme?: string },
> {
  profiles: P[];
  activeId: string;
  defaultProfileId: string;
  theme: string;
  loading: boolean;
  activeProfile: P | undefined;
  setActiveId: Dispatch<SetStateAction<string>>;
  // Applies the theme at once, then persists it.
  setTheme: (next: string) => Promise<void>;
  // Makes id the launch default, or clears the default if id already is.
  setDefault: (id: string) => Promise<void>;
  // Loads profiles and settings, applies the theme, picks the launch profile,
  // and returns the settings so the caller can read anything app-specific.
  reload: () => Promise<S | null>;
}

// One context object serves every app; the hook casts to the app's own
// profile and settings shapes. The cast is safe because the provider is the
// only writer and it is typed by the backend it was given.
const ProfileContext = createContext<ProfileState<{ id: string }, object> | null>(null);

export function useProfile<
  P extends { id: string },
  S extends { defaultProfileId?: string; theme?: string },
>(): ProfileState<P, S> {
  const ctx = useContext(ProfileContext);
  if (!ctx) {
    throw new Error("useProfile must be used within a ProfileProvider");
  }
  return ctx as unknown as ProfileState<P, S>;
}

export function ProfileProvider<
  P extends { id: string },
  S extends { defaultProfileId?: string; theme?: string },
>({ backend, children }: { backend: ProfileBackend<P, S>; children: ReactNode }) {
  const [profiles, setProfiles] = useState<P[]>([]);
  const [activeId, setActiveId] = useState("");
  const [defaultProfileId, setDefaultProfileId] = useState("");
  const [theme, setThemeState] = useState("light");
  const [loading, setLoading] = useState(false);

  const setTheme = useCallback(
    async (next: string) => {
      setThemeState(next);
      applyTheme(next);
      try {
        await backend.setTheme(next);
      } catch (e) {
        console.error("set theme:", errMsg(e));
      }
    },
    [backend],
  );

  const setDefault = useCallback(
    async (id: string) => {
      const next = defaultProfileId === id ? "" : id;
      try {
        await backend.setDefaultProfile(next);
        setDefaultProfileId(next);
      } catch (e) {
        console.error("set default profile:", errMsg(e));
      }
    },
    [backend, defaultProfileId],
  );

  const reload = useCallback(async (): Promise<S | null> => {
    setLoading(true);
    try {
      const [ps, s] = await Promise.all([
        backend.listProfiles(),
        backend.getSettings(),
      ]);
      setProfiles(ps);
      setDefaultProfileId(s.defaultProfileId ?? "");
      const t = s.theme || "light";
      setThemeState(t);
      applyTheme(t);
      if (ps.length > 0) {
        const def =
          s.defaultProfileId && ps.some((p) => p.id === s.defaultProfileId)
            ? s.defaultProfileId
            : ps[0].id;
        setActiveId(def);
      } else {
        setActiveId("");
      }
      return s;
    } catch (e) {
      console.error("load profiles:", errMsg(e));
      return null;
    } finally {
      setLoading(false);
    }
  }, [backend]);

  const activeProfile = useMemo(
    () => profiles.find((p) => p.id === activeId),
    [profiles, activeId],
  );

  const value = useMemo<ProfileState<P, S>>(
    () => ({
      profiles,
      activeId,
      defaultProfileId,
      theme,
      loading,
      activeProfile,
      setActiveId,
      setTheme,
      setDefault,
      reload,
    }),
    [
      profiles,
      activeId,
      defaultProfileId,
      theme,
      loading,
      activeProfile,
      setTheme,
      setDefault,
      reload,
    ],
  );

  return (
    <ProfileContext.Provider
      value={value as unknown as ProfileState<{ id: string }, object>}
    >
      {children}
    </ProfileContext.Provider>
  );
}
```

Run: `npx vitest run` inside `frontend/core/`.
Expected: PASS, every test in the package.

- [ ] **Step 8: The package entry point and stylesheets**

Create `frontend/core/src/index.ts`:

```ts
export { DialogProvider, useDialogs } from "./contexts/DialogContext";
export { createModalContext } from "./contexts/createModalContext";
export type { ModalApi } from "./contexts/createModalContext";
export { createViewContext } from "./contexts/createViewContext";
export type { ViewApi } from "./contexts/createViewContext";
export { ProfileProvider, useProfile } from "./contexts/ProfileContext";
export type { ProfileBackend, ProfileState } from "./contexts/ProfileContext";
export { Modal } from "./components/Modal";
export { Menu } from "./components/Menu";
export type { MenuItem } from "./components/Menu";
export { LiveRegion, announce } from "./components/LiveRegion";
export { useNotice } from "./components/useNotice";
export type { NoticeOptions } from "./components/useNotice";
export { useConfirm } from "./components/useConfirm";
export type { ConfirmOptions } from "./components/useConfirm";
export { usePrompt } from "./components/usePrompt";
export type { PromptOptions } from "./components/usePrompt";
export { call } from "./lib/apiCall";
export { ApiError, normalizeError } from "./lib/apiError";
export { errMsg } from "./lib/errMsg";
export { applyTheme } from "./lib/theme";
export { createQueryClient } from "./lib/queryClient";
```

Create `frontend/core/styles/tokens.css` by copying, verbatim, the `:root { ... }` and `:root[data-theme="dark"] { ... }` blocks from the top of `xtm/frontend/src/App.css` (the two blocks that define `--surface` through `--bg-subtle`), preceded by this comment:

```css
/* Design tokens shared by the suite's apps. Light values are the defaults;
   [data-theme="dark"] re-points them. XTM's App.css carries the same blocks
   until its chrome moves onto the shared shell. Keep the two in step. */
```

Create `frontend/core/styles/primitives.css` with the rules the moved components render against, copied verbatim from `xtm/frontend/src/App.css`, in this order and with this header:

```css
/* Styles for the primitives in @agile-suite/core: the screen-reader helper,
   buttons, the dropdown menu, the modal shell, form rows, and the notice,
   confirm, and prompt dialogs. Copied from XTM's App.css, which keeps its own
   copies for now. */
```

followed by the rules for these selectors, each with every variant XTM defines for it (`:hover`, `:disabled`, descendant rules, and the `@keyframes menu-in` block): `.sr-only`; `.menu`, `.menu-caret`, `.menu-backdrop`, `.menu-panel`, `.menu-panel-left`, `.menu-panel-right`, `.menu-item`, `.menu-item-danger`, `.menu-check`, `.menu-label`, `.menu-divider`; `.btn`, `.btn-primary`, `.btn-ghost`, `.btn-danger`, `.btn-block`; `.form-actions`, `.form-actions-end`; `.modal-overlay`, `.modal`; `.field-label`, `.detail-input`, `.detail-input-inline`; `.confirm-message`; `.pending-head` (and `.pending-head h2`); `.pending-actions`; `.prompt-modal` (and `.prompt-modal .bulk-body`); `.bulk-body`. Also add a `.topbar-btn` rule, because `Menu` defaults its trigger to that class and TAM has no XTM topbar styles:

```css
.topbar-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--surface);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  padding: 5px 10px;
  color: var(--text);
  cursor: pointer;
}
.topbar-btn:hover {
  background: var(--row-hover);
}
```

Check the copy with `grep -c '{' frontend/core/styles/primitives.css` (expect a count in the forties) and by opening the TAM shell in Task 6, where every one of these classes is exercised.

- [ ] **Step 9: Point XTM at the package**

Replace each moved XTM file with a re-export. The full contents of each:

`xtm/frontend/src/contexts/DialogContext.tsx`
```ts
export { DialogProvider, useDialogs } from "@agile-suite/core";
```
`xtm/frontend/src/components/Modal.tsx`
```ts
export { Modal } from "@agile-suite/core";
```
`xtm/frontend/src/components/LiveRegion.tsx`
```ts
export { LiveRegion, announce } from "@agile-suite/core";
```
`xtm/frontend/src/components/Menu.tsx`
```ts
export { Menu } from "@agile-suite/core";
export type { MenuItem } from "@agile-suite/core";
```
`xtm/frontend/src/components/useNotice.tsx`
```ts
export { useNotice } from "@agile-suite/core";
export type { NoticeOptions } from "@agile-suite/core";
```
`xtm/frontend/src/components/useConfirm.tsx`
```ts
export { useConfirm } from "@agile-suite/core";
export type { ConfirmOptions } from "@agile-suite/core";
```
`xtm/frontend/src/components/usePrompt.tsx`
```ts
export { usePrompt } from "@agile-suite/core";
export type { PromptOptions } from "@agile-suite/core";
```
`xtm/frontend/src/lib/apiCall.ts`
```ts
export { call } from "@agile-suite/core";
```
`xtm/frontend/src/lib/apiError.ts`
```ts
export { ApiError, normalizeError } from "@agile-suite/core";
```
`xtm/frontend/src/lib/queryClient.ts`
```ts
import { createQueryClient } from "@agile-suite/core";

export const queryClient = createQueryClient();
```

Then three edits inside files that stay:

1. `xtm/frontend/src/api.ts`: delete the `errMsg` function and put `export { errMsg } from "@agile-suite/core";` directly above `export function isDemoUrl`.
2. `xtm/frontend/src/contexts/ProfileContext.tsx`: delete the local `applyTheme` function and add `import { applyTheme } from "@agile-suite/core";` after the `../api` imports. Everything else in the file stays.
3. `xtm/frontend/src/contexts/ModalContext.tsx`: keep the file comment and the `ModalId` union; delete `ModalAction`, `modalReducer`, `ModalApi`, `ModalContext`, `useModal`, and `ModalProvider`; replace them with:

```ts
import { createModalContext } from "@agile-suite/core";

export const { ModalProvider, useModal } = createModalContext<ModalId>();
```

(drop the now-unused `react` imports.)

In `xtm/frontend/package.json` add `"@agile-suite/core": "0.0.0"` to `dependencies`. In `xtm/wails.json` change `"frontend:install"` to `"npm install --prefix ../.."`.

- [ ] **Step 10: Install and run everything**

From the repo root:

```bash
rm -rf xtm/frontend/node_modules
git rm -q xtm/frontend/package-lock.json
npm install
npm test --workspaces --if-present
npm run typecheck --workspaces --if-present
```

Expected: one `package-lock.json` at the root; `node_modules/@agile-suite/core` is a link to `frontend/core`; XTM's suite passes with the moved files gone (it runs fewer than 196 tests now) and core's suite passes the moved `DialogContext` and `apiError` tests plus the new ones. Add the two "Tests N passed" lines: the sum must be at least 196. XTM has no `typecheck` script, so also run `npm run build` inside `xtm/frontend` (tsc plus the Vite build); it must be clean.

Then, inside `xtm/`, `wails build` must succeed and `wails dev` must open the XTM window with the dialogs, menus, and theme working: open Manage Profiles, trigger a confirm (delete a throwaway profile and cancel), and switch the theme.

- [ ] **Step 11: Commit**

```bash
git add package.json package-lock.json frontend/core xtm/frontend xtm/wails.json
git commit -m "refactor(frontend): lift the dialog system, primitives, and shell contexts into @agile-suite/core"
```

---
### Task 6: The TAM frontend

Build TAM's React app on `@agile-suite/core`: the shell from the mockup (topbar with the DEMO chip and profile picker, nav rail with the Suite section, status bar), placeholder views that say which phase brings them, the Profiles dialog, and an About dialog. `wails build` generates the bindings and proves the whole app packages.

**Files:**
- Create: `tam/frontend/package.json`, `tam/frontend/index.html`, `tam/frontend/vite.config.ts`, `tam/frontend/tsconfig.json`, `tam/frontend/tsconfig.node.json`, `tam/frontend/src/test/setup.ts`, `tam/frontend/src/main.tsx`, `tam/frontend/src/api.ts`, `tam/frontend/src/api.test.ts`, `tam/frontend/src/profileBackend.ts`, `tam/frontend/src/nav.ts`, `tam/frontend/src/modals.ts`, `tam/frontend/src/App.tsx`, `tam/frontend/src/App.test.tsx`, `tam/frontend/src/App.css`, `tam/frontend/src/components/Placeholder.tsx`, `tam/frontend/src/components/ProfilesModal.tsx`, `tam/frontend/src/components/ProfilesModal.test.tsx`, `tam/frontend/src/components/AboutModal.tsx`, `tam/build/**` (copied from `xtm/build/`), `tam/frontend/wailsjs/**` (generated)
- Modify: `package.json` (root) to add the workspace

**Interfaces:**
- Consumes: the bound methods from Task 4 (exact names in `api.ts` below) and the `@agile-suite/core` surface from Task 5.

- [ ] **Step 1: Package files**

Add `"tam/frontend"` to the root `package.json` `workspaces` array. Create `tam/frontend/package.json`:

```json
{
  "name": "tam-frontend",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@agile-suite/core": "0.0.0",
    "@tanstack/react-query": "^5.102.3",
    "react": "^19.2.7",
    "react-dom": "^19.2.7"
  },
  "devDependencies": {
    "@testing-library/dom": "^10.4.1",
    "@testing-library/jest-dom": "^7.0.1",
    "@testing-library/react": "^16.3.2",
    "@testing-library/user-event": "^14.6.7",
    "@types/react": "^19.2.17",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^6.0.3",
    "jsdom": "^29.1.1",
    "typescript": "^6.0.3",
    "vite": "^8.1.3",
    "vitest": "^4.1.11"
  }
}
```

Copy `xtm/frontend/vite.config.ts`, `tsconfig.json`, `tsconfig.node.json`, and `src/test/setup.ts` to the same paths under `tam/frontend/`. Create `tam/frontend/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8"/>
    <meta content="width=device-width, initial-scale=1.0" name="viewport"/>
    <title>Task Activity Manager</title>
</head>
<body>
<div id="root"></div>
<script src="./src/main.tsx" type="module"></script>
</body>
</html>
```

Copy the Wails build assets: `cp -r xtm/build tam/build`, then delete `tam/build/windows/installer` (TAM has no installer yet) and `tam/build/bin` if it was copied. In `tam/build/windows/info.json` and `tam/build/darwin/Info.plist` (and `Info.dev.plist`), replace every "Xray Test Manager" with "Task Activity Manager" and `xray-test-manager` with `task-activity-manager`. The icon stays XTM's for now; a TAM icon is a design task for later.

Run `npm install` at the root, then, inside `tam/`, `wails generate module`.
Expected: `tam/frontend/wailsjs/go/main/App.d.ts`, `App.js`, `models.ts`, and `wailsjs/runtime/` appear, listing the eight bound methods. Commit the generated directory like XTM does.

- [ ] **Step 2: The API layer, with a failing test for the demo rule**

Create `tam/frontend/src/api.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { isDemoUrl } from "./api";

describe("isDemoUrl", () => {
  it("matches demo, demo: and demo- forms, case-insensitively", () => {
    expect(isDemoUrl("demo")).toBe(true);
    expect(isDemoUrl(" DEMO ")).toBe(true);
    expect(isDemoUrl("demo:pkcs")).toBe(true);
    expect(isDemoUrl("demo-agile")).toBe(true);
  });
  it("rejects live URLs, blanks, and the Kiwi demo", () => {
    expect(isDemoUrl("https://jira.acme.example")).toBe(false);
    expect(isDemoUrl("")).toBe(false);
    expect(isDemoUrl(undefined)).toBe(false);
    expect(isDemoUrl("kiwi-demo")).toBe(false);
  });
});
```

Create `tam/frontend/src/api.ts`:

```ts
// api.ts is the frontend's typed access to the Go backend. It re-exports the
// generated bindings and defines plain shapes for what they return, so state
// and test fixtures can be object literals.

export {
  Health,
  GetDiagnostics,
  ListProfiles,
  CreateProfile,
  DeleteProfile,
  GetSettings,
  SetTheme,
  SetDefaultProfile,
} from "../wailsjs/go/main/App";
export { EventsOn, BrowserOpenURL } from "../wailsjs/runtime/runtime";

export interface Profile {
  id: string;
  name: string;
  jiraUrl: string;
  projectKey: string;
  backend: string;
  createdAt: string;
}

export interface Settings {
  defaultProfileId: string;
  theme: string;
}

export interface HealthInfo {
  ok: boolean;
  error: string;
  dbPath: string;
  sharedPath: string;
  logPath: string;
}

export interface Diagnostics {
  version: string;
  dbPath: string;
  sharedPath: string;
  logPath: string;
  os: string;
  arch: string;
  goVersion: string;
  schemaVersion: number;
  profileCount: number;
  startupError: string;
}

// isDemoUrl mirrors suiteprofiles.IsDemoURL in the backend: "demo" on its own
// or a "demo:" / "demo-" variant selects the offline dataset.
export function isDemoUrl(url?: string): boolean {
  const u = (url ?? "").trim().toLowerCase();
  return u === "demo" || u.startsWith("demo:") || u.startsWith("demo-");
}
```

Create `tam/frontend/src/profileBackend.ts`:

```ts
import type { ProfileBackend } from "@agile-suite/core";
import {
  ListProfiles,
  GetSettings,
  SetTheme,
  SetDefaultProfile,
} from "./api";
import type { Profile, Settings } from "./api";

// The adapter the shared ProfileProvider talks through. Bindings are wrapped
// in arrow functions so tests can mock ./api without touching this file.
export const profileBackend: ProfileBackend<Profile, Settings> = {
  listProfiles: () => ListProfiles(),
  getSettings: () => GetSettings(),
  setTheme: (theme) => SetTheme(theme),
  setDefaultProfile: (id) => SetDefaultProfile(id),
};
```

Run: `npx vitest run src/api` inside `tam/frontend/`.
Expected: PASS.

- [ ] **Step 3: Navigation and modal ids**

Create `tam/frontend/src/nav.ts`:

```ts
import { createViewContext } from "@agile-suite/core";

export type View = "backlog" | "epics" | "boards" | "reports" | "rituals";

export interface ViewInfo {
  id: View;
  label: string;
  // The phase of the foundation design that delivers the view.
  phase: string;
  blurb: string;
}

export const VIEWS: ViewInfo[] = [
  {
    id: "backlog",
    label: "Backlog",
    phase: "Phase 1",
    blurb: "Issue sync, the grid, and the detail panel are the first feature slice.",
  },
  {
    id: "epics",
    label: "Epics",
    phase: "Phase 2",
    blurb: "The epic to story to task tree.",
  },
  {
    id: "boards",
    label: "Boards",
    phase: "Phase 3",
    blurb: "Kanban and the active sprint, with live drag.",
  },
  {
    id: "reports",
    label: "Reports",
    phase: "Phase 4",
    blurb: "Burndown, velocity, and sprint analytics.",
  },
  {
    id: "rituals",
    label: "Rituals",
    phase: "Phase 5",
    blurb: "Confluence pages for planning, standups, reviews, and retros.",
  },
];

export const { ViewProvider, useView } = createViewContext<View>("backlog");
```

Create `tam/frontend/src/modals.ts`:

```ts
import { createModalContext } from "@agile-suite/core";

export type ModalId = "profiles" | "about";

export const { ModalProvider, useModal } = createModalContext<ModalId>();
```

- [ ] **Step 4: Failing tests for the shell and the Profiles dialog**

Create `tam/frontend/src/App.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import * as api from "./api";
import App from "./App";
import { profileBackend } from "./profileBackend";
import { ViewProvider } from "./nav";
import { ModalProvider } from "./modals";

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    Health: vi.fn(),
    GetDiagnostics: vi.fn(),
    ListProfiles: vi.fn(),
    CreateProfile: vi.fn(),
    DeleteProfile: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
    BrowserOpenURL: vi.fn(),
  };
});

function renderApp() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ViewProvider>
            <ModalProvider>
              <App />
            </ModalProvider>
          </ViewProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(api.Health).mockResolvedValue({
    ok: true, error: "", dbPath: "C:/tam.db", sharedPath: "C:/profiles.db", logPath: "C:/tam.log",
  });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
});

describe("App shell", () => {
  it("shows the title, the demo chip, and the active profile", async () => {
    renderApp();
    expect(screen.getByText("Task Activity Manager")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: /profile/i })).toHaveValue("p1");
  });

  it("switches views from the nav rail and names the phase", async () => {
    renderApp();
    await userEvent.click(screen.getByRole("button", { name: "Epics" }));
    expect(screen.getByRole("heading", { name: "Epics" })).toBeInTheDocument();
    expect(screen.getByText(/arrives in Phase 2/)).toBeInTheDocument();
  });

  it("surfaces a startup failure instead of a blank page", async () => {
    vi.mocked(api.Health).mockResolvedValue({
      ok: false, error: "open local store: disk full", dbPath: "", sharedPath: "", logPath: "",
    });
    renderApp();
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("disk full"),
    );
  });
});
```

Create `tam/frontend/src/components/ProfilesModal.test.tsx`:

```tsx
import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider, ProfileProvider } from "@agile-suite/core";
import * as api from "../api";
import { ProfilesModal } from "./ProfilesModal";
import { profileBackend } from "../profileBackend";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    CreateProfile: vi.fn(),
    DeleteProfile: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
  };
});

beforeEach(() => {
  vi.mocked(api.ListProfiles).mockResolvedValue([]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "", theme: "light" });
  vi.mocked(api.CreateProfile).mockResolvedValue({
    id: "new", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "",
  });
});

function renderModal(onClose = vi.fn()) {
  render(
    <DialogProvider>
      <ProfileProvider backend={profileBackend}>
        <ProfilesModal onClose={onClose} />
      </ProfileProvider>
    </DialogProvider>,
  );
  return onClose;
}

describe("ProfilesModal", () => {
  it("creates a demo profile without a token and makes it the default", async () => {
    renderModal();
    await userEvent.type(screen.getByLabelText("Name"), "Demo team");
    await userEvent.type(screen.getByLabelText("Jira URL"), "demo");
    await userEvent.type(screen.getByLabelText("Project key"), "DEMO");
    await userEvent.click(screen.getByLabelText("Make this the default profile"));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(api.CreateProfile).toHaveBeenCalledWith("Demo team", "demo", "DEMO", "", true),
    );
    // The provider reloads the list once the save went through.
    expect(api.ListProfiles).toHaveBeenCalledTimes(1);
  });

  it("shows the backend's validation message", async () => {
    vi.mocked(api.CreateProfile).mockRejectedValue(
      new Error("a live Jira profile needs a personal access token"),
    );
    renderModal();
    await userEvent.type(screen.getByLabelText("Name"), "Acme");
    await userEvent.type(screen.getByLabelText("Jira URL"), "https://jira.acme.example");
    await userEvent.type(screen.getByLabelText("Project key"), "PLAT");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("personal access token"),
    );
  });
});
```

Run: `npx vitest run` inside `tam/frontend/`.
Expected: FAIL, `./App` and `./ProfilesModal` do not exist.

- [ ] **Step 5: The components**

Create `tam/frontend/src/components/Placeholder.tsx`:

```tsx
import type { ViewInfo } from "../nav";

// Placeholder stands in for a view that a later phase delivers. The copy
// names the phase so nobody mistakes the empty state for a bug.
export function Placeholder({ view }: { view: ViewInfo }) {
  return (
    <section className="placeholder" aria-labelledby="view-title">
      <div className="placeholder-card">
        <div className="placeholder-glyph" aria-hidden="true" />
        <h2 className="placeholder-title">
          The {view.label} view arrives in {view.phase}
        </h2>
        <p className="placeholder-blurb">{view.blurb}</p>
        <p className="placeholder-blurb">
          This build proves the shell, the shared profiles, and the demo profile.
        </p>
      </div>
    </section>
  );
}
```

Create `tam/frontend/src/components/ProfilesModal.tsx`:

```tsx
import { useState } from "react";
import type { FormEvent } from "react";
import { Modal, useConfirm, useProfile, errMsg } from "@agile-suite/core";
import { CreateProfile, DeleteProfile, isDemoUrl } from "../api";
import type { Profile, Settings } from "../api";

// ProfilesModal lists the profiles the suite shares and creates or deletes
// them. Deleting removes the row from XTM as well, so the confirm says so.
export function ProfilesModal({ onClose }: { onClose: () => void }) {
  const { profiles, defaultProfileId, reload, setDefault } = useProfile<Profile, Settings>();
  const { confirm } = useConfirm();
  const [name, setName] = useState("");
  const [jiraUrl, setJiraUrl] = useState("");
  const [projectKey, setProjectKey] = useState("");
  const [token, setToken] = useState("");
  const [makeDefault, setMakeDefault] = useState(false);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await CreateProfile(name, jiraUrl, projectKey, token, makeDefault);
      await reload();
      setName("");
      setJiraUrl("");
      setProjectKey("");
      setToken("");
      setMakeDefault(false);
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setSaving(false);
    }
  }

  async function remove(p: Profile) {
    const ok = await confirm({
      title: `Delete ${p.name}?`,
      message: "This removes the profile from Xray Test Manager too, along with its stored token.",
      confirmLabel: "Delete",
      danger: true,
    });
    if (!ok) return;
    try {
      await DeleteProfile(p.id);
      await reload();
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <Modal onClose={onClose} className="modal profiles-modal" labelledBy="profiles-title">
      <div className="pending-head">
        <h2 id="profiles-title">Profiles</h2>
        <button className="btn btn-ghost" onClick={onClose} aria-label="Close">×</button>
      </div>
      <div className="bulk-body">
        <p className="muted">
          One list for the whole suite. A profile made here shows up in Xray Test Manager too.
        </p>
        <ul className="profile-list">
          {profiles.map((p) => (
            <li key={p.id} className="profile-row">
              <span className="profile-name">
                {p.name} ({p.projectKey})
              </span>
              <span className="muted">
                {isDemoUrl(p.jiraUrl) ? "demo" : p.jiraUrl}
                {p.id === defaultProfileId ? " · default" : ""}
              </span>
              <button className="btn" onClick={() => setDefault(p.id)}>
                {p.id === defaultProfileId ? "Clear default" : "Make default"}
              </button>
              <button className="btn btn-danger" onClick={() => remove(p)}>
                Delete
              </button>
            </li>
          ))}
          {profiles.length === 0 && <li className="muted">No profiles yet.</li>}
        </ul>
        <p className="muted small">
          Kiwi TCMS profiles from XTM are not listed; TAM talks to Jira only.
        </p>

        <form onSubmit={submit} className="profile-form">
          <h3>New profile</h3>
          <label className="field-label" htmlFor="pf-name">Name</label>
          <input id="pf-name" className="detail-input" value={name} onChange={(e) => setName(e.target.value)} />
          <label className="field-label" htmlFor="pf-url">Jira URL</label>
          <input id="pf-url" className="detail-input" value={jiraUrl} onChange={(e) => setJiraUrl(e.target.value)} placeholder="https://jira.example.com or demo" />
          <label className="field-label" htmlFor="pf-key">Project key</label>
          <input id="pf-key" className="detail-input" value={projectKey} onChange={(e) => setProjectKey(e.target.value.toUpperCase())} />
          <label className="field-label" htmlFor="pf-token">Personal access token</label>
          <input id="pf-token" className="detail-input" type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder={isDemoUrl(jiraUrl) ? "not needed for demo" : "stored in the OS credential manager"} />
          <label className="check-row">
            <input type="checkbox" checked={makeDefault} onChange={(e) => setMakeDefault(e.target.checked)} />
            Make this the default profile
          </label>
          {error && <p className="form-error" role="alert">{error}</p>}
          <div className="form-actions form-actions-end">
            <button type="button" className="btn" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={saving}>Save</button>
          </div>
        </form>
      </div>
    </Modal>
  );
}
```


Create `tam/frontend/src/components/AboutModal.tsx`:

```tsx
import { useEffect, useState } from "react";
import { Modal } from "@agile-suite/core";
import { GetDiagnostics } from "../api";
import type { Diagnostics } from "../api";

export function AboutModal({ onClose }: { onClose: () => void }) {
  const [d, setD] = useState<Diagnostics | null>(null);
  useEffect(() => {
    GetDiagnostics().then(setD).catch(() => setD(null));
  }, []);
  return (
    <Modal onClose={onClose} labelledBy="about-title">
      <div className="pending-head">
        <h2 id="about-title">About Task Activity Manager</h2>
      </div>
      <div className="bulk-body">
        <p>Agile task management for Jira Data Center. Part of the agile suite with Xray Test Manager.</p>
        {d && (
          <dl className="about-list">
            <dt>Version</dt><dd>{d.version || "dev"}</dd>
            <dt>Local store</dt><dd>{d.dbPath} (schema {d.schemaVersion})</dd>
            <dt>Shared profiles</dt><dd>{d.sharedPath}</dd>
            <dt>Log</dt><dd>{d.logPath}</dd>
            <dt>Runtime</dt><dd>{d.goVersion} on {d.os}/{d.arch}</dd>
          </dl>
        )}
        <div className="form-actions form-actions-end">
          <button className="btn btn-primary" onClick={onClose}>Close</button>
        </div>
      </div>
    </Modal>
  );
}
```

- [ ] **Step 6: The shell**

Create `tam/frontend/src/App.tsx`:

```tsx
import { useEffect, useState } from "react";
import { Menu, LiveRegion, useProfile, errMsg } from "@agile-suite/core";
import { Health, EventsOn, isDemoUrl } from "./api";
import type { HealthInfo, Profile, Settings } from "./api";
import { VIEWS, useView } from "./nav";
import { useModal } from "./modals";
import { Placeholder } from "./components/Placeholder";
import { ProfilesModal } from "./components/ProfilesModal";
import { AboutModal } from "./components/AboutModal";

// App is the shell: topbar, nav rail, the active view, and the status bar.
// It matches docs/superpowers/specs/assets/2026-09-05-tam-scaffold-shell.svg.
export default function App() {
  const { profiles, activeId, setActiveId, activeProfile, theme, setTheme, reload } =
    useProfile<Profile, Settings>();
  const { view, setView } = useView();
  const { isOpen, openModal, closeModal } = useModal();
  const [health, setHealth] = useState<HealthInfo | null>(null);

  useEffect(() => {
    Health()
      .then((h) => {
        setHealth(h);
        if (h.ok) void reload();
      })
      .catch((e) =>
        setHealth({ ok: false, error: errMsg(e), dbPath: "", sharedPath: "", logPath: "" }),
      );
  }, [reload]);

  useEffect(() => {
    const offProfiles = EventsOn("menu:profiles", () => openModal("profiles"));
    const offAbout = EventsOn("menu:about", () => openModal("about"));
    return () => {
      offProfiles();
      offAbout();
    };
  }, [openModal]);

  const current = VIEWS.find((v) => v.id === view) ?? VIEWS[0];
  const demo = isDemoUrl(activeProfile?.jiraUrl);

  return (
    <div className="app">
      <header className="topbar">
        <div className="topbar-left">
          <span className="brand">Task Activity Manager</span>
          {demo && <span className="chip chip-demo">DEMO</span>}
          <label className="sr-only" htmlFor="profile-select">Profile</label>
          <select
            id="profile-select"
            className="profile-select"
            value={activeId}
            onChange={(e) => setActiveId(e.target.value)}
          >
            {profiles.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.projectKey})
              </option>
            ))}
            {profiles.length === 0 && <option value="">No profile</option>}
          </select>
        </div>
        <div className="topbar-right">
          <button className="topbar-btn" onClick={() => openModal("profiles")}>Manage</button>
          <Menu
            label="Theme"
            align="right"
            items={["light", "dark", "system"].map((t) => ({
              key: t,
              label: t[0].toUpperCase() + t.slice(1),
              checked: theme === t,
              onClick: () => void setTheme(t),
            }))}
          />
          <Menu
            label="Help"
            align="right"
            items={[{ key: "about", label: "About", onClick: () => openModal("about") }]}
          />
        </div>
      </header>

      <nav className="nav-rail" aria-label="Views">
        {VIEWS.map((v) => (
          <button
            key={v.id}
            className={`nav-item${v.id === view ? " nav-item-active" : ""}`}
            aria-current={v.id === view ? "page" : undefined}
            onClick={() => setView(v.id)}
          >
            {v.label}
          </button>
        ))}
        <div className="nav-divider" />
        <div className="nav-section">Suite</div>
        <button className="nav-item" disabled title="The launcher arrives in Phase 6">
          Tests (XTM)
        </button>
        <div className="nav-hint">opens Xray Test Manager</div>
      </nav>

      <main className="main">
        {health && !health.ok ? (
          <div className="startup-error" role="alert">
            <h2>The local store could not be opened</h2>
            <p>{health.error}</p>
            {health.logPath && <p>Log: {health.logPath}</p>}
          </div>
        ) : (
          <>
            <div className="view-head">
              <h2 id="view-title">{current.label}</h2>
              {activeProfile && (
                <span className="muted">
                  {activeProfile.name} · {activeProfile.projectKey}
                </span>
              )}
            </div>
            <Placeholder view={current} />
          </>
        )}
      </main>

      <footer className="statusbar">
        <span className={`dot ${health?.ok ? "dot-ok" : "dot-warn"}`} aria-hidden="true" />
        <span>{health?.ok ? "Local store ready · tam.db" : "Starting up"}</span>
        <span className="muted">Profiles shared with XTM · agile-suite/profiles.db</span>
        <span className="muted statusbar-right">Theme: {theme}</span>
      </footer>

      <LiveRegion />
      {isOpen("profiles") && <ProfilesModal onClose={closeModal} />}
      {isOpen("about") && <AboutModal onClose={closeModal} />}
    </div>
  );
}
```

Create `tam/frontend/src/main.tsx`:

```tsx
import React from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import "@agile-suite/core/styles/tokens.css";
import "@agile-suite/core/styles/primitives.css";
import "./App.css";
import App from "./App";
import { profileBackend } from "./profileBackend";
import { ViewProvider } from "./nav";
import { ModalProvider } from "./modals";

const queryClient = createQueryClient();

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ViewProvider>
            <ModalProvider>
              <App />
            </ModalProvider>
          </ViewProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
```

Create `tam/frontend/src/App.css` (the shell layout; colours come from the tokens):

```css
* { box-sizing: border-box; margin: 0; padding: 0; }
html, body, #root { height: 100%; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  font-size: 13px;
  color: var(--text);
  background: var(--surface-2);
  -webkit-font-smoothing: antialiased;
}
input, button, select { font-family: inherit; font-size: inherit; }

.app {
  height: 100%;
  display: grid;
  grid-template-columns: 180px 1fr;
  grid-template-rows: 48px 1fr 28px;
  grid-template-areas: "top top" "rail main" "rail status";
}
.topbar {
  grid-area: top;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}
.topbar-left, .topbar-right { display: flex; align-items: center; gap: 12px; }
.brand { font-size: 15px; font-weight: 600; }
.chip { padding: 2px 8px; border-radius: 9px; font-size: 10px; font-weight: 600; }
.chip-demo { background: var(--warning-bg); color: var(--warning-text); border: 1px solid var(--warning); }
.profile-select {
  min-width: 232px;
  padding: 4px 8px;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--surface);
  color: var(--text);
}

.nav-rail {
  grid-area: rail;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px 8px;
  background: var(--surface);
  border-right: 1px solid var(--border);
}
.nav-item {
  text-align: left;
  padding: 7px 14px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  cursor: pointer;
}
.nav-item:hover:not(:disabled) { background: var(--row-hover); }
.nav-item:disabled { color: var(--text-muted); cursor: default; }
.nav-item-active { background: var(--accent-soft); color: var(--accent); font-weight: 600; }
.nav-divider { height: 1px; margin: 12px 8px; background: var(--border); }
.nav-section { padding: 0 14px 4px; font-size: 11px; color: var(--text-muted); }
.nav-hint { padding: 0 14px; font-size: 10px; color: var(--text-muted); }

.main { grid-area: main; padding: 20px; overflow: auto; }
.view-head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 16px; }
.view-head h2 { font-size: 16px; }
.muted { color: var(--text-muted); }
.small { font-size: 11px; }

.placeholder-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 24px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  text-align: center;
}
.placeholder-glyph { width: 60px; height: 44px; border-radius: 6px; background: var(--accent-soft); margin-bottom: 16px; }
.placeholder-title { font-size: 15px; }
.placeholder-blurb { color: var(--text-muted); }

.statusbar {
  grid-area: status;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 0 16px;
  font-size: 11px;
  background: var(--surface);
  border-top: 1px solid var(--border);
}
.statusbar-right { margin-left: auto; }
.dot { width: 8px; height: 8px; border-radius: 50%; }
.dot-ok { background: var(--ok-text); }
.dot-warn { background: var(--warning); }

.startup-error {
  padding: 16px;
  border: 1px solid var(--warn-border);
  background: var(--warn-soft);
  border-radius: 8px;
}
.startup-error h2 { font-size: 14px; margin-bottom: 8px; }

.profiles-modal { width: min(520px, 92vw); }
.profile-list { list-style: none; display: flex; flex-direction: column; gap: 6px; }
.profile-row { display: flex; align-items: center; gap: 10px; padding: 6px 8px; border-radius: 4px; }
.profile-row:hover { background: var(--row-hover); }
.profile-name { font-weight: 600; flex: 1; }
.profile-form { display: flex; flex-direction: column; border-top: 1px solid var(--border); padding-top: 12px; }
.profile-form h3 { font-size: 13px; margin-bottom: 8px; }
.check-row { display: flex; align-items: center; gap: 8px; margin: 4px 0 12px; }
.form-error { color: var(--danger-text); margin-bottom: 8px; }
.about-list { display: grid; grid-template-columns: max-content 1fr; gap: 4px 12px; margin: 12px 0; }
.about-list dt { color: var(--text-muted); }
```

- [ ] **Step 7: Run the tests, then the app**

Inside `tam/frontend/`:

```bash
npx vitest run
npm run build
```

Expected: the three test files pass (`api`, `App`, `ProfilesModal`) and `tsc` plus the Vite build are clean. Then inside `tam/`:

```bash
wails build
```

Expected: `tam/build/bin/task-activity-manager.exe`. Run it (or `wails dev`) and walk the mockup: the title and empty profile picker; Manage opens the Profiles dialog; create "Demo team" with URL `demo`, key `DEMO`, no token, default checked; the DEMO chip appears and the picker shows the profile; switch the theme to dark and back; open About and check the paths; each nav item shows its placeholder. Then open XTM (`wails dev` inside `xtm/`) and confirm "Demo team" is listed in its profile picker too.

- [ ] **Step 8: Commit**

```bash
git add package.json package-lock.json tam/frontend tam/build
git commit -m "feat(tam): the shell, profiles dialog, and placeholder views on the shared frontend core"
```

---

### Task 7: CI, dependency updates, and docs

Teach CI about the third module and the npm workspaces, and bring the guides up to date.

**Files:**
- Modify: `.github/workflows/build.yml`, `.github/dependabot.yml`, `CLAUDE.md` (root), `README.md` (root), `xtm/CLAUDE.md`, `xtm/CHANGELOG.md`
- Create: `tam/CLAUDE.md`

- [ ] **Step 1: The build workflow**

In `.github/workflows/build.yml`:

In the `test` job, after the "Go tests (core)" step add:

```yaml
      - name: Go tests (tam)
        working-directory: tam
        run: go test ./internal/...
```

Add a new job after `test`:

```yaml
  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6.4.0
        with:
          node-version: "20"
          cache: npm
      - run: npm ci
      - name: Vitest (every workspace)
        run: npm test --workspaces --if-present
      - name: Type check (every workspace)
        run: npm run typecheck --workspaces --if-present
```

In `build-windows`, after "Build (.exe)" add:

```yaml
      - name: Build TAM (.exe)
        working-directory: tam
        run: wails build
```

In `build-macos`, after "Build universal .app" add:

```yaml
      - name: Build TAM universal .app
        working-directory: tam
        run: wails build -platform darwin/universal
```

- [ ] **Step 2: Dependabot**

In `.github/dependabot.yml`, add `"/tam"` to the `gomod` `directories` list and change the npm entry's directory from `"/xtm/frontend"` to `"/"` (the workspace root owns the lock file now).

- [ ] **Step 3: Docs**

Root `CLAUDE.md`: replace the `tam/` bullet ("arrives with plan 0b") with a description of the app and its own guide, add a bullet for `frontend/core` (the shared npm package, `@agile-suite/core`), and add:

```markdown
## Frontends

The three React packages are npm workspaces. Run `npm install` once at the
repo root; `npm test --workspaces --if-present` runs every Vitest suite and
`npm run typecheck --workspaces --if-present` type-checks them. Wails does the
root install itself through each app's `frontend:install`.
```

Root `README.md`: add TAM to the overview (one paragraph: what it is, that it shares profiles with XTM, and that it is at the scaffold stage), the workspace commands above, and the Task 1 sync section if it is not already there.

`tam/CLAUDE.md` (new):

```markdown
# CLAUDE.md

Task Activity Manager (TAM) is the agile task-management app of the suite:
Jira DC tasks, epics, stories, bugs, and requirements for scrum masters,
product owners, and team members. It shares connection profiles and the
Windows Credential Manager entries with Xray Test Manager through
`core/profile` and the shared `profiles.db`. The design lives in
`docs/superpowers/specs/2026-09-04-tam-foundation-design.md`; the Outline
collection "Task Activity Manager" mirrors it.

## Status

Foundation scaffold (plan 0b): the shell, the Profiles dialog, and
placeholder views. Phase 1 adds issue sync and the Backlog.

## Layout

    main.go              Wails entry point, window, menu
    app.go               App struct: validates and delegates, nothing more
    internal/tamstore/   TAM's own SQLite file (schema version 1, no app tables yet)
    internal/suiteprofiles/  which shared profiles TAM shows, demo detection, validation
    frontend/            React app on @agile-suite/core (see ../frontend/core)
      src/api.ts         re-exports the generated bindings, defines plain shapes
      wailsjs/           GENERATED bindings, do not hand-edit

## Commands

    wails dev                      # run with hot reload
    wails build                    # build/bin/task-activity-manager.exe
    go test ./internal/...         # Go tests
    cd frontend; npx vitest run    # frontend tests
    cd frontend; npm run build     # tsc + vite build

`npm install` runs at the repo root (npm workspaces). `frontend:install` in
wails.json does that for you.

## Conventions

Same as XTM's: logic in `internal/`, `app.go` only adapts it to Wails; Jira
is the system of record; credentials go to the OS credential manager only;
`TODO(tam): desc` marks planned work. Profiles TAM creates carry backend
`jira`; Kiwi profiles from XTM are hidden. UI text uses no em dashes.
```

`xtm/CLAUDE.md`: in the Layout block add a line after `frontend/src/api.ts`:

```
  src/contexts/, components/   several files are re-exports from @agile-suite/core (frontend/core)
```

and in "Building, running, and testing" note that `npm install` happens at the repo root and that the dialog system, `Modal`, `Menu`, `LiveRegion`, the API call helpers, and the query client factory come from the shared package.

`xtm/CHANGELOG.md`: under the unreleased heading add:

```markdown
- The dialog system, the modal and menu primitives, the API call helpers,
  and the query client now come from the shared `@agile-suite/core`
  package. No behaviour change; XTM's own files re-export them.
```

- [ ] **Step 4: Verify and commit**

Run the humanizer pass over every prose change in this task. Then `git diff --stat` to confirm only the files above changed, and:

```bash
git add .github CLAUDE.md README.md tam/CLAUDE.md xtm/CLAUDE.md xtm/CHANGELOG.md
git commit -m "ci: build and test the TAM module and every frontend workspace"
```

Push the branch and confirm the four CI jobs (test, frontend, build-windows, build-macos) are green before opening the PR.

---

## Self-review notes

- **Spec coverage.** Phase 0's "scaffold TAM (Wails app, shell, demo profile)" is Tasks 4 and 6; "`frontend/core` as TAM reaches for it" is Task 5; the shared-profile seam (§7) is exercised end to end in Task 6 step 7. `core/jira`, `core/journal`, `core/backend`, and `core/demo` are not touched: nothing in this plan needs them, and the Phase 1 spec will pull them.
- **Things this plan deliberately leaves out.** A TAM icon, an installer, a release workflow, and the XTM-side removal of the duplicated CSS and chrome. Each is its own change.
- **Type consistency.** The bound signatures in Task 4 match the re-exports in Task 6's `api.ts`; `ProfileBackend<P, S>` in Task 5 is what `profileBackend.ts` implements; `createModalContext` and `createViewContext` return the same names Tasks 5 and 6 destructure.
- **Risky spots for the implementer.** npm workspace hoisting with Wails (Task 5 step 10 and Task 6 step 7 both run `wails build` to prove it); the `PRAGMA table_info` scan in Task 2 (six columns, `dflt_value` nullable); the shifted merge in Task 1 (the dry run must be aborted).
