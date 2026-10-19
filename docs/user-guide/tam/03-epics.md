# Epics

The Epics view is the project as a tree: each epic with the issues that hang off it, and the issues that hang off nothing. It answers how much of a piece of work has landed, which the flat backlog cannot show.

## The tree

Each epic is a row with its key, its summary and its progress: a bar and a count, for example `7 of 12 done, 21 of 34 pts`. The points half only appears when the issues carry estimates. Open an epic to see its children; a child with subtasks opens again.

Under the epics is everything with no epic, so work that was never filed under one is visible rather than missing.

The line above the tree counts what you are looking at: epics, issues, and how many have no epic. A project larger than 5,000 issues is cut off at that point, with a note asking you to narrow the filter.

## Narrow it

- **Search** matches a summary, a key or a label, across epics and their children.
- **Sprint** narrows to one sprint.
- **Show done** includes completed work, which is off by default so a long-running epic shows what is left.
- **Expand all** and **Collapse all** work on every epic currently listed.

## Work on an issue

Select any row, epic or child, and the detail panel opens beside the tree. It is the same panel the [Backlog](01-backlog.md) uses, with the same editable fields.

Changing an issue's **Epic** in that panel moves it in the tree as soon as the change is saved, and the row flashes where it lands so a move of a dozen rows is visible rather than something to hunt for. Like every other edit, the move is queued and reaches Jira on Commit.

## Create an epic

**+ New epic** opens the same dialog the Backlog uses, with the type set to epic. The epic appears in the tree as a **Draft** until Commit creates it in Jira.
