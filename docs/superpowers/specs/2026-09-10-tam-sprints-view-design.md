# Task Activity Manager: the Sprints view

Phase 3 gave TAM sprints on a board. It can draw one, drag cards through it, start it and close
it. What it cannot do is make one, rename one, fix the dates somebody typed wrong, delete one that
was created by mistake, or look at a sprint's contents without first choosing the board that
happens to carry it. This design adds the view that does those things.

It is the fourth view, between Boards and Reports.

## 1. What this delivers

- A **Sprints view**: a board picker, then that board's sprints as a two level tree, each sprint
  expanding to the issues in it grouped by the person carrying them, with a detail panel beside it
  exactly as the Epics view has.
- **Creating a sprint**, with the name and dates the board's own history suggests.
- **Editing a sprint**: its name, its goal, and its dates, including clearing a goal.
- **Deleting a sprint**, with a confirmation that says where its issues go.
- **Filling a sprint**: selecting several issues and moving them in with one action, journaled the
  way every other membership change is.
- **Starting and completing** a sprint from here as well as from the Boards toolbar, sharing one
  definition of what "unfinished" means rather than growing a second.
- **A marker on every write that reaches Jira immediately**, because by this design five of them
  do and nothing on screen tells them apart from the ones that wait for Commit.

## 2. What this does not deliver

Creating or editing a board. Moving a sprint between boards, which Jira does not offer either.
Reordering sprints, which is not a thing Jira has. A team roster with capacity per person, which
is Phase 4 work; the assignee grouping here is its seed. Any change to the burndown or velocity
work, which is Phase 4 and reads the sprints this view will produce.

## 3. The decision that shapes this plan

**Creating, editing and deleting a sprint go to Jira immediately. They are not journaled.**

Phase 3c made starting and completing a sprint the only writes in TAM that reach Jira outside a
Commit, fenced them into `internal/sprints`, and said in its own spec that this is the one place
in TAM where a button talks to Jira without Commit. This design extends that exception to three
more actions and amends section 14.2 of the boards spec to say so, rather than leaving a sentence
there that has quietly become false.

The reasoning that justified the first exception applies unchanged. A sprint is not a private
draft: it is a container a whole team plans into, and a sprint that exists only in one person's
laptop is a sprint nobody else can move an issue into. There is nothing to reconcile, either. A
card move can be rebased onto a status that moved underneath it; a sprint somebody else already
deleted cannot be renamed, and the only sensible answer is to say so and refresh.

The cost of the alternative is what settles it. Journaling a create would mean inventing a local
sprint id, the way a draft issue gets `TAM-NEW-n`, and then teaching the commit pass to rekey
every `issue_sprint` journal row that points at the placeholder once Jira hands back the real id.
That is the draft rekey machinery built a second time, for a different id space, to defer a write
that takes one round trip and that the user is going to want to see in Jira anyway. It buys
offline sprint creation, which nobody asked for, and it puts the placeholder id in the board
cache, the Backlog's sprint filter, and the importer's Sprint column, all of which would then have
to know that some sprint ids are not real.

Membership is untouched by this: moving an issue into or out of a sprint stays journaled, through
the same path the board drag and the detail panel already use, including the bulk move this design
adds.

The honest statement of the line is worth writing down, because the tempting one is wrong. It is
not that a sprint is a Jira object and an issue is not: a new issue is every bit as much a Jira
object a team plans around, and TAM journals it behind a `TAM-NEW-n` placeholder. The real rule is
**a sprint's id has to be real before anything can point at it**, and TAM already carries the
machinery to defer an issue's id and nothing that would defer a sprint's. That is a cost, not a
principle, and calling it a principle is how a fourth exception gets added without an argument.

So the rule is fenced structurally rather than by paragraph. The immediate writes are exactly the
methods on the unexported `lifecycle` interface in `internal/sprints`, and a test asserts that
method set by name. Growing it means editing a failing test whose message says what the list is
for. The previous version of this rule lived in a spec sentence and lasted one phase.

## 4. Decisions

| Question | Decision | Why |
|---|---|---|
| Where sprint management lives | Its own view, between Boards and Reports | It is about the sprint rather than the cards in it, and the Boards toolbar can only ever act on the sprint already selected there |
| The view's shape | A board picker, then sprints as a two level tree with their issues beneath, and a detail panel | It is the Epics view's shape, which is the one TAM already teaches for "a thing and what hangs off it" |
| Why a board picker rather than every board at once | A sprint belongs to a board, and creating one requires the board's id | Grouping by board would make a three level tree of what is really a filter, and the create dialog would still have to ask |
| When the view appears | Always | Hiding it would have cost a cross cutting navigation mechanism for one consumer, and shown a new user nothing at all on their first launch, because the condition reads a cache a fresh profile has not filled yet. TAM already ships Reports and Rituals as visible entries that lead nowhere useful; a fourth one that explains itself is better than one that vanishes |
| What a project with no scrum board sees | An empty state saying sprints belong to a scrum board, that none is synced, and where to go | It teaches. A missing menu entry teaches nothing and cannot be asked about |
| Creating a sprint | A dialog with name, goal, start and end, prefilled from the board's own numbering and its usual sprint length | The same prefill `SuggestSprintDates` already computes for the start dialog, and a sprint created with plausible dates is one fewer edit later |
| The state a new sprint is in | Future, always | Jira's create endpoint makes a future sprint and offers no other option; starting it is a separate, deliberate act |
| Editing a sprint | Name, goal and dates, on a future or an active sprint | These are the four fields Jira lets a client change, and they are the four the create dialog already has |
| Editing a closed sprint | Refused | Its dates are what velocity and burndown are computed from, and rewriting history silently changes charts nobody is looking at |
| Deleting a sprint | Future sprints only, behind a confirmation naming how many issues it holds and that they return to the backlog | Deleting an active sprint strands work a team is doing right now, and it cannot be undone from TAM or anywhere else |
| Deleting a sprint with pending journal rows | Refused, naming Commit as the thing to do first | The same guard a completion already takes, for the same reason: those rows point at a sprint that is about to stop existing |
| Starting and completing from here | The same two dialogs the Boards toolbar opens | One dialog, one definition of unfinished, one place to fix a bug in either |
| What "unfinished" means | An issue whose status id is not in the board's last column | Phase 3c's definition, which now has to be readable without a drawn board, so it moves into a helper both views call |
| Membership | Read, grouped by assignee, and writable: one issue through the detail panel's Sprint field, several through a selection and one Move | Reaching a sprint without picking its board is why this view exists, and deferring the bulk move would send the user back to the board to fill it, which is the trip the view removes |
| The bulk move's write path | The journaled `MoveManyToSprint` the board's selection already uses | One write path, already reviewed in Phase 3b, with a conflict story and a Discard case that already exist |
| A closed sprint's contents | Shown as unavailable rather than as empty, with the reason | The boards sync deliberately never fetches a closed sprint's membership, so an empty list here would be a lie |
| Deleting and the sprint cache | Delete removes the sprint's rows itself; it does not go through `refreshSprints` | That helper refuses to persist an empty answer, justified by a comment saying a ceremony proves the board has a sprint. Delete is the one write that makes the answer genuinely empty, so the refusal would leave a deleted sprint in the cache forever, and from there into the Backlog picker, the Epics tree, the New issue dialog and the importer's Sprint column |
| Clearing a sprint's goal | Sent as an explicit empty string on edit, never on create | The partial update rule that stops an empty box wiping a real goal also makes a goal impossible to clear. The two cases are different and the code distinguishes them |
| A local record of an immediate write | An audit row for each of create, edit and delete | Once Jira no longer has the sprint, an audit row is the only trace that will exist anywhere on the machine |
| Telling immediate writes apart | A marker and a sentence on all five | An app whose promise is that you see every write before it happens must say which buttons break that promise |
| Permissions | A 403 says the account cannot manage sprints on this board, and the buttons stay | Phase 3c's rule: guessing at permissions before trying is how tools hide capability from people who have it |

## 5. The view

```
Sprints                                                    [Board: PLAT Scrum v]  [+ New sprint]
+--------------------------------------------------+  +--------------------------------------+
| v  Sprint 12          ACTIVE   18 Aug - 1 Sep     |  |  PLAT-412                            |
|      8 of 14 done, 21 of 34 points                |  |  [Story]  In Progress                |
|      [Complete]  [Edit]                           |  |                                      |
|      PLAT-412   Apply promo code       In Progress|  |  FIELDS                              |
|      PLAT-418   Refund a part order    To Do      |  |  Sprint    [Sprint 12          v]    |
|      ...                                          |  |  Assignee  ...                       |
| >  Sprint 13          FUTURE   1 Sep - 15 Sep     |  |                                      |
|      6 issues                                     |  |                                      |
|      [Start]  [Edit]  [Delete]                    |  |                                      |
| >  Sprint 11          CLOSED   4 Aug - 18 Aug     |  |                                      |
|      Contents are not cached for a closed sprint  |  |                                      |
+--------------------------------------------------+  +--------------------------------------+
```

The tree is `folder-tree` and its rows are `folder-item`, the classes the Epics tree already uses,
so the two views read as one app. A sprint row carries its name, a state chip, its dates, and a
progress line built the way `EpicNode` already builds one: done of total, and points of points.
The row's actions appear on the row rather than in a menu, because there are at most three of them
and they differ by state, so a menu would be three items long and mostly disabled.

Order is active first, then future by start date, then closed by start date with the most recent
first. Closed sprints are behind a "Show closed" toggle that starts off, because a board two years
old has fifty of them and none of them are what the view is for.

Selecting an issue opens the detail panel, which is the same `IssueDetailPanel` the Backlog and
the Epics tree use, and it receives the profile wide open sprint list so its Sprint field is a
choice here too.

## 6. The wire

Three new calls in `core/jira/agile.go`, beside the four Agile writes already there.

| Action | Request | Notes |
|---|---|---|
| Create | `POST /rest/agile/1.0/sprint` with `{name, goal, startDate, endDate, originBoardId}` | Answers with the created sprint, whose `id` is what the cache and the view need. The sprint is created in the future state; the endpoint offers no other |
| Edit | `POST /rest/agile/1.0/sprint/{id}` with only the keys that changed | The same partial update `StartSprint` uses, and the same trap: a key that is present overwrites and one that is absent is left alone, so an empty goal box must not be sent as `""` |
| Delete | `DELETE /rest/agile/1.0/sprint/{id}` | Jira returns the sprint's issues to the backlog. TAM does not move them itself, and the confirmation says which behaviour the user is getting |

`goal` is omitted when empty on create as well as on edit, for the reason `StartSprint`'s comment
already gives. Dates go in the Agile datetime format `internal/sprintdate` already owns; the
dialog's date inputs are bare dates and are converted there, exactly as the start dialog's are.

## 7. The seams

`backend.BoardBackend` gains three methods, and both implementations gain them: the Jira backend
delegating to the three calls above, and the demo backend mutating the same in run sprint overlay
that already makes `StartSprint` and `CompleteSprint` change state for the session. The demo has
to be a real implementation rather than a stub, because the acceptance walk through runs on the
demo profile and Phase 3c's worst defect was a demo backend that ignored an argument.

`internal/sprints` gains `Create`, `Edit` and `Delete` beside `Start` and `Complete`, and its
unexported `lifecycle` interface gains the three methods so a backend that cannot manage sprints
is refused with a sentence rather than a panic. The guards live beside the ones already in
`guards.go`: a closed sprint refuses an edit, a non future sprint refuses a delete, and a delete
refuses while journal rows point at the sprint.

Every one of the three re reads the board's sprints into the cache afterwards through the existing
`refreshSprints`, which already refuses to persist an empty answer, because a single 400 on that
endpoint is indistinguishable from "no sprints" and would otherwise delete the board's history.

The bound methods go in a new file rather than growing `app_sprints.go`, and they take the same
per profile lock under the same `"sprint"` label, which means the frontend reaches them through
`SyncContext.runSprintCeremony`, the path that already exists for exactly this.

## 8. The store

No schema change. The `sprint` table already holds everything the view lists, and the three writes
refresh it through the existing replace path.

One new read, in its own file in `internal/boardrepo`: the sprints of one board with their dates
and their cached issue keys, which is what the tree draws. `OpenSprints` is not it: that read
exists for a picker, drops the dates and the board id on purpose, and is deliberately folded to
one row per sprint.

## 9. The empty states

The view is always present. Three things it can find, and what each says.

**No scrum board on this profile.** Sprints belong to a scrum board; this project has none synced.
Go to the Boards view and refresh. This is the case an earlier draft of this design hid the whole
view for, and hiding was wrong for three reasons: it needed a conditional navigation mechanism
across `nav.ts`, `App.tsx` and the native menu for exactly one consumer; the condition reads a
cache that a brand new profile has not filled, so a first launch on a perfectly ordinary scrum
project would have shown nothing and explained nothing; and TAM already ships Reports and Rituals
as visible entries leading to placeholder pages, so a fourth view that vanishes would have
contradicted the app's own convention.

**A scrum board with no sprints.** Says so, and offers the create action. This is the state a new
board is in, and it is the one moment the create button matters most.

**Loading.** A state, not an absence. The board picker and the tree each say they are loading
rather than rendering as empty, because an empty render is a claim, and the previous branch
shipped a defect that was exactly this: a Sprint select that stated there were no sprints for the
first few frames.

The board picker itself is hidden when the profile has exactly one scrum board, with the board
named in the heading instead. A select whose list has one entry is friction on every visit for a
choice with no alternatives.

## 10. Errors

A 403 on any of the three says the account cannot manage sprints on this board, and the buttons
stay enabled, because a permission guessed at in advance hides capability from people who have it.
Every Jira sentence goes through `internal/errtext` first, since a Data Center answering with an
HTML login page would otherwise put a kilobyte of markup in a dialog.

A create or an edit that reached Jira and whose cache refresh then failed reports the same one
line "press Refresh" note both ceremonies already carry, and returns success, because the write
did happen.

## 11. Verification

**Before anything is built.** Three assumptions here are unverified against a real Data Center and
three tasks are shaped by all of them: that create answers with the new sprint's id in the shape
this expects, that a delete returns the issues to the backlog rather than deleting them, and that
an account without Manage Sprints gets a 403 rather than a 200 that silently does nothing. They
are three requests and ten minutes, and finding out after the code is written is the wrong order.
The code tolerates a create answering with no body regardless, because that costs nothing.

**On the demo profile.** Create a sprint, see it appear as future with the suggested dates, edit
its name and goal, clear the goal and confirm it stays cleared, start it, select three issues and
move them in, Commit, complete it, then delete a future sprint and read the confirmation. Delete
the last sprint on a board and confirm it does not come back. Switch to a profile whose project
has only a kanban board and confirm the view is present and says why it is empty.

## 12. Out of scope, recorded

Editing a closed sprint's dates, which velocity would silently reinterpret. Deleting an active
sprint. Creating a board. Carrying unfinished issues forward at create time, which is a completion
behaviour. A team roster with capacity per person. Remembering a 403 for the session so a user
without Manage Sprints is told once rather than once per action.
