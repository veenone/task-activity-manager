# Sprints

The Boards view answers where a card is. This one answers what is in a sprint and what happens to it next: every sprint a board has, the work each one carries, and every action a sprint has, on one screen.

## The list

Pick a **Board** and the sprints appear as a tree: a sprint, and the issues in it beneath. The board's unassigned work is folded in as a row of its own, so nothing on the board is missing from this page.

Each sprint row carries its name, its state, its dates and a line saying what it holds: how many of its issues are done, and how many points have landed. A closed sprint whose figures have not been rebuilt yet says so rather than drawing a bar at zero, which would claim the sprint held nothing.

**Show closed** brings closed sprints back into the list. They are hidden by default.

## Move work between sprints

Select issues with **Space**, or with the checkbox, and extend the selection with **Shift** and an arrow. A bar appears with the count and a destination: another sprint, or **The backlog**. Moving them is an ordinary queued change, exactly like dragging a card on the board.

An issue the cache does not hold is skipped rather than guessed at, and the bar says how many of the selected cards actually moved.

## Create, start, complete

Each sprint's menu carries what it can do in its current state:

- **New sprint** drafts one on this board. A draft is marked as such and exists only here until Commit creates it in Jira.
- **Edit sprint…** changes the name, the goal and the dates. TAM warns before a name that duplicates another sprint's, and before an end date that moves.
- **Start sprint…** is offered for a future sprint, **Complete sprint…** for the active one. Both open the same dialogs the board uses.
- **Delete sprint…** is offered for a sprint that is not running.

Everything in that list is queued. A deletion says so plainly: Commit deletes the sprint in Jira, Jira moves its issues back to the backlog, and the issues themselves are not deleted. Until you commit, the deletion can be discarded from Pending changes; after, it cannot be undone.

Deleting a **draft** touches nothing in Jira. The cards moved into it go back where they came from.

Where TAM cannot be sure of a sprint's size, it says "at least" rather than quoting a number that might be low: some of its issues are outside the cache, the list was cut short, or a queued move has already taken cards in or out.

## The detail panel

Select any issue in the tree and the panel opens beside it, the same one the [Backlog](01-backlog.md) uses, with the same editable fields.

## What the banner says

A write leaves a sentence above the list: how many cards moved and where, or what a sprint action did. It is also where the reason appears when something was refused, since the dialog that asked closes on success.
