# TAM releases and components, PR 1: client, schema, sync, demo

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** TAM caches every fix version and component of the profile's project, and each issue's fix versions and components, from Jira and from the demo backend. Nothing in the UI changes yet.

**Architecture:** The Jira calls for components move from `xtm/internal/jira` into `core/jira`, and the version calls are added next to them. TAM reads both through a new optional backend interface, `backend.CatalogBackend`. The syncer stores them in two new tables beside the issue cache, the same way the permissions pass already runs after the issues land.

**Tech Stack:** Go 1.x modules `core`, `tam`, `xtm` (go.work); SQLite through `core/store`; tests with `testing` and `httptest`.

**Spec:** `docs/superpowers/specs/2026-11-23-tam-releases-components-design.md` (issue #605). This plan covers Delivery item 1 only.

## Global Constraints

- Worktree `C:\projects\tam-605`, branch `feat/605-releases-components`. Commits say `Refs #605`. No closing keywords and no AI attribution in commits or the PR.
- Run Go commands from the module directory. In this worktree run `go vet ./internal/...` and `go test ./internal/...`, not `./...`: `main.go` embeds `frontend/dist`, which a worktree does not have.
- Test first (P2). Each task's first commit includes a test you saw fail on an assertion, not only on a missing symbol.
- Files stay under 400 lines (C2). `tam/internal/tamstore/tamstore.go` is already 801 lines and is grandfathered; put the new DDL in a new file in that package instead of growing it.
- A profile-keyed table must be in `PurgeProfile` (`tam/internal/issuerepo/state.go`). `frontend/core/src/instruction-gate.test.ts` checks this.
- Jira is the system of record: this PR only reads in TAM. The core write methods are used by XTM now and by TAM in PRs 3 and 4.
- Ids inside TAM are `int`: the cached tables hold negative ids for drafts later (`nextDraftID`, as sprints do). Jira's ids arrive as numeric strings and are converted at the TAM backend boundary.

## Review Focus

1. **The profile's project changes.** A sync for project B must drop the synced versions and components of project A, or the Releases view would list another project's releases. Pinned in Task 3 (`TestReplaceVersionsDropsAnotherProjectsSyncedRows`).
2. **A project with no versions or components.** An empty answer is a fact and empties the synced rows. It is not "unknown". Pinned in Task 3 (`TestReplaceVersionsWithNoneEmptiesSyncedRows`).
3. **The catalog read fails mid-sync.** The issues and the watermark must still land, and the old catalog must stay. Pinned in Task 5 (`TestSyncKeepsCatalogWhenItsReadFails`).
4. **Jira answers with an id that is not a number.** Converting it must fail the catalog read, not store id 0. Pinned in Task 4 (`TestCatalogRejectsANonNumericID`).
5. **An existing cache after upgrade.** The two new issue columns read as empty, and the next sync re-reads every issue to fill them. Pinned in Task 3 (`TestVersionTwentySevenMigrationAddsCatalogColumnsAndClearsEveryWatermark`).

---

### Task 1: Move the components client into core

**Files:**
- Create: `core/jira/components.go`, `core/jira/components_test.go`
- Modify: `xtm/internal/jira/components.go` (shrinks to aliases and demo branches), `xtm/internal/jira/components_test.go` (HTTP tests leave; demo tests stay)

**Interfaces:**
- Produces, in `core/jira`:
  - `type Component struct { ID, Name, Description, LeadName, LeadDisplayName, AssigneeType string }` with the same JSON tags as today
  - `type ComponentInput struct { Project, Name, Description, LeadUserName, AssigneeType string }`
  - `(c *Client) ProjectComponentDetails(ctx, projectKey string) ([]Component, error)`
  - `(c *Client) CreateComponent(ctx, in ComponentInput) (Component, error)`
  - `(c *Client) UpdateComponent(ctx, id string, in ComponentInput) (Component, error)`
  - `(c *Client) DeleteComponent(ctx, id, moveIssuesTo string) error`
  - `(c *Client) ComponentIssueCount(ctx, id string) (int, error)`
  - `(c *Client) SearchUsers(ctx, query string) ([]User, error)`
  - `(c *Client) writeChecked(ctx, method, path string, body, out any) error` and `apiHTTPError(method, path string, code int, status string, body []byte) *HTTPError`, renamed from `componentWrite` and `componentHTTPError`. Task 2 reuses both.
- XTM keeps every exported name it has today: `type Component = corejira.Component`, `type ComponentInput = corejira.ComponentInput`. Each XTM method keeps its `isDemoURL` branch and otherwise calls `c.Client.<Method>`.

- [ ] **Step 1:** Move the HTTP tests from `xtm/internal/jira/components_test.go` to `core/jira/components_test.go`: `TestProjectComponentDetailsDecodesLead`, `TestCreateComponentSendsBody`, `TestUpdateComponentOmitsProjectAndClearsLead`, `TestDeleteComponentMovesIssues`, `TestDeleteComponentWithoutMoveSendsNoQuery`, `TestComponentWritesReport403`, `TestComponentIssueCountAndUserSearch`, `TestCreateComponentOmitsEmptyOptionalFields`. Build clients with `NewClientWithHTTP(srv.URL, "tok", srv.Client())` as `core/jira/sprintwrite_test.go` does. Keep the assertions unchanged.
- [ ] **Step 2:** `cd core && go test ./jira/ -run Component`. Expected: build failure, `undefined: Component`.
- [ ] **Step 3:** Move the code into `core/jira/components.go`. `DeleteComponent` calls `writeChecked(ctx, http.MethodDelete, path, nil, nil)` in place of its hand-built request. Before switching, check that `WriteJSONRaw` sends the `null` body a nil `body` marshals to, which Jira accepts on a DELETE. If it does not, keep the hand-built request in core.
- [ ] **Step 4:** Shrink `xtm/internal/jira/components.go` to the aliases and the demo-branching wrappers.
- [ ] **Step 5:** `cd core && go test ./jira/` and `cd xtm && go vet ./internal/... && go test ./internal/jira/ ./internal/componentadmin/...`. Expected: PASS. XTM's demo component tests are unchanged.
- [ ] **Step 6:** Commit: `refactor(core): move the Jira components client into core`.

### Task 2: Versions client in core

**Files:**
- Create: `core/jira/versions.go`, `core/jira/versions_test.go`

**Interfaces:**
- Produces:
  - `type Version struct { ID, Name, Description, StartDate, ReleaseDate string; Released, Archived bool }`, JSON tags `id name description startDate releaseDate released archived`
  - `type VersionInput struct { Project, Name, Description, StartDate, ReleaseDate string }`. `Project` is used on create only.
  - `(c *Client) ProjectVersions(ctx, projectKey string) ([]Version, error)`: `GET /rest/api/2/project/{key}/versions`
  - `(c *Client) CreateVersion(ctx, in VersionInput) (Version, error)`: `POST /rest/api/2/version`. Sends `project` and `name` always. Sends `description`, `startDate` and `releaseDate` only when not empty.
  - `(c *Client) UpdateVersion(ctx, id string, in VersionInput) (Version, error)`: `PUT /rest/api/2/version/{id}`. Sends `name` and `description` always. An empty date goes as JSON `null` so an edit can clear it.
  - `(c *Client) ReleaseVersion(ctx, id, releaseDate, moveUnfixedTo string) error`: PUT `{"released": true, "releaseDate": releaseDate}`. When `moveUnfixedTo` is set, also sends `"moveUnfixedIssuesTo": c.BaseURL()+"/rest/api/2/version/"+moveUnfixedTo`, because Jira takes a version URI there, not an id.
  - `(c *Client) ArchiveVersion(ctx, id string, archived bool) error`: PUT `{"archived": archived}`
  - `(c *Client) DeleteVersion(ctx, id, moveFixIssuesTo string) error`: `DELETE /rest/api/2/version/{id}`, adding `?moveFixIssuesTo=<id>` when set
- Every write goes through `writeChecked`, so a refusal is an `*HTTPError` with `Code` set.

- [ ] **Step 1: Write the failing tests** in `core/jira/versions_test.go`. Each uses an `httptest` server that records the method, path, query and decoded body.
  - `TestProjectVersionsDecodes`: the server returns `[{"id":"10100","name":"v2.4","description":"d","startDate":"2026-12-01","releaseDate":"2026-12-15","released":false,"archived":false},{"id":"10090","name":"v2.3","released":true,"archived":true}]` for `/rest/api/2/project/PLAT/versions`. Assert both rows field by field.
  - `TestCreateVersionOmitsEmptyOptionalFields`: `CreateVersion(VersionInput{Project:"PLAT", Name:"v3"})`. The body has exactly the keys `project` and `name`. The answer `{"id":"10200","name":"v3"}` comes back as `Version{ID:"10200", Name:"v3"}`.
  - `TestUpdateVersionClearsAnEmptyDate`: `UpdateVersion("10100", VersionInput{Name:"v2.4", ReleaseDate:"2026-12-20"})`. The body's `startDate` is present and `nil`, `releaseDate` is `"2026-12-20"`, there is no `project`, and the method and path are `PUT /rest/api/2/version/10100`.
  - `TestReleaseVersionMovesUnfixedByURI`: `ReleaseVersion("10100", "2026-12-15", "10200")`. The body is `released: true`, `releaseDate: "2026-12-15"`, `moveUnfixedIssuesTo: srv.URL+"/rest/api/2/version/10200"`.
  - `TestReleaseVersionWithoutMoveSendsNoMove`: no `moveUnfixedIssuesTo` key.
  - `TestArchiveVersionSendsOnlyArchived`: `ArchiveVersion("10100", false)`. The body is exactly `{"archived": false}`.
  - `TestDeleteVersionMovesFixIssues`: the query is `moveFixIssuesTo=10200`. Without a move, `RawQuery` is empty.
  - `TestVersionWriteReportsJiraMessage`: the server answers 400 `{"errors":{"name":"A version with this name already exists in this project."}}`. `errors.As` finds `*HTTPError` with `Code == 400`, and `Message` contains `name: A version with this name`.
- [ ] **Step 2:** `cd core && go test ./jira/ -run Version`. Expected: build failure on `undefined: Version`. Add a stub that returns zero values so the run then fails on assertions; that is the red to record.
- [ ] **Step 3:** Implement `core/jira/versions.go` to the interface above.
- [ ] **Step 4:** `cd core && go test ./jira/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(core): Jira fix version client`.

### Task 3: Schema v28 and the catalog repository

**Files:**
- Create: `tam/internal/tamstore/catalog.go` (`catalogDDL`), `tam/internal/issuerepo/catalog.go`, `tam/internal/issuerepo/catalog_test.go`
- Modify: `tam/internal/tamstore/tamstore.go` (`Version: 27`, add `catalogDDL` to `Base`, migration 27, issue DDL columns), `tam/internal/tamstore/tamstore_test.go`, `tam/internal/issuerepo/issues.go` (columns, upsert, scan), `tam/internal/issuerepo/state.go` (`PurgeProfile` list), `tam/internal/backend/backend.go` (`Issue` fields)

**Interfaces:**
- Produces, in `tam/internal/backend` (new file `catalog.go`):
  - `type Version struct { ID int; Project, Name, Description, StartDate, ReleaseDate string; Released, Archived, Draft bool }`, JSON tags in camelCase
  - `type Component struct { ID int; Project, Name, Description, LeadName, LeadDisplayName, AssigneeType string; Draft bool }`
  - On `Issue`: `Components []int \`json:"components"\`` and `FixVersions []int \`json:"fixVersions"\``, never nil once read from the store
- Produces, in `issuerepo`:
  - `(r *Repository) ReplaceVersions(ctx, profileID, project string, vs []backend.Version) error`
  - `(r *Repository) ReplaceComponents(ctx, profileID, project string, cs []backend.Component) error`
  - `(r *Repository) Versions(ctx, profileID, project string) ([]backend.Version, error)`: ordered by release date with undated last, then name
  - `(r *Repository) Components(ctx, profileID, project string) ([]backend.Component, error)`: ordered by name, case-insensitive
- DDL (`catalogDDL`):
  - `project_version(profile_id TEXT NOT NULL, project TEXT NOT NULL DEFAULT '', id INTEGER NOT NULL, name, description, start_date, release_date TEXT NOT NULL DEFAULT '', released, archived, draft INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (profile_id, id))`
  - `project_component(profile_id, project, id INTEGER, name, description, lead_name, lead_display_name, assignee_type TEXT NOT NULL DEFAULT '', draft INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (profile_id, id))`
  - On `issue`: `components TEXT NOT NULL DEFAULT '[]'` and `fix_versions TEXT NOT NULL DEFAULT '[]'`
- Replace semantics: in one transaction, delete every row of the profile with `draft = 0`, whatever its project, then insert the given rows. Draft rows stay. The pending-change guard the spec asks for arrives with the journal types in PR 3; this PR has no pending catalog changes.

- [ ] **Step 1: Write the failing tests.**
  - `tamstore_test.go`: `TestVersionTwentySevenMigrationAddsCatalogColumnsAndClearsEveryWatermark`, in the shape of `TestVersionFourteenMigrationAddsAssigneeNameAndClearsEveryWatermark`. Build a v26 file holding an issue row and a `sync_state` row with `last_synced` set, then open it. The issue's `components` and `fix_versions` read `'[]'`, both catalog tables exist, and `last_synced` is `''`.
  - `catalog_test.go`:
    - `TestReplaceVersionsRoundTrips`: store two versions, read them back equal, ordered with the dated one first.
    - `TestReplaceVersionsKeepsDrafts`: insert a draft row with id `-1` straight into the table, `ReplaceVersions` with one synced row, and both come back.
    - `TestReplaceVersionsDropsAnotherProjectsSyncedRows`: replace for `A` with id 1, then for `B` with id 2. `Versions(.., "A")` is empty.
    - `TestReplaceVersionsWithNoneEmptiesSyncedRows`: replace with one row, then with `nil`. `Versions` is empty.
    - `TestReplaceComponentsRoundTrips`: covers lead and assignee type.
    - `TestPurgeProfileDropsTheCatalog`: both tables hold no row of the purged profile, and another profile's rows remain.
  - In the existing issuerepo issue tests, `TestUpsertPageKeepsComponentsAndFixVersions`: upsert `Components: []int{10000}, FixVersions: []int{10100, 10101}` and read the same back through `Issue`. An issue upserted with nil reads `[]int{}`.
- [ ] **Step 2:** `cd tam && go test ./internal/tamstore/ ./internal/issuerepo/`. Expected: FAIL. The migration test fails on the missing column; the others fail to build until the stub signatures exist, then fail on assertions.
- [ ] **Step 3:** Implement. Migration 27 follows version 26's shape: `store.AddColumnIfMissing` for both issue columns, then `UPDATE sync_state SET last_synced = ''`. Clearing the watermark is what makes the next sync re-read every issue. Add both tables to the `PurgeProfile` list.
- [ ] **Step 4:** `cd tam && go test ./internal/tamstore/ ./internal/issuerepo/` passes. Then from the repo root run `npx vitest run frontend/core/src/instruction-gate.test.ts --root frontend/core`; the purge gate passes.
- [ ] **Step 5:** Commit: `feat(tam): cache fix versions and components (schema v28)`.

### Task 4: Backends read the catalog and the two issue fields

**Files:**
- Modify: `tam/internal/backend/catalog.go` (add the interface), `tam/internal/backend/jira/fields.go` (`baseFields`, `parseIssue`), `tam/internal/backend/jira/fields_test.go`, `tam/internal/demo/demo.go` (tag dataset issues), `tam/internal/demo/demo_test.go`
- Create: `tam/internal/backend/jira/catalog.go`, `tam/internal/backend/jira/catalog_test.go`, `tam/internal/demo/catalog.go`, `tam/internal/backend/demo/catalog.go`

**Interfaces:**
- Consumes: Task 1's `ProjectComponentDetails`, Task 2's `ProjectVersions`, and Task 3's types.
- Produces:
  ```go
  // CatalogBackend reads a project's fix versions and components. Optional,
  // like PermissionBackend: the syncer skips it when a backend lacks it.
  type CatalogBackend interface {
      ProjectVersions(ctx context.Context, projectKey string) ([]Version, error)
      ProjectComponents(ctx context.Context, projectKey string) ([]Component, error)
  }
  ```
  - `tam/internal/demo`: `Versions(projectKey string) []backend.Version` and `Components(projectKey string) []backend.Component`, each returning a fresh slice
  - Both `backend/jira.Backend` and `backend/demo.Backend` satisfy `CatalogBackend` (`var _ backend.CatalogBackend = (*Backend)(nil)`)
- Demo data: four versions, with ids 10100 to 10103.
  - `v1.0`: released and archived, release date 60 days before the dataset's `base`.
  - `v1.1`: released, 14 days before.
  - `v1.2`: unreleased, start 7 days before, release 14 days after.
  - `v1.3`: unreleased, no dates.
  - Five components, with ids 10000 to 10004: `Auth`, `Board`, `Reports`, `Sync`, `UI`. One has a lead.
  - Every dataset issue gets one or two components by index. Issues in the closed sprint get `v1.1`, the active sprint's get `v1.2`, and the rest of the backlog is split between `v1.3` and none.

- [ ] **Step 1: Write the failing tests.**
  - `fields_test.go`: `TestParseIssueReadsComponentsAndFixVersions`. Raw fields `components: [{"id":"10000","name":"Auth"}]` and `fixVersions: [{"id":"10100"},{"id":"10101"}]` give `[]int{10000}` and `[]int{10100, 10101}`. Absent fields give empty, non-nil slices. Also assert `baseFields` contains `components` and `fixVersions`.
  - `catalog_test.go` (jira): `TestCatalogConvertsVersionsAndComponents` uses an `httptest` server answering both endpoints and asserts ids converted, `Project` set to the key, and `Draft` false. `TestCatalogRejectsANonNumericID` answers a version id of `"abc"`, and `ProjectVersions` returns an error naming it.
  - `demo_test.go`: `TestDemoIssuesNameOnlyCatalogIDs` checks that every id on every `Issues("DEMO")` row exists in `Versions("DEMO")` or `Components("DEMO")`, that at least one issue has each unreleased version, and that `Versions` returns a copy (mutating it does not change the next call).
- [ ] **Step 2:** `cd tam && go test ./internal/backend/... ./internal/demo/`. Expected: FAIL.
- [ ] **Step 3:** Implement. `parseIssue` decodes both fields as `[]struct{ ID string \`json:"id"\` }` and skips an id that does not parse; an issue row is not worth failing over one bad id. The catalog reads fail on a bad id instead, because a row stored as id 0 would collide.
- [ ] **Step 4:** Re-run. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(tam): read fix versions and components from Jira and the demo`.

### Task 5: Sync stores the catalog

**Files:**
- Create: `tam/internal/syncer/catalog.go`, `tam/internal/syncer/catalog_test.go`
- Modify: `tam/internal/syncer/syncer.go` (one call next to `e.syncPermissions` at line 153)

**Interfaces:**
- Consumes: `backend.CatalogBackend` and Task 3's `ReplaceVersions` and `ReplaceComponents`.
- Produces: `(e *Engine) syncCatalog(ctx context.Context, profileID, projectKey string)`. It returns nothing. It has the same contract as `syncPermissions`: a missing capability, a failed read or a failed write is logged with `log.Printf("tam: ...")` and leaves the stored catalog alone. Versions and components are independent: one failing does not skip the other.

- [ ] **Step 1: Write the failing tests** with the fake backend the existing syncer tests use, extended with the two catalog methods.
  - `TestSyncStoresCatalog`: after `Sync`, `repo.Versions` and `repo.Components` equal what the fake answered.
  - `TestSyncKeepsCatalogWhenItsReadFails`: seed the catalog, then make the fake's `ProjectVersions` fail. `Sync` returns no error, the issues are upserted, the stored versions are unchanged, and the components are refreshed.
  - `TestSyncWithoutCatalogBackendLeavesCatalogEmpty`: a backend without the methods syncs normally.
- [ ] **Step 2:** `cd tam && go test ./internal/syncer/ -run Catalog`. Expected: FAIL on the empty catalog.
- [ ] **Step 3:** Implement and wire the call.
- [ ] **Step 4:** `cd tam && go vet ./internal/... && go test ./internal/syncer/ ./internal/issuerepo/ ./internal/backend/...`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(tam): refresh fix versions and components on every sync`.

### Task 6: Docs, gates, PR

**Files:**
- Modify: `agents/project/glossary.md` (add a **Release** entry: a Jira fix version of the profile's project; TAM's UI says release and the code says version), `agents/project/domain-context.md` (a short note that `PUT /version/{id}` takes `moveUnfixedIssuesTo` as a version URI and that `DELETE /version/{id}` takes `moveFixIssuesTo` as an id)

- [ ] **Step 1:** Write both entries with the humanizer skill in file mode.
- [ ] **Step 2:** Run the checks for this change once (minimal-checking): `cd core && go test ./jira/`; `cd tam && go vet ./internal/... && go test ./internal/...`; `cd xtm && go test ./internal/jira/ ./internal/componentadmin/...`; then `npm run lint` and `bash scripts/ratchet.sh` from the worktree root. Expected: all pass, and the ratchet holds.
- [ ] **Step 3:** Commit: `docs: release in the glossary, Jira version API quirks`.
- [ ] **Step 4:** Push the branch and open the PR titled `feat(tam): cache fix versions and components (#605, 1 of 4)`.
  - The body quotes #605's acceptance lines this PR covers and links the spec under `## Spec`.
  - It says what is left for PRs 2 to 4, and ends with `Refs #605`.
  - No screenshots: no screen changes.
