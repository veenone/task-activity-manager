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
- **A marker wherever the point of no return is**, because five writes here reach Jira the moment
  they are pressed, in an app where everything else waits for Commit, and nothing on screen tells
  them apart.

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
| Telling immediate writes apart | One chip and one sentence, in the four dialogs and the confirmation, never on a menu item that only opens one of them | An app whose promise is that you see every write before it happens must say which buttons break that promise, and the point of no return is the honest place to say it |
| What colour that marker is | The accent, never amber | Amber already means "held locally, waiting for Commit" in this app: the pending dot, the draft chip, the moved row flash. A marker meaning the opposite, painted amber, would invert the app's own vocabulary and read as a warning about a normal action |
| A sprint's goal | Added to `RawSprint`, `backend.Sprint`, `boardrepo.Sprint` and the `sprint` table, at schema version 7 | It existed at no layer. Without it the row cannot show a goal, the edit dialog overwrites one blind, and clearing a goal cannot be told from leaving it alone |
| Permissions | A 403 says the account cannot manage sprints on this board, and the buttons stay | Phase 3c's rule: guessing at permissions before trying is how tools hide capability from people who have it |

## 5. The view

```
Sprints                                                   [Board: PLAT Scrum v]  [+ New sprint]
Sprint 12, day 6 of 14, 8 of 14 done, 21 of 34 pts
+---------------------------------------------------+  +--------------------------------------+
| v Sprint 12  Active  18 Aug to 1 Sep  8/14   [...] |  |  PLAT-412                            |
|     Ship the promo engine before pricing lands     |  |  [Story]  In Progress                |
|     Ana Silva, 4 issues, 13 pts                    |  |                                      |
|     [ ] PLAT-412  Apply promo code    In Progress  |  |  FIELDS                              |
|     [ ] PLAT-418  Refund a part order To Do        |  |  Sprint    [Sprint 12          v]    |
|     Unassigned, 2 issues, 5 pts                    |  |  Assignee  ...                       |
|     [ ] PLAT-455  Audit the tax table To Do        |  |                                      |
| > Sprint 13  Future  1 Sep to 15 Sep     6   [...] |  |                                      |
| > Sprint 11  Closed  4 Aug to 18 Aug         [...] |  |                                      |
| > Unassigned on this board              31         |  |                                      |
+---------------------------------------------------+  +--------------------------------------+
```

The tree is `folder-tree` and its rows share `folder-item`, `folder-selected` and `folder-caret`
with the Epics tree, so the two views read as one app. The sprint row's own cells are
`.sprint-row` and `.sprint-cell`, because `.epic-row`'s grid tracks are fixed and genuinely
different; reusing them would mean sizing sprint content to an epic's columns.

A sprint row carries five things and a caret: name, a state chip, its dates, a progress line built
the way `EpicNode` already builds one, and a menu trigger. **The goal is not one of them.** It is
the only field of a sprint that is a sentence rather than a token, every cell in these trees clips
with an ellipsis, and opening the detail panel narrows the pane, so a goal column would be empty
on most rows and clipped to nothing on the rest. It renders instead as the first line inside an
expanded sprint, where it has the width to be read once, which is how a goal is read.

**The actions are a menu, not buttons on the row.** Inline buttons would need fixed tracks sized
for their widest label and empty on every row that does not offer them; the existing on-row action
pattern reveals itself only on hover, which would hide the app's one irreversible action from
every keyboard user; and the tree is a roving tabindex with a single tab stop, which is the exact
constraint `CardMoveMenu` was built for and already solves.

**An assignee group is a separator, not a node.** A person has no detail panel to open, no
children to expand, and no sensible answer for the arrow keys, so making one a tree item would put
the first member in the tree that neither opens nor toggles. The issues stay the only tree items
under a sprint, which is what keeps this a two level tree.

Order is active first, then future by start date, then closed by start date with the most recent
first, then the board's unassigned work last. Closed sprints are behind a "Show closed" toggle that starts
off, because a board two years old has fifty of them and none of them are what the view is for.
Only the active sprint starts expanded, because a board with twelve future sprints would otherwise
paint twelve open branches.

**The board's unassigned work is in the tree because otherwise a sprint cannot be filled here.** A
tree of sprints holds only issues already in a sprint, so a selection over it could move work
between sprints and never into a new one, and filling the sprint you just made would still mean
leaving for the board.

The cache does not store that work as a scope, and it is worth being exact about why, because the
obvious reading of the schema is wrong. `board_issue` does hold a scope with an empty sprint id,
but that scope is `BoardIssueKeys(board, "", project)`, the board's **entire** issue list with its
sprint issues included, which is what TAM's own code calls "the board's own list". Rendering it as
a backlog would list every sprint's issues a second time and would offer the fill bar work that is
already in a sprint. So the unassigned node is computed: the board's own list, minus every key the
sprint scopes hold once the journal has been replayed over them.

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

Create and edit re read the board's sprints into the cache afterwards through the existing
`refreshSprints`, which refuses to persist an empty answer because a single 400 on that endpoint
is indistinguishable from "no sprints" and would otherwise delete the board's history. **Delete
uses a variant that permits an empty answer**, because deleting a board's only sprint makes the
empty answer true; everything else about the path is shared, including the orphan membership
cleanup `ReplaceSprints` already does.

The bound methods go in a new file rather than growing `app_sprints.go`, and they take the same
per profile lock under the same `"sprint"` label, which means the frontend reaches them through
`SyncContext.runSprintCeremony`, the path that already exists for exactly this.

## 8. The store

**One column, and it is one this app should always have had.** A sprint's goal does not exist
anywhere in TAM: not in `core/jira`'s `RawSprint`, not in `backend.Sprint`, not in
`boardrepo.Sprint`, and not in the `sprint` table. Jira has been sending it on every sprint read
since Phase 3a and TAM has been dropping it on the floor.

Nothing here works without it. The row cannot show a goal, the edit dialog opens with an empty box
over a goal the sprint already has and the user types over it blind, and clearing a goal is
impossible to implement honestly, because the code cannot tell "the user left this alone" from
"the user emptied it" without knowing what it was. So the column is added at all four layers, with
schema version 7 and a migration that adds it in place rather than dropping the table: this is a
cache, but dropping it would empty every board's sprint picker until the next sync, for a column
that back fills itself on the first read.

Everything else the view needs is already stored. One new read, in its own file in
`internal/boardrepo`: one board's sprints with their dates, their goal, and **their issues**,
which is what the tree draws. Keys alone would not do: a row shows a summary and a status, the
grouping needs an assignee, and the detail panel needs a whole issue, so returning keys would
force a second query and lose the single snapshot the read exists for. The board's unassigned work
is computed rather than read, for the reason given in section 5. `OpenSprints` is
not this read: it exists for a picker, drops the dates and the board id on purpose, and is folded
to one row per sprint.

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
