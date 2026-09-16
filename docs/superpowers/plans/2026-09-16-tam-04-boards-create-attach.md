# Boards: create and attach, local-first

**Spec:** `docs/superpowers/specs/2026-09-15-tam-04-boards-create-attach-design.md` (approved 2026-09-15, amended 2026-09-16 for local-first).
**Issue:** #49.
**Branch:** `feat/tam-bundle-04`, cut from `main` at `6a12aa8`.
**Method:** ponytail agents per task, scoped gates, one whole-branch review at PR time.

**Goal:** draft a board locally and create it with its filter on Commit; journal issues onto a board and push them on Commit; stop drawing every draft on every board.

---

## The rule this bundle is built on

**No feature writes to Jira directly.** Everything is journalled and pushed on
Commit. A draft board is the draft-sprint pattern one level up. This is why
there is no service layer, no new lock, no in-flight dialog state, and no
JQL validation call.

## Global constraints

- **G1. Test first (P2).** Red for the intended reason before implementing; CI's `proven-red` runs changed tests against pre-change code.
- **G2. Assertions must be able to fail (C6).** Assert values and user-visible outcomes.
- **G3. Files under 400 lines (C2).** `files_over_400` is at 107 and may not grow.
- **G4. Ratchet never grows (C7).**
- **G5. No em dashes in UI text.**
- **G6. Docs go to `agents/project/tam-phases.md`.** `tam/CLAUDE.md` is a stub since #42.
- **G7. Regenerate bindings** with `cd tam && wails generate module` when an exported Go struct or bound method crossing Wails changes. A new field not regenerated into `models.ts` is **silently dropped**.
- **G8. No AI trailers in commits.**
- **G9. One logical change per conventional commit (P5).**
- **G10. Scoped gates.** Run the packages and suites the task touches plus `npm run typecheck`; CI runs the full suites.

## Reuse ledger, read before writing anything

Every item below already exists. Copy the shape; do not invent a parallel one.

| Need | Reuse |
|---|---|
| Draft object on a negative id | `CreateDraftSprint` / `RekeySprint` / `sprint.draft` (schema 13), `issuerepo/sprintdrafts.go`, `rekeysprint.go` |
| Placeholder firewall | `committer/firewall.go` `assertNoPlaceholders` |
| POST returning a body | `Client.WriteJSONReturning`, template `CreateSprint` in `core/jira/sprintwrite.go:28` |
| Bulk write where 207 fails the batch | `Client.bulkWrite` in `core/jira/bulkwrite.go:26` |
| Journal row for a board-ish move | `recordMove` (`issuerepo/boardwrites.go:218`), `MoveValue` packing (`movevalue.go`) |
| A new commit phase | one entry in `committer/phases.go:42` |
| Optional backend capability | type assertion, as `boardWriter` at `committer/boards.go:125`; `noBoardWrites{}` stub at `:71` |
| A modal that writes | `CreateSprintModal.tsx` (97 lines) |
| Adding an issue to a sprint | `MoveToSprint`, already exists, already puts the issue on the board |

## The seven places a new journal entity must be registered

Missing any one of these is the classic failure here. Registered for both
`board_create` and `issue_board`:

1. `issuerepo/pending.go` — the constant, and `BoardEntities` if board reads should see it.
2. `committer/committer.go:161` `boardRow(...)`, **or** a new phase in `phases.go`. Missing both sends the row into `pushEdits`, which sends the field to Jira as an issue field and fails the issue's real edits with it.
3. `committer/committer.go:172` `heldBoardRow(...)`.
4. `issuerepo/discard.go:87` `discardOne` switch, or discard falls through to `writeField` on an `issue` row.
5. `issuerepo/issues.go:397` `PendingMoves` switch, if the drawn board should show it.
6. `boardvalues.go` `classifyMove` / `moveLabel` / `remoteValue`, if it is conflict-checked.
7. `tam/frontend/src/api.ts:606-639` — `ENTITY_*`, `MOVE_ENTITIES`, the label and field maps.

---

## Task 1: transport and the backend seam

**Files:** create `core/jira/boardwrite.go` + test; modify `tam/internal/backend/backend.go`, `tam/internal/backend/jira/boards.go` (or new), `tam/internal/backend/demo/boards.go`.

**Produces**
```go
// core/jira
func (c *Client) CreateFilter(ctx context.Context, name, jql, description, projectID string) (string, error)
func (c *Client) DeleteFilter(ctx context.Context, filterID string) error
func (c *Client) CreateBoard(ctx context.Context, name, boardType, filterID, projectKey string) (int, error)
func (c *Client) AddToBoardBacklog(ctx context.Context, boardID int, keys []string) error

// tam/internal/backend, an OPTIONAL interface, type-asserted like boardWriter
type BoardCreator interface {
    CreateBoard(ctx context.Context, d BoardDraft) (int, error)
    AddToBoardBacklog(ctx context.Context, boardID int, keys []string) error
}
```

**Steps**
1. Failing tests against `httptest`: filter create returns its id; board create returns its id; a board create that fails deletes the filter and reports both; `AddToBoardBacklog` batches and a 207 fails the whole batch.
2. Implement on `WriteJSONReturning` (copy `CreateSprint`'s shape) and `bulkWrite` (which already does 207). Assert a non-empty id, the way `CreateIssue` does.
3. Demo: mint board ids above the two fixed ones, honour the draft's type, refuse nothing a real Jira would accept. Do not fabricate state a real operation would not produce.
4. Gate: `cd core && go test ./jira/... -count=1`, `cd tam && go test ./internal/backend/... -count=1`.

**Do not** add these to `backend.BoardBackend`, whose doc says "It never writes". A separate optional interface keeps a backend that cannot write boards out of it.

---

## Task 2: draft boards

**Files:** `tam/internal/tamstore/tamstore.go`; `tam/internal/issuerepo/boarddrafts.go` (new) + test; `tam/internal/issuerepo/pending.go`; `tam/internal/committer/firewall.go`.

**Produces** `board.draft` (schema 14), `EntityBoardCreate = "board_create"`, `CreateDraftBoard`, `RekeyBoard`.

**Steps**
1. Migration test in this repo's required shape: seed the **old** schema, rewind the recorded version, reopen, assert the column exists. A fresh open runs every migration, so a test that lets the new schema build the column proves nothing. `board` has no `draft` column today; `sprint` got one at 13, so copy that entry.
2. Failing tests: `CreateDraftBoard` writes a `board` row with a negative id and `draft = 1`, plus one `board_create` journal row carrying name, type, filter name and JQL as JSON; the board appears in `ListBoards` labelled as a draft; `RekeyBoard` repoints `board`, `board_column`, `board_issue`, `sprint` and any journal rows naming the draft id, in one transaction.
3. Extend `assertNoPlaceholders` to reject **negative board ids**, not just the sprint case it guards today, and test that it does.
4. Gate: `cd tam && go test ./internal/tamstore/... ./internal/issuerepo/... ./internal/committer/... -count=1`.

**Watch:** `baseDDL` runs on every open, before migrations, so the column goes in **both**.

---

## Task 3: `issue_board`

**Files:** `tam/internal/issuerepo/` (the entity, `AddToBoard`, discard, `PendingMoves`), `tam/frontend/src/api.ts`.

**Produces** `EntityIssueBoard = "issue_board"`, field `boardId`, `after_val = boardId|scope` where scope is `backlog` or a sprint id; `AddToBoard(ctx, profileID, keys []string, boardID int, scope string) error`, one row per key per board, in one transaction.

**Steps**
1. Failing tests: adds one row per key; adding a key already on the board is a no-op rather than a duplicate row; discard removes the row and makes no Jira call; `PendingMoves` reports it so the board can draw it; Pending changes labels it in words ("Add to PLAT Checkout Board (backlog)").
2. Implement reusing `recordMove` and `MoveValue`; do not write a second packing.
3. Register in all seven places listed above, and say in the report which line each went on.
4. Gate: `cd tam && go test ./internal/issuerepo/... -count=1`; `cd tam/frontend && npx vitest run src/api.test.ts && npm run typecheck`.

---

## Task 4: the commit phases

**Files:** `tam/internal/committer/phases.go`, `boards.go`, `held.go` + tests.

**Steps**
1. **Write the hard test first**, because it is the one most likely to be wrong: a draft board with a draft sprint on it, committed together. The board phase runs first, the sprint is pushed with the **real** board id, and when the board create fails the sprint is held with a stated reason and no negative id reaches Jira.
2. Add one `boards` phase entry **before** `sprints` in `phases()`. Push order inside it: filter, board, rekey, then the `issue_board` adds.
3. Backlog scope uses `AddToBoardBacklog`; sprint scope uses the existing `MoveToSprint`. Drafts are held until the create phase gives them keys, the way board moves already hold.
4. Post-Commit filter check: for each board touched, read `/board/{id}/issue?jql=key in (...)&fields=key` and produce a result line for missing keys. **The journal row is still removed**: Jira accepted the write. A failure of the check itself must not fail the Commit.
5. Gate: `cd tam && go test ./internal/committer/... -count=1`.

---

## Task 5: drawing, and the two surfaces

**Files:** `tam/internal/boardrepo/view.go` + test; `tam/frontend/src/components/NewBoardModal.tsx`, the add-issues picker in `BoardsToolbar.tsx`, `BoardsView.tsx`, + tests.

**Steps**
1. Failing `boardrepo` test first: a draft with no tie to a board is **not** drawn on it; a draft with a pending `issue_board` row, a pending sprint move, or a sprint of that board **is**. Today `composeBoard` appends every draft to every board (`view.go:176-190`) and that is the behaviour being removed.
2. A pending add draws on its target board only, with a pending marker, in the first column collecting its status (drafts in the first column, as today).
3. `NewBoardModal`: name, type, filter name, JQL, all local. No lock, no in-flight state, no Escape refusal, no JQL validation call. Model the markup on `CreateSprintModal`.
4. The picker searches cached issues and drafts not already in the board's membership, multi-selects, and offers backlog or one of that board's active or future sprints. Keyboard reachable.
5. **Test traps here** (`agents/project/testing.md`): never `waitFor` a control to be enabled then pick an option, wait for the **option**; `getByText` finds content inside a collapsed `<details>`; `getByRole("definition")` never matches a `<dd>`.
6. Gate: `cd tam && go test ./internal/boardrepo/... -count=1`; `cd tam/frontend && npx vitest run src/components/BoardsView.test.tsx src/components/BoardsToolbar.test.tsx src/components/NewBoardModal.test.tsx && npm run typecheck`.

---

## Task 6: docs

**Files:** `agents/project/tam-phases.md`, `TODOS.md`.

1. A boards section covering: the local-first rule and why board create is journalled rather than immediate; draft boards and the rekey; the phase order and why the board phase precedes sprints; `issue_board` and its scope packing; the filter check and why the row is removed anyway.
2. **The draft-drawing behaviour change, stated as a release note**: drafts no longer appear on every board, because they were never on any board in Jira.
3. `TODOS.md`: the four pre-existing immediate writes in `internal/sprints` now sit against the local-first rule; record that converting them is its own piece of work and that Start and Complete have a real argument against it.

---

---

## Task 7: journal the sprint edit and delete

The last four immediate writes live in `internal/sprints`, fenced by
`exceptions_test.go` as `Start`, `Complete`, `Edit`, `Delete`. Create was
already journalled by bundle 01. These two are the straightforward half.

**Files:** `tam/internal/issuerepo/` (two entities), `tam/internal/committer/`, `tam/internal/sprints/`, `tam/app_sprintmanage.go`, the frontend dialogs.

**Produces** `sprint_edit` and `sprint_delete` journal entities, pushed in the
existing `sprints` phase beside `sprint_create`.

**Steps**
1. Failing tests: an edit to a draft sprint changes the draft in place and journals nothing extra; an edit to a real sprint journals one row and pushes on Commit; a delete of a draft removes it and its journal row with no Jira call; a delete of a real sprint journals and pushes; discard reverses both.
2. `clearGoal` already distinguishes "clear it" from "leave it" on `Edit`; carry that through the journal value rather than inventing a second convention.
3. Remove `Edit` and `Delete` from `sprints.Service` and from the fenced list. Register both entities in the seven places.
4. Gate: `cd tam && go test ./internal/issuerepo/... ./internal/committer/... ./internal/sprints/... . -count=1`.

---

## Task 8: journal the sprint ceremonies

**Start** is easy and was only ever grouped with Complete. The dialog already
asks for name, goal, start and end, so the dates are the user's, not a clock
reading. Journal `sprint_start` and push it.

**Complete is the one with a real design problem, and it needs a ruling before
code.** Today the dialog reads the sprint's contents from Jira, computes which
issues are unfinished by the board's last column, shows "47 cards are not
finished and will move out of the sprint", and the user confirms. Journalling
splits that: the preview is computed when the dialog opens, the move happens at
Commit, and the two can disagree if anything changed in between.

**Ruling: journal the intent, recompute at Commit, and word the dialog for it.**
The journal row carries the sprint and the destination for unfinished work. The
unfinished set is computed at push time from Jira, exactly as the current code
computes it at button time, so Jira's state at the moment of the write is what
decides, which is the only correct answer. The dialog says "about N cards",
names that it is recomputed on Commit, and the Commit result reports the real
number moved. What the dialog must not do is promise an exact count it cannot
keep.

**Steps**
1. Write the disagreement test first: preview says N, the sprint changes, Commit moves M, and the result reports M without failing.
2. `sprint_start` and `sprint_complete` entities; push in the `sprints` phase, after creates so a draft sprint can be started in the same Commit.
3. Refuse to start a sprint whose cached state is not `future`, as `guards.go` already does, and keep that refusal local.
4. Delete `internal/sprints/exceptions_test.go` and the fenced list rather than shortening it: an empty exception list is the point, and a fence guarding nothing is worse than none.
5. Gate as Task 7, plus `cd tam/frontend && npx vitest run src/components/StartSprintModal.test.tsx src/components/CompleteSprintModal.test.tsx`.

**Scope note:** tasks 7 and 8 roughly double this bundle. If the branch grows
unwieldy they split into their own PR against the same issue; the boards work
does not depend on them.

---

## Whole-bundle gate, before the PR

```
cd core && go test ./... -count=1
cd tam  && go test ./... -count=1
cd tam/frontend && npx vitest run <the suites this bundle touched> && npm run typecheck
sh scripts/lint-report.sh && sh scripts/ratchet.sh    # must say counts within baseline
cd tam && wails build
```

## Open items

1. **The probe** (issue #49): whether `POST /rest/agile/1.0/board` with `location` is accepted on the target Data Center version, which permission it needs, and whether sharing the filter with the project is right.
2. **Rebase on #48** once bundle 03 merges; both bundles add a section to `agents/project/tam-phases.md`.
