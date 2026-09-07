# Task Activity Manager Phase 3a: boards and sprints, the read path

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan runs under the lean cycle: implementers build and write the tests named here but do not run suites, except one Go suite run at the end of Task 2 (the store and the client both feed everything after it). Task 5 runs every gate once, then one fix wave.

**Goal:** a Boards view that shows a project's Jira boards, the columns each board defines, the cards in them from the local issue cache, and the sprint picker for a scrum board, all offline-capable and read-only.

**Succeeds when** a scrum master can hold standup on TAM's board instead of the browser tab, because it draws the same columns Jira draws and adds the two things Jira cannot show: the drafts Commit has not created yet and the edits waiting in the journal. 3a is that surface. The drag that makes standup writeable is 3b.

**Architecture:** `core/jira/agile.go` is transport for `/rest/agile/1.0` (boards, configuration, sprints, board issue keys), returning raw shapes. A new `tam/internal/boardrepo` owns three new tables (`board`, `board_column`, `sprint`) at schema version 4 and composes the board view by joining the columns' status ids against the existing issue cache. The syncer gains a boards pass after its issues pass. The frontend adds a `BoardsView` whose toolbar mirrors XTM's `board-head` and whose columns, cells, and cards are new but built from the existing card and chip vocabulary.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

**Spec:** [`../specs/2026-09-07-tam-boards-design.md`](../specs/2026-09-07-tam-boards-design.md). **Mockup:** [`../specs/assets/2026-09-07-tam-boards.svg`](../specs/assets/2026-09-07-tam-boards.svg).

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, `agile-suite/tam`; run Go commands from inside the module directory. Nothing under `xtm/` changes. `frontend/core` changes only by adding XTM's `board-head`, `board-picker`, `board-head-actions`, `board-counts`, and `board-scroll` rules to `styles/primitives.css` verbatim, plus the new board rules this plan names.
- Nothing in 3a writes to Jira. The board is read-only; no drag, no transition, no rank, no sprint start or complete. `IssueBackend` grows read methods only.
- The agile API is `/rest/agile/1.0`. A kanban board's sprint call answering 400 means no sprints, not an error. A board answering 403 is dropped from the sync with a line in the summary. A board whose configuration cannot be read is stored with no columns.
- Cards come from the issue cache only; the board issue keys narrow which cached issues appear, they never introduce an issue TAM has not synced. A board key naming an issue the cache does not hold is counted into `NotSynced`, never dropped in silence.
- A card matches a column by **status id**, not status name. The cache has no status id today, so this plan adds one (Task 2, Steps 1 and 2). Nothing in the view may fall back to matching names.
- The board renders at most 200 cards per column cell; the rest are reported as a count. A kanban board's Done column is every issue the project ever finished, and that must not decide how the view performs.
- The board is read only in 3a and says so on screen. No card is draggable, and nothing hints that it is.
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
7. **The issue cache gains a status id.** Jira's board configuration speaks status ids and the cache only kept the display name, so there was no key to join on. `parseIssue` already receives `status.id` from Jira and throws it away; the column, the DTO field, and the two lines that read it are the whole change. Matching by name was rejected: two workflows can carry the same status name, and Jira's own board would then disagree with TAM's.
8. **The first real store migration.** `baseDDL` is all `CREATE TABLE IF NOT EXISTS`, which never adds a column to a table that already exists. `store.Schema.Migrations` has been there since the store was written and has never been used; version 4 is its first user. The same migration clears `sync_state.last_synced` so the next sync is full and backfills the new column, because an incremental sync only re-reads issues Jira says changed.
9. **A draft card sits in the first column.** A draft has no Jira status (`writes.go` inserts `StatusDraft`), so it matches no column's status ids. Dropping it into "not on the board" would hide the one thing this board has that Jira's does not.
10. **Cells order by the board's own position**, the order `/board/{id}/issue` returned. The cached `rank` is the LexoRank string 3b will need to compute a drop target; it is not needed to draw a board, and keeping both would invite them to disagree.

## File structure

**Created:** `core/jira/agile.go`, `agile_test.go`; `tam/internal/boardrepo/boardrepo.go`, `boards.go`, `view.go`, `boardrepo_test.go`, `view_test.go`; `tam/internal/syncer/boards.go`, `boards_test.go`; `tam/frontend/src/queries/boards.ts`, `components/BoardsView.tsx`, `BoardCard.tsx`, `BoardsView.test.tsx`.

**Modified:** `tam/app_issues.go` (hands the boards repository to the engine); `tam/internal/tamstore/tamstore.go`, `tamstore_test.go`; `tam/internal/backend/backend.go`; `tam/internal/backend/jira/boards.go` (new file in the existing package), `fields.go` (the status id), `jira_test.go`, `fields_test.go`; `tam/internal/backend/demo/demo.go`, `demo_test.go`; `tam/internal/issuerepo/state.go` (purge), `issues.go` (the status id and a keyed read), `issues_test.go`; `tam/internal/syncer/syncer.go`, `syncer_test.go`; `tam/app.go`, `app_boards.go` (new); `tam/frontend/wailsjs/**` (regenerated); `frontend/core/styles/primitives.css`; `tam/frontend/src/api.ts`, `queries/keys.ts`, `queries/invalidate.ts`, `nav.ts`, `App.tsx`, `App.test.tsx`, `App.css`; `tam/CLAUDE.md`, `README.md`; `docs/superpowers/specs/2026-09-07-tam-boards-design.md` (reconciled with this plan in Task 5).

---

### Task 1: The agile client

**Files:** create `core/jira/agile.go`, `core/jira/agile_test.go`.

**Produces:** `jira.RawBoard{ID int, Name, Type string}`, `jira.RawSprint{ID int, Name, State, StartDate, EndDate string}`, and the configuration shaped the way Jira actually sends it, nested and with its own field names:

```go
// RawBoardConfig is /board/{id}/configuration. Jira nests the columns under
// columnConfig and gives each column a list of status objects, not a list of
// ids; flattening that here would mean this package quietly disagreeing with
// the API it exists to speak.
type RawBoardConfig struct {
	ID           int `json:"id"`
	ColumnConfig struct {
		Columns []RawColumn `json:"columns"`
	} `json:"columnConfig"`
}

type RawColumn struct {
	Name     string `json:"name"`
	Statuses []struct {
		ID string `json:"id"`
	} `json:"statuses"`
}

// StatusIDs is the flattening every caller wants, kept beside the raw shape
// rather than inside it.
func (c RawColumn) StatusIDs() []string
```

Decoding a nested payload into a flat struct is not an error in Go: it succeeds and leaves the slice empty, so every board on the instance would render with no columns and nothing would say why. The httptest fixtures in Step 1 are the contract, and they must carry Jira's real envelopes, not a convenient shape.

Also produces: `(*Client).Boards(ctx, projectKey string) ([]RawBoard, error)`, `(*Client).BoardConfiguration(ctx, boardID int) (RawBoardConfig, error)`, `(*Client).Sprints(ctx, boardID int) ([]RawSprint, error)`, `(*Client).BoardIssueKeys(ctx, boardID int, sprintID string) ([]string, error)`; `jira.ErrNoSprints`; `jira.ErrNoAgile`.

`ErrNoAgile` is what `Boards` returns when the instance answers 404 on `/rest/agile/1.0/board`: a Jira Data Center without Jira Software has no Agile API at all, and that is a fact about the instance, not a failure of the sync.

- [ ] **Step 1: The tests.** Create `core/jira/agile_test.go` with an httptest server that answers `/rest/agile/1.0/board`, `/board/1/configuration`, `/board/1/sprint`, `/board/2/sprint` (400 with Jira's kanban message), `/board/1/issue`, and `/board/1/sprint/12/issue`. Cover: boards paged over two pages with `isLast` false then true, and the second page's values appended in order; a board list that includes a type TAM does not know (`simple`) coming back unchanged (the caller filters); the configuration mapping each column to its status ids in order; sprints paged; a kanban board's 400 returning `ErrNoSprints` and no error text leaking Jira's HTML; issue keys asking for `fields=key` and returning the keys of both pages; a 403 on a configuration surfacing as an error the caller can see (the sync decides what to do with it); a 404 on the board list returning `ErrNoAgile`.

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

`Boards` calls it with `projectKeyOrId`; `Sprints` calls it and maps a 400 to `ErrNoSprints` by checking `errors.As` for the client's `*HTTPError` with `Code == 400`, and `Boards` maps a 404 to `ErrNoAgile` the same way; `BoardConfiguration` is a single `Get` into the nested shape above.

`BoardIssueKeys` cannot use `pageAgile`. `/board/{id}/issue` and `/board/{id}/sprint/{sprintId}/issue` do not answer with the `values` and `isLast` envelope the board and sprint lists use; they answer with the search envelope, `{"startAt": 0, "maxResults": 50, "total": 231, "issues": [{"key": "PLAT-412", ...}]}`. Decoding that into `agilePage[T]` finds no `values`, returns an empty page, ends the loop on the first request, and hands back zero keys for every board with no error anywhere. It gets its own small pager:

```go
type agileIssuePage struct {
	Issues []struct {
		Key string `json:"key"`
	} `json:"issues"`
	StartAt    int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total      int `json:"total"`
}
```

It asks for `fields=key` and pages until `startAt + len(issues) >= total`, or until a page comes back empty, which is the guard against an instance that reports a total it will not serve. Doc comments say which endpoint each one calls and that they are transport only.

- [ ] **Step 3: Commit** as `feat(core): the Jira Agile client for boards, sprints, and board issues`.

---

### Task 2: The store and the backend seam

**Files:** create `tam/internal/boardrepo/{boardrepo.go,boards.go,view.go,boardrepo_test.go,view_test.go}`; modify `tam/internal/tamstore/tamstore.go` and its test, `tam/internal/backend/backend.go`, create `tam/internal/backend/jira/boards.go`, modify `tam/internal/backend/jira/jira_test.go`, `tam/internal/backend/demo/demo.go` and its test, `tam/internal/issuerepo/state.go`, `tam/internal/issuerepo/issues.go`.

**Produces:** schema version 4 with `board`, `board_column`, `board_issue`, `sprint`; `backend.Board{ID int, Name, Type string}`, `backend.BoardColumn{Name string, StatusIDs []string}`, `backend.Sprint{ID int, BoardID int, Name, State, StartDate, EndDate string}`; on `IssueBackend`: `Boards(ctx, projectKey string) ([]Board, error)`, `BoardColumns(ctx, boardID int) ([]BoardColumn, error)`, `BoardSprints(ctx, boardID int) ([]Sprint, error)`, `BoardIssueKeys(ctx, boardID int, sprintID string) ([]string, error)`; `boardrepo.New(db *sql.DB) *Repository` with `UpsertBoards`, `UpsertColumns`, `UpsertSprints`, `UpsertIssueKeys`, `RemoveBoards`, `ListBoards`, `Columns`, `ListSprints`, `PurgeProfile`; `boardrepo.Board`, `Sprint`, `BoardView`, `ColumnView`, `LaneView`; `boardrepo.Board(ctx, ...)` the view read; `issuerepo.IssuesByKeys(ctx, profileID string, keys []string) ([]backend.Issue, error)`.

- [ ] **Step 1: Schema and the version 4 migration.** In `tam/internal/tamstore/tamstore.go`, bump `Version` to 4, add `status_id TEXT NOT NULL DEFAULT ''` to the `issue` table in `baseDDL` (right after `status`), and append to `baseDDL`:

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

Add to `indexDDL`: `CREATE INDEX IF NOT EXISTS board_issue_lookup ON board_issue (profile_id, board_id, sprint_id);`.

`CREATE TABLE IF NOT EXISTS` adds the four new tables to an existing file, but it never adds `status_id` to an `issue` table that is already there, and no code in this repo has used `store.Schema.Migrations` yet. Version 4 is its first user. Add to the `Schema` literal:

```go
Migrations: []store.Migration{{
	Version: 4,
	// SQLite has no ADD COLUMN IF NOT EXISTS, and a database created
	// fresh at version 4 already has the column from baseDDL, so a
	// duplicate-column error here is the expected no-op.
	Apply: func(db *sql.DB) error {
		if _, err := db.Exec(`ALTER TABLE issue ADD COLUMN status_id TEXT NOT NULL DEFAULT ''`); err != nil &&
			!strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
		// The board matches cards to columns by status id, and an
		// incremental sync only re-reads issues Jira says changed, so
		// rows cached before version 4 would never get one. Clearing
		// the watermark drops the `updated >=` filter from the next
		// sync, which refetches every issue and fills the column. It
		// is not the same as a full sync: nothing is purged first,
		// and nothing here should be made to purge.
		_, err := db.Exec(`UPDATE sync_state SET last_synced = ''`)
		return err
	},
}},
```

Extend the store test the way the version 3 test does: open, drop the four tables, rewind `schema_version` to 3, reopen, and assert all four exist and the version reads 4. Add a second test for the migration itself: open at version 3 with an `issue` row and a `sync_state` row carrying a `last_synced`, reopen at version 4, and assert the issue has an empty `status_id` column that can be read and that `last_synced` is now empty. A third asserts the migration is idempotent: opening twice more leaves both facts unchanged and returns no error.

- [ ] **Step 2: The status id, end to end, and the backend seam.** The board's whole join depends on it, so carry it first. In `tam/internal/backend/backend.go` add `StatusID string` with the JSON tag `statusId` to `Issue`, directly after `Status`. In `tam/internal/backend/jira/fields.go` the `named` helper decodes only `Name`; give it `ID string` with the tag `id` and set `iss.StatusID = status.ID` beside the existing `iss.Status = status.Name` (Jira returns both in the same object, so no extra field is requested and no extra call is made). In `tam/internal/issuerepo/issues.go` add `status_id` to the upsert's column list, its `VALUES`, and its `DO UPDATE SET`, and to `scanIssue`'s column list and destinations; the draft insert in `writes.go` leaves it empty on purpose, which is what puts a draft in the first column later. In `tam/internal/backend/demo/demo.go` fill `StatusID` from the `StatusID(name)` helper of Step 4 wherever the demo returns an `Issue`. Tests: `fields_test.go` asserts a parsed issue carries both the status name and its id; `issues_test.go` asserts an upserted issue reads its status id back; `demo_test.go` asserts every dataset issue has a non-empty status id.

Then add the three board types (JSON tags `id`, `name`, `type`, `statusIds`, `boardId`, `state`, `startDate`, `endDate`) and the four interface methods with doc comments saying they read Jira's Agile API and never write. Add the four stubs returning `errors.New("not used")` to both fakes in `tam/internal/syncer/syncer_test.go` and to the fake in `tam/internal/committer/committer_test.go`, so the packages still compile.

- [ ] **Step 3: The Jira implementation.** Create `tam/internal/backend/jira/boards.go` mapping the client's raw shapes to the DTOs: `Boards` filters to `scrum` and `kanban` and drops anything else with a log line, and passes `jira.ErrNoAgile` through unwrapped so the sync can tell "this Jira has no boards" from "this call failed"; `BoardColumns` maps the configuration; `BoardSprints` returns an empty slice when the client reports `ErrNoSprints`; `BoardIssueKeys` passes through. Test in `jira_test.go` by extending the fake server with the agile paths (a board list of one scrum, one kanban, one `simple`; a configuration; sprints; issue keys) and asserting the filtering, the kanban's empty sprints, and the key pass-through.

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

Four rules the view has to get right, each with its own test in Step 8:

- **The join is by status id.** A card lands in the first column whose `StatusIDs` contains the issue's `StatusID`. A card whose status id matches no column counts into `BoardView.Unmapped` and is drawn nowhere.
- **A draft has no status id, so it lands in the first column**, not in `Unmapped`. It is the reason this board exists, and hiding it would be the worst outcome of the phase. A board with no columns has nowhere to put it, so it counts as unmapped there.
- **`BoardView.UnmappedStatuses []string` names the statuses that had no column**, deduplicated and sorted, and `LaneView.Count int` carries the lane's card total. Both exist so the view can say something actionable instead of a bare number, which is what the mockup already draws.
- **A board key the cache does not hold counts into `BoardView.NotSynced`.** Board filters routinely reach outside the profile's project; those cards cannot be drawn, and a silent absence is how a board quietly lies. `NotSynced` is an `int` beside `Unmapped`.
- **Each cell renders at most `MaxCardsPerCell = 200` cards** and reports the rest in `LaneView.Overflow [][]int` (parallel to `Cells`), so a kanban Done column of nine hundred issues cannot decide how the view performs. `ColumnView.Total` and `Points` still count every card, capped or not.

- [ ] **Step 6: `IssuesByKeys`.** In `tam/internal/issuerepo/issues.go`, add a read that returns the cached issues for a key list in the list's own order, chunked at 500 keys per statement, with the same `pendingFlag` and `scanIssue` the other reads use. SQL will not return an `IN` list in the list's order and no `ORDER BY` can ask it to, so scan into a `map[string]backend.Issue` and then walk the caller's slice to build the result; a key with no row is skipped rather than yielding a zero issue. Test it in `issues_test.go` for order, a missing key (skipped), an empty list (empty result, no query), and a chunk boundary.

- [ ] **Step 7: Purge.** Add `board`, `board_column`, `board_issue`, and `sprint` to `issuerepo.PurgeProfile`'s table list (it already runs in one transaction) so deleting a profile clears them, and extend its test.

- [ ] **Step 8: Tests for the repository and the view.** `boardrepo_test.go`: upsert and read back boards, columns in position order, sprints ordered active, future, closed, then start date; `RemoveBoards` taking the children; profile isolation. `view_test.go` (with a small in-test `IssueSource`): cards land in the right columns by status id; an issue whose status id is in no column counts in `Unmapped` and appears nowhere else; a draft (empty status id) lands in the first column, and in `Unmapped` when the board has no columns; a board key with no cached issue counts in `NotSynced`; a cell of 250 cards renders 200 and reports 50 overflow while the column total still reads 250; the three swimlanes, including the empty-value lane last; column totals and point sums; an unknown swimlane errors; a board with no columns returns an empty view rather than an error.

- [ ] **Step 9: The one Go suite run.** Inside `tam/`: `go build ./... && go vet ./... && go test ./... -count=1`, and inside `core/`: `go test ./jira/ -count=1`. Fix what fails, rerun at most twice, and report. Commit as `feat(tam): the board store, the board view, and the backend seam`.

---

### Task 3: The sync pass and the bindings

**Files:** create `tam/internal/syncer/boards.go`, `boards_test.go`; modify `tam/internal/syncer/syncer.go`, `syncer_test.go`, `tam/app.go`, create `tam/app_boards.go`; regenerate `tam/frontend/wailsjs/**`.

**Produces:** `syncer.BoardSummary{Boards, Columns, Sprints, Cards int; Dropped []string; Unavailable bool; Elapsed string}`, `(*Engine).SyncBoards(ctx, profileID, projectKey string, onProgress func(Progress)) (BoardSummary, error)`; the four bound methods from the Global Constraints.

- [ ] **Step 1: The pass.** `boards.go` holds `SyncBoards`: list the boards; for each, read the configuration, the sprints, and the issue keys (for a scrum board, the keys of every sprint plus the board's own list; for a kanban board, the board's list); upsert each part; then `RemoveBoards` for the boards the profile had that Jira no longer returns. Each board's parts land in one transaction per board (its row, its columns, its sprints, its issue keys), so a reader on the WAL never sees a board half rewritten, and a board that fails halfway leaves the previous good copy in place.

The keys are a replace, not an upsert: `UpsertIssueKeys` deletes the `(profile_id, board_id, sprint_id)` scope inside that transaction and then inserts. An upsert alone only ever adds, so a card moved out of a sprint in Jira would sit on TAM's board until the profile was deleted. Columns are the same shape: delete the board's columns, then insert the new positions, or a board that loses its fifth column keeps drawing it.

A board whose configuration or keys fail is recorded in `Dropped` with its name and the reason, and the pass continues; the summary counts what landed. An instance with no Agile API (the backend reporting `ErrNoAgile`) is not a failure: the pass returns a summary with `Unavailable: true` and no error, the existing boards are left alone, and the view says this Jira has no boards rather than showing an error line. `BoardSummary` therefore carries `Unavailable bool` alongside its counts. Progress frames use phase `"boards"` with the board name as `Stage`. The engine gains an exported `Boards *boardrepo.Repository` field, left nil by `New` and set by the caller, which is the shape `PageSize` and `Now` already use; a second constructor would multiply with the next optional dependency. A nil `Boards` means the pass does not run, which is what every existing syncer test wants.

`tam/app_issues.go:136` builds its engine per call as `syncer.New(b, a.repo)`, so that line becomes three: build it, set `eng.Boards = a.boards`, call `Sync`. Without that edit the boards pass compiles, tests green, and never runs on a real sync, which is the worst kind of passing build.

- [ ] **Step 2: Wire it into `Sync`.** At the end of `Sync`, after the sync state is written and before the done frame, call `SyncBoards` when the engine has a boards repository; its failure does not fail the issue sync (log it, put it in the summary's new `Boards *BoardSummary` field, which is nil when the pass did not run).

- [ ] **Step 3: Tests.** `boards_test.go` with the existing test fake extended: a scrum and a kanban board land with their columns, sprints, and keys; a board Jira stops returning is removed; a configuration failure drops one board and keeps the other; the summary counts. Extend `syncer_test.go` for the composed run (issues then boards) and for a boards failure leaving the issue sync successful.

- [ ] **Step 4: The app.** `tam/app.go` constructs `boardrepo.New(...)` beside the issue repository in `initStore` and holds it as `a.boards`. `tam/app_issues.go` hands it to the engine (Step 1). Create `tam/app_boards.go` with the four bound methods: `ListBoards`, `ListBoardSprints`, `GetBoard` (passing `a.repo` as the `IssueSource`), and `SyncBoards` (under `a.acquire(p.ID, "sync")`, logging the summary). Each returns non-nil slices.

The standalone `SyncBoards` does not emit progress frames. The frontend's sync reducer only knows about a run that began with its own `SYNC_START`, and frames arriving outside one would be a second, undeclared entry point into shared state; the Refresh button owns its own pending label instead. Inside `Sync` the frames are welcome, because the issues pass has already opened the run.

- [ ] **Step 5: Generate.** `go build ./... && go vet ./...` inside `tam/`, then `wails generate module`; check `App.d.ts` for the four methods and `models.ts` for the `boardrepo` namespace with `Board`, `Sprint`, `BoardView`, `ColumnView`, `LaneView`; revert the runtime and md5 churn and keep `go.mod` unchanged. Commit as `feat(tam): sync boards and sprints, bind the board reads`.

---

### Task 4: The Boards view

**Files:** modify `frontend/core/styles/primitives.css`; create `tam/frontend/src/queries/boards.ts`, `components/BoardCard.tsx`, `components/BoardsView.tsx`, `components/BoardsView.test.tsx`; modify `tam/frontend/src/api.ts`, `queries/keys.ts`, `queries/invalidate.ts`, `App.tsx`, `App.test.tsx`, `App.css`.

**Produces:** `api.ts` types `Board`, `Sprint`, `ColumnView`, `LaneView`, `BoardView`, the four bindings; `keys.boards(profileId)`, `keys.boardSprints(profileId, boardId)`, `keys.board(profileId, boardId, sprintId, swimlane)`; `useBoards`, `useBoardSprints`, `useBoard`, `useSyncBoards`; `BoardCard({ issue, selected, onSelect })`, `BoardsView()`.

- [ ] **Step 1: Shared styles.** Copy XTM's `.board-head`, `.board-picker`, `.board-picker select`, `.board-head-actions`, `.board-head-actions .btn`, `.board-counts`, and `.board-scroll` from `xtm/frontend/src/App.css` verbatim into `primitives.css` under "Board toolbar, mirrored from XTM's App.css". Copy those six and nothing else: the `.board-picker select.container-filter-select` and `.container-type-select` rules beside them size XTM's Test Set and Plan pickers and have no meaning outside XTM, so they stay where they are. Add the new board rules there too, since both apps may take a kanban later:

```css
.board-columns { --board-col-w: 320px; display: flex; gap: 12px; align-items: flex-start; min-width: max-content; }
.board-column { flex: 0 0 var(--board-col-w); display: flex; flex-direction: column; gap: 8px; }
.board-cell, .board-column-head { width: var(--board-col-w); }
.board-column-head { position: sticky; top: 0; z-index: 1; display: flex; align-items: baseline; gap: 8px; padding: 6px 10px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 4px; }
.board-column-count { margin-left: auto; font-size: 11px; color: var(--text-muted); font-variant-numeric: tabular-nums; }
.board-lane { display: flex; flex-direction: column; gap: 6px; }
.board-lane-head { display: flex; align-items: baseline; gap: 8px; padding: 4px 10px; background: var(--row-hover); border-radius: 4px; font-size: 12px; }
.board-cell { display: flex; flex-direction: column; gap: 8px; min-height: 40px; }
.board-card { border: 1px solid var(--border); border-radius: 6px; padding: 8px 10px; background: var(--surface); cursor: pointer; user-select: none; display: flex; flex-direction: column; gap: 6px; }
.board-card:hover { background: var(--row-hover); }
.board-card-selected, .board-card-selected:hover { background: var(--accent-soft); border-color: var(--accent); }
.board-card:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
.board-summary { font-size: 12px; color: var(--text-muted); margin: 0 0 10px; }
.board-picker select.board-select-narrow { min-width: 0; width: auto; }
.board-card-head { display: flex; align-items: center; gap: 6px; }
.board-card-foot { display: flex; align-items: center; gap: 8px; font-size: 11px; color: var(--text-muted); }
.board-unmapped { margin-top: 12px; }
.board-overflow { font-size: 11px; color: var(--text-muted); padding: 2px 4px; }
.boards-body { display: flex; gap: 12px; align-items: flex-start; min-height: 0; flex: 1; }
```

Every token above exists in `frontend/core/styles/tokens.css` and was checked against it: `--surface`, `--surface-3`, `--row-hover`, `--border`, `--text-muted`, `--accent`. There is no `--surface-1` in this design system (the scale is `--surface`, `--surface-2`, `--surface-3`, `--surface-sunken`); do not invent one. The sticky column head needs an opaque background, which is why it takes `--surface-3` rather than a tint.

`.boards-body` is the same shape `.epics-body` uses: the scroller takes `flex: 1; min-width: 0` so it stretches to the detail panel and never pushes it off, and the panel keeps the `.detail-panel` width of 352px. `min-width: 0` is not optional on a horizontal scroller inside a flex row; without it the board's own width wins and the panel is pushed out of the window. If PR #24 (the epic tree width fix) has landed on `main` by the time this task runs, rebase before editing `App.css`: it touches `.epics-body` a few lines above.

- [ ] **Step 2: `api.ts`, keys, hooks.** Types mirroring the Go shapes, all of them: `Board`; `Sprint`; `ColumnView` with `name`, `statusIds`, `total`, `points`; `LaneView` with `key`, `label`, `count`, `cells`, `overflow`; `BoardView` with `board`, `columns`, `lanes`, `unmapped`, `unmappedStatuses`, `notSynced`. A type that quietly omits a field the view needs is how a shape drifts from its Go original, so mirror every one. The four bindings (`GetBoard` wrapping its arguments plainly, no class needed since they are scalars), the three keys, and `queries/boards.ts` with `useBoards`, `useBoardSprints`, `useBoard` (with the same `placeholderData` profile guard the tree uses), and `useSyncBoards` (a mutation invalidating all three keys plus the sync state). `invalidateWrites` also invalidates `[profileId, "board"]` so a pending edit repaints its card, and `invalidateProfileData` (the one `runSync` calls when a sync ends) invalidates the boards, the sprint lists, and the board view too, or the board stays stale after the very sync that refreshed it.

Read the "no Agile API" fact here too: `SyncBoards` records it as the profile setting `boards_unavailable`, so a small `useBoardsUnavailable` hook over the existing `GetProfileSetting` binding lets the view know on a cold start, before it has run a sync of its own, that this Jira has no boards to offer.

- [ ] **Step 3: `BoardCard.tsx`.** A card: `board-card` (plus `board-card-selected`), head with `TypeChip`, the key in `accent-text`, the Draft chip when `issue.draft`, and the pending dot when `issue.pending`; the summary; the foot with the assignee or "Unassigned" and the points or nothing.

The card is one cell of a grid, not a lone button, so its semantics say so: the props are `{ issue, selected, focused, columnName, onSelect, onFocus }`, the element takes `role="gridcell"`, `aria-selected={selected}`, `tabIndex={focused ? 0 : -1}`, and an `aria-label` of key, summary, and column name. The column name matters here more than anywhere else in TAM: Decision 5 took the status chip off the card, so the column is the only thing carrying the status, and a screen reader that never hears it is reading a list of summaries.

It also refuses a drag honestly, because 3a cannot perform one: `draggable={false}`, and an `onDragStart` that calls `preventDefault` and then `announce("Read only for now. Dragging arrives in the next release.")` (`announce` is exported from `@agile-suite/core`). The caveat line at the top of the view is prevention; this is the answer at the moment the user actually asks the question. `.board-card` takes `user-select: none` so a failed drag does not leave text smeared across three cards.

- [ ] **Step 4: `BoardsView.tsx`.** Toolbar in `board-head`: the board picker (`board-picker`, a select of `useBoards`), the sprint picker (only when the chosen board is `scrum`, from `useBoardSprints`, defaulting to the active sprint, labelled `Name (state)`), the swimlane select (None, Assignee, Epic), and `board-head-actions` with a "Read only" chip and Refresh calling `useSyncBoards`. XTM's `.board-picker select` is 280px wide, which is right for a board name and absurd for three swimlane options, so the sprint and swimlane selects take a `board-select-narrow` modifier (`min-width: 0; width: auto`), the same escape hatch XTM gives its own short pickers.

Under the toolbar, the line that orients a standup, because orientation is a read-only board's whole job and the sprint is the frame the conversation sits in: `.board-summary` reads "Sprint 12, active, ends 12 Sep. 27 of 47 points done." from the selected sprint's `startDate` and `endDate` and the summed `ColumnView.Points`, then "synced 12 min ago" from `useSyncState(profileId).lastSynced`. Format the age with `formatWhen` from `src/lib/format.ts`, which the status bar already uses; a second time helper in the same window would give one fact two different wordings.

Body in `board-scroll`: the column heads row, then one band per lane (the lane head hidden when the swimlane is none, and carrying the lane's label and `count` when it is not), each band a row of `board-cell`s aligned to the columns. Heads and cells take their width from one custom property, `--board-col-w`, so the two rows cannot drift apart.

Under the board, the honesty line, carrying whichever counts are not zero: "3 cards are not on the board (Approved, Blocked)" from `unmapped` and `unmappedStatuses`, naming the statuses so the user can act on it; "2 cards on this board have not been synced" from `notSynced`; and, per cell, a `board-overflow` "+41 more" line at its foot. A capped cell still shows the true total in its column head. Beside them, the second half of the caveat: "Columns and cards follow your board's configuration. Quick filters and the board's own swimlanes are not applied."

The detail panel opens beside the board for the selected card, in a `boards-body` flex row: the scroller takes `flex: 1; min-width: 0` and the panel keeps its 352px.

States, each written out, because an unstated state is a state the implementer invents:

| State | What the view shows |
|---|---|
| Loading | "Loading the board" |
| Refetching after the first load | "Refreshing" beside the toolbar, the previous board still drawn, the way the Backlog and Epics do it |
| No boards, never synced | "This project has no boards in Jira, or the sync has not run" |
| No boards, no Agile API (`boards_unavailable`) | "This Jira has no boards", with no suggestion to sync, because syncing will not help |
| The board has no columns | A `pending-banner pending-banner-warn`: "This board's configuration could not be read, so it has no columns." |
| Boards the last sync dropped (`summary.dropped`) | A `pending-banner` naming them: "2 boards were skipped: Ops, Platform". A board silently missing from a picker is a support ticket |
| No cards, nothing outside the project | "No cards in this sprint" |
| No cards but `notSynced > 0` | "No cards in this sprint have been synced. 14 sit outside this project." A plain empty state here would be a lie |
| Error | The error line with a Retry |

Refresh is disabled while the shared sync or a commit is running, since `App.acquire` would otherwise answer with an error the user did not cause, and while its own mutation is in flight. The board, sprint, and swimlane choices reset on a profile switch through the render-time `filtersFor` pattern both other views use, not an effect, which resets one render too late.

Keyboard, lifted from `EpicTree.tsx` rather than invented: one tab stop for the whole board, a `focusKey` in `BoardsView` deciding which card is focusable, Left and Right across columns keeping the row, Up and Down within a cell and then into the next lane, Enter and Space to select, Home and End to the first and last card of the cell. Six hundred tab stops, which is what `tabIndex={0}` on every card gives at this plan's own cap, is not a keyboard model. The scroller takes `role="grid"`, each lane band a `role="row"` labelled with the lane, and each cell a `role="gridcell"` with `aria-colindex`.

Selection matches the rest of TAM: `.board-card-selected` fills with `--accent-soft` the way `.issue-row-selected` and `.folder-selected` do, and `.board-card:focus-visible` takes the same 2px accent outline `.issue-row` has. A border colour alone is not a selection anywhere else in this app.

- [ ] **Step 5: Wire the view.** `App.tsx`'s switch gains `current.id === "boards" ? <BoardsView /> : ...`. `App.test.tsx` mocks the four new bindings (`ListBoards` returning `[]` by default) and gains a test that clicking Boards renders the empty state. `nav.ts`'s Boards blurb still promises "Kanban and the active sprint, with live drag"; 3a has no drag, so it becomes "The board's own columns, the active sprint, and the cards in them." A nav that promises a verb the view does not have is the same lie as a draggable-looking card.

- [ ] **Step 6: Tests.** `BoardsView.test.tsx` mocking `../api` and `../contexts/SyncContext`: the toolbar renders both boards and picks the first; choosing the kanban board hides the sprint picker; the columns render with their counts and point sums; cards land in the right columns; the swimlane select switches to assignee and renders lane heads with "Unassigned" last, each carrying its count; clicking a card opens the panel with its key; the honesty line names the unmapped statuses and the not-synced count; a cell over the cap shows its "+N more" line while its column head still reads the true total; Refresh calls `SyncBoards` and is disabled while the shared sync is running; the summary line reads the sprint dates and the point split; every one of the nine states in the table above, the no-columns banner, the dropped-boards banner, and the "no cards synced" wording among them; and the keyboard block, the way `EpicsView.test.tsx` tests the tree: one tab stop, Right moves a column, Down moves within a cell and then into the next lane, Enter selects, and no reachable state leaves the board with nothing focusable. Commit as `feat(tam): the Boards view`.

---

### Task 5: Docs, the single gate run, and the fix wave

- [ ] **Step 1: Docs.** `tam/CLAUDE.md` gains a "Phase 3a: boards" section (the agile client in `core/jira`, the four tables at schema version 4, the status id column and the first store migration, the boards pass in the syncer, the four bound methods, the view, and the fact that nothing writes yet), its Layout section lists `internal/boardrepo/`, `app_boards.go`, `BoardsView`, and `BoardCard`, and its Status section says Phase 3a is on this branch. Note the two facts a future reader will otherwise learn the hard way: an incremental sync cannot backfill `status_id`, which is why version 4 clears the sync watermark; and a board's membership is whatever Jira's board endpoint returned at sync time, so a card moved on the web board moves in TAM only after the next sync. `README.md` gains one sentence.

Reconcile the spec with the plan in the same commit: `docs/superpowers/specs/2026-09-07-tam-boards-design.md` section 4 says "Three tables" where the plan builds four (`board_issue` joined them), and section 5 says unmapped issues are "listed" where both the plan and the mockup only count them. Fix both, add the status id to section 4's schema, and record the version 4 migration there. Check the mockup in the same pass: `assets/2026-09-07-tam-boards.svg` draws the sprint summary line, the lane counts, and the unmapped note with its status names, so the spec, the plan, and the mockup now say one thing. If the mockup and the shipped view disagree at Task 4's end, the mockup is the one that gets corrected, and Task 5 is where that happens.

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

Added by the `/autoplan` review, and deferred with a reason rather than silently:

- **Syncing the cards a board's filter reaches outside the profile's project.** 3a counts them (`NotSynced`) and says so. Fetching them would mean a second sync scope and a second project's issue types, which is a Phase 4 conversation, not a line in this plan.
- **The sprint goal.** Jira carries one on the sprint and it belongs on the summary line, but the sprint endpoint's `goal` field is not in this plan's fixtures and it is not worth a fifth field on `Sprint` before a real instance has been read.
- **The board's days-to-show rule for the Done column.** The 200-per-cell cap bounds the render honestly; matching Jira's own rule needs the column configuration's `max` and a date filter, which is board-fidelity work that belongs with the quick filters.
- **A live contract test for the Agile envelopes.** The httptest fixtures encode them from documentation; the Task 5 walk-through is what confronts them with a real Data Center.
- **Board counts in the shell's status bar.** The mockup draws "2 boards" there; the status bar belongs to the sync summary and changing it is out of this plan's blast radius.

This repository has no `TODOS.md` and does not use one: deferred work lives in the plan's Deferred section and the spec's "Out of scope", where the next plan's author reads it. That convention wins over the review tool's default of writing a root-level TODO file.

---

## GSTACK REVIEW REPORT

Run 2026-09-07 by `/autoplan` on branch `feat/phase-3a-boards`. Codex was not
installed, so every outside voice ran as a Claude subagent and each phase is
tagged `[subagent-only]`. Restore point:
`~/.gstack/projects/veenone-task-activity-manager/feat-phase-3a-boards-autoplan-restore-20260907-095559.md`.

### Phase 1: CEO review (strategy and scope) [subagent-only]

**0A. Premise challenge.** Six premises carry the plan. Four hold, two do not.

| Premise | Stated or assumed | Verdict |
|---|---|---|
| A board inside TAM is worth building when Jira's own board is a browser tab away | Assumed | Holds, but only for the reason the plan never writes down: TAM's board is the only board that shows uncommitted drafts and pending edits. That is the feature. |
| Columns come from the board's own configuration, and a card's status matches a column's status ids | Stated | **Fails.** The issue cache stores the status **name** and nothing else (`issue.status`, `backend.Issue.Status`, `parseIssue` reading `status.Name`). Jira's configuration returns status **ids**. The join at the centre of the view has no key to join on. Amendment 1. |
| Adding tables to `tamstore` needs no migration because the DDL is `CREATE TABLE IF NOT EXISTS` | Assumed | Holds for new tables, **fails** for a new column on `issue`. `store.Schema.Migrations` exists and this repo has never used it. Amendment 2. |
| A drag that cannot write is worse than no drag, so 3a ships read-only | Stated, seven words | Holds. See the rejected challenge below. |
| The board issue keys never introduce an issue TAM has not synced | Stated | Holds as a rule, **fails as an experience**: a board filter routinely spans projects, and those cards vanish with no count. Amendment 4. |
| Bucketing a few hundred cards is trivial | Stated | Holds for a sprint, **fails for a kanban board**, whose Done column is every issue the project ever finished. Amendment 6. |

**0B. What already exists (leverage map).**

| Sub-problem | Already in the repo | This plan adds |
|---|---|---|
| Authenticated Jira transport, paging, typed errors | `core/jira/client.go` (`Get`, `*HTTPError` with `Code`), `issues.go` paging | `agile.go`, four calls |
| Issue cache with rank, sprint id, assignee, points, parent | `issue` table, `issuerepo` | a status id column |
| Pending and draft flags on every card | `issuerepo` reads compute them | reuse, no change |
| Sync engine, progress frames, partial-failure reporting | `internal/syncer` | a boards pass |
| Busy guard so sync and commit exclude each other | `App.acquire` and `release` | `SyncBoards` joins it |
| Detail panel, type chip, card shapes | `IssueDetailPanel`, `TypeChip`, `pending-card` | `BoardCard` |
| Board toolbar styling | XTM's `board-head` family | move to `frontend/core/styles` |

Nothing here is duplicated work. The one real build is the column composition.

**0C. Dream state delta.** Current: TAM syncs, lists, edits, imports, links, and groups by epic, all offline, all through one Commit. This plan: the same cache drawn as the team's own board, with drafts and pending edits visible on it, still read-only. Twelve-month ideal: standup runs inside TAM because the board writes back through the journal, so a whole standup's moves land in one Commit and survive a dropped VPN. 3a is the surface; 3b is the verb. The gap after this plan is exactly 3b.

**0C-bis. Alternatives.**

| Approach | Effort (CC) | Risk | Verdict |
|---|---|---|---|
| Boards read-only now, writes in 3b (the plan) | ~5 tasks | Users try to drag on day one | **Chosen.** The read path is a prerequisite either way, and the surface is honest once it says it is read-only (amendment 9). |
| Collapse 3a and 3b, ship drag-to-transition now | ~9 tasks | Transitions are new machinery, not a field edit | Rejected on evidence, see the challenge below. |
| Skip the board, group the Backlog grid by status | ~1 task | No column configuration, no board fidelity | Rejected: it answers a different question and throws away the board's own configuration, which is the part users trust. |

**0D. Scope decisions.** Six expansions approved, all inside the blast radius and each under a day: the status id column and its migration, the not-synced count, the staleness age, the column cap, the draft rule, the read-only caveat. Two rejected as duplicates or premature: a second membership model for sprints (the sync writes both in one pass, so drift is a 3b problem) and a schema change for LexoRank (the cache already stores `rank`, so 3b computes from that, not from `position`).

**0E. Temporal interrogation.** Hour 1: a scrum master opens Boards, picks the board, sees the active sprint, and recognises their own columns. Hour 6: they notice a card they created in TAM this morning sitting in the first column with a Draft chip, which no browser tab can show them. Day 2: they try to drag, and the caveat line tells them why they cannot yet. Week 2: a kanban board's Done column has 900 cards and the cap keeps the view alive.

**0F. Mode: SELECTIVE EXPANSION** (auto-decided, P2: every expansion below is in the blast radius and under a day).

**Error and rescue registry.**

| Failure | What the user sees | Rescue |
|---|---|---|
| Jira has no Jira Software (Agile 404) | "This Jira has no boards" | The view says so once; the sync does not fail |
| A board answers 403 | The board is missing from the picker | Named in the sync summary's dropped list |
| A board's configuration fails | The board loads with no columns | The view says the configuration could not be read; other boards are unaffected |
| A kanban board's sprint call answers 400 | No sprint picker | Expected, not an error |
| A card's status is in no column | It is counted, not drawn | The "not on the board" note carries the count |
| A board key names an issue TAM never synced | The card is absent | The "not synced" count says how many |
| An issue cached before schema 4 has no status id | It would land in "not on the board" | The migration clears `last_synced`, so the next sync is full and refills it |

**Failure modes registry.**

| Mode | Likelihood | Blast radius | Mitigation |
|---|---|---|---|
| Status id join fails silently, every card unmapped | Certain without amendment 1 | The whole view | Amendments 1 and 2 |
| A kanban Done column renders thousands of cards | Likely on a real board | The view freezes | Amendment 6, cap at 200 with a count |
| Board membership goes stale between syncs | Certain by design | Cards a minute old | Amendment 5, the age is on screen |
| A draft card has no real status | Certain | Drafts vanish from the board | Amendment 3, drafts sit in the first column |

**0.5. Outside voice** [subagent-only]. The CEO subagent returned twelve findings. Nine are adopted as amendments 3 to 12. One is rejected on evidence and travels to the gate as a taste item. Two are logged as considered.

**Rejected, with the evidence:** the voice's headline was "collapse 3a and 3b and ship drag-to-transition now, because a transition is already a journaled field edit on `status` and rides the machinery 1b built." It is not. Searching `tam/internal` and `core/jira` for "transition" returns nothing: TAM has no transition discovery, no transition ids, and `status` is not one of the seven editable fields. Drag-to-transition needs a new Jira call, a new journal entity, a new conflict rule, and a mapping from a target column's status ids to an available transition. That is 3b, and it is why 3b exists. The read path is a prerequisite for it either way, so nothing is wasted.

**CEO consensus table** (single voice: no Codex, so no dimension can read CONFIRMED).

```
Dimension                              Claude    Codex   Consensus
1. Premises valid?                     no        n/a     single-voice, 2 of 6 failed
2. Right problem to solve?             disputed  n/a     single-voice, challenge rejected on evidence
3. Scope calibration correct?          yes       n/a     single-voice
4. Alternatives explored?              no        n/a     single-voice, table added above
5. Product risks covered?              no        n/a     single-voice, caveat added
6. 6-month trajectory sound?           yes       n/a     single-voice
```

**NOT in scope for 3a** (unchanged, restated after review): every write, quick filters, sub-filters, the board's own swimlane rules, days-to-show, board creation, non-project boards, card virtualisation, and epics as a board swimlane beyond the plain grouping.

**Phase 1 amendments landed in the plan (12).**

| # | Amendment | Where | Source |
|---|---|---|---|
| 1 | The issue cache gains `status_id`; the board joins on it, never on the status name | Global Constraints, Decision 7, Task 2 Steps 1 and 2 | Primary review, verified in `fields.go` and the `issue` DDL |
| 2 | Schema version 4 ships this repo's first `store.Migration`: add the column, clear `sync_state.last_synced` so the next sync is full | Decision 8, Task 2 Step 1 | Primary review, verified in `core/store/store.go` |
| 3 | A draft card lands in the first column, never in "not on the board" | Decision 9, Task 2 Steps 5 and 8 | Primary review |
| 4 | `BoardView.NotSynced` counts board keys the cache does not hold, and the view says so | Global Constraints, Task 2 Steps 5 and 8, Task 4 Step 4 | CEO voice |
| 5 | The toolbar carries the sync age ("synced 12 min ago"), from `useSyncState` through a new `lib/relativeTime.ts` | Task 4 Step 4 | CEO voice |
| 6 | Each cell renders at most 200 cards and reports the overflow; column totals still count everything | Global Constraints, Task 2 Steps 5 and 8, Task 4 Step 4 | CEO voice |
| 7 | `jira.ErrNoAgile` for a 404 board list, `BoardSummary.Unavailable`, and an empty state that says this Jira has no boards | Task 1, Task 2 Step 3, Task 3 Step 1, Task 4 Step 4 | CEO voice |
| 8 | The read-only caveat line under the toolbar, naming what the board does and does not follow | Global Constraints, Task 4 Step 4 | CEO voice |
| 9 | `nav.ts`'s Boards blurb stops promising live drag | Task 4 Step 5 | Primary review |
| 10 | A success statement at the top of the plan | Goal | CEO voice |
| 11 | The alternatives table, including the two rejected approaches | Report 0C-bis | CEO voice |
| 12 | The spec is reconciled with the plan (four tables, the status id, what "unmapped" means) in Task 5 | Task 5 Step 1 | CEO voice |

**Considered and not adopted (2).** A second sprint-membership model was rejected because one sync writes both the membership rows and `issue.sprint_id`, so they cannot disagree within a pass; the drift the voice describes is a 3b problem and 3b will own it. A schema change for LexoRank was rejected because the cache already stores `rank`, which is what 3b will compute drop targets from; `board_issue.position` only preserves the order Jira's board returned.

**Phase 1 complete.** Codex: unavailable. Claude subagent: 12 findings. Consensus: single-voice, no dimension confirmable; 1 challenge rejected on evidence and carried to the gate. Passing to Phase 2.

### Phase 2: design review (UI scope detected) [subagent-only]

**Step 0, design scope: 6/10 before the review.** The plan specified its CSS in
detail and its layout, hierarchy, states, and keyboard barely at all. There is
no `DESIGN.md` in this repo; the design system is `frontend/core/styles/tokens.css`
and `primitives.css` plus XTM's `App.css` as the reference implementation, and
the standing rule is that TAM mirrors XTM wherever XTM has a counterpart. The
existing patterns this view had to match: `.filter-bar` toolbars, `.backlog-body`
and `.epics-body` flex rows with a fixed 352px `.detail-panel`, `.issue-row-selected`
and `.folder-selected` filling with `--accent-soft`, `EpicTree`'s roving-tabindex
keyboard model, and `formatWhen` for every timestamp in the window.

**The seven passes.**

| Pass | Before | After | What changed |
|---|---|---|---|
| 1. Information hierarchy | 5 | 9 | The sprint was a value inside a dropdown. A read-only board exists to orient a standup, and the sprint is the frame the conversation sits in, so `.board-summary` now carries the sprint, its dates, the point split, and the sync age above the columns. |
| 2. States | 4 | 9 | Four states named, nine needed. The five that were missing: refetching, a board with no columns, boards the sync dropped, no Agile API on a cold start, and the empty board that is empty only because nothing was synced. |
| 3. Journey | 6 | 9 | The whole answer to "why can I not drag this" was one line at the top of a view the user has scrolled past. The card now refuses the drag where it happens, announcing why. |
| 4. Specificity | 5 | 9 | Head-to-cell alignment, the panel beside a horizontal scroller, the lane head's content, the column head's format, and the selection style were all left to the implementer. Named, each of them. |
| 5. Accessibility | 3 | 9 | Six hundred tab stops, no arrow model, `role="button"` on a grid cell, and a card whose only status carrier, the column name, was never spoken. Now the grid roles and `EpicTree`'s roving focus, with the same keyboard tests. |
| 6. Consistency | 6 | 9 | 280px selects for three-option pickers, an effect-based profile reset where both other views reset at render time, and a second time helper beside `formatWhen`. |
| 7. Design system | 4 | 10 | `var(--surface-1)` does not exist in `tokens.css`; the plan told the implementer to check every token against that file and then named the one token that fails the check. Fixed, along with a column head two shades off the mockup and XTM's Test Set picker rules riding into shared primitives. |

**Design litmus scorecard** (single voice: no Codex, so nothing can read CONFIRMED).

```
Dimension                       Claude   Codex   Consensus
1. Hierarchy serves the user?   no       n/a     single-voice, fixed
2. States specified?            no       n/a     single-voice, 5 added
3. Journey sound?               no       n/a     single-voice, drag answered at the card
4. Specific, not generic?       no       n/a     single-voice, 5 gaps named
5. Accessibility real?          no       n/a     single-voice, grid model adopted
6. Consistent with TAM?         partly   n/a     single-voice, 3 fixed
7. Design system honoured?      no       n/a     single-voice, token fixed
```

**Phase 2 amendments (16).** The sprint summary line; the sync age through
`formatWhen` rather than a new module; the "Read only" chip and the caveat split
between toolbar and foot; `draggable={false}` with an announcement; grid roles
and roving focus lifted from `EpicTree`; `aria-label` carrying the column name;
selection filling with `--accent-soft` and a `:focus-visible` outline;
`--board-col-w` binding heads to cells; `boards-body` with `min-width: 0`;
sticky column heads; the `board-select-narrow` modifier; the nine-state table;
Refresh disabled under the busy guard; the render-time profile reset;
`UnmappedStatuses` and `LaneView.Count` so the notes say something actionable;
and the `--surface-1` correction with the column head moved to `--surface-2`.

**Considered and not adopted (1).** Replacing XTM's `board-head` family with
TAM's own `.filter-bar` would make the Boards toolbar consistent with the
Backlog and the Epics view but inconsistent with XTM's board, and the standing
rule is that TAM mirrors XTM where XTM has a counterpart. The `board-select-narrow`
modifier fixes the one thing that was actually wrong, the 280px width.

**Phase 2 complete.** Codex: unavailable. Claude subagent: 17 findings, all
verified against the code before adoption. Consensus: single-voice, no dimension
confirmable. Passing to Phase 3.
