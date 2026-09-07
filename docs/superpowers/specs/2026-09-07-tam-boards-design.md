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
| Sprint picker | Active sprint by default, then any sprint of the board, closed ones last | The daily case is the active sprint |
| The demo | The demo backend gains one scrum board with three columns and two sprints, and one kanban board, over the existing dataset | The whole view runs offline, as every other TAM view does |
| Drag | Not in 3a. The board is read-only until 3b | A drag that cannot write is worse than no drag |

## 3. `core/jira/agile.go`

Transport only, four calls, each returning Jira's shape with fields left raw where TAM does not need them:

- `Boards(ctx, projectKey string) ([]RawBoard, error)` over `/rest/agile/1.0/board?projectKeyOrId=`, paged to exhaustion. `RawBoard{ID int, Name, Type string}`.
- `BoardConfiguration(ctx, boardID int) (RawBoardConfig, error)` over `/board/{id}/configuration`. `RawBoardConfig{ColumnConfig struct{ Columns []RawColumn }}`, `RawColumn{Name string, Statuses []struct{ ID string }}`.
- `Sprints(ctx, boardID int) ([]RawSprint, error)` over `/board/{id}/sprint`, paged, tolerating the 400 Jira returns for a kanban board by reporting no sprints rather than an error.
- `BoardIssueKeys(ctx, boardID int, sprintID string) ([]string, error)` over `/board/{id}/issue` or `/board/{id}/sprint/{sprintId}/issue`, asking for the `key` field only, paged. TAM joins the keys to its own cache, so no issue parsing happens here.

Jira's paging on these endpoints uses `startAt`, `maxResults`, and `isLast`; the helper honours `isLast` and falls back to a short page.

## 4. TAM's store, schema version 4

Three tables, all keyed by profile:

```
board(profile_id, id, name, type, synced_at, PRIMARY KEY (profile_id, id))
board_column(profile_id, board_id, position, name, status_ids, PRIMARY KEY (profile_id, board_id, position))
sprint(profile_id, id, board_id, name, state, start_date, end_date, PRIMARY KEY (profile_id, id))
```

`status_ids` is a JSON array of strings, the same shape the configuration returns. `boardrepo` (a new package beside `issuerepo`, since boards are their own concern) owns them: `UpsertBoards`, `UpsertColumns`, `UpsertSprints`, `ListBoards`, `BoardColumns`, `ListSprints` (by board, ordered active, future, closed, then by start date), and `PurgeProfile` joins the existing purge. The issue table keeps its `sprint_id`, which the Sprint custom field already fills.

## 5. The board read

`boardrepo.Board(ctx, profileID string, boardID int, sprintID string, swimlane string) (BoardView, error)` returns what the view draws:

```
BoardView{Board, Columns []ColumnView, Lanes []LaneView, Unmapped int}
ColumnView{Name string, StatusIDs []string, Total int, Points float64}
LaneView{Key, Label string, Cells [][]backend.Issue}
```

One lane when the swimlane is none, one per assignee or epic otherwise (an empty value groups under "Unassigned" or "No epic", last). `Cells[i]` holds the issues of column `i` in that lane, in rank order. An issue whose status is in no column counts into `Unmapped` and is listed under a "Not on the board" note, so nothing disappears silently. Cards come from the cache with the same `pending` and `draft` flags every other view uses.

## 6. Sync

`syncer` gains a boards pass that runs after the issues pass of the same sync: list the boards, their configurations, and their sprints, then upsert. A board that Jira no longer returns is removed with its columns and sprints. Failures are reported the way a partial issue sync is: the issues that landed stay, the error goes to the sync state, and the Boards view shows the last good data with the error line. The Boards view's own Refresh runs the same pass alone.

## 7. Bound methods

`ListBoards(profileID string) ([]boardrepo.Board, error)`, `ListBoardSprints(profileID string, boardID int) ([]boardrepo.Sprint, error)`, `GetBoard(profileID string, boardID int, sprintID, swimlane string) (boardrepo.BoardView, error)`, `SyncBoards(profileID string) (syncer.BoardSummary, error)`.

## 8. Frontend

`BoardsView` replaces the Boards placeholder: a toolbar with the board picker, the sprint picker (scrum only, hidden on kanban), a swimlane select (None, Assignee, Epic), and Refresh; then the board itself, a horizontal row of columns with a header (name, card count, point sum) and a vertical list of cards, wrapped in one scroll container so wide boards scroll rather than squeeze. Cards are a new `BoardCard` component reusing `TypeChip`, the status-free line (key, summary, assignee, points, the pending dot). Clicking a card opens the existing detail panel beside the board. Swimlanes render as labelled bands, each with its own row of column cells. Empty states: no boards ("This project has no boards in Jira, or the sync has not run"), no cards in a sprint, and the "Not on the board" note with its count. The toolbar mirrors XTM's (`xtm/frontend/src/components/ContainersView.tsx` uses `board-head`, `board-picker`, `board-head-actions`, `board-counts`, and `board-scroll`), and those rules move into `frontend/core/styles` verbatim. XTM's board is a table of containers, not a kanban, so the columns, the cells, and the cards are new; they are built from the same tokens and the card shape the Pending changes dialog already uses (`pending-card`), so the two apps still read as one product.

## 9. Errors

An agile endpoint that answers 403 (a board the user cannot see) drops that board with a line in the sync summary rather than failing the sync. A kanban board's sprint call answering 400 means no sprints. A board whose configuration cannot be read is stored with no columns and the view says the configuration could not be read. Everything else follows Phase 1's rules.

## 10. Verification

Go: the agile client against an httptest server (paging with `isLast`, the kanban 400, a 403 board); `boardrepo` upserts, removals, the ordering of sprints, and the board read with each swimlane, including an issue in no column; the syncer's boards pass with a mid-run failure. Vitest: the toolbar's pickers, columns with counts and points, swimlanes, the "Not on the board" note, card selection opening the panel, and the empty states. Offline: the demo profile shows one scrum board with two sprints and one kanban board.

## 11. Out of scope for 3a

Every write (3b), the board's own swimlane and quick-filter configuration, sub-filters, board creation, epics as swimlanes on the epic panel, and non-project boards.

## 12. Mockup

[`assets/2026-09-07-tam-boards.svg`](assets/2026-09-07-tam-boards.svg): the Boards view with a scrum board, the active sprint, assignee swimlanes, a draft card, and the "not on the board" note.
