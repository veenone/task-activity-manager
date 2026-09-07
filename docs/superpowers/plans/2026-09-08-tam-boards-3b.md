# Task Activity Manager Phase 3b: the board write path

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan runs under the lean cycle: implementers build and write the tests named here but do not run suites, except one Go suite run at the end of Task 3 (the store, the commit pass, and the client all feed the view after it). Task 5 runs every gate once, then one fix wave.

**Goal:** the Boards view becomes writable. A card can be dragged across columns (a transition), reordered within a column (a rank), or moved to another sprint, and every one of those goes into the journal and reaches Jira on Commit.

**Succeeds when** a scrum master can run a standup's worth of moves on a train with no signal, and land all of them with one Commit when the laptop comes back online. That is the thing Jira's own board cannot do, and 3a built every part of it except the verb.

**Architecture:** `core/jira` gains the four write calls (transitions read, transition push, rank, sprint move). `tam/internal/issuerepo` gains three journal entity types and the three writes that produce them, plus the reverts. `tam/internal/boardrepo` applies the pending intents when it places a card, so a moved card is drawn where it was dropped. `tam/internal/committer` gains a board pass between edits and links. The Boards view gains native drag and drop, a keyboard move on the focused card, and loses the read-only caveat.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

**Spec:** [`../specs/2026-09-07-tam-boards-design.md`](../specs/2026-09-07-tam-boards-design.md), section 13.

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, `agile-suite/tam`; run Go commands from inside the module directory. Nothing under `xtm/` changes.
- Every board write goes through the journal and Commit. Nothing in this plan writes to Jira outside `internal/committer`.
- A transition is not a field edit. It is journaled by target status id and resolved to a transition id at push time, from the transitions Jira offers for that issue at that moment.
- A transition's own required fields are read with it (`?expand=transitions.fields`) and honoured. Most Data Center workflows put a Resolution screen on the transition into Done, and a push that sends only a transition id gets a 400 from every one of them. This is the single most likely way this feature fails on a real instance.
- A board write is checked against the issue's remote **status**, not its `updated` stamp. A comment bumps `updated` without moving the card, and a status that moved does not always bump it in a way that helps. Remote status equal to the journaled target means the move already happened, which is satisfaction, not conflict.
- Every board failure carries what the user can do next: the columns that were actually reachable, and a way to put the card back. "Commit again to retry" is the one thing that cannot help a transition with no path, and that line is already in the product.
- No fabricated LexoRank. A rank is journaled as a neighbour and a side; the cache's `rank` column is only ever written by a sync.
- One journal row per issue per entity: a second drag replaces the first, and a card dragged back to where it started deletes the row rather than journaling a move to itself.
- A held-back write reuses 1b's conflict card and its two resolutions. A rank is never held back: it has nothing to compare and nothing to rebase.
- Native HTML5 drag and drop. No new frontend dependency.
- Every move has a keyboard path on the focused card. The board shipped a keyboard model in 3a and must not lose it.
- Bound method signatures are exactly: `MoveIssueToColumn(profileID, key, statusID string) error`, `RankIssue(profileID, key, neighbourKey string, before bool) error`, `MoveIssueToSprint(profileID, key, sprintID string) error`.
- The PAT stays in the Jira client's Authorization header only.
- Files stay small and single purpose; a helper used from two places lives in its own module. TAM mirrors XTM's design language where XTM has a counterpart.
- UI text uses no em dashes. No AI attribution or mentions anywhere. Conventional commit prefixes, no trailers. Never add, commit, or delete untracked local tooling files; revert Wails churn under `tam/frontend/wailsjs/runtime`, `tam/frontend/package.json.md5`, and `tam/go.mod` with `git checkout --`.

## Decisions

1. **Three entity types, not one.** `issue_transition`, `issue_rank`, and `issue_sprint` are separate rows because they fail separately, are checked separately, and are pushed in a fixed order. One combined "move" row would make a failed transition drag its innocent rank down with it.
2. **The push order per issue is sprint, transition, rank.** A sprint move can change which columns apply, a transition changes the status a rank is measured against, and a rank is the one that is harmless to repeat.
3. **The backend resolves the transition id.** `core/jira` reports what Jira offers; `backend/jira` picks the transition whose `to.id` matches the journaled target and returns `ErrNoTransition` when none does. The store never holds a transition id, which would be stale the moment the issue moved.
4. **A rank stays out of the cache.** The board read orders a pending rank by its neighbour. Writing a made-up LexoRank into `issue.rank` would be a second source of truth that the next sync silently overwrites.
5. **A draft is dragged in place.** A `TAM-NEW-n` card has no Jira state, so a drag updates its draft JSON (status id, sprint) rather than journaling a transition against an issue that does not exist yet.
6. **The sprint move is an action, not a drop target.** There is nowhere on the board to drop a card that means "the next sprint", so it lives on the card's menu and on the detail panel. Moving several at once is what sprint planning actually does, and it arrives with 3c, where planning lives; 3b moves one card at a time.
7. **Ranks push last, all together, top to bottom.** Rank is the one write that is relative between issues: two cards ranked against each other have no meaning in isolation, and pushing them per issue can commit an order the screen never showed. The pass therefore re-derives each neighbour from the board's final local order at push time rather than trusting the neighbour that was journaled at drop time.
8. **A drop is verified against Jira when the app is online.** One `transitions` call for the card that was just dropped, in the background, and an inline warning with a way to put it back when the target is not reachable. This is what turns "your commit failed an hour later" into "that column is not reachable from To Do", and it costs one request per drag. Offline, the drop is accepted and the answer waits for Commit, which is the case the journal exists for.
9. **Pushing straight to Jira when online was rejected.** It would give a faster answer and it is what most tools do, but it forks TAM's model: every other write in the app is journaled and lands on Commit, and a board that pushed on drop would be the one surface where Discard means nothing and the pending count lies. Decision 8 buys most of the same benefit without the fork.

## File structure

**Created:** `core/jira/transitions.go`, `transitions_test.go`; `tam/internal/issuerepo/boardwrites.go`, `boardwrites_test.go`; `tam/internal/committer/boards.go`, `boards_test.go`; `tam/frontend/src/lib/cardMove.ts`; `tam/frontend/src/components/CardMoveMenu.tsx`.

**Modified:** `core/jira/agile.go`, `agile_test.go`; `tam/internal/backend/backend.go`; `tam/internal/backend/jira/boards.go`, `writes.go`, `jira_test.go`; `tam/internal/backend/demo/boards.go`, `demo.go`, `demo_test.go`; `tam/internal/issuerepo/pending.go` (the entity constants), `discard.go`, `issues.go` (the pending-intent read); `tam/internal/boardrepo/view.go`, `view_test.go`; `tam/internal/committer/committer.go`, `committer_test.go`; `tam/app_boards.go`, `app_boards_test.go`; `tam/frontend/wailsjs/**` (regenerated); `tam/frontend/src/api.ts`, `queries/boards.ts`, `queries/invalidate.ts`, `components/BoardCard.tsx`, `BoardGrid.tsx`, `BoardBody.tsx`, `BoardsView.tsx`, `BoardsToolbar.tsx`, `BoardsView.test.tsx`, `PendingChangesModal.tsx`, `PendingChangesModal.test.tsx`, `ActivityTab.tsx`, `App.css`; `frontend/core/styles/primitives.css`; `tam/CLAUDE.md`, `README.md`.

**New CSS**, all beside the 3a board rules: `.board-drop-line`, `.board-cell-over`, `.board-card-dragging`, `.board-card-checking`, `.board-card-warn`, `.board-card-failed`, `.board-card-conflict`, `.pending-dot-move`, and `.pending-row-move`. Check every token against `frontend/core/styles/tokens.css` before using it; there is no `--surface-1` in this design system, and the failure and conflict markers take the danger and warn palettes the conflict card already uses.

---

### Task 1: The write transport

**Files:** create `core/jira/transitions.go`, `core/jira/transitions_test.go`; modify `core/jira/agile.go`, `core/jira/agile_test.go`.

**Produces:** `jira.RawTransition{ID, Name string; To RawStatus; Fields map[string]RawTransitionField}` with `RawTransitionField{Required bool; AllowedValues []RawAllowedValue}` and `RawAllowedValue{ID, Name string}` (reusing 3a's `RawStatus`, which carries the id; add `Name` to it, which the configuration ignores and this needs); `(*Client).Transitions(ctx, key string) ([]RawTransition, error)`, which asks for `?expand=transitions.fields` so a caller can see what a transition demands before pushing it; `(*Client).DoTransition(ctx, key, transitionID string) error`; `(*Client).RankIssue(ctx, key, neighbourKey string, before bool) error`; `(*Client).MoveToSprint(ctx, sprintID string, keys []string) error`; `(*Client).MoveToBacklog(ctx, keys []string) error`.

- [ ] **Step 1: The tests.** Extend the httptest server in the style of `agile_test.go` (read it first) with `/rest/api/2/issue/PLAT-412/transitions`, `/rest/agile/1.0/issue/rank`, `/rest/agile/1.0/sprint/12/issue`, and `/rest/agile/1.0/backlog/issue`. Cover: the transitions list decoding id, name, `to.id`, and each transition's required fields with their allowed values (the fixture's Done transition requires `resolution` with two allowed values, which is what a Data Center workflow actually sends); a transition POST sending `{"transition":{"id":"31"}}` and nothing else; a rank PUT sending `{"issues":["PLAT-412"],"rankBeforeIssue":"PLAT-409"}` and the `rankAfterIssue` form; a sprint move POSTing `{"issues":["PLAT-412"]}` to the sprint path; a backlog move POSTing the same body to the backlog path; and a 404 on the rank path surfacing as an `*HTTPError` the caller can read. Assert on the request bodies, not only on the status: these calls are all side effect and a body that is silently wrong is the failure mode.

- [ ] **Step 2: The calls.** `transitions.go` holds `Transitions` and `DoTransition` over `/rest/api/2/issue/{key}/transitions` (the GET decodes `{"transitions":[...]}`), with the key path-escaped. `agile.go` gains `RankIssue`, `MoveToSprint`, and `MoveToBacklog` over the existing `WriteJSON` helper. Doc comments name the endpoint and say these are the only four calls in `core/jira` that change anything on the instance.

- [ ] **Step 3: Commit** as `feat(core): the Jira calls a board move needs`.

---

### Task 2: The journal, the reverts, and the board read

**Files:** create `tam/internal/issuerepo/boardwrites.go`, `boardwrites_test.go`; modify `tam/internal/issuerepo/pending.go`, `discard.go`, `issues.go`, `tam/internal/boardrepo/view.go`, `view_test.go`.

**Produces:** `issuerepo.EntityTransition = "issue_transition"`, `EntityRank = "issue_rank"`, `EntitySprintMove = "issue_sprint"`; `(*Repository).MoveToColumn(ctx, profileID, key, statusID string) error`, `RankIssue(ctx, profileID, key, neighbourKey string, before bool) error`, `MoveToSprint(ctx, profileID, key, sprintID string) error`; `(*Repository).PendingMoves(ctx, profileID string) ([]backend.PendingMove, error)`; `backend.PendingMove{Key, StatusID, SprintID, RankNeighbour string; RankBefore bool; HasTransition, HasSprint, HasRank bool}`; `boardrepo.IssueSource` grows `PendingMoves`.

- [ ] **Step 1: The entity constants and the writes.** `boardwrites.go` holds the three writes, each one transaction: read the issue's current value and `updated`, refuse a key the cache does not hold, journal with `before_val` set to the current value and `base_version` to `updated`, audit the change, and write the local column so the view repaints (status and `status_id` for a transition, `sprint_id` and `sprint_name` for a sprint move; a rank writes no column, by Decision 4).

`before_val` and `after_val` on a transition and a sprint move carry `id|Name`, the same shape the link row already uses for its three parts. The committer splits on the pipe and pushes the id; the Pending changes dialog and the Activity tab read the name. Without it a user reads "statusId: 3 to 5", which is a row only a developer can act on, and no dialog has the board's columns in hand to translate it.

Two rules every one of them shares, and both need their own test:
- A move back to the value the row already had deletes the journal row and audits the undo, rather than writing a change to nothing.
- A second move replaces the first, which the journal's unique key gives for free, but the `before_val` of the **first** row has to survive the replacement or a later discard puts the card in the wrong place.

A draft key (`DraftPrefix`) takes a different path: update the draft's JSON and its columns, journal nothing new.

- [ ] **Step 2: Discard.** `discardOne` learns the three: a transition revert restores `status` and `status_id` from `before_val`, a sprint revert restores `sprint_id` and `sprint_name`, and a rank revert has no column to restore, so dropping the row is the whole job. Test each, including a discard after two moves of the same card.

- [ ] **Step 3: `PendingMoves`.** In `issues.go`, one read returning every pending board intent for the profile, keyed by issue, so the board read can apply them in memory without a second query per card. It reads the three entity types in one statement and folds them into one `PendingMove` per key.

- [ ] **Step 4: The board read applies them.** In `boardrepo/view.go`, after the cards are fetched and before they are placed: a pending transition overrides the card's status id, a pending sprint move includes or excludes the card on a sprint-scoped view, and a pending rank places the card immediately before or after its neighbour in the cell it lands in. A neighbour that is not in that cell leaves the order alone. `IssueSource` grows `PendingMoves`, so `boardrepo` still never imports `issuerepo`.

- [ ] **Step 5: Tests.** `boardwrites_test.go` for the three writes, their replacement rule, their undo rule, the draft path, and the reverts. `view_test.go` for each intent: a card drawn in its target column, a card that appears in a sprint it was moved into, a card that disappears from the sprint it left, a rank placing a card before and after a neighbour, and a rank whose neighbour is elsewhere leaving the order untouched.

- [ ] **Step 6: Commit** as `feat(tam): journal a board move, and draw the card where it was dropped`.

---

### Task 3: The backend seam and the commit pass

**Files:** create `tam/internal/committer/boards.go`, `boards_test.go`; modify `tam/internal/backend/backend.go`, `backend/jira/boards.go`, `backend/jira/writes.go`, `backend/jira/jira_test.go`, `backend/demo/boards.go`, `backend/demo/demo.go`, `backend/demo/demo_test.go`, `tam/internal/committer/committer.go`, `committer_test.go`.

**Produces:** on `IssueBackend`: `Transition(ctx, key, targetStatusID string) error`; on `BoardBackend`: `RankIssue(ctx, key, neighbourKey string, before bool) error`, `MoveIssuesToSprint(ctx, sprintID string, keys []string) error`; `backend.ErrNoTransition` and `backend.TransitionCheck{Reachable []string; Allowed bool}`; `(*Engine).commitBoardMoves(...)` and the `Result` growing `Moved []Moved`.

`committer.Failure` also grows, and this is not optional: today it is `{Key, Error string}`, which cannot support either promise this plan makes. One card can fail a transition and drop a rank in the same Commit, so a per-failure Undo has no way to know which journal row to discard; and nothing distinguishes a transition with no path, which will fail identically forever, from a rank whose neighbour moved, which will not. It becomes `Failure{Key, EntityType string; RowID int64; Error string; Retryable bool; Reachable []string}`, and the dialog's "Commit again to retry the failures" line shows only when some failure is actually retryable.

- [ ] **Step 1: The backends.** The Jira backend's `Transition` lists the issue's transitions (with their fields), picks the one whose `To.ID` equals the target, and pushes it. Three answers, not one:
- No transition reaches the target: return `ErrNoTransition`, and carry the names of the statuses that **are** reachable, so the failure can tell the user where the card can actually go.
- The transition requires only a resolution: send it, taking the profile setting `transition_resolution` when it names one of the transition's allowed values, and the first allowed value otherwise. A Data Center workflow almost always puts a resolution screen on the way into Done, and refusing every such move would make this feature useless on most real instances. `CreateFields` in the same package is the precedent for reading what Jira demands before writing.
- The transition requires anything else: return an error naming the fields, because guessing a value for someone else's custom field is worse than saying it has to be done in Jira.

`ErrNoTransition` and the required-field error both name the issue and the target. `RankIssue` and `MoveIssuesToSprint` pass through, with an empty sprint id meaning the backlog. The demo backend applies all three to its in-memory dataset and refuses one transition on the curated story (`<project>-412`), so the offline walk-through can see a failure without a real Jira.

- [ ] **Step 2: The pass.** `committer/boards.go` runs after the edits pass and before links. Per issue, in the order sprint then transition; the ranks of every issue go last, in one group (Step 2b).

The check is against the issue's remote state, read fresh, not against `updated`:
- Remote status equals the journaled target: the move already happened, on the web or by someone else. Delete the row as satisfied and count it, do not raise a conflict over an outcome the user wanted.
- Remote status equals the journaled `before_val`: nothing moved under us, push it.
- Remote status is something else: hold the issue back as a conflict carrying before, target, and remote, in the card 1b built.

The same three answers apply to a sprint move against the issue's remote sprint. Push what is left, delete each row as it lands, and record a `Moved` per successful write. A failure is per write and per issue: it stays in the journal, is reported with its reason and with what the user can do next, and does not stop the pass. A row whose key is still a draft waits for the next Commit, the same rule the link pass already uses.

- [ ] **Step 2b: The rank group.** Ranks push after every transition and sprint move has landed, because both change where a card sits. They push in the board's final local order, top to bottom, each neighbour re-derived at push time from that order rather than from the key journaled at drop time: a neighbour that moved or left makes the journaled key meaningless, and pushing per issue in journal order can commit a sequence the screen never showed. A rank whose re-derived neighbour has gone from the cell is dropped with a reported reason rather than pushed against a card that is not there.

- [ ] **Step 3: Wire it in.** `Commit` calls the board pass between edits and links, and `Result.Remaining` counts the board rows that stayed.

- [ ] **Step 4: Tests.** `boards_test.go` with the existing committer fake extended: the writes land in order for one issue and the ranks land after all of them; a transition with no path fails, names the reachable statuses, and leaves the rank alone; a transition needing only a resolution sends one; a transition needing another field fails naming it; a remote status already at the target deletes the row as satisfied instead of conflicting; a remote status somewhere else holds the issue as a conflict; two cards ranked against each other commit in the board's order; a rank whose neighbour has left the cell is dropped with a reason; a draft's rows wait; and `Remaining` counts what stayed.

- [ ] **Step 5: The one Go suite run.** Inside `tam/`: `go build ./... && go vet ./... && go test ./... -count=1`, and inside `core/`: `go test ./jira/ -count=1`. Fix what fails, rerun at most twice, and report. Commit as `feat(tam): push board moves on Commit`.

---

### Task 4: Drag, drop, and the keyboard

**Files:** create `tam/frontend/src/lib/cardMove.ts`, `cardMove.test.ts`, `components/CardMoveMenu.tsx`; modify `tam/app_boards.go`, `app_boards_test.go`, `tam/frontend/wailsjs/**` (regenerated), `api.ts`, `queries/boards.ts`, `queries/invalidate.ts`, `components/BoardCard.tsx`, `BoardGrid.tsx`, `BoardBody.tsx`, `BoardsView.tsx`, `BoardsToolbar.tsx`, `BoardsView.test.tsx`, `PendingChangesModal.tsx`, `PendingChangesModal.test.tsx`, `ActivityTab.tsx`, `App.css`, `frontend/core/styles/primitives.css`.

**Produces:** the three bound methods from the Global Constraints; `useMoveToColumn`, `useRankIssue`, `useMoveToSprint`; `cardMove.ts` holding the drop-target arithmetic (which column, which neighbour, which side) as pure functions.

- [ ] **Step 1: The bindings.** `app_boards.go` gains the three methods, shaped exactly like `EditIssue` in `app_writes.go`: check the store, check the profile and key, call the repository, and return. **No busy guard.** `acquire` is used in exactly one place in this app, the Commit binding, and no local journal write takes it; giving the board moves one would refuse a drag while a commit runs, which no other write does. If journalling during a commit is a hazard it is a hazard for every write, and it belongs in its own change rather than being invented for one surface.

Add a fourth, `CanTransition(profileID, key, statusID string) (backend.TransitionCheck, error)`, which asks the backend what the card can reach right now. It is the only board binding that touches the network, it is best-effort, and an error from it means "we could not check", never "the move is illegal". Regenerate with `wails generate module`, check `App.d.ts`, and revert the runtime churn.

- [ ] **Step 2: `cardMove.ts`.** Pure functions, no React: `columnDrop(cards, clientY, rects)` returning the neighbour key and side for a drop inside a cell, and `isSameCell(from, to)` so a drop that changes nothing journals nothing. Its own module because both the drag handlers and the keyboard handlers use it, and because arithmetic with no DOM in it is the part worth testing directly.

- [ ] **Step 3: The card.** `BoardCard` becomes `draggable`, sets `dataTransfer` to its key on drag start, and drops the 3a refusal handler with the announcement that went with it. It keeps `aria-grabbed` off: the keyboard path is a menu, not a simulated drag.

- [ ] **Step 4: The cell, and what a drag looks like.** `BoardGrid`'s cells take `onDragOver`, `onDrop`, and the states a drag needs. A cell highlight alone is not enough: `columnDrop` computes a position *within* the cell from the cursor, and nothing on screen would say which gap the card is about to land in, so reordering would be a precision act performed blind.

- **The drop line.** During `onDragOver`, render `.board-drop-line` (a 2px `var(--accent)` rule) at the index `columnDrop` returns, driven by the same call the drop will make. The line is the feature; `.board-cell-over` is the backdrop.
- **Drop forbidden.** When the target is the card's own place (`isSameCell` and the index unchanged), set `e.dataTransfer.dropEffect = "none"` and withhold both the line and the highlight. Preventing default unconditionally makes the cursor promise a move and then do nothing, which reads as a bug rather than as a no-op.
- **The card in flight.** `.board-card-dragging` (half opacity, dashed border) from `onDragStart` to `onDragEnd`.
- **Empty cells.** `.board-cell` gets `flex: 1` inside its lane row so a column's whole height is a drop target. A 40px strip is not something a person can aim at once the last card leaves a column.
- **Off-screen columns.** `.board-scroll` scrolls horizontally and native drag and drop does not auto-scroll a custom container, so a seven-column board would have columns a mouse cannot reach. `onDragOver` scrolls the container when the pointer is within 48px of either edge.
- **Capped cells.** A cell that hit `MaxCardsPerCell` is showing "+N more", so a drop below the last visible card cannot know its real neighbour. That drop journals nothing and announces why.

- [ ] **Step 5: The keyboard and the menu.** Three things about 3a's keyboard model that the first draft of this plan got wrong; read `BoardsView.tsx`'s `onKeyDown` and `lib/boardCells.ts` before writing any of it.

- **Enter is taken.** Enter and Space both select a card and open the detail panel. The move menu opens from a trigger in `board-card-head` (`aria-haspopup="menu"`, reusing `Menu` from `@agile-suite/core` rather than inventing a panel), reachable with the menu key or Shift and F10, and the card must not swallow that trigger's own Enter.
- **A move must not reuse `nextColumn`.** That helper deliberately skips columns whose cell is empty, so focus never strands in one. Reusing it for a move would teleport a card past the empty column the user was aiming at, which is exactly where a card usually goes. `cardMove.ts` gets its own stepper that stops at the adjacent column, empty or not, and refuses to step past the last.
- **Focus follows the slot, not the card.** `posId` is `lane-col-index`: after a move the focused id still names the slot, which now holds a different card, so a second Ctrl and Right would move that one instead. Track the moved key and, once the board query settles, set focus to wherever that key landed. `EpicTree.tsx` already does this with its moved-row flash, and the same flash marks the moved card.

With that settled: Ctrl and Left or Right moves a card a column, Ctrl and Up or Down moves it within its cell, and each one announces itself. Write the strings, do not leave them to the implementer: "PLAT-412 moved to In Progress, 3 of 7", "PLAT-412 moved up, 2 of 5", "PLAT-412 is already in the first column". A column head that recounts silently tells a screen reader nothing. The grid also carries an `aria-describedby` naming the move keys and the menu, since nothing else on screen says they exist.

- [ ] **Step 6: The view, and every state a move can be in.** Remove the read-only caveat line and the "Read only" chip, and put nothing in their place. Keep the honesty line, which is about cards the board cannot show, not about writing. A moved card wears its marker the moment its mutation settles, and the column heads recount.

The card is where the user made the move, so the card is where its state has to show. `BoardsView` already holds `lastCommit` through `useSync()`; pass a per-key status down through `BoardBody` and `BoardGrid` to `BoardCard`:

| State | On the card |
|---|---|
| Pending move | `.pending-dot-move`, labelled "Pending move", not the plain pending dot: a moved card's **position** is what is provisional, and a card carrying a pending summary edit is not making that claim |
| Verifying | `.board-card-checking`, a quiet marker while `CanTransition` is in flight, so a card that is being checked does not look settled |
| Warned | `.board-card-warn` plus the banner below |
| Failed | `.board-card-failed`, with the reason in the card's `aria-label` |
| Conflicted | `.board-card-conflict`, in the warn palette the conflict card already uses |

Today a card that will never land looks exactly like one that is about to: both wear the same amber pending dot. That is the single worst outcome this phase can ship, because the board is the surface the user will look at, and the failure only exists in a modal they may never open.

**The verification warning, fully located.** `BoardsView` keeps `warnings: Map<key, string[]>`. A `CanTransition` answer is written only if it is the newest request for that key (a standup is ten drags and the answers do not return in order), it is cleared when that key is moved again or its row is discarded, and it renders in two places: the marker on the card, and one `pending-banner pending-banner-warn` above the grid naming the reachable statuses with a "Put it back" button that discards the row. One banner, not a stack.

**A failed move offers a way out.** In the commit result, a board failure gets an "Undo this move" action that discards exactly that journal row, which is what `Failure.RowID` is for. The existing "Commit again to retry the failures" line shows only when some failure is retryable, and a transition with no path is not.

**During a commit.** This plan deliberately gives the move bindings no busy guard, matching every other local write, so a card can be dragged while the committer is pushing. `BoardCard` renders `draggable={false}` while a commit is in flight, which is the honest way to say "not now" without inventing a guard no other write has.

A Commit that moved cards ends with a boards sync, so the board's own membership catches up with what was just pushed; without it a card can jump back to where the last sync saw it.

- [ ] **Step 7: The two dialogs that already show pending work.** `PendingChangesModal` renders a row through `fieldLabel`, which falls back to the raw field name, and `ActivityTab.describe` branches on entity type for links and creates and then does the same. Neither knows a board move, so today a journaled transition reads "statusId: 3 to 5". Give both the three entity types: the pending row reads "Status: To Do to In Progress", "Sprint: Sprint 12 to Sprint 13", and "Rank: before PLAT-409", each with its own Discard and its own `.pending-row-move` grid template beside the `.pending-row-link` one; the activity entry reads as a sentence the way a link's does. The `id|Name` encoding from Task 2 is what makes this possible without either dialog knowing the board.

Two lines in that dialog also go stale the moment this ships. `bannerLine` enumerates what a Commit did (committed, created, linked, conflicts, failures) and would report "nothing pushed" after a standup of twelve drags, so it gains the moves. And the footer's "Edits are pushed with Jira's own field update; a conflict holds only that issue back" is now wrong for three of the six entity types, so it says what is true of each group or moves into the edits group where it still holds.

- [ ] **Step 8: Tests.** `BoardsView.test.tsx`: a drop on another column calls `MoveIssueToColumn` and repaints the card there; a drop inside a cell calls `RankIssue` with the neighbour and side; a drop that changes nothing calls neither; Ctrl and an arrow does what the drop does; the menu moves a card to a sprint; a pending card wears the dot; and the caveat is gone; a drop whose target is unreachable shows the warning and its put-it-back button; a board failure in the commit result offers Undo; and the pending dialog and the activity tab read a move in words rather than in ids. `cardMove.test.ts` for the arithmetic, including the top and bottom edges of a cell.

- [ ] **Step 9: Commit** as `feat(tam): move a card by drag or by keyboard`.

---

### Task 5: Docs, the single gate run, and the fix wave

- [ ] **Step 1: Docs.** `tam/CLAUDE.md` gains a "Phase 3b: board writes" section: the three entity types and what each one journals, why a transition is resolved at push time and a rank is stored as a neighbour, the push order and why, and the fact that the board's keyboard moves are the accessible path rather than a simulated drag. Update its Status section and the Layout list. `README.md` gains one sentence. Reconcile the spec if the implementation moved: section 13 is the contract, and where the code disagrees, whichever is right gets corrected, with a line in the report saying which.

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

Record each result, fix what fails, rerun only what failed, and report each failure and its fix. Do not lower an assertion to make a test pass.

- [ ] **Step 3: The walk-through for the user** (not run by agents): on the demo profile, drag a card from To Do to In Progress, reorder two cards in a column, move a card to another sprint from its menu, check the pending count, then Commit and read the result. Do the same with the keyboard alone. Then on a real Jira DC: a card whose workflow has no path to the target column, to see the failure read properly, and a card someone else moved first, to see the conflict.

- [ ] **Step 4: Commit** as `docs(tam): Phase 3b notes for the board write path`, then push and open the PR against `main` titled "Task Activity Manager Phase 3b: the board write path" with the tasks, the gates, and the walk-through. No AI attribution anywhere.

## Deferred

Sprint start and complete, and moving several cards to a sprint at once, both in **3c**, which follows this plan directly and lands before Phase 4 rather than at some unnamed later date: sprint planning is where multi-select earns its keep, and it is the same surface that starts and completes a sprint. Also deferred: the board's own quick filters and swimlane rules, dragging between boards, and dragging an epic.

Not deferred any more: warning about an impossible drop before Commit. It was deferred in the first draft of this plan as a transitions cache, which would have cost a request per card on every sync; Decision 8 gets the same answer from one request per drop, and only when the app is online.
