# Project Issue Explorer: User Guide

Project Issue Explorer (PIE) is a desktop app for people who look across Jira
projects rather than inside one. You give it a JQL filter and it lists what
matches, whichever projects those issues are in. This first version only reads
from Jira. It does not change anything there.

## Add a profile

A profile is how PIE reaches Jira. Choose **Add profile** and enter a name, your
Jira URL and a personal access token. PIE keeps the token in your operating
system's credential manager, not in its database.

PIE's profiles are separate from the ones Task Activity Manager and Xray Test
Manager share, because they have no project key. A profile set up in those apps
does not appear here, so you enter the URL and token again.

You can keep several profiles, for example one for each Jira account, and switch
between them from the list at the top of the window. **Delete profile** removes
the selected profile and its saved token from this computer.

## Run a query

Paste JQL into the box and choose **Run**. The table shows each issue's key,
project, type, summary, status, priority and last update.

If Jira refuses the query, PIE shows Jira's own message and leaves the previous
results on screen. A query that names a saved filter with `filter = 12345` fails
as a whole if your account cannot see that filter, even when the filter is only
one part of the query.

PIE fetches at most 5,000 issues from one run. When a query matches more, the
count above the table says how many of the total it fetched, so narrow the query
to see the rest.

Results are not saved. They are gone when you close PIE or run another query, and
PIE needs a connection to Jira to run one.
