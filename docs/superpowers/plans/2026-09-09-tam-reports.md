# Task Activity Manager Phase 4: reports

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan runs under the lean cycle: implementers build and write the tests named here but do not run suites, except one Go suite run at the end of Task 3. Task 5 runs every gate once, then one fix wave.

**Goal:** a sprint's burndown, a board's velocity, and the summary a sprint review starts with, all reconstructed from Jira's changelog rather than invented, and readable offline once fetched.

**Succeeds when** a scrum master opens Reports the morning after a sprint closes and gets the three things a review needs, including an honest account of what was added and removed mid-sprint, without opening a browser.

**Architecture:** `core/jira`'s search learns an `expand` parameter so a caller can ask for the changelog. A new `tam/internal/reports` replays those changelogs into a day-by-day series and a velocity table. A new `sprint_report` table keeps a closed sprint's series forever, because a closed sprint's history cannot change; a live sprint is recomputed every time. The Reports view draws two charts in hand-written SVG, with no charting dependency.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

**Spec:** [`../specs/2026-09-09-tam-reports-design.md`](../specs/2026-09-09-tam-reports-design.md).

## Global Constraints

- Go modules stay `agile-suite/core`, `agile-suite/xtm`, `agile-suite/tam`; run Go commands from inside the module directory. Nothing under `xtm/` changes.
- **A report is reconstructed from the changelog, never from Jira's internal chart endpoints.** `/rest/greenhopper/1.0/rapid/charts/...` would be quicker and is undocumented, unversioned Jira Software internals: the one thing in this app that could break on an upgrade with no warning and no recourse. The public search with `expand=changelog` is the source.
- **A chart never invents a number.** A sprint with no dates cannot be charted and says so; a changelog that could not be read leaves the previous report on screen with the failure named; a live sprint stops its line at today rather than drawing the future.
- Scope changes are drawn, not absorbed. A card added mid-sprint raises the line on the day it arrived.
- "Done" is the same rule the board and the sprint completion use: a status id in the board's last column. That rule now has one home on each side, `lib/unfinished.ts` on the frontend and the completion's own status set in `internal/sprints/guards.go`. Call one of them; do not write a third.
- A closed sprint's series is cached and never refetched. A live sprint's is never cached.
- No charting dependency. Two chart shapes do not justify one, and this frontend has four runtime dependencies.
- Reports are read-only. Nothing in this phase writes to Jira or to the journal.
- The PAT stays in the Jira client's Authorization header only.
- Files stay small and single purpose; a helper used from two places lives in its own module. TAM mirrors XTM's design language where XTM has a counterpart.
- UI text uses no em dashes. No AI attribution or mentions anywhere, in code, comments, commit messages, or documents. Conventional commit prefixes, no trailers. Never add, commit, or delete untracked local tooling files; revert Wails churn under `tam/frontend/wailsjs/runtime` and `tam/frontend/package.json.md5` with `git checkout --`.

## What shipped after this plan was written

This plan was drafted the day Phase 3c merged. The sprint work that followed changed four things
it assumes, and each one is folded into the tasks below rather than left for an implementer to
trip over:

- **Schema version 7 is taken.** It added a sprint's `goal`. The `sprint_report` table is version 8.
- **The Reports view already exists in the navigation.** It has a `VIEWS` entry, a blurb and the
  native menu item at accelerator 5, and it renders a `Placeholder`. This phase replaces what it
  renders; it does not add a view.
- **The Sprints view exists**, with a board picker that hides itself when the profile has one scrum
  board, an assignee grouping, and a read (`BoardSprintDetails`) that returns a board's sprints
  with their issues and four computed numbers. Reports should borrow that picker's behaviour, and
  should check whether that read answers its sprint picker before adding a third way to list
  sprints.
- **A closed sprint has no cached membership**, by design: the boards sync fetches issue keys for
  active and future sprints only. That is not a problem for this phase, because a report is built
  from a changelog search by JQL rather than from the cache, but it does mean the sprint picker
  must come from the `sprint` table and not from anything membership-shaped.

Two lessons from the branch that just shipped are worth carrying, because both cost a fix wave
there:

- **A count read from a cache can understate silently.** A sprint's cached membership can be
  capped, or hold keys whose issues were never synced. Anything this phase reports as a total must
  either come from the changelog search, which is authoritative, or say that it might be low.
- **A comment must be true.** Eight consecutive reviews on the previous branch each found at least
  one that was not, including one that credited the wrong mechanism for a safety property and one
  that described protection the code did not provide.

## Decisions

1. **The changelog is fetched with the issues, not per issue.** One paged search with `expand=changelog` brings back a page of issues and their whole history together. Per-issue fetching would be one call per card and would make a report unusable on a sprint of any size.
2. **A report is fetched when it is opened, not on every sync.** The boards pass already takes minutes on a real project, and a user who never opens Reports should not pay for it.
3. **A closed sprint's series is stored; a live one is not.** A closed sprint's history cannot change, so it is fetched once and then works offline forever. A live sprint changes hourly, and a stale burndown is worse than a slow one.
4. **An estimate that changed mid-sprint takes effect on the day it changed.** A team that re-estimates has changed the work, not the past, and back-dating a new estimate to day one would draw a sprint that never happened.
5. **The ideal line runs over working days.** A fortnight with a weekend in it is eight or nine days of work, and an ideal line that slopes through Saturday makes every team look behind on Monday.

## File structure

**Created:** `tam/internal/reports/reports.go`, `series.go`, `velocity.go`, and their tests; `tam/app_reports.go`, `app_reports_test.go`; `tam/frontend/src/components/ReportsView.tsx`, `BurndownChart.tsx`, `VelocityChart.tsx`, `SprintSummary.tsx`; `tam/frontend/src/lib/chartScale.ts`, `chartScale.test.ts`; `tam/frontend/src/queries/reports.ts`.

**Modified:** `core/jira/issues.go`, `issues_test.go`; `tam/internal/backend/backend.go`, `backend/jira/jira.go` or its issue file, `jira_test.go`, `backend/demo/demo.go`, `demo_test.go`; `tam/internal/tamstore/tamstore.go`, `tamstore_test.go` (the `sprint_report` table at schema version 8); `tam/internal/boardrepo/` (the reader and writer for it); `tam/app.go`; `tam/frontend/wailsjs/**` (regenerated); `tam/frontend/src/api.ts`, `queries/keys.ts`, `App.tsx`, `App.test.tsx`, `nav.ts`, `App.css`; `frontend/core/styles/primitives.css`; `tam/CLAUDE.md`, `README.md`.

---

### Task 1: The changelog on the wire

**Files:** modify `core/jira/issues.go`, `issues_test.go`, `tam/internal/backend/backend.go`, `backend/jira` (the issue search), `jira_test.go`, `backend/demo/demo.go`, `demo_test.go`.

**Produces:** `SearchIssues` gains an `expand []string` parameter; `jira.RawIssue` gains `Changelog` with `Histories []RawHistory`, `RawHistory{Created string; Items []RawHistoryItem}`, `RawHistoryItem{Field, FieldID, From, FromString, To, ToString string}`; on `IssueBackend`: `SearchIssuesWithHistory(ctx, jql string, startAt, maxResults int) ([]backend.IssueHistory, int, error)`; `backend.IssueHistory{Issue Issue; Changes []Change}`, `backend.Change{At, Field, From, To string}`.

- [ ] **Step 1: The tests.** Extend `issues_test.go`'s httptest server to answer a search carrying `expand=changelog` with two pages, each issue holding histories with several items. Cover: the expand reaching the query string only when asked for, so every existing caller's request is byte-for-byte unchanged; the histories decoding in order; an issue with no changelog decoding to an empty slice rather than a nil panic; and paging carrying the changelog on both pages. Assert on the request's query string, not only the response.

- [ ] **Step 2: The call.** Add the parameter and the raw shapes. Every existing caller passes nil, which must produce exactly the request it produces today: this call is on the sync's hot path and a changed query string is a changed sync.

- [ ] **Step 3: The seam.** `SearchIssuesWithHistory` on `IssueBackend`, mapping the raw histories into `backend.Change` with the field name normalised (Jira calls the sprint field `Sprint` and the status field `status`; both arrive with a `fieldId` on Data Center, which is the more reliable of the two). The Jira backend maps; the demo backend answers from a curated history over its own dataset, so the offline walk-through draws a real chart. Test both.

- [ ] **Step 4: Commit** as `feat(core): the issue search can ask for a changelog`.

---

### Task 2: The reconstruction

**Files:** create `tam/internal/reports/reports.go`, `series.go`, `velocity.go`, `series_test.go`, `velocity_test.go`.

**Produces:** `reports.Series` and `reports.Day` as the spec names them; `reports.Build(sprint backend.Sprint, done func(statusID string) bool, issues []backend.IssueHistory) (Series, error)`; `reports.Velocity(sprints []backend.Sprint, per map[int][]backend.IssueHistory, done func(string) bool) ([]VelocityRow, error)`; `VelocityRow{SprintID int; Name string; Committed, Completed float64}`.

- [ ] **Step 1: The walk.** `Build` replays each issue's changes forward from the sprint's start. For every day from start to end inclusive it records what was still open. Four rules, each with its own test:
- A card in the sprint at the start counts from day one; one whose Sprint change added it later counts from that day, and into `Added`.
- A card removed from the sprint stops counting on that day, and into `Removed`.
- A card whose status entered the done set stops counting from that moment; one that came back out counts again. A card can cross more than once and the last crossing before the end of a day is what that day records.
- An estimate that changed mid-sprint applies from the day it changed, never backwards.

- [ ] **Step 2: The unit.** Points when any issue in the sprint carries an estimate, cards when none does, named in `Series.Unit` so the view can label the axis. A sprint where some cards have points and some do not counts the ones that do and says so in the summary, because silently treating an unestimated card as zero is how a burndown lies.

- [ ] **Step 3: The ideal line.** From the committed total on the first day to zero on the last, dropping only on working days, so a weekend is flat. State the definition of a working day in the doc comment: Monday to Friday, no holiday calendar, because Jira does not carry one and inventing one per team is worse than a stated simplification.

- [ ] **Step 4: A live sprint.** `Build` fills days up to today and no further, and the caller can tell where the data stops. The view draws that as a line that ends rather than a line that reaches zero.

- [ ] **Step 5: Velocity.** Committed is what was in the sprint when it started, which the same replay answers; completed is what was done when it closed. Six sprints, oldest first, and a board with fewer returns what it has.

- [ ] **Step 6: Tests.** Every rule above, plus: a sprint with no dates returning an error rather than a chart; a card added and removed in the same sprint; a card done before the sprint started; an issue whose changelog is empty; and a velocity over zero closed sprints.

- [ ] **Step 7: Commit** as `feat(tam): reconstruct a sprint's history from its changelog`.

---

### Task 3: The store and the bindings

**Files:** modify `tam/internal/tamstore/tamstore.go`, `tamstore_test.go`, `tam/internal/boardrepo/` (a reader and writer for the new table), `tam/app.go`; create `tam/app_reports.go`, `app_reports_test.go`; regenerate `tam/frontend/wailsjs/**`.

**Produces:** schema version 8 with `sprint_report(profile_id, sprint_id, unit, built_at, series_json, PRIMARY KEY (profile_id, sprint_id))`; `boardrepo.SavedReport` and `SaveReport`; bound methods `GetBurndown(profileID string, boardID, sprintID int) (reports.Series, error)` and `GetVelocity(profileID string, boardID int) ([]reports.VelocityRow, error)`.

- [ ] **Step 1: The table.** Version 7 adds it through `baseDDL`, which needs no migration because it is a new table, and the existing version tests get a case for it the way version 6 did.

- [ ] **Step 2: The read path.** `GetBurndown` reads the cached series when the sprint is closed and one is stored; otherwise it fetches the sprint's issues with their changelogs, builds the series, stores it when the sprint is closed, and returns it. The board is needed for the done rule, so the binding takes it. Both bindings run under `a.acquire(p.ID, "report")`, because a changelog fetch is a long read against Jira and the app already serialises those per profile.

- [ ] **Step 3: Tests, then the one Go suite run.** `app_reports_test.go`: a closed sprint served from the cache without touching the backend; a live sprint never cached; a fetch failure leaving any previous cached series intact; the guard refusing a second report while one runs. Then, inside `tam/`: `go build ./... && go vet ./... && go test ./... -count=1`, and inside `core/`: `go test ./jira/ -count=1`. Fix what fails, rerun at most twice, and report. Commit as `feat(tam): serve a sprint's burndown and a board's velocity`.

---

### Task 4: The view

**Files:** create `tam/frontend/src/components/ReportsView.tsx`, `BurndownChart.tsx`, `VelocityChart.tsx`, `SprintSummary.tsx`, `tam/frontend/src/lib/chartScale.ts`, `chartScale.test.ts`, `queries/reports.ts`; modify `api.ts`, `queries/keys.ts`, `App.tsx`, `App.test.tsx`, `nav.ts`, `App.css`, `frontend/core/styles/primitives.css`, and `ReportsView.test.tsx`.

- [ ] **Step 1: The scale.** `chartScale.ts` is pure: given a series and a pixel box it returns the point arrays, the ticks and the label positions. No React, no DOM, and it is the part worth testing directly. Both charts use it.

- [ ] **Step 2: The burndown.** Hand-written SVG: the remaining line, the ideal line behind it in a muted stroke, a marker on each day scope changed, and axes labelled with the unit `Series.Unit` names. A live sprint draws a today line and stops. Every colour comes from a token that exists in `tokens.css`; check before using one, since there is no `--surface-1` in this design system.
- [ ] **Step 3: The summary.** Committed, added, removed, completed and carried over, as the sentence a review starts with, plus the note when some cards had no estimate.
- [ ] **Step 4: Velocity.** A bar pair per sprint, committed against completed, with the mean across, oldest on the left.
- [ ] **Step 5: The view.** A board picker and a sprint picker following the Sprints view's rather than the Boards view's, which means hiding the board picker when the profile has exactly one scrum board and naming the board in the heading instead, the three pieces stacked, and the empty states the spec names: no closed sprint on this board, no estimates so the chart counts cards, no dates so it cannot be charted, and a failed changelog read that keeps the last chart and names the failure with a retry.
- [ ] **Step 6: Wire it in.** `App.tsx`'s switch gains Reports in place of its `Placeholder` fall-through, and `nav.ts`'s blurb stops promising what this does not do. The `VIEWS` entry and the native menu item already exist and already carry accelerator 5, so nothing is added there and nothing is renumbered. `App.test.tsx` mocks the two new bindings.
- [ ] **Step 7: Tests.** `chartScale.test.ts` for the arithmetic including a zero-range series and a single-day sprint. `ReportsView.test.tsx`: both charts render from a fixture; the summary reads its sentence; a live sprint stops at today; the unit label follows `Series.Unit`; each empty state; and a failed fetch keeping the previous chart. Commit as `feat(tam): the Reports view`.

---

### Task 5: Docs, the single gate run, and the fix wave

- [ ] **Step 1: Docs.** `tam/CLAUDE.md` gains a "Phase 4: reports" section saying where a report's history comes from and why not the internal chart endpoints, what is cached and what never is, what a working day means, and that a report never invents a number. Update Status and Layout. `README.md` gains a sentence. Reconcile the spec with what shipped, saying which side changed.

- [ ] **Step 2: Every gate, once.**

```bash
cd core && go vet ./... && go test ./... -count=1 && cd ..
cd xtm && go vet ./... && go test ./internal/... -count=1 && cd ..
cd tam && go vet ./... && go test ./... -count=1 && cd ..
npm run typecheck --workspaces --if-present
npm test --workspaces --if-present
cd tam && wails build && cd ..
git status --short --untracked-files=no
```

Record each result, fix what fails, rerun only what failed. Do not lower an assertion to make a test pass. Vitest before this plan: 46 in `frontend/core`, 159 in `xtm`, 270 in `tam`.

- [ ] **Step 3: The walk-through for the user** (not run by agents): on the demo profile open Reports, read the closed sprint's burndown and the velocity chart. Then on a real Data Center, the two things no fixture proves: a sprint whose cards were added and removed mid-flight, to see the scope line move on the right days, and a project large enough to show what a changelog expansion costs.

- [ ] **Step 4: Commit** as `docs(tam): Phase 4 notes for reports`, then push and open the PR against `main` titled "Task Activity Manager Phase 4: reports" with the tasks, the gates, and the walk-through. No AI attribution anywhere.

## Deferred

A cumulative flow diagram, a control chart, an epic burndown, cross-project reporting, exporting a chart as an image, a holiday calendar for the ideal line, and any report not about a sprint. Rituals is Phase 5, and is where a report gets written up and published.
