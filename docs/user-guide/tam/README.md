# TAM user guide

Task Activity Manager (TAM) is a desktop app for planning and tracking work in Jira Data Center: the backlog, epics, boards, sprints, the numbers a sprint review needs, and the Confluence pages a team keeps around its ceremonies. It syncs a project into a local cache, so the grid and the board are instant, and your edits stay on this machine until you commit them.

It shares its connection profiles with Xray Test Manager, so a project set up in one is already set up in the other.

There is one guide for each entry in the **View** menu. Start with Getting started if you have not connected a profile yet.

| Guide | What it covers |
|---|---|
| [Getting started](00-getting-started.md) | Profiles, demo mode, syncing, the local-first loop, committing, conflicts, settings |
| [Backlog](01-backlog.md) | The issue grid, filters, the detail panel, creating issues, importing, exporting |
| [Assigned to me](02-assigned.md) | The same grid, narrowed to your own issues |
| [Epics](03-epics.md) | The epic to story to task tree, and what hangs off an epic |
| [Boards](04-boards.md) | Columns, cards, moving work, swimlanes, column limits, sprint ceremonies |
| [Sprints](05-sprints.md) | Every sprint a board has, what is in each, and creating, starting and completing them |
| [Reports](06-reports.md) | Sprint figures, charts, velocity, people, and publishing to Confluence |
| [Dashboards](07-dashboards.md) | Saved Jira filters, counted and kept so they read offline |
| [Rituals](08-rituals.md) | Sprint ritual pages in Confluence, and the done agreement |

## Switching views

Each view has a tab in the band under the top bar. The **View** menu lists the same eight with the shortcuts Ctrl+1 to Ctrl+8:

| Shortcut | View | Shortcut | View |
|---|---|---|---|
| Ctrl+1 | Backlog | Ctrl+5 | Sprints |
| Ctrl+2 | Assigned to me | Ctrl+6 | Reports |
| Ctrl+3 | Epics | Ctrl+7 | Dashboards |
| Ctrl+4 | Boards | Ctrl+8 | Rituals |

**Ctrl+B** shows or hides the navigation rail down the left side, which lists the same views. The rail is off by default and its state is remembered.

## One thing to know before anything else

TAM writes nothing to Jira until you press Commit. Editing a field, dragging a card, filling a sprint, starting one, completing one, even deleting one: each is written to the local cache and queued as a pending change. The count sits in the top bar, and the dialog behind it is where changes are reviewed, discarded and sent.

Everything else in these guides assumes that loop. [Getting started](00-getting-started.md) walks through it.

## One document instead of nine

[USER_GUIDE.md](USER_GUIDE.md) is the same material as one numbered document, which is what the PDF is built from. The pages here are the ones to read on screen and the ones to edit; the single file carries the same content for distribution.

## About the figures

These pages have no screenshots yet. Where a guide names a button or a column, the name is the one on screen.
