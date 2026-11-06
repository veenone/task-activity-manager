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

The **Project**, **Status** and **Priority** filters above the table narrow
the rows without contacting Jira. Each lists only the values in the run, and
they combine. While any filter hides rows, PIE says how many it shows of how
many. A filter set to a value the next run no longer holds goes back to All.

Wherever PIE names a project, it gives the project's name with its key in
brackets, for example "Card Platform (ALPHA)". A saved query's run from before
PIE read project names shows the key alone until the query runs again.

Long lists come a page at a time: Results, the linked sets and project groups
in Trace, the Project map and the per-project table in Statistics. Below each
list, choose how many rows a page holds, move to the first, previous, next or
last page, or type a page number. When a filter or a new run leaves fewer
pages, PIE shows the last page that is left.

If Jira refuses the query, PIE shows Jira's own message and leaves the previous
results on screen. A query that names a saved filter with `filter = 12345` fails
as a whole if your account cannot see that filter, even when the filter is only
one part of the query.

When a query matches more than 2,000 issues, PIE asks before fetching them.
If you decline, it fetches nothing and the previous rows stay on screen. PIE
fetches at most 5,000 issues from one run. When a query matches more, the count
above the table says how many of the total it fetched, so narrow the query to
see the rest.

## Start from a Jira filter

**From a Jira filter** lists the filters you have starred in Jira, with their
JQL. Choose one to put `filter = <id>` into the JQL box. A run then follows
the filter as its owner changes it. Nothing runs until you choose **Run**, and
the JQL counts as unsaved until you save it.

Jira lists only starred filters. To use a filter you have not starred, type
the number from its Jira address into **Filter id** and choose **Open**. If
Jira has no filter with that id that your account can see, PIE says so.
Demo has two starred filters and one more with the id 4300.

## Save a query

**Save as...** keeps the JQL in the box under a name, for the selected profile.
Pick it from **Saved query** later to get its JQL back, along with the rows,
Trace and Project map from its last run and the time of that run. Opening a
saved query does not contact Jira, so it works offline.

Running a saved query replaces its stored run. If you edited the JQL first, the
saved query keeps the edited JQL. Below the form, PIE says how many issues the
run added and dropped compared with the previous run; open the line to see
their keys. After an edit there is no comparison, because the two runs asked
Jira different questions. Only issues the query matched count, not the ones reached through
links.

JQL run while **Unsaved query** is selected is not stored. **Delete query**
removes a saved query and its last run, and deleting a profile removes all of
its saved queries.

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
  a dash when it has none. Choose a key to open its set in Trace, where the set list opens on the page
  that holds it.

A set is **out of step** when at least one of its issues is finished in Jira
(its status is in Jira's done category) and at least one is not. That is the
case PIE is for: a fix made in one project but not in another. A Rejected
issue counts as finished, so a Rejected clone beside an open one is out of
step. Both pages can show only the sets that are out of step.

Following links stops when a run reaches 5,000 issues in all, and PIE says so.
If Jira refuses one of the searches for linked issues, PIE shows Jira's
message and keeps your results, but the sets may be incomplete.

## Statistics

The **Statistics** page counts the run on screen without contacting Jira. Only
issues your query matched are counted. Issues reached through links count only
towards how far an issue reaches.

The numbers at the top are:

- how many issues are not finished, and their median age in days
- how many issues reach more than one project, meaning their linked set spans
  more than one project
- how many reach more than one product base (see below)
- how many linked sets are out of step

Three charts show issues created per month, the ages of unfinished issues, and
how many projects each issue reaches. A table below them gives each project's
issues, unfinished issues and median age. Another table lists each finished
status with its issue count and its median time from creation to resolution.
Closed and Rejected are listed apart, because a rejection is not a fix. Jira
gives Corrected issues no resolution date, so Corrected is counted but has no
time. An issue with no resolution date never enters a median, and a median
with nothing to measure shows as a dash.

The **Project** and **Created** filters apply to every number on the page.

## Jira fields

Jira has no product base or client fields of its own, so you tell PIE which
custom fields hold them. Choose **Jira fields** and enter each field's name as
Jira shows it:

- **Product base**, counted on the Statistics page. A field with several
  options gives an issue several product bases.
- **Client visibility**, an option such as how likely a customer is to see
  the problem. Results gets a column and a filter for it.
- **Client impact**, free text about what the customer saw. It shows in the
  issue's details.

PIE checks every name against Jira's field list when you save. If one is
missing in Jira, or two custom fields share it, PIE says which and saves none
of them. Leave a name empty if the profile doesn't use that field. Each
profile keeps its own names, and deleting the profile deletes them.

The fields are read when a query runs, so run the query again after a change.
Until a product base field is set, Statistics leaves product bases out. Demo
has invented fields named **Product base**, **Client visibility** and
**Client impact**.

## Issue details

Choose an issue's key in Results to open its details beside the table: its
summary, project, status, priority, client visibility and the full client
impact text. **Close details** or Escape closes them.

Only a saved query's last run is kept. Other results are gone when you close
PIE or run another query, and PIE needs a connection to Jira to run one.
