# TAM kanban flow reports

A project's boards may be scrum, kanban, or both. The Reports view serves
only the scrum ones: it filters the board list to `type === "scrum"` and
every number it prints is scoped to a sprint. A team running kanban opens
Reports, finds its boards missing, and has nothing.

This design adds a second kind of report to the same view. It is not a
second view: the frame, its head and its outputs bar are the ones already
there, and the three publishers write the same document shape.

## What a kanban report answers

Four questions, chosen with the maintainer:

- **Throughput.** How many cards finished each week? The nearest thing
  kanban has to velocity.
- **Cycle time.** Once work starts on a card, how long until it finishes?
  Reported as the 50th and 85th percentiles with the individual cards
  behind them, because a mean is ruined by one outlier and the 85th is the
  figure a team actually quotes.
- **Work in progress against the board's own limits.** How many cards sit
  in each column now, and which columns are over the maximum the board
  sets? This one already exists and is not built again here; see below.
- **Cumulative flow.** How did the cards distribute across the columns, day
  by day, over the window?

## What already exists

Work in progress against a column's limit was built by #105 and is not
rebuilt here. The status to column lookup is `columnIndex` in
`boardrepo/view.go`, the subtask rule the board's `constraintType` selects is
`ColumnView.counts`, the count a limit is measured against is
`ColumnView.Counted`, the over and under rule is `limitBreach` in
`lib/columnLimit.ts`, and `capacitySection` in `lib/reportDocument.ts`
already publishes it as a Column capacity section of the report.

What is missing is only reach. `Report.Capacity` is filled by
`sprintreport/build.go` from a sprint id, and `ReportsView` filters its
picker to `type === "scrum"`, so a kanban board never gets as far as the
section that would serve it.

Anything here that needs a status to column lookup uses the one in
`boardrepo` or an extraction of it. A second copy would be the third
implementation of a rule this codebase already carries two scars about;
`donerule`'s header records the first two.

## Non-goals

Deferred to their own issues, not built thin here: aging work in progress,
flow efficiency (active against waiting time), lead time from creation
rather than from start, per-column cycle time breakdown, and blocked time.
A kanban board's backlog, and anything about Jira versions or releases.

## What scopes a report

A sprint bounds a scrum report. Kanban has no such boundary, so the user
picks a rolling window: the last 30, 60 or 90 days. It takes the slot in
the head that the sprint picker takes on a scrum board, so the head holds
the same number of controls either way, and 30 is the default because the
window sets what the report costs.

That cost is the reason the window is a control rather than a constant.
`sprintreport/fetch.go` pages 25 issues at a time with the changelog
expanded, and its own comment says one sprint can take minutes. A kanban
board has no sprint to narrow the query, so the window is the only thing
that does.

## Architecture

`internal/reports` is pure: issues and a clock in, numbers out, no I/O,
which is what makes its tests cheap. `internal/sprintreport` is everything
that costs something. The kanban side mirrors that split exactly, and the
shared vocabulary is extracted rather than copied.

### `internal/changelog` (new, pure)

Extracted from `internal/reports/series.go`, which is at 398 lines against
the 400 line cap and has no room for anything further.

Moving: `parseChanges` to `Parse`, `change` to `Change`, `civil` to
`Civil`, `daysBetween` to `DaysBetween`, `workingDays` to `WorkingDays`,
`isWorkingDay` to `IsWorkingDay`, `parsePoints` to `ParsePoints`.

Staying in `reports`: `rewound`, `walker`, `inSprint`, `sprintEnd`, `card`,
and everything else that keys on sprint membership or points scope. Those
are not shared and moving them would only spread the sprint model.

The reason for the extraction is the warning `reports`' own package comment
opens with: a changelog is today's values plus a list of deltas, and
reconstructing history forwards from today draws a sprint that never
happened. That trap now has one implementation instead of two.
`donerule`'s header exists to record what two implementations of one rule
already cost this codebase.

### `internal/flow` (new, pure)

```
func Build(
    cols []backend.BoardColumn,
    done func(string) bool,
    issues []backend.IssueHistory,
    from, to, now time.Time,
    loc *time.Location,
) (Flow, error)
```

`Flow` carries:

- `Throughput []Week` with `WeekStart`, `Cards`, `Points`.
- `CycleTimes []Card` with `Key`, `Summary`, `Started`, `Finished`, `Hours`.
- `P50`, `P85 float64`, hours, from `CycleTimes`.
- `Days []ColumnDay`, a `Date` plus a count per column, for the cumulative
  flow chart.
- `Unit`, `UnitReason`, the existing `reports` vocabulary unchanged, so a
  board that estimates nothing counts cards and says why.
- `Truncated []string`, the same honesty the sprint report carries: Jira
  returns a page of an issue's changelog and says how many entries exist,
  so a card whose history came back cut short is named and the figures do
  not claim to be exact.
- `AlgoVersion` of its own, so a stored flow a lower version wrote is
  rebuilt rather than served.

It carries no work in progress figure. That one is `boardrepo`'s and is
already published; see What already exists. What `flow` adds to the column
model is the classification `boardrepo` has no notion of: which column is
the first, which is the last, and which are the working ones between them.

### The column model

A card's column at a time is the column whose `StatusIDs` contains the
card's status at that time. This is the one rule the whole package rests
on, and it has three edges worth writing down.

- A status in no column is off the board. Those cards are excluded rather
  than bucketed into the nearest column: a board whose filter is wider than
  its columns is ordinary, and inventing a column for them would report
  cards the team's own board does not show.
- **Finished** is `donerule.Done`, the board's last column, which is the
  rule the sprint report and the sprint completion already share. It is not
  `backend.IsDone`, which matches on status name; `donerule`'s header
  records why the two differ and that the difference is deliberate.
- **Started** is the first entry into a column that is neither the first
  nor the last. A card that jumps straight from the first column to the
  last has no start, so it has no cycle time and is counted in throughput
  only. A board with fewer than three columns has no working column at all,
  which is the `noWorkingColumns` reason below rather than a report of
  zeroes.

`Constraint`, Jira's `issueCount` against `issueCountExclSubs`, decides
whether a column's count includes subtasks, because it is what Jira's own
board counts and a figure that ignored it would disagree with the number
the same team sees in Jira.

### `internal/flowreport` (new)

Mirrors `sprintreport`. It owns the fetch, the store, the progress frames,
and the reasons a report has nothing to show.

The query is the board's own filter narrowed to the window. Paging advances
by what came back rather than by the page size asked for, and an empty page
ends the loop, which is `sprintreport/fetch.go`'s arithmetic and is correct
against an instance that clamps `maxResults`.

A condition the view renders beside the report travels in the result, not
as a Go error, because Wails fills in a bound method's value or its error
and never both. The reasons:

- `boardNotSynced`, the board's columns were never read, so there is no
  column model to place a card in.
- `noColumns`, the board is synced and has no columns.
- `noWorkingColumns`, fewer than three columns, so nothing distinguishes
  started from finished.
- `noIssuesInWindow`, the window is empty, which is a fact about the window
  and not an empty report.

A failure of the call itself, a refused lock, a transport error partway
through a fetch, stays a Go error. Reasons are constants; the frontend
words them.

### Storage

A `flow_reports` table in `core/store` migrations, keyed by profile, board
and window length, holding the computed `Flow` and its `AlgoVersion`.

It is profile-keyed, so it goes in **both** `PurgeProfile` implementations,
`tam/internal/issuerepo/state.go` and `tam/internal/boardrepo/boardrepo.go`.
The Store schema contract's gate asserts that every profile-keyed table
name appears in both lists, and it fails the build otherwise.

Unlike a sprint, a window has no closed state: the last 30 days always
moves. A stored flow is therefore a cache with a timestamp the view prints,
refreshed on demand, never served as final.

### Bindings

`GetFlowReport(profileID string, boardID, windowDays int, refresh bool)`
and `CancelFlowReport(profileID string)` in `app_reports.go`, which adapts
and does nothing else. Then `wails generate module` from the app directory;
the `wailsjs` tree is never hand-edited.

## Frontend

`ReportsView` stops filtering to scrum. The board picker offers every
board. The second control switches on `board.type`: the sprint picker for
scrum, the window picker for kanban. The head and the outputs bar are
unchanged from #107.

New, under `components/flow/`: `FlowSummary`, `ThroughputChart`,
`CycleTimeChart`, a scatter of the cards behind the percentiles,
`CumulativeFlowChart`, a stacked area, and `WipTable`. `FlowSummary` puts
the window's total and the two percentiles in the existing
`.report-metrics` grid, so the two report kinds read as one design
language.

Every word lives in `lib/flowText.ts`, mirroring `reportText.ts`. No
user-facing string is written in a component and none carries an em dash.

`lib/flowDocument.ts` builds the same `ReportDocument` shape
`reportDocument.ts` builds. This is the load-bearing decision: it means
`ReportOutputs`, the Confluence publisher, the spreadsheet and the deck all
work on a kanban report with no change at all, and a figure reads the same
in the app, on the page, in the sheet and on the slide.

Charts follow the existing contract: every colour a `--chart-*` token,
marks styled by class so a theme change reaches the picture, and each chart
stating its numbers in text beside the marks so none of them is the only
way to read a figure.

## Error handling

The split is `sprintreport`'s: data conditions in the result, call failures
as Go errors. A read refused because the profile's lock is held reuses
`isBusyRefusal` and reads as busy rather than as a failure. A report left
running when the view moves on is cancelled, the way `ReportsView`'s
existing effect cancels a sprint report, because a read holds the profile
against the user's next sync for minutes.

## Testing

`internal/changelog` and `internal/flow` are pure, so they get table tests
over hand-built changelogs, the pattern `reports_test.go` uses. The
extraction is proven by the existing `reports` tests continuing to pass
unchanged.

Cases that must be covered because they are the edges above: a status in no
column; a card that never enters a working column; a board with two
columns; a card that moves backwards; a card finished and reopened inside
the window; a changelog that came back truncated; a board whose columns
carry no limits at all, which is the ordinary case and the reason `Max` is
a pointer (#101).

Every change leads with a failing test (P2), and the proven-red job checks
the red was an assertion rather than a missing symbol.

## Phases

Three PRs under this spec, cheapest and most useful first.

1. **The `changelog` extraction, and the Reports view opening a kanban
   board.** The view stops filtering to scrum, the window picker takes the
   sprint picker's slot, and a kanban board is shown the column capacity
   section that #105 already builds. No new metric, no new table, no Jira
   fetch: capacity reads the synced cache. It proves the view's board-type
   switch, which every later phase needs, and it is the slice that makes a
   kanban team's Reports view stop being empty.
2. **Throughput and cycle time.** Two boundary timestamps per card, and
   where `internal/flow` arrives. It carries only what is new: the first,
   working and last column classification, which `boardrepo` has no notion
   of. The lookup itself comes from `boardrepo`.
3. **Cumulative flow.** The day-by-day reconstruction, the expensive one,
   and the only metric here needing a status for every card on every day
   rather than two moments per card.

## Risks

- **Fetch cost is unmeasured.** `pageIssues` is 25 and its comment says so
  explicitly: the probe that would have timed 50 against 25 on a real
  instance was never run. A 90 day window on a busy board is the first
  thing in TAM to test that, and the window picker is what keeps a user
  from discovering it by accident.
- **The board filter may be wider than the project.** Jira answers the
  board list with every board whose filter mentions the project, so a
  board's own filter can return another team's cards. The off-board rule
  above limits the damage but does not remove it.
- **`Constraint` may be empty** on a board synced before TAM read it, which
  reads as a board whose limits nothing has read rather than a third way of
  counting (#101).
