# Sprint rows: a state badge, a timeline bar, and closed sprints that have numbers

**Spec:** `docs/superpowers/specs/2026-09-15-tam-05-sprints-reports-visuals-design.md`
(approved 2026-09-15), section D1 and implementation tasks 1, 2, 3 and part of 8.
**Issue:** TBD. No issue exists for this bundle yet; open one before the first
commit (P1) and put its number here.
**Branch:** `feat/tam-05a-sprint-rows`, cut from `main` at `329f428` (PR #61).
**Method:** ponytail agents per task, scoped gates, one review scoped to this
branch at PR time.

**Goal:** answer request item 6. A sprint row says at a glance what state a
sprint is in, where it is in its calendar, and how much of it is done, and the
closed sprints that make up most of the list have real numbers to draw.

## Refreshed 2026-09-22 against `main` at `329f428`

This plan was written against `main` at `0caecb4`. Two PRs have landed since,
and the second of them worked on exactly these surfaces. Everything below is
re-measured against `329f428`. What moved is listed here once so a reader does
not have to diff the plan against itself.

**#55 (Jira's screens decide what TAM offers and sends)** touches the create and
edit paths and nothing this bundle draws. No task changes because of it.

**#61 (the TAM layout bugs a design critique found, and the gates for them)**
is the one that matters. It did five things this plan had planned or assumed:

1. **The hardcoded chip colours are gone.** `.chip-status-active` and
   `.chip-status-done` (`App.css:158-159`) read `var(--chip-blue-bg)` /
   `var(--chip-blue-text)` and the green pair, both declared in `:root` **and**
   in `:root[data-theme="dark"]` in `frontend/core/styles/tokens.css`.
   `.chip-draft` (`App.css:416`) reads `--warning-bg`, `--warning-text` and
   `--warn-border`, all three real tokens in both blocks. The spec's D1 line
   "Replace the hard-coded chip colours" is **satisfied**, and so is the
   premise of the old Task 1 step 5, which is deleted below. TAM's App.css now
   sets **zero** colour literals.
2. **The whole charts half shipped**, under issue #60:
   `tam/frontend/src/lib/chartScale.ts`, `components/charts/BurndownChart.tsx`,
   `VelocityChart.tsx`, `OutcomeChart.tsx`, `frame.tsx`, `geometry.ts`,
   `useChartWidth.ts`, `usePointFocus.ts`, with tests beside each, plus the
   twelve `--chart-*` tokens in both theme blocks. Spec sections D2 and
   implementation tasks 4, 5, 6 and 7 are **done**. The companion plan
   `2026-09-19-tam-05b-report-charts.md` describes work that no longer exists;
   this plan is now the whole of what is left of spec 05. Every cross-reference
   to 05b in the old text is dropped, including the claim that 05a should land
   first so 05b's chart tokens inherit its gate.
3. **The dark mode gate this plan proposed already exists, in a stronger form.**
   `tam/frontend/src/theme.test.ts` reads `App.css` and the token table off
   disk and asserts that every token App.css names is declared in `tokens.css`
   and, when its value is a colour, has a dark counterpart; a second block
   measures contrast for fifteen filled chip and card classes in both themes.
   What it does not cover is a token read only from
   `frontend/core/styles/primitives.css`, which is where this bundle's badge
   and bar rules go. Task 1 closes that gap with one case in `theme.test.ts`
   rather than the `instruction-gate.test.ts` case the old plan described.
4. **`RowLead` is the house precedent for a shared primitive.**
   `frontend/core/src/components/RowLead.tsx` (58 lines), its test beside it
   (56), its rules in `frontend/core/styles/primitives.css:1444-1470`, its
   export from `frontend/core/src/index.ts:12-13`, and a tam-side test helper
   at `tam/frontend/src/test/rowLead.ts`. `StatusBadge` and `ProgressBar` copy
   that shape exactly, and not the older examples the earlier draft cited.
5. **Two new gates and two new ratchet counters bind this bundle.** The class
   gate in `instruction-gate.test.ts:256-339` fails any class a component sets
   that no stylesheet defines, reading template literals and braced ternaries
   as well as plain attributes. `hardcoded_hex` and `hardcoded_px_font` are in
   `.ratchet-counters`. All three are in the global constraints below with
   their measured numbers.

**`tam/frontend/src/components/SprintList.tsx` is 386 lines, not 427.** #61
moved its row model into `lib/sprintRows.ts`, which took `files_over_400` from
107 to 106 in the same change. The old plan's "SprintList must not gain a line"
rule is therefore **lifted**: it has fourteen lines of headroom now. Task 2
step 5 and Task 6 both say so.

**Nothing on the Go side moved.** `syncer/boards.go`, `boardrepo/sprintlist.go`
and their tests are byte for byte what Task 2 was written against, and every
line number it cites still resolves.

---

## The rule this bundle rests on

**The row needs one Go change and nothing else.**

Everything the row draws about an active or future sprint already reaches the
frontend: `SprintDetail` carries `total`, `done`, `points`, `donePoints`,
`startDate`, `endDate`, `state`, `goal` and `draft`. A closed sprint carries all
of those too, and `total` and `points` are structurally **zero**, because no
closed sprint has any cached membership at all (`tam/internal/syncer/boards.go:214-224`
fetches keys for active and future sprints only). Most rows in a real Sprints
view are closed. A progress bar that is always empty on the majority row type is
worse than no progress bar.

So the truth for this bundle: **one Go change, in the boards sync and in the
sprint detail read, and no new binding, no new query, no schema migration and
no new API surface.** `SprintDetail`'s wire shape does not change, so there is
still no `wails generate module` step; CI's regenerate-and-diff job is the check
on that, and a diff from it is a finding.

## Global constraints

Every task satisfies all of these. Numbers measured against `main` at `329f428`
on 2026-09-22.

- **G1. Test first (P2).** A failing test precedes every change, red for the
  intended reason. CI's `proven-red` job runs each change's tests against the
  pre-change code and says whether the red was an assertion or a missing symbol.
  A test whose only red is "StatusBadge is not exported" has demonstrated
  nothing; write the assertion against a stub that renders nothing, so the red
  is a missing label or a missing `aria-valuenow`.
- **G2. Assertions must be able to fail (C6).** Assert the text a badge printed,
  the `aria-valuenow` a bar reported, the keys a sync wrote. Never "the element
  is in the document" on its own, and never a property of a fixture.
- **G3. Files under 400 lines (C2).** `files_over_400` is **106** in the tree
  today against a baseline of **106**. There is **no headroom**. Task 6 restates
  which new code goes in which new file so this number comes out at 106. The
  counter globs `*.go *.ts *.tsx` minus `wailsjs`, which means **it counts
  `*_test.go` and `*.test.tsx` too**. `SprintsView.test.tsx` (556) and
  `ReportsView.test.tsx` (537) are already over and already counted; a new file
  crossing 400 is a new failure.
- **G4. The ratchet never grows (C7).** Baselines, and what each actually reads
  on `main` today: `files_over_400` 106 (actual 106), `ui_em_dashes` 114 (actual
  114), `unscoped_todos` 43 (actual 43), `bespoke_modals` 39 (actual 39),
  `hardcoded_hex` 441 (actual 441), `hardcoded_px_font` 60 (actual 60). All six
  are **at baseline with zero headroom**. The four eslint counters
  (`eslint_a11y` 95, `eslint_max_lines` 23, `eslint_no_console` 23,
  `eslint_problems` 196) are read out of `eslint-report.json`, so
  `sh scripts/lint-report.sh` runs before `sh scripts/ratchet.sh`. Their actuals
  are not restated here because they are not measurable without running lint;
  read them from the report rather than trusting a number in a plan (M2).
- **G5. No new colour literal and no new pixel font size in an app stylesheet
  (new in #61).** `hardcoded_hex` counts hex values in `tam/frontend/src/*.css`
  and `xtm/frontend/src/*.css`; all 441 of them are XTM's and **TAM's share is
  zero**. `hardcoded_px_font` counts `font-size: Npx` in `tam/frontend/src/*.css`
  and reads 60. Both are at baseline, so one new hex or one new pixel font size
  in TAM's `App.css` fails the gate. `frontend/core/styles` is excluded from
  both counters on purpose: `tokens.css` is the one place a hex value belongs.
  That is the practical reason this bundle's badge and bar rules live in
  `frontend/core/styles/primitives.css` and its colours in `tokens.css`, and why
  every rule Task 4 adds to `App.css` sets geometry, not colour or type size.
- **G6. Every class a component sets has a rule (new in #61).** The gate is
  `frontend/core/src/instruction-gate.test.ts:256-339` and it reads template
  literals and braced ternaries, not only `className="..."`. Two things follow
  for this bundle. Every new class (`status-badge`, `progress-bar` and their
  tone modifiers, the timeline cell, the 900px stacking hooks) needs a rule in
  the same change. And **`sprint-cell-state` is currently in that test's `HOOKS`
  set**, the list of classes allowed to have no rule; a second assertion fails
  when a listed class gains one. So if Task 4 gives `.sprint-cell-state` a rule,
  it must be removed from `HOOKS` in the same commit.
- **G7. No em dash in a user-visible string.** Every badge label, bar label,
  timeline phrase and empty state this bundle adds is UI copy under that
  contract. The counter drops whole-line comments but not trailing ones, and it
  skips `.test.` files. With zero headroom, one em dash fails the gate.
- **G8. No `console` in a workspace `src` tree.** A row that cannot read a
  sprint's dates draws no timeline; it does not warn.
- **G9. No new dependency (C1).** Nothing here needs one.
- **G10. Docs go to `agents/project/tam-phases.md`.** `tam/CLAUDE.md` has been a
  two line stub since #42, so the spec's "replace the paragraph in
  `tam/CLAUDE.md`" is satisfied there. Task 5 names the exact lines.
- **G11. No AI trailers of any kind in commits or the PR body.**
- **G12. One logical change per conventional commit (P5).**
- **G13. Scoped gates.** Each task runs the suites it touches plus
  `npm run typecheck`, and the Go tasks run `go test` for the packages they
  touch. CI runs the full set.
- **G14. Modal primitives contract.** This bundle opens no dialog, so it adds no
  `modal`, `overlay` or `backdrop` class anywhere.
- **G15. Test counts rise, never fall.** Measured today:
  **253** in `frontend/core`, **159** in `xtm/frontend`, **978** in
  `tam/frontend`. `agents/project/testing.md` still records 46, 159 and 507,
  which were right before #61 and are not right now; Task 5 refreshes them.
  Two of the three rise here. None may fall.

## Reuse ledger, read before writing anything

Every row exists today on `main` at `329f428`.

| Need | Reuse | Where |
|---|---|---|
| The sprint's date range as a phrase | `sprintDates(s)` | `tam/frontend/src/lib/format.ts:47` |
| "day 6 of 14", already clamping an overdue sprint to its last day | `dayOfSprint(startIso, endIso, now)` | `format.ts:93`, and **Task 3 changes how it counts** |
| "8 of 14 done, 21 of 34 pts" | `progressText(done, total, donePoints, allPoints)` | `format.ts:80` |
| One day as "12 Sep" | `day(iso)` | `format.ts:25`, and **Task 3 fixes its day boundary** |
| The civil day off a bare date, already local-safe, added by #61 | `calendarDay(date)` | `format.ts:36`, and **Task 3's fix is built out of it** |
| The civil day off a Jira stamp, without a `Date` round trip | `dayInput(iso)` and the comment above it | `format.ts:59` |
| A whole number that is really a float | `points(n)` | `format.ts:67` |
| "1 card" against "2 cards" | `plural(n, one, many)` | `format.ts:18` |
| The `now` parameter shape a formatter takes in this repo | `formatWhen(iso, now = new Date())`, `dayOfSprint(a, b, now = new Date())` | `format.ts:4, 93`, and D3 below |
| **The shape a shared primitive takes in this repo** | `RowLead` | `frontend/core/src/components/RowLead.tsx`, its test beside it, its rules at `primitives.css:1444-1470`, its export at `index.ts:12-13`, its tam-side helper at `tam/frontend/src/test/rowLead.ts` |
| A chip that already shouts in CSS rather than in its text | `.chip-conflict { ... text-transform: uppercase; }` | `tam/frontend/src/App.css:419`, the precedent D4 rests on |
| The chip colour pairs, already tuned for both themes by #61 | `--chip-blue-bg/-text`, `--chip-green-bg/-text`, `--chip-sky-*`, `--chip-purple-*`, `--chip-red-*`, `--warning-bg`/`--warning-text` | `frontend/core/styles/tokens.css:63-72` and `:146-155` |
| The state chips this bundle replaces, already on tokens | `.chip-status-todo`, `.chip-status-active`, `.chip-status-done`, `.chip-draft` | `App.css:157-159, 416` |
| A chip that is waiting on Commit | `.chip-draft`, and the `waiting` value `SprintList` passes per sprint id | `App.css:416`, `SprintList.tsx:39-40, 370` |
| The sprint tree's row grid | `.sprint-row`, `grid-template-columns: 12px minmax(120px, 1fr) 72px 150px 190px 76px` | `App.css:608-610` |
| Every cell already clips to its track | `.sprint-cell` | `App.css:617` |
| The goal's own line under an expanded sprint, and its empty wording | `.sprint-goal`, `No goal recorded yet; refresh the board.` | `App.css:625`, `SprintList.tsx:302-303` |
| The honest wording for a closed sprint with nothing cached | `SprintList.tsx:332` | that line, and Task 2 rewords it |
| A progress-ish bar already in this repo | `.syncbar-track` / `.syncbar-fill` | `App.css:130-131` |
| The narrow-pane block this bundle joins rather than duplicates | `@media (max-width: 900px)` | `App.css:797-799`, added by #61 for the report charts |
| Theme tokens, light and dark | `--accent`, `--accent-soft`, `--ok-text`, `--warn-text`, `--warn-soft`, `--danger-text`, `--text-muted`, `--border`, `--surface`, `--surface-sunken` | `frontend/core/styles/tokens.css`, `:root` at 5 and `:root[data-theme="dark"]` at 101 |
| The stylesheet reader every CSS test in TAM shares | `appCss`, `coreStyle`, `tokenTable`, `declarationsOf`, `declarationsMentioning`, `valueOf`, `resolve`, `contrast`, `tokensRead` | `tam/frontend/src/test/cssRules.ts`, 106 lines, added by #61 |
| The dark-counterpart and contrast gate that already runs | `tam/frontend/src/theme.test.ts` | that file, 94 lines |
| A visually hidden element | `.sr-only` | see the three-copy note below |
| One scope's membership, read back from the cache | `boardrepo.Repository.SprintIssues(ctx, profileID, boardID, sprintID)` | `tam/internal/boardrepo/boards.go:208` |
| One scope's membership, written on its own | `ReplaceSprintIssues` | `tam/internal/boardrepo/replace.go:144` |
| The order the view already reads closed sprints in | `detailSprintsSQL`, closed by `start_date` descending | `tam/internal/boardrepo/sprintlist.go:12-28` |
| The traps this suite falls into | `agents/project/testing.md` | that file |

**`.sr-only` is defined in three places, not two.**
`frontend/core/styles/primitives.css:8`, `tam/frontend/src/App.css:717`, and
`xtm/frontend/src/App.css:67`. Do not add a fourth and do not redefine it. XTM
is the one out of step: it carries its own copy because it does not import the
shared `tokens.css` or `primitives.css` at all, so a primitive exported from
`@agile-suite/core` renders unstyled there. That is acceptable for this bundle
because **XTM imports neither `StatusBadge` nor `ProgressBar`**, and both are
written so their meaning survives unstyled: the badge's glyph and label are text
and the bar's value is in `aria-valuenow` and `aria-valuetext`. Moving XTM onto
the shared stylesheets is its own change and is listed under "not in this
bundle".

**`instruction-gate.test.ts` does not forbid project names inside
`frontend/core/src`.** `FORBIDDEN_IN_CORE` (line 29) is applied to `AGENTS.md`
and to nothing else (lines 158-162). Fourteen files under `frontend/core/src`
mention one of those words today, so the gate could not be extended without
fourteen files of unrelated work. The old plan's claim that the gate covers the
package is dropped, and #61 did not change that: the file grew from 242 to 340
lines, all of it the class-name gate. The portability rule for `StatusBadge` and
`ProgressBar` is held by review, not by a script.

---

## Task 1: state tokens, `StatusBadge`, `ProgressBar`, and one gate case

**What #61 already did of this task.** The hardcoded chip colours are on tokens
in both themes, so the old step 5 is gone. `theme.test.ts` already gates every
token `App.css` names for a dark counterpart, so the old step 6 shrinks from a
new file-reading test to one case beside the ones that exist.

**What is left.** The twelve semantic tokens the spec names, the two primitives,
and one gate case that covers tokens read from `primitives.css`.

**Files:** `frontend/core/styles/tokens.css`; new
`frontend/core/src/components/StatusBadge.tsx` + `StatusBadge.test.tsx`; new
`frontend/core/src/components/ProgressBar.tsx` + `ProgressBar.test.tsx`;
`frontend/core/src/index.ts`; `frontend/core/styles/primitives.css`;
`tam/frontend/src/theme.test.ts`.

**Produces**

```tsx
// StatusBadge: a glyph, then a label. The shout is CSS, not the string.
type BadgeTone = "active" | "future" | "closed" | "draft";
function StatusBadge({ tone, label, glyph }: { tone: BadgeTone; label: string; glyph?: string }): JSX.Element

// ProgressBar: one track, one fill, an optional marker at a fraction of the track.
function ProgressBar({
  value, max, label, valueText, marker, tone,
}: {
  value: number; max: number;
  label: string;          // the accessible name, e.g. "Time elapsed in Sprint 14"
  valueText: string;      // aria-valuetext, e.g. "Day 6 of 14"
  marker?: number;        // 0..1 along the track, the today line
  tone?: "time" | "points" | "muted";
}): JSX.Element
```

**Steps**

1. Failing test, `StatusBadge.test.tsx`: a badge renders the label text
   **unchanged** and a non-empty glyph whose character differs between `active`,
   `future` and `closed`. The assertion is on the glyph's text content, not on a
   class, so it fails when someone distinguishes the tones by colour alone.
   Default glyphs: a filled dot for active, a hollow circle for future, a check
   for closed, a hollow dot for draft. A second case asserts that
   `getByText("Active")` matches, which is the assertion D4 rests on.
2. Failing test, `ProgressBar.test.tsx`: `getByRole("progressbar", { name })`
   reports `aria-valuenow`, `aria-valuemin`, `aria-valuemax` and
   `aria-valuetext` as the caller gave them; a `value` above `max` clamps to
   `max` and a negative one clamps to 0; `max` of 0 yields `aria-valuenow` 0 and
   no `NaN` anywhere in the rendered `style` attribute; `marker` outside 0..1 is
   dropped rather than drawn off the track.
3. Implement both, in the shape `RowLead` sets: a function component in
   `frontend/core/src/components/`, no children, a small props interface with
   the reason for each prop in a comment, its test beside it. Neither takes a
   colour and neither knows what a sprint is. `ProgressBar` is a
   `<div role="progressbar">` with a fill `<span>` whose width is a percentage,
   and the marker is a second absolutely positioned `<span aria-hidden="true">`.
4. Tokens. Add to **both** `:root` and `:root[data-theme="dark"]` in
   `frontend/core/styles/tokens.css`: `--state-active`, `--state-active-soft`,
   `--state-future`, `--state-future-soft`, `--state-closed`,
   `--state-closed-soft`, `--state-draft`, `--state-draft-soft`, `--bar-track`,
   `--bar-time`, `--bar-points`, `--today-marker`.
   **Eleven of the twelve are aliases, not new colours.** #61 tuned a chip
   palette for both themes and measured its contrast; a state token that
   invented a thirteenth blue would be a second answer to a question that is
   already answered. So `--state-active` is `var(--chip-blue-text)` and
   `--state-active-soft` `var(--chip-blue-bg)`; closed takes the green pair;
   future takes what `.chip-status-todo` already draws (`--text` on
   `--surface-3`); draft takes `--warning-text` on `--warning-bg`, which is what
   `.chip-draft` reads today. `--bar-track` is `var(--surface-sunken)`, the
   `.syncbar-track` fill; `--bar-time` is `var(--accent)`; `--bar-points` is
   `var(--chart-completed)`, so a points bar and a completed line on the
   burndown beside it are the same green. Only `--today-marker` is a value of
   its own.
   **Declare each in both blocks even where the value is identical**, which is
   the house style `--chart-*`, `--card-border` and the chrome tokens already
   follow: a reader of the dark block sees the whole palette, and step 5's gate
   is a plain "is it there" rather than a resolver.
   This repo switches themes with the `data-theme` attribute, not
   `prefers-color-scheme`, so a media query would be dead code here.
5. **The gate case.** `tam/frontend/vite.config.ts:11` sets `css: false`, so no
   test in this repo can read a computed colour. #61's answer is to read the
   stylesheet text, and `theme.test.ts` already asserts a dark counterpart for
   every token **`App.css`** names. The badge and bar rules live in
   `primitives.css`, which that test does not read, so their tokens would not be
   covered. Add one case to `theme.test.ts`, beside the two that are there:
   every custom property declared in `tokens.css`'s `:root` block whose resolved
   value is a colour also appears in the `:root[data-theme="dark"]` block. All
   59 light colour tokens pass this today, measured, so it locks in what #61
   achieved rather than asking for new work. Prove it red by adding a light-only
   token first, then deleting it. It reuses `tokenTable`, `resolve` and the
   `isColour` predicate already in that file.
6. Export both from `frontend/core/src/index.ts`, and add their classes to
   `frontend/core/styles/primitives.css` beside the other primitives, which is
   where `RowLead`'s rules are. Every new class needs a rule in this same
   commit (G6).
7. Gate: `cd frontend/core && npx vitest run src/components/StatusBadge.test.tsx src/components/ProgressBar.test.tsx && npm run typecheck`, then
   `cd tam/frontend && npx vitest run src/theme.test.ts`.

**Watch:** `frontend/core` is the portable package. The badge takes a tone and a
label and knows nothing about sprints; the bar takes numbers and knows nothing
about time. That rule is held by review here, not by the instruction gate (see
the reuse ledger note).

---

## Task 2: a closed sprint's membership is read once, and kept

**#61 changed nothing here.** Every file and every line number below was
re-checked at `329f428` and resolves unchanged.

**Files:** `tam/internal/syncer/boards.go`; new
`tam/internal/syncer/closedmembership_test.go`;
`tam/internal/boardrepo/sprintlist.go`;
`tam/internal/boardrepo/sprintlist_test.go`;
`tam/frontend/src/components/SprintList.tsx`;
`tam/frontend/src/components/SprintList.test.tsx`.

**This task reverses a decision this repository recorded on purpose.**
`agents/project/tam-phases.md:911-917` says closed sprints carry no cached
membership by design, because "chart Phase 4 draw from it should not depend on
mostly-idle poll of history nobody asked for", and `:622-624` says only active
and future sprints have cards fetched. The reasoning was sound about a *poll*.
It is wrong about a *row*, because the row promises an at-a-glance progress bar
and the majority of rows in any real Sprints view are closed. A bar that is
structurally zero on the majority row type is a lie the row cannot tell apart
from an empty sprint. So the reversal is narrow: a closed sprint's membership is
read **once**, and then never read again, because it cannot change.

**How the cost is bounded, and why this shape rather than a lazy fetch.**
`ReplaceBoard` deletes **every** `board_issue` row of a board before it inserts
the scopes the pass read (`replace.go:51`, `deleteBoardIssues` at 227). That is
deliberate and it is what stops a sprint that is no longer active leaving its
membership behind for good. It also means a lazily fetched closed scope written
through `ReplaceSprintIssues` would be **wiped by the next boards sync**, so the
lazy fetch would fire again after every sync and would not be read-once at all,
unless `ReplaceBoard`'s wholesale delete were changed too. The lazy option
therefore costs a new binding, a second holder of the per-profile lock for a
read, a new frontend query with its own loading, error and offline states, **and**
a change to that delete, for a worse bound. The read-once shape the code makes
natural is to consult the cache inside `readBoard` and re-supply what is already
there, which needs no new binding, no new lock holder, no frontend call, no
schema change, and survives `ReplaceBoard` untouched because the keys go through
it like every other scope. That is what this task builds, and it is chosen
against the review's stated preference for a lazy fetch for exactly that reason.

**Steps**

1. Failing test first, in a new `tam/internal/syncer/closedmembership_test.go`
   (`boards_test.go` is 364 lines and a second file is cheaper than a C2
   failure). Against a fake board backend that records every `BoardIssueKeys`
   call:
   - **the regression that proves the point.** Two `SyncBoards` passes over a
     board with one closed sprint: after the second pass the closed sprint's
     membership is still in `board_issue` and the backend was asked for it
     **once**. Run this against the pre-change code and watch it fail with an
     empty scope on the first pass, which is the honest red.
   - an active sprint's keys are re-fetched on every pass, unchanged.
   - a sprint that was active on pass one and closed by pass two keeps the
     membership pass one cached, and the backend is not asked again.
   - a board with more uncached closed sprints than the per-pass budget reads
     the newest ones first, leaves the oldest uncached, and reads the oldest on
     the next pass.
   - a closed sprint that genuinely holds nothing of this project is asked
     about again on the next pass, one request, and the row stays honest about
     it. This is the stated ceiling, asserted rather than left implicit.
2. Implement in `readBoard` (`boards.go:200`). It gains `profileID`, which its
   one caller already has (`boards.go:148`). For a sprint that is neither active
   nor future: ask `e.Boards.SprintIssues(ctx, profileID, b.ID, sid)` first and
   re-supply those keys when there are any; otherwise spend one unit of a
   per-pass budget and fetch. Closed sprints are walked newest first, by
   `start_date` descending with the id breaking a tie, which is the order
   `detailSprintsSQL` already reads them in, so a board backfills from the top
   of the view downward.
   ```go
   // closedMembershipBudget caps how many of a board's closed sprints one
   // pass reads membership for. A closed sprint's membership never changes,
   // so each is read once and re-supplied from the cache for ever after;
   // the cap only spreads the first pass on a board with years of history
   // across a few syncs instead of stalling one.
   const closedMembershipBudget = 12
   ```
   Rewrite `readBoard`'s doc comment (`boards.go:189-199`), which currently
   states the old rule in full ("Closed sprints are kept in the sprint list but
   their keys are never fetched"), and say what replaced it and why.
3. Failing test in `sprintlist_test.go` (357 lines; keep the additions tight or
   the new cases go in their own file): a closed sprint with cached keys reports
   `MembershipCached` true and its `Total`, `Done`, `Points` and `DonePoints`
   counted from those keys; a closed sprint with none reports `MembershipCached`
   false and zeroes. The existing cases at `sprintlist_test.go:117-127`, which
   assert `MembershipCached` is read from state, are rewritten rather than
   deleted: their comment is the record of the rule being replaced.
4. Implement in `sprintDetails` (`sprintlist.go:122`). `scopeKeys` is already in
   hand at the point `newSprintDetail` is called, so:
   ```go
   detail := newSprintDetail(s)
   if s.State == "closed" {
       detail.MembershipCached = len(scopeKeys) > 0
   }
   ```
   `MembershipCached` keeps its name and its wire shape and changes meaning for
   closed sprints only: from "the sync tries to fetch this scope" to "this scope
   has been read and kept". Active and future sprints keep the state derivation,
   because the sync asks for them on every pass. Rewrite the field's doc comment
   (`sprintlist.go:70-79`) and `newSprintDetail`'s (`:219-228`); both currently
   state the rule this task reverses.
   **The ceiling, stated in the comment:** rows in `board_issue` cannot tell a
   closed sprint nobody has read from one that really holds nothing of this
   project. The second is re-read once per boards pass, which is one request,
   and it is the only case that is. If that ever shows up in the field, the
   upgrade is a `membership_synced` stamp on the `sprint` row, which is a
   migration this task deliberately does not spend.
5. Frontend, honest before the fetch. `SprintList.tsx:332` already words the
   uncached case ("A closed sprint's cards are not synced, so this list is empty
   whatever the sprint held."). That sentence is now wrong in its second half:
   the cards are coming. Reword it to say the membership has not been read yet
   and that the next board sync reads it, once. **The no-growth rule the old
   plan put on this file is lifted:** #61 moved the tree's row model out to
   `lib/sprintRows.ts` and `SprintList.tsx` is 386 lines, so it has headroom and
   is no longer one of the files `files_over_400` counts. Keep it under 400 all
   the same. Add the assertion to `SprintList.test.tsx` (210 lines).
6. Gate: `cd tam && go test ./internal/syncer/... ./internal/boardrepo/... -count=1 && go vet ./internal/syncer/... ./internal/boardrepo/...`, then
   `cd tam/frontend && npx vitest run src/components/SprintList.test.tsx && npm run typecheck`.

**Watch:** no binding changes. `SprintDetail` gains no field and changes no
type, so `wails generate module` produces no diff and the generated-bindings
contract is satisfied by CI's existing regenerate-and-diff job. If that job
reports a diff, a binding changed and the task went wrong.

---

## Task 3: `day` counts the civil day, `dayOfSprint` counts inclusively, and `sprintRelative`

**#61 changed one thing here, and it makes fix B smaller.** It added
`calendarDay` (`format.ts:36`), which reads a bare `YYYY-MM-DD` as a local day
precisely to avoid the `new Date(iso)` round trip. Fix B is now a composition of
two functions that already exist rather than a new regex.

**Files:** `tam/frontend/src/lib/format.ts` + `format.test.ts`;
`tam/frontend/src/components/SprintsView.test.tsx` (one fixture expectation).

**Produces**

```ts
// lib/format.ts, beside dayOfSprint, which it reuses.
export interface SprintTiming {
  // "Day 6 of 14", "Starts in 11 days", "Closed 11 Sep", "2 days over", "".
  label: string;
  // "8 days left", "14 days", "" for a row with no length to state.
  trailing: string;
  // 0..1, how far through the calendar the sprint is. -1 when it has no
  // readable range, which is what tells the row to draw no time bar at all.
  elapsed: number;
}
export function sprintRelative(
  s: { startDate: string; endDate: string; state: string; completeDate?: string },
  now?: Date,
): SprintTiming
```

**Two existing functions are fixed first, because `sprintRelative` cannot be
consistent with either of them as they stand.**

**Fix A: `dayOfSprint` counts exclusively.** `format.ts:98` computes
`length = Math.round((end - start) / MS_PER_DAY)`, so a sprint running 12 Sep to
25 Sep is "day 13 of 13" and the spec's row wants "Day 14 of 14". `SprintsView`
renders `dayOfSprint` in its own summary line (`SprintsView.tsx:271`) on the same
screen as the row, so the two would print different lengths for the same sprint.
`length < 1` also returns `""`, so it cannot express a one-day sprint at all,
which D6 requires a test for. Fix it: count inclusively, `length = round(...) + 1`.
A reversed range still yields a length below 1 and still returns `""`, so the
existing guard keeps its meaning.

**Fix B: `day()` has the day-boundary bug `dayInput`'s own comment warns about.**
`day()` (`format.ts:25`) puts the stamp through `new Date(...)` and
`toLocaleDateString`, so a sprint starting 09:00 UTC reads as the previous day
for a reader west of it. `dayInput` (`format.ts:59`) exists precisely because of
that and reads the leading `YYYY-MM-DD` off the stamp, and `calendarDay`
(`format.ts:36`, added by #61) already turns a bare day into a local `Date` and
formats it with the same options `day()` uses. So the fix is a composition, not
new code: `day(iso)` returns `calendarDay(dayInput(iso))` when the stamp carries
a leading date, and falls back to the current path when it does not, which keeps
`day("not a date")` at `""`. Its two callers, `sprintDates` (`format.ts:47`) and
`BoardNotes.tsx:16`, both get the fix for free and both want it. There is one
civil-day reader in this file afterwards, not two.

**Steps**

1. Failing tests for fix A in `format.test.ts`, replacing the four `dayOfSprint`
   cases at `format.test.ts:68-91`: the existing 29 Aug to 12 Sep fixture is
   "day 1 of 15", "day 6 of 15" and "day 15 of 15" late, a same-day sprint is
   "day 1 of 1", and the reversed and unreadable cases still return `""`. The
   comment above the "stops at the last day" case is kept; it is the record of
   the clamp.
2. Failing tests for fix B: `day("2026-09-12T23:00:00Z")` and
   `day("2026-09-12T01:00:00Z")` both read 12 Sep whatever the runner's zone,
   and `day("not a date")` is still `""`. Prove the red by running against the
   pre-change code with `TZ` set west of UTC.
3. `SprintsView.test.tsx:202` asserts
   `/^Sprint 12, day \d+ of 14, 1 of 3 done, 3 of 13 pts$/`. The regex was a
   workaround for a wall-clock-dependent line; with fix A the fixture reads "of
   15". Change the number and leave the regex, or pin the clock as D3 describes
   and assert the whole string. Either is fine; do not leave a stale 14.
4. Failing tests for `sprintRelative`, with `now` pinned, never `new Date()`:
   - an active sprint mid-range gives `Day 6 of 15` and `8 days left`, with
     `elapsed` between 0 and 1;
   - a sprint starting today gives `Day 1 of 15`, not `Day 0`;
   - a sprint whose last day is today gives the last day and `0 days left`
     without going negative;
   - an active sprint past its end gives `2 days over` and `elapsed` of 1,
     matching `dayOfSprint`'s clamp rather than contradicting it;
   - a future sprint gives `Starts in 11 days` and `15 days`, and `elapsed` 0;
   - a closed sprint gives `Closed 11 Sep` from `completeDate` when it has one
     and from `endDate` when it does not, and `elapsed` 1;
   - a single-day sprint gives `Day 1 of 1`, not an empty string and not a
     divide by zero;
   - a sprint with either date missing or unparseable gives `("", "", -1)`;
   - **the local day boundary:** a sprint starting `2026-09-12T23:00:00Z` and
     one starting `2026-09-12T01:00:00Z` are the same civil start day, asserted
     by constructing both instants explicitly rather than by naming a zone.
5. Implement `sprintRelative` on top of `dayOfSprint`'s arithmetic rather than
   beside it. If the two need the same rounding, factor the day count into one
   unexported helper in that file; there is exactly one `MS_PER_DAY` in this
   repo (`format.ts:85`) and there will still be exactly one afterwards.
6. Gate: `cd tam/frontend && npx vitest run src/lib/format.test.ts src/components/SprintsView.test.tsx src/components/BoardNotes.test.tsx && npm run typecheck`. Run the suite once with `TZ=America/Los_Angeles` and once with `TZ=Asia/Tokyo`; both must pass.

---

## Task 4: `SprintTimeline` and the redesigned row

**#61 changed nothing in `SprintRow.tsx` or `SprintRow.test.tsx`**, so every
citation below resolves as written. What it did change is the stylesheet around
the row: `App.css` is 881 lines, `.sprint-row` is at 608-610, and there is now a
`@media (max-width: 900px)` block at 797-799 that this task joins instead of
opening a second one.

**Files:** new `tam/frontend/src/components/SprintTimeline.tsx` +
`SprintTimeline.test.tsx`; `tam/frontend/src/components/SprintRow.tsx` +
`SprintRow.test.tsx`; `tam/frontend/src/App.css`;
`frontend/core/src/instruction-gate.test.ts` (the `HOOKS` entry, if step 4
reaches it).

**Steps**

1. Failing test, `SprintTimeline.test.tsx`, with the clock pinned as D3
   describes: the component renders the label line, the trailing note, a
   `progressbar` named for the time with the right `aria-valuetext`, the points
   line (`progressText`, reused) and a second `progressbar` for points whose
   `aria-valuenow` is `donePoints` and whose `aria-valuemax` is `points`. For a
   future sprint the points bar reads `0 of 21 pts planned` and `aria-valuenow`
   0. For `elapsed === -1` nothing is drawn but the words `no timeline`. For an
   active sprint the time bar carries a marker and for a closed one it does not.
   **For a closed sprint whose membership is not cached, no points bar is drawn
   at all** and the line says the contents have not been read yet; this is every
   first run on every board, so it is not an edge case.
2. Implement `SprintTimeline` as a new file. It owns the whole "timeline and
   progress" cell: two lines of text and up to two `ProgressBar`s from
   `@agile-suite/core`. It takes a `SprintDetail` and nothing else. It does
   **not** take a `now`: `sprintRelative`'s own default covers it, the way
   `dayOfSprint` and `formatWhen` are already called with their defaults
   throughout this frontend, and D3 says how the tests pin the clock instead.
3. `SprintRow` changes, and only these:
   - the state chip becomes `StatusBadge`;
   - the dates cell prints `sprintDates(detail)`;
   - the timeline cell becomes `<SprintTimeline detail={detail} />`;
   - the old progress cell becomes the scope cell: the issue count, then one
     secondary fact, `N done` for an open sprint and `N carried over`
     (`total - done`) for a closed one, and for a closed sprint with no cached
     membership the honest line instead of a zero;
   - the name cell gains the goal line, or `not created in Jira yet` for a
     draft. See D4: this reverses an assertion the current test makes.
   The actions cell, the menu, the tree roles, the keyboard handling and the
   `waiting` chip are untouched.
4. `App.css`: retemplate `.sprint-row` (`App.css:608-610`) to caret, sprint,
   state and dates, timeline and progress, scope, actions. Put the spec's narrow
   rule, which stacks the timeline column under the sprint name, **into the
   existing `@media (max-width: 900px)` block at `App.css:797-799`** rather than
   opening a second one at the same breakpoint; #61 added that block for the
   report charts. The 720px block below it at `:800-802` is the report metrics
   grid and stays as it is.
   Two gates bind what this step may write (G5, G6). **No hex literal and no
   `font-size: Npx`:** colour comes from the Task 1 tokens and any type size
   from an existing class or a relative unit. **Every class gets a rule**, and
   if the row's state cell gains one, `sprint-cell-state` comes out of the
   `HOOKS` set in `instruction-gate.test.ts:326-330`, or that file's "these are
   defined now and can leave HOOKS" assertion fails.
5. Update `SprintRow.test.tsx` for the new row, keeping every existing assertion
   that still describes the row. `SprintRow.test.tsx:41` and `:44` both survive
   D4 unchanged; `:48`, which asserts `Ship checkout` is **absent**, is inverted
   and the comment above it is rewritten rather than deleted. Add: a draft sprint
   shows the Draft badge and the `not created in Jira yet` line; a sprint with a
   `waiting` value shows both the state badge and the waiting chip, so bundle
   04's pending Commit chip survives the redesign; the Board backlog row shows
   `no dates`, `no timeline` and its issue count and has no badge, no bars and no
   menu.
6. Gate: `cd tam/frontend && npx vitest run src/components/SprintTimeline.test.tsx src/components/SprintRow.test.tsx src/components/SprintList.test.tsx src/components/SprintsView.test.tsx src/styles.test.ts src/theme.test.ts && npm run typecheck`, then
   `cd frontend/core && npx vitest run src/instruction-gate.test.ts`.

**Watch:** `SprintsView.tsx` is 483 lines and gains none in this task; the
view's summary line is Task 3's business, not this one's. `SprintList.tsx` is
386 and passes everything `SprintTimeline` needs down to `SprintRow` already.

---

## Task 5: the docs

**Files:** `agents/project/tam-phases.md`; `agents/project/testing.md`;
`tam/frontend/src/components/SprintList.tsx` (comment only, if Task 2 did not
already reach it).

**Steps**

1. `agents/project/tam-phases.md:622-624` ("Only active and future sprints have
   cards fetched") and `:911-917` ("Closed sprint carry no cached membership, by
   design not accident") both state the rule Task 2 reversed. Rewrite both to the
   new rule and, in the second, keep the old reasoning and say what changed about
   it: the objection was to polling history nobody asked for, and a read-once
   backfill is not a poll. Name the ceiling from Task 2 step 4 so the next reader
   does not rediscover it.
2. Record the row itself in the Sprints view section: the state badge and why the
   shout is CSS rather than a second string, the two bars and what each one's
   accessible name and `aria-valuetext` say, the goal moving onto the row, the
   900px stacking rule joining the block #61 opened, and `sprintRelative` as the
   one place the relative wording lives.
3. Record the two `format.ts` fixes from Task 3 with their symptoms, since both
   are the kind of bug that comes back: an exclusive day count that disagreed
   with the row on the same screen, and a `Date` round trip that moved a sprint
   date by a day west of UTC, fixed by routing `day()` through the
   `calendarDay` reader #61 already added for the report's bare days.
4. `agents/project/testing.md:16` records 46 tests in `frontend/core`, 159 in
   `xtm/frontend` and 507 in `tam/frontend`. Measured on `329f428` the three are
   **253, 159 and 978**: #61 landed the charts, the family rows and three
   stylesheet-reading suites and did not refresh the line. A counter nobody can
   trust is worse than none, so correct it to what this branch leaves behind.
   This is the only reason `testing.md` is touched.
5. The "charts are next plan" corrections the old plan deferred to 05b are
   **done**, by #61: `tam-phases.md`, `ReportsView.tsx`, `api.ts` and
   `VelocityTable.tsx` all describe what is there. Nothing to do; the step is
   kept so the next reader does not go looking.
6. Gate: `sh scripts/lint-report.sh && sh scripts/ratchet.sh`, and re-run
   `frontend/core`'s instruction gate if `AGENTS.md` or `AGENTS.project.md` was
   touched, which they should not be: the word budget case covers them and
   `AGENTS.project.md` gained two contracts in #61 already.

---

## Task 6: where the new code lives, so C2 holds

Not a task to execute, a constraint on tasks 1 to 5. `files_over_400` is **106**
against a baseline of **106** and must come out at 106. There is no headroom.
Every "Lines on `main`" figure below was re-counted at `329f428`.

| File | Lines on `main` | After |
|---|---|---|
| `SprintRow.tsx` | 154 | about 190. The timeline cell is one element. |
| `SprintRow.test.tsx` | 146 | about 230 |
| `SprintList.tsx` | 386 (was 427; #61 moved the row model out) | about 386, one sentence reworded. Headroom now, but stay under 400. |
| `SprintsView.tsx` | 483 (already counted) | 483, untouched |
| `lib/format.ts` | 102 | about 165, `sprintRelative` and its helper |
| `lib/format.test.ts` | 106 | about 200 |
| `SprintsView.test.tsx` | 556 (already counted) | 556, one expectation |
| `theme.test.ts` | 94 | about 105, one gate case |
| `boardrepo/sprintlist.go` | 276 | about 285 |
| `boardrepo/sprintlist_test.go` | 357 | **the risk.** Keep the new cases tight or put them in their own file. |
| `syncer/boards.go` | 264 | about 300 |
| `syncer/boards_test.go` | 364 | 364. The new cases go in `closedmembership_test.go`, which is why that file exists. |
| `frontend/core/styles/tokens.css` | 170 | about 195, twelve tokens in two blocks |
| `frontend/core/styles/primitives.css` | 1475 | grows, and is not a ratcheted extension |
| `App.css` | 881 | grows, and is not a ratcheted extension. No hex, no pixel font size. |

Everything else is a new file, each well under 400:
`frontend/core/src/components/StatusBadge.tsx`, `ProgressBar.tsx`;
`tam/frontend/src/components/SprintTimeline.tsx`;
`tam/internal/syncer/closedmembership_test.go`; and the tests beside each.

**The rule, restated to cover tests.** A file approaching 400 wants its second
concern in its own module, and that applies to `*_test.go` and `*.test.tsx`
exactly as it does to source, because the counter makes no distinction. A test
file splits by the behaviour it covers, not by an arbitrary halfway line. #61 is
the worked example: `SprintList.tsx` was 427 and its pure row model went to
`lib/sprintRows.ts` as part of the change that would have grown it.

---

## Decisions, settled here and not left to the implementer

Each of these was settled by the eng review or by the user before `329f428`.
Every one was re-checked against the code as it is now, and the check is
recorded beside it.

**D1. Closed sprints get real numbers, and it costs one Go change.** Settled by
the user on 2026-09-19 against the alternative of hiding the bars or printing a
"data not available" note. The reversal, its bound, its ceiling and the shape
chosen over a lazy fetch are all in Task 2, and the recorded decision it
overturns is rewritten rather than deleted in Task 5.
*Still true at `329f428`:* the Go side is untouched by #55 and #61.

**D2. `MembershipCached` keeps its name and changes meaning for closed sprints
only.** From "the sync tries to fetch this scope" to "this scope has been read
and kept". It stays a `bool` on the wire, so no binding changes. The frontend
reads it in exactly one new place, the row's scope cell and points bar, beside
the one place it already reads it, `SprintList.tsx:332`.
*Still true:* unchanged, only the line number moved from 371.

**D3. The clock is pinned in the test, not threaded through the component.**
`SprintTimeline` takes no `now` prop and `SprintRow` threads none, so `SprintList`
gains no prop and nothing is drilled. The pure functions keep the defaulted `now`
parameter this repo already uses (`formatWhen`, `dayOfSprint`) and `format.test.ts`
passes instants to them directly. Component tests pin the clock with
`vi.useFakeTimers({ toFake: ["Date"] })` and `vi.setSystemTime(...)` in a
`beforeEach`, with `vi.useRealTimers()` after. `toFake: ["Date"]` is not
decoration: faking every timer breaks `userEvent`, and `SprintRow.test.tsx`
clicks. This is what stops a row test reading "8 days over" next month, which is
what the current fixture does today.
*Still true:* unchanged.

**D4. The badge shouts in CSS, and the goal moves onto the row.** Two reversals
of decisions `SprintRow.tsx` argues for in comments (`:42-46` and `:60-64`), both
required by the spec, both visible in mockup d1, and both put to the user on
2026-09-18 before implementation and confirmed in favour of the spec.
- The uppercase look comes from `text-transform: uppercase` on the badge's chip
  class, not from an uppercased string. `SprintRow` uses one `label` value in
  the chip (`:119-120`) and in the row's `aria-label` (`:102`), so a literal
  `"ACTIVE"` would make a screen reader spell the state out and would need a
  second field on `StatusBadge` to keep the accessible name in mixed case. CSS
  gets the look for nothing, keeps the accessible name as it is, and keeps
  `SprintRow.test.tsx:44`'s `getByText("Active")` alive. `.chip-conflict`
  (`App.css:419`) still does exactly this after #61's stylesheet rewrite, so it
  is the repo's own pattern and not a new one.
- The goal is drawn under the sprint name. `SprintRow`'s comment says it is kept
  off the row because every cell clips and the detail panel narrows the pane. The
  mockup answers that by giving the name column the widest track and letting the
  goal clip with a `title` attribute, the way the name already does. The goal
  also stays under the expanded sprint (`SprintList.tsx:302-303`), because that
  copy handles the empty case and the row does not.
*Still true:* `SprintRow.tsx` and its test are untouched by #61, and
`.chip-conflict` kept its `text-transform` when it moved onto tokens.

**D5. New state tokens live in `frontend/core/styles/tokens.css`**, declared in
both `:root` and `:root[data-theme="dark"]`.
*Amended by #61.* The old wording said the gate on them was a new case in
`instruction-gate.test.ts` whose pattern would also cover `--chart-` so that 05b
inherited it. Both halves are stale: the chart tokens exist and ship, and
`theme.test.ts` is where this repo now reads stylesheets and token tables. The
gate is one case in `theme.test.ts` over every colour token in `tokens.css`,
which is broader than a prefix regex, passes on all 59 today, and needs no
second edit when the next family of tokens is added. Colour *values* are still
checked by the `/browse` pass in the whole-bundle gate for anything
`theme.test.ts`'s contrast block does not cover, because `css: false` puts a
computed colour out of reach of any unit test.

**D6. Edge cases that must each have a test.** A single-day sprint, in
`sprintRelative` and in the row; a sprint with no readable dates, drawing no
timeline; an active sprint past its end; a **draft sprint** from bundle 01
(negative id, `draft: true`, possibly no dates, drawing the Draft badge and
`not created in Jira yet` and no timeline); a sprint carrying a bundle 04
`waiting` value, which shows its state badge **and** the waiting chip; a **closed
sprint whose membership is not cached**, which draws its time bar, no points bar
and an honest scope cell; the local day boundary, in `day` and in `sprintRelative`.
*Still true:* unchanged.

**D7. No column header row on the sprint tree.** Mockup d1 draws
`SPRINT / STATE AND DATES / TIMELINE AND PROGRESS / SCOPE` above the rows. The
spec's prose does not ask for it, the tree is a `role="tree"` and a header row
inside one is neither a `treeitem` nor a column header anything can associate
with, and every cell in the redesigned row already names itself in words. Not
built.
*Still true:* unchanged.

**D8. The user guide is not touched.** The spec's task 8 names it, but
`docs/user-guide/USER_GUIDE.md` is XTM's guide and has no TAM Sprints section.
Docs go to `agents/project/tam-phases.md` (G10).
*Still true:* unchanged.

**D9. The per-pass closed-membership budget is a named constant, not a setting.**
Twelve, in `syncer/boards.go`, with the reason in its comment. It is not in
profile settings and it is not a binding: nobody has asked to tune it, and a
constant that turns out to be wrong is one commit away from being right.
*Still true:* unchanged.

**D10. The state tokens alias the palette #61 tuned rather than adding colours.**
New in this refresh, and the reason Task 1 step 4 is shorter than it was. #61
built a six-hue chip palette with a measured dark counterpart for every pair,
and `.chip-status-active`, `.chip-status-done`, `.chip-status-todo` and
`.chip-draft` all read from it. Eleven of the twelve tokens this bundle declares
are therefore a semantic name over a value that is already right in both themes;
only `--today-marker` is a colour nothing else in the suite draws. A thirteenth
blue would be a second answer to a question #61 answered, and it would put a hex
value in play where `hardcoded_hex` has zero headroom on TAM.

## Test traps this bundle walks past

From `agents/project/testing.md`, the ones these tests would otherwise hit:

- **A `<dd>` takes no accessible name from its `<dt>`.** The row is not a `<dl>`,
  but the same rule is why the bars carry `aria-valuetext` and are queried with
  `getByRole("progressbar", { name })` rather than by their surrounding text.
- **A sentence in a banner and in `LiveRegion` is in the DOM twice.** The Sprints
  view announces its states; scope to the banner.
- **`getByText` finds content inside a collapsed `<details>`.** The goal now
  appears both on the row and under the expanded sprint, so an unscoped
  `getByText(goal)` will find two once a row is expanded. Scope with `within`.
- **Waiting for an enabled control then picking an option is a race.** The
  Sprints view's board select is populated by a query. Wait for the option, not
  the control.
- **Mock factories that spread the real module fail open.** The row tests take
  fixtures as props and mock nothing.
- **A Go test that lets the new code create the state it is meant to migrate
  proves nothing.** Task 2's regression test seeds a first pass, runs a second,
  and asserts both the surviving rows and the call count; a one-sided version
  passes against code that re-fetches every time.
- **jsdom applies no stylesheet, so no render test can see a cascade bug.** #61's
  lesson and the reason `styles.test.ts`, `theme.test.ts` and `appearance.test.ts`
  exist. Anything this bundle asserts about a rule rather than about markup goes
  through `tam/frontend/src/test/cssRules.ts`, not a second reader.

## Whole-bundle gate, before the PR

```
npm test --workspaces --if-present
npm run typecheck --workspaces --if-present
npm run lint
sh scripts/lint-report.sh && sh scripts/ratchet.sh    # must say counts within baseline
cd tam && go test ./... -count=1
cd tam && go vet ./...
cd tam && wails build
```

CI's regenerate-and-diff job is the check that no binding changed; a diff from it
is a finding, not something to commit.

Then the visual pass the spec asks for, through `/browse` on the demo profile:
Sprints, light and dark, at 1280px and at 800px, plus one 400px check that the
narrow rules hold. The demo dataset has one closed sprint, Sprint 11
(`tam/internal/backend/demo/boards.go:111`), so the pass covers both the
before-the-backfill row and the after-it row if the profile is re-synced between
screenshots. Take both. Screenshots go on the PR.

## Not in this bundle

Noticed while reading the code, outside the spec, and deliberately left alone.
Two entries the old plan carried are gone because #61 did them, and they are
named here so nobody goes looking for work that is finished.

1. **XTM does not import the shared stylesheets.** It carries its own `.sr-only`
   (`xtm/frontend/src/App.css:67`), its own token blocks and all 441 of the
   `hardcoded_hex` count, so any primitive exported from `@agile-suite/core`
   renders unstyled there. It imports neither primitive this bundle adds, so
   nothing breaks; moving XTM's chrome onto the shared shell is its own change,
   and #61 recorded that XTM is heading out of this repo, which is why the class
   gate is clean over `tam/frontend` and `frontend/core` only.
2. **`instruction-gate.test.ts` does not police project names inside
   `frontend/core/src`.** Fourteen files there mention one today. Extending
   `FORBIDDEN_IN_CORE` to that tree is fourteen files of unrelated work.
3. **`SprintsView.tsx` (483) is over C2's 400** and is one of the files the
   ratchet counts. Splitting it is its own change, not something to do while
   redesigning a row. `SprintList.tsx` was on this list at 427 and is no longer:
   #61 took it to 386.
4. **A `membership_synced` stamp on the `sprint` row.** It is the exact answer to
   "has this scope been read", it would retire the one ceiling in Task 2, and it
   would also fix the ambiguity `sprintlist.go:70-79` already complains about for
   a partly-synced future sprint. It costs a migration for one bit, and the bit
   is only wrong for a closed sprint that genuinely holds nothing of this
   project. Add it when that shows up in the field.
5. **Done by #61, not by this bundle.** The old list's item 3 said `.chip-held`
   and `.chip-conflict` name a `--warn-bg` token that does not exist and rely on
   their hardcoded fallbacks; both now read `--warning-bg` and `--warning-text`,
   which are real tokens in both themes. The old list's item 6 said everything in
   05b was out of scope here; all of it shipped in #61 under issue #60, including
   `lib/chartScale.ts`, the chart tokens, the `ResizeObserver` handling in
   `useChartWidth.ts`, the hidden tables, and keyboard navigation along a chart's
   points in `usePointFocus.ts`.

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope and strategy | 0 | not run | optional, no product direction change |
| Codex Review | `/codex review` | Independent second opinion | 0 | not run | outside voice not run |
| Eng Review | `/plan-eng-review` | Architecture and tests (required) | 1 | CLEAR | 16 findings, 3 critical, all resolved into this split |
| Design Review | `/plan-design-review` | UI and UX gaps | 0 | not run | the spec carries approved mockups d1 and d2 |
| DX Review | `/plan-devex-review` | Developer experience gaps | 0 | not run | no developer-facing surface |

**The three critical findings, and what they changed.** The single plan claimed
every number it drew already reached the frontend, and that no Go change was
needed. False for the rows half: a closed sprint has no cached membership, so
`total`, `done`, `points` and `donePoints` are structurally zero on the most
common row, and "N carried over" exists only inside the reports machinery. The
user chose to fetch and keep a closed sprint's membership rather than hide the
bars, which reverses the decision recorded at `tam-phases.md:911`. This plan
owns that reversal, its budget and its stated ceiling. The spec's chart tooltip
and keyboard navigation had been dropped with no task and no note; they shipped
in #61 instead of in 05b.

**Scope.** The user split the work: rows here, charts in 05b. #61 landed the
charts ahead of both, so this plan is now the remainder of spec 05 on its own.

**Refreshed 2026-09-22 against `main` at `329f428`, no new review run.** #61
retired Task 1's chip-recolouring step and replaced its proposed dark mode gate
with one case in the `theme.test.ts` #61 added; it shipped the whole of 05b, so
every cross-reference to that bundle is gone; it added a class-name gate and the
`hardcoded_hex` and `hardcoded_px_font` counters, all three now in the global
constraints; and it took `SprintList.tsx` to 386 lines, which lifts the
no-growth rule Task 2 carried. `files_over_400` moved 107 to 106 and
`eslint_problems` 199 to 196; the recorded test counts moved 46 to 253 in
`frontend/core` and 507 to 978 in `tam/frontend`, which Task 5 now corrects in
`testing.md`. Tasks 2, 3 and 4 keep every eng-review decision intact, and no
decision was invalidated: D5 is the only one amended, and it is amended in the
direction of a gate that already exists.

**UNRESOLVED:** none. Every finding was settled in the revision.

**VERDICT:** ENG CLEARED. Open an issue, then implement.
