<!-- /autoplan restore point: /c/Users/ARaha/.gstack/projects/veenone-task-activity-manager/feat-tam-sprints-view-autoplan-restore-20260910-090550.md -->
# Task Activity Manager: the Sprints view

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax. Lean cycle: implementers write the tests named here but run no suites, except the one Go run at the end of Task 3 and the one frontend run at the end of Task 6. Task 7 runs every gate once.

**Goal:** a view where a sprint can be created, edited, deleted, started, completed, filled from the backlog, and read with its contents and who is carrying them.

**Why this exists:** Phase 3 can draw a sprint and move cards through it but cannot make one, and the only way to reach a sprint at all is to pick the board that carries it first. The design is `docs/superpowers/specs/2026-09-10-tam-sprints-view-design.md`.

**Architecture:** a sprint's goal added at every layer it was missing from, three new Agile calls behind `backend.BoardBackend`, three new methods on `internal/sprints.Service` beside the two ceremonies it already owns, one new cached read in `boardrepo`, and a new view built from the Epics view's parts. The view is always present; a project that cannot have sprints gets an empty state that says why.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

## Global Constraints

- Nothing under `xtm/` changes, and nothing is added to `core/settings`, which is global and shared with XTM. One change lands in `frontend/core` (widening `ConfirmOptions.message`); it is additive and XTM is unaffected. Go commands run from inside the module directory.
- **Creating, editing and deleting a sprint reach Jira immediately, and nothing else added here does.** They live in `internal/sprints` beside `Start` and `Complete`, behind the same per profile lock under the same `"sprint"` label. Membership stays journaled through the paths that already exist. No sixth exception may be added without amending the spec, and Task 3 makes that rule a failing test rather than a paragraph.
- **The marker sits wherever the point of no return is.** Four dialogs and one confirmation, not five actions: a menu item that opens a dialog does not need it, because the dialog carries it.
- Section 14.2 of `docs/superpowers/specs/2026-09-07-tam-boards-design.md` says the two ceremonies are the only place in TAM where a button talks to Jira without Commit. That sentence becomes false in this plan and is amended in Task 7, along with the spec's own mockup, which currently contradicts this plan twice.
- A sprint's dates are written and parsed only through `internal/sprintdate`.
- Jira's sprint update endpoint is a partial update: a key that is present overwrites, one that is absent is left alone. **On create an empty goal is omitted. On edit, a goal the user has emptied is sent as `""`.** These are different rules and the code says which is which.
- Every Jira sentence shown to a user goes through `internal/errtext` first.
- Files stay small and single purpose; a helper used from two places lives in its own module. `internal/backend/backend.go` (449 lines) and `frontend/src/api.ts` (848) are already at the ceiling: add a line each, not a block.
- TAM mirrors XTM's design language: reuse `folder-tree`, `folder-item`, `folder-selected`, `folder-caret`, `folder-children`, `edit-row`, `detail-input`, `muted small`. **The sprint row gets its own `.sprint-row` / `.sprint-cell` classes**, because `.epic-row`'s grid tracks are fixed and genuinely different; that is not a violation of the no-new-class rule, it is the rule applied correctly.
- **Amber in this app means "held locally, waiting for Commit"**: the pending dot, the draft chip, the moved row flash. Nothing added here may use it to mean anything else.
- UI text uses no em dashes. No AI attribution anywhere. Conventional commit prefixes, no trailers. Never add, commit or delete untracked local tooling files; revert Wails churn under `tam/frontend/wailsjs/runtime/` and `tam/frontend/package.json.md5` with `git checkout --`.

## Decisions

1. **The unfinished rule moves before it is used twice.** `incompleteCards(view)` in `BoardCeremonies.tsx` derives "unfinished" from a drawn `BoardView`. The Sprints view has no `BoardView`, so the rule moves to its own module taking issues and columns, and both callers use it.
2. **The create dialog and the start dialog share a form, not a component.** The four fields plus their validation lift out of `StartSprintModal`, and each dialog keeps its own title, its own button, and its own call.
3. **The view is always present.** A profile with no scrum board gets an empty state naming the reason. Hiding it would have cost a cross cutting navigation mechanism for one consumer and shown a new user nothing at all on their first launch.
4. **Delete does not go through `refreshSprints`.** That helper refuses to persist an empty answer because a ceremony proves the board has a sprint, and delete is the write that falsifies it.
5. **A sprint is filled from this view, not only inspected**, and that requires the board's backlog to be in the tree. Without it the fill bar can only move issues that are already in some other sprint, and filling a new sprint would still mean leaving.
6. **Membership is grouped by the person carrying it, but a person is not a tree node.** The group is a presentation separator inside the sprint's children; the issues stay the only `treeitem`s. The tree is two levels, which is what the spec claims.
7. **Space checks, Enter selects.** `EpicTree` binds both to select; the board binds Space to check. They cannot both be right in a tree that has a multi selection, and this is a deliberate divergence from `EpicTree` rather than an oversight.
8. **A sprint's goal is added to the model.** It did not exist in `RawSprint`, `backend.Sprint`, `boardrepo.Sprint` or the `sprint` table. Editing a goal is impossible to do honestly without it, because the dialog cannot know what it is overwriting and `clearGoal` cannot be computed.

---

### Task 0: verify the wire before anything is built on it

**Files:** create `docs/superpowers/plans/assets/2026-09-10-sprint-wire-probe.md`.

Three assumptions shape tasks 1 through 4. Verifying them after the code is written is the wrong order.

- [ ] **Step 1: Write the probe.** A short document the user runs by hand against their own instance: three `curl` invocations with a placeholder base URL and PAT, and beside each the answer this plan assumes.
  1. `POST /rest/agile/1.0/sprint` with a name, an origin board id, two dates and a goal. **Assumed:** 200 or 201 with a JSON body carrying the new sprint's `id`, **and a `goal` field echoed back**, which is what Task 1 needs in order to read a goal at all.
  2. `DELETE /rest/agile/1.0/sprint/{id}` on that sprint, after putting one issue in it. **Assumed:** the issue returns to the backlog rather than being deleted.
  3. Either call as an account without Manage Sprints. **Assumed:** 403 with a message, not a 200 that silently does nothing.
  The document says what to do if an answer differs, and that the PAT goes in a header and must never be pasted into a file that gets committed.

- [ ] **Step 2: Say what each answer changes.** Assumption 1 shapes Task 1's signature and Task 2's schema. **Assumption 2 is different in kind: it is a promise made to a user about their data**, in the words of the delete confirmation. If a delete turns out to destroy the issues rather than return them, that copy changes and whether delete ships at all is reopened. Write that down in the probe. Assumption 3 shapes only an error message. Commit as `docs(tam): a probe for the three sprint calls`.

---

### Task 1: a sprint has a goal, and three new Agile calls

**Files:** modify `core/jira/agile.go` (`RawSprint` only), create `core/jira/sprintwrite.go` and `core/jira/sprintwrite_test.go`.

**Produces:** `RawSprint.Goal`; `CreateSprint(ctx, boardID int, name, goal, start, end string) (RawSprint, error)`, `UpdateSprint(ctx, sprintID int, name, goal, start, end string, clearGoal bool) error`, `DeleteSprint(ctx, sprintID int) error`.

- [ ] **Step 1: The goal on the wire.** `RawSprint` gains `Goal string \`json:"goal"\``. Jira has always sent it and TAM has always dropped it. Test: a sprint payload carrying a goal decodes it.

- [ ] **Step 2: Create.** `POST /rest/agile/1.0/sprint` with `originBoardId`, `name`, `startDate`, `endDate`, and `goal` only when non empty. Decode the answer into `RawSprint`. An empty body, or a body whose `id` is zero, is not an error: return the zero value and let the caller refresh, because the sprint exists in Jira either way. Read `StartSprint` in `agile.go` first and follow its shape, its error wrapping and its comment style. Test: the body carries the board id and omits an empty goal; the answer's id and goal come back; an empty answer body returns no error.

- [ ] **Step 3: Update.** `POST /rest/agile/1.0/sprint/{id}`, sending only the keys the caller gave. Same partial update `StartSprint` uses, same trap, and the comment says so. `clearGoal` is what distinguishes "the user did not touch the goal" from "the user emptied it"; when true the body carries `goal: ""` on purpose. Do not send `state`. Test: an edit changing only the name sends only the name; `clearGoal` false with an empty goal omits it; `clearGoal` true sends the empty string.

- [ ] **Step 4: Delete.** `DELETE /rest/agile/1.0/sprint/{id}`. Check what `WriteJSON` and its siblings offer for a body-less method before adding one. Test: the path carries the id; a 404 is an error naming the sprint.

- [ ] **Step 5: The comment that keeps the exception honest.** Each of the three carries the sentence `StartSprint` already carries. Commit as `feat(core): a sprint's goal, and create, edit and delete`.

---

### Task 2: the goal reaches the cache

**Files:** modify `tam/internal/tamstore/tamstore.go` (the DDL and a version 7 migration), `tam/internal/backend/backend.go` (`Sprint` only), `tam/internal/backend/jira/boards.go` (`BoardSprints` mapping), `tam/internal/backend/demo/boards.go` (the fixture sprints), `tam/internal/boardrepo/boardrepo.go` (`Sprint` only), `tam/internal/boardrepo/replace.go` and `boards.go` (the read and write of the new column), and their tests.

- [ ] **Step 1: The column.** `sprintDDL` gains `goal TEXT NOT NULL DEFAULT ''`. Schema version becomes 7 with a migration using `store.AddColumnIfMissing`, which treats a duplicate column as success, following version 5's shape rather than version 6's drop-and-recreate: this table is a cache but dropping it would empty every board's sprint picker until the next sync for a column that back-fills itself.

- [ ] **Step 2: The type, at both layers.** `backend.Sprint` and `boardrepo.Sprint` each gain `Goal string \`json:"goal"\``. `jira.BoardSprints` maps it across the way it already maps state and dates. `ReplaceSprints` writes it and `ListSprints` reads it.

- [ ] **Step 3: The demo fixture.** The three demo sprints gain goals, because the walk through reads one and an empty string proves nothing.

- [ ] **Step 4: Tests.** A goal survives a sync round trip into the cache and back out; a database created at version 6 gains the column and keeps its rows; a database created fresh at 7 has it from the DDL. Commit as `feat(tam): a sprint's goal reaches the cache`.

---

### Task 3: the backend seam, the service, and its guards

**Files:** modify `tam/internal/backend/backend.go` (three interface lines only), create `tam/internal/backend/jira/sprintwrite.go`, modify `tam/internal/backend/demo/boards.go` and its test, create `tam/internal/sprints/manage.go`, `manage_test.go` and `exceptions_test.go`, modify `tam/internal/sprints/sprints.go` (the `lifecycle` interface only), `guards.go` and `cache.go`.

**Produces:** `BoardBackend.CreateSprint(ctx, boardID int, d backend.SprintDraft) (backend.Sprint, error)`, `EditSprint(ctx, sprintID int, d backend.SprintDraft, clearGoal bool) error`, `DeleteSprint(ctx, sprintID int) error`; on `*sprints.Service`: `Create(ctx, profileID string, boardID int, d backend.SprintDraft) (backend.Sprint, string, error)`, `Edit(ctx, profileID string, boardID, sprintID int, d backend.SprintDraft, clearGoal bool) (string, error)`, `Delete(ctx, profileID string, boardID, sprintID int) (string, error)`. The trailing `string` is the note a failed cache refresh leaves, as `Service.Start` already returns one.

- [ ] **Step 1: The seam.** Three methods on `backend.BoardBackend` and the same three on the unexported `lifecycle` interface, so a backend that cannot manage sprints is refused through `errNoLifecycle` rather than panicking. The Jira implementation delegates to Task 1 and maps `RawSprint` the way `BoardSprints` does, taking `BoardID` from the argument rather than the wire.

- [ ] **Step 2: The rule becomes a test.** `exceptions_test.go` asserts the exact method set of `lifecycle` by name, with a failure message saying every method on it is a write reaching Jira outside a Commit and that adding one requires amending the spec. The previous version of this rule lived in a spec sentence and lasted one phase.

- [ ] **Step 3: The demo implementation.** Mutates the same in run sprint overlay `StartSprint` and `CompleteSprint` already mutate, under the same mutex. Create appends a future sprint with a fresh id above the highest held; edit rewrites the fields given and honours `clearGoal`; delete removes the sprint and returns its issues to the board's own scope. A real implementation, not a stub: Phase 3c's worst defect was a demo backend that ignored an argument. Test: create then list shows it as future with its goal; editing a name does not clear the goal; `clearGoal` does; delete removes it and its issues are still on the board.

- [ ] **Step 4: `Service.Create`.** Convert the bare dates through `sprintdate` as `guards.dates` already does, call the backend, refresh through `refreshSprints`, which is correct for a create. Write an audit row. Test: the created sprint reaches the cache with its goal; a refresh failure returns the sprint and a note rather than an error.

- [ ] **Step 5: `Service.Edit`, and the closed guard.** Read the state from the cache through `BoardSprintState` as `requireCompletable` does, and refuse a closed sprint, saying its dates are what velocity is computed from. An **active** sprint may be edited, but its end date is a burndown's axis, so the result carries a flag saying the sprint is running and the dialog confirms before sending. Then convert, call, refresh, audit. Test: closed is refused before any call; active is allowed and reports running; `clearGoal` reaches the backend.

- [ ] **Step 6: `Service.Delete`, its guards, and its own cache path.** The only irreversible action in TAM.
  - **Guard one, the state, read from Jira and not from the cache.** A sprint started on the web an hour ago still reads `future` locally. Re-read the board's sprints from the backend immediately before destroying anything and refuse on anything but `future`, naming the state actually found. Every other guard here reads the cache and refuses too much when stale, costing a Refresh. This one would permit too much, costing a sprint.
  - **Guard two, the journal.** Refuse while any `issue_sprint` row targets the sprint, through the `Pending` hook `refusePending` uses, naming Commit. Those rows would otherwise point at an id that no longer exists and fail at Commit days later.
  - **A sprint that vanished between the re-read and the call** is reported as already gone, which is a success with its own sentence, not an error.
  - **The cache, and all four places a deleted sprint hides.** Do not call `refreshSprints`; its empty-answer refusal is justified by an invariant delete falsifies. Remove, in one transaction: the `sprint` row; its `board_issue` scope; and **`issue.sprint_id` and `issue.sprint_name` on every issue that was in it**. That last one matters because `SprintField` deliberately re-adds an unknown sprint as its own option from `sprint_name`, so without it a deleted sprint keeps appearing in the Backlog grid, the Epics tree and the detail panel's select indefinitely, and `boardrepo.OpenSprints` keeps offering it to the New issue dialog and the importer's Sprint column.
  - Write an audit row. It is the only trace that will exist once Jira no longer has the sprint.
  - Test: a sprint the cache calls future but Jira calls active is refused, naming active; a future sprint with a pending row is refused and names Commit; a clean delete leaves the cache, leaves `OpenSprints`, and leaves no issue carrying its name; deleting the board's last sprint leaves an empty list rather than a stale row.

- [ ] **Step 7: The one Go suite run.** Inside `core/` then `tam/`: `go build ./... && go vet ./... && go test ./... -count=1`. Fix what fails, rerun at most twice, report. Commit as `feat(tam): a sprint can be made, changed and removed`.

---

### Task 4: the bound methods and the cached read

**Files:** create `tam/app_sprintmanage.go` and `app_sprintmanage_test.go`, create `tam/internal/boardrepo/sprintlist.go` and its test; regenerate `tam/frontend/wailsjs/**`.

**Produces:** `boardrepo.SprintDetail{Sprint; Issues []backend.Issue; Total, Done int; Points, DonePoints float64; MembershipCached, Truncated bool}` and `BoardSprintDetails(ctx, issues IssueSource, profileID string, boardID int) ([]SprintDetail, error)`; the bound methods `CreateSprint`, `EditSprint`, `DeleteSprint`, `ListBoardSprintDetails`.

- [ ] **Step 1: The read.** One board's sprints with their dates, their goal, **their issues as `backend.Issue` and not as keys**, and the four progress numbers `issuerepo.EpicTree` computes per epic, counted with `backend.IsDone`. Keys alone cannot draw a row that shows a summary and a status, cannot group by assignee, and cannot feed `IssueDetailPanel`, so returning them would force a second query and lose the single-snapshot guarantee this read exists for.
  - **The board's backlog is one of the returned scopes**, read from `board_issue` where `sprint_id` is empty, which is how the cache already stores it. Without it the view cannot fill a sprint, only move issues between sprints.
  - Order: active, then future by start date, then closed by start date descending. The backlog scope is last and is not a sprint.
  - A closed sprint has no cached membership by design, so `MembershipCached` is false and `Issues` is empty. The view must tell that from genuinely empty, which is why it is a field.
  - Cap the issue rows as `issuerepo.EpicTree` caps at 5,000 and set `Truncated`.
  - Run on the deferred read transaction the other board reads use.
  - Test: the order, the counts, a closed sprint reporting membership uncached, the backlog scope arriving, and the cap reporting truncation.

- [ ] **Step 2: The three write bindings.** Each through `requireProfile`, taking `a.acquire(p.ID, "sprint")`, reducing Jira's sentence through `ceremonyError`. Follow `app_sprints.go`; new file, not a bigger one. Wails discards a bound method's return value when the method also returns a non-nil error, so anything a dialog needs on partial failure travels in the result.

- [ ] **Step 3: The read binding.** `ListBoardSprintDetails`: `requireProfile`, no guard, non-nil slice. Test the guards.

- [ ] **Step 4: Regenerate the bindings.** Confirm the only Go visible additions are the four methods and the new struct. Commit as `feat(tam): the bindings a Sprints view needs`.

---

### Task 5: the view and its tree

**Files:** create `tam/frontend/src/components/SprintsView.tsx`, `SprintList.tsx`, `SprintRow.tsx`, `tam/frontend/src/lib/sprintGroups.ts`, `tam/frontend/src/lib/unfinished.ts`, and a test file for each; create `tam/frontend/src/queries/sprints.ts`; modify `queries/keys.ts`, `lib/format.ts`, `nav.ts`, `App.tsx`, `main.go`, `components/BoardCeremonies.tsx`, `CompleteSprintModal.tsx`, `StartSprintModal.tsx`, `EpicRow.tsx`, `App.css`.

- [ ] **Step 1: The shared pieces, first.** `lib/unfinished.ts` takes issues and columns and answers which are unfinished, by Phase 3c's rule; `BoardCeremonies`'s `incompleteCards` calls it and its copy is deleted. `lib/sprintGroups.ts` partitions a sprint's issues by assignee with a count and points per person, ordered **points descending, then count descending, then display name, unassigned last**, because Map insertion order is cache order and would reshuffle the tree between syncs. It is a pure partition with no tree semantics. `sprintDates` and `day` move out of `CompleteSprintModal` into `lib/format.ts`, which is the plan's own helper rule applied. `StartSprintModal`'s existing tests must still pass untouched.

- [ ] **Step 2: The view exists.** `View` gains `"sprints"`; `VIEWS` gains the entry between `boards` and `reports` **with its `phase` and `blurb`, both of which `ViewInfo` requires and the rail renders**; `menuViews` in `main.go` gains it with accelerator `4`, Reports becoming `5` and Rituals `6`. Nothing is conditional. `App.tsx` renders `SprintsView` for `current.id === "sprints"`.

- [ ] **Step 3: The queries.** `useBoardSprintDetails(profileId, boardId)` and the three mutations. The two ceremonies keep going through `runSprintCeremony`, which flips the whole app into its syncing state, because that is what they are. **Create, edit and delete take the Go lock but do not drive the global sync banner:** renaming a sprint is not a ceremony, and routing it through that path would make New sprint refuse during exactly the boards refresh a user runs to populate this view. Each invalidates the board's sprints, the board, the profile wide open sprints, and the suggestion.

- [ ] **Step 4: The row.** `SprintRow` is presentational, taking everything as props. It carries **five cells and a caret: name, state chip, date range, progress, menu trigger**. The goal is not on the row: it is the only field that is a sentence rather than a token, `.epic-row`'s tracks are fixed and every cell ellipsis-clips, and opening the detail panel narrows the pane, so a goal column would be empty on most rows and clipped to nothing on the rest. Its own classes are `.sprint-row` / `.sprint-cell`, sharing `.folder-item`, `.folder-selected`, `.folder-caret`.
  - The state chip renders "Active", "Future", "Closed" in the `TypeChip` idiom. Not uppercase: ten shouting rows is noise.
  - The date range uses `lib/format.ts`'s helper: "18 Aug to 1 Sep".
  - The progress line reads "8 of 14 done, 21 of 34 pts", and `EpicRow.progressText` is changed to match so two trees meant to read as one app do not use two sentences in the same column. **Suppress the progress cell entirely when `Total` is zero**, extending what `EpicRow` already does for points.
  - Actions live in a menu, following `CardMoveMenu`, reusing its `triggerTabIndex={-1}` and its class handle so the keyboard path can open it. Not inline buttons: they would need fixed tracks sized for the widest label and empty on every row that does not offer them, and the existing on-row action pattern `.folder-actions` is hover-only, which would hide the app's only irreversible action from every keyboard user. Menu items read "Edit sprint…" and "Delete sprint…", the ellipsis meaning a dialog follows.

- [ ] **Step 5: The tree.** `SprintList` owns the flattened rows, expansion, and roving tabindex keyboard navigation, mirroring `EpicTree`. An expanded sprint renders, in order: its goal as a `muted small` line; then its issues, with each assignee group introduced by a **non-interactive `role="presentation"` separator** carrying the display name, a count and points. A person is not a node: a group row would take an index in the roving model, need an expanded state, need answers for arrow keys and Enter, and have no detail panel to open. Issue rows stay the only `treeitem`s under a sprint, which is what keeps the tree two levels.
  - **Initial expansion is the active sprint only**, everything else collapsed, reset on profile switch. `EpicTree` opens every newly seen node, which on a board with twelve future sprints would paint twelve open branches.
  - The backlog scope renders as the last node, labelled as the backlog rather than as a sprint, with no state chip, no dates and no actions.

- [ ] **Step 6: The view's shell.** `SprintsView` holds the board picker, the show closed toggle, the selection, and the detail panel, mirroring `EpicsView` including its render phase profile switch reset.
  - **Above the tree**, in the slot `EpicsView` uses for `.epics-summary`, one line answering the question this view is opened for: "Sprint 12, day 6 of 14, 8 of 14 done, 21 of 34 pts". Absent when no sprint is active.
  - **The board picker is hidden when the profile has exactly one scrum board**, with the board named in the heading instead.
  - Branch in `EpicsView`'s order, **all four arms, for the picker and the tree alike**: error, loading, empty, data. Loading is a state, not an absence; an empty render is a claim.
  - **Write the three empty state sentences verbatim here**, the way the spec writes the ceremony sentences: no scrum board, a board with no sprints, and a sprint with no issues. Described-but-not-written copy is how "explains itself" becomes "No sprints."

- [ ] **Step 7: Tests.** The tree lists sprints in order and the backlog last; a closed one says its contents are not cached; a profile with no scrum board gets the empty state; the picker is absent with one board and present with two; the summary line names the active sprint; only the active sprint starts expanded; the assignee grouping orders by points then count then name with unassigned last; `unfinished` and `sprintGroups` have their own cases. Commit as `feat(tam): the Sprints view`.

---

### Task 6: the writes, the selection, and the marker

**Files:** create `tam/frontend/src/components/NewSprintModal.tsx`, `EditSprintModal.tsx`, `SprintDraftForm.tsx`, `SprintFillBar.tsx`, `ImmediateWriteChip.tsx`, and a test file for each; modify `frontend/core/src/components/useConfirm.tsx`, `SprintsView.tsx`, `SprintList.tsx`, `SprintRow.tsx`, `StartSprintModal.tsx`, `CompleteSprintModal.tsx`, `queries/sprints.ts`, `App.css`.

- [ ] **Step 1: The shared form.** `SprintDraftForm` is the four fields, their validation, and the suggestion seeding lifted out of `StartSprintModal`, which then renders it. Name required and trimmed; end not before start; a name past Jira's limit refused locally rather than by a round trip. **A name matching an existing open sprint on this board shows a `muted small` nudge**, not a block: Jira allows duplicates, and `lib/sprintOptions.duplicateNameIds` exists because two sprints sharing a name already made the pickers ambiguous.

- [ ] **Step 2: The marker.** `ImmediateWriteChip` is a `chip chip-now` at `.chip-status` scale reading "Sends to Jira now", painted `--accent-soft` on `--text-strong`. **Not amber and not danger:** amber in TAM means held locally and waiting for Commit, so an amber marker meaning the opposite inverts the app's own colour vocabulary and would read as a warning. This is a statement of fact about a normal action.
  - It goes in four dialogs' `.pending-head` and in the delete confirmation, each with the same shaped sentence: "Jira creates it now. This does not wait for Commit."
  - `StartSprintModal` already carries a bare version of that sentence; replace it with the chip so all of them read alike. This is why a shipped file is in this task's list.
  - `CompleteSprintModal` keeps its existing `.pending-banner-warn` about irreversibility. That is a different claim, and letting the new marker inherit the warn styling there would make the other four read as warnings by association.
  - It does **not** go on menu items. Three of them open a dialog that carries it, and a nine word menu item is a wall.
  - `ConfirmOptions.message` in `frontend/core` widens from `string` to `ReactNode` so the confirmation can carry it. Additive; XTM passes strings and is unaffected.

- [ ] **Step 3: The two dialogs.** `NewSprintModal` and `EditSprintModal` each render `SprintDraftForm` with their own title, button and call. Both **disable on submit**, as the create issue dialog does with `saving`: the per profile lock refuses a concurrent second call but not a second click after the first returns, and Jira allows two sprints with one name. Edit sends `clearGoal` when the user has emptied a goal that arrived non-empty, which is knowable now that Task 2 put the goal in the cache, and confirms before moving a running sprint's end date.

- [ ] **Step 4: Delete.** A `useConfirm` with `danger: true`, `confirmLabel: "Delete sprint"`, `cancelLabel: "Keep it"`, and a message saying three things: the sprint by name and how many issues it holds; **where those issues go**, "Jira moves its 6 issues back to the backlog. The issues themselves are not deleted"; and that it cannot be undone from TAM or from Jira.
  - **When `Truncated` is set the count is a floor**, so say "at least 5,000 issues" or omit the number. Naming a wrong number in the one irreversible confirmation is worse than naming none.
  - No type-the-name gate: it is a sprint, not a repository, and the app has no other typed confirmation to be consistent with.
  - Success says where the record is: "Sprint 13 deleted. It is in the activity log."
  - **A 403 arrives after the confirmation has closed and has nowhere to land**, so it goes to `useNotice({ tone: "error" })`, the way `SprintField` reports a failed move.

- [ ] **Step 5: Success, and the states nobody specified.** Every one of the five writes ends somewhere visible.
  - A created sprint scrolls into view and flashes once, reusing `EpicTree`'s moved-row effect and its class, and `announce()` names it. Without this the dialog closes and the user hunts a list for the thing they just made, which is the defect the create issue dialog already fixed by announcing its key.
  - The `note` a partial success carries renders in the same `role="status"` banner the ceremonies already use. Nothing currently says who renders it.
  - A row's menu is disabled while that row's own mutation is pending.
  - A truncated sprint says it is showing 5,000 of more.

- [ ] **Step 6: Filling a sprint.** `SprintFillBar` is a count and one Move action over a selection of issue rows, journaled through the `MoveManyToSprint` path the board's selection already uses. Reuse `lib/boardSelection.ts`, with four things it does not currently handle:
  - Its `order` must contain **only issue row keys**, in flattened visible order, never sprint or group header ids: `extend` slices `order` blindly, and a header id would end up in the set and then in the bulk move.
  - **Shift-extend can cross a sprint boundary here**, which is impossible on a board. Clamp the range to the anchor's own scope. If that turns out to fight the helper, say so in the report rather than forking it.
  - **Space checks, Enter selects** (Decision 7). `EpicTree` binds both to select today.
  - **The checked paint is a real checkbox** in a cell beside the caret on issue rows only, not a background: `.folder-selected` is already `--accent-soft` so a checked-not-selected row would be indistinguishable, and amber is spoken for. Ctrl-click on a tree row is undiscoverable in a way Ctrl-click on a card is not.
  - **Reserve the tree pane's width while a selection exists.** `.epics-body:has(.detail-panel)` narrows the pane, so hiding the panel when a second row is checked would jump every row sideways mid-gesture. Keep the panel mounted with a "3 issues selected" message instead.

- [ ] **Step 7: Start and complete from here.** Both open the existing modals. The completion takes its unfinished list from `lib/unfinished.ts` over this view's data rather than a drawn board.

- [ ] **Step 8: Tests, then the one frontend run.** Creating calls the binding with the board id; a second click while saving does nothing; a duplicate name nudges without blocking; emptying a goal sends the clear flag; moving a running sprint's end date confirms first; deleting names the count and the destination and offers no count when truncated; deleting an active sprint is not offered; a 403 after the confirm reaches a notice; the fill bar journals through the bulk move and never selects a header; Space checks and Enter selects; the marker appears in four dialogs and the confirmation and on no menu item. Then `npx vitest run` and `npx tsc --noEmit` in `tam/frontend`; fix what fails, rerun at most twice. Commit as `feat(tam): sprints are made, changed and filled from their own view`.

---

### Task 7: docs, the amendments, the single gate run

- [ ] **Step 1: Two amendments.** Section 14.2 of the boards spec says the two ceremonies are the only place a button talks to Jira without Commit: name the three actions this plan adds, point at this design, note that `internal/sprints`'s `lifecycle` interface is now the enforced list, and keep the original argument, because it is still the argument. And **the Sprints design's own mockup**, which draws inline row buttons and omits the goal, disagrees with what this plan builds twice over; fix the picture, because someone will build from it.

- [ ] **Step 2: Docs.** `tam/CLAUDE.md` gains a section: what the view does; that create, edit and delete are online only and why; that the `lifecycle` interface is the list and a test asserts it; that membership stays journaled; that a closed sprint has no cached membership by design; that delete removes rows from the cache itself rather than through `refreshSprints`, with the reason; and that it also clears `issue.sprint_id` and `issue.sprint_name`, with the reason. Record the honest version of the rule: these writes are immediate because a sprint's id has to be real before anything can point at it, not because a sprint is more of a Jira object than an issue is. Note schema version 7. Update the Status paragraph and the Layout tree.

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

Record each result, fix what fails, rerun only what failed. Do not lower an assertion to make a test pass. Vitest before this plan: 46 in `frontend/core`, 159 in `xtm`, 293 in `tam`.

- [ ] **Step 4: The walk-through for the user** (not run by agents): on the demo profile, create a sprint, see it flash into the tree, edit its name and goal, clear the goal and confirm it stays cleared, start it, select three backlog issues and move them in, Commit, complete it, then delete a future sprint and read the confirmation. Delete the last sprint on a board and confirm it does not come back, and that no issue still shows its name. Switch to a kanban only profile and confirm the view is present and explains itself. On a real Data Center, the three things Task 0's probe answered, confirmed against the built app.

- [ ] **Step 5: Commit** as `docs(tam): the Sprints view`, then push and open the PR against `main`, noting that it stacks on the sprint linking PR.

## Deferred

Editing a closed sprint's dates. Deleting an active sprint. Creating a board. Carrying unfinished issues forward at create time, which is a completion behaviour. A team roster with capacity per person, which is Phase 4 work and which the assignee grouping here is the seed of. Remembering a 403 for the session so a user without Manage Sprints is told once rather than once per action. Collapsible assignee groups, which would make the tree three levels and need their own keyboard answers.
