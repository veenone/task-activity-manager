# Project Issue Explorer: User Guide

**Version:** 0.1.0 · **Platform:** Windows 10/11 (64-bit) and macOS ·
**Audience:** analysts who follow an issue across many Jira Data Center
projects.

Project Issue Explorer (PIE) is a desktop app for people who look across Jira
projects rather than inside one. You give it a JQL filter and it lists what
matches, whichever projects those issues are in. PIE only reads from Jira. It
does not change anything there. The one place it writes is Confluence, when
you publish a cycle.

## The menu bar and the theme

The menu bar has three menus:
- **File** has Manage profiles, Jira fields, Run query (Ctrl+R) and Quit (Ctrl+Q).
- **View** switches tabs, from Ctrl+1 for Results to Ctrl+7 for Cycles. Its
  **Theme** submenu picks Light, Dark, or System to follow your operating
  system.
- **Help** has About. It shows PIE's version, the database it uses, the
  runtime and the number of profiles, in the same box TAM and XTM use. Paste
  it into a bug report.

The moon or sun button at the right of the top bar switches between light and
dark. PIE remembers the theme you pick, from either place, for the next time
it starts.

## The status bar

The bar along the bottom of the window shows the profile in use, how many
issues the last run returned, when a saved query last ran, and the theme. While
a query runs, a progress bar takes the place of the count. It says what PIE is
doing, **Fetching issues** and then **Following links**, and how many of how
many it has done so far.

## Add a profile

A profile is how PIE reaches Jira. Choose **Manage profiles** in the top bar,
then **Create new profile**, and enter a name, your Jira URL and a personal
access token. PIE keeps the token in your operating system's credential
manager, not in its database. The same form has a Confluence part, used when
you publish a cycle (see Publishing to Confluence below).

PIE's profiles are separate from the ones Task Activity Manager and Xray Test
Manager share, because they have no project key. A profile set up in those apps
does not appear here, so you enter the URL and token again.

You can keep several profiles, for example one for each Jira account, and switch
between them from the list at the top of the window. Manage profiles is the
same dialog TAM and XTM have:
- Pick a profile on the left to edit it. Its saved queries, cases and cycles
  stay. Leave a token field empty to keep the token already saved.
- The star marks the profile PIE opens when it starts. Click it again to clear.
- **Export** writes the profile to a file without its tokens. **Import from
  file** reads one back; open the imported profile and enter a token before
  running a query.
- **Delete** removes the profile, its tokens, saved queries, cases and cycles
  from this computer.

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

Every project filter in PIE is a box you can type into: Trace, Project map,
Statistics, Results, and the Network centre on the Link map. As you type, the
suggestions narrow to the projects whose key or name contains it, and picking
one applies it. The suggestion list opens when you click into the box. Use the
arrow keys and Enter to pick from it, or Escape to close it. Clear the box to
see every project again. Text that names no
project applies no filter, and the box says "No project matches".

Paste JQL into the box and choose **Run**. The table shows each issue's key,
project, type, summary, status, priority and last update.

The JQL box folds away. When you open a saved query it folds, and the query
shows on one line beside **JQL**. Click that line to edit it. **Run** works
either way. With no JQL yet, the box stays open.

The **Project**, **Status** and **Priority** filters above the table narrow
the rows without contacting Jira. Each lists only the values in the run, and
they combine. **Search** keeps the rows whose key, summary or project name
contains what you type. While any filter hides rows, PIE says how many it shows
of how many. A filter set to a value the next run no longer holds goes back to
All.

Click a column header to sort the rows by it, across every page. Click it again
to reverse the order. Empty values stay at the bottom either way. Export to
Excel writes the rows in the order on screen. The tables in Statistics sort the
same way.

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
in the status bar says how many of the total it fetched, so narrow the query to
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

- **Trace** lists the linked sets on the left. **Key** above the list keeps
  the sets with an issue key that contains what you type. Pick one to see its issues
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
step. A set is **in step** when its issues agree: all finished, or all still
open. Both pages can show only the sets that are out of step.

On a large run the Project map can also be narrowed to the sets you care
about:
- **Baseline** keeps the sets that reach a project under that baseline, or
  under none.
- **Project** keeps the sets that reach that project. Its list shows only the
  chosen baseline's projects.
- **Key** keeps the sets with a key that contains what you type.

The filters apply to Flow, Tree and Table together. A line says how many sets
are showing out of the whole run, and **Clear filters** brings them all back.
The counts above the map always describe the whole run.

Above the map, PIE counts the sets out of step, the issues in them that are
still open, the issues in linked sets and the baselines. **Flow**, **Tree** and **Table**
show the same sets three ways, and **Baseline** shows one baseline and what links to it:

Both Flow charts and the Link map's Network can be zoomed. Use
**+** and **−** above the chart, or hold Ctrl and turn the mouse wheel. The
chart then scrolls inside its own area, and **Reset** returns it to fit.

- **Flow** draws each issue as a line from its baseline to its project to
  one of four states. *Fix missing* is an open issue in a set out of step: the
  fix was made elsewhere and still has to be made here. *Fixed, others behind*
  is a finished issue in such a set. *Fixed everywhere* and *Open
  everywhere* are issues in sets that are in step. A band is as wide as the
  number of issues in it; hover over it to see the keys. The sets out of step
  are listed beside the chart.
- **Tree** branches each baseline into its projects and each project into its
  issues, and says how many of a project's issues are finished. Choose a set
  in the list beside it to mark its issues in the tree. Tick **Group by
  status** to fold each project's issues under one node per status, with its
  count. Click a status to open it. Untick it to list every issue again.
- **Baseline** puts one baseline in the middle. Its issues sit inside a
  dashed boundary, each derived project in a smaller group inside it, and the
  issues of other projects linked to them sit outside, grouped by project.
  - Pick the baseline, a **Version** (its Fix Versions) and, to compare, a
    second version under **Compared with**. A run saved before PIE read Fix
    Versions shows none until you run it again.
  - **Inside the baseline**, **Crossing its boundary** or **Both** chooses which
    links are drawn, and the list below hides a link type.
  - Click an issue to trace it. **Blocker chain** lights up every issue that
    blocks it, then what blocks those, and so on; **Impact** lights up what it
    holds up instead. Only "blocks" links are followed: clones, duplicates and
    relates-to say nothing about which issue waits for which. The chain is
    listed on the right; **Clear the trace** ends it.
  - With **Only issues with links out** ticked, issues inside with no link out
    of the baseline fold into "+ N more" per project. Click one to open it.
  - The right-hand panel counts the baseline's health for the version: issues
    inside, the share finished, issues blocked from outside the baseline,
    external dependencies, linked sets out of step and the median age of open
    issues. With a second version chosen, each number shows its change, in red
    when it is worse.
- **Table** is the grid described above.

In every view, choosing a key opens its set in Trace.

Following links stops when a run reaches 5,000 issues in all, and PIE says so.
If Jira refuses one of the searches for linked issues, PIE shows Jira's
message and keeps your results, but the sets may be incomplete.

### Baselines

**Baselines** on the Project map keeps your own mapping of Jira projects for
the selected profile: which projects are product baselines, and which projects
were derived from each. Type a project key (the list offers Jira's projects by
key and name) and choose **Add baseline**, then add descendants under it. A
project sits under one baseline at most, and a baseline cannot be a
descendant; PIE says where a project already sits if you try. **Remove
baseline** asks first and takes its descendants out of the mapping with it.
Nothing in Jira changes.

With at least one baseline, the Project map orders its columns by baseline:
each baseline, then its descendants, under a header naming the baseline.
Projects under no baseline come last, under **No baseline**. The mapping is
separate from the Product base field, which Statistics still reads from each
issue.

### Link map

The **Link map** tab counts the links between projects by type. Each row is
the project a link starts from and each column the project it points to, read
in Jira's outward wording: a row of Reader Firmware with "Cloners 2" under
Card Platform means two Reader Firmware issues clone Card Platform issues. A
type worded the same both ways, such as "relates to", has no direction, so
its link shows under whichever end Jira calls outward.

The list on the left names every link type in the run with its outward and
inward wording and its count. Untick a type to hide it, or use **All** and
**None**. Links between issues of one project are hidden until you tick
**Show links inside one project**. Links to issues PIE did not fetch are
hidden while **Only links where both ends are in the run** is ticked; untick
it to count them under the project in their key.

Choose a cell to list its links below the grid, with each end's status and
the summary of the issue the link starts from. The button beside the title
switches to the other direction and says how many links it has. Choose a key
to open it in Trace. A run saved before PIE kept link direction asks you to
run the query again.

The grid's top-left cell has **Key** and **Name**. They put the rows and
columns in order of project key or project name. Click the same one again to
reverse the order. The export follows it.

To narrow the Link map, type into **Add a project** above the link types and
press Enter, or pick from the suggestions. Once you have chosen some projects,
the grid, Flow and Network keep only the links between them. Remove a project
with the × on its chip. **Key** keeps the links with an end whose issue key
contains what you type.

On a large run the grid scrolls both ways. The column headers and the project
column stay in view. A column shows the project key, and hovering over it
shows the name. A cell gets darker the more links it holds, and the last
column totals each row.

**Grid**, **Flow** and **Network** at the top right switch how the links
are drawn. The link type filters and the two tick boxes on the left apply to
all three views.
- **Flow** runs each link from the project it starts in, through its link type,
  to the project it points to. Bands are sized by link count. Hover over a band
  to see its count. Type into **Left side** to keep only the links starting in
  the projects you choose. The chart leaves room for the longest project name,
  and scrolls sideways when the window is narrower than that.
- **Network** puts one project in the middle, the busiest at first, with the
  projects it links to around it. Each link type has its own spoke, with its
  count, and the arrows show which way the links point. Pick the centre from
  **Centre project**, or choose a project in the diagram or in the list beside
  it to move the centre there. On a busy project the diagram draws the 24 with
  the most links, and the list names them all. Hover over a project to bring its
  name to the front, and drag a project to move it when names crowd. Choosing
  another centre lays the projects out again. The centre circle grows to fit a
  long project key. A legend under the diagram explains it: an arrow away from
  the centre means the centre's issues link out to that project, an arrow
  towards it means that project's issues link in, and a thicker spoke has more
  links.

  **Projects** and **Issues** above the diagram switch between projects and
  the issues themselves. **Issues** draws each issue as a box with its key, a
  short summary and a border coloured by status. Each relationship is one
  line, coloured by link type, with an arrow from the issue that does it to
  the one it is done to ("blocks", "clones", "parent of"). "relates to" is
  dashed and has no arrow. A dashed box is an issue outside the run, and
  **+3** on a box says three of its links are not drawn yet.
  - The graph is laid out in rings around a centre issue, at first the run's
    issue with the most links. Press Enter on a key in **Find an issue**, or
    choose **Centre here** in the panel, to lay it out around that issue
    instead.
  - Click an issue to move to it. The panel on the right shows its fields, its
    links by type and, for an issue fetched from Jira, when it was fetched.
    **Expand links** draws its linked issues, 50 at a time. **Open in Trace**
    and **Open in Jira** open it there.
  - **‹ Back** and **Forward ›** step through the issues you visited, and the
    line beside them shows the path.
  - **Find an issue** looks among the issues drawn by key or summary. Type a
    full key, such as ABC-120, and press Enter to fetch one that is not drawn
    and put it at the centre.
  - The **Relationships** list hides a link type, and **Show** hides issues
    outside the run or finished issues.
  - Drag to pan, hold Ctrl and turn the wheel or use **+** and **−** to zoom,
    and use **Fit** to see everything. **Re-layout** arranges the boxes again.
    Changing a filter lays the graph out again and fits it to the window.
  - The graph holds at most 300 issues and says so when it is full.

  Issues outside the run are fetched from Jira and kept on this computer.
  When Jira cannot be reached, the graph opens and expands from what is kept,
  each issue says when it was fetched, and the status bar says PIE is offline
  until Jira answers again. To have a saved query's linked issues ready before
  you go offline, open it and tick **Keep links for offline**: each run then
  also fetches the issues one link away. It is off by default, because on a
  large run it costs more Jira searches.

**Refresh links** at the top of the list fetches the linked issues again
without running the query. Your query's own rows stay as they were. For a
saved query the refreshed run is stored, so it is what PIE shows next time.
If Jira fails partway, PIE keeps the run it had and says why.

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

## Open cases

A case is an issue you have picked for analysis. PIE never opens one by
itself. Tick the rows you want in Results, or the box in the header for every
row on the page, and choose **Open N as cases**. An issue that already has a
case keeps it, with the date it was first opened.

Rows with a case show **Case**, and the **Case** filter shows all rows, only
those with a case, or only those without. Cases are kept on this computer for
the profile and survive a restart and a new run. When a run doesn't match an
issue that has a case, the case stays, and PIE lists its key above the views.
Deleting the profile deletes its cases.

## Work on a case

The **Cases** tab lists your cases, with or without a run on screen. Each one
shows its summary when the run on screen holds the issue, and how many of its
projects have a verdict and how many are affected. Choose a case to open it.

A case has notes, which you save with **Save notes**, and a list of projects
to assess. **Add projects** searches Jira's project list by key or name.
Tick the projects you want and add them. A project starts at *not assessed*.

For each project, pick a verdict: *not assessed*, *investigating*,
*affected* or *not affected*, and give a reason if you like. PIE stores the
verdict as soon as you pick it, and a reason when you leave its box, with the
time either was set. **Remove** takes a project and its verdict off the case,
after asking.

Projects you often assess together can be saved as a group: tick them and
choose **Save as group...**. Choosing a group in the picker ticks all its
projects. Deleting a group leaves the cases that used it as they are.

Cases, verdicts and groups stay on this computer until you delete the profile.
PIE never sets a verdict by itself and writes nothing to Jira.

### Evidence

Below the projects, **Find evidence** reads the case's issue from Jira and
suggests evidence for other projects:

- its links to issues in other projects, labelled with the link's own words
  (for example "is cloned by"). These are strong evidence.
- other projects with a component or fix version of the same name. A shared
  name may be a coincidence, so these are marked weak.
- issues of the same type that match the phrases you type, one phrase per
  line. Each phrase is searched in quotes. These are marked weak too, and a
  phrase can't contain a double quote.

Accept or dismiss each suggestion. Running **Find evidence** again adds what
is new and keeps what you decided about the rest. You can also add a note or
an issue key by hand for any project on the case, and remove what you added.
Evidence for a project not yet on the case shows a button that adds it.

Evidence helps you decide a verdict but never changes one. Demo has links,
components, a fix version and summaries for every kind to find: try
ALPHA-101 with the phrase "retry counter".

### Cases by project

**Grid** on the Cases tab puts every case against every project on any case.
Each cell gives that case's verdict for the project, or a dash when the
project is not on the case. The last row counts how many cases have affected
each project. Choose a case's key to open it in the list. **List** goes back.

## Export to Excel

Results, Trace, the Project map, the Link map, Statistics, Cases and a cycle
on the Cycles tab each have **Export to Excel**. It asks where to save, then writes
an .xlsx of what the view is showing, with its filters applied:
- **Results** writes every row its filters leave, across all pages, in the
  table's columns.
- **Trace** writes the linked sets it lists, one issue per row.
- **Project map** writes its table, and the same sets one issue per row.
- **Link map** writes its grid and every link the grid counts.
- **Statistics** writes the headline numbers and both tables, for the project
  and time range you chose.
- **Cases** writes every case with each project's verdict and reason.
- **A cycle** writes what changed since the last published cycle, and its
  cases.

Each sheet has a bold header row that stays in view and a filter on every
column. Counts are stored as numbers, so Excel can sum and sort them. If you
cancel the Save dialog, nothing is written.

When the file is written, the status bar says where it went. If writing fails,
the status bar turns red and says why. Choose **Dismiss** to clear either.

## Cycles

A cycle is one reporting period, such as a week or a release. **New cycle**
on the Cycles tab asks for a name and keeps a copy of every case with its
projects, verdicts and reasons. Verdicts you change afterwards do not change
the cycle, so it still says what was true when you took it.

Choose a cycle in the list to see what changed since the last cycle published
before it: the cases that are new, the cases that are gone, and each project
whose verdict moved, with what it was and what it is now. A project taken off
a case shows as removed. A cycle with no
published cycle before it counts every case as new. Below the changes is the
cycle's own grid of cases against projects.

Until a cycle is published, **Take again** replaces its copy with the cases as
they are now, and **Delete** removes it.

### Publishing to Confluence

The Confluence part of the profile's form in **Manage profiles** takes the Confluence URL, the space
key and the id of the page the reports go under. Leave the page id empty to put
them at the top of the space. The token goes to the operating system's
credential store, never to PIE's database. Leave the token field empty to keep
the token you saved before. A new URL needs its own token, and PIE
forgets the pages it wrote on the old one.

**Publish** writes the cycle to a page called "Impact report:" followed by the
cycle's name. The page has the change counts, the cases by project grid, and
each case's verdicts with their reasons. After that the cycle is published:
**Take again** and **Delete** go away, and the next cycle is compared with
this one.

PIE asks before it writes in two cases. The first is a page with that title
that PIE did not write. The second is a page someone edited in Confluence
after PIE published it. Say no and nothing changes. **Publish again** updates
the same page. If the page was deleted in Confluence, PIE writes a new one.

## Issue details

Choose an issue's key in Results to open its details beside the table: its
summary, project, status, priority, client visibility and the full client
impact text. **Close details** or Escape closes them.

Only a saved query's last run is kept. Other results are gone when you close
PIE or run another query, and PIE needs a connection to Jira to run one.
