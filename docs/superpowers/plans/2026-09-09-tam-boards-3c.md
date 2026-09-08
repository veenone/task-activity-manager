# Task Activity Manager Phase 3c: the sprint lifecycle

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan runs under the lean cycle: implementers build and write the tests named here but do not run suites, except one Go suite run at the end of Task 3. Task 5 runs every gate once, then one fix wave.

**Goal:** a sprint can be started and completed from TAM, several cards can be moved into a sprint at once, an issue can be put in a sprint from its detail panel, and a board read stops being able to observe a half-written board.

**Succeeds when** a sprint planning session and a sprint review both happen inside TAM: pick the cards, move them in one gesture, start the sprint; two weeks later complete it and answer Jira's question about what did not finish, without opening a browser.

**Architecture:** `core/jira/agile.go` gains three lifecycle calls. `BoardBackend` gains them too, with both backends implementing them. The sprint lifecycle is the one write in TAM that does not go through the journal, for reasons the spec argues and this plan enforces. `boardrepo` gains one read for the dialog's default dates and takes its board reads in a transaction. The Boards view gains two dialogs and a selection model; the detail panel gains a sprint select over the write 3b already built.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

**Spec:** [`../specs/2026-09-07-tam-boards-design.md`](../specs/2026-09-07-tam-boards-design.md), section 14.

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, `agile-suite/tam`; run Go commands from inside the module directory. Nothing under `xtm/` changes.
- **The sprint lifecycle is the one exception to the journal, and it is deliberate.** Starting and completing a sprint go to Jira immediately, because a sprint's start is a timestamped fact a whole team reads and Phase 4's burndown will be computed from it, and because a completion's answer depends on the sprint's contents at the moment it closes. Every other write in this plan, including the bulk sprint move, is journaled exactly as 3b built it. No second exception may be added without amending the spec.
- **There is no offline state, so no button claims one.** TAM has no connectivity signal: `TestConnection` is an explicit action and nothing polls. An earlier draft of this plan said the lifecycle buttons would be "disabled when the app has no connection", which cannot be computed and would have shipped an enabled button and a raw transport error for the case it named, a VPN that is off at 9:01. The buttons are always enabled, the action is attempted, and a transport failure is reported in the dialog, which stays open with a Retry. This is the same rule the spec already states for permissions: do not guess before trying.
- Completing a sprint moves the incomplete issues first and closes second, so a failed move leaves the sprint open rather than half closed.
- A 403 says the account cannot manage sprints on this board. The buttons stay: TAM does not guess at permissions before trying.
- Multi-select lives on the board only, and is cleared by any change of board, sprint, swimlane or profile.
- The bulk move journals one row per card, through `issuerepo.MoveToSprint`, so Discard, the pending dialog and the commit pass need no new cases.
- The PAT stays in the Jira client's Authorization header only.
- Files stay small and single purpose; a helper used from two places lives in its own module. TAM mirrors XTM's design language where XTM has a counterpart.
- UI text uses no em dashes. No AI attribution or mentions anywhere, in code, comments, commit messages, or documents. Conventional commit prefixes, no trailers. Never add, commit, or delete untracked local tooling files; revert Wails churn under `tam/frontend/wailsjs/runtime`, `tam/frontend/package.json.md5`, and `tam/go.mod` with `git checkout --`.

## Decisions

1. **Lifecycle first, moves second, when completing.** TAM moves the incomplete issues with the Agile call it already makes, then closes the sprint. The reverse order would leave a closed sprint whose issues went nowhere, which no one can undo from TAM.
1b. **Incomplete means the card is not in the board's last column.** One undefined word here decides which cards an irreversible action moves, so it is defined once: an issue is complete when its status id is in the last `board_column`'s `status_ids`, which is the same mapping the board itself draws with and the same one the summary line counts done points by. Not `statusCategory`, which a board can disagree with, and not the board's own guess. The rule is written on the dialog, not just in this plan.
2. **The end date comes from the board, then from a default.** Jira's board configuration carries a sprint length; take it when it is there. Otherwise two weeks, with the field focused. An earlier draft took the median of the last three closed sprints, which is cleverness with no owner: a team that changed cadence, or a quarter with a holiday sprint in it, produces a plausible wrong date that nobody checks.
3. **A sprint move is still journaled, even in bulk.** It is an issue write, it has a conflict story, and 3b built all of it. Only the lifecycle bypasses the journal.
4. **Selection is a set of keys, not of positions.** The board redraws on every refetch, and a position-based selection would silently select different cards after a sync, which is the same class of bug 3b hit with focus.
4b. **The two selections are different things and look different.** The board already has a selection: one card, painted `.board-card-selected`, which opens the detail panel. The multi-selection is a second model on the same surface, so it gets its own class, `.board-card-checked`, and the grid takes `aria-multiselectable`. XTM solved this exact problem years ago with `row-selected` for the cursor and `row-checked` for the checkbox, and TAM mirrors XTM by standing rule rather than inventing a third vocabulary. A multi-selection of more than one card closes the detail panel and puts the selection bar in its place, because a panel claiming to describe one card while three are marked is a lie about what the next action will touch; dropping back to one reopens it.
5. **The board read takes a transaction.** `Board` and `CellOrder` each issue several statements and assume a consistent snapshot; SQLite gives that only inside a transaction. This is the inherited flake, and it is a read-side fix: `ReplaceBoard` was already correct.

## File structure

**Created:** `tam/internal/boardrepo/sprintlength.go`, `sprintlength_test.go`; `tam/internal/sprints/sprints.go`, `sprints_test.go` (the lifecycle service, kept out of `app_boards.go` so the online-only rule has one home); `tam/frontend/src/components/StartSprintModal.tsx`, `CompleteSprintModal.tsx`, `tam/frontend/src/lib/boardSelection.ts`, `boardSelection.test.ts`.

**Modified:** `core/jira/agile.go`, `agile_test.go`; `tam/internal/backend/backend.go`; `tam/internal/backend/jira/boards.go`, `jira_test.go`; `tam/internal/backend/demo/boards.go`, `demo_test.go`; `tam/internal/boardrepo/view.go`, `cellorder.go`, `boardrepo.go`, `view_test.go`; `tam/app_boards.go`, `app_boards_test.go`; `tam/frontend/wailsjs/**` (regenerated); `tam/frontend/src/api.ts`, `queries/boards.ts`, `components/BoardsToolbar.tsx`, `BoardsView.tsx`, `BoardGrid.tsx`, `BoardCard.tsx`, `BoardsView.test.tsx`, `IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx`, `App.css`; `frontend/core/styles/primitives.css`; `tam/CLAUDE.md`, `README.md`.

---

### Task 1: The lifecycle on the wire and on the seam

**Files:** modify `core/jira/agile.go`, `agile_test.go`, `tam/internal/backend/backend.go`, `backend/jira/boards.go`, `jira_test.go`, `backend/demo/boards.go`, `demo_test.go`.

**Produces:** `(*Client).StartSprint(ctx, sprintID int, name, goal, start, end string) error`, `(*Client).CompleteSprint(ctx, sprintID int) error`; on `BoardBackend`: `StartSprint(ctx, sprintID int, s backend.SprintDraft) error`, `CompleteSprint(ctx, sprintID int) error`; `PushIssuesToSprint` is 3b's existing `MoveIssuesToSprint` on the same interface, referred to by that name throughout this plan so it is never confused with the journal binding; `backend.SprintDraft{Name, Goal, StartDate, EndDate string}`.

- [ ] **Step 1: The tests.** Extend the httptest server in `agile_test.go` with `POST /rest/agile/1.0/sprint/12`. Cover: a start sending `state: "active"` with the dates; a completion sending `state: "closed"` **and nothing else**, since a completion that also sends dates would rewrite them; a 400 whose Jira message survives into the error, which is how "another sprint is already active on this board" reaches the user; and a 403 likewise. Assert on the request bodies, not only the status: these are all side effect.

- [ ] **Step 2: The calls.** Add them to `agile.go` beside 3b's writes, over the existing helpers, with doc comments naming the endpoint and saying that these three are the only calls in TAM that reach Jira outside a Commit.

Name the date format in those doc comments and convert to it in one place. The Agile API wants `2026-09-09T09:00:00.000+0000` and rejects a bare `2026-09-09`, and an HTML date input produces exactly the bare form, so the conversion belongs in `sprints.Service` where both dates pass through, not in the dialog and not in the client.

- [ ] **Step 3: The seam and the backends.** `BoardBackend` gains the three methods. The Jira backend passes them through. The demo backend keeps its sprints in memory: starting one refuses with Jira's own words when another is already active on that board, completing one moves nothing itself (the caller does that) and flips the state, and the compile-time `var _ backend.BoardBackend` assertions both packages carry keep answering. Extend `demo_test.go` for the refusal and the state change.

- [ ] **Step 4: Commit** as `feat(core): start and complete a sprint`.

---

### Task 2: A board read is a snapshot

**Files:** modify `tam/internal/boardrepo/view.go`, `cellorder.go`, `boardrepo.go`, `view_test.go`.

**Produces:** no new exported API; `Board` and `CellOrder` each read inside one deferred transaction.

This is the inherited bug, not a feature. `TestReplaceBoardLandsColumnsAndMembershipTogether` has failed for two separate sessions on two branches and on clean `main`; it asserts that a concurrent read never sees a half-applied board, and it is right to. `ReplaceBoard` is already correct: it writes in one transaction. The gap is on the read side, where `Board` issues several statements (columns, membership, issues, drafts, pending moves) and `CellOrder` several more, each seeing whatever is committed at the moment it runs.

- [ ] **Step 1: Read in a transaction.** Give `Repository` a small `inReadTx` helper beside the existing `inTx`, opening a deferred transaction, running the reads, and rolling back (a read transaction has nothing to commit). `Board` and `CellOrder` take all of their statements through it, including the ones behind `IssueSource`, which means `IssuesByKeys`, `DraftIssues` and `PendingMoves` need a form that accepts the transaction. Follow whatever the package already does for a query that must run inside a caller's transaction rather than inventing a second pattern.

- [ ] **Step 2: Prove it.** The existing concurrency test is the proof: run it with `-count=20` and it must pass every time. Add one more that fails without the fix, driving a reader in a loop while a writer replaces a board twice, asserting the reader only ever sees one whole board or the other. Say in the test's comment that this is the flake two sessions chased.

- [ ] **Step 3: Commit** as `fix(tam): a board read sees one board, not the moment between two`.

---

### Task 3: The lifecycle service, the dates, and the bindings

**Files:** create `tam/internal/sprints/sprints.go`, `sprints_test.go`, `tam/internal/boardrepo/sprintlength.go`, `sprintlength_test.go`; modify `tam/app_boards.go`, `app_boards_test.go`; regenerate `tam/frontend/wailsjs/**`.

**Produces:** `sprints.Service` with `Start(ctx, profileID string, boardID, sprintID int, d backend.SprintDraft) error` and `Complete(ctx, profileID string, sprintID int, moveTo string) (sprints.Completion, error)`; `boardrepo.SprintLength(ctx, profileID string, boardID int) (int, error)`; bound methods `StartSprint(profileID string, boardID, sprintID int, name, goal, start, end string) error`, `CompleteSprint(profileID string, sprintID int, moveTo string) (sprints.Completion, error)`, `SuggestSprintDates(profileID string, boardID int) (sprints.Suggestion, error)`, `JournalSprintMoves(profileID string, keys []string, sprintID string) error`.

- [ ] **Step 1: `SprintLength`.** The median whole-day length of that board's last three closed sprints, from the `sprint` table's `start_date` and `end_date`, ignoring a sprint missing either. No history returns zero, and the caller decides what that means.

- [ ] **Step 2: The service.** `internal/sprints` owns the two lifecycle actions so the online-only rule has one home and one place to test. `Start` calls the backend and returns Jira's error unchanged. `Complete` re-reads the sprint's issues **from Jira**, not from the cache, which can be minutes stale and would otherwise decide the fate of cards on data the user cannot see. It pushes the incomplete ones to the backlog or the named sprint with the backend's `PushIssuesToSprint` **first**, in chunks of twenty for the same reason the bulk journal move chunks, closes the sprint **second**, and returns `Completion{Moved int, MovedTo string, Failed []string}`. A completion of forty issues that loses one chunk must be able to say "twelve of forty moved, the sprint is still open", which a count alone cannot express.

Two names that must not be confused, because an earlier draft of this plan used one word for both and would have shipped a completion that journals rows and then closes the sprint, moving nothing and leaving a journal full of rows pointing at a closed sprint that fail on every future Commit. `PushIssuesToSprint` is the backend call that reaches Jira now, and it is what a completion uses, bypassing the journal exactly as the lifecycle does. `JournalSprintMoves` is the bulk binding the board's selection uses, and it only ever writes journal rows. A failed move returns before the close, with the sprint still open and an error saying so. Neither method touches the journal.

- [ ] **Step 3: The bindings.** `StartSprint` and `CompleteSprint` run under `a.acquire(p.ID, "sprint")`, which is a real guard here because they push to Jira, unlike the journal writes 3b deliberately left unguarded. Both end by refreshing **that board's sprints only**, one `Sprints` call and an upsert, not a boards sync: `tam/CLAUDE.md` records that the boards pass takes minutes on a real project and acquires under its own name, so ending a lifecycle action with one would block the UI for minutes or be refused outright by the lock the action itself is holding. The same file's rule applies here too, that anything calling a bound method which takes `acquire` goes through the sync reducer, so the two lifecycle actions run through `SyncContext` the way `runBoardsRefresh` already does. `MoveIssuesToSprint` is the bulk journal write: it loops `issuerepo.MoveToSprint` inside one transaction and is not guarded, matching every other journal write. `SuggestSprintDates` returns today, today plus `SprintLength` days (two weeks when there is no history), and the name the board's last sprint suggests.

- [ ] **Step 4: Tests, then the one Go suite run.** `sprints_test.go` with a fake backend: a start that passes the draft through; a start Jira refuses, whose message survives; a completion that moves three issues to the backlog then closes; a completion whose move fails, leaving the sprint open and reporting it; a completion with no incomplete issues that closes without a move. `app_boards_test.go`: the guard refuses a second lifecycle call while one runs, and a bulk move journals one row per key. Then, inside `tam/`: `go build ./... && go vet ./... && go test ./... -count=1`, and inside `core/`: `go test ./jira/ -count=1`. Fix what fails, rerun at most twice, report. Commit as `feat(tam): the sprint lifecycle, its dates, and a bulk move`.

---

### Task 4: The dialogs, the selection, and the panel

**Files:** create `tam/frontend/src/components/StartSprintModal.tsx`, `CompleteSprintModal.tsx`, `tam/frontend/src/lib/boardSelection.ts`, `boardSelection.test.ts`; modify `api.ts`, `queries/boards.ts`, `components/BoardsToolbar.tsx`, `BoardsView.tsx`, `BoardGrid.tsx`, `BoardCard.tsx`, `BoardsView.test.tsx`, `IssueDetailPanel.tsx`, `IssueDetailPanel.test.tsx`, `App.css`, `frontend/core/styles/primitives.css`.

- [ ] **Step 1: The selection.** `boardSelection.ts` is pure: a set of keys, and the three gestures over an ordered key list (click replaces, shift-click extends from the anchor, control-click toggles). It has no React and no DOM, which is the part worth testing directly. Selection is by key, never by position, because the board redraws on every refetch and a position-based selection would quietly select different cards after a sync.

- [ ] **Step 1a: The bulk move wears the same marks as a single one.** 3b gave a moved card its states: checking, warned, pending, failed. A bulk move through a separate binding would leave fifty cards unmarked until a refetch, so the bulk result goes through the same `useBoardMoves` path a single move does and the cards mark themselves the same way.

- [ ] **Step 1b: The bulk move is chunked.** 3b recorded a known limit: a 207 from Jira's bulk endpoints fails the whole batch, because Jira reports its rejection by numeric issue id and the message cannot map it back to a key. This plan is what creates fifty-card batches, so it must not walk into that: the bulk move sends at most twenty keys per call and reports per chunk, so one rejected card costs nineteen retries rather than forty-nine. The limit itself stays recorded, and splitting the 207 body properly belongs with whoever next has a real instance to read one from.

- [ ] **Step 2: The board honours it.** A checked card takes `.board-card-checked` and the grid takes `aria-multiselectable`; `.board-card-selected` stays what it is, the one card the detail panel is about. The count, the Move to sprint action and Clear live in their own `BoardSelectionBar.tsx` rather than as six more props on `BoardsToolbar`, which is already at the size this repo splits at; the two ceremony buttons sit in `board-head-actions` beside Refresh. Mirror XTM's `.bulk-count` for the count. Selection clears on any change of board, sprint, swimlane or profile, through the same render-time reset the view already uses for its filters. Keyboard, and this needs care because 3b already owns these keys: in `BoardsView.onKeyDown` the Enter and Space branch returns before the Control branch, so Control with Space never arrives, and Control with an arrow is already a move. Shift with an arrow currently falls through to plain focus movement. So: Shift with an arrow extends the selection, Space alone toggles the checked state of the focused card (Enter keeps opening the panel), and the selection clears on a plain click or on the bar's Clear, not on Escape, which `Modal` and the panel already own.

- [ ] **Step 3: The start dialog.** `StartSprintModal` opens from the toolbar for a future sprint, in the same dialog shape as the completion: name prefilled, goal, start date defaulting to today and end date to today plus the board's sprint length, both editable, with the end field focused when there was nothing to suggest from. It names the sprint it is starting in its heading, and when another sprint is already active on that board it says so in a `.muted` line, which is reading the picker's own data rather than guessing at an answer Jira owns.

Dates are validated before submit, in the dialog, the way `NewIssueModal` reports a bad field: an end before a start, or an empty either, does not reach Jira. Submit disables the primary button while it runs, closes on success and selects the sprint it just started, and on failure keeps the dialog open with Jira's message and the fields as the user left them.

There is no date input anywhere in this repo yet, so this is new vocabulary: one `.date-field` rule beside the other form rules in `primitives.css`, used by both dialogs, rather than two dialogs inventing their own.

- [ ] **Step 4: The complete dialog.** `CompleteSprintModal` opens for the active sprint, in the shape every other TAM dialog uses: `modal pending-modal`, a `.pending-head` whose heading names the sprint ("Complete Sprint 14") with its dates in a `.muted` span beside it, a `.bulk-body`, and `.pending-actions` at the foot.

It **lists the incomplete issues**, it does not just count them. This is the one action in TAM that cannot be undone from TAM, and "12 issues will move to the backlog" is a claim the user has no way to check; Jira's own dialog names them, and so does this one, as `.pending-card` rows carrying key and summary. Under the list, where they go: the backlog, or any future sprint of that board, hidden entirely when nothing is incomplete. The sentence says plainly that this cannot be reversed from TAM.

Submit disables the primary button while it runs. On success the dialog closes, the picker moves to the destination sprint rather than falling back to whatever is first, and a banner says what moved where. On failure the sprint is still open and the dialog says so, with the failed keys named.

It also has to answer for the user's own uncommitted work, which nothing else in this app has had to do: a card the user dragged to Done an hour ago is Done on their screen and not in Jira, and closing the sprint would move it to the backlog as incomplete. When any pending journal row targets an issue in this sprint, the user is stopped **before the dialog opens**, not after they have chosen a destination: the toolbar's Complete button asks first, through a `useConfirm` naming Commit as the thing to do. That needs a binding, because pending rows carry issue keys and know nothing about sprints, so Task 3 also produces `PendingInSprint(profileID string, sprintID int) (int, error)`.

- [ ] **Step 5: The detail panel.** The Details tab gains Sprint as an editable field: a select of the board's open sprints plus the backlog, going through the same `MoveIssueToSprint` binding the card menu uses, so the pending row, the discard and the commit path are the ones 3b already reviewed. It is the last thing Decision 6 of the 3b design promised and did not ship.

- [ ] **Step 6: Tests.** `boardSelection.test.ts` for the three gestures including a shift-click that crosses the anchor. `BoardsView.test.tsx`: three cards selected shows the count, the bulk move calls the binding once with three keys, a sprint change clears the selection, the start dialog's dates default from the suggestion, the complete dialog names the incomplete count and its destinations, a failed start keeps the dialog open with Jira's message, and a failed start reports Jira's message without closing the dialog. `IssueDetailPanel.test.tsx`: the sprint select journals through the same binding.

- [ ] **Step 7: Commit** as `feat(tam): start a sprint, complete it, and plan into it`.

---

### Task 5: Docs, the single gate run, and the fix wave

- [ ] **Step 1: Docs.** `tam/CLAUDE.md` gains a "Phase 3c: the sprint lifecycle" section saying what the three lifecycle calls are, **why they are the one write that bypasses the journal**, that a completion moves before it closes, that selection is by key, and that a board read now runs in a transaction and why. Update Status and Layout. `README.md` gains one sentence. Reconcile spec section 14 with what shipped, correcting whichever side is wrong and saying which in the report.

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

Record each result, fix what fails, rerun only what failed. Do not lower an assertion to make a test pass. The Vitest counts before this plan were 46 in `frontend/core`, 159 in `xtm`, and 232 in `tam`. Run `go test ./internal/boardrepo/ -count=20` as well: Task 2 exists to make that pass every time.

- [ ] **Step 3: The walk-through for the user** (not run by agents): on the demo profile, select the future sprint, start it, select three cards and move them in, then complete it and read where the incomplete ones went. Then on a real Data Center, the two cases no fixture proves: starting a sprint on a board that already has an active one, and completing one as an account without the Manage Sprints permission.

- [ ] **Step 4: Commit** as `docs(tam): Phase 3c notes for the sprint lifecycle`, then push and open the PR against `main` titled "Task Activity Manager Phase 3c: the sprint lifecycle" with the tasks, the gates, and the walk-through. No AI attribution anywhere.

## Deferred

Creating a board, editing a sprint's dates once it has started, and a bulk transition, a bulk rank, a bulk assignee and bulk points. That last group is a cut rather than a completion: selecting several cards and being offered one action reads as unfinished, so the toolbar action is worded as exactly what it does, "Move N cards to sprint", and promises nothing else. A bulk transition needs every selected card's workflow checked, which is its own design, sprint reports and burndown (Phase 4, which is what these completed sprints feed), and the board's own quick filters and swimlane rules.
