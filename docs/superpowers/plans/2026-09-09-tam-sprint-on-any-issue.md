# Task Activity Manager: a sprint on any issue

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax. Lean cycle: implementers write the tests named here but run no suites, except the one Go run at the end of Task 1. Task 3 runs every gate once.

**Goal:** an issue can be put in a sprint from anywhere it appears, and a new or imported issue can be created straight into one.

**Why this exists:** Phase 3 put sprints on the board and nowhere else. The detail panel offers a sprint only when something hands it a board's sprint list, so from the Backlog and the Epics tree the field prints a name and cannot be changed. The New issue dialog has no sprint at all, and a draft's `SprintID` is board-drag state the create deliberately never sends. The import template has no sprint column. So the one view a user spends most of their day in cannot do the thing sprint planning is made of.

**Architecture:** `boardrepo` gains a profile-wide read of the open sprints across every board it knows, so a view with no board still has a list to offer. The detail panel takes that list in the Backlog and the Epics tree, and its existing `SprintField` becomes a real choice there. The New issue dialog gains a Sprint select, and a draft created into a sprint gets its sprint pushed after the create, through the sprint-move path the committer already has, rather than by sending a field on create that most Data Center screens do not carry.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

## Global Constraints

- Nothing under `xtm/` changes. Go commands run from inside the module directory.
- Every write here is journaled and pushed on Commit, exactly as Phase 3b built it. Nothing in this change reaches Jira outside the committer.
- **A draft's sprint is pushed after the create, not sent with it.** The Sprint field is not on most Data Center create screens, and the create path's own comment already says it sends none of the board state. The Agile move endpoint is the reliable one, and the committer already pushes sprint moves and already repoints a draft's rows when `Rekey` gives it a real key.
- A sprint list offered away from a board is the open sprints of every board the profile has synced, each labelled with its board when two boards have a sprint of the same name, because "Sprint 12" alone is a guess when a project has three boards.
- A profile with no synced boards has no sprint list, and the field says that rather than offering an empty select.
- Files stay small and single purpose; a helper used from two places lives in its own module. TAM mirrors XTM's design language where XTM has a counterpart.
- UI text uses no em dashes. No AI attribution anywhere. Conventional commit prefixes, no trailers. Never add, commit, or delete untracked local tooling files; revert Wails churn under `tam/frontend/wailsjs/runtime` and `tam/frontend/package.json.md5` with `git checkout --`.

## Decisions

1. **One profile-wide list, not a board picker on the Backlog.** A user in the Backlog is thinking about an issue, not about a board, and making them choose a board first to reach a sprint would be the app's own model leaking into their task.
2. **The existing `SprintField` is reused, not forked.** It already journals through `MoveIssueToSprint`, keeps a closed sprint as its own option, and disables itself while a sync or commit runs. All it lacked was a list.
3. **The import gains a Sprint column** because a planning spreadsheet is where a sprint's contents usually start, and the importer already writes every other draft field.

---

### Task 1: The sprint list and the create path

**Files:** modify `tam/internal/boardrepo/boards.go` and its test, `tam/app_boards.go`, `app_boards_test.go`, `tam/internal/committer/committer.go` or its create path and test, `tam/internal/importer/importer.go`, `template.go`, and their tests; regenerate `tam/frontend/wailsjs/**`.

**Produces:** `boardrepo.OpenSprints(ctx, profileID string) ([]SprintChoice, error)` with `SprintChoice{ID int; Name, BoardName string; State string}`; the bound method `ListOpenSprints(profileID string) ([]boardrepo.SprintChoice, error)`; the importer's `Sprint` column.

- [ ] **Step 1: The read.** Every sprint whose state is active or future, across every board of the profile, ordered active before future then by start date, each carrying its board's name. A profile with no boards returns an empty slice and no error. Test: two boards each with sprints, a closed sprint left out, and the board name coming back.

- [ ] **Step 2: The binding.** `ListOpenSprints` beside the board methods, shaped like the other reads: check the store, check the profile, return a non-nil slice. No guard, since it is a local read.

- [ ] **Step 3: A draft's sprint reaches Jira.** A draft created with a sprint currently keeps it in the draft JSON and drops it at the create. After the create pass rekeys the draft, journal a sprint move for the real key so the board pass pushes it in the same Commit. Put it where `Rekey` already repoints a draft's other rows, so a draft's sprint travels the way its parent already does. A draft with no sprint journals nothing. Test: a draft created into a sprint lands in Jira with a sprint move after its create, in that order; a draft with no sprint pushes no move; a create that fails leaves the sprint intent alone for the next Commit.

- [ ] **Step 4: The import column.** `Sprint` joins the importer's columns, matched by the sprint's name against the profile's open sprints, case-insensitively. A name that matches nothing is a row error naming the sprint and listing what was available, the way an unknown type already reads. An empty cell means the backlog. `SaveImportTemplate` gains the column, its dropdown listing the profile's open sprints, and a line on the notes sheet. Test: a row naming an open sprint, a row naming a closed one, a row naming nothing, and a row whose sprint does not exist.

- [ ] **Step 5: The one Go suite run.** Inside `tam/`: `go build ./... && go vet ./... && go test ./... -count=1`. Fix what fails, rerun at most twice, report. Commit as `feat(tam): a sprint list that does not need a board`.

---

### Task 2: The sprint on every issue

**Files:** modify `tam/frontend/src/queries/boards.ts`, `keys.ts`, `components/IssueDetailPanel.tsx`, `SprintField.tsx`, `BacklogView.tsx`, `EpicsView.tsx`, `NewIssueModal.tsx`, `api.ts`, their tests, and `App.css` if a rule is needed.

- [ ] **Step 1: The query.** `useOpenSprints(profileId)` over the new binding, with the same profile guard the other board queries use, invalidated by `invalidateProfileData` so a sync that brings new sprints refreshes the list.

- [ ] **Step 2: The panel, everywhere.** `BacklogView` and `EpicsView` hand the panel the profile's open sprints, so `SprintField` offers a choice there rather than printing a fact. The Boards view keeps passing its own board's sprints, which are the right ones when a board is on screen. `SprintField` shows the board's name beside a sprint only when two sprints share a name, and says "No sprints yet, sync a board first" when the list is empty rather than offering an empty select.

- [ ] **Step 3: The New issue dialog.** A Sprint select above the description, defaulting to the backlog, listing the same open sprints. The chosen sprint goes onto the draft, which Task 1 pushes after the create. It is hidden for an epic, which does not belong to a sprint.

- [ ] **Step 4: Tests.** `BacklogView.test.tsx`: the panel offers the sprints and a choice journals through `MoveIssueToSprint`. `EpicsView.test.tsx`: the same on a child issue. `NewIssueModal.test.tsx`: the select lists the sprints, the draft carries the chosen one, and the field is absent for an epic. `SprintField`'s own cases: the empty-list sentence, and the board name appearing only on a duplicate name. Commit as `feat(tam): put an issue in a sprint from anywhere it appears`.

---

### Task 3: Docs, the single gate run, and the fix wave

- [ ] **Step 1: Docs.** `tam/CLAUDE.md` records that the sprint list away from a board is profile-wide, that a draft's sprint is pushed after its create rather than sent with it and why, and the importer's new column. `README.md` if it lists what import accepts.

- [ ] **Step 2: Every gate, once.**

```bash
cd core && go vet ./... && go test ./... -count=1 && cd ..
cd xtm && go vet ./... && go test ./internal/... -count=1 && cd ..
cd tam && go vet ./... && go test ./... -count=1 && cd ..
npm run typecheck --workspaces --if-present
npm test --workspaces --if-present
cd tam && wails build && cd ..
git status --short --untracked-files=no
```

Record each result, fix what fails, rerun only what failed. Do not lower an assertion to make a test pass. Vitest before this plan: 46 in `frontend/core`, 159 in `xtm`, 270 in `tam`.

- [ ] **Step 3: The walk-through for the user** (not run by agents): on the demo profile, open an issue from the Backlog and put it in a sprint, create a new issue into a sprint, import a sheet with a Sprint column, then Commit and read the result. On a real Data Center, confirm the created issue actually lands in the sprint, since that is the one step no fixture proves.

- [ ] **Step 4: Commit** as `docs(tam): a sprint can be set from anywhere`, then push and open the PR against `main`.

## Deferred

Setting a sprint on several issues at once from the Backlog (the board's multi-select already does it there), moving an issue between boards, and creating a sprint from the issue's own sprint field.
