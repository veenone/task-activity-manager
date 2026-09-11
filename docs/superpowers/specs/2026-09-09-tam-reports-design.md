# Task Activity Manager, Phase 4: the sprint report

Decided 2026-09-09, after Phase 3 closed. Revised 2026-09-11, after the Sprints view shipped and
two independent reviews took this apart. The user pre-approves the recommended option at every
decision point, so this records the choices and their reasons rather than the alternatives.

## 1. What this phase delivers

**The numbers a sprint review starts with, reconstructed from Jira's history rather than invented.**
Committed, added, removed, completed, carried over, for one sprint, plus the same figures for the
last closed sprints of a board. Phase 3 made TAM run a sprint; this is what a team reads
afterwards.

**The charts are a separate phase.** A burndown line and a velocity bar chart are drawn from
exactly this data and add no new facts, and there is no SVG anywhere in this frontend today, so
axis ticks, label collision, empty ranges, single day sprints, chart colour tokens and screen
reader access are all new surface. Splitting means the numbers get trusted before anything is
drawn from them, and it gets Phase 5 its input sooner, which is the real reason this phase exists:
Rituals has to publish a sprint's figures to Confluence, and this is where they come from.

## 2. The problem that shapes everything else

**A report is a history, and TAM has none.** The issue cache is a snapshot: it knows a card is Done
today and nothing about when it got there. The `sprint` table knows a sprint's dates and state. The
journal knows what *this user* changed locally and nothing about the rest of the team. Nothing in
TAM can say what the sprint looked like last Tuesday.

Four ways to get that history:

| Source | Verdict |
|---|---|
| Jira's own chart endpoints (`/rest/greenhopper/1.0/rapid/charts/...`) | Rejected. Jira Software internals: undocumented, unversioned, and the one thing in this app that would break on an upgrade with no warning and no recourse |
| TAM's journal and audit log | Rejected. It records what this user did on this machine, so a report drawn from it would describe one person's afternoon |
| Snapshot the sprint daily from now on | Rejected as the primary source, for one reason only: it cannot describe the sprint that closed last week, and the first thing anyone will do is open the sprint that just finished |
| **The issue changelog** | **Chosen.** `/rest/api/2/search` with `expand=changelog` returns every field change with its timestamp. It is documented, public, and it works retroactively on a sprint that closed months ago |

**The honest version of that argument, which an earlier draft got wrong.** This spec used to claim
the changelog's advantage was that it makes scope changes visible rather than invented. It does
not, and section 3 says why. The changelog's one real advantage over snapshotting is that it works
*backwards*: a snapshot can only describe sprints that happen after the user installs TAM, on days
the app was open. Everything else about the two is a wash, and the snapshot is better in one
respect, which is that it sees what the changelog route cannot.

## 3. What this cannot see, and why it says so out loud

**A card removed from a sprint is invisible to this phase.**

The issues are fetched with `sprint = N`, which is the only membership JQL Jira offers, and it
returns whoever is in the sprint **now**. A card dragged out on day four no longer carries sprint
N, so it is never fetched, its changelog is never read, and the removal leaves no trace. Jira has
no `WAS IN` operator for the Sprint field; its own report gets this right only because it keeps
private sprint-change records that the public API does not expose.

The alternative is to query every issue in the project updated during the sprint's window and
replay each one's changelog to work out who was in the sprint when. That is correct and it is a
large multiple of the work, with a changelog expansion on every issue, on a project where most of
them were never near the sprint. It is recorded as the upgrade, not built here.

So this phase reports what it can see and labels what it cannot:

- **Removed** counts only cards that left and came back, which is the sole case the query can
  observe. The field is named for that, and the summary says so in words.
- **Committed** is therefore a floor, not a total: it is what was in the sprint at the start
  *among cards still in it now*.
- Every surface that shows either number carries the qualification. A number that might be low and
  does not say so is the failure this phase exists to avoid.

## 4. The disagreement, and the answer to it

TAM's figures and Jira's will differ, in public, during a review, because they are computed from
different sources: Jira from its private sprint records, TAM from the public changelog, with its
own "done" rule, its own working days, and the blind spot in section 3.

**So the method is printed with the numbers.** One line under the summary: what done means here,
that history is reconstructed from the public changelog rather than Jira's stored sprint records,
and what it cannot see. It costs a sentence and it turns an argument in front of a team into a
footnote. Nobody reads documentation during a sprint review.

## 5. Decisions

| Question | Decision | Why |
|---|---|---|
| Where history comes from | The issue changelog, fetched with the sprint's issues in one paged search | Section 2 |
| Which issues | `sprint = N`, accepting the blind spot | Section 3 |
| When it is fetched | When a report is opened, not on every sync | A changelog expansion is the heaviest read this app makes, and a sync already takes minutes; a report nobody asked for should not cost that |
| Where it is kept | A `sprint_report` table holding the reconstructed series per sprint, keyed by board as well as sprint | A closed sprint's history cannot change, so it is fetched once and belongs to the user offline. Keyed by board because Jira hands one sprint to every board whose filter reaches it, and the done rule comes from the board's own last column, so two boards over one project would otherwise share one wrong answer forever |
| When a cached report is wrong | A stored `algo_version`, compared against a constant on read, rebuilding on mismatch | The first reconstruction bug would otherwise be baked into every user's database with no way out but deleting the file |
| A live sprint | Recomputed on demand, never cached | It changes hourly, and a stale report is worse than a slow one |
| What is measured | Story points, falling back to issue count when the sprint has no points at all, and the report says which and why | A board that does not estimate is common. "Why" matters: no Story Points field on this instance is a different problem from nothing estimated, and only one of them is the user's to fix |
| What "done" means | A status id in the board's last column, from one shared package | There are three definitions of done in this codebase today and the one this phase wants is unexported. Section 7 |
| When a sprint ended | Jira's `completeDate`, not its `endDate` | A sprint closed three days late measured at its end date mis-reports every number in the review, and TAM does not currently carry the field at all |
| Velocity | Committed and completed per closed sprint, for the last six, from the same reconstruction, each row carrying its own unit | A board that changed from points to cards mid-year would otherwise average two different things |
| Where it lives | The Reports view, which already exists as a placeholder with its own tab and accelerator | Nothing is added to the navigation; what it renders is replaced |

## 6. The wire

`core/jira`'s search learns an `expand` parameter. Two things about the answer that an earlier
draft assumed wrongly:

**The changelog is paginated and silently truncates.** It arrives as an object carrying `startAt`,
`maxResults`, `total` and `histories[]`, and a search expansion caps what it returns per issue. A
long lived card's history is cut, and a truncated changelog decodes identically to a complete one
unless those three numbers are read. So they are read, an issue whose history was cut is marked,
and a report that contains one says which cards it could not read in full rather than presenting
its numbers as exact.

**Sprint and Story Points are custom fields.** Their `fieldId` is `customfield_NNNNN` and differs
per instance, so a normaliser keyed on a literal field id matches `status` and silently never
matches either of the other two, which draws a flat line with no scope and no re-estimates on every
real instance while every fixture based test passes. TAM already resolves those ids per instance;
the changelog matching uses the same resolution, falling back to the display name only when
discovery failed.

## 7. The reconstruction

`tam/internal/reports` turns a sprint's issues plus their changelogs into a series. Three things
the first draft left for an implementer to invent, each of which would have produced a confidently
wrong answer:

**It runs backwards before it runs forwards.** The changelog gives today's fields plus a list of
deltas. Status, estimate and membership *at the sprint's start* are not known; they are derived by
unwinding the changelog from the present back to the start, and only then walked forward. A naive
forward replay from today's values draws a sprint that never happened, and an issue changed after
the sprint closed corrupts day one.

**A day is a local day.** Jira's timestamps carry the server's offset and are not RFC 3339, since
the offset has no colon; this repo already learned that once and wrote it down in
`internal/sprintdate`. Every timestamp is parsed through it. Days are bucketed in the machine's own
zone with boundaries at local midnight, day one being the local date of the sprint's start, and
that rule is stated where a reader will find it, because bucketing in UTC gives a team on the other
side of the world a day one that starts the previous afternoon.

**The Sprint field's changelog values are lists, not scalars.** A card in two sprints at once, which
is ordinary during a rollover, changes from `"12, 13"` to `"13"`. Membership is therefore "is this
sprint's id in the new set", computed per change, not a boolean toggle.

The rest of the rules, unchanged from the first draft and still right: an estimate that changed
mid-sprint takes effect on the day it changed, except that an estimate changed while the card was
outside the sprint takes effect when the card re-enters; the ideal line runs over working days; and
a card that was already done before the sprint started counts into committed with nothing left to
burn.

**"Done" gets its own package.** There are three definitions in this codebase: the board's last
column, used by the sprint completion; `backend.IsDone`, which matches on status name and powers
the Sprints view's numbers; and the frontend's `lib/unfinished.ts`. This phase needs the first, and
it is currently unexported and hanging off a service. It moves to its own small package that the
completion and this phase both call, and the docs record that the name based rule still exists and
where it is used, because two screens disagreeing about a closed sprint's done count is exactly the
kind of thing this phase makes visible.

## 8. The view

The Reports view, which exists today as a placeholder, gains a board picker and a sprint picker
following the Sprints view's rather than the Boards view's, which means hiding the board picker
when the profile has one scrum board and naming the board in the heading instead.

Then the summary: committed, added, removed, completed, carried over, in the sentence a review
starts with, with the method line from section 4 beneath it, and the velocity figures for the last
closed sprints as a table.

Empty states, each of which has to be distinguishable from the others: a board with no closed
sprint; a sprint with no estimates, saying which of the two reasons applies; a sprint with no dates,
which cannot be reported on at all; a report whose changelog could not be read, which keeps the
last one on screen and names the failure with a retry; a report containing cards whose history was
truncated; and the one the first draft missed entirely, a read refused because the profile is busy
with a sync or a commit.

## 9. Errors and cost

A changelog fetch that fails leaves the previous cached report on screen with the failure named,
and never a half built series.

**One binding, not two.** The first draft had the view call for the burndown and the velocity
separately, each taking the per profile lock. That lock refuses rather than waits, so the two
concurrent calls a view makes on mount would have meant one of them failing with "a report is
already running for this profile" every single time it opened. One call returns both, under one
lock, sharing one fetch.

**The cost is real and is reported.** A sprint of two hundred issues is several pages, each far
heavier than anything TAM fetches today, and velocity multiplies it. So: a smaller page size than
the sync uses, progress reported the way every other long read in this app reports it, cancellation
when the user leaves the view, and velocity reusing stored reports for the sprints it has already
built rather than refetching all six.

## 10. Verification

Before anything is built, four Jira behaviours this phase rests on are checked against a real
instance, the way the previous phase's wire probe was: whether a search expansion truncates a long
changelog and what its `total` says, what the Sprint field's changelog entries actually look like
on this instance, that `sprint = N` really does not return a card that left, and what page size is
tolerable with the expansion on.

Go: the expansion against the httptest server including paging and a truncated changelog; the
rewind-then-walk over a sprint with a card added mid-sprint, one that left and returned, one
re-estimated inside the sprint and one re-estimated outside it, one done before the sprint started,
one changed after the sprint closed, and one with no estimate; the local day bucketing across an
offset; the working day ideal line; the fallback to counting cards with each of its two reasons;
velocity over six sprints, over none, and over a board whose unit changed.

Vitest: the summary reads its sentence, the method line is present, each empty state including the
busy one, and a failed fetch keeping the previous report.

Offline: on the demo profile, open Reports for the closed sprint and read the summary. The demo
dataset has one closed sprint and no changelog of any kind, so either it gains a curated history or
the walk through says velocity needs a real instance.

## 11. Out of scope

The burndown and velocity charts, which are the next plan and are drawn from this data. A
cumulative flow diagram, a control chart, an epic burndown, cross project reporting, and exporting
anything as an image. Querying the whole project to recover removed cards, recorded in section 3.
Daily membership snapshots, which would make removals visible going forward and are the one asset
Jira cannot reproduce; worth doing, not here.
