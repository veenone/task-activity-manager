# TAM releases and components, PR 4: the Components view

**Goal:** A user lists the project's components with their lead, default
assignee and issue count, sees the issues filed under one, and creates, edits
and deletes components. Each of those is a pending change that Commit sends.

**Architecture:** Three journal entity types modelled on the release ones
(`component_create`, `component_edit`, `component_delete`). A drafted
component is a `project_component` row with a negative id from its own
sequence. The id rewriting PR 3 wrote for fix versions (cached issues, pending
edits, draft issues, move targets) becomes one helper driven by a catalog
kind, so releases and components share it. Component creates and edits run in
the releases phase after the release ones; deletes run in the closing phase
after the release deletes. The backends gain an optional
`backend.ComponentBackend`; the committer type-asserts it as it does
`VersionBackend`.

**Spec:** `docs/superpowers/specs/2026-11-23-tam-releases-components-design.md`
(issue #605), Delivery item 4.

## Global constraints

- Worktree `C:\projects\tam-605-4`, branch `feat/605-components-view`.
  Commits say `Refs #605`, no closing keywords, no AI attribution.
- `go vet` and `go test` on the touched packages only, the touched Vitest
  files, `npm run lint` once before the PR.
- Test first (P2): each behaviour's test is seen failing on an assertion.
- Files under 400 lines. New logic goes in new files.

## Decisions where the spec leaves room

- **Lead and default assignee.** Both are in the form and in the journal
  payload, as in XTM. The lead picker is TAM's `AssigneePicker`, which already
  searches Jira and falls back to the user cache offline; it now hands back
  the display name with the username so the cached row can show it.
- **Delete.** As XTM: the dialog offers to move the component's issues to
  another component, or to leave them without it. A draft is discarded at
  once and leaves every issue it was set on. A component Jira holds is queued;
  Commit sends `moveIssuesTo`, then the cache moves the issues as Jira did.
- **Issue count.** From the cache, so the dialog works offline. When the
  profile has a scope JQL the view says the count may be short.
- **Conflicts.** As releases: before an edit Commit reads the project's
  components and compares Jira's copy with the edit's before value. A
  difference fails the row with base, mine and Jira's value per field.
- **Delete supersedes edit.** Deleting a component with a pending edit drops
  the edit, as a release delete does.
- **Shared code.** The id rewrite and the queued-change refusal move into a
  catalog helper both kinds use. In the frontend, XTM's component CSS and its
  assignee type list move to `frontend/core`, and both apps read them from
  there.
- **No live note.** XTM's warning that its component writes skip Commit does
  not apply here; TAM's form says the change waits for Commit instead.

## Tasks

1. Shared catalog id rewrite for versions and components (refactor, covered
   by the existing release tests).
2. Draft components: `CreateDraftComponent`, `EditComponent`, discard of a
   create and of an edit, sync keeps a component with a pending edit.
3. `QueueComponentDelete`, `RekeyComponent`, `ApplyComponentCommitted`,
   `ComponentIssues`, `ComponentCounts`.
4. `backend.ComponentBackend` in the Jira and demo backends.
5. Commit: component creates and edits in the releases phase, deletes in
   closing, issue edits and drafts held on a draft component.
6. Bindings in `app_components.go`, gated on Administer Projects; regenerate
   wailsjs.
7. Frontend api, queries and Pending changes words.
8. Shared CSS and assignee types in core.
9. `ComponentsView`, the form and the delete dialog; nav, menu and tour.
10. User guide chapter, `tam-phases.md`, screenshots, PR.
