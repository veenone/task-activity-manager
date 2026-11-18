# Worklog

A week of your own logged time. Your issues run down the side, the days run across the top, and each cell holds what you logged on that issue on that day. The Log work form opens beside the grid, so a day can be filled without opening each issue in turn.

Only your own time is shown. Entries other people logged on the same issues are left out.

## The grid

**Rows.** The first group, **Assigned to me**, is every issue assigned to you that is not done. Under it, **Also logged in these days** lists any other issue you logged time on in the week shown: one that is finished, or one that belongs to someone else. The filter box narrows the rows by key or summary.

**Cells.** A cell shows the time you logged, adding Jira's entries to the ones still pending. A dot marks a cell with a pending entry. An empty cell shows `+`.

**Day headers.** Each workday has a bar and a line of text measuring the day against the day target:

| Text | Meaning |
|---|---|
| `8h, complete` | The day reached the target. |
| `2h 30m short` | The day is over and came in under the target. |
| `nothing logged` | The day is over and has no entries. |
| `2h of 8h` | Today or a day ahead. It is not counted as short yet. |

Saturday and Sunday have no target. They show a total only when something was logged.

**Day target.** The number of hours you mean to log on a workday, 8 by default. It is kept per profile. The week total in the filter bar is measured against five times this.

**Moving around.** The arrows either side of the dates step a week at a time, and **This week** comes back to today. With a cell focused, the arrow keys move between cells and Enter opens the form.

## Logging work

Press a cell. The panel on the right opens for that issue and day and shows:

- How the day stands against the target.
- The **Log work** form: issue, time spent, started, comment.
- Every entry you logged that day, on any issue. A pending entry can be discarded here.

**Started** is filled in for you. It is the time the day's last entry ended, or 09:00 on a day with no entries, and it can be changed.

The buttons under **Time spent** fill the field in one press. **Fill the day** puts in exactly what the day is missing.

**Log work** journals the entry. Like every other change in TAM it reaches Jira when you [commit](00-getting-started.md).

The **Issue** list in the form changes which issue the entry goes on without closing the panel.

## Days to fill

When a workday in the last 14 days is under the target, a strip above the grid lists it. Picking a day moves the grid to that week and opens the panel on it, keeping the issue you were logging on.

**Log and go to next gap** logs the entry and moves the panel to the next day in the strip, so several days can be filled in a row.

## What it does not do

- It does not edit or delete an entry Jira already holds. Do that in Jira.
- It does not know about public holidays or leave. A day off shows as short until 14 days have passed.
- An issue you logged on that TAM has not synced, such as one in another project, is listed with its time but cannot be logged on from here.

## If the view cannot load

TAM learns your Jira username when it syncs or when you test the connection. Before that it has nobody to ask Jira about, and the view says so. Sync once, or open **Profiles** and test the connection.
