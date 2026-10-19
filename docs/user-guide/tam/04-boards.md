# Boards

The Boards view is your Jira board: its columns, the cards in them, and the sprint ceremonies. Moving a card here changes the local cache and queues a pending change, so a board can be rearranged as often as you like and only the result is sent to Jira.

## Choose what you are looking at

The bar above the board carries:

- **Board**: the boards this project has.
- **Filter**: narrows the cards to a key, an assignee or a type. The board keeps its columns; only the cards thin out.
- **Sprint**: on a scrum board, which sprint's cards to draw. A board with no open sprint says so.
- **Swimlanes**: **None**, **Assignee** or **Epic**, which splits the board into horizontal lanes.

**Refresh** re-reads the board from Jira. It takes the same lock a sync does.

## Move a card

**With the mouse**, drag a card to another column, to another place in the same column, or onto a sprint. The card lands where you dropped it and carries a pending mark until the next commit.

**With the keyboard**, move focus with the arrow keys and:

- **Ctrl** with an arrow moves the focused card: left and right between columns, up and down within one.
- **Enter** opens the card in the detail panel.
- **Space** selects the card.
- **Shift** with an arrow extends the selection as you move.
- **Context menu** or **Shift+F10** opens the card's own menu, which lists **Move to <column>** for each column, each open sprint, and **Move to the backlog**.

A move the board cannot make is refused out loud rather than silently ignored, and nothing can be moved while a commit is running.

### Moving several at once

Select cards with **Space** or by clicking the checkbox, and a bar appears with the count, a destination picker and **Clear**. Choose a sprint or **The backlog** and the whole selection moves in one go.

The selection follows what the board is drawing: a card that a refresh or a filter takes off the board leaves the selection too, so an action can never touch a card you cannot see.

## Column limits

Under each column name is its work-in-progress limit, for example `4 of 6, from Jira` or `3, minimum 2, set in TAM`. A column over its limit, or under its minimum, says so and is marked.

The attribution matters:

- **from Jira** is the board's own configuration. TAM prints it and never changes it.
- **set in TAM** is a limit this team set here, for a column Jira does not limit. Press the limit to set or clear one.

A column Jira does not limit and TAM has no limit for reads **No limit, set one**, so a board with no limits anywhere still shows where they would go.

## Fill a board

**Add issues** opens a picker: search by key, summary or assignee, choose the issues, and say whether they go into the board's backlog or into one of its sprints. Issues already on the board are left out of the list.

**New board** drafts a board. Like a drafted sprint, it exists locally until Commit creates it in Jira.

## Sprint ceremonies

On a scrum board, the toolbar grows the ceremony that fits the sprint you are looking at:

- **New sprint** drafts one. It appears in the picker straight away, marked as a draft, and Commit creates it in Jira.
- **Start sprint** appears for a future sprint. The dialog offers a name, a goal and dates, suggesting them from the board's history where it can.
- **Complete sprint** appears for an active one. The dialog lists the cards that are not in the last column, says where they will go (a future sprint or the backlog), and names the column that decides it.

All three are queued, not sent. Nothing reaches Jira until Commit, which is also what makes it safe to complete a sprint, look at the result and discard it.

The list of unfinished cards in the Complete dialog is a preview built from the cache. Commit reads Jira again and moves whatever is unfinished then, so the two can differ if somebody closed a card in the meantime. Where the board is not drawing every card, the dialog says how many it cannot name.

## While work is pending

A card with an uncommitted move carries a pending mark, and a banner above the board says how much is waiting. Completing a sprint while other changes are pending asks you to commit first, because the completion is read against what Jira holds rather than what your screen shows.
