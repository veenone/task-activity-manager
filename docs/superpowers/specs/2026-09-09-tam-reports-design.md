# Task Activity Manager, Phase 4: reports

Decided 2026-09-09, after Phase 3 closed and the sprint lifecycle merged. The user pre-approves the recommended option at every decision point, so this records the choices and their reasons rather than the alternatives.

## 1. What this phase delivers

The Reports view: a burndown for a sprint, a velocity chart across the last closed sprints, and the sprint summary a review actually reads. Phase 3 made TAM run a sprint; this is what a team looks at afterwards.

One spec, one plan.

## 2. The problem that shapes everything else

**A burndown is a history, and TAM has none.** The issue cache is a snapshot: it knows a card is Done today and nothing about when it got there. The `sprint` table knows a sprint's dates and state. The journal knows what *this user* changed locally and nothing about the rest of the team. Nothing in TAM can say what the sprint looked like last Tuesday.

Four ways to get that history, and the choice decides the phase:

| Source | Verdict |
|---|---|
| Jira's own chart endpoints (`/rest/greenhopper/1.0/rapid/charts/...`) | Rejected. They are Jira Software's internal API: undocumented, unversioned, and the one thing in this app that would break on an upgrade with no warning and no recourse |
| TAM's journal and audit log | Rejected. It records what this user did on this machine, so a burndown drawn from it would be a chart of one person's afternoon |
| Snapshot the sprint daily from now on | Rejected as the primary source. It only works for sprints that start after the user installs TAM, only on days the app was open, and it cannot draw the sprint that finished last week |
| **The issue changelog** | **Chosen.** `/rest/api/2/search` with `expand=changelog` returns every field change with its timestamp, one page of issues at a time. It is documented, it is public API, it works retroactively on a sprint that closed months ago, and it carries the Sprint field's own changes, which is what makes scope changes visible rather than invented |

So: a report is reconstructed from the changelog, not accumulated. This is the decision the rest of the phase hangs on.

## 3. Decisions

| Question | Decision | Why |
|---|---|---|
| Where history comes from | The issue changelog, fetched with the sprint's issues in one paged search | Section 2 |
| When it is fetched | When a report is opened, not on every sync | A changelog expansion is a heavy read, and a sync already takes minutes on a real project; a report the user has not asked for should not pay for it |
| Where it is kept | A new `sprint_report` table holding the reconstructed series per sprint | A closed sprint's history cannot change, so it is fetched once and then belongs to the user offline, which is the whole point of this app |
| A live sprint's report | Recomputed on demand, never cached | It changes every hour, and a stale burndown is worse than a slow one |
| What is measured | Story points, falling back to issue count when the sprint has no points at all, and the chart says which | A board that does not estimate is common, and a burndown of nothing is worse than a burndown of cards |
| What "done" means | The same rule the board uses: a status id in the board's last column | One definition across the app; the sprint lifecycle already made it explicit |
| Scope changes | Drawn, not hidden: a card added mid-sprint raises the line, one removed lowers it, and both are marked | A burndown that silently absorbs scope change is how a team argues about a chart instead of about the work |
| Velocity | Committed and completed points per closed sprint, for the last six, from the same reconstruction | Committed means what was in the sprint when it started, which only the changelog can answer |
| The charts | Inline SVG, drawn by hand, no charting dependency | The reuse ladder: two chart shapes do not justify a library, and the app has four runtime dependencies today |
| Where it lives | The Reports view, with a board and sprint picker like the Boards view's | The report is about a sprint, and the sprint pickers already exist |

## 4. The wire

`core/jira` gains one thing: `SearchIssues` learns an `expand` parameter, so a caller can ask for `changelog` alongside the fields it already requests. Nothing else is added; the sprint list, the board configuration and the issue search are all already there.

The changelog arrives as `histories[]`, each with a `created` timestamp and `items[]` naming the field, its from and its to. Two fields matter: the status field, which gives the moment a card became done or stopped being done, and the Sprint field, which gives the moment a card entered or left the sprint.

## 5. The reconstruction

`tam/internal/reports` turns a sprint's issues plus their changelogs into a series:

```
Series{
  SprintID int; Name string; Start, End, Complete string
  Unit string            // "points" or "cards"
  Days []Day             // one per day from start to end, inclusive
  Committed, Added, Removed, Done float64
}
Day{ Date string; Remaining, Ideal, Added, Removed float64 }
```

The walk is: take the sprint's issues, replay each changelog forward from the sprint's start, and record for every day the total still open. A card that entered the sprint on day four adds its estimate on day four; one that left subtracts it. An estimate that changed mid-sprint takes effect on the day it changed, because a team that re-estimates has changed the work, not the history.

The ideal line runs from the committed total on day one to zero on the last day, over working days only.

`Velocity` is the same reconstruction over the last six closed sprints of a board, reporting committed and completed for each.

## 6. The view

Reports gets a board picker, a sprint picker, and two charts stacked with the summary between them.

- **The burndown**: the remaining line, the ideal line behind it, scope changes marked on the days they happened, and the axis labelled in the unit being measured. A live sprint draws a vertical line at today and stops the remaining line there rather than pretending to know the future.
- **The summary**: committed, added, removed, completed, and what carried over, in the same sentence a sprint review starts with.
- **Velocity**: a bar per closed sprint, committed against completed, with the mean drawn across.

Empty states: a board with no closed sprint says so; a sprint with no estimates says the chart is counting cards; and a sprint whose changelog could not be read says which sprint and offers a retry, because the alternative is a chart that looks authoritative and is wrong.

## 7. Errors

A changelog fetch that fails leaves the previous cached report on screen with the failure named, and never a half-built series. A sprint whose start or end date Jira does not carry cannot be charted, which is stated rather than guessed at. A 403 on the search says the account cannot read that project's issues.

## 8. Verification

Go: the changelog expansion against the httptest server, including paging; the reconstruction over a sprint with a card added mid-sprint, one removed, one re-estimated, one that finished and reopened, and one with no estimate; the working-day ideal line; the fallback to counting cards; velocity over six sprints and over none. Vitest: both charts render from a fixture, the summary reads its sentence, the pickers switch, a live sprint stops at today, and each empty state. Offline: on the demo profile, open Reports for the closed sprint and read the burndown, then the velocity chart.

## 9. Out of scope

A cumulative flow diagram, a control chart, an epic burndown, cross-project reporting, exporting a chart as an image, and any report that is not about a sprint. Rituals, which is Phase 5, is where a report gets written up and published to Confluence.
