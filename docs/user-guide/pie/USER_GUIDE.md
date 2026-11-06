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

## Try PIE without Jira

Enter `demo` as the Jira URL to make a demo profile. It needs no token, and
the header shows a yellow DEMO chip while it is selected. A demo profile never
contacts Jira and saves nothing in the credential manager.

Demo answers every query with the same 63 invented issues in six invented
projects, because it has no JQL engine. There are two exceptions. `key in
(...)` returns just the issues it names, including one linked issue that a
plain query never lists. `filter = 99999` is refused, so you can see how PIE
shows Jira's message. Dates are counted back from today, so ages look the
same whenever you run it.

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

## Follow linked issues across projects

A run also follows the links your results have to issues in other projects,
and the links those have in turn, until no new issue turns up. PIE fetches
those issues even when your query did not match them. Links between issues in
the same project are not followed.

Issues joined this way form a linked set. **Results** lists only the issues
your query matched. The two other pages show the sets:

- **Trace** lists the linked sets on the left. Pick one to see its issues
  project by project, with status, age and how each issue links to the others
  (for example "clones ALPHA-101"). An issue your query did not match is
  marked "Not in the query".
- **Project map** has a row for each set and a column for each project. A
  cell gives the key and status of each issue the set has in that project, or
  a dash when it has none. Choose a key to open its set in Trace.

A set is **out of step** when at least one of its issues is finished in Jira
(its status is in Jira's done category) and at least one is not. That is the
case PIE is for: a fix made in one project but not in another. A Rejected
issue counts as finished, so a Rejected clone beside an open one is out of
step. Both pages can show only the sets that are out of step.

Following links stops when a run reaches 5,000 issues in all, and PIE says so.
If Jira refuses one of the searches for linked issues, PIE shows Jira's
message and keeps your results, but the sets may be incomplete.

Results are not saved. They are gone when you close PIE or run another query, and
PIE needs a connection to Jira to run one.
