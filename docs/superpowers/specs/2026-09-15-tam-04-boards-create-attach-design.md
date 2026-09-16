# 04 - planning (feat:boards-create-attach)

Mirror of Outline: Tools › Task Activity Manager (TAM) › Planning › 04 - planning (feat:boards-create-attach). Approved 2026-09-15.

## Summary

Answers request item **5**. The Boards view gains **+ New board**, which creates a saved filter and a scrum or kanban board in Jira immediately and syncs it, and **+ Add issues**, which journals chosen issues (drafts included) onto a board and pushes them on Commit like every other board write. After Commit, TAM tells the user plainly when an added issue is outside the board's filter and therefore will not show.

## Problem and root cause

- **No create call exists.** `core/jira/agile.go` has reads plus `RankIssue`, `MoveToSprint`, `MoveToBacklog`, `StartSprint`, `CompleteSprint`; `sprintwrite.go` has sprint create, update, delete. There is no filter or board create.
- **No attach path exists.** Board membership in TAM is only what `/board/{id}/issue` answered at sync time. Nothing journals "put this issue on this board".
- **Drafts look attached when they are not.** `boardrepo` `composeBoard` appends `DraftIssues` to every board and every sprint, so a new issue appears on all boards locally and on none in Jira.

## Decisions

| Question | Decision | Not taken |
|---|---|---|
| New board | **Immediate write** (filter + board), then board sync | Journaled board create with placeholder id |
| Add issues | **Journaled**, pushed on Commit | Immediate call from the picker |
| Board filter mismatch | **Warn after Commit**, never edit the filter | Rewrite the board's JQL automatically |

Board create is immediate for the same cost reason sprint management is: a board has no cached version, no conflict story and no rekey path, and a user does it rarely. It goes in the documented exception list next to the sprint writes, with its own fence test.

## Design

### C1. New board

![New board dialog](assets/2026-09-15-tam-bundles/c1.png)

- **Transport** (`core/jira/boardwrite.go`):
  - `CreateFilter(name, jql, description) (FilterID, error)`: `POST /rest/api/2/filter`, shared with the project by default (`sharePermissions: [{type: "project", project: {id}}]`).
  - `CreateBoard(name, type, filterID, projectKey) (BoardID, error)`: `POST /rest/agile/1.0/board` with `location: {type: "project", projectKeyOrId}`.
  - A board create that fails after the filter was created deletes the filter (`DELETE /rest/api/2/filter/{id}`) and reports both outcomes.
- **Service** `internal/boards.Service.Create` takes the profile lock under the name `board`, and the frontend reaches it through `SyncContext.runQuietLock` (dialog holds focus and refuses Escape while in flight, like sprint dialogs).
- **Dialog:** name (required), type scrum or kanban, filter JQL defaulting to `project = KEY ORDER BY Rank` (validated with a `maxResults=0` search on blur, showing the match count), filter name defaulting to `Filter for <board name>`.
- **After create:** boards sync for that board, then the board picker selects it. Kanban board sprint features stay hidden as today.
- Fence: `internal/boards/exceptions_test.go` asserts the exported immediate-write set is exactly `Create`.

### C2. Add issues to a board

![Add issues picker on the Boards view with a pending card](assets/2026-09-15-tam-bundles/c2.png)

- **Journal entity** `issue_board`, field `boardId`, `after_val` = `boardId|scope` where scope is `backlog` or a sprint id. Written by `issuerepo.AddToBoard(keys, boardID, scope)` in one transaction; one row per key per board.
- **Picker:** search over cached issues and drafts not already in the board's membership; multi-select; "Into" backlog or an active or future sprint of that board.
- **Drawing:** a pending add is drawn on its target board only, with a dashed outline and a **Pending add** chip, in the first column that collects its status (drafts in the first column, as today).
- **Draft drawing fix:** `composeBoard` stops appending every draft to every board. A draft appears on a board only when it has a pending `issue_board` row, a pending sprint move or sprint id belonging to that board, or a draft sprint of that board.
- **Commit:** new board-add step after creates and before sprint moves.
  - Backlog scope: `POST /rest/agile/1.0/backlog/{boardId}/issue` in batches of 20 (`issues: [keys]`).
  - Sprint scope: the existing `MoveToSprint`, which also puts the issue on the board.
  - 207 is failure for the whole batch, per `core/jira/bulkwrite.go`.
  - Drafts are held until the create phase gives them a key.
- **Filter check after Commit:** for each board touched, read `/rest/agile/1.0/board/{id}/issue?jql=key in (...)&fields=key` and compare. Missing keys produce a Commit result line: "PLAT-360 is outside PLAT Checkout Board's filter, so it will not show on that board." The journal row is still removed: Jira accepted the write.
- **Discard** removes the row; Pending changes lists it as "Add to PLAT Checkout Board (backlog)".

## Implementation tasks

1. `core/jira` `CreateFilter`, `CreateBoard`, `DeleteFilter`, `AddToBoardBacklog`; tests including 207.
2. `internal/boards.Service.Create` with rollback of the filter; `board` lock name; fence test; binding `CreateBoard`.
3. `NewBoardModal` with JQL check; runs through `runQuietLock`; syncs and selects the new board.
4. `issue_board` journal entity, `AddToBoard`, revert and discard, Pending changes label.
5. `composeBoard` draws pending adds and stops drawing unattached drafts everywhere; `boardrepo` tests.
6. `AddIssuesPopover` in `BoardsToolbar`; keyboard reachable; Vitest.
7. Committer board-add step with batches, draft holding and post-Commit filter check with result lines.
8. Demo backend: board create and add, one staged out-of-filter issue.
9. Docs: `tam/CLAUDE.md` boards sections and exception list, User Guide.

## Testing

- **Go:** filter then board create; board failure deletes the filter; add to backlog batches of 20; 207 fails the batch; draft held until created; filter check reports a missing key without failing Commit.
- **Frontend:** dialog validation and JQL count; picker excludes current members; pending card chip; unattached draft no longer drawn on other boards.
- **Manual, demo profile:** create a kanban board, add a draft and an existing issue, Commit, refresh, both drawn without the chip.
- **Manual, real instance:** create a board on a sandbox project; add one issue that matches and one that does not; confirm the warning.

## Risks and probes

- **Probe:** whether `POST /rest/agile/1.0/board` with `location` is accepted on the target Data Center version and which permission it needs.
- Filters are private by default; a board over a private filter is visible only to its creator. Default here: share with the project, to be confirmed by the probe.
- Changing draft drawing is a visible behaviour change on existing boards; the release note explains why drafts no longer show everywhere.

## Out of scope

- Editing or deleting boards and their filters.
- Column configuration and swimlane settings.
- Removing an issue from a board (only possible by changing the issue or the filter).
