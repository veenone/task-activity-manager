# Task Activity Manager: the Sprints view

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax. Lean cycle: implementers write the tests named here but run no suites, except the one Go run at the end of Task 2 and the one frontend run at the end of Task 5. Task 6 runs every gate once.

**Goal:** a view where a sprint can be created, edited, deleted, started, completed, and read with its contents, and which is absent on a project that cannot have sprints.

**Why this exists:** Phase 3 can draw a sprint and move cards through it but cannot make one, and the only way to reach a sprint at all is to pick the board that carries it first. The design is `docs/superpowers/specs/2026-09-10-tam-sprints-view-design.md`.

**Architecture:** three new Agile calls behind `backend.BoardBackend`, three new methods on `internal/sprints.Service` beside the two ceremonies it already owns, one new cached read in `boardrepo`, and a new view built from the Epics view's parts. The view hides itself through a flag on `ViewInfo` that the tabs, the rail and the native menu all read, with the native menu told by the frontend the way the nav rail checkbox already tells it.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

## Global Constraints

- Nothing under `xtm/` changes. Go commands run from inside the module directory.
- **Creating, editing and deleting a sprint reach Jira immediately, and nothing else added here does.** They live in `internal/sprints` beside `Start` and `Complete`, behind the same per profile lock under the same `"sprint"` label, and are reached from the frontend through `SyncContext.runSprintCeremony`. Membership stays journaled through the paths that already exist. No fourth exception may be added without amending the spec.
- Section 14.2 of `docs/superpowers/specs/2026-09-07-tam-boards-design.md` currently says the two ceremonies are the only place in TAM where a button talks to Jira without Commit. That sentence becomes false in this plan and is amended in Task 6, not left to rot.
- A sprint's dates are written and parsed only through `internal/sprintdate`.
- Jira's sprint update endpoint is a partial update: a key that is present overwrites, one that is absent is left alone. An empty goal is omitted, never sent as `""`.
- Every Jira sentence shown to a user goes through `internal/errtext` first.
- Files stay small and single purpose; a helper used from two places lives in its own module. `internal/backend/backend.go` (449 lines) and `frontend/src/api.ts` (848) are already at the ceiling: add a line each, not a block. New behaviour goes in new files.
- TAM mirrors XTM's design language: reuse `folder-tree`, `folder-item`, `folder-caret`, `edit-row`, `detail-input`, `muted small`. No new class where one exists.
- UI text uses no em dashes. No AI attribution anywhere. Conventional commit prefixes, no trailers. Never add, commit or delete untracked local tooling files; revert Wails churn under `tam/frontend/wailsjs/runtime/` and `tam/frontend/package.json.md5` with `git checkout --`.

## Decisions

1. **The unfinished rule moves before it is used twice.** `incompleteCards(view)` in `BoardCeremonies.tsx` derives "unfinished" from a drawn `BoardView`. The Sprints view has no `BoardView`, so the rule moves to its own module taking issues and columns, and both callers use it. Copying it would put Phase 3c's one definition in two places.
2. **The create dialog and the start dialog share a form, not a component.** Both collect name, goal, start and end, and both seed from `SuggestSprintDates`. `StartSprintModal` is built around a sprint that already exists in Jira, so the four fields plus their validation lift into a shared piece and each dialog keeps its own title, its own button, and its own call.
3. **The view is conditional on a scrum board, not on a sprint.** Conditioning on sprints would mean the first sprint could never be created, because this view is where it is created.

---

### Task 1: the three Agile calls

**Files:** create `core/jira/sprintwrite.go` and `core/jira/sprintwrite_test.go`.

**Produces:** `func (c *Client) CreateSprint(ctx context.Context, boardID int, name, goal, start, end string) (RawSprint, error)`, `func (c *Client) UpdateSprint(ctx context.Context, sprintID int, name, goal, start, end string) error`, `func (c *Client) DeleteSprint(ctx context.Context, sprintID int) error`.

- [ ] **Step 1: Create.** `POST /rest/agile/1.0/sprint` with `originBoardId`, `name`, `startDate`, `endDate`, and `goal` only when non empty. It answers with the created sprint; decode it into the existing `RawSprint` and return it, because the caller needs the id Jira assigned and nothing else knows it. Read `StartSprint` in `agile.go` first and follow its shape, its error wrapping and its comment style. Test: the request body carries the board id and omits an empty goal, and the answer's id comes back.

- [ ] **Step 2: Update.** `POST /rest/agile/1.0/sprint/{id}`, sending only the keys the caller gave. This is the same partial update `StartSprint` uses and carries the same trap, so the comment says so: an absent key is left alone and an empty string overwrites. Do not send `state`. Test: an edit that changes only the name sends only the name; an empty goal is not in the body.

- [ ] **Step 3: Delete.** `DELETE /rest/agile/1.0/sprint/{id}`. Check what `WriteJSON` and its siblings offer for a body-less method before adding one. Test: the path carries the id, and a 404 comes back as an error naming the sprint.

- [ ] **Step 4: The comment that keeps the exception honest.** Each of the three carries the sentence `StartSprint` already carries, that this is one of the calls reaching Jira outside a Commit, and why. Commit as `feat(core): create, edit and delete a sprint`.

---

### Task 2: the backend seam, the service, and its guards

**Files:** modify `tam/internal/backend/backend.go` (three interface lines only), create `tam/internal/backend/jira/sprintwrite.go`, modify `tam/internal/backend/demo/boards.go` and its test, create `tam/internal/sprints/manage.go` and `manage_test.go`, modify `tam/internal/sprints/sprints.go` (the `lifecycle` interface only) and `guards.go`.

**Interfaces:**
- Consumes: Task 1's three client methods.
- Produces: `BoardBackend.CreateSprint(ctx, boardID int, d backend.SprintDraft) (backend.Sprint, error)`, `BoardBackend.EditSprint(ctx, sprintID int, d backend.SprintDraft) error`, `BoardBackend.DeleteSprint(ctx, sprintID int) error`; and on `*sprints.Service`: `Create(ctx, profileID string, boardID int, d backend.SprintDraft) (backend.Sprint, string, error)`, `Edit(ctx, profileID string, boardID, sprintID int, d backend.SprintDraft) (string, error)`, `Delete(ctx, profileID string, boardID, sprintID int) (string, error)`. The trailing `string` on each is the note a failed cache refresh leaves, empty when there is none, exactly as `Service.Start` already returns one.

- [ ] **Step 1: The seam.** Three methods on `backend.BoardBackend`, and the same three on the unexported `lifecycle` interface in `sprints.go` so a backend that cannot manage sprints is refused through the existing `errNoLifecycle` path rather than panicking. The Jira implementation delegates to Task 1 and maps `RawSprint` to `backend.Sprint` the way `BoardSprints` already does, including taking `BoardID` from the argument rather than the wire.

- [ ] **Step 2: The demo implementation.** The demo mutates the same in run sprint overlay `StartSprint` and `CompleteSprint` already mutate, under the same mutex. Create appends a future sprint with a fresh id above the highest it holds; edit rewrites the fields it was given and leaves the rest; delete removes the sprint and returns its issues to the board's own scope. This is a real implementation, not a stub: the acceptance walk through runs on the demo profile, and Phase 3c's worst defect was a demo backend that ignored an argument. Test: creating then listing shows the new sprint as future; editing a name does not clear the goal; deleting removes it and its issues are still on the board.

- [ ] **Step 3: `Service.Create`.** Convert the dialog's bare dates through `sprintdate` the way `guards.go`'s `dates` already does, call the backend, then refresh the board's sprints through the existing `refreshSprints`, whose refusal to persist an empty answer is deliberate and must not be bypassed. Return the created sprint, the note, and any error. Test: the created sprint reaches the cache; a refresh failure returns the sprint and a note rather than an error.

- [ ] **Step 4: `Service.Edit`, and the guard that a closed sprint refuses.** Read the sprint's state from the cache through `BoardSprintState` the way `requireCompletable` does, and refuse a closed one with a sentence saying its dates are what velocity is computed from. Then the same convert, call, refresh. Test: a closed sprint is refused before any call is made; an active and a future one are not.

- [ ] **Step 5: `Service.Delete`, and its two guards.** Refuse anything that is not future, naming the state it found. Refuse while journal rows target the sprint, through the same `Pending` hook `refusePending` already uses, naming Commit as the thing to do first. Then delete, then refresh. Test: an active sprint is refused; a future sprint with a pending row is refused and names Commit; a clean future sprint is deleted and leaves the cache.

- [ ] **Step 6: The one Go suite run.** Inside `core/`: `go build ./... && go test ./... -count=1`. Inside `tam/`: the same. Fix what fails, rerun at most twice, report. Commit as `feat(tam): a sprint can be made, changed and removed`.

---

### Task 3: the bound methods and the cached read

**Files:** create `tam/app_sprintmanage.go` and `app_sprintmanage_test.go`, create `tam/internal/boardrepo/sprintlist.go` and its test, modify `tam/internal/boardrepo/boardrepo.go` only if a shared type belongs there; regenerate `tam/frontend/wailsjs/**`.

**Interfaces:**
- Consumes: Task 2's three service methods.
- Produces: `boardrepo.SprintDetail{Sprint; Total int; Done int; Points float64; DonePoints float64; Keys []string}` and `func (r *Repository) BoardSprintDetails(ctx context.Context, issues IssueSource, profileID string, boardID int) ([]SprintDetail, error)`; the bound methods `CreateSprint`, `EditSprint`, `DeleteSprint`, `ListBoardSprintDetails`, and `HasScrumBoard(profileID string) (bool, error)`.

- [ ] **Step 1: The read.** One board's sprints with their dates, their cached issue keys, and the same four progress numbers `issuerepo.EpicTree` computes per epic, counted with `backend.IsDone` so the definition stays in one place. Order active, then future by start date, then closed by start date descending. A closed sprint has no cached membership by design, so its `Keys` is empty and the caller must be able to tell that from a sprint that is genuinely empty: carry a field saying so rather than making the view infer it. Run on the deferred read transaction the other board reads use, so the sprints and the issues come from one snapshot. Test: the order, the counts, and a closed sprint reporting unavailable rather than empty.

- [ ] **Step 2: `HasScrumBoard`.** True when the profile has a cached board whose type is scrum. A profile with no boards is false and no error. Test: both, plus a kanban only profile being false.

- [ ] **Step 3: The three write bindings.** Each goes through `requireProfile`, takes `a.acquire(p.ID, "sprint")` and releases it, and reduces a Jira sentence through `ceremonyError` the way `StartSprint` already does. Follow `app_sprints.go` exactly; put them in a new file rather than growing it. Remember that Wails discards a bound method's return value when the method also returns a non-nil error, so anything the dialog needs on a partial failure travels in the result, not in the error.

- [ ] **Step 4: The two read bindings.** `ListBoardSprintDetails` and `HasScrumBoard` are cache reads: `requireProfile`, no guard, non-nil slice. Test the guards: no profile is an error, an unknown board is an empty list.

- [ ] **Step 5: Regenerate the bindings** and confirm the only Go visible additions are the five methods and the new struct. Commit as `feat(tam): the bindings a Sprints view needs`.

---

### Task 4: the view can be hidden

**Files:** modify `tam/frontend/src/nav.ts`, `App.tsx`, `tam/main.go`, `tam/app.go`, and their tests; create `tam/frontend/src/queries/sprints.ts`.

**Interfaces:**
- Consumes: Task 3's `HasScrumBoard`.
- Produces: `useHasScrumBoard(profileId)`; `ViewInfo.conditional?: boolean`; the bound `SetSprintsViewVisible(v bool) error`.

- [ ] **Step 1: The flag.** `ViewInfo` gains a field marking a view conditional. `VIEWS` gains the `sprints` entry between `boards` and `reports`, marked conditional. `View` gains `"sprints"`. Nothing else in `nav.ts` changes.

- [ ] **Step 2: The three places that filter.** The view tabs and the nav rail render from a filtered list. `App.tsx`'s `menu:view` handler rejects a hidden id as well as an unknown one, because a stale accelerator would otherwise switch to a view with no tab. If the active view becomes hidden on a profile switch, the app falls back to the Backlog rather than rendering nothing.

- [ ] **Step 3: The native menu.** `menuViews` gains the entry with accelerator `4`; Reports becomes `5` and Rituals `6`. The loop that builds the View submenu skips an item the app has been told to hide. `App` holds the answer, `SetSprintsViewVisible` stores it and calls `refreshMenu`, and the value is persisted as a shared setting so the next launch builds the menu right the first time. Read `SetNavRailVisible` and `setShowNavRail` in `app.go` first: this is the same shape, including the rebuild, and including that the rebuild exists because Wails renders an item from the value it was built with.

- [ ] **Step 4: The frontend tells it.** When `useHasScrumBoard` resolves, and again on a profile switch, the frontend calls `SetSprintsViewVisible`. It is a hint being corrected, so a failure is logged and not shown: the tabs and the rail are already right, and only the menu is stale.

- [ ] **Step 5: Tests.** `nav.test.ts` or `App.test.tsx`: a profile with a scrum board shows the tab, one without hides it from both the tabs and the rail, a `menu:view` event naming a hidden view does nothing, and a profile switch away from a scrum board moves the active view off Sprints. Commit as `feat(tam): a view that is not always there`.

---

### Task 5: the view itself

**Files:** create `tam/frontend/src/components/SprintsView.tsx`, `SprintList.tsx`, `SprintRow.tsx`, `NewSprintModal.tsx`, `EditSprintModal.tsx`, `SprintFields.tsx`, `tam/frontend/src/lib/unfinished.ts`, and a test file for each; modify `queries/sprints.ts`, `queries/keys.ts`, `components/BoardCeremonies.tsx`, `StartSprintModal.tsx`, `App.tsx`, and `App.css`.

**Interfaces:** consumes Task 3's reads and writes and Task 4's query.

- [ ] **Step 1: The shared pieces, before anything uses them.** `lib/unfinished.ts` takes issues and the board's columns and answers which are unfinished, by the rule Phase 3c set: a status id not in the last column. `BoardCeremonies`'s `incompleteCards` is rewritten to call it and its own copy deleted. `SprintFields.tsx` is the four field form, its validation, and its suggestion seeding, lifted out of `StartSprintModal` which then renders it. Both get their own tests, and `StartSprintModal`'s existing tests must still pass untouched: if one needs changing, the lift changed behaviour and that is a defect, not a test to update.

- [ ] **Step 2: The queries.** `useBoardSprintDetails(profileId, boardId)` and the three mutations, each through `runSprintCeremony` the way `useStartSprint` already is, each invalidating what a sprint write can change: the board's sprints, the board itself, the profile wide open sprints, and the suggestion.

- [ ] **Step 3: The tree.** `SprintRow` is presentational, taking everything as props, mirroring `EpicRow`: name, state chip, dates, the progress line, and its actions. `SprintList` owns the flattened rows, expansion, and roving tabindex keyboard navigation, mirroring `EpicTree`. Read both files first and follow them; where they solved a problem, solve it the same way rather than a new way.

- [ ] **Step 4: The view.** `SprintsView` holds the board picker (scrum boards only), the show closed toggle, the selection, and the detail panel, mirroring `EpicsView` including its render phase profile switch reset. A board with no sprints says so and offers the create button rather than drawing an empty tree. A closed sprint's contents say they are not cached, with the reason, rather than looking empty.

- [ ] **Step 5: The dialogs.** `NewSprintModal` and `EditSprintModal` each render `SprintFields` with their own title, button and call. Delete is a confirmation through `useConfirm`, naming how many issues the sprint holds and that Jira returns them to the backlog. Start and Complete open the existing modals; wire them so the completion gets its unfinished list from `lib/unfinished.ts` over this view's own data rather than a drawn board.

- [ ] **Step 6: The route.** `App.tsx` renders `SprintsView` for `current.id === "sprints"` instead of falling through to `Placeholder`.

- [ ] **Step 7: Tests, then the one frontend run.** `SprintsView.test.tsx`: the tree lists the board's sprints in order, a closed one says its contents are not cached, creating one calls the binding with the board id, deleting a future one confirms first and names the count, and deleting an active one is not offered. `SprintRow` and `SprintList`: the actions a state offers, and keyboard navigation. `SprintFields`: the validation cases, once, since two dialogs now depend on them. Then `npx vitest run` and `npx tsc --noEmit` in `tam/frontend`; fix what fails, rerun at most twice. Commit as `feat(tam): the Sprints view`.

---

### Task 6: docs, the amendment, the single gate run

- [ ] **Step 1: The amendment.** Section 14.2 of `docs/superpowers/specs/2026-09-07-tam-boards-design.md` says the two ceremonies are the only place in TAM where a button talks to Jira without Commit. Amend it: name the three actions this plan adds, point at this design for the reasoning, and keep the original argument intact rather than rewriting it, because it is still the argument.

- [ ] **Step 2: Docs.** `tam/CLAUDE.md` gains a section for the view: what it does, that create, edit and delete are online only and why, that membership stays journaled, the rule for when the view is present and the consequence that a profile which has never refreshed its boards will not see it, and that a closed sprint has no cached membership by design. Update the Status paragraph and the Layout tree.

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

- [ ] **Step 4: The walk-through for the user** (not run by agents): on the demo profile, create a sprint, edit its name and goal, start it, put an issue in it from the detail panel, Commit, complete it, then delete a future sprint and read the confirmation. Switch to a kanban only profile and confirm the view is gone from the tabs, the rail and the View menu. On a real Data Center, the three things no fixture proves: that create answers with the sprint's id in the shape this expects, that delete returns the issues to the backlog rather than deleting them, and that an account without Manage Sprints gets a 403 rather than a silent 200.

- [ ] **Step 5: Commit** as `docs(tam): the Sprints view`, then push and open the PR against `main`, noting that it stacks on the sprint linking PR.

## Deferred

Editing a closed sprint's dates. Deleting an active sprint. Creating a board. Bulk adding issues to a sprint from this view. A sprint's goal shown outside its dialogs, which is a Reports question.
