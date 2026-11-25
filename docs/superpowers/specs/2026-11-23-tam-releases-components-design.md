# TAM releases and components

Issue: #605

## Why

Project and delivery managers use TAM to plan. Two things they do in Jira
today have no home in TAM:

- **Releases.** A release is a Jira fix version. Managers plan what ships in
  each one, follow how far along it is, and close it when it ships. Following
  agile practice, work that is not done when a release closes either carries
  over to the next release or is descoped.
- **Components.** Each project has a predefined set of components, and
  managers group issues by them. They keep the set up to date and file issues
  under the right ones.

## Goals

- A **Releases** view: every fix version of the project with its dates,
  status and progress, and the issues in the one selected.
- Create, edit, release, archive, unarchive and delete a release.
- Release a version with a decision for each unfinished issue: carry it over
  to another unreleased version, or descope it.
- A **Components** view: every component with its lead and issue count, and
  the issues filed under the one selected.
- Create, edit and delete a component.
- Set an issue's components and fix versions from the issue panel, from New
  issue, and in bulk from the Backlog selection bar.

## Non-goals

- Release reports, burndown charts or release notes.
- Affects versions. Only fix versions are in scope.
- Releases that span several projects. A release belongs to the profile's
  project, as in Jira.
- Merging versions, or reordering them by drag.
- Archiving components.
- Grouping the Backlog table by component. The Components view does the
  grouping for now.

## Anti-goals

These would make the feature a failure even if it works:

- Anything reaching Jira before Commit. TAM's rule is that nothing does, and
  releases and components follow it like sprints do.
- Jira showing a released version that still holds unfinished issues.
- A second way of managing components that looks different from XTM's.
  TAM matches XTM's layout, classes and copy.

## Approaches considered

1. **Everything through Commit (chosen).** Creating, editing, releasing,
   archiving and deleting releases and components are pending changes, as
   sprint drafts, starts and completions are today. Releasing queues the
   carry-overs, the descopes and the release itself, and Commit sends them in
   that order. Consistent with the rest of TAM, and a release stays undoable
   until Commit. It costs journal entity types, discard handling and conflict
   checks for two new kinds of entity.
2. **Assignments through Commit, admin straight to Jira.** XTM's way: release
   and component changes go to Jira on click. Less code, but TAM gains a
   second rule, and a release would happen at once while its carry-overs wait
   for Commit, so Jira would show a released version holding unfinished work.

The user picked 1.

## Design

### Shared Jira client (core)

The Jira calls live in `core/jira`, which both apps already use.

- Move `xtm/internal/jira/components.go` into `core/jira` (component list
  with details, create, update, delete with "move issues to", related issue
  counts, user search). XTM keeps working through the embedded client.
- New `core/jira/versions.go`: list with details
  (`GET /rest/api/2/project/{key}/versions`), create (`POST /version`),
  update (`PUT /version/{id}`, also used to release, archive and unarchive by
  setting `released`, `releaseDate` and `archived`), delete
  (`DELETE /version/{id}`, with `moveFixIssuesTo` when asked).
- Demo data for both, in TAM's demo backend, with in-memory writes like
  `demoSprints`.

### Storage (tamstore, schema v28)

- `project_version`: profile_id, project, id, name, description, start_date,
  release_date, released, archived, draft, PK (profile_id, id). A draft has a
  negative id, the same scheme as sprints (`nextDraftID`).
- `project_component`: profile_id, project, id, name, description, lead_name,
  lead_display_name, assignee_type, draft, PK (profile_id, id).
- `issue.components` and `issue.fix_versions`: JSON arrays of ids, so a
  rename does not touch issues. Names are read through the two tables.
- Both tables join `PurgeProfile` in `issuerepo/state.go`. The instruction
  gate checks the list.
- The migration forces the next sync to be full, the same way changing the
  requirement issue type does, so existing caches gain the two issue fields.

### Sync

- Add `components` and `fixVersions` to `baseFields`.
- Each sync also refreshes the project's versions and components. Drafts and
  rows with a pending change are left alone, as sprints are.

### Journal and Commit

New entity types, each with one field and a JSON payload in `after_val`,
modelled on the sprint ones:

| Entity type | Payload |
|---|---|
| `version_create` | the draft version |
| `version_edit` | changed fields; `before_val` holds the cached version |
| `version_release` | release date |
| `version_archive` | archived true or false |
| `version_delete` | optional version to move fix issues to |
| `component_create` | the draft component |
| `component_edit` | changed fields; `before_val` holds the cached component |
| `component_delete` | optional component to move issues to |

Issue assignments are ordinary issue edits on the fields `components` and
`fix_versions`, journaled like `labels` (a comma string of ids) and pushed by
`commitEdit` as `[{"id": ...}]` arrays.

Commit phases (`committer/phases.go`):

1. A new **releases and components** phase after sprint changes and before
   epics: creates, then edits. A created draft is rekeyed everywhere its
   negative id appears, issue edits included, as `RekeySprint` does.
2. Issue edits run where they run today. An edit that names a draft that was
   not created is held, as a sprint start is.
3. A new **closing** phase at the end: releases, archives, then deletes. A
   release goes last so the carry-overs and descopes before it have landed.
   If any of them failed or conflicted, the release is held and the pending
   dialog says why.

Issues outside TAM's scope. The cache only holds issues the profile's scope
JQL matches, so the version can hold unfinished issues TAM never saw. Just
before a release, Commit asks Jira for the version's unresolved issues
(`fixVersion = {id} AND statusCategory != Done`). Any it finds that the
Release dialog did not cover go with the release call's `moveUnfixedIssuesTo`
set to the carry-over version the user picked in the set-all control. When
the user descoped everything and named no carry-over version, the release is
held and the pending dialog lists those issues, so Jira never shows a released
version holding unfinished work. The Release dialog says up front when the
scope JQL may hide some of the version's issues.

Discard follows the sprint pattern: a create drops the draft, an edit
restores the cached fields, and release, archive and delete are removed.

Conflicts: before an edit, release or delete, Commit reads the version or
component from Jira and compares it with the cached copy. A difference shows
in the existing conflict card (Base, Mine, Remote).

### Permissions

Managing versions and components needs Jira's **Administer Projects**.
Setting a fix version needs **Resolve Issues** on top of **Edit Issues**. Add
both to the gated list in `tam/internal/permission/permission.go`, with their
display names, and gate the actions in Go and in the UI the way #516 gates
sprint actions.

### Releases view

New view after Sprints, `gated: true`, with no number accelerator (1 to 9 are
taken). Built from the Sprints view's parts.

```
Releases       [Unreleased ▾]                [New release] [⋯]
 v2.4  Unreleased  1 Dec → 15 Dec   ███████░░░  31 of 44 done
 v2.3  Released    15 Nov           ██████████  40 of 40
 ── v2.4 ────────────────────────────── [Edit] [Release…] [⋯]
 Key      Summary               Status   Component  Assignee
 PLAT-12  Export to CSV         Done     Reports    A. Lee
 PLAT-19  SSO login             In prog  Auth       —
```

- Filter: Unreleased (default), Released, Archived, All.
- Progress counts done issues by the profile's done rule
  (`internal/donerule`), the same rule the board uses.
- A row with a pending change shows the change ("Releasing", "Draft"), as a
  sprint does.
- The issue table opens issues in the issue panel and keeps view state
  (`useViewState`).

**Release dialog.** Lists the version's unfinished issues. Each has a choice:
carry over to an unreleased version (a picker; defaults to the next one by
release date) or descope. Descope removes the closing version from the issue's
fix versions and nothing else: the issue keeps its sprint, status and other
fix versions, and goes back to the unplanned pool. One control sets all rows
at once. The release date
defaults to today. When no other unreleased version exists, the dialog offers
New release inline, and descope stays available. Confirming queues the issue
edits and `version_release`.

### Components view

Ported from XTM's Components view with the same layout, classes and copy;
shared parts move into `frontend/core`. It adds the selected component's
issue list, opening issues in the issue panel. Delete asks where to move the
component's issues, as XTM does.

### Issue fields

- Issue panel: **Components** and **Fix versions** multi-select fields. Drafts
  are listed with the cached rows. A released or archived version is shown
  but cannot be newly picked.
- New issue: the same two fields, optional.
- Backlog selection bar: **Set release** and **Set components**, each queuing
  one edit per selected issue. Adding to or replacing the existing values is
  a choice in the control.

The shared multi-select in core has open accessibility findings from the
2026-11-23 audit (no expanded state, no Escape, placeholder-only label). They
get fixed before these fields use it.

### Docs

- A Releases and components chapter in the TAM user guide, and the getting
  started list of pending change kinds updated.
- `agents/project/glossary.md`: "release" means a Jira fix version.

## Delivery

Four PRs, each usable on its own:

1. Core client (components moved, versions added), schema v28, sync, demo
   data.
2. Issue fields: panel, New issue, Backlog bulk set, journal and commit of the
   two fields.
3. Releases view with the release lifecycle and the Release dialog.
4. Components view.

## Calls made in design

- "Unfinished" means Jira status category is not Done, so the dialog needs no
  board picker. Progress bars still use the done rule.
- An issue in several fix versions only loses or swaps the closing one.
- Issues from other projects show the fields read-only, and bulk set skips
  them.
- Counts come from cached issues, with a note when the scope JQL limits them.
- Out-of-scope unfinished issues are caught by Commit at release time, not by
  loading the dialog live, so releasing works offline like every other change.
- Descope leaves the sprint alone; sprint planning stays in the Sprints view.

## Left to the builder

- Table and column names beyond those above, and file layout within the
  domain folders.
- How the version picker sorts (release date, then name, is a fine default).
- Exact wording, following the UI copy contract (no em dashes) and XTM's copy
  for components.
