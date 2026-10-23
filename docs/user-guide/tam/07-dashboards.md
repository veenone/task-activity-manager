# Dashboards

A dashboard is one of your saved Jira filters, counted and kept. The view holds as many as you pin, and each panel says what its filter matched the last time it was run.

![Two dashboards, each counting one saved filter](images/dashboards-01-view.png)

## Add one

Pick a filter from **Saved filter** and add it. The list holds the filters you have starred in Jira, under two headings: **Your filters** for the ones you wrote and **Shared with you** for the rest. A heading with nothing under it is not drawn, so a list of filters you all wrote yourself shows **Your filters** alone. TAM does not create or edit them.

Starring a filter in Jira is what puts it in this list. Jira Data Center has no way to list the filters you own, so one you wrote and never starred is in no list TAM can read.

If TAM cannot tell who you are in Jira, or your Jira names its users in a way that cannot be matched against a filter's owner, the list is a single ungrouped one of your starred filters, the same as it was before.

## What a panel shows

- **Issues**, **Points**, **Estimated** and **Logged** for everything the filter matched.
- **By status**, **By type** and **By assignee**, as rankings. A panel draws the top few rows rather than every status a large filter touches, which would stop it being a ranking.
- When it was last counted. A panel that has never run says **Not counted yet**.

## Refresh is yours to press

The figures are a snapshot, not a live query. That is deliberate: a dashboard has to say the same thing when Jira is unreachable, and the same thing again when you reopen it next week.

**Refresh** on a panel re-runs that filter against Jira and restamps it. Nothing refreshes on its own.

## Remove one

**Remove** takes the panel off this view. The Jira filter is untouched; TAM only forgets that you pinned it.

Nothing in this view writes to Jira. Reading a filter and counting what it matches is all a dashboard does.
