<!-- /autoplan restore point: /c/Users/ARaha/.gstack/projects/veenone-task-activity-manager/feat-tam-sprints-view-autoplan-restore-20260910-090550.md -->
# Task Activity Manager: the Sprints view

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax. Lean cycle: implementers write the tests named here but run no suites, except the one Go run at the end of Task 2 and the one frontend run at the end of Task 5. Task 6 runs every gate once.

**Goal:** a view where a sprint can be created, edited, deleted, started, completed, filled, and read with its contents and who is carrying them.

**Why this exists:** Phase 3 can draw a sprint and move cards through it but cannot make one, and the only way to reach a sprint at all is to pick the board that carries it first. The design is `docs/superpowers/specs/2026-09-10-tam-sprints-view-design.md`.

**Architecture:** three new Agile calls behind `backend.BoardBackend`, three new methods on `internal/sprints.Service` beside the two ceremonies it already owns, one new cached read in `boardrepo`, and a new view built from the Epics view's parts. The view is always present; a project that cannot have sprints gets an empty state that says why.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

## Global Constraints

- Nothing under `xtm/` changes, and nothing is added to `core/settings`, which is global and shared with XTM. Go commands run from inside the module directory.
- **Creating, editing and deleting a sprint reach Jira immediately, and nothing else added here does.** They live in `internal/sprints` beside `Start` and `Complete`, behind the same per profile lock under the same `"sprint"` label. Membership stays journaled through the paths that already exist. No sixth exception may be added without amending the spec, and Task 2 makes that rule a failing test rather than a paragraph.
- **Every one of the five immediate writes is marked as such in the UI.** A user cannot otherwise tell which buttons wait for Commit and which do not, and by this plan five of them do not.
- Section 14.2 of `docs/superpowers/specs/2026-09-07-tam-boards-design.md` currently says the two ceremonies are the only place in TAM where a button talks to Jira without Commit. That sentence becomes false in this plan and is amended in Task 6, not left to rot.
- A sprint's dates are written and parsed only through `internal/sprintdate`.
- Jira's sprint update endpoint is a partial update: a key that is present overwrites, one that is absent is left alone. **On create an empty goal is omitted. On edit, a goal the user has emptied is sent as `""`, because otherwise a goal can be set from TAM and never cleared.** These are different rules and the code says which is which.
- Every Jira sentence shown to a user goes through `internal/errtext` first.
- Files stay small and single purpose; a helper used from two places lives in its own module. `internal/backend/backend.go` (449 lines) and `frontend/src/api.ts` (848) are already at the ceiling: add a line each, not a block. New behaviour goes in new files.
- TAM mirrors XTM's design language: reuse `folder-tree`, `folder-item`, `folder-caret`, `edit-row`, `detail-input`, `muted small`. No new class where one exists.
- UI text uses no em dashes. No AI attribution anywhere. Conventional commit prefixes, no trailers. Never add, commit or delete untracked local tooling files; revert Wails churn under `tam/frontend/wailsjs/runtime/` and `tam/frontend/package.json.md5` with `git checkout --`.

## Decisions

1. **The unfinished rule moves before it is used twice.** `incompleteCards(view)` in `BoardCeremonies.tsx` derives "unfinished" from a drawn `BoardView`. The Sprints view has no `BoardView`, so the rule moves to its own module taking issues and columns, and both callers use it.
2. **The create dialog and the start dialog share a form, not a component.** Both collect name, goal, start and end, and both seed from `SuggestSprintDates`. `StartSprintModal` is built around a sprint that already exists in Jira, so the four fields plus their validation lift into a shared piece and each dialog keeps its own title, its own button, and its own call.
3. **The view is always present.** A profile with no scrum board gets an empty state naming the reason and pointing at the Boards view, the way Reports and Rituals already ship as visible entries. Hiding it would have cost a cross cutting navigation mechanism for one consumer, and would have shown a new user nothing at all on their first launch, because the condition reads a cache that a fresh profile has not filled yet.
4. **Delete does not go through `refreshSprints`.** That helper refuses to persist an empty answer because a ceremony proves the board has at least one sprint. Delete is the one write that can make the answer genuinely empty, so it removes the sprint from the cache itself.
5. **A sprint is filled from this view, not only inspected.** Reaching a sprint without picking its board is the reason the view exists, and deferring bulk add would have sent the user back to the board to fill it.
6. **Membership is grouped by the person carrying it.** The same read answers what is in the sprint and who is on it.

---

### Task 0: verify the wire before anything is built on it

**Files:** create `docs/superpowers/plans/assets/2026-09-10-sprint-wire-probe.md`.

Three assumptions in this plan are unverified against a real Data Center, and tasks 1 through 3 are shaped by all three. Verifying them after six tasks of work is the wrong order: if create answers 201 with no body, Task 1's signature is wrong and everything downstream shifts.

- [ ] **Step 1: Write the probe.** A short document the user runs by hand against their own instance: three `curl` invocations, with a placeholder for the base URL and the PAT, and next to each the answer this plan assumes.
  1. `POST /rest/agile/1.0/sprint` with a name, an origin board id and two dates. **Assumed:** 200 or 201 with a JSON body carrying the new sprint's `id`.
  2. `DELETE /rest/agile/1.0/sprint/{id}` on that sprint, after putting one issue in it. **Assumed:** the issue returns to the backlog rather than being deleted, and the sprint is gone.
  3. Either call as an account without Manage Sprints. **Assumed:** 403 with a message, not a 200 that silently does nothing.
  The document says plainly what to do if an answer differs, and that the PAT goes in a header and must not be pasted into any file that gets committed.

- [ ] **Step 2: Code for both shapes anyway.** Task 1 treats a create whose answer has no body, or a body whose `id` is zero, as "created, the refresh will find it" rather than as a failure. The probe tells us which path is live; the code survives either. Commit as `docs(tam): a probe for the three sprint calls`.

---

### Task 1: the three Agile calls

**Files:** create `core/jira/sprintwrite.go` and `core/jira/sprintwrite_test.go`.

**Produces:** `func (c *Client) CreateSprint(ctx context.Context, boardID int, name, goal, start, end string) (RawSprint, error)`, `func (c *Client) UpdateSprint(ctx context.Context, sprintID int, name, goal, start, end string, clearGoal bool) error`, `func (c *Client) DeleteSprint(ctx context.Context, sprintID int) error`.

- [ ] **Step 1: Create.** `POST /rest/agile/1.0/sprint` with `originBoardId`, `name`, `startDate`, `endDate`, and `goal` only when non empty. Decode the answer into the existing `RawSprint`. An empty body, or a body whose `id` is zero, is not an error: return the zero value and let the caller refresh, because the sprint exists in Jira either way and failing here would tell the user a lie. Read `StartSprint` in `agile.go` first and follow its shape, its error wrapping and its comment style. Test: the body carries the board id and omits an empty goal; the answer's id comes back; an empty answer body returns no error.

- [ ] **Step 2: Update.** `POST /rest/agile/1.0/sprint/{id}`, sending only the keys the caller gave. This is the same partial update `StartSprint` uses and carries the same trap, so the comment says so: an absent key is left alone and an empty string overwrites. `clearGoal` is what distinguishes "the user did not touch the goal" from "the user emptied it"; when it is true the body carries `goal: ""` on purpose. Do not send `state`. Test: an edit that changes only the name sends only the name; `clearGoal` false with an empty goal omits it; `clearGoal` true sends the empty string.

- [ ] **Step 3: Delete.** `DELETE /rest/agile/1.0/sprint/{id}`. Check what `WriteJSON` and its siblings offer for a body-less method before adding one. Test: the path carries the id, and a 404 comes back as an error naming the sprint.

- [ ] **Step 4: The comment that keeps the exception honest.** Each of the three carries the sentence `StartSprint` already carries, that this is one of the calls reaching Jira outside a Commit, and why. Commit as `feat(core): create, edit and delete a sprint`.

---

### Task 2: the backend seam, the service, and its guards

**Files:** modify `tam/internal/backend/backend.go` (three interface lines only), create `tam/internal/backend/jira/sprintwrite.go`, modify `tam/internal/backend/demo/boards.go` and its test, create `tam/internal/sprints/manage.go`, `manage_test.go` and `exceptions_test.go`, modify `tam/internal/sprints/sprints.go` (the `lifecycle` interface only), `guards.go` and `cache.go`.

**Interfaces:**
- Consumes: Task 1's three client methods.
- Produces: `BoardBackend.CreateSprint(ctx, boardID int, d backend.SprintDraft) (backend.Sprint, error)`, `BoardBackend.EditSprint(ctx, sprintID int, d backend.SprintDraft, clearGoal bool) error`, `BoardBackend.DeleteSprint(ctx, sprintID int) error`; and on `*sprints.Service`: `Create(ctx, profileID string, boardID int, d backend.SprintDraft) (backend.Sprint, string, error)`, `Edit(ctx, profileID string, boardID, sprintID int, d backend.SprintDraft, clearGoal bool) (string, error)`, `Delete(ctx, profileID string, boardID, sprintID int) (string, error)`. The trailing `string` is the note a failed cache refresh leaves, exactly as `Service.Start` already returns one.

- [ ] **Step 1: The seam.** Three methods on `backend.BoardBackend`, and the same three on the unexported `lifecycle` interface in `sprints.go` so a backend that cannot manage sprints is refused through the existing `errNoLifecycle` path rather than panicking. The Jira implementation delegates to Task 1 and maps `RawSprint` to `backend.Sprint` the way `BoardSprints` already does, including taking `BoardID` from the argument rather than the wire.

- [ ] **Step 2: The rule becomes a test.** `exceptions_test.go` asserts the exact method set of the `lifecycle` interface by name. Growing it then means editing a failing test that says, in its own message, that every method on it is a write reaching Jira outside a Commit and that adding one requires amending the spec. The rule that fences TAM's differentiator has lived in a markdown paragraph and lasted exactly one phase; this makes it structural.

- [ ] **Step 3: The demo implementation.** The demo mutates the same in run sprint overlay `StartSprint` and `CompleteSprint` already mutate, under the same mutex. Create appends a future sprint with a fresh id above the highest it holds; edit rewrites the fields it was given and leaves the rest, honouring `clearGoal`; delete removes the sprint and returns its issues to the board's own scope. This is a real implementation, not a stub: the acceptance walk through runs on the demo profile, and Phase 3c's worst defect was a demo backend that ignored an argument. Test: creating then listing shows the new sprint as future; editing a name does not clear the goal; `clearGoal` does; deleting removes it and its issues are still on the board.

- [ ] **Step 4: `Service.Create`.** Convert the dialog's bare dates through `sprintdate` the way `guards.go`'s `dates` already does, call the backend, then refresh the board's sprints through the existing `refreshSprints`, whose refusal to persist an empty answer is deliberate and correct for a create. Write an audit row. Return the created sprint, the note, and any error. Test: the created sprint reaches the cache; a refresh failure returns the sprint and a note rather than an error.

- [ ] **Step 5: `Service.Edit`, and the guard that a closed sprint refuses.** Read the sprint's state from the cache through `BoardSprintState` the way `requireCompletable` does, and refuse a closed one with a sentence saying its dates are what velocity is computed from. An **active** sprint may be edited, but moving its end date is what a burndown's axis is built from, so the service returns a flag saying the sprint is running and the dialog confirms before sending. Then convert, call, refresh, audit. Test: a closed sprint is refused before any call is made; an active one is allowed and reports that it is running; `clearGoal` reaches the backend.

- [ ] **Step 6: `Service.Delete`, its three guards, and its own cache path.** This is the only irreversible action in TAM and it gets the most care.
  - **Guard one, the state, read from Jira and not from the cache.** A sprint someone started on the web an hour ago still reads `future` locally. Re-read the board's sprints from the backend immediately before destroying anything and refuse on anything but `future`, naming the state that was actually found. Every other guard in this package reads the cache, and every one of them refuses too much when it is stale, which costs a Refresh; this one would permit too much, which costs a sprint.
  - **Guard two, the journal.** Refuse while any `issue_sprint` row targets the sprint, through the same `Pending` hook `refusePending` already uses, naming Commit as the thing to do first. Those rows would otherwise point at an id that no longer exists and fail at Commit days later.
  - **Guard three, the state again after the read.** The re-read is also what the delete call uses, so a sprint that vanished between the two is reported as already gone rather than as an error.
  - **The cache.** Do not call `refreshSprints`. Its refusal to persist an empty answer is justified by a comment saying a ceremony proves the board has a sprint, and delete is precisely the write that falsifies that. Remove the sprint's own row and its `board_issue` scope directly, because delete is the one write that knows exactly what changed. Leaving it in the cache would leak a dead sprint into `boardrepo.OpenSprints`, which feeds the Backlog picker, the Epics tree, the New issue dialog and the importer's Sprint column, and an issue journaled into a dead id fails at Commit long after the user has forgotten.
  - Write an audit row. It is the only trace that will exist once Jira no longer has the sprint.
  - Test: a sprint the cache calls future but Jira calls active is refused, naming active; a future sprint with a pending row is refused and names Commit; a clean future sprint is deleted, leaves the cache, and leaves `OpenSprints`; deleting the board's last sprint leaves an empty list rather than a stale row.

- [ ] **Step 7: The one Go suite run.** Inside `core/`: `go build ./... && go test ./... -count=1`. Inside `tam/`: the same. Fix what fails, rerun at most twice, report. Commit as `feat(tam): a sprint can be made, changed and removed`.

---

### Task 3: the bound methods and the cached read

**Files:** create `tam/app_sprintmanage.go` and `app_sprintmanage_test.go`, create `tam/internal/boardrepo/sprintlist.go` and its test; regenerate `tam/frontend/wailsjs/**`.

**Interfaces:**
- Consumes: Task 2's three service methods.
- Produces: `boardrepo.SprintDetail{Sprint; Total, Done int; Points, DonePoints float64; Keys []string; MembershipCached bool; Truncated bool}` and `func (r *Repository) BoardSprintDetails(ctx context.Context, issues IssueSource, profileID string, boardID int) ([]SprintDetail, error)`; the bound methods `CreateSprint`, `EditSprint`, `DeleteSprint`, `ListBoardSprintDetails`.

- [ ] **Step 1: The read.** One board's sprints with their dates, their cached issue keys, and the same four progress numbers `issuerepo.EpicTree` computes per epic, counted with `backend.IsDone` so the definition stays in one place. Order active, then future by start date, then closed by start date descending.
  - A closed sprint has no cached membership by design, so `MembershipCached` is false and `Keys` is empty. The view must be able to tell that from a sprint that is genuinely empty, which is why this is a field rather than something inferred.
  - Cap the issue rows the way `issuerepo.EpicTree` caps at 5,000 and set `Truncated` when the cap bites. One future sprint holding two thousand issues would otherwise be materialised in full to draw a row that starts collapsed.
  - Run on the deferred read transaction the other board reads use, so the sprints and the issues come from one snapshot.
  - Test: the order, the counts, a closed sprint reporting membership uncached rather than empty, and the cap reporting truncation.

- [ ] **Step 2: The three write bindings.** Each goes through `requireProfile`, takes `a.acquire(p.ID, "sprint")` and releases it, and reduces a Jira sentence through `ceremonyError` the way `StartSprint` already does. Follow `app_sprints.go` exactly; put them in a new file rather than growing it. Remember that Wails discards a bound method's return value when the method also returns a non-nil error, so anything the dialog needs on a partial failure travels in the result, not in the error.

- [ ] **Step 3: The read binding.** `ListBoardSprintDetails` is a cache read: `requireProfile`, no guard, non-nil slice. Test the guards: no profile is an error, an unknown board is an empty list.

- [ ] **Step 4: Regenerate the bindings** and confirm the only Go visible additions are the four methods and the new struct. Commit as `feat(tam): the bindings a Sprints view needs`.

---

### Task 4: the view and its tree

**Files:** create `tam/frontend/src/components/SprintsView.tsx`, `SprintList.tsx`, `SprintRow.tsx`, `tam/frontend/src/lib/sprintGroups.ts`, `tam/frontend/src/lib/unfinished.ts`, and a test file for each; create `tam/frontend/src/queries/sprints.ts`; modify `queries/keys.ts`, `nav.ts`, `App.tsx`, `main.go`, `components/BoardCeremonies.tsx`, `StartSprintModal.tsx`, `App.css`.

- [ ] **Step 1: The shared pieces, before anything uses them.** `lib/unfinished.ts` takes issues and the board's columns and answers which are unfinished, by the rule Phase 3c set: a status id not in the last column. `BoardCeremonies`'s `incompleteCards` is rewritten to call it and its own copy deleted. `StartSprintModal`'s existing tests must still pass untouched: if one needs changing, the lift changed behaviour and that is a defect, not a test to update. `lib/sprintGroups.ts` groups a sprint's issues by assignee, with a count and a points total per person and an unassigned group last.

- [ ] **Step 2: The view exists.** `View` gains `"sprints"`, `VIEWS` gains the entry between `boards` and `reports`, `menuViews` in `main.go` gains it with accelerator `4`, and Reports becomes `5` and Rituals `6`. Nothing is conditional and nothing is filtered: the entry is always present, exactly as Reports and Rituals already are. `App.tsx` renders `SprintsView` for `current.id === "sprints"`.

- [ ] **Step 3: The queries.** `useBoardSprintDetails(profileId, boardId)` and the three mutations. The two ceremonies keep going through `runSprintCeremony`, which flips the whole app into its syncing state, because that is what they are. **Create, edit and delete take the Go lock but do not drive the global sync banner:** renaming a sprint is not a ceremony, and routing it through that path would make the New sprint button refuse during exactly the boards refresh a user runs to populate this view. Each mutation invalidates the board's sprints, the board, the profile wide open sprints, and the suggestion.

- [ ] **Step 4: The tree.** `SprintRow` is presentational, taking everything as props, mirroring `EpicRow`: name, state chip, dates, the goal, the progress line. Its actions live in a row menu rather than as three buttons on every row, following `CardMoveMenu`, which is the pattern TAM already uses for per row actions and which avoids putting extra tab stops inside a `treeitem`. `SprintList` owns the flattened rows, expansion, and roving tabindex keyboard navigation, mirroring `EpicTree`. An expanded sprint shows its issues grouped by assignee.

- [ ] **Step 5: The view's own shell.** `SprintsView` holds the board picker, the show closed toggle, the selection, and the detail panel, mirroring `EpicsView` including its render phase profile switch reset.
  - **The board picker is hidden when the profile has exactly one scrum board**, and the board is named in the heading instead. A select with one option is friction on every visit.
  - **A profile with no scrum board** gets an empty state saying sprints belong to a scrum board, that this project has none synced, and pointing at the Boards view. This is the case the plan used to hide the whole view for.
  - **A board with no sprints** says so and offers the create action.
  - **Loading** is a state, not an absence: the picker and the tree each say they are loading rather than rendering as empty. The previous branch shipped a defect that was exactly this.
  - A closed sprint's contents say they are not cached, with the reason.

- [ ] **Step 6: Tests.** `SprintsView.test.tsx`: the tree lists the board's sprints in order; a closed one says its contents are not cached; a profile with no scrum board gets the empty state and no crash; the picker is absent with one board and present with two. `SprintRow` and `SprintList`: which actions a state offers, keyboard navigation, and the assignee grouping. `lib/unfinished.ts` and `lib/sprintGroups.ts`: their own cases. Commit as `feat(tam): the Sprints view`.

---

### Task 5: the writes, from the dialogs and from a selection

**Files:** create `tam/frontend/src/components/NewSprintModal.tsx`, `EditSprintModal.tsx`, `SprintDraftForm.tsx`, `SprintFillBar.tsx`, and a test file for each; modify `SprintsView.tsx`, `SprintList.tsx`, `queries/sprints.ts`, `App.css`.

- [ ] **Step 1: The shared form.** `SprintDraftForm` is the four fields, their validation, and the suggestion seeding, lifted out of `StartSprintModal`, which then renders it. Name is required and trimmed; end must not precede start; a name longer than Jira's limit is refused locally rather than by a round trip.

- [ ] **Step 2: The two dialogs.** `NewSprintModal` and `EditSprintModal` each render `SprintDraftForm` with their own title, button and call. Both **disable on submit**, the way the create issue dialog already does with its `saving` flag: the per profile lock refuses a concurrent second call but not a second click after the first returns, and Jira allows two sprints with the same name. The edit dialog sends `clearGoal` when the user has emptied a goal that was not empty, and confirms before moving the end date of a running sprint.

- [ ] **Step 3: Delete.** A confirmation through `useConfirm`, naming how many issues the sprint holds and that Jira returns them to the backlog. It is the one action in TAM that cannot be undone and the confirmation says so in those words.

- [ ] **Step 4: The immediate write marker.** Five actions here reach Jira the moment they are pressed, in an app where everything else waits for Commit, and nothing on screen distinguishes them. Each of the five carries the same small marker and the same sentence in its confirmation or its dialog, saying this goes to Jira now rather than at Commit. One shared component, used five times, including by the two existing ceremony dialogs.

- [ ] **Step 5: Filling a sprint.** `SprintFillBar` is how several issues reach a sprint from here: a selection over the issues in the tree, a count, and one Move action, journaled through the `MoveManyToSprint` path the board's own selection already uses. It takes no guard, has a conflict story already reviewed in 3b, and needs no new case in Discard, the pending dialog, or the commit pass. Reuse `lib/boardSelection.ts` rather than writing a second selection model; if its shape does not fit a tree, say so in the report rather than forking it.

- [ ] **Step 6: Start and complete from here.** Both open the existing modals. The completion gets its unfinished list from `lib/unfinished.ts` over this view's own data rather than a drawn board.

- [ ] **Step 7: Tests, then the one frontend run.** Creating calls the binding with the board id; a second click while saving does nothing; emptying a goal sends the clear flag; moving a running sprint's end date confirms first; deleting a future sprint confirms and names the count; deleting an active one is not offered; the fill bar journals through the bulk move; the immediate write marker appears on all five. Then `npx vitest run` and `npx tsc --noEmit` in `tam/frontend`; fix what fails, rerun at most twice. Commit as `feat(tam): sprints are made, changed and filled from their own view`.

---

### Task 6: docs, the amendment, the single gate run

- [ ] **Step 1: The amendment.** Section 14.2 of `docs/superpowers/specs/2026-09-07-tam-boards-design.md` says the two ceremonies are the only place in TAM where a button talks to Jira without Commit. Amend it: name the three actions this plan adds, point at this design for the reasoning, note that `internal/sprints`'s `lifecycle` interface is now the enforced list, and keep the original argument intact rather than rewriting it, because it is still the argument.

- [ ] **Step 2: Docs.** `tam/CLAUDE.md` gains a section for the view: what it does, that create, edit and delete are online only and why, that the `lifecycle` interface is the list and a test asserts it, that membership stays journaled, that a closed sprint has no cached membership by design, and that delete removes rows from the cache itself rather than through `refreshSprints`, with the reason. Record the honest version of the rule: these writes are immediate because the sprint id has to be real before anything can point at it, not because a sprint is more of a Jira object than an issue is. Update the Status paragraph and the Layout tree.

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

- [ ] **Step 4: The walk-through for the user** (not run by agents): on the demo profile, create a sprint, edit its name and goal, clear the goal and confirm it stays cleared, start it, select three issues and move them in, Commit, complete it, then delete a future sprint and read the confirmation. Switch to a profile whose project has only a kanban board and confirm the view is present and explains itself. On a real Data Center, the three things Task 0's probe should already have answered, confirmed against the built app this time.

- [ ] **Step 5: Commit** as `docs(tam): the Sprints view`, then push and open the PR against `main`, noting that it stacks on the sprint linking PR.

## Deferred

Editing a closed sprint's dates. Deleting an active sprint. Creating a board. Carrying unfinished issues forward at create time, which is a completion behaviour and belongs with the completion dialog. A team roster with capacity per person, which is Phase 4 work and which the assignee grouping here is the seed of. Remembering a 403 for the session so a user without Manage Sprints is told once rather than per action.
