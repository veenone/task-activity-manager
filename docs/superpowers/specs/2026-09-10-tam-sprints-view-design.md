# Task Activity Manager: the Sprints view

Phase 3 gave TAM sprints on a board. It can draw one, drag cards through it, start it and close
it. What it cannot do is make one, rename one, fix the dates somebody typed wrong, delete one that
was created by mistake, or look at a sprint's contents without first choosing the board that
happens to carry it. This design adds the view that does those things.

It is the fourth view, between Boards and Reports, and the first view in TAM that is not always
there: a project with no scrum board cannot have sprints, and a menu entry that leads to a page
explaining why it is empty is worse than no entry.

## 1. What this delivers

- A **Sprints view**: a board picker, then that board's sprints as a two level tree, each sprint
  expanding to the issues in it, with a detail panel beside it exactly as the Epics view has.
- **Creating a sprint**, with the name and dates the board's own history suggests.
- **Editing a sprint**: its name, its goal, and its dates.
- **Deleting a sprint**, with a confirmation that says where its issues go.
- **Starting and completing** a sprint from here as well as from the Boards toolbar, sharing one
  definition of what "unfinished" means rather than growing a second.
- **The view hiding itself** when the profile has no scrum board, in the view tabs, the nav rail,
  and the native menu bar.

## 2. What this does not deliver

Creating or editing a board. Moving a sprint between boards, which Jira does not offer either.
Adding several issues to a sprint at once from here, because the board's multi selection already
does that and doing it twice would mean two selection models. Reordering sprints, which is not a
thing Jira has. Any change to the burndown or velocity work, which is Phase 4 and reads the
sprints this view will produce.

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
the same path the board drag and the detail panel already use. The line this design draws is that
**the sprint is a Jira object and goes now; what is in it is the user's work and waits for
Commit.**

## 4. Decisions

| Question | Decision | Why |
|---|---|---|
| Where sprint management lives | Its own view, between Boards and Reports | It is about the sprint rather than the cards in it, and the Boards toolbar can only ever act on the sprint already selected there |
| The view's shape | A board picker, then sprints as a two level tree with their issues beneath, and a detail panel | It is the Epics view's shape, which is the one TAM already teaches for "a thing and what hangs off it" |
| Why a board picker rather than every board at once | A sprint belongs to a board, and creating one requires the board's id | Grouping by board would make a three level tree of what is really a filter, and the create dialog would still have to ask |
| When the view appears | The profile has at least one board of type scrum in the cache | A kanban only project and an instance without Jira Software both genuinely cannot have sprints. Conditioning on "has sprints" instead would mean the first sprint could never be created, since this view is where it is created |
| How the native menu learns the condition | The frontend tells Go through a bound setter that rebuilds the menu, the way the nav rail checkbox already does | Go has no notion of an active profile; the frontend is the only side that knows which profile is on screen |
| Creating a sprint | A dialog with name, goal, start and end, prefilled from the board's own numbering and its usual sprint length | The same prefill `SuggestSprintDates` already computes for the start dialog, and a sprint created with plausible dates is one fewer edit later |
| The state a new sprint is in | Future, always | Jira's create endpoint makes a future sprint and offers no other option; starting it is a separate, deliberate act |
| Editing a sprint | Name, goal and dates, on a future or an active sprint | These are the four fields Jira lets a client change, and they are the four the create dialog already has |
| Editing a closed sprint | Refused | Its dates are what velocity and burndown are computed from, and rewriting history silently changes charts nobody is looking at |
| Deleting a sprint | Future sprints only, behind a confirmation naming how many issues it holds and that they return to the backlog | Deleting an active sprint strands work a team is doing right now, and it cannot be undone from TAM or anywhere else |
| Deleting a sprint with pending journal rows | Refused, naming Commit as the thing to do first | The same guard a completion already takes, for the same reason: those rows point at a sprint that is about to stop existing |
| Starting and completing from here | The same two dialogs the Boards toolbar opens | One dialog, one definition of unfinished, one place to fix a bug in either |
| What "unfinished" means | An issue whose status id is not in the board's last column | Phase 3c's definition, which now has to be readable without a drawn board, so it moves into a helper both views call |
| Membership | Read only here: expand a sprint to see its issues, and change one through the detail panel's Sprint field | That field was built in the previous branch and is journaled, reviewed, and already the answer everywhere else |
| A closed sprint's contents | Shown as unavailable rather than as empty, with the reason | The boards sync deliberately never fetches a closed sprint's membership, so an empty list here would be a lie |
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

## 9. Hiding the view

Three places have to agree, and today none of them can express a condition.

`ViewInfo` gains a flag saying the view is conditional. The tabs and the nav rail filter on it,
and `App.tsx`'s `menu:view` guard filters on it too, because a hidden view that is still in
`VIEWS` would otherwise be reachable from a stale accelerator.

The native menu is the awkward one. It is built in Go, once, at launch, before any profile is on
screen, and Go has no notion of which profile the user is looking at. So the frontend tells it:
one bound setter, called when the condition's query resolves and again on a profile switch, which
stores the answer and rebuilds the menu. That is precisely the shape `SetNavRailVisible` already
has, including the rebuild, which exists because Wails renders a menu item from the value it was
built with.

The answer is also persisted as a shared setting, so the next launch builds the menu right the
first time instead of showing the view appearing a moment after the window opens. It is a hint,
not a source of truth: the frontend corrects it as soon as it knows.

The condition itself is a cache read, whether the profile has a board of type scrum. A profile
that has never refreshed its boards has none, so the view is absent until the first boards
refresh, which happens in the Boards view. That is a real consequence and the Boards view is where
a user would look anyway.

## 10. Errors

A 403 on any of the three says the account cannot manage sprints on this board, and the buttons
stay enabled, because a permission guessed at in advance hides capability from people who have it.
Every Jira sentence goes through `internal/errtext` first, since a Data Center answering with an
HTML login page would otherwise put a kilobyte of markup in a dialog.

A create or an edit that reached Jira and whose cache refresh then failed reports the same one
line "press Refresh" note both ceremonies already carry, and returns success, because the write
did happen.

## 11. Verification

On the demo profile: create a sprint, see it appear as future with the suggested dates, edit its
name and goal, start it, put an issue in it from the detail panel, Commit, complete it, then
delete a future sprint and read the confirmation. Switch to a profile whose project has only a
kanban board and confirm the view is gone from the tabs, the rail, and the View menu, and that
Ctrl and the accelerator does nothing.

On a real Data Center, the three things no fixture proves: that the create endpoint answers with
the sprint's id in the shape this expects, that a delete returns the issues to the backlog rather
than deleting them, and that an account without Manage Sprints gets a 403 rather than a 200 that
silently does nothing.

## 12. Out of scope, recorded

Editing a closed sprint's dates, which velocity would silently reinterpret. Deleting an active
sprint. Creating a board. A sprint's own goal shown anywhere outside its dialogs, which is a
Reports question. Bulk adding issues to a sprint from this view.
