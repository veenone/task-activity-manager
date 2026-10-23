# Changelog

All notable changes to **Task Activity Manager** are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The version is single-sourced in `wails.json` (`info.productVersion`).

## [Unreleased]

### Added

**A dashboard from a filter id, and a pointer to Jira's own dashboards (#349)**
- **Filter id** takes the number out of a saved filter's Jira URL and pins that
  filter as a dashboard. Data Center lists only the filters somebody has
  starred, so a filter you wrote and never starred could not be reached from
  the picker at all. The name and the JQL come back from Jira, so a dashboard
  cannot end up labelled one thing and counting another, and an id nobody can
  see adds nothing and says so.
- **Jira dashboard** names the dashboards Jira holds and opens the chosen one
  in your browser. TAM draws none of their contents: Data Center has no route
  that enumerates a dashboard's gadgets, so there is nothing it could count
  without guessing. The row appears only when Jira answers with at least one
  dashboard.

## [1.1.0] - 2026-10-23

A release about finding issues and trusting what the window says. Every view
that lists issues narrows by the same three controls. Epics splits into the
work that sits under an epic and the work that sits under none, so the issues
an epic cannot account for are a tab of their own rather than rows a cap
withheld. Dashboards says which saved filters you wrote and which somebody
shared with you. A failed sync is a sentence across the status bar instead of a
status code in the corner of a footer.

### Added

**Type, status and assignee filters on every issue view (#321)**
- Backlog, Assigned to me, Epics, Boards and Sprints narrow by **Type**,
  **Status** and **Assignee** from one shared control, so the five views cannot
  drift apart on labels, order or behaviour, which is what #321 found them
  doing.
- The status and assignee lists are read from the cached issues, so neither
  offers a value the grid has nothing for. **Unassigned** is a choice of its
  own rather than the absence of one, and it is offered whether or not the
  cache holds an unassigned row.
- Each view keeps its own filters, because the filters belong to the view you
  set them on, and clears them along with its page and selection when you
  switch profile.
- Changing a filter returns to page one, so narrowing never lands you on a page
  that no longer exists.

**Epics splits into the issues under an epic and the issues under none (#341)**
- Epics has two tabs, each labelled with the count of its own side, for example
  `Under epics 2,238` and `No epic 5,988`. Between them they account for every
  cached issue that is not itself an epic. The labels count the whole side, so
  they hold still while a filter narrows what the tab shows.
- **No epic** is a flat list with the same grid, filter bar and pager the
  Backlog has, so an orphaned issue is reachable however many of them there
  are. It was previously only visible through whatever the epic tree had room
  for.

**Dashboards says who wrote each saved filter (#346, #348)**
- The **Saved filter** picker groups the filters you have starred in Jira under
  **Your filters** for the ones you wrote and **Shared with you** for the rest.
  A heading with nothing under it is not drawn.
- The owner is read from each filter in the starred list and compared with the
  username Jira answers `/myself` with. Jira Data Center has no route that
  lists the filters a user owns, so the scope is a per-filter answer rather
  than a per-list one.
- When the owner or the reader cannot be named, which is what a Jira that
  identifies its users another way does, the list stays flat and unlabelled
  rather than guessing a heading.

### Changed

**One dropdown for the issue type filter, not one chip per type (#323)**
- The type filter is a single dropdown reading **All types** until you narrow
  it. A project with five types used to spend five controls on one filter, and
  the view's action buttons were what fell to the second row.
- Several types at once still works, because "stories and bugs" is a real
  question to ask of a backlog.

**The Rituals sidebar reads like the other views (#324)**
- The document list in the left sidebar carries each document's name, whether
  it is synced and when it last was, with a name filter above it, which is the
  treatment the other list views already had.

**The frontend API layer is nine modules instead of one file (#318)**
- `api.ts` was 1,763 lines. Its shapes and bindings live in nine modules under
  `src/api/` (core, issues, boards, reports, edits, pending, importer, rituals,
  dashboards), the largest 365 lines, and `api.ts` re-exports them. No import
  site changed.

### Fixed

**A failed sync says what it means (#315)**
- The status bar showed Jira's raw error, for example `jira: 401 : Unauthorized`:
  an internal step name, a status code with stray punctuation, and on a 404
  Jira's generic "Issue Does Not Exist" for a call that never mentioned an
  issue. Both of TAM's error surfaces now read as sentences naming what to
  change: the token, the Jira URL and project key, or nothing at all when Jira
  itself failed.
- A failure takes the whole status bar as a red ribbon rather than sitting as
  red text beside the health dot, the issue count and the theme label, where it
  read as one more value instead of the one thing to act on. A running sync
  supersedes the previous failure.

**The epic tree no longer loses children to the issues that have no epic (#338)**
- The tree read every non-epic issue in one capped pass, and the issues with no
  epic crowded out the ones under epics through the read they shared. On a
  profile of 8,226 non-epic issues, 38 of the 56 epics with children reported a
  wrong total and most reported none at all, drawing as "0 of 0" and opening
  empty. The two halves are read and capped separately.
- The notice over the no-epic list no longer reports a ratio. "Showing 761 of
  5,988" was true word by word and read as though a cap had withheld 5,227
  issues when it withheld 988 and the **Show done** checkbox hid the rest.

**The ritual cards sit on a ground of their own (#329)**
- A card's fill was its container's fill, so only its border separated the two
  and the card read as bleeding into the page. The list had also lost its
  horizontal padding to a duplicate rule, leaving each card flush against the
  panel border.

**The filter bar stops cutting the bottom off its own text (#323)**
- The `detail-input` controls truncated descenders. The whole class is sized
  from one set of rules now, matching XTM.

## [1.0.0] - 2026-10-20

First release. Backlog, Assigned to me, Epics, Boards, Sprints, Reports,
Dashboards and Rituals, over a local cache of a Jira Data Center project that
holds every edit as a pending change until you commit it. Connection profiles
shared with Xray Test Manager, demo mode, CSV and XLSX import and export,
sprint reports published to Confluence, the sprint ritual pages and the done
agreement, diagnostics, and light and dark themes.

[1.1.0]: https://github.com/veenone/task-activity-manager/releases/tag/tam/v1.1.0
