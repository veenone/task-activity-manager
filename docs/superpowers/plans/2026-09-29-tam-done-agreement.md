# The done agreement: a team's definition of done, as a ritual document

**Issues:** #115 (the documents, this branch) and #116 (the per-issue ticks,
the next one).
**Branch:** `feat/tam-dod-documents`, cut from `main` at `57c4107`.
**Spec:** none of its own. It reuses the rituals design,
`docs/superpowers/specs/2026-09-14-tam-rituals-local-first-design.md`, which
already settles storage, conflict handling and the editor; what follows is
only what this feature adds to it.

**Goal:** a team's agreement about what finished means has somewhere to live.
One standing document per board, optional additions per sprint, edited in TAM
and synced to Confluence through the machinery the other ritual pages use.
#116 then reads its items and ticks them per issue.

## The name

The document kind is **`doneagreement`**, labelled **Done agreement** on
screen. Three things in TAM already answer to "definition of done" and the
package comment on `tam/internal/donerule` says so; a fourth called `dod`,
`definitionofdone` or `done` would be indistinguishable from them in a grep
and in a code review.

| Name | What it decides | Where |
|---|---|---|
| `donerule` | a status id the board's last column collects | Go, board cache |
| `backend.IsDone` | a status *name* that reads as finished | Go, one issue |
| `lib/unfinished.ts` | the column rule again, on the frontend | TypeScript |
| `doneagreement` | what the *team agreed* must be true, in prose | a document |

The first three answer a question about an issue from data Jira gives. The
fourth is text people wrote, and no code in TAM derives a status from it. The
distinction goes in `donerule`'s package comment, where the other three are
already distinguished, so the next reader finds all four in one place.

## What this branch does (#115)

`ritual_document` is keyed on `(profile_id, board_id, sprint_id,
ritual_type)`, so the board's standing document is **`sprint_id = 0`** and a
sprint's additions are the same type with the sprint's own id. No schema
change, no new table, no migration entry.

1. **The kind and its template.** `ritualtemplate.DoneAgreement` joins the
   `labels` map, so `Known` accepts it and `Label` names it, and stays out of
   `Types`, which is what `EnsureSprintRituals` writes for every sprint. That
   is the whole of "not a sixth automatic page". `Render` gets a case: a
   sentence and one `<ac:task-list>`, board level and sprint level worded
   differently and nothing else in either. It reads no clock and ignores its
   `*time.Location`, so the adoption check in `run.go` (stored body against a
   fresh render, the check #74 broke) is byte-stable by construction.
   `BoardTitle` titles the board's document after the board, since a
   document with no sprint cannot be titled after one.

2. **The board-level path through the sync.** `Run` reconciles the
   `sprint_id = 0` row, if there is one, under the rituals root before it
   walks the sprints, with a zero `Sprint`: no sprint, which is the document's
   own case. The sprint loop stops iterating `ritualtemplate.Types[1:]` and
   iterates the documents the sprint actually holds instead, so a sprint's
   additions sync like everything else and any later kind does too without a
   second list to keep in step.

3. **Creation on request.** `ritualsync.EnsureAgreement` writes one document
   from the template if the row is missing, and the `CreateDoneAgreement`
   binding is the only caller. `EnsureSprintRituals` and
   `ListRitualDocuments` answer with the sprint's documents plus the board's
   standing agreement, so the view, the editor, the conflict banners and the
   save guard reach it through the paths they already use: every one of them
   passes `doc.sprintId` from the document rather than from the picker.

4. **The view.** The Rituals view's nav gains the two entries when they
   exist, and an "Add" item in their place when they do not. Nav identity
   moves from the ritual type to `sprintId:ritualType`, in a new pure
   `lib/ritualNav.ts`, because the board's document and a sprint's additions
   share a type.

5. **The sprint's own pages point at it** (added to the issue after it was
   opened). Review carries a Done agreement section above its two issue
   lists, where the argument about whether something is finished happens, and
   Planning carries the same after Committed scope, because the bar is what
   committing to that scope means. Each is one line of prose naming the
   sprint's own additions page and an `include` macro of the board's
   agreement **by title**.

   By title, never by page id or URL. A page id is empty until the page is
   published and changes afterwards, so a body carrying one renders
   differently once a neighbouring page is published, and `run.go` would read
   an untouched page as one somebody wrote in. That is #74. A title built
   from the board and the sprint is the same bytes every time, and the
   include macro also means the section shows the agreement as it is the day
   the page is opened rather than a copy taken when it was written. The
   golden files for `planning.xml` and `review.xml` move with it, which is
   also the frontend's round-trip corpus, so the macro is proven to survive
   the editor.

## Places that assumed a ritual belongs to a sprint

Searched for, rather than hoped about. Two needed changing, the rest hold.

- `ritualsync.Run`'s `Types[1:]` loop: changed, see above. It would have left
  a sprint's additions never synced.
- `App.demoSpace` rebuilds the in-memory demo space from stored pages by
  looking up each document's sprint overview as its parent. A document with
  no sprint has no overview, so it was dropped and the next demo Sync would
  have called the page gone. It is now restored under the root.
- `ritualrepo` holds every key as `(profile, board, sprint, type)` already;
  `sprint_id = 0` is an ordinary value to it. Its error lines read "for
  sprint 0", which is cosmetic and reaches no user surface.
- `ritualrepo.BoardSprintIDs` answers `0` for a board with an agreement, and
  `ritualsync.Sprints` matches it against no cached sprint and drops it. That
  is the wanted behaviour: the board's document is not a sprint to walk.
- `app_reportout.go`'s `sprintPageID` asks for the `_sprint` overview by
  name, so a report never picks the agreement up.
- `RitualEditor` saves with `doc.boardId, doc.sprintId, doc.ritualType`, and
  `RitualsView`'s `onSaved` matches on the same three, so both work for
  `sprintId = 0` unchanged.
- `boardrepo.RemoveBoards` leaves `ritual_document` alone by design and
  `PurgeProfile` sweeps it; both already cover the new rows.

A sprint report section drawn from the agreement is **not** in scope here or
in #116: the figures it would need are the per-issue ticks, so it comes after
them, in its own issue.

## What the next branch does (#116)

Built, on `feat/tam-done-agreement-ticks`. The objective was stated after the
issue was opened and settles two things this section had guessed at: **the
point of the ticks is a record a team can be shown at a sprint review**, and
the artefact is the report #117 publishes rather than the store. What follows
is what landed.

- A new profile-keyed table, `done_agreement_tick`, keyed on
  `(profile_id, board_id, issue_key, item_text)`. Through `baseDDL` with a
  version bump to 22, the way `board_column_limit` arrived: a whole new table
  needs no migration entry. Swept by `boardrepo.PurgeProfile`, which is the
  purge list a board-keyed table belongs to, and by `RemoveBoards` for the
  reason `board_column_limit` is in it. The instruction gate checks both.
- **Local only.** The ticks are TAM's own record of what a team checked, not a
  change to a Jira issue, and Jira has nowhere to put them, so they are not
  journalled and not pushed on Commit. That is the one write in TAM that works
  this way, and the reason is on the table's DDL, on the repository and on the
  bindings. The panel says it too, because Work log says the opposite two
  sections above it.
- **An item is identified by its text**, because a task list in Confluence
  storage gives nothing else that survives an edit. So the row stores the
  wording the tick was made against, and nothing rewrites or drops it because
  the agreement has moved on: a tick that followed the document would be
  evidence that changed underneath the review. Reword an item and the old tick
  stays, shown against the old words and marked as made against wording since
  changed. The same rule answers the removed-and-restored case: the words
  match again only if they really are identical, which is what the row says.
- **The effective list** is the board's items plus, for an issue in a sprint,
  that sprint's additions. An issue with no sprint sees the board's alone.
- **The parse lives on the frontend**, in `lib/doneAgreement.ts`, over
  `lib/storage/xml`. #117's report document is built in TypeScript in
  `lib/reportDocument`, so a parse in Go would be answered by a second one on
  this side anyway. Go stores a tick against an item's text and never learns
  what an item is. `DoneAgreementTicks` takes a list of issues so the report's
  section can ask for a whole sprint in the call the panel makes for one.
- The panel section sits beside the others in `IssueDetailPanel`, collapsed
  by default, like Work log, with a `4 of 7` count in its summary counting the
  items the agreement still states. Unlike Work log both its reads are local,
  so they run when the panel opens rather than on expand; a count nobody can
  see until they expand the section is not a summary.

## Gates

`cd tam && go build ./... && go vet ./... && go test ./... -count=1`;
`frontend/core/src/instruction-gate.test.ts`; the frontend suites this
touches, by path; `npm run lint` then `bash scripts/ratchet.sh`;
`wails generate module` from `tam/` for the new binding.
