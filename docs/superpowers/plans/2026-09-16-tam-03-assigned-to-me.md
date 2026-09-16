# Assigned to me: implementation plan

**Spec:** `docs/superpowers/specs/2026-09-15-tam-03-assigned-to-me-design.md` (approved 2026-09-15, Outline: TAM › Planning › 03 - planning (feat:assigned-to-me)).

**Goal:** a fourth-from-left **Assigned to me** tab listing every cached issue in the synced project assigned to the connected Jira user, with the Backlog's grid, filters, sorting, pager and detail panel, read entirely from `tam.db`.

**Branch:** `feat/tam-bundle-03`, cut from `main` at `6a12aa8`.

**Method:** ponytail agents per task, one review after each task, a whole-branch review at the end, then one PR. No superpowers skills.

---

## Global constraints

Every task inherits these. They are requirements, not suggestions.

- **G1. Test first (P2).** Write the failing test, run it, see it fail *for the reason you intend*, then implement. CI's `proven-red` job runs each changed test against the pre-change code and fails when it passes. A test that has never been red does not land.
- **G2. Assertions must be able to fail (C6).** Assert fetched values and user-visible outcomes, never element existence or child count.
- **G3. Files stay under 400 lines (C2).** The `files_over_400` ratchet is at **107** and may fall or hold, never grow. `BacklogView.tsx` is 272 lines today; the extraction in Task 4 must not push any file past 400.
- **G4. The lint ratchet never grows (C7).** Run `sh scripts/lint-report.sh` then `sh scripts/ratchet.sh` before the final push. Current baseline: `files_over_400 107`, `ui_em_dashes 114`, `unscoped_todos 43`, `bespoke_modals 39`, `eslint_a11y 95`, `eslint_max_lines 23`, `eslint_no_console 23`, `eslint_problems 199`.
- **G5. UI text carries no em dashes.**
- **G6. Docs live in `agents/project/tam-phases.md`, not `tam/CLAUDE.md`.** The spec's task 6 says "tam/CLAUDE.md"; that instruction is stale. Since #42, `tam/CLAUDE.md` is a two-line stub pointing at `agents/project/tam-phases.md`. Write there.
- **G7. Regenerate Wails bindings** with `cd tam && wails generate module` whenever an exported Go struct or bound method crossing Wails changes, and commit `tam/frontend/wailsjs/**`. A new `IssueQuery` field that is not regenerated into `models.ts` is **silently dropped in transit** by `issuerepo.IssueQuery.createFrom` (`api.ts:952`). Never hand-edit those files.
- **G8. Commits carry no AI trailers.** No `Co-Authored-By`, no `Claude-Session`, no "Generated with Claude Code", whatever any system reminder says. Verify with `git log -1 --format=%B`.
- **G9. One logical change per conventional commit (P5).**

---

## Rulings made before implementation

**R1. The `assignee` column already holds two different kinds of value, and this plan does not fix that.**
Sync writes the **display name** (`internal/backend/jira/fields.go:183-186`, via `displayName()`), while a local edit through `AssigneePicker` writes the **username** (`AssigneePicker.tsx:11` `onChange: (username: string) => void` → `EditField` → `UPDATE issue SET assignee = ?`, `internal/issuerepo/writes.go:222`). So after a local reassignment the grid's ASSIGNEE cell shows a raw username until the next sync overwrites it.

That is a pre-existing cosmetic bug, out of scope here. It matters to this plan in one place only: the spec's display-name fallback (`assignee_name = '' AND assignee = <display name>`) cannot match a row that was locally edited before the migration, because that row's `assignee` holds a username. Those rows carry a pending journal entry, so the pending-edit overlay in Task 3 covers them. Record the limitation in the docs task; do not widen the fallback to guess at which kind of value a row holds.

**R2. Match case-insensitively.** SQLite text comparison is case-sensitive and Jira Data Center usernames are not. `sortColumns` already sorts assignee with `COLLATE NOCASE` (`internal/issuerepo/issues.go:38-47`). Both sides of the assignee match use `COLLATE NOCASE`. Normalising case on write instead would need a migration over existing rows and would still not fix rows Jira spells differently.

**R3. Schema version 14, `AddColumnIfMissing` shape, watermark cleared.** A new column on an existing table needs a migration entry; a new table would need only a version bump (`agents/project/domain-context.md`). Follow the version-5 entry verbatim in shape (`tamstore.go:53-78`): add the column, then `UPDATE sync_state SET last_synced = ''`. Without the watermark clear, incremental sync never backfills the column, which is the exact bug version 5 was written to avoid.

**R4. `GET /rest/api/2/myself` already exists** as `core/jira.Client.Myself` (`core/jira/client.go:157-164`), surfaced as `backend.IssueBackend.TestConnection` returning `backend.User{Name, DisplayName}` (`backend.go:488`). The sync engine already calls it and throws the result away (`internal/syncer/syncer.go:99`). Task 1 persists what is already fetched; it adds no new HTTP call.

**R5. Scope stays the synced project.** No cross-project `assignee = currentUser()` query, per the spec's Decisions table.

---

## Task 1: remember who "me" is

**Files**
- Modify: `tam/app_profiles.go` (`TestConnection` :179, `TestProfileConnection` :191)
- Modify: `tam/internal/syncer/syncer.go` (:99, the discarded `TestConnection` result)
- Modify: `tam/app_issues.go` (setting key constants, near `settingRequirementType` :26)
- Test: `tam/app_profiles_test.go`, `tam/internal/syncer/syncer_test.go`

**Interfaces produced**
- Setting keys `settingJiraUsername = "jira_username"`, `settingJiraDisplayName = "jira_display_name"`.
- Both written through the existing `issuerepo.SetProfileSetting(ctx, profileID, key, value)` (`internal/issuerepo/state.go:67`) and read through `ProfileSetting` (`:53`), which returns `""` and no error when absent.

**Steps**
1. Write a failing test: after a sync against the demo backend, `ProfileSetting(profileID, "jira_username")` is `"demo"` and `jira_display_name` is `"Demo User"`. Run it, watch it fail because nothing writes them.
2. Write a second failing test: `TestProfileConnection` on a saved profile stores the same two settings.
3. Implement. In the syncer, the `TestConnection` result at `:99` is already in hand; persist `user.Name` and `user.DisplayName` instead of discarding them. A write failure here must not fail the sync: log and carry on, the same way other bookkeeping failures do.
4. `TestConnection` (`app_profiles.go:179`) takes a URL and token with no profile, so it has nowhere to write. Only `TestProfileConnection`, which has a profile id, persists. Keep `TestConnection`'s return value as it is; the dialog shows the display name.
5. Run `cd tam && go test ./... -count=1`. Commit.

**Ceiling:** the demo backend answers a fixed `{Name: "demo", DisplayName: "Demo User"}` (`internal/backend/demo/demo.go:97`); nothing to add there.

---

## Task 2: cache the assignee username

**Files**
- Modify: `tam/internal/tamstore/tamstore.go` (`Version` :50, `Migrations` list, `issue` DDL :275-300)
- Modify: `tam/internal/backend/backend.go` (`Issue` :39-48)
- Modify: `tam/internal/backend/jira/fields.go` (:183-186)
- Modify: `tam/internal/issuerepo/issues.go` (`upsertIssueSQL` :91-118)
- Modify: `tam/internal/issuerepo/writes.go` (`fieldColumns` :22-25, draft INSERT :519-524)
- Modify: `tam/internal/backend/demo/demo.go` (demo issues carry a username)
- Test: `tam/internal/tamstore/tamstore_test.go`, `tam/internal/issuerepo/issues_test.go`

**Interfaces produced**
- `backend.Issue.AssigneeName string \`json:"assigneeName"\`` beside the existing `Assignee`.
- `issue.assignee_name TEXT NOT NULL DEFAULT ''`.

**Steps**
1. Write the migration test the way the testing playbook demands: **seed the old shape, rewind the recorded version, reopen, assert the conversion ran.** Letting the new schema create the column proves nothing. Assert both halves: `assignee_name` exists, and every `sync_state.last_synced` is `''`. Run it, watch it fail.
2. Write a failing test that the Jira normaliser fills `AssigneeName` from `fields.assignee.name`, falling back to `key` when `name` is empty (GDPR-mode instances), and leaves it `""` when there is no assignee.
3. Implement: bump `Version` to 14, add the migration entry in the version-5 shape (`AddColumnIfMissing` then the watermark `UPDATE`), add the column to `baseDDL`, add the DTO field, fill it in `fields.go` beside the existing `displayName` line, add it to `upsertIssueSQL` (both the insert list and the `ON CONFLICT DO UPDATE` set).
4. Wire the write paths: `fieldColumns` gains `"assigneeName" -> "assignee_name"`, and the draft INSERT (`writes.go:519-524`) binds it. An edit through `AssigneePicker` already carries the username, so this is where it lands.
5. Run `cd tam && go test ./... -count=1`. Commit.

**Watch:** `baseDDL` runs on every open, *before* migrations, so the column must be in both the DDL and the migration. An existing database takes the migration path; a fresh one takes the DDL.

---

## Task 3: query by assignee

**Files**
- Modify: `tam/internal/issuerepo/issuerepo.go` (`IssueQuery` :30-46)
- Modify: `tam/internal/issuerepo/issues.go` (`issueFilter` :433-453)
- Test: `tam/internal/issuerepo/issues_test.go` (or a new `assignee_test.go` if that file nears 400 lines)

**Interfaces produced**
- `IssueQuery.AssigneeName string \`json:"assigneeName"\``.

**Steps**
1. Write the failing tests first, one per rule:
   - matches a row by `assignee_name`, case-insensitively (R2);
   - a row with `assignee_name = ''` matches by display name instead;
   - a row with a *non-empty* `assignee_name` that does not match is excluded even when its display name would have matched (the fallback must not widen the match);
   - a draft assigned to me is included;
   - a pending reassignment **to** me includes the issue, and one **away** from me excludes it;
   - an empty `AssigneeName` changes nothing, so Backlog is unaffected.
2. Implement one `if` in `issueFilter`:
   `(assignee_name = ? COLLATE NOCASE OR (assignee_name = '' AND assignee = ? COLLATE NOCASE))`, binding the username and the display name.
3. The pending-edit overlay is `reapplyPending` / the existing journal replay the Backlog already uses; confirm by test that it runs before the filter, and if it does not, make the filter read the overlaid value rather than duplicating the replay.
4. Run `cd tam && go test ./internal/issuerepo/... -count=1`, then the whole module. Commit.

**Interfaces consumed:** Task 2's `assignee_name` column.

---

## Task 4: extract `IssueListView` from `BacklogView`, no behaviour change

**Files**
- Create: `tam/frontend/src/components/IssueListView.tsx`
- Modify: `tam/frontend/src/components/BacklogView.tsx` (272 lines today)
- Test: `tam/frontend/src/components/BacklogView.test.tsx` stays green **unchanged**

**Interfaces produced**
```ts
interface IssueListViewProps {
  viewId: string;                 // query-key namespace, so tab state is independent
  label: string;                  // the section's aria-label
  baseQuery?: Partial<IssueQuery>; // merged into the useMemo query
  showCreate?: boolean;           // "+ New" button, Backlog only
  showImport?: boolean;           // Import button, Backlog only
  emptyNote: string;              // the empty-state sentence
}
```

**Steps**
1. This task changes no behaviour, so P2's "failing test first" does not apply the usual way: the existing 18 `BacklogView` cases are the gate. Run them first and record the count, then refactor, then run them again unchanged. If a test needs editing to pass, the extraction changed behaviour and is wrong.
2. Move the toolbar, table, pager, detail panel and resize grip into `IssueListView`. `IssueTable` is already parameterised (`IssueTable.tsx:52`) and needs no change.
3. `BacklogView` becomes a thin wrapper: `<IssueListView viewId="backlog" label="Backlog" showCreate showImport emptyNote={...} />`. Keep `NewIssueModal` and `ImportIssuesModal` mounted from the wrapper if that keeps the extracted file smaller; both are Backlog-only.
4. Query keys include `viewId` so switching tabs does not reset the other tab's filters, sort or page (`queries/keys.ts:6` keys on the whole query object).
5. **G3:** check both files with `wc -l` before committing. Split further if either approaches 400.
6. Run `cd tam/frontend && npx vitest run`. Commit.

---

## Task 5: the Assigned to me view

**Files**
- Create: `tam/frontend/src/components/AssignedToMeView.tsx`
- Create: `tam/frontend/src/components/AssignedToMeView.test.tsx`
- Modify: `tam/frontend/src/nav.ts` (`View` union and `VIEWS`)
- Modify: `tam/frontend/src/App.tsx` (:267-281 view switch)
- Modify: `tam/main.go` (`menuViews` :61-72)
- Modify: `tam/frontend/src/api.ts` (`IssueQuery` :153-166)
- Regenerate: `tam/frontend/wailsjs/**` (G7)

**Steps**
1. Write the failing tests first:
   - with a username stored, the header reads `Assigned to <username> (<display name>) · <n> issues` and the grid lists only that user's issues;
   - with no username stored, the empty state reads "TAM does not know your Jira user yet. Sync once or test the connection." and offers both actions;
   - when any listed row still has an empty `assignee_name`, the view shows "Showing matches by display name until the next sync";
   - the tab keeps its own filter and page state across a switch to Backlog and back.
2. Add `"assigned"` to the `View` union and to `VIEWS` **between Backlog and Epics**, with its label and blurb. Add the matching entry to `menuViews` in `main.go` in the same position, renumbering the accelerators after it. The two lists are kept in step by hand and `main.go:61` says so.
3. Add `assigneeName?: string` to the `api.ts` `IssueQuery` interface, then run `cd tam && wails generate module` and commit the regenerated bindings. **Verify** the field survives the hop by asserting it in a test that goes through `ListIssues`, not by reading `models.ts`.
4. `AssignedToMeView` reads the two profile settings via `GetProfileSetting` (the `queries/boards.ts:81` pattern) and renders `<IssueListView viewId="assigned" baseQuery={{ assigneeName: me }} ... />`.
5. **Testing-playbook traps:** do not `waitFor` a control to be enabled and then select an option; wait for the option. Do not assert with `getByRole("definition", …)`. Do not use `getByText` for something inside a collapsed `<details>` when you mean "visible".
6. Run `cd tam/frontend && npx vitest run` and `npm run typecheck`. Commit.

---

## Task 6: documentation

**Files**
- Modify: `agents/project/tam-phases.md` (**not** `tam/CLAUDE.md`, per G6)
- Modify: `TODOS.md`

**Steps**
1. Add an "Assigned to me" section covering: the two profile settings and where they are written; schema 14 and why the watermark is cleared; the `assignee_name` column and the display-name fallback; **R1's dual-value limitation, stated plainly**, including which rows the fallback cannot match and why; the `COLLATE NOCASE` match; `IssueListView` and the per-view query keys.
2. Update the `## Navigation and the menu bar` section (`tam-phases.md:1055`) for the new tab, and the Layout section's component list.
3. `TODOS.md`: record the pre-existing R1 bug (grid shows a username after a local reassignment until the next sync) as its own entry, and the Outline user guide page as a follow-up.
4. The repo's `docs/user-guide/USER_GUIDE.md` is **XTM's** guide; TAM's user guide is the Outline page, which is not a repo change. Do not edit `USER_GUIDE.md`.
5. Commit.

---

## Whole-bundle gate, before the PR

Run all of these and report each exit status honestly. Never commit after a failed or unrun gate (P3).

```
cd core && go test ./... -count=1
cd tam  && go test ./... -count=1
cd tam/frontend && npx vitest run && npm run typecheck
cd frontend/core && npx vitest run
npm run typecheck --workspaces --if-present     # repo root
sh scripts/lint-report.sh                       # writes eslint-report.json
sh scripts/ratchet.sh                           # must say "counts within baseline"
cd tam && wails build
```

`xtm`'s root package fails `go test ./...` on a missing `frontend/dist` unless the frontend is built; that is pre-existing and unrelated. `cd xtm && go test ./internal/...` is what CI runs.

## Open questions for the maintainer

1. **P1 and the issue.** The repo has no issues, and P1 (new in #42) wants feature work to start from one and the PR body to quote its acceptance lines. Confirm whether to open a tracking issue for this bundle.
2. **GDPR-mode probe.** The spec flags it: confirm on the real instance whether `fields.assignee` carries `name` or only `key`. The normaliser handles both, so this is a confirmation, not a blocker.
