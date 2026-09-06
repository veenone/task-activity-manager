# Task Activity Manager, Phase 2: the epic and story hierarchy

Decided 2026-09-07 after plan 1c merged. The user pre-approved the recommended option at every decision point, so this spec records the choices and their reasons rather than the alternatives.

## 1. What this phase delivers

The Epics view: a tree of epics with their stories, tasks, bugs, and requirements, built from the issue cache Phase 1 already syncs; a way to put an issue under an epic, move it, or take it out, through the journal and Commit like every other edit; and epic creation. No new sync, no new tables, no drag and drop (that arrives with boards in Phase 3, where a dragged card is a live write).

One plan, four tasks, executed under the lean cycle: implementers build and write tests, the suites run once at the end, then one fix wave.

## 2. Decisions

| Question | Decision | Why |
|---|---|---|
| Where the hierarchy comes from | The `issue` table: `type = epic` rows are epics, `parent_key` is the child's epic (the Epic Link field on Jira DC, already synced). | Phase 1 syncs every issue of the project; nothing else needs fetching. |
| Nesting depth | Two levels: epic and its children. Subtasks stay out of scope. | Jira DC epics do not nest; subtasks are a Phase 3 concern with boards. |
| How an issue moves under an epic | `parentKey` joins the editable fields ("Epic" on the Details tab), journaled and pushed on Commit through the Epic Link field. | One write path, one conflict model, one Activity trail. |
| Epic creation | `epic` joins the creatable types. Jira DC requires Epic Name; the create-meta path already asks for required custom fields, and the backend defaults Epic Name to the summary when the form did not set it. | The New issue dialog needs no new fields. |
| Tree presentation | Mirrors XTM's folder tree (`folder-tree`, `folder-item`, `folder-caret-toggle`, `folder-name`, `folder-count`), with the shared rules moved to `frontend/core/styles`. | TAM mirrors XTM's design language by rule. |
| What the right pane shows | The existing detail panel for the selected epic or child. | Nothing new to build; the Details tab gains the Epic field. |
| Progress per epic | Done and total counts and story points summed from the children, computed in SQL. | Cheap, and what the mockup shows. |
| Orphans | A "No epic" node lists non-epic issues without a known parent. | The Backlog's issues stay reachable from the tree. |
| Filters | Search text (matches the epic or any child; a matching child keeps its epic), sprint, and a Show done toggle (default off hides done children and fully done epics). | The same filter vocabulary as the Backlog. |
| Import and epics | The importer accepts type `epic`; an epic row must not carry a parent. | The template gains an Epic row. |

## 3. Go

### 3.1 Repository (`tam/internal/issuerepo/tree.go`)

`TreeQuery{Text string; SprintID string; ShowDone bool}` and `Tree{Epics []EpicNode; Orphans []backend.Issue}` with `EpicNode{Issue backend.Issue; Children []backend.Issue; Total, Done int; Points, DonePoints float64}`. `EpicTree(ctx, profileID string, q TreeQuery) (Tree, error)` runs two queries: the profile's epics (drafts included) ordered by rank then key, and every non-epic issue ordered the same way, then groups children by `parent_key`. Done means the status name is `Done` or `Closed` or `Resolved`, case-insensitively (the same bucket the grid's `statusClass` calls done). The text filter keeps an epic when its key or summary matches or when any child matches, and then shows only the matching children; the sprint filter applies to children (an epic stays when any child is in the sprint); Show done off drops done children and epics whose children are all done. `Total`, `Done`, `Points`, and `DonePoints` count the unfiltered children so the progress reads the same whatever the filter. Orphans are non-epic issues whose `parent_key` is empty or names a key that is not an epic in the cache. `ListEpics(ctx, profileID) ([]backend.Issue, error)` returns the epics for the picker.

### 3.2 The Epic field

`parentKey` joins `EditableFields` (label "Epic", column `parent_key`). `validateField` for it: empty is allowed (take the issue out of its epic); otherwise the value must be the key of an epic in the profile's cache and must not be the issue itself; the issue being edited must not be an epic. The check needs the row, so it runs inside `EditField`'s transaction. `FieldValue` returns `ParentKey`. Drafts keep working: `updateDraftJSON` sets `ParentKey`.

### 3.3 Backends

`UpdateIssue` maps `parentKey` to the discovered Epic Link field, null to clear; without the field it refuses like story points do. `fieldIDs` gains `EpicName` (custom field "Epic Name"), discovered with the others. `CreateIssue` for an epic sets Epic Name to the summary when `Extra` does not already carry the field, and never sends Epic Link for an epic. `CreateFields` hides the Epic Name field from the form when the backend can default it (the field id is known), so the dialog stays minimal for epics. The demo backend stores the parent on update, reports Epic Name as nothing extra, and its dataset already has four curated epics with children.

### 3.4 Bound methods

`GetEpicTree(profileID string, q issuerepo.TreeQuery) (issuerepo.Tree, error)` and `ListEpics(profileID string) ([]backend.Issue, error)`.

## 4. Frontend

- `EpicsView` replaces the Epics placeholder: a toolbar (search, sprint select, Show done checkbox, New epic button opening the New issue dialog preset to epic), the tree on the left, the detail panel on the right for the selected issue. Tree markup mirrors XTM's `FolderTree`: `nav.folder-tree`, an "All epics" root item with the epic count, one `folder-node` per epic (`folder-item` with the caret toggle, the type chip, `folder-name` as key and summary, `folder-count` as "3 of 8 done, 21 pts", the pending dot) whose `folder-children` list the children as compact rows (key, type chip, summary, status chip, points, pending dot), and a "No epic" node last. Expanded state is per profile in memory; the epic whose child is selected stays open. Keyboard: arrows move, Enter selects, Left and Right collapse and expand, the way XTM's tree does it.
- The Details tab's `EditableFields` gains "Epic": a select listing the profile's epics (key and summary) plus "(none)", hidden for epics themselves; it goes through the same Save edit and journal row as the other fields.
- The New issue dialog offers Epic (Story points and the Epic field hidden for it). The import template gains an Epic row.
- Queries: `keys.tree(profileId, q)` and `keys.epics(profileId)`; `invalidateWrites` also invalidates the tree and the epic list, so a parent edit, a create, or a commit refreshes the view.
- Shared styles: XTM's `folder-*` rules move into `frontend/core/styles/primitives.css` verbatim under "Folder tree, mirrored from XTM's App.css".

## 5. Errors

An Epic value that is not an epic, or an epic set on an epic, is refused by `EditField` with a sentence the panel shows. A Commit of a parent change on a Jira without the Epic Link field fails per issue with that message, like points. Everything else follows Phase 1's rules.

## 6. Verification

Go: `EpicTree` grouping, progress counts, the three filters, orphans, and drafts; `EditField` on `parentKey` (valid epic, empty, non-epic target, self, an epic as source); `UpdateIssue` and `CreateIssue` against the httptest Jira (Epic Link on update, Epic Name defaulted on create, Epic Link absent on an epic create); the importer's epic rows. Vitest: the tree renders epics, children, progress, orphans; expand and collapse; selection opens the panel; the filters; the Epic select saves through `EditIssue`; the New dialog's Epic type. The suites run once when every task is in place. Offline: on the demo profile, open Epics, expand Promotions and discounts, move a story to Checkout experience, create an epic, Commit.

## 7. Mockup

[`assets/2026-09-07-tam-epics.svg`](assets/2026-09-07-tam-epics.svg): the Epics view with one epic expanded, a child selected, and the Details tab showing the Epic field.

## 8. Out of scope

Drag and drop (Phase 3), subtasks, epic status transitions, Jira Agile epic endpoints (Epic Link covers DC), and the Backlog grid's Epic column.
