# TODOS

Deferred work, with enough context to pick it up cold.

## Render XTM's test descriptions with core RichText

**What:** use `@agile-suite/core`'s `richtext` (added by TAM bundle 06) to render XTM's test descriptions, preconditions and step text, instead of showing raw Jira wiki markup.

**Why:** the renderer was put in `frontend/core` so both apps could use it. XTM shows the same Jira wiki markup as plain text today, so a test whose description carries `h3.`, `{code}` or a table reads as symbols.

**Pros:** one rendering behaviour across the suite; no new code, only wiring. **Cons:** XTM has its own views, fixtures and tests to update, and its own review; it does not belong in the TAM bundle branch.

**Context:** after bundle 06 lands, core exports `RichText`, `parseRich`, `detectFormat`, `toPlainText`, `RichTextField`, `isAllowedLink`. XTM's link rule is its own; check whether it should move onto `core/src/lib/links.ts` too. TAM wires `onOpenLink` to `BrowserOpenURL` and `onIssueKey` to `browseUrl`; XTM needs the same two handlers.

**Depends on:** TAM bundle 06 merged (branch `feat/tam-bundles-01-06`).

**Raised by:** /plan-eng-review of the bundle 06 plan, 2026-09-16.

## TestProfileConnection records a user for the URL in the form, not the saved one

**What:** `App.TestProfileConnection` (`tam/app_profiles.go`) writes
`jira_username` and `jira_display_name` for the profile after a successful
test, using the URL passed from the Profiles form rather than the one on the
saved row. Its doc comment says that is deliberate, so an unsaved edit is what
gets tested. The consequence is that pointing a profile at a second instance,
pressing Test and then pressing Cancel leaves the second instance's user
recorded against a profile that still points at the first.

**Why it was left:** the next sync overwrites both settings, so the window is
bounded and self-correcting. Gating the write on the URL matching the saved
row costs a profile read on every connection test and breaks the ordinary
flow of testing a URL you are about to save.

**When it would matter:** if anything starts trusting these settings without a
sync having run since, or if the Assigned to me list is ever wrong in a way
that traces back to a connection test. Then gate the write on
`jiraURL == p.JiraURL`, or move it to the save path.

**Raised by:** review of bundle 03 task 1, 2026-09-16.

## The assignee column holds two different kinds of value

**What:** sync writes the assignee's **display name** into `issue.assignee`
(`tam/internal/backend/jira/fields.go`, via `displayName()`), while a local
edit through `AssigneePicker` writes the **username**
(`tam/internal/issuerepo/writes.go`). So after reassigning an issue in TAM the
Backlog grid's ASSIGNEE cell shows a raw username until the next sync
overwrites it with the display name.

**Why it was left:** bundle 03 adds `assignee_name` beside it rather than
changing what either path writes, because normalising the existing column
means a backfill migration over every cached row and a decision about which
value wins for rows that were edited but not yet committed.

**Consequence to know about:** the Assigned to me display-name fallback (for
rows synced before schema 14, whose `assignee_name` is empty) cannot match a
row that was locally edited before the migration, because that row's
`assignee` holds a username. Those rows carry a pending journal entry, so the
pending-edit overlay covers them.

**Raised by:** bundle 03 planning, 2026-09-16.

## The committer's held-draft-sprint-on-a-draft-board path is not reachable yet

**What:** `tam/internal/committer/phases.go`'s `createSprints` resolves a draft
sprint's `originBoardId` through `r.boardRealID` and holds the sprint
(`r.deps.blockedBy`) when that board is itself still a draft this Commit
could not (yet, or ever) create. `tam/internal/issuerepo/boarddrafts.go`'s
`RekeyBoard` correspondingly does not repoint a draft sprint's scope inside
an `issue_board` row's value (see its `ponytail:` comment).

**Why it is not dead code:** both are correct and covered by tests
(`boardcreate_test.go`'s board+sprint cases; `boarddrafts_test.go`), but
`issuerepo.CreateDraftSprint` refuses `BoardID <= 0`
(`tam/internal/issuerepo/sprintdrafts.go`), so nothing in the app can draft a
sprint onto a draft board today. Read cold, the held-sprint path looks like
speculative machinery for a state that cannot occur, and it would be easy to
"clean up" as dead code. It is defensive, not dead.

**Depends on:** `CreateDraftSprint`'s `BoardID <= 0` guard being lifted, if a
future bundle wants to let someone draft a sprint straight onto a board that
is also still a draft.

**Raised by:** bundle 04 fix round 1, 2026-09-16 (task: `RekeyBoard` missing
`issue_board` rows, fix-round-1.md item 2).

## A boards refresh shows a pending sprint edit as undone

**What:** a journaled `sprint_edit` changes the cached `sprint` row at once.
A boards refresh (or the re-read after a start or a completion) replaces
every non-draft `sprint` row with what Jira sent, so the old name and dates
come back on screen while the edit is still pending. Commit still pushes the
edit, and Discard still restores the before value, so nothing is lost; only
the display is stale until Commit.

**When it would matter:** if users edit sprints and refresh before
committing often enough to be confused by it. The fix is to re-apply
pending `sprint_edit` rows after `boardrepo.writeSprints`, the way issue
syncs keep pending field edits.

**Raised by:** bundle 04 Task 7, 2026-09-17.
