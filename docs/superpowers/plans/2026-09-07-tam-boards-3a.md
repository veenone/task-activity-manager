# Task Activity Manager Phase 3a: boards and sprints, the read path

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan runs under the lean cycle: implementers build and write the tests named here but do not run suites, except one Go suite run at the end of Task 2 (the store and the client both feed everything after it). Task 5 runs every gate once, then one fix wave.

**Goal:** a Boards view that shows a project's Jira boards, the columns each board defines, the cards in them from the local issue cache, and the sprint picker for a scrum board, all offline-capable and read-only.

**Architecture:** `core/jira/agile.go` is transport for `/rest/agile/1.0` (boards, configuration, sprints, board issue keys), returning raw shapes. A new `tam/internal/boardrepo` owns three new tables (`board`, `board_column`, `sprint`) at schema version 4 and composes the board view by joining the columns' status ids against the existing issue cache. The syncer gains a boards pass after its issues pass. The frontend adds a `BoardsView` whose toolbar mirrors XTM's `board-head` and whose columns, cells, and cards are new but built from the existing card and chip vocabulary.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

**Spec:** [`../specs/2026-09-07-tam-boards-design.md`](../specs/2026-09-07-tam-boards-design.md). **Mockup:** [`../specs/assets/2026-09-07-tam-boards.svg`](../specs/assets/2026-09-07-tam-boards.svg).

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, `agile-suite/tam`; run Go commands from inside the module directory. Nothing under `xtm/` changes. `frontend/core` changes only by adding XTM's `board-head`, `board-picker`, `board-head-actions`, `board-counts`, and `board-scroll` rules to `styles/primitives.css` verbatim, plus the new board rules this plan names.
- Nothing in 3a writes to Jira. The board is read-only; no drag, no transition, no rank, no sprint start or complete. `IssueBackend` grows read methods only.
- The agile API is `/rest/agile/1.0`. A kanban board's sprint call answering 400 means no sprints, not an error. A board answering 403 is dropped from the sync with a line in the summary. A board whose configuration cannot be read is stored with no columns.
- Cards come from the issue cache only; the board issue keys narrow which cached issues appear, they never introduce an issue TAM has not synced.
- Bound method signatures are exactly: `ListBoards(profileID string) ([]boardrepo.Board, error)`, `ListBoardSprints(profileID string, boardID int) ([]boardrepo.Sprint, error)`, `GetBoard(profileID string, boardID int, sprintID, swimlane string) (boardrepo.BoardView, error)`, `SyncBoards(profileID string) (syncer.BoardSummary, error)`.
- `SyncBoards` runs under the app's `busy` guard as `"sync"`, so it and a full sync and a commit exclude each other.
- The PAT stays in the Jira client's Authorization header only.
- Files stay small and single-purpose; a helper used from two places lives in its own module. TAM mirrors XTM's design language where XTM has a counterpart.
- UI text uses no em dashes. No AI attribution or mentions anywhere. Conventional commit prefixes, no trailers. Never add, commit, or delete untracked local tooling files; revert Wails line-ending churn under `tam/frontend/wailsjs/runtime`, `tam/frontend/package.json.md5`, and `tam/go.mod` with `git checkout --`.

## Decisions

1. **`boardrepo` is its own package, not a file in `issuerepo`.** Boards are a separate concern with their own tables, and `issuerepo` is already the repo's largest package. `boardrepo` takes the same `*sql.DB` and is constructed beside `issuerepo` in `app.go`.
2. **The board view is composed in Go, not SQL.** `boardrepo.Board` reads the columns, asks `issuerepo` for the sprint's or board's issues, and buckets them in memory. Bucketing a few hundred cards is trivial and keeps the SQL readable.
3. **Board issue keys are stored, not fetched per view.** The sync writes each board's issue keys into `board_issue(profile_id, board_id, sprint_id, key)` so the view never calls Jira. A kanban board stores its keys under sprint id `""`.
4. **The swimlane value is a plain string** (`"none"`, `"assignee"`, `"epic"`), validated in `boardrepo`, so the binding stays simple and the frontend has no enum to keep in step.
5. **Cards carry no status chip.** The column is the status; repeating it on the card is noise. The card shows key, type, summary, assignee, points, and the pending dot, as the mockup does.
6. **The demo's boards are generated from the dataset**, not hardcoded rows, so a rekeyed project (`ACME-412`) works the way every other demo path does.

## File structure

**Created:** `core/jira/agile.go`, `agile_test.go`; `tam/internal/boardrepo/boardrepo.go`, `boards.go`, `view.go`, `boardrepo_test.go`, `view_test.go`; `tam/internal/syncer/boards.go`, `boards_test.go`; `tam/frontend/src/queries/boards.ts`, `components/BoardsView.tsx`, `BoardCard.tsx`, `BoardsView.test.tsx`.

**Modified:** `tam/internal/tamstore/tamstore.go`, `tamstore_test.go`; `tam/internal/backend/backend.go`; `tam/internal/backend/jira/boards.go` (new file in the existing package), `jira_test.go`; `tam/internal/backend/demo/demo.go`, `demo_test.go`; `tam/internal/issuerepo/state.go` (purge), `issues.go` (a keyed read); `tam/internal/syncer/syncer.go`, `syncer_test.go`; `tam/app.go`, `app_boards.go` (new); `tam/frontend/wailsjs/**` (regenerated); `frontend/core/styles/primitives.css`; `tam/frontend/src/api.ts`, `queries/keys.ts`, `queries/invalidate.ts`, `App.tsx`, `App.test.tsx`, `App.css`; `tam/CLAUDE.md`, `README.md`.

---

### Task 1: The agile client

**Files:** create `core/jira/agile.go`, `core/jira/agile_test.go`.

**Produces:** `jira.RawBoard{ID int, Name, Type string}`, `jira.RawColumn{Name string, StatusIDs []string}`, `jira.RawBoardConfig{Columns []RawColumn}`, `jira.RawSprint{ID int, Name, State, StartDate, EndDate string}`; `(*Client).Boards(ctx, projectKey string) ([]RawBoard, error)`, `(*Client).BoardConfiguration(ctx, boardID int) (RawBoardConfig, error)`, `(*Client).Sprints(ctx, boardID int) ([]RawSprint, error)`, `(*Client).BoardIssueKeys(ctx, boardID int, sprintID string) ([]string, error)`; `jira.ErrNoSprints`.

- [ ] **Step 1: The tests.** Create `core/jira/agile_test.go` with an httptest server that answers `/rest/agile/1.0/board`, `/board/1/configuration`, `/board/1/sprint`, `/board/2/sprint` (400 with Jira's kanban message), `/board/1/issue`, and `/board/1/sprint/12/issue`. Cover: boards paged over two pages with `isLast` false then true, and the second page's values appended in order; a board list that includes a type TAM does not know (`simple`) coming back unchanged (the caller filters); the configuration mapping each column to its status ids in order; sprints paged; a kanban board's 400 returning `ErrNoSprints` and no error text leaking Jira's HTML; issue keys asking for `fields=key` and returning the keys of both pages; a 403 on a configuration surfacing as an error the caller can see (the sync decides what to do with it).

```go
func TestBoardsPageUntilIsLast(t *testing.T) { ... }
func TestBoardConfigurationMapsColumnsToStatusIDs(t *testing.T) { ... }
func TestSprintsPageAndAKanbanBoardHasNone(t *testing.T) { ... }
func TestBoardIssueKeysAskForKeysOnly(t *testing.T) { ... }
```

Write them fully in the style of `core/jira/issues_test.go` (read it first; it builds a client with `NewClientWithHTTP(srv.URL, "tok", srv.Client())`).

- [ ] **Step 2: `agile.go`.** Implement the four calls over the existing `Client.Get`. One unexported helper does the paging:

```go
// agilePage is the envelope every paged Agile endpoint returns.
type agilePage[T any] struct {
	Values     []T  `json:"values"`
	IsLast     bool `json:"isLast"`
	MaxResults int  `json:"maxResults"`
	StartAt    int  `json:"startAt"`
}

// pageAgile walks an Agile 1.0 collection to its end. Jira reports the end
// with isLast; a short page ends it too, for instances that omit the flag.
func pageAgile[T any](ctx context.Context, c *Client, path string, query url.Values) ([]T, error) {
	const size = 50
	out := []T{}
	for start := 0; ; {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		q.Set("startAt", strconv.Itoa(start))
		q.Set("maxResults", strconv.Itoa(size))
		var page agilePage[T]
		if err := c.Get(ctx, path+"?"+q.Encode(), &page); err != nil {
			return nil, err
		}
		out = append(out, page.Values...)
		if page.IsLast || len(page.Values) == 0 || len(page.Values) < size {
			return out, nil
		}
		start += len(page.Values)
	}
}
```

`Boards` calls it with `projectKeyOrId`; `Sprints` calls it and maps a 400 to `ErrNoSprints` by checking `errors.As` for the client's `*HTTPError` with `Code == 400`; `BoardIssueKeys` pages `{"fields": {"key"}}` over the right path and pulls `key` from each value; `BoardConfiguration` is a single `Get` decoding only the column config. Doc comments say which endpoint each one calls and that they are transport only.

- [ ] **Step 3: Commit** as `feat(core): the Jira Agile client for boards, sprints, and board issues`.

---

### Task 2: The store and the backend seam

**Files:** create `tam/internal/boardrepo/{boardrepo.go,boards.go,view.go,boardrepo_test.go,view_test.go}`; modify `tam/internal/tamstore/tamstore.go` and its test, `tam/internal/backend/backend.go`, create `tam/internal/backend/jira/boards.go`, modify `tam/internal/backend/jira/jira_test.go`, `tam/internal/backend/demo/demo.go` and its test, `tam/internal/issuerepo/state.go`, `tam/internal/issuerepo/issues.go`.

**Produces:** schema version 4 with `board`, `board_column`, `board_issue`, `sprint`; `backend.Board{ID int, Name, Type string}`, `backend.BoardColumn{Name string, StatusIDs []string}`, `backend.Sprint{ID int, BoardID int, Name, State, StartDate, EndDate string}`; on `IssueBackend`: `Boards(ctx, projectKey string) ([]Board, error)`, `BoardColumns(ctx, boardID int) ([]BoardColumn, error)`, `BoardSprints(ctx, boardID int) ([]Sprint, error)`, `BoardIssueKeys(ctx, boardID int, sprintID string) ([]string, error)`; `boardrepo.New(db *sql.DB) *Repository` with `UpsertBoards`, `UpsertColumns`, `UpsertSprints`, `UpsertIssueKeys`, `RemoveBoards`, `ListBoards`, `Columns`, `ListSprints`, `PurgeProfile`; `boardrepo.Board`, `Sprint`, `BoardView`, `ColumnView`, `LaneView`; `boardrepo.Board(ctx, ...)` the view read; `issuerepo.IssuesByKeys(ctx, profileID string, keys []string) ([]backend.Issue, error)`.

- [ ] **Step 1: Schema.** In `tam/internal/tamstore/tamstore.go`, bump `Version` to 4 and append to `baseDDL`:

```sql
CREATE TABLE IF NOT EXISTS board (
	profile_id TEXT NOT NULL,
	id         INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	type       TEXT NOT NULL DEFAULT '',
	synced_at  TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, id)
);
CREATE TABLE IF NOT EXISTS board_column (
	profile_id TEXT NOT NULL,
	board_id   INTEGER NOT NULL,
	position   INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	status_ids TEXT NOT NULL DEFAULT '[]',
	PRIMARY KEY (profile_id, board_id, position)
);
CREATE TABLE IF NOT EXISTS board_issue (
	profile_id TEXT NOT NULL,
	board_id   INTEGER NOT NULL,
	sprint_id  TEXT NOT NULL DEFAULT '',
	key        TEXT NOT NULL,
	position   INTEGER NOT NULL,
	PRIMARY KEY (profile_id, board_id, sprint_id, key)
);
CREATE TABLE IF NOT EXISTS sprint (
	profile_id TEXT NOT NULL,
	id         INTEGER NOT NULL,
	board_id   INTEGER NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	state      TEXT NOT NULL DEFAULT '',
	start_date TEXT NOT NULL DEFAULT '',
	end_date   TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, id)
);
```

Add to `indexDDL`: `CREATE INDEX IF NOT EXISTS board_issue_lookup ON board_issue (profile_id, board_id, sprint_id);`. Extend the store test the way the version 3 test does: open, drop the four tables, rewind `schema_version` to 3, reopen, and assert all four exist and the version reads 4.

- [ ] **Step 2: The backend seam.** In `tam/internal/backend/backend.go` add the three types (JSON tags `id`, `name`, `type`, `statusIds`, `boardId`, `state`, `startDate`, `endDate`) and the four interface methods with doc comments saying they read Jira's Agile API and never write. Add the four stubs returning `errors.New("not used")` to both fakes in `tam/internal/syncer/syncer_test.go` and to the fake in `tam/internal/committer/committer_test.go`, so the packages still compile.

- [ ] **Step 3: The Jira implementation.** Create `tam/internal/backend/jira/boards.go` mapping the client's raw shapes to the DTOs: `Boards` filters to `scrum` and `kanban` and drops anything else with a log line; `BoardColumns` maps the configuration; `BoardSprints` returns an empty slice when the client reports `ErrNoSprints`; `BoardIssueKeys` passes through. Test in `jira_test.go` by extending the fake server with the agile paths (a board list of one scrum, one kanban, one `simple`; a configuration; sprints; issue keys) and asserting the filtering, the kanban's empty sprints, and the key pass-through.

- [ ] **Step 4: The demo implementation.** In `tam/internal/backend/demo/demo.go`, derive the boards from the dataset so a rekeyed project works: board 1 `"<project> Scrum"` type `scrum` with columns To Do (status ids `1`), In Progress (`3`), Done (`5`); board 2 `"<project> Kanban"` type `kanban` with the same three columns; sprints 11 (closed), 12 (active), 13 (future) on board 1 only; `BoardIssueKeys(1, "12")` returns the dataset's issues whose `SprintID` is `12`, `BoardIssueKeys(1, "")` returns every non-requirement issue, `BoardIssueKeys(2, "")` the same. The demo's issue statuses are names, not ids, so the demo also maps its status names to those ids in one small exported helper `demo.StatusID(name string) string` used by both the column definition and the tests. Extend `demo_test.go` for each.

- [ ] **Step 5: `boardrepo`.** `boardrepo.go` holds `New`, the row types, and the upserts and reads (each a short method, all SQL from constants, every write in one transaction per call, `RemoveBoards` deleting a board's columns, issue keys, and sprints with it). `view.go` holds the composition:

```go
// Board composes what the Boards view draws: the columns in order, the
// cards bucketed into them, and the lanes the swimlane asked for. Cards
// come from the issue cache; a status no column covers is counted in
// Unmapped so nothing disappears silently.
func (r *Repository) Board(ctx context.Context, issues IssueSource, profileID string, boardID int, sprintID, swimlane string) (BoardView, error)
```

`IssueSource` is a one-method interface (`IssuesByKeys(ctx, profileID string, keys []string) ([]backend.Issue, error)`) so `boardrepo` does not import `issuerepo`; `app.go` passes the repository. Swimlane values are `none`, `assignee`, `epic`; anything else is an error naming the three. Lanes keep the card order the `board_issue` positions give, and lanes themselves sort by label with the empty value last under "Unassigned" or "No epic". `ColumnView` carries `Total` and `Points` summed over the lane cells of that column.

- [ ] **Step 6: `IssuesByKeys`.** In `tam/internal/issuerepo/issues.go`, add a read that returns the cached issues for a key list in the list's own order, chunked at 500 keys per statement, with the same `pendingFlag` and `scanIssue` the other reads use. Test it in `issues_test.go` for order, a missing key (skipped), an empty list (empty result, no query), and a chunk boundary.

- [ ] **Step 7: Purge.** Add `board`, `board_column`, `board_issue`, and `sprint` to `issuerepo.PurgeProfile`'s table list (it already runs in one transaction) so deleting a profile clears them, and extend its test.

- [ ] **Step 8: Tests for the repository and the view.** `boardrepo_test.go`: upsert and read back boards, columns in position order, sprints ordered active, future, closed, then start date; `RemoveBoards` taking the children; profile isolation. `view_test.go` (with a small in-test `IssueSource`): cards land in the right columns by status id; an issue whose status is in no column counts in `Unmapped` and appears nowhere else; the three swimlanes, including the empty-value lane last; column totals and point sums; an unknown swimlane errors; a board with no columns returns an empty view rather than an error.

- [ ] **Step 9: The one Go suite run.** Inside `tam/`: `go build ./... && go vet ./... && go test ./... -count=1`, and inside `core/`: `go test ./jira/ -count=1`. Fix what fails, rerun at most twice, and report. Commit as `feat(tam): the board store, the board view, and the backend seam`.

---

### Task 3: The sync pass and the bindings

**Files:** create `tam/internal/syncer/boards.go`, `boards_test.go`; modify `tam/internal/syncer/syncer.go`, `syncer_test.go`, `tam/app.go`, create `tam/app_boards.go`; regenerate `tam/frontend/wailsjs/**`.

**Produces:** `syncer.BoardSummary{Boards, Columns, Sprints, Cards int; Dropped []string; Elapsed string}`, `(*Engine).SyncBoards(ctx, profileID, projectKey string, onProgress func(Progress)) (BoardSummary, error)`; the four bound methods from the Global Constraints.

- [ ] **Step 1: The pass.** `boards.go` holds `SyncBoards`: list the boards; for each, read the configuration, the sprints, and the issue keys (for a scrum board, the keys of every sprint plus the board's own list; for a kanban board, the board's list); upsert each part; then `RemoveBoards` for the boards the profile had that Jira no longer returns. A board whose configuration or keys fail is recorded in `Dropped` with its name and the reason, and the pass continues; the summary counts what landed. Progress frames use phase `"boards"` with the board name as `Stage`. The engine gains a `boards` field (the `boardrepo` repository) set by a new constructor `NewWithBoards(b backend.IssueBackend, repo *issuerepo.Repository, boards *boardrepo.Repository) *Engine`; the existing `New` stays for the issue-only tests.

- [ ] **Step 2: Wire it into `Sync`.** At the end of `Sync`, after the sync state is written and before the done frame, call `SyncBoards` when the engine has a boards repository; its failure does not fail the issue sync (log it, put it in the summary's new `Boards *BoardSummary` field, which is nil when the pass did not run).

- [ ] **Step 3: Tests.** `boards_test.go` with the existing test fake extended: a scrum and a kanban board land with their columns, sprints, and keys; a board Jira stops returning is removed; a configuration failure drops one board and keeps the other; the summary counts. Extend `syncer_test.go` for the composed run (issues then boards) and for a boards failure leaving the issue sync successful.

- [ ] **Step 4: The app.** `tam/app.go` constructs `boardrepo.New(...)` beside the issue repository in `initStore` and holds it as `a.boards`. Create `tam/app_boards.go` with the four bound methods: `ListBoards`, `ListBoardSprints`, `GetBoard` (passing `a.repo` as the `IssueSource`), and `SyncBoards` (under `a.acquire(p.ID, "sync")`, emitting progress, logging the summary). Each returns non-nil slices.

- [ ] **Step 5: Generate.** `go build ./... && go vet ./...` inside `tam/`, then `wails generate module`; check `App.d.ts` for the four methods and `models.ts` for the `boardrepo` namespace with `Board`, `Sprint`, `BoardView`, `ColumnView`, `LaneView`; revert the runtime and md5 churn and keep `go.mod` unchanged. Commit as `feat(tam): sync boards and sprints, bind the board reads`.

---

### Task 4: The Boards view

**Files:** modify `frontend/core/styles/primitives.css`; create `tam/frontend/src/queries/boards.ts`, `components/BoardCard.tsx`, `components/BoardsView.tsx`, `components/BoardsView.test.tsx`; modify `tam/frontend/src/api.ts`, `queries/keys.ts`, `queries/invalidate.ts`, `App.tsx`, `App.test.tsx`, `App.css`.

**Produces:** `api.ts` types `Board`, `Sprint`, `ColumnView`, `LaneView`, `BoardView`, the four bindings; `keys.boards(profileId)`, `keys.boardSprints(profileId, boardId)`, `keys.board(profileId, boardId, sprintId, swimlane)`; `useBoards`, `useBoardSprints`, `useBoard`, `useSyncBoards`; `BoardCard({ issue, selected, onSelect })`, `BoardsView()`.

- [ ] **Step 1: Shared styles.** Copy XTM's `.board-head`, `.board-picker`, `.board-picker select`, `.board-head-actions`, `.board-counts`, and `.board-scroll` from `xtm/frontend/src/App.css` verbatim into `primitives.css` under "Board toolbar, mirrored from XTM's App.css". Add the new board rules there too, since both apps may take a kanban later:

```css
.board-columns { display: flex; gap: 12px; align-items: flex-start; min-width: max-content; }
.board-column { flex: 0 0 320px; display: flex; flex-direction: column; gap: 8px; }
.board-column-head { display: flex; align-items: baseline; gap: 8px; padding: 6px 10px; background: var(--surface-3); border: 1px solid var(--border); border-radius: 4px; }
.board-column-count { margin-left: auto; font-size: 11px; color: var(--text-muted); font-variant-numeric: tabular-nums; }
.board-lane { display: flex; flex-direction: column; gap: 6px; }
.board-lane-head { display: flex; align-items: baseline; gap: 8px; padding: 4px 10px; background: var(--row-hover); border-radius: 4px; font-size: 12px; }
.board-cell { display: flex; flex-direction: column; gap: 8px; min-height: 40px; }
.board-card { border: 1px solid var(--border); border-radius: 6px; padding: 8px 10px; background: var(--surface-1); cursor: pointer; display: flex; flex-direction: column; gap: 6px; }
.board-card:hover { background: var(--row-hover); }
.board-card-selected { border-color: var(--accent); }
.board-card-head { display: flex; align-items: center; gap: 6px; }
.board-card-foot { display: flex; align-items: center; gap: 8px; font-size: 11px; color: var(--text-muted); }
.board-unmapped { margin-top: 12px; }
```

Check every token against `frontend/core/styles/tokens.css` first and use the ones that exist (`--surface-1`, `--surface-3`, `--row-hover`, `--border`, `--text-muted`, `--accent`); if one is missing, take XTM's `:root` value for it.

- [ ] **Step 2: `api.ts`, keys, hooks.** Types mirroring the Go shapes (`Board`, `Sprint`, `ColumnView` with `name`, `statusIds`, `total`, `points`; `LaneView` with `key`, `label`, `cells`; `BoardView` with `board`, `columns`, `lanes`, `unmapped`), the four bindings (`GetBoard` wrapping its arguments plainly, no class needed since they are scalars), the three keys, and `queries/boards.ts` with `useBoards`, `useBoardSprints`, `useBoard` (with the same `placeholderData` profile guard the tree uses), and `useSyncBoards` (a mutation invalidating all three keys plus the sync state). `invalidateWrites` also invalidates `[profileId, "board"]` so a pending edit repaints its card.

- [ ] **Step 3: `BoardCard.tsx`.** A card: `board-card` (plus `board-card-selected`), head with `TypeChip`, the key in `accent-text`, the Draft chip when `issue.draft`, and the pending dot when `issue.pending`; the summary; the foot with the assignee or "Unassigned" and the points or nothing. `role="button"`, `tabIndex={0}`, Enter and Space select, `aria-pressed` for the selected state, `aria-label` of key and summary.

- [ ] **Step 4: `BoardsView.tsx`.** Toolbar in `board-head`: the board picker (`board-picker`, a select of `useBoards`), the sprint picker (only when the chosen board is `scrum`, from `useBoardSprints`, defaulting to the active sprint, labelled `Name (state)`), the swimlane select (None, Assignee, Epic), and `board-head-actions` with Refresh calling `useSyncBoards`. Body in `board-scroll`: the column heads row, then one band per lane (the lane head hidden when the swimlane is none), each band a row of `board-cell`s aligned to the columns. Under the board, the "not on the board" note when `unmapped > 0`. The detail panel opens beside the board for the selected card, as the Backlog does. States: "Loading the board", "This project has no boards in Jira, or the sync has not run", "No cards in this sprint", and the error line with a Retry. The board, sprint, and swimlane choices reset on a profile switch.

- [ ] **Step 5: Wire the view.** `App.tsx`'s switch gains `current.id === "boards" ? <BoardsView /> : ...`. `App.test.tsx` mocks the four new bindings (`ListBoards` returning `[]` by default) and gains a test that clicking Boards renders the empty state.

- [ ] **Step 6: Tests.** `BoardsView.test.tsx` mocking `../api` and `../contexts/SyncContext`: the toolbar renders both boards and picks the first; choosing the kanban board hides the sprint picker; the columns render with their counts and point sums; cards land in the right columns; the swimlane select switches to assignee and renders lane heads with "Unassigned" last; clicking a card opens the panel with its key; the unmapped note shows its count; Refresh calls `SyncBoards` and refetches; the empty and error states. Commit as `feat(tam): the Boards view`.

---

### Task 5: Docs, the single gate run, and the fix wave

- [ ] **Step 1: Docs.** `tam/CLAUDE.md` gains a "Phase 3a: boards" section (the agile client in `core/jira`, the four tables at schema version 4, the boards pass in the syncer, the four bound methods, the view, and the fact that nothing writes yet), its Layout section lists `internal/boardrepo/`, `app_boards.go`, `BoardsView`, and `BoardCard`, and its Status section says Phase 3a is on this branch. `README.md` gains one sentence.

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

- [ ] **Step 3: The walk-through for the user** (not run by agents): on the demo profile open Boards, see the scrum board with three columns and the active sprint, switch the swimlane to Assignee, click a card and read its detail panel, switch to the kanban board and see the sprint picker go, then Refresh. On a real Jira DC, confirm the board list, the columns, and that a status outside the columns lands in the note.

- [ ] **Step 4: Commit** as `docs(tam): Phase 3a notes for boards and sprints`, then push and open the PR against `main` titled "Task Activity Manager Phase 3a: boards and sprints, the read path" with the tasks, the gates, and the walk-through. No AI attribution anywhere.

## Deferred

Everything that writes (plan 3b: drag to transition, drag to rank, move to sprint, start and complete a sprint), the board's own swimlane and quick-filter configuration, sub-filters, board creation, and non-project boards. Card virtualisation is out until a real board proves it slow.
