# TAM releases and components, PR 3: the Releases view

**Goal:** A user lists the project's releases with their dates, status and
progress, sees the issues in one, and creates, edits, releases, archives,
unarchives and deletes releases. Each of those is a pending change that
Commit sends. Releasing asks what happens to each unfinished issue.

**Architecture:** Five journal entity types modelled on the sprint ones
(`version_create`, `version_edit`, `version_release`, `version_archive`,
`version_delete`). A drafted release is a `project_version` row with a
negative id, the scheme sprints use. The Release dialog's carry-overs and
descopes are ordinary `fixVersions` issue edits journaled in the same
transaction as the `version_release` row. Commit gains two phases: a
releases phase after sprint changes (creates, then edits) and a closing
phase at the end (releases, archives, deletes). The backends gain an
optional `backend.VersionBackend`; the committer type-asserts it, as it does
`sprintCreator`.

**Spec:** `docs/superpowers/specs/2026-11-23-tam-releases-components-design.md`
(issue #605), Delivery item 3. Components (PR 4) are out: no component
journal types, no Components view.

## Global constraints

- Worktree `C:\projects\tam-605-3`, branch `feat/605-releases-view`. Commits
  say `Refs #605`, no closing keywords, no AI attribution.
- Run `go vet ./internal/...` and `go test` on the touched packages only, and
  the touched Vitest files. `npm run lint` once before the PR.
- Test first (P2): each behaviour's test is seen failing on an assertion.
- Files under 400 lines. New logic goes in new files.

## Decisions where the spec leaves room

- **Progress.** The view is project-wide and the done rule
  (`internal/donerule`) needs a board's columns. Progress counts an issue
  done when its status category is done, falling back to `backend.IsDone`
  on the status name when the row has no category. The Release dialog's
  "unfinished" is the complement, so the bar and the dialog agree.
- **Conflicts.** The conflict card is built around issue keys and their two
  resolutions. A release edited or released in Jira since the cache read it
  is reported as a failure that names base, mine and Jira's value per field
  and keeps the row; discarding it and syncing takes Jira's copy.
- **Release of a draft.** Refused ("Commit this release first"), the rule
  sprint completion follows. Archive is refused on a draft too.
- **Local effect.** An edit changes the cached row at once (sprint edits do
  the same). Release, archive and delete change nothing until Commit; the
  row wears "Releasing on Commit", "Archiving on Commit",
  "Unarchiving on Commit" or "Deleting on Commit".
- **Delete supersedes edit**, as for sprints. A pending release or archive
  refuses a delete until discarded.
- **Release held.** The closing phase holds a release while any `fixVersions`
  edit that takes an issue out of that release is still in the journal
  (failed, conflicted or held). Out-of-scope unfinished issues found at push
  time go with `moveUnfixedIssuesTo` set to the dialog's set-all target; with
  no target the release is held and the reason names them.
- **Version picker sort.** Release date, then name, the store's order.
- **Where closing runs.** After links and before attachments and worklogs,
  which wait on nothing, so worklogs stay the last phase and every issue
  write still lands before a release.

## Tasks

1. `permission.AdministerProjects` ("Administer Projects") and its frontend
   constant.
2. Draft releases: `CreateDraftVersion`, `EditVersion` (draft or real),
   discard of a create and of an edit, `nextDraftID` on `project_version`.
3. Queue release, archive and delete of a real release; the release queues the
   per-issue edits with it in one transaction.
4. `RekeyVersion` and the cache work after each push.
5. `backend.VersionBackend` in the Jira and demo backends.
6. Commit phases: releases (creates, edits) and closing (releases, archives,
   deletes), with holds and the out-of-scope check.
7. Bindings in `app_releases.go`, gated on Administer Projects, plus
   `ListReleases` (with counts) and `ReleaseIssues`; regenerate wailsjs.
8. Frontend: api and queries, nav and menu entry, `ReleasesView`, the release
   form, the Release dialog, the delete dialog.
9. Pending changes words and the Commit banner line for release changes.
10. User guide chapter, getting-started list, `tam-phases.md`, screenshots,
    PR.
