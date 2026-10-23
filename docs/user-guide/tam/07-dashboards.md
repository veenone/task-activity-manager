# Dashboards

A dashboard is one of your saved Jira filters, counted and kept. The view holds as many as you pin, and each panel says what its filter matched the last time it was run.

![Two dashboards, each counting one saved filter](images/dashboards-01-view.png)

## Add one

Pick a filter from **Saved filter** and add it. The list holds the filters you have starred in Jira, under two headings: **Your filters** for the ones you wrote and **Shared with you** for the rest. A heading with nothing under it is not drawn, so a list of filters you all wrote yourself shows **Your filters** alone. TAM does not create or edit them.

Starring a filter in Jira is what puts it in this list. Jira Data Center has no way to list the filters you own, so one you wrote and never starred is in no list TAM can read.

If TAM cannot tell who you are in Jira, or your Jira names its users in a way that cannot be matched against a filter's owner, the list is a single ungrouped one of your starred filters, the same as it was before.

## Add one by its id

**Filter id** is the way in for a filter the list above cannot offer, which is any filter you have not starred. The id is the number in the filter's own Jira URL, for example the `25475` in `.../issues/?filter=25475`. Type it and press **Add by id**; the name and the search both come from Jira, so there is nothing else to fill in.

An id Jira will not resolve, because it does not exist or because your account cannot see that filter, is reported beside the box and adds nothing.

## Opening a Jira dashboard

**Jira dashboard** lists the dashboards your Jira account can see and **Open in Jira** opens the one you chose in your browser. The row is only there when there are dashboards to open.

That is all this does. A Jira dashboard is a page of gadgets, and Data Center offers no way to read what gadgets a dashboard holds, so TAM cannot turn one into panels here. Counting a saved filter is what a TAM dashboard does, and the two things share a word and nothing else.

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
