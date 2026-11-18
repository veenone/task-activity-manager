# TAM Worklog view

Issue #512. Builds on #510, which gave a worklog a chosen start time.

## Why

TAM logs work one issue at a time, from the Work log section of the detail panel. Nothing shows a person their own week, so a day that came in under what they meant to log goes unnoticed, and filling it means opening each issue in turn.

## What it is

A view, tabbed after Assigned to me, that shows a week of the connected user's own time as a grid and puts the Log work form beside it.

- Rows: the user's open assigned issues, then any other issue they logged on in the days shown.
- Columns: Monday to Sunday. A cell is the user's time on that issue and day, Jira's entries plus the pending ones.
- Each workday header measures the day against a day target, in words and with a bar.
- Selecting a cell opens a panel with the form for that issue and day, the day's standing, and every entry the user logged that day.
- Workdays in the last 14 days under the target are listed above the grid. Picking one opens it.

## Goals

- Fill a short day in as few steps as possible.
- Keep past short days in view until they are filled.
- Add no new write path: entries are journalled through `LogWork` and pushed on Commit like any other.

## Not in this version

- Editing or deleting entries Jira holds.
- Other people's time, team totals, approvals.
- Holidays and leave.
- Narrowing rows to the active sprint. A cached issue carries its sprint id but not the sprint's state, so this needs a read the view does not otherwise make.

## Approaches

**Where the entries come from.**

1. Read from Jira each time (chosen). One search, `worklogAuthor = currentUser()` with a date range, returns the issues with their worklogs. An issue with more than the 20 worklogs a search carries is read whole. This matches the Work log section, which also stores nothing.
2. Cache worklogs in SQLite during sync. Faster to draw and readable offline, but it adds a table, a migration, a purge entry and a staleness question for data the user can change in Jira at any time. Not worth it for a view read a few times a day.

**Where the form goes.**

1. In the right-hand detail panel (chosen). It is where every other view puts detail, and the grid stays visible while typing.
2. Inline in the cell. Fastest for a bare duration, but there is no room for the start time and comment, and Started is the point of #510.
3. A modal. Hides the grid, which is the context for choosing an amount.

**The day target.**

1. A per-profile setting, default 8 hours on Monday to Friday (chosen).
2. No target, totals only. Simpler, but then nothing can say a day is short, which is the reason for the view.

## Design

### Reads

`backend.OwnWorklogBackend` is a new optional interface with one method, `WorklogsBy(ctx, author, fromDay, toDay)`. The Jira backend implements it with the search above. Two details matter:

- Jira evaluates `worklogDate` in the zone of the user's Jira profile. The search asks for one day either side of the range, and the range is then applied to each entry's own `started` stamp.
- The search returns every worklog on a matching issue. Entries are kept only where the author is the connected user.

An entry belongs to the day its `started` stamp reads in its own offset, the first ten characters. No instant is converted.

`App.WorklogWeek(profileID, fromDay, toDay)` reads the username a sync recorded (`jira_username`), the user's assigned issues from the cache, the entries from the backend and the pending entries from the journal, and hands them to `worklogweek.Build`, which is pure. A range longer than 62 days is refused.

A row for an issue the cache does not hold is marked `cached: false`. `LogWork` journals against cached issues only, so such a row shows its time and offers no cell to press.

### Frontend

- `WorklogView` owns the week on screen, the selected cell, the filter text and the day target. It runs two reads: the week shown, and the 14 days behind today for the strip.
- `WorklogGrid` draws the table. Cells of cached rows are buttons, and arrow keys move focus between them.
- `WorklogDayPanel` is the form. Started defaults to the end of the day's last entry, or 09:00.
- `lib/worklogWeek.ts` holds the date arithmetic, totals, gap list and day wording, all pure.

Both reads sit under one query-key prefix, which `invalidateWrites` invalidates, so logging or discarding an entry refreshes the grid and the strip.

The day target is the profile setting `worklog_day_hours`.

### Demo mode

The demo backend implements `WorklogsBy` over the same in-memory entries its `Worklogs` reads.

## Left to the builder

Styling within the existing tokens and primitives, and the wording of empty and error states.
