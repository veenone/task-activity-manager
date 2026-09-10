<!-- /autoplan restore point: /c/Users/ARaha/.gstack/projects/veenone-task-activity-manager/feat-tam-sprints-view-autoplan-restore-20260910-090550.md -->
# Task Activity Manager: the Sprints view

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax. Lean cycle: implementers write the tests named here but run no suites, except the Go run at the end of Task 5 and the frontend run at the end of Task 8. Task 9 runs every gate once.

**Goal:** a view where a sprint can be created, edited, deleted, started, completed, filled from the board's unassigned work, and read with its contents and who is carrying them.

**Why this exists:** Phase 3 can draw a sprint and move cards through it but cannot make one, and the only way to reach a sprint at all is to pick the board that carries it first. The design is `docs/superpowers/specs/2026-09-10-tam-sprints-view-design.md`.

**Architecture:** a sprint's goal added at every layer it was missing from, three new Agile calls behind `backend.BoardBackend`, a store seam for the one write that has to touch two repositories, three new methods on `internal/sprints.Service`, one new cached read, and a view built from the Epics view's parts.

**Shipping shape:** this is **two pull requests**. Tasks 0 to 5 are a sprint write path, reachable and walkable from the Boards toolbar with no new view. Tasks 6 to 9 are the view. The split exists because the write half contains the only irreversible action in the application and should be exercised before five components are layered on top of it.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

## Global Constraints

- Nothing under `xtm/` changes, and nothing is added to `core/settings`, which is global and shared with XTM. One additive change lands in `frontend/core` (widening `ConfirmOptions.message` to `ReactNode`; `DialogContext` already renders it into a div and XTM only passes strings).
- **Creating, editing and deleting a sprint reach Jira immediately, and nothing else added here does.** Membership stays journaled. The enforced list is the `Service`'s own exported immediate writes, and Task 5 makes that a failing test.
- **The marker sits wherever the point of no return is:** four dialogs and one confirmation, never on a menu item that only opens one of them.
- Two spec sentences become false in this plan and are amended in Task 9, not left to rot: section 14.2 of the boards spec, and this design's own §7, which still says all three writes refresh through `refreshSprints`.
- A sprint's dates are written and parsed only through `internal/sprintdate`.
- Jira's sprint update endpoint is a partial update. **On create an empty goal is omitted. On edit, a goal the user emptied is sent as `""`.** Different rules; the code says which is which.
- **An empty answer from `BoardSprints` is not a fact.** `core/jira` maps any 400 on the first page to `ErrNoSprints` and the backend turns that into an empty slice and no error, so an empty list is indistinguishable from a failure. Nothing added here may read an empty list as "this sprint is gone" or "this board has no sprints".
- Every Jira sentence shown to a user goes through `internal/errtext` first.
- Files stay small and single purpose. `internal/backend/backend.go` (449 lines) and `frontend/src/api.ts` (848) are already at the ceiling: add a line each, not a block.
- TAM mirrors XTM's design language. The sprint row gets its own `.sprint-row` / `.sprint-cell` classes because `.epic-row`'s tracks are fixed and genuinely different.
- **Amber means "held locally, waiting for Commit"** in this app. Nothing added here may use it to mean anything else.
- UI text uses no em dashes. No AI attribution anywhere. Conventional commit prefixes, no trailers. Never add, commit or delete untracked local tooling files; revert Wails churn under `tam/frontend/wailsjs/runtime/` and `tam/frontend/package.json.md5`.

## Decisions

1. **The unfinished rule moves before it is used twice.** `incompleteCards(view)` derives "unfinished" from a drawn `BoardView`; this view has none, so the rule moves to its own module and both callers use it.
2. **The create and start dialogs share a form, not a component.**
3. **The view is always present**, with an empty state for a project that cannot have sprints.
4. **Delete does not refuse an empty sprint list, and does not hand-roll the cache surgery either.** `refreshSprints` refuses an empty answer because a ceremony proves the board has a sprint, and delete falsifies that. But hand-rolling the cleanup would reimplement `ReplaceSprints` and `deleteOrphanSprintIssues`. So delete gets a variant that permits an empty list, and reuses everything else.
5. **A sprint is filled from the board's unassigned work**, which the view has to compute, because the cache does not store it.
6. **Membership is grouped by assignee, and a person is not a tree node.**
7. **Space checks, Enter selects**, a deliberate divergence from `EpicTree`.
8. **A sprint's goal is added to the model.** It existed at no layer.
9. **The fence is the service's own immediate writes, not the `lifecycle` interface.** `lifecycle` already carries `BoardSprints`, a read, and `MoveIssuesToSprint`, which is journaled from its other caller. A test claiming every method on it is an immediate write would be false the day it was written.

---

# Part one: the sprint write path

### Task 0: verify the wire before anything is built on it

**Files:** create `docs/superpowers/plans/assets/2026-09-10-sprint-wire-probe.md`.

- [ ] **Step 1: Write the probe.** Four `curl` invocations against the user's own instance, each beside the answer this plan assumes.
  1. `POST /rest/agile/1.0/sprint` with a name, origin board id, two dates and a goal. **Assumed:** 200 or 201 with a body carrying the new sprint's `id` **and its `goal` echoed back**.
  2. `DELETE /rest/agile/1.0/sprint/{id}` on that sprint, with one issue in it. **Assumed:** the issue returns to the backlog rather than being deleted.
  3. Either call as an account without Manage Sprints. **Assumed:** 403 with a message.
  4. `POST /rest/agile/1.0/sprint/{id}` against a **closed** sprint. **Assumed:** refused. This one decides Task 5's edit guard: if Jira accepts it, TAM's guard is the only thing standing between a stale cache and a rewritten velocity chart, and it has to be strict.
  The document says what to do when an answer differs, and that the PAT goes in a header and never into a committed file.

- [ ] **Step 2: Say what each answer changes.** 1 shapes Task 1's signature and Task 2's schema. **2 is a promise made to a user about their data**, in the delete confirmation's own words; if it is wrong, that copy changes and whether delete ships is reopened. 3 shapes an error message. 4 shapes a guard. Commit as `docs(tam): a probe for the sprint write calls`.

---

### Task 1: a sprint has a goal, and three new Agile calls

**Files:** modify `core/jira/agile.go` (`RawSprint` only), create `core/jira/sprintwrite.go` and `sprintwrite_test.go`.

- [ ] **Step 1: The goal on the wire.** `RawSprint` gains `Goal string \`json:"goal"\``. Jira has sent it on every sprint read since Phase 3a and TAM has dropped it. Test: a payload carrying a goal decodes it.

- [ ] **Step 2: Create.** `POST /rest/agile/1.0/sprint` with `originBoardId`, `name`, `startDate`, `endDate`, and `goal` only when non empty. Decode into `RawSprint`. An empty body, or a zero `id`, is not an error: return the zero value and let the caller refresh. Follow `StartSprint`'s shape, error wrapping and comment style. Test: the body carries the board id and omits an empty goal; id and goal come back; an empty body is no error.

- [ ] **Step 3: Update.** `POST /rest/agile/1.0/sprint/{id}` with only the keys given. Same partial update trap, same comment. `clearGoal` distinguishes "untouched" from "emptied"; when true the body carries `goal: ""`. Do not send `state`. Test: an edit changing only the name sends only the name; `clearGoal` false with an empty goal omits it; true sends the empty string.

- [ ] **Step 4: Delete.** `DELETE /rest/agile/1.0/sprint/{id}`. Check what `WriteJSON` and its siblings offer for a body-less method first. Test: the path carries the id; a 404 names the sprint.

- [ ] **Step 5: The comment that keeps the exception honest.** Each of the three carries the sentence `StartSprint` already carries. Commit as `feat(core): a sprint's goal, and create, edit and delete`.

---

### Task 2: the goal reaches the cache, and the cache knows it is stale

**Files:** modify `tam/internal/tamstore/tamstore.go`, `tam/internal/backend/backend.go` (`Sprint` only), `tam/internal/backend/jira/boards.go`, `tam/internal/backend/demo/boards.go`, `tam/internal/boardrepo/boardrepo.go` (`Sprint` only), `tam/internal/boardrepo/boards.go` (`insertSprintSQL`, `listSprintsSQL`, the `Scan`), and their tests.

- [ ] **Step 1: The column.** `sprintDDL` gains `goal TEXT NOT NULL DEFAULT ''`. Schema version 7, migrated with `store.AddColumnIfMissing` following version 5's shape, not version 6's drop and recreate: dropping would empty every board's sprint picker until the next refresh.

- [ ] **Step 2: The type, at both layers, through one helper.** `backend.Sprint` and `boardrepo.Sprint` each gain `Goal string \`json:"goal"\``. Both struct literals in the tree are keyed, so nothing breaks. **The write seam is `writeSprints`, the single helper `ReplaceBoard` and `ReplaceSprints` both call**, not two files: the sync writes through the first and every ceremony through the second, so a column added to only one path would land on a full sync and vanish after a ceremony. The read seam is `listSprintsSQL` and its `rows.Scan`.

- [ ] **Step 3: The column does not back-fill itself, so say so.** Version 5's migration cleared `sync_state.last_synced` precisely because a column nothing refetches never gets filled. There is no equivalent trigger for boards: sprints refresh only when the user presses Refresh in the Boards view. Until then every cached sprint has an empty goal, and the Sprints view would show none for any sprint. The migration comment says this plainly, and Task 7 renders an absent goal as "no goal recorded yet; refresh the board" rather than as no goal.

- [ ] **Step 4: The demo fixture.** The three demo sprints gain goals. An empty string proves nothing in a walk through.

- [ ] **Step 5: Tests.** A goal survives a sync round trip and a ceremony round trip, which are two different write paths. **A database at version 5 migrated to 7 keeps its rows and gains the column**: migration 6 drops and recreates the table with the new DDL, then migration 7's add is a duplicate no-op, and nothing currently tests that interaction. A fresh database at 7 has it from the DDL, which is a test of the DDL and not of the migration, and the test name says so. Commit as `feat(tam): a sprint's goal reaches the cache`.

---

### Task 3: the store seam for a delete that spans two repositories

**Files:** create `tam/internal/boardrepo/deletesprint.go` and its test; modify `tam/internal/sprints/cache.go`; create `tam/internal/issuerepo/clearsprint.go` and its test.

This task exists because Task 5's delete has to remove a sprint from four places across two repositories, and the seam for that does not exist. Deciding it inside the delete step would mean an implementer discovering, at the largest step of the largest task, that `dbtx.In` opens its own transaction from the handle, so nesting `boardrepo` inside `issuerepo` takes a second pooled connection, blocks on the first's write lock, and dies on the driver's busy timeout. That is the failure `MoveManyToSprint` already documents.

- [ ] **Step 1: The board side, reusing what exists.** `refreshSprints` in `internal/sprints/cache.go` gains a variant that permits an empty answer, used only by delete. Everything else about it stays: it still calls `ReplaceSprints`, which still calls `deleteOrphanSprintIssues`, which already removes the `board_issue` rows of every scope the new list does not name. That is two of the four removals for free, and it keeps one rule in one place. The existing refusal keeps its comment and its callers.

- [ ] **Step 2: The cross board problem.** `sprint` is keyed `(profile_id, board_id, id)` because Jira hands the same sprint to every board whose filter reaches it, and `OpenSprints` joins across every board of the profile and folds by sprint id. A delete that cleans one board leaves the other board's row, and `OpenSprints` keeps offering the deleted sprint to the New issue dialog, the detail panel and the importer's Sprint column, which is the entire outcome this path exists to prevent. `DeleteSprintEverywhere(ctx, profileID string, sprintID int)` removes by `(profile_id, id)` with no board id, and its `board_issue` rows by `(profile_id, sprint_id)` with no board id. Test with **the sprint seeded under two boards**, copying `replace_test.go`'s existing two board fixture, and assert `OpenSprints` returns nothing afterwards. A one board fixture would pass while the bug ships.

- [ ] **Step 3: The issue side.** `ClearSprint(ctx, profileID, sprintID string) error` in `issuerepo` sets `sprint_id` and `sprint_name` to empty on every issue carrying that sprint. Without it `SprintField` keeps re-adding the deleted sprint as its own option from `sprint_name`, and `issuerepo.ListSprints` keeps feeding it to the Backlog and Epics filters, so a deleted sprint lingers in three views until a full issue sync. Test that it is scoped to the one sprint and does not touch an issue with a pending move to a different one.

- [ ] **Step 4: The ordering, and the crash window, written down.** These are two transactions in two repositories, not one, and no shared transaction helper exists. Adding one is a real design change and is not in this plan. So: **the board rows go first, the issue columns second**, and the comment says what a crash between them leaves, which is issues naming a sprint that is gone, recoverable by the next full sync and by nothing else. Choosing this silently at implementation time is how it ships broken. Commit as `feat(tam): the store seam a deleted sprint needs`.

---

### Task 4: the backend seam

**Files:** modify `tam/internal/backend/backend.go` (three interface lines), create `tam/internal/backend/jira/sprintwrite.go`, modify `tam/internal/backend/demo/boards.go` and its test, modify `tam/internal/sprints/sprints.go` (the `lifecycle` interface only).

**Produces**, on both `backend.BoardBackend` and the unexported `lifecycle`:
`CreateSprint(ctx, boardID int, d SprintDraft) (Sprint, error)`,
`EditSprint(ctx, sprintID int, d SprintDraft, clearGoal bool) error`,
`DeleteSprint(ctx, sprintID int) error`.

The middle one is `EditSprint` here while `core/jira`'s client method is `UpdateSprint`, and that is deliberate rather than a slip: the client is named for the wire operation, which is a partial update, and the two layers above it are named for the user's action, which is an edit. The chain reads `Service.Edit` to `BoardBackend.EditSprint` to `Client.UpdateSprint`, and the Jira implementation carries a one line comment saying where the name changes and why.

- [ ] **Step 1: The seam.** Three methods on `backend.BoardBackend` and on `lifecycle`, so a backend that cannot manage sprints is refused through `errNoLifecycle`. The Jira implementation delegates to Task 1 and maps `RawSprint` the way `BoardSprints` does, taking `BoardID` from the argument rather than the wire.

- [ ] **Step 2: The demo implementation, which is five edits and not one.** `demoSprints()` is a pure function over three literals, consumed by `findDemoSprint`, `sprintsOverlay`, `demoSprintName` and the state lookup. Create needs a new overlay entry that **all four** consult, or `MoveIssuesToSprint` refuses to move a card into a sprint the demo just made, because `demoSprintName` answers empty and the move errors. Delete needs a tombstone the same four honour, a walk returning the sprint's issues to the board's own scope, and a `BoardIssueKeys` filter that stops naming them. All under the existing mutex. This is a real implementation: the walk through runs on this backend, and Phase 3c's worst defect was a demo backend that ignored an argument. Test: create then list shows it as future with its goal, and a card can be moved into it; edit honours `clearGoal`; delete removes it and its issues are still on the board. Commit as `feat(tam): a backend that can manage sprints`.

---

### Task 5: the service, its guards, and the fence

**Files:** create `tam/internal/sprints/manage.go`, `manage_test.go`, `exceptions_test.go`; modify `tam/internal/sprints/sprints.go` (the package doc and `Store`), `guards.go`.

**Produces:** `Create(ctx, profileID string, boardID int, d backend.SprintDraft) (backend.Sprint, string, error)`, `Edit(ctx, profileID string, boardID, sprintID int, d backend.SprintDraft, clearGoal bool) (string, error)`, `Delete(ctx, profileID string, boardID, sprintID int) (string, error)`. The trailing `string` is the note a failed cache refresh leaves.

- [ ] **Step 1: The package doc changes.** It currently says the package owns "the two sprint ceremonies" and justifies the exception from properties specific to starting and closing. Neither argument covers create, edit or delete. Replace it with the design's reasoning: a sprint's id has to be real before anything can point at it, and TAM has the machinery to defer an issue's id and none to defer a sprint's. `Store` grows the two methods Task 3 produced.

- [ ] **Step 2: The fence, over the right set.** `exceptions_test.go` asserts the `Service`'s own exported immediate-write methods by name: `Start`, `Complete`, `Create`, `Edit`, `Delete`. **Not the `lifecycle` interface**, which already carries `BoardSprints`, a read, and `MoveIssuesToSprint`, which its other caller journals; a test claiming every method there is an immediate write would be false the day it was written, and a test whose message is wrong teaches the next reader to distrust it. The failure message says what the list is for and that growing it means amending the spec.

- [ ] **Step 3: `Create`.** Convert dates through `sprintdate` as `guards.dates` does, call the backend, refresh through the existing `refreshSprints`, whose empty-answer refusal is correct here because a create proves the board has a sprint. Write an audit row. Test: it reaches the cache with its goal; **a refresh that answers empty returns the sprint and a note, and the note says what to do**, which nothing currently covers.

- [ ] **Step 4: `Edit`.** Read the state and refuse a closed sprint. **Whether that read comes from the cache or from Jira is decided by Task 0's fourth probe:** if Jira refuses a closed edit itself, the cached read is a courtesy and staleness costs a round trip; if Jira accepts it, this guard is the only thing between a stale cache and a rewritten velocity chart and must re-read. Write down which, and why. Then convert, call, refresh, audit.
  **The running-sprint confirmation is not here.** A flag in the result arrives after the write. The dialog confirms from the cached state before calling, in Task 8.

- [ ] **Step 5: `Delete`, and the three answers its guard must tell apart.**
  - **The state, read from Jira.** A sprint started on the web an hour ago still reads `future` locally, and every other guard in this package refuses too much when stale, which costs a Refresh; this one would permit too much, which costs a sprint.
  - **Three answers, not two.** A list holding the sprint as `future` proceeds. A list holding it in any other state refuses, naming the state. A **non-empty** list not holding it means already gone, which is a success with its own sentence. An **empty** list is refused outright, because `core/jira` turns any 400 on the first page into `ErrNoSprints` and the backend turns that into an empty slice and no error, so an empty list is what one flaky 400 looks like from here. Without this branch, a network flap at 2am tells the user their sprint was already deleted and removes it locally while it is still running in Jira.
  - **The journal.** Refuse while journal rows target the sprint, naming Commit. Note in the code what this does not cover: the journaled board writes deliberately take no lock, so a move can be journaled between this check and the call. That is rare and recoverable, and the comment says the recovery is a Commit that reports the sprint is gone.
  - **The cache**, through Task 3's two calls, board rows first.
  - **The audit row**, with an honest comment: see Step 6.
  - Test: cache says future and Jira says active, refused naming active; **`BoardSprints` answers empty, refused, cache untouched**; `BoardSprints` errors, refused, cache untouched; non-empty list without the sprint, reported already gone; a pending row, refused naming Commit; a clean delete leaves the cache **on both of two boards** and leaves no issue carrying its name.

- [ ] **Step 6: The audit row is written, and the claim about it is not made.** `journal.Entries` filters by entity key and `issuerepo.ListActivity` takes an issue key; there is no profile level activity view and this plan does not add one. So the row is written, because it is the only machine readable trace once Jira no longer has the sprint, and **the delete confirmation must not say "it is in the activity log"**, because no screen shows it. A profile level activity view is recorded in Deferred.

- [ ] **Step 7: The one Go suite run.** Inside `core/` then `tam/`: `go build ./... && go vet ./... && go test ./... -count=1`. Fix what fails, rerun at most twice. Commit as `feat(tam): a sprint can be made, changed and removed`.

---

### Task 6: the bindings, and part one's walk-through

**Files:** create `tam/app_sprintmanage.go` and its test; modify `tam/frontend/src/components/BoardsToolbar.tsx` and its test; regenerate `tam/frontend/wailsjs/**`.

- [ ] **Step 1: The three write bindings.** Each through `requireProfile`, taking `a.acquire(p.ID, "sprint")`, reducing Jira's sentence through `ceremonyError`. New file, not a bigger one. Wails discards a bound method's return value when it also returns a non-nil error, so anything a dialog needs on partial failure travels in the result. Test the guards **including `acquire`**, copying the pattern `app_boards_test.go` already uses; testing only `requireProfile` leaves the lock untested.

- [ ] **Step 2: A create button on the Boards toolbar,** so part one is walkable on its own and the irreversible path gets exercised before a view is built on it. It opens the same dialog Task 8 will reuse, so this is the dialog's first home rather than throwaway work.

- [ ] **Step 3: Regenerate the bindings, run every gate once, and open the first PR.** The gate list is Task 9's. Commit as `feat(tam): sprints can be managed from the board`.

---

# Part two: the view

### Task 7: the read

**Files:** create `tam/internal/boardrepo/sprintlist.go` and its test; modify `tam/app_sprintmanage.go`; regenerate bindings.

**Produces:** `boardrepo.SprintDetail{Sprint; Issues []backend.Issue; Total, Done int; Points, DonePoints float64; MembershipCached, Truncated bool}` and `BoardSprintDetails(ctx, issues IssueSource, profileID string, boardID int) ([]SprintDetail, error)`; the binding `ListBoardSprintDetails`, which passes `a.repo` as the `IssueSource` the way `GetBoard` already does.

- [ ] **Step 1: Issues, not keys.** A row shows a summary and a status, the grouping needs an assignee, and the detail panel needs a whole `Issue`. Returning keys would force a second query and lose the single snapshot this read exists for.

- [ ] **Step 2: The journal, replayed, exactly as the board does it.** `composeBoard` calls `issues.PendingMoves`, then `withMovedIn` to pull in cards journaled into the scope, then `applyMoves` to drop cards journaled out. It does this because a journaled sprint move writes `issue.sprint_id` but **never** touches `board_issue`, which is Jira's list from the last sync. Omit it and the fill bar's headline gesture appears to do nothing until a full boards sync, and these four numbers disagree with the ones the Boards view shows for the same sprint. Same helpers, same transaction.

- [ ] **Step 3: The board's unassigned work, which the cache does not store.** Scope `""` in `board_issue` is **not** the backlog: it is `BoardIssueKeys(boardID, "", projectKey)`, the board's entire issue list, sprint issues included, and TAM's own code calls it "the board's own list". Rendering it as a Backlog node would list every sprint's issues a second time and would offer the fill bar work that is already in a sprint. So the unassigned scope is computed: the board's own list, minus every key that the journal-replayed sprint scopes hold. Name it for what it is.

- [ ] **Step 4: The rest of the read.** Order active, then future by start date, then closed by start date **descending**, which `listSprintsSQL` does not do today; the unassigned node last. `MembershipCached` is derived from the sprint's state, and its doc comment says so, because `board_issue` is equally empty for a closed sprint and for a future sprint whose sync failed. Cap at `boardrepo.MaxCardsPerView`, the board's own budget, rather than the epic tree's larger one: this payload crosses Wails and Task 8 invalidates it on six different mutations. Set `Truncated`. Run on the deferred read transaction.

- [ ] **Step 5: Tests.** The order, including closed descending. The four numbers. A closed sprint reporting membership uncached. **A card journaled into a sprint appears there and not in its source**, which is the test that would catch Step 2 being skipped. **The unassigned node seeded the way `readBoard` seeds it**, with a key present in both the board's own list and a sprint scope, asserting no duplication; a fixture seeded with unassigned-only keys would pass while the bug ships. The cap reporting truncation. Commit as `feat(tam): one read for a board's sprints and their work`.

---

### Task 8: the view, its tree, and its writes

**Files:** create `SprintsView.tsx`, `SprintList.tsx`, `SprintRow.tsx`, `NewSprintModal.tsx`, `EditSprintModal.tsx`, `SprintDraftForm.tsx`, `SprintFillBar.tsx`, `ImmediateWriteChip.tsx`, `lib/sprintGroups.ts`, `lib/unfinished.ts`, `queries/sprints.ts`, and a test file for each; modify `frontend/core/src/components/useConfirm.tsx`, `queries/keys.ts`, `lib/format.ts`, `nav.ts`, `App.tsx`, `main.go`, `BoardCeremonies.tsx`, `StartSprintModal.tsx`, `CompleteSprintModal.tsx`, `EpicRow.tsx`, `App.css`, **and the test files of every shipped component this touches**, which are not otherwise in this list and are what Task 9's gate run would otherwise fail on.

- [ ] **Step 1: The shared pieces, first.** `lib/unfinished.ts` takes issues and columns; `BoardCeremonies`'s `incompleteCards` calls it and its copy is deleted. `lib/sprintGroups.ts` partitions by assignee, ordered points descending, then count, then display name, unassigned last, because Map order is cache order and would reshuffle the tree between syncs. `sprintDates` and `day` move from `CompleteSprintModal` into `lib/format.ts`. `StartSprintModal`'s existing tests must still pass untouched.

- [ ] **Step 2: The view exists.** `View` gains `"sprints"`; `VIEWS` gains the entry **with its required `phase` and `blurb`**; `menuViews` gains it at accelerator `4`, renumbering Reports to `5` and Rituals to `6`, which will break whatever asserts the menu today. `App.tsx` renders it.

- [ ] **Step 3: The queries.** `useBoardSprintDetails` and the three mutations, each invalidating the board's sprints, the board, the profile wide open sprints, and the suggestion. The ceremonies keep `runSprintCeremony`; create, edit and delete do not, because a rename is not a ceremony. **The honest reason, which is not the banner:** the refusal comes from `acquire`, so New sprint will still refuse during a boards refresh whether or not a banner shows. Suppressing the banner removes the explanation, so the dialog must surface that refusal in words.

- [ ] **Step 4: The row.** Five cells and a caret: name, state chip ("Active", not shouting), date range from `lib/format.ts`, progress, menu trigger. **The goal is not on the row**; it is a sentence, the cells clip, and opening the panel narrows the pane. Suppress progress entirely when `Total` is zero. Actions in a menu following `CardMoveMenu`, reusing its `triggerTabIndex={-1}` and class handle; items read "Edit sprint…", "Delete sprint…". Align `EpicRow.progressText` so two trees do not use two sentences in one column.

- [ ] **Step 5: The tree.** `SprintList` mirrors `EpicTree`. An expanded sprint renders its goal (or "no goal recorded yet; refresh the board" when the column is still empty from Task 2 Step 3), then its issues under non-interactive `role="presentation"` assignee separators. Issues stay the only tree items, which keeps the tree two levels. **Only the active sprint starts expanded.** The unassigned node renders last, labelled for what it is, with no chip, dates or actions.

- [ ] **Step 6: The shell.** Board picker (hidden at one scrum board, named in the heading instead), show-closed toggle, selection, detail panel, render-phase profile reset. A summary line above the tree: "Sprint 12, day 6 of 14, 8 of 14 done, 21 of 34 pts". **All four branches, error, loading, empty, data, for the picker and the tree alike.** Write the three empty-state sentences verbatim.

- [ ] **Step 7: The dialogs and the marker.** `SprintDraftForm` lifted from `StartSprintModal`, **seeding the goal from the sprint** now that Task 2 put it in the cache, which fixes the same blindness in the Start dialog. A duplicate name nudges without blocking. Both dialogs disable on submit. Edit sends `clearGoal` and confirms before moving a running sprint's end date. `ImmediateWriteChip` is `chip chip-now` reading "Sends to Jira now", painted accent, **never amber**, in four dialogs' `.pending-head` and the confirmation, never on a menu item; `CompleteSprintModal` keeps its irreversibility banner, which is a different claim. `ConfirmOptions.message` widens to `ReactNode`.

- [ ] **Step 8: Delete's confirmation.** `danger: true`, `confirmLabel: "Delete sprint"`, `cancelLabel: "Keep it"`. It says the sprint by name and its issue count, **where the issues go** ("Jira moves its 6 issues back to the backlog. The issues themselves are not deleted"), and that it cannot be undone from TAM or Jira. **The count is a floor whenever `Truncated` is set or whenever `board_issue` holds keys the issue cache does not**, which board filters routinely produce; say "at least N" then, because a wrong number in the one irreversible confirmation is worse than none. No type-the-name gate. A 403 arrives after the confirmation has closed and goes to `useNotice`.

- [ ] **Step 9: Success, and the states nobody specified.** A created sprint scrolls into view, flashes once reusing `EpicTree`'s moved-row effect, and is announced. The `note` from a partial success renders in the `role="status"` banner the ceremonies already use. A row's menu is disabled while its own mutation is pending. A truncated sprint says so.

- [ ] **Step 10: Filling a sprint.** `SprintFillBar` over a selection of issue rows, journaled through `MoveManyToSprint`. Reuse `lib/boardSelection.ts` with four things it does not handle: `order` holds **only issue row keys**, never sprint or group ids, because `extend` slices it blindly; shift-extend is clamped to the anchor's own scope, which cannot happen on a board and is one drag away here; **Space checks, Enter selects**, where `EpicTree` binds both to select; and the checked paint is **a real checkbox** on issue rows, because `.folder-selected` is already accent-soft and amber is spoken for. **Reserve the tree pane's width while a selection exists**, since `.epics-body:has(.detail-panel)` narrows it and hiding the panel mid-gesture would jump every row sideways.

- [ ] **Step 11: Start and complete from here**, opening the existing modals, with the unfinished list from `lib/unfinished.ts`.

- [ ] **Step 12: Tests, then the one frontend run.** Every behaviour above, plus: the fill bar never selects a header; a second click while saving does nothing; the marker's actual sentence, not just its presence. Then `npx vitest run` and `npx tsc --noEmit` in `tam/frontend`. Commit as `feat(tam): the Sprints view`.

---

### Task 9: docs, the amendments, the single gate run

- [ ] **Step 1: Three amendments.** Boards spec section 14.2, which says the two ceremonies are the only immediate writes. **This design's own §7**, which still says all three writes refresh through `refreshSprints` while §4 and Decision 4 say the opposite. And the design's mockup, which someone will build from.

- [ ] **Step 2: Docs.** `tam/CLAUDE.md`: the view; that create, edit and delete are immediate and the honest reason; that the fence is the service's own methods and a test asserts them; that membership stays journaled; that a closed sprint has no cached membership by design; that delete spans two repositories in two transactions, board rows first, and what a crash between them leaves; that scope `""` is the board's own list and not the backlog, which is computed; schema version 7 and that the goal back-fills only on a boards refresh. Update the Status paragraph and the Layout tree.

- [ ] **Step 3: Every gate, once.**

```bash
cd core && go vet ./... && go test ./... -count=1 && cd ..
cd xtm && go vet ./... && go test ./internal/... -count=1 && cd ..
cd tam && go vet ./... && go test ./... -count=1 && cd ..
npm run typecheck --workspaces --if-present
npm test --workspaces --if-present
cd tam && wails build && cd ..
git status --short --untracked-files=no
```

Vitest before this plan: 46 in `frontend/core`, 159 in `xtm`, 293 in `tam`. Do not lower an assertion to make a test pass.

- [ ] **Step 4: The walk-through for the user** (not run by agents): create a sprint and watch it flash into the tree; edit its name and goal; clear the goal and confirm it stays cleared; start it; select three unassigned issues and move them in, and confirm they appear in the sprint **before** any sync; Commit; complete it; delete a future sprint and read the confirmation; delete a board's last sprint and confirm it does not come back and no issue still shows its name. On a kanban only profile, confirm the view explains itself. On a real Data Center, the four probe answers confirmed against the built app.

- [ ] **Step 5: Commit** as `docs(tam): the Sprints view`, then push and open the second PR.

## Deferred

Editing a closed sprint's dates. Deleting an active sprint. Creating a board. Carrying unfinished issues forward at create time. A team roster with capacity per person. Remembering a 403 for the session. Collapsible assignee groups. **A profile level activity view**, without which the audit rows this plan writes are machine readable only. **A shared transaction helper spanning `boardrepo` and `issuerepo`**, which would let the delete surgery be one transaction instead of two.
