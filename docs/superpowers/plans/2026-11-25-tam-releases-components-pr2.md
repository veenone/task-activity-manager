# TAM releases and components, PR 2: issue fields

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A user sets an issue's components and fix versions from the issue panel, from New issue, and for several issues at once from the Backlog. Each change is a pending edit that Commit sends to Jira.

**Architecture:** The two fields join the edit path `labels` already uses. The journal holds a comma list of catalog ids, the issue row holds the JSON id array PR 1 added, and `commitEdit` sends `[{"id": ...}]` arrays. New issue carries the ids on the draft and the create sends them. The Backlog gains row checkboxes and a selection bar whose two actions call one binding that journals an edit per issue in one transaction. The frontend reads the catalog PR 1 caches through two new read bindings.

**Tech Stack:** Go modules `core`, `tam`; SQLite through `core/store`; React and Vitest in `frontend/core` and `tam/frontend`; Wails bindings regenerated with `wails generate module`.

**Spec:** `docs/superpowers/specs/2026-11-23-tam-releases-components-design.md` (issue #605). This plan covers Delivery item 2 only: no Releases view, no Components view, no version or component journal types.

## Global Constraints

- Worktree `C:\projects\tam-605-2`, branch `feat/605-issue-fields`. Commits say `Refs #605`. No closing keywords and no AI attribution.
- In this worktree run `go vet ./internal/...` and `go test ./internal/...` plus `go test -run <Name> .` for the bindings, not `./...` with a fresh `frontend/dist`.
- Test first (P2). Each behaviour's commit includes a test seen failing on an assertion.
- Files stay under 400 lines (C2). `issuerepo/writes.go`, `NewIssueModal.tsx` and `IssueDetailPanel.tsx` are already over and grandfathered: new logic goes in new files, and those files gain only the lines that wire it in.
- Field names are the JSON names on `backend.Issue`, `components` and `fixVersions`, because every other editable field uses that convention (`storyPoints`, `parentKey`). The spec writes `fix_versions`, which is the column.
- No schema change: PR 1's v28 already has both columns.
- Drafted releases and components do not exist yet (PR 3 and PR 4 add their creates), so the commit-time hold for "an edit naming a draft that was not created" is not built here. The firewall refuses a negative id in either field, which is the backstop until then.

## Review Focus

1. **A released or archived version.** It stays on an issue that has it, but nothing may newly add it: the picker disables it, and `EditField` refuses it in Go too (I1). Pinned in Task 2 (`TestEditFixVersionsRefusesANewlyAddedReleasedVersion`).
2. **An id that is not in the catalog.** Refused, so a stale or forged id never reaches the journal. Pinned in Task 2.
3. **Bulk set over issues from another project.** Skipped and reported, never journaled. Pinned in Task 4 (`TestSetCatalogSkipsOtherProjects`).
4. **Add versus replace.** Add keeps the issue's other values and does not duplicate; an issue already holding the result journals nothing. Pinned in Task 4.
5. **Fix versions need Resolve Issues.** Refused in the binding and disabled in the UI with the reason. Pinned in Task 3 and Task 6.

---

### Task 1: MultiSelect accessibility (core)

**Files:** Modify `frontend/core/src/components/MultiSelect.tsx`, `frontend/core/src/components/MultiSelect.test.tsx`.

The spec asks for the audit findings to be fixed before the issue fields use the control.

- Produces new optional props: `id` (on the button, so a `<label htmlFor>` names it), `label` (the accessible name of the button and the search box), `describedBy`, `disabled`; and `MultiOption.disabled`, an option that cannot be newly checked (it can still be unchecked when selected).
- [ ] **Step 1:** Tests: the button reports `aria-expanded` false then true and `aria-haspopup="true"` (a checkbox panel is not a listbox); Escape closes the panel and returns focus to the button; the search box is named by `label` and not by its placeholder; a disabled option's checkbox is disabled unless it is checked; `disabled` disables the button.
- [ ] **Step 2:** `npx vitest run src/components/MultiSelect.test.tsx` in `frontend/core`. Expected: FAIL on the assertions.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Re-run. Expected: PASS.
- [ ] **Step 5:** Commit: `fix(core): MultiSelect states, Escape and a real label`.

### Task 2: Journal and commit the two fields (Go)

**Files:**
- Create: `tam/internal/backend/catalogids.go` (+ test), `tam/internal/issuerepo/catalogedit.go`, `tam/internal/issuerepo/catalogedit_test.go`
- Modify: `tam/internal/issuerepo/writes.go` (field list, columns, `FieldValue`, `readField`, `writeField`, `validateField`, draft JSON), `tam/internal/backend/jira/editwrite.go`, `tam/internal/backend/jira/editscreen.go`, `tam/internal/backend/demo/demo.go`, `tam/internal/committer/committer.go`, `tam/internal/committer/firewall.go`

**Interfaces:**
- `backend.FormatIDs([]int) string` gives `"10000, 10001"`; `backend.ParseIDs(string) ([]int, error)` splits on commas, trims, skips blanks, drops repeats, and fails on anything not a whole number.
- `issuerepo.EditableFields` gains `components` and `fixVersions`. The journal value is `FormatIDs`, the row stores the JSON array.
- `checkCatalogIDs(ctx, q, profileID, field string, current, next []int) error`: every id in `next` but not in `current` must be in the profile's catalog, and for `fixVersions` neither released nor archived.
- Jira: `components` and `fixVersions` go as `[{"id": "..."}]`; both are labelled for the screen guard and read from the edit screen.
- Demo: both fields are on every screen and `UpdateIssue` applies them.
- Committer: the firewall treats a key naming a version or a component like a sprint, and `commitEdit` passes the parsed ids as references.

- [ ] **Step 1: Failing tests.** `TestParseIDs` and `TestFormatIDs`; `TestEditComponentsJournalsIDsAndStoresArray` (journal `after_val` `"10000, 10001"`, row reads `[]int{10000, 10001}`, conflict value via `FieldValue`); `TestEditFixVersionsRefusesANewlyAddedReleasedVersion` (keeping it is fine); `TestEditComponentsRefusesAnUnknownID`; `TestReapplyKeepsAPendingFixVersionEdit` through a sync replace; jira `TestJiraFieldsSendsCatalogIDObjects` and `editableNames` listing both; demo `UpdateIssue` round trip; committer `TestCommitPushesFixVersions` (backend receives `"10100, 10101"`) and firewall `TestFirewallRefusesADraftVersionID`.
- [ ] **Step 2:** `cd tam && go test ./internal/backend/... ./internal/issuerepo/ ./internal/committer/`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Re-run and `go vet ./internal/...`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(tam): journal and commit components and fix versions`.

### Task 3: Resolve Issues and the catalog bindings

**Files:**
- Create: `tam/app_catalog.go`, `tam/app_catalog_test.go`
- Modify: `tam/internal/permission/permission.go`, `tam/app_permissions.go`, `tam/app_writes.go`, `tam/frontend/wailsjs/**` (regenerated)

**Interfaces:**
- `permission.ResolveIssues = "RESOLVE_ISSUES"`, named "Resolve Issues". Administer Projects waits for PR 3, which is the first PR to gate on it.
- `editPermissions(field) []string`: `assignee` needs Assign Issues, `fixVersions` needs Edit Issues and Resolve Issues, the rest Edit Issues.
- `(a *App) GetVersions(profileID) ([]backend.Version, error)` and `GetComponents(profileID) ([]backend.Component, error)`: the cached catalog of the profile's project.

- [ ] **Step 1: Failing tests.** `TestEditIssueRefusesFixVersionsWithoutResolveIssues` (Edit Issues held, Resolve Issues denied: refused naming "Resolve Issues"; a components edit still goes through); `TestGetVersionsReadsTheProfilesProject`.
- [ ] **Step 2:** Run. Expected: FAIL.
- [ ] **Step 3:** Implement, then `wails generate module` from `tam/`.
- [ ] **Step 4:** Re-run. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(tam): gate fix versions on Resolve Issues, read the catalog`.

### Task 4: Bulk set

**Files:** `tam/internal/issuerepo/catalogedit.go` (+ test), `tam/app_catalog.go` (+ test), wailsjs regenerated.

**Interfaces:**
- `(r *Repository) SetCatalog(ctx, profileID, project string, keys []string, field string, ids []int, add bool) (CatalogSetResult, error)`, `CatalogSetResult{Changed, Skipped []string}`. One transaction. `add` unions with each issue's own ids in their order; otherwise the ids replace them. A key outside `project` is skipped. A draft's create row is rewritten instead of journaled, as `EditField` does.
- `(a *App) SetIssuesCatalog(profileID string, keys []string, field string, ids []int, add bool) (issuerepo.CatalogSetResult, error)`, checking `editPermissions(field)` on every key.

- [ ] **Step 1: Failing tests.** `TestSetCatalogAddsWithoutDuplicating`, `TestSetCatalogReplaces`, `TestSetCatalogSkipsOtherProjects`, `TestSetCatalogLeavesAnUnchangedIssueAlone`, `TestSetCatalogRewritesADraft`.
- [ ] **Step 2–4:** Red, implement, green.
- [ ] **Step 5:** Commit: `feat(tam): set components or fix versions on many issues at once`.

### Task 5: New issue carries both fields

**Files:** `tam/internal/backend/backend.go` (`IssueDraft.Components`, `IssueDraft.FixVersions`), `tam/internal/issuerepo/writes.go` (`CreateDrafts` writes the columns), `tam/internal/backend/jira/writes.go` (`baseFieldIDs`, create-meta trigger), `tam/internal/backend/jira/createscreen.go` (`applyOwnFields`), `tam/internal/backend/demo/demo.go`, `tam/frontend/src/components/MetaField.tsx` (`FORM_OWNED_FIELDS`).

- Both ids are TAM's own fields now, so create-meta no longer offers them as extras. A create sends them as id objects when the screen carries them and names them as left out when it does not, the rule the estimate follows.
- [ ] **Step 1: Failing tests.** `TestCreateDraftStoresCatalogIDs`; jira `TestCreateIssueSendsComponentsAndFixVersions` and `TestCreateIssueLeavesOutFixVersionsOffTheScreen`; demo create keeps them.
- [ ] **Step 2–4:** Red, implement, green.
- [ ] **Step 5:** Commit: `feat(tam): new issues carry components and fix versions`.

### Task 6: The issue panel fields

**Files:**
- Create: `tam/frontend/src/api/catalog.ts`, `tam/frontend/src/queries/catalog.ts`, `tam/frontend/src/lib/catalog.ts` (+ test), `tam/frontend/src/components/CatalogField.tsx` (+ test)
- Modify: `tam/frontend/src/api/issues.ts` (`components`, `fixVersions` on `Issue`), `tam/frontend/src/api/edits.ts`, `tam/frontend/src/api/permissions.ts`, `tam/frontend/src/components/EditableFields.tsx`, `tam/frontend/src/queries/invalidate.ts`, `tam/frontend/src/components/PendingGroupRows.tsx`

- `catalogOptions(rows, selected)`: release date then name, drafts included; a released or archived version is labelled and disabled unless the issue already has it.
- `CatalogField` renders the shared MultiSelect for one field. An issue from another project shows it read-only with the reason; fix versions are disabled when Resolve Issues is denied.
- The Pending changes dialog names the releases and components instead of printing ids.
- [ ] **Step 1: Failing tests.** `lib/catalog.test.ts`; `CatalogField.test.tsx` (picks journal `"10000, 10001"` through Save edit; a released version is disabled; another project's issue is read-only; Resolve Issues denied disables fix versions with the reason).
- [ ] **Step 2–4:** Red, implement, green, and the workspace typecheck.
- [ ] **Step 5:** Commit: `feat(tam): components and fix versions in the issue panel`.

### Task 7: New issue fields

**Files:** `tam/frontend/src/components/NewIssueModal.tsx` (+ test), `tam/frontend/src/api/edits.ts` (`IssueDraft`).

- [ ] **Step 1: Failing test:** the draft sent carries the picked ids; with nothing picked it carries empty arrays.
- [ ] **Step 2–4:** Red, implement, green.
- [ ] **Step 5:** Commit: `feat(tam): components and fix versions in New issue`.

### Task 8: Backlog selection bar

**Files:**
- Create: `tam/frontend/src/components/CatalogBulkBar.tsx` (+ test), `tam/frontend/src/components/SetCatalogModal.tsx`
- Modify: `tam/frontend/src/components/IssueTable.tsx` (optional row checkboxes), `tam/frontend/src/components/IssueListView.tsx`, `tam/frontend/src/components/BacklogView.tsx`, `tam/frontend/src/App.css`

- The Backlog's rows gain a checkbox in the key cell; Assigned to me does not. A checked set shows the bar: the count, **Set release**, **Set components**, Clear. Each action opens a core `Modal` with the picker and an Add to / Replace choice, and Apply calls `SetIssuesCatalog`. The result names skipped issues.
- [ ] **Step 1: Failing tests:** checking two rows shows "2 issues selected"; Set release with Replace calls the binding with both keys, `fixVersions`, the ids and `false`; Resolve Issues denied disables Set release with the reason.
- [ ] **Step 2–4:** Red, implement, green.
- [ ] **Step 5:** Commit: `feat(tam): set release and components from the Backlog selection`.

### Task 9: Docs, checks, PR

**Files:** `docs/user-guide/tam/01-backlog.md`, `docs/user-guide/tam/00-getting-started.md` if it lists pending change kinds, `docs/user-guide/tam/USER_GUIDE.md` if it is kept by hand.

- [ ] **Step 1:** Write the sections with the humanizer skill in file mode.
- [ ] **Step 2:** Checks: the touched Go packages (`go vet`, `go test`), the touched Vitest files, both workspace typechecks, `npm run lint` once. The pre-commit hook runs the ratchet.
- [ ] **Step 3:** Commit: `docs(tam): issue components and fix versions in the user guide`.
- [ ] **Step 4:** Push and open `feat(tam): set components and fix versions on issues (#605, 2 of 4)`, quoting the acceptance lines it covers, with `Refs #605`.
