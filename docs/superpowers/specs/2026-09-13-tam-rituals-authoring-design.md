# TAM Rituals authoring: drafts, a wizard, and publishing to Confluence

Phase 5 gave TAM a Rituals view that reads a Confluence page somebody else
wrote. This design turns rituals into documents TAM authors: a draft held
locally, built through a wizard so every sprint produces the same four
documents, carrying the sprint's Jira issues and the remarks a team makes about
them, published to Confluence through the journal like every other TAM write.

## What already exists

More than the requirements suggest, and one piece is built but unreachable.

`tam/internal/ritualrepo` has a `Draft` type and `Get`, `Upsert` and `Delete`,
with tests. Its table `ritual_document` lives in `tam.db`
(`tam/internal/tamstore/tamstore.go:256`) and is keyed
`(profile_id, board_id, sprint_id, ritual_type)`, carrying `title`, `remark`,
`body`, `issue_keys_json`, `confluence_page_id`, `confluence_version`,
`status`, `updated_at` and `published_at`.

Nothing calls that repository except `SeedDemo`. No `App` method exposes it, so
the frontend cannot read or write a draft today.

`core/confluence` is read-only: `GetPage`, `ListChildPages`, and an internal
`get`. There is no way to write a page from TAM at all.

`profile.RitualAssociation` links an existing Confluence page to a
board/sprint/ritual type, and `app_rituals.go` can list, set and delete those
associations and fetch a page. That path stays; linking a hand-written page is a
different job from authoring one.

## Ownership: TAM owns blocks, humans own the page

A published ritual page has TAM-owned regions and human prose side by side.
TAM rewrites its own blocks on every publish and never reads or writes anything
else on the page.

This matches how rituals actually run. TAM supplies facts, which are the sprint
figures and the issue table. The room supplies notes, typed into Confluence
during the meeting, often while the page is open. A model where TAM owns the
whole page would delete those notes on the next publish.

Three blocks, in fixed order, each independently replaceable:

| block | content | source |
|---|---|---|
| `summary` | sprint name, dates, state, review figures | the stored `sprint_report` from Phase 4 |
| `remark` | the author's note for this ritual | `ritual_document.remark` |
| `issues` | key, summary, status, and per-issue remark | `issues_json` joined to the `issue` table |

### The marker contract is not yet decided

Split ownership requires TAM to find its blocks after a human has edited around
them. The candidates are not equally durable. HTML comments
(`<!-- tam:issues:start -->`) are the obvious choice and the fragile one: the
rich editor can drop or move them on save, and a lost end marker means the next
publish either duplicates the block or consumes human text below it. The anchor
macro survives editor round-trips because Confluence treats it as content. A
structured macro wrapping the block body is stronger still, because the editor
keeps a macro whole.

This design does not choose between them, because the answer depends on the
Confluence Data Center version and editor in use.

**Prerequisite probe.** Before the splice is built, create a page through the
API carrying each candidate marker, open it in the editor, type above, below and
between the blocks, save, and re-read through the API. Record which markers came
back intact. Four requests, one answer, and it decides the contract the rest of
this design rests on.

Phase 4 opened with exactly this kind of probe and its asset still records the
answers as assumed, because the probe never ran. Do not repeat that here: the
splice can destroy a team's meeting notes, which the changelog read could not.

If every candidate proves fragile, fall back to TAM owning a whole page and
linking a separate child page for human notes. That is uglier and cannot
corrupt anything.

## Rendering is derived, and must be deterministic

TAM stores the inputs to a ritual, never an authored body. The blocks are
rendered at publish time from those inputs. Storing a body as well would create
a second source of truth that drifts the moment a story point changes.

Determinism is a hard requirement, not a preference. If unchanged inputs render
differently twice, every publish shows a false diff in the Commit tray and
people stop reading it. Three rules buy it:

1. Issue order comes from `issues_json`, never from a query result. Query order
   is not stable across syncs.
2. No clock inside a block. "Published at 14:03" belongs in `published_at` and
   the TAM UI, not in rendered content.
3. Figures render from the stored `sprint_report`, not a live recomputation, so
   a report rebuilt at a new `algo_version` is a visible input change rather
   than silent drift.

The `body` column keeps the last published render. Publish compares a fresh
render against it, which gives the journal its before-value without a network
call and lets the Commit tray diff offline. When they match, nothing is
journaled.

## Data model

Keep `ritual_document` and extend it.

**`issue_keys_json` becomes `issues_json`.** Per-issue remarks mean the array
holds objects rather than strings, and its order is the published table order:

```json
[{"key": "PLAT-14", "remark": "demoed, docs follow-up"},
 {"key": "PLAT-22", "remark": "blocked on infra"}]
```

This is schema version 10. `ritual_document` itself arrived at version 9 and
needed no migration entry, because `Base` runs before migrations on every open
and the version bump records the change. A column on an existing table is
different and does need one, in the `AddColumnIfMissing` shape that versions 7
and 8 use for `sprint.goal` and `sprint.complete_date`.

Add `issues_json` rather than renaming in place: SQLite makes renames awkward,
and a migration that reads the old `issue_keys_json` array and writes each key
as `{"key": k, "remark": ""}` converts existing rows without a table rebuild.
The old column is left in place and unread, the way a cautious migration
should. Test it the way versions 3 and 4 are tested: rewind the recorded
version, reopen, and assert the column and its converted contents came back.

**`body` is redefined as the last published render**, not a draft body. Nothing
authors it.

**The sprint parent page** needs an id of its own. A second row in
`ritual_document` with `ritual_type = "_sprint"` reuses the existing primary key
and version columns rather than adding a table.

**`status` gains defined values**, which nothing sets today:

| status | meaning |
|---|---|
| `draft` | local only, never published |
| `queued` | journaled, waiting for Commit |
| `published` | on Confluence, render matches `body` |
| `stale` | published, but inputs have changed since |

`stale` is what makes this usable mid-sprint: issues move and reports rebuild,
and the view says the page has drifted rather than implying it is current.

Nothing changes database. `ritual_document` stays in `tam.db` beside `sprint`
and `sprint_report` as per-sprint working data; the Confluence connection stays
in `profiles.db`.

**`ritual_document` joins the purge list.** `PurgeProfile` at
`tam/internal/boardrepo/boardrepo.go:84` sweeps `board`, `board_column`,
`board_issue`, `sprint` and `sprint_report`. A profile's rituals must go with
it.

## Page tree

TAM creates a page per sprint under the configured root, with the rituals as
its children:

```
Root (confluenceRootPageId)
 └─ Sprint 14
     ├─ Planning
     ├─ Standup
     ├─ Review
     └─ Retrospective
```

The sprint page becomes a natural index, and `ListChildPages` already reads this
shape. The cost is five pages per sprint rather than four.

## Issue selection

Each ritual type proposes the slice that fits it, and the wizard lets the user
adjust before saving:

| ritual | default slice |
|---|---|
| Planning | the sprint's backlog, unestimated first |
| Standup | in progress and blocked |
| Review | completed this sprint |
| Retrospective | carried over and completed |

Defaults give the consistency the requirement asks for; the adjustment is the
escape hatch for a sprint that does not fit the pattern.

## Authoring

**Scaffolding a sprint.** One action creates all four drafts at once with
per-type issue defaults selected, titles from a template, and empty remarks. The
same action next sprint produces the same four documents in the same shape,
because the same code chose them.

**The wizard** refines one ritual in four steps: scope (board, sprint, type,
pre-filled when launched from a slot), issues (defaults pre-selected, the rest
of the sprint listed, reorderable because order is published order), remarks
(the author note and a remark per selected issue), and preview (the three
rendered blocks exactly as they will appear).

Save writes a draft. **Publish is a separate, deliberate press.** Nothing
reaches Confluence because a user finished filling in a form.

Remarks are plain text. Confluence owns prose; TAM owns facts. A rich text
editor here would compete with the tool being published into.

## App seam

Six methods, mirroring the association methods already in `app_rituals.go`,
each taking `profileID` first and calling `requireProfile`:

```
ListRitualDrafts(profileID, boardID, sprintID)
GetRitualDraft(profileID, boardID, sprintID, ritualType)
SaveRitualDraft(profileID, draft)
DeleteRitualDraft(profileID, boardID, sprintID, ritualType)
ScaffoldSprintRituals(profileID, boardID, sprintID)
PublishRitual(profileID, boardID, sprintID, ritualType)
```

`ListRitualDrafts` is new on the repository; the rest map onto existing methods.

`RitualsView` gains the sprint's four slots, each showing type, status and
whether it has drifted. An empty slot offers Create; a filled one offers Edit,
Publish and Delete.

## Publishing

### Confluence has no partial update

A page body is written whole or not at all. Three journaled blocks therefore
become one page write, and the committer must group pending changes by ritual
before touching the network.

### Two new client methods

```go
CreatePage(ctx, spaceKey, parentID, title, body string) (Page, error)
UpdatePage(ctx, id, title, body string, version int) (Page, error)
```

`UpdatePage` sends `version.number = version + 1`. Confluence rejects a stale
number itself, which provides optimistic concurrency without inventing a scheme.

### Journaling

Publish renders the three blocks, compares them against `body`, and journals one
`PendingChange` per changed block:

```
EntityType   "ritual"
EntityKey    "<boardID>:<sprintID>:<ritualType>"
Field        "summary" | "remark" | "issues"
BeforeVal    that block from body
AfterVal     the fresh render
BaseVersion  confluence_version
```

Unchanged blocks are not journaled. The ritual moves to `queued`.

Per-block rows exist so the Commit tray shows a readable diff even though they
push as a single write. That legibility is the reason for routing publishes
through the journal rather than sending them immediately.

### Commit

For each ritual key, in order: ensure the sprint parent page exists and create
it if not; fetch the current page, or create it when `confluence_page_id` is
empty; splice each TAM block between its markers, leaving every byte of human
content untouched; update with `BaseVersion + 1`; then store the new version,
set `body` to the render just pushed, set `status` to `published`, stamp
`published_at`, and delete the pending rows.

### Failure

**Version moved.** Confluence rejects the write. Surface it in the vocabulary
the committer already speaks, as a `Conflict` with override or keep-remote, the
way board conflicts work in `committer/boardconflict_test.go`. Override
re-splices onto the newer page and pushes again; keep-remote drops the pending
rows and marks the ritual `stale`.

**Markers missing or damaged.** Someone deleted a block in the editor. Do not
guess where it went: fail that ritual with a sentence naming the block, and
offer republishing the page whole as an explicit choice.

**Confluence unreachable or unauthorized.** Fail that ritual alone. A backend
that cannot write boards still pushes its transitions, and a dead Confluence
must not hold up a Commit full of Jira writes.

**A partial sprint set.** Each ritual is independent; the Commit report names
which landed.

### Not included

No retry loop, no background publishing, and no page deletion. Deleting a ritual
locally leaves the Confluence page alone, and the UI says so. A tool that
deletes a team's meeting notes because someone tidied a list is not trusted
twice.

## Verification

Three tests carry the most weight.

**Splice preserves human content.** Human text above, below and between blocks
survives a republish byte for byte; a page with blocks and no human text still
works; and a block whose markers were damaged fails loudly rather than guessing.

**Rendering is deterministic.** The same inputs render identically twice, and
republishing an unchanged ritual journals zero pending changes rather than three
no-ops.

**A dead Confluence does not hold up Jira.** A Commit carrying issue edits and a
ritual publish lands the issue edits when Confluence is unreachable.

The rest:

- Client: `httptest` covering create and update, the auth header, the version
  increment, and a 409 mapping to a conflict.
- Conflict: a moved version surfaces `Conflict`; override re-splices onto the
  newer page; keep-remote drops pending rows and marks the ritual `stale`.
- Ordering: the sprint parent is created before its first child, and a ritual
  never publishes under a missing parent.
- Repository: `ListRitualDrafts` returns one sprint's four; `issues_json`
  round-trips per-issue remarks; `PurgeProfile` clears `ritual_document`.
- Frontend: the four slots and each status; the wizard's per-type defaults for
  all four types against two fixtures, so a hardcoded list cannot pass; Save
  writes a draft and does not publish.

The existing gate runs once at the end, plus `wails build`. Counts to beat, not
to lower: 46 in `frontend/core`, 159 in `xtm`, 485 in `tam`.

## Out of scope

Editing human prose from TAM. Comments, attachments and full-text search.
Publishing anything other than rituals. Deleting Confluence pages. Templates
beyond the four ritual types. Scheduling or reminders.
