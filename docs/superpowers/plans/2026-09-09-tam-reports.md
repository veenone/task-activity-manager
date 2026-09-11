# Task Activity Manager Phase 4: the sprint report

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax. Lean cycle: implementers write the tests named here but run no suites, **except that every task ends with `go build ./... && go vet ./...` in `tam/`**, because Task 1 widens a seam and a plan that does not compile until Task 3 is a plan that debugs itself in the wrong place. Task 6 runs every gate once.

**Goal:** the numbers a sprint review starts with, reconstructed from Jira's changelog rather than invented, honest about what they cannot see, and readable offline once fetched.

**Succeeds when** a scrum master opens Reports the morning after a sprint closes and gets committed, added, removed, completed and carried over, with a line saying how they were computed and what they miss, without opening a browser.

**Not in this plan: the charts.** A burndown line and a velocity bar chart are drawn from exactly this data and add no new facts. They are the next plan, for the reasons the spec's section 1 gives.

**Architecture:** `core/jira`'s search learns an `expand` parameter. A new capability interface carries it, rather than widening `IssueBackend`, which nine types implement. A new `tam/internal/reports` rewinds each issue's changelog to the sprint's start and walks it forward into a series. A new `sprint_report` table keeps a closed sprint's series, keyed by board as well as sprint, stamped with the algorithm version that built it. One binding returns the sprint's report and the board's velocity together.

**Tech Stack:** Go 1.25 with `go.work`, Wails v2.15.0, `modernc.org/sqlite`, React 19, TanStack Query 5, Vite 8, Vitest 4.

**Spec:** [`../specs/2026-09-09-tam-reports-design.md`](../specs/2026-09-09-tam-reports-design.md).

## Global Constraints

- Nothing under `xtm/` changes. Go commands run from inside the module directory.
- **A report is reconstructed from the changelog, never from Jira's internal chart endpoints.**
- **A number that might be low says so.** Removed counts only cards that left and returned; committed is a floor; a truncated changelog is named. Every surface showing one of these carries the qualification. This replaces the first draft's "a chart never invents a number", which was the right instinct aimed at the wrong risk.
- **The method is printed with the numbers**, not only in the docs: what done means, that history comes from the public changelog rather than Jira's stored sprint records, and what it cannot see.
- A closed sprint's series is cached and refetched only when its stored `algo_version` no longer matches. A live sprint's is never cached.
- Reports are read only. Nothing in this phase writes to Jira or to the journal.
- The PAT stays in the Jira client's Authorization header only.
- Files stay small and single purpose; a helper used from two places lives in its own module.
- UI text uses no em dashes. No AI attribution anywhere. Conventional commit prefixes, no trailers. Never add, commit or delete untracked local tooling files; revert Wails churn with `git checkout --` after checking `git diff -w`.
- **A comment must be true.** Eight consecutive reviews on the previous branch each found at least one that was not, including one that credited the wrong mechanism for a safety property and one that described protection the code did not provide.

## What shipped after this plan was first written

- **Schema version 7 is taken**, by a sprint's `goal`. This phase is version 8.
- **The Reports view already exists** in `VIEWS`, in the native menu at accelerator 5, rendering a `Placeholder`. This phase replaces what it renders and adds nothing to the navigation.
- **The Sprints view exists**, with a board picker that hides itself when the profile has one scrum board. Reports follows that, not the Boards view's.
- **A closed sprint has no cached membership.** The boards sync fetches keys for active and future sprints only. Harmless here, since a report is built from a search, but it means the sprint picker comes from the `sprint` table.

## Decisions

1. **The changelog is fetched with the issues, not per issue.** Per issue would be one call per card.
2. **A report is fetched when it is opened, not on every sync.**
3. **A closed sprint's series is stored, a live one is not**, and a stored one is rebuilt when the algorithm that made it has moved on.
4. **An estimate that changed mid-sprint takes effect on the day it changed**, unless the card was outside the sprint that day, in which case the estimate in force when it re-enters is the one that enters with it.
5. **The ideal line runs over working days.**
6. **`sprint = N` stays, and the blind spot is labelled** rather than queried around. Spec section 3.
7. **A capability interface, not a wider `IssueBackend`.** `SyncBoards` already set this precedent: a capability off the backend, reached by type assertion, so a backend that cannot do it is skipped rather than forced to stub. Widening `IssueBackend` would touch nine implementers, seven of them test stubs in four packages.

---

### Task 0: probe the wire before building on it

**Files:** create `docs/superpowers/plans/assets/2026-09-11-report-wire-probe.md`.

Four behaviours this phase rests on are unverified, and three of the reviews' findings resolve to "we guessed" without them. The previous phase's probe is the model.

- [ ] **Step 1: Write the probe.** Four requests against the user's own instance, each beside the answer this plan assumes.
  1. `GET /rest/api/2/search?jql=sprint=N&expand=changelog&maxResults=50`. **Assumed:** each issue carries a `changelog` object with `startAt`, `maxResults`, `total` and `histories`, and for a long lived card `total` exceeds the histories returned. Record what the cap actually is.
  2. In that answer, find a Sprint field change. **Assumed:** `field` is `Sprint`, `fieldId` is a `customfield_NNNNN`, and `from`/`to` are comma separated lists of sprint ids while `fromString`/`toString` are comma separated names. Record the real shape, because the reconstruction's membership rule is built on it.
  3. Take a card known to have been removed from sprint N and confirm `sprint = N` does not return it. **Assumed:** it does not. This is the blind spot the spec labels; prove it rather than asserting it.
  4. Time the same search at `maxResults` 50 and 25, on the largest sprint available. **Assumed:** noticeably slower than a search without the expansion. Record both.

- [ ] **Step 2: Say what each answer changes.** 1 shapes the truncation handling in Task 1 and the "incomplete" state in Task 5. 2 shapes the membership rule in Task 2 and is the one most likely to differ. 3 confirms or refutes the spec's section 3; if a card that left *is* returned, that section is wrong and this phase gets better. 4 sets the page size in Task 4. The PAT goes in a header and never into a committed file. Commit as `docs(tam): a probe for the changelog read`.

---

### Task 1: the changelog on the wire

**Files:** modify `core/jira/issues.go` and its test; create `tam/internal/backend/history.go`, `tam/internal/backend/jira/history.go` and its test; modify `tam/internal/backend/demo/` for the curated history and its test.

**Produces:** `SearchIssues` gains an `expand []string` parameter; `jira.RawChangelog{StartAt, MaxResults, Total int; Histories []RawHistory}`; `backend.HistoryBackend` with `SearchIssuesWithHistory(ctx, jql string, startAt, maxResults int) ([]backend.IssueHistory, int, error)`; `backend.IssueHistory{Issue Issue; Changes []Change; Truncated bool}`; `backend.Change{At, Field, From, To string}`.

- [ ] **Step 1: The expand.** One parameter, set only when non empty. `url.Values.Encode` sorts keys, so an unchanged caller's query string cannot move; the test that matters is not in `core` but in the backend, asserting the **sync's** search carries no `expand` key at all, which is the test that fails if someone later threads changelog through the sync's paging.

- [ ] **Step 2: Truncation is a first class answer.** Decode `startAt`, `maxResults` and `total` on the changelog object, and set `IssueHistory.Truncated` when `total` exceeds what came back. A cut history decodes identically to a complete one otherwise, and the numbers built from it would be presented as exact. Test: an issue whose `total` exceeds its `histories` is marked; one whose does not is not.

- [ ] **Step 3: The capability, not a wider seam.** `backend.HistoryBackend` in its own file, reached by type assertion the way `SyncBoards` reaches `BoardBackend`, so no existing stub changes and a backend without it is refused with a sentence. **Do not add the method to `IssueBackend`.**

- [ ] **Step 4: The field rule, which is backwards in the first draft.** `status` matches on `fieldId == "status"`. **Sprint and Story Points are custom fields**, so their ids are `customfield_NNNNN` and differ per instance: match them on the ids the backend already resolves per instance, falling back to the display name only when discovery failed. A normaliser keyed on a literal id matches `status`, never matches the other two, and draws a flat line on every real instance while every fixture test passes. The mapping lives in `backend/jira`, beside the id discovery, not in `internal/reports`. Test with a fixture whose Sprint change carries a `customfield_` id and no recognisable name.

- [ ] **Step 5: The demo.** A curated history over the demo dataset, enough for one closed sprint's report to be real: an add, a card that left and returned, a re-estimate, and a card done before the start. The dataset has one closed sprint and no history at all today, so this is its own piece of work rather than a clause. Say in the report whether velocity is demonstrable offline or needs a real instance.

- [ ] **Step 6: Build, vet, commit** as `feat(core): the issue search can ask for a changelog`.

---

### Task 2: the reconstruction

**Files:** create `tam/internal/donerule/donerule.go` and its test; create `tam/internal/reports/reports.go`, `series.go`, `velocity.go` and their tests; modify `tam/internal/sprints/guards.go` to call the new package.

**Produces:** `donerule.Done(cols []backend.BoardColumn) func(statusID string) bool`; `reports.Build(sprint backend.Sprint, done func(string) bool, issues []backend.IssueHistory, now time.Time, loc *time.Location) (Series, error)`; `reports.Velocity(...) []VelocityRow` with `Unit` on each row.

- [ ] **Step 1: One definition of done.** There are three in this codebase: the board's last column, unexported on `sprints.Service`; `backend.IsDone`, which matches on status name and powers the Sprints view's numbers; and the frontend's `lib/unfinished.ts`. This phase needs the first and cannot reach it. Extract it to `internal/donerule`, have the completion call it, and record in the docs that the name based rule still exists and where, because two screens disagreeing about one closed sprint's done count is exactly what this phase makes visible.

- [ ] **Step 2: Rewind, then walk.** The changelog gives today's fields plus deltas, so status, estimate and membership **at the sprint's start are not known**: derive them by unwinding from the present back to the start, then walk forward. A forward replay from today's values draws a sprint that never happened. Test: an issue whose status, points and sprint all changed **after** the sprint closed leaves the series unmoved.

- [ ] **Step 3: Time, explicitly.** `Build` takes its clock and its location rather than reaching for them, the way the syncer's engine already does. Every timestamp parses through `internal/sprintdate`, because Jira's are not RFC 3339: the offset carries no colon, this repo learned that once already, and a hand written fixture using `Z` passes while every real instance returns nothing. Days bucket in the given zone at local midnight, day one being the local date of the sprint's start, and `Day.Date` is the `2006-01-02` form. Test: a sprint starting at `+1000` has day one on its own local date, not the day before.

- [ ] **Step 4: Membership is a set, not a toggle.** The Sprint field's changelog `from` and `to` are comma separated lists, because a card can be in two sprints at once during a rollover. Membership is "is this sprint's id in the `to` set", computed per change. Test: a card moving from `"12, 13"` to `"13"` has left sprint 12 and not sprint 13.

- [ ] **Step 5: The rest of the rules, each with its test.** An estimate changed inside the sprint takes effect that day; one changed while the card was outside takes effect when it re-enters. A card done before the sprint started counts into committed with nothing to burn. A sprint whose start date was edited after it began computes committed at the current start date, and days before the earliest evidence report day one's value. A card with no estimate contributes nothing in points mode.

- [ ] **Step 6: The unit, and why.** Points when any card in the sprint has one, cards otherwise, and the series carries **which reason**: no Story Points field on this instance is a different problem from nothing estimated, and only one of them is the user's to fix.

- [ ] **Step 7: Velocity.** The same reconstruction over the last six closed sprints, each row carrying its own `Unit` so a board that changed from points to cards is not averaged into nonsense. Six is the number Jira shows minus one and nothing better was argued; if it is a placeholder, say so in the comment rather than implying it was measured. Test: six sprints, none, a sprint closed with cards still open, and a board whose unit changed mid-history.

- [ ] **Step 8: Build, vet, commit** as `feat(tam): a sprint's history, reconstructed`.

---

### Task 3: the store

**Files:** modify `tam/internal/tamstore/tamstore.go` and its test; create `tam/internal/boardrepo/reportstore.go` and its test; modify `tam/internal/boardrepo/boardrepo.go` and `boards.go` (the two purge lists); modify `tam/internal/backend/backend.go`, `backend/jira` and the `sprint` table for `CompleteDate`.

**Produces:** schema version 8: `sprint_report(profile_id, board_id, sprint_id, unit, algo_version, built_at, series_json, PRIMARY KEY (profile_id, board_id, sprint_id))`, and a `complete_date` column on `sprint`; `boardrepo.SavedReport`, `SaveReport`, `Report`.

- [ ] **Step 1: Keyed by board, and here is why.** Jira hands one sprint to every board whose filter reaches it, which is why the `sprint` table is keyed `(profile_id, board_id, id)` and why version 6 exists at all. A report's done rule comes from its board's last column, so a key without the board serves board B the chart built for board A, forever, because a closed sprint's series is never refetched. Same mistake, same table family, one version later.

- [ ] **Step 2: `algo_version`.** A constant in `internal/reports`, stored on write, compared on read, rebuilt on mismatch. Without it the first reconstruction bug is permanent in every user's database. Bump it whenever the reconstruction changes. Test: a row written at an older version is rebuilt rather than served.

- [ ] **Step 3: `completeDate`.** Jira carries the moment a sprint actually closed, routinely days from its planned end, and TAM carries it nowhere: not on `backend.Sprint`, not in the table. Velocity measured at the planned end mis-reports every sprint closed late or early. Add it to the Agile decode, the type and the table. That makes version 8 a real migration in version 7's shape, not only a new table.

- [ ] **Step 4: The new table in both purge lists.** `PurgeProfile` and `RemoveBoards` each hold a hardcoded table list, and `PurgeProfile`'s own comment predicts this exact failure "when a fifth table arrives". It is arriving. Test: purging a profile that has a stored report leaves none behind.

- [ ] **Step 5: The migration's shape.** A new table needs no migration entry, because `Base` runs before migrations on every open; the version bump records the change. Follow the version 3 and 4 tests, which drop the table, rewind the recorded version, reopen and assert it came back, rather than version 6's rewrite fixture. The `complete_date` column does need an entry, in version 7's `AddColumnIfMissing` shape.

- [ ] **Step 6: Build, vet, commit** as `feat(tam): where a sprint's report is kept`.

---

### Task 4: one binding

**Files:** create `tam/app_reports.go` and its test; regenerate `tam/frontend/wailsjs/**`.

**Produces:** `GetSprintReport(profileID string, boardID, sprintID int) (reports.Report, error)` where `Report` carries the series, the velocity rows, and what the read could not see.

- [ ] **Step 1: One call, not two.** The first draft had the view ask for the burndown and the velocity separately, each taking the per profile lock. **That lock refuses rather than waits**, so two concurrent calls on mount would fail one of them with "a report is already running for this profile" every single time the view opened. One call returns both, under one lock, sharing one fetch, and velocity reuses stored reports for the sprints already built rather than refetching six.

- [ ] **Step 2: The cost, handled rather than hoped about.** A page size smaller than the sync's, stated in the code with its reason. Progress reported the way every other long read reports it, because a silent freeze on opening a view is the worst version of this. Cancellation honoured when the user leaves, so a slow report does not hold the profile lock after nobody is looking.

- [ ] **Step 3: The guards, tested.** `requireProfile`, the lock including its refusal, and the backend that cannot expand a changelog. Test the refusal, not only the happy path: the previous branch shipped a task where only `requireProfile` was covered and a review had to ask for the rest.

- [ ] **Step 4: Regenerate, build, vet, commit** as `feat(tam): the binding a sprint report needs`.

---

### Task 5: the view

**Files:** create `tam/frontend/src/components/ReportsView.tsx`, `SprintSummary.tsx`, `queries/reports.ts` and a test for each; modify `api.ts`, `queries/keys.ts`, `App.tsx`, `App.test.tsx`, `nav.ts`, `App.css`.

- [ ] **Step 1: Pickers that already have an answer.** A board picker and a sprint picker following the Sprints view's: hidden when the profile has one scrum board, with the board named in the heading instead.

- [ ] **Step 2: The sentence.** Committed, added, removed, completed, carried over, in the words a review starts with, with the unit and its reason.

- [ ] **Step 3: The method line**, beneath it: what done means here, that history is reconstructed from the public changelog rather than Jira's stored sprint records, and that removals are only visible for cards that came back. This is the answer to a disagreement in front of a team, and it is worth a sentence on screen rather than a paragraph in a document nobody opens mid-review.

- [ ] **Step 4: Every empty state, each distinguishable.** No closed sprint on this board. No estimates, saying which of the two reasons. No dates, so the sprint cannot be reported on. A failed read, keeping the last report and naming the failure with a retry. A report containing truncated histories, naming the cards. **And a read refused because the profile is busy syncing or committing**, which the first draft missed and which is the one a user hits by accident.

- [ ] **Step 5: Wire it in.** `App.tsx` renders `ReportsView` in place of its `Placeholder`, and `nav.ts`'s blurb stops promising charts this plan does not build. The `VIEWS` entry and the menu item already exist at accelerator 5; nothing is added or renumbered.

- [ ] **Step 6: Tests, then the one frontend run.** The sentence; the method line; the unit label against **two** fixtures, points and cards, since one fixture passes against a hardcoded word; every empty state including busy; a failed fetch keeping the previous report *and that report still being correct afterwards*. Then `npx vitest run` and `npx tsc --noEmit`. Commit as `feat(tam): the sprint report view`.

---

### Task 6: docs and the single gate run

- [ ] **Step 1: Docs.** `tam/CLAUDE.md` gains the phase: where history comes from and why not the alternatives, what the report cannot see and why the labels exist, the three definitions of done and which lives where now, that reports are keyed by board and stamped with an algorithm version, and the cost of opening the view. Update the Status paragraph and the Layout tree.

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

Vitest before this plan: 46 in `frontend/core`, 159 in `xtm`, 395 in `tam`. Do not lower an assertion to make a test pass.

- [ ] **Step 3: The walk-through for the user** (not run by agents): on the demo profile, open Reports for the closed sprint and read the summary and the method line. On a real Data Center, the four probe answers confirmed against the built app, and the one thing no fixture proves: that the numbers are recognisably the sprint that happened.

- [ ] **Step 4: Commit** as `docs(tam): the sprint report`, then push and open the PR against `main`.

## Deferred

The burndown and velocity charts, which are the next plan and draw from this data. Querying the whole project to recover cards removed from a sprint. Daily membership snapshots, which would make removals visible going forward and are the one asset Jira cannot reproduce. A cumulative flow diagram, a control chart, an epic burndown, cross project reporting, and exporting anything as an image.
