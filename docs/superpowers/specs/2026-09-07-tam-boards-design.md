# Task Activity Manager, Phase 3: boards and sprints

Decided 2026-09-07 after Phase 2 merged. The user pre-approves the recommended option at every decision point, so this spec records the choices and their reasons rather than the alternatives.

## 1. What this phase delivers

The Boards view: the project's boards, the columns a board defines, and the issues in them, plus the sprint picker for a scrum board. Phase 3 is one spec and two plans, in the same shape as Phase 1.

**Plan 3a, the read path.** The agile client in `core/jira` (boards, board configuration, sprints, board issues), TAM's board and sprint tables, a sync that fills them, and the Boards view: board picker, sprint picker, columns from the board's own configuration, cards from the issue cache, and a swimlane toggle by assignee or epic. Nothing in 3a writes to Jira.

**Plan 3b, the write path.** Dragging a card between columns (a transition), dragging within a column (a rank), moving an issue to a sprint, and starting or completing a sprint. Section 12 is its design, written when 3a merges.

## 2. Decisions

| Question | Decision | Why |
|---|---|---|
| Where the agile client lives | `core/jira/agile.go`, transport only, returning raw shapes; TAM's backend maps them to DTOs | The same seam as `core/jira/issues.go`; XTM may take boards later, and `core` is populated pull-based |
| Which API version | `/rest/agile/1.0`, the version Jira DC 8 and 9 both serve | The classic Greenhopper endpoints are gone; Agile 1.0 is what DC exposes |
| What a board is | Whatever `/rest/agile/1.0/board?projectKeyOrId=KEY` returns for the profile's project, of type `scrum` or `kanban` | The user picks among their own boards rather than TAM inventing one |
| Where columns come from | The board's own configuration (`/board/{id}/configuration`), which maps each column to a set of status ids | Jira's board is the source of truth; guessing columns from status names is how tools get boards wrong |
| Which issues a column holds | The issue cache, matched by status id, filtered to the chosen sprint on a scrum board or to the board's backlog and columns on a kanban board | No second issue store; the cache already syncs the project |
| Board and sprint storage | Two new tables in `tam.db`, `board` and `sprint`, plus `board_column` for the ordered columns and their status ids | Small, and they let the view render offline like everything else |
| When they sync | With the issue sync, after the issues land, and on demand from the Boards view's Refresh | One button, one progress bar, the pattern the Backlog already sets |
| Cards | The Backlog's row data reused: key, type chip, summary, assignee, points, the pending dot | One issue vocabulary across views |
| Swimlanes | A toggle: none, by assignee, or by epic; grouping happens in the view over the same cards | Two groupings cover the standups TAM is for; the board's own swimlane config is out of scope |
| Sprint picker | Active sprint by default, then the board's other open (future) sprints; closed sprints are not offered | The daily case is the active sprint, and the sync only fetches membership for the active and future sprints, so those are the only ones the picker can honestly offer |
| The demo | The demo backend gains one scrum board with three columns and two sprints, and one kanban board, over the existing dataset | The whole view runs offline, as every other TAM view does |
| Drag | Not in 3a. The board is read-only until 3b | A drag that cannot write is worse than no drag |

## 3. `core/jira/agile.go`

Transport only, four calls, each returning Jira's shape with fields left raw where TAM does not need them:

- `Boards(ctx, projectKey string) ([]RawBoard, error)` over `/rest/agile/1.0/board?projectKeyOrId=`, paged to exhaustion. `RawBoard{ID int, Name, Type string}`.
- `BoardConfiguration(ctx, boardID int) (RawBoardConfig, error)` over `/board/{id}/configuration`. `RawBoardConfig{ColumnConfig struct{ Columns []RawColumn }}`, `RawColumn{Name string, Statuses []struct{ ID string }}`.
- `Sprints(ctx, boardID int) ([]RawSprint, error)` over `/board/{id}/sprint`, paged, tolerating the 400 Jira returns for a kanban board by reporting no sprints rather than an error.
- `BoardIssueKeys(ctx, boardID int, sprintID string) ([]string, error)` over `/board/{id}/issue` or `/board/{id}/sprint/{sprintId}/issue`, asking for the `key` field only, paged. TAM joins the keys to its own cache, so no issue parsing happens here.

Jira's paging on these endpoints uses `startAt`, `maxResults`, and `isLast`; the helper honours `isLast` and falls back to a short page.

## 4. TAM's store, schema versions 5 and 6

Version 4 on main is the cached Jira user list behind the assignee picker, so the board tables land at version 5. Four tables, all keyed by profile:

```
board(profile_id, id, name, type, synced_at, PRIMARY KEY (profile_id, id))
board_column(profile_id, board_id, position, name, status_ids, PRIMARY KEY (profile_id, board_id, position))
board_issue(profile_id, board_id, sprint_id, key, position, PRIMARY KEY (profile_id, board_id, sprint_id, key))
sprint(profile_id, id, board_id, name, state, start_date, end_date, PRIMARY KEY (profile_id, board_id, id))
```

`status_ids` is a JSON array of strings, the same shape the configuration returns. `board_issue` holds one board's membership per scope, `sprint_id` empty for the board's own list and a sprint id otherwise; a sync replaces a board's rows whole, so a card that left a board or sprint since the last sync does not linger.

Version 5 also adds a `status_id` column to `issue`, which is how a card is matched to a column. `CREATE TABLE IF NOT EXISTS` cannot add a column to a table that already exists, so this is the plan's first store migration: it adds the column with `store.AddColumnIfMissing` (a no-op on a database created fresh at version 5, which already has the column from the base DDL) and then clears every profile's sync watermark with `UPDATE sync_state SET last_synced = ''`. That second step exists because an incremental sync only re-reads issues Jira reports changed since the watermark, so a row cached before version 5 would never get a status id filled in on its own; clearing the watermark makes each profile's next sync re-read every issue for this column, without purging anything first.

Version 6 re-keys `sprint`. The first cut keyed it `(profile_id, id)`, which reads as obvious until you remember that Jira returns a sprint from every board whose filter reaches it: two scrum boards over one project would collide on the second board's write, and because a write failure ended the pass rather than dropping the board, every board after it went unsynced. The key is now `(profile_id, board_id, id)`, and because a database already at version 5 never re-runs version 5's migration, the fix needed its own version, which drops and recreates the table. Nothing is lost: the sprint table is a cache and the next sync refills it.

`boardrepo` (a new package beside `issuerepo`, since boards are their own concern) owns the four tables: `UpsertBoards`, `UpsertColumns`, `UpsertSprints`, `UpsertIssueKeys`, `ReplaceBoard` (all four writes for one board in a single transaction, the sync pass's own write step), `ListBoards`, `Columns`, `ListSprints` (by board, ordered active, future, closed, then by start date), `RemoveBoards`, and `PurgeProfile` joins the existing purge. The issue table keeps its `sprint_id`, which the Sprint custom field already fills.

## 5. The board read

`boardrepo.Board(ctx, issues IssueSource, profileID string, boardID int, sprintID, swimlane string) (BoardView, error)` returns what the view draws. `IssueSource` is the two methods boardrepo needs from the issue cache (`IssuesByKeys`, `DraftIssues`), kept as an interface so this package never imports `issuerepo`; `app.go` passes the issue repository, which already has both.

```
BoardView{BoardID int, SprintID, Swimlane string, Columns []ColumnView, Lanes []LaneView, Unmapped int, UnmappedStatuses []string, NotSynced int, Capped bool, NeedsStatusSync bool}
ColumnView{Name string, Total int, Points float64}
LaneView{ID, Label string, Count int, Cells [][]backend.Issue, Overflow []int}
```

One lane when the swimlane is none, one per assignee or epic otherwise (an empty value groups under "Unassigned" or "No epic", last). `Cells[i]` holds the issues of column `i` in that lane. A cell caps at `MaxCardsPerCell` (200 cards) and the whole view caps at `MaxCardsPerView` (2,000); past either cap a card is only counted, in `Overflow` and `Capped`, not rendered. An issue whose status is in no column counts into `Unmapped`, and its status name (deduplicated, sorted) into `UnmappedStatuses`; the "Not on the board" note draws that count and those names, not the issues themselves, so nothing disappears silently without naming a number the count on the note can be checked against. `NotSynced` counts the board's keys the issue cache does not hold, which is how a board's filter reaching outside the profile's project is reported rather than silently dropped. `NeedsStatusSync` is true when every cached card carries an empty status id, which is the state right after the version 5 migration and before the sync that fills the column back in; the view tells the user to sync rather than drawing an empty board and blaming them for it. Cards come from the cache with the same `pending` and `draft` flags every other view uses; a draft is matched to the first column that collects any status at all, since Jira has never assigned one a status.

## 6. Sync

`syncer.SyncBoards` is a distinct pass reached through `backend.BoardBackend`, a capability off the engine's own backend rather than a widening of `IssueBackend`, so a backend that cannot speak Jira's Agile API is simply skipped. `SyncIssues` runs it after the issues pass of the same sync lands; the Boards view's own Refresh runs it alone.

An instance with no Agile API at all answers `Boards` with `core/jira`'s `ErrNoAgile`, which is not a failure: the pass sets the `boards_unavailable` profile setting and `BoardSummary.Unavailable`, and leaves the boards already cached alone. That setting is written whenever the boards call answers, either way, so a profile later repointed at a Jira Software instance stops claiming it has none.

For each board that Jira lists, the pass reads its columns, its sprints, its own issue list, and the issue keys of its active and future sprints, closed sprints are kept in the sprint list for the picker's history but their membership is never fetched, before anything is written. A board whose read fails at any point is recorded in `BoardSummary.Dropped` with its name and one readable line of the reason, and whatever it held before this run is left exactly as it was; only once every read for a board has succeeded does `boardrepo.ReplaceBoard` write all of it in one transaction. A board Jira no longer returns is removed, with its columns, sprints, and issue keys, by `RemoveBoards`. `BoardSummary` totals the boards, columns, sprints, and distinct cards that landed (a key on two scopes of one board counts once), plus the dropped boards and the elapsed time.

## 7. Bound methods

`ListBoards(profileID string) ([]boardrepo.Board, error)`, `ListBoardSprints(profileID string, boardID int) ([]boardrepo.Sprint, error)`, `GetBoard(profileID string, boardID int, sprintID, swimlane string) (boardrepo.BoardView, error)`, `SyncBoards(profileID string) (syncer.BoardSummary, error)`.

## 8. Frontend

`BoardsView` replaces the Boards placeholder: a toolbar with the board picker, the sprint picker (scrum only, hidden on kanban), a swimlane select (None, Assignee, Epic), and Refresh; then the board itself, a horizontal row of columns with a header (name, card count, point sum) and a vertical list of cards, wrapped in one scroll container so wide boards scroll rather than squeeze. Cards are a new `BoardCard` component reusing `TypeChip`, the status-free line (key, summary, assignee, points, the pending dot). Clicking a card opens the existing detail panel beside the board. Swimlanes render as labelled bands, each with its own row of column cells. Empty states: no boards ("This project has no boards in Jira, or the sync has not run"), no cards in a sprint, and the "Not on the board" note with its count. The toolbar mirrors XTM's (`xtm/frontend/src/components/ContainersView.tsx` uses `board-head`, `board-picker`, `board-head-actions`, `board-counts`, and `board-scroll`), and those rules move into `frontend/core/styles` verbatim. XTM's board is a table of containers, not a kanban, so the columns, the cells, and the cards are new; they are built from the same tokens and the card shape the Pending changes dialog already uses (`pending-card`), so the two apps still read as one product.

## 9. Errors

An agile endpoint that answers 403 (a board the user cannot see) drops that board with a line in the sync summary rather than failing the sync. A kanban board's sprint call answering 400 means no sprints. A board whose configuration cannot be read is stored with no columns and the view says the configuration could not be read. The one-line reason next to a dropped board comes from `internal/errtext`, added during review: it strips HTML tags and collapses whitespace from an error's message, since a Data Center answering 403 with an HTML login page hands the transport a kilobyte of markup and the summary line has room for one sentence. Everything else follows Phase 1's rules.

## 10. Verification

Go: the agile client against an httptest server (paging with `isLast`, the kanban 400, a 403 board); `boardrepo` upserts, removals, the ordering of sprints, and the board read with each swimlane, including an issue in no column; the syncer's boards pass with a mid-run failure. Vitest: the toolbar's pickers, columns with counts and points, swimlanes, the "Not on the board" note, card selection opening the panel, and the empty states. Offline: the demo profile shows one scrum board with three sprints (one closed, one active, one future) and one kanban board.

## 11. Out of scope for 3a

Every write (3b), the board's own swimlane and quick-filter configuration, sub-filters, board creation, epics as swimlanes on the epic panel, and non-project boards.

## 12. Mockup

[`assets/2026-09-07-tam-boards.svg`](assets/2026-09-07-tam-boards.svg): the Boards view with a scrum board, the active sprint, assignee swimlanes, a draft card, and the "not on the board" note.
