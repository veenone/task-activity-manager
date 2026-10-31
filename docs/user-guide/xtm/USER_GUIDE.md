# Xray Test Manager — User Guide

**Version:** 1.10.0 · **Platform:** Windows 10/11 (64-bit) · **Audience:** QA engineers,
test leads, and anyone managing Xray test cases in Jira Data Center.

Xray Test Manager is a lightweight Windows desktop app for managing **Xray test
cases in Jira Data Center** at scale (10k+ test cases). It keeps a fast local
copy of your tests so browsing, searching, filtering, and bulk editing are
instant — then pushes your edits back to Jira when you choose to commit. **Jira
is always the system of record;** the app never changes anything in Jira until
you commit.

> **About the screenshots in this guide.** Figures are referenced from the
> `images/` folder next to this file. If a figure is blank, the screenshot has
> not been captured yet — see the [Screenshot capture guide](#screenshot-capture-guide) appendix for the exact
> view and state to capture for each one. The fastest way to produce every
> screenshot is to run the app against a **demo profile** (no Jira required —
> see [Demo mode](#demo-mode-try-it-without-jira)).

---

## What's new in 1.10.0

Version 1.10.0 adds bulk summary renaming and a guided tour, rebuilds the
precondition sync so a first sync on a large project keeps what it fetched, and
gives the Preconditions view a large overhaul: its definition text, its own
Sync button, and a list that opens instantly on a project with thousands of
them. Kiwi TCMS users can now file bugs into a Jira project.

- **Rename many summaries at once.** Select tests in Browse, then **Rename
  summaries…**, and add a common **prefix, suffix, or both**. A live preview
  shows each summary before and after with the inserted text highlighted, so a
  trailing space or a missing bracket is visible rather than something to spot
  by eye. Tests that already carry the affix are left alone, so the same rename
  is safe to run twice, and a result that would exceed Jira's 255-character
  limit is flagged and left out while the rest still applies.
- **A guided tour.** A seven-step walkthrough of the core loop: profile, sync,
  views, search, the grid, and the local-edit-then-commit model. It starts once
  after your first successful sync and can be replayed any time from **More ->
  Take the tour** or from **About**.
- **Select all when adding tests.** The add-tests picker gains a select-all for
  the page and, when the page is full, a banner that selects **every test
  matching the current folder and search**. Tests already in the container stay
  excluded. The Preconditions view uses the same picker and gets this too.
- **Preconditions are no longer lost on a first sync.** The stage used to hold
  everything in memory until it finished, so an interruption saved nothing, and
  its failure was swallowed while the sync reported success. It now saves as it
  goes, runs far faster, and a stage that fails is reported as a **partial**
  sync in sync history instead of a silent success.
- **Preconditions load on demand.** Opening a test whose preconditions were
  never stored now fetches them from Jira, with a **Refresh** control on the
  Preconditions section.
- **Imports warn about unknown components.** A CSV or XLSX import naming a
  component the project does not have used to report success and then fail
  every test at commit. Unknown names are now listed before anything is queued,
  with a suggestion when the only difference is capitalisation, and an option
  to import without them.
- **A keyboard-operable, virtualized test grid**, an accessible modal layer that
  traps and restores focus, and screen-reader announcements for results that
  were previously silent.
- **A faster interface.** Editing something no longer reloads a whole view;
  only what actually changed refreshes.
- **A precondition's Condition is shown and editable.** The definition text on
  a Precondition was never read, so every one of them arrived blank and the
  detail panel said "No condition defined" whatever Xray held. It now syncs,
  and editing it pushes back on commit.
- **The Preconditions view has its own Sync.** Refreshing preconditions no
  longer costs a full sync of tests, folders and containers. The status bar
  tracks both halves of the run, **Finding preconditions** and **Linking
  preconditions to tests**, each with its own count.
- **A precondition key opens its Jira issue.** The key in the Preconditions
  detail header is a link, the way the test key already was.
- **The Preconditions list opens instantly on a large project.** On a project
  with several thousand preconditions the list used to sit on "Loading..." for
  about a minute. It is now near-instant.
- **Switching profiles clears what the last one had selected.** A view used to
  keep the previous profile's selection, so the Preconditions detail panel went
  on showing a precondition from the profile you had just left.
- **Browse shows the folder tree even with no folders.** A project whose tests
  were never filed into a folder showed "No folders synced." and no way to
  reach them. The tree is now always drawn, with an **All tests** row counting
  every test in the profile.
- **Kiwi TCMS bugs can be filed into Jira.** A Kiwi profile can be given a Jira
  bug tracker of its own: URL, project, issue type, token and TLS settings,
  configured on the profile. Filing a bug creates the Jira issue and links it
  back on the Kiwi Test Execution, and sync reads those links so the Bugs view
  shows what relates to this product.
- **The About dialog fits its window.** It used to stretch to the full height
  of the screen, and in a short window its footer sat below the fold.

---

## What's new in 1.9.0

Version 1.9.0 is a major feature release. Highlights:

- **Backend-agnostic core + Kiwi TCMS.** The local store is now a neutral hub
  that can pull from and publish to more than one backend. **Kiwi TCMS** is the
  first non-Xray target, with a **migration bridge** to move a dataset between
  connections. Each backend advertises its capabilities and the UI hides what a
  backend cannot do — Xray keeps every feature unchanged.
- **Publish coverage groups to Xray.** From the Coverage module you can publish a
  coverage group to Xray as a **Test Set**, with **drift detection** when the
  published set diverges from the local group.
- **Cross-project linking.** The precondition, call-test, and clone-steps pickers
  can reach tests and preconditions in other projects you configure as
  cross-project sources.
- **Test Calls: cross-project vs missing.** An unresolved call is now split into
  **cross-project** (the target lives in another project — expected) and
  **missing** (deleted or not synced), each clearly labelled. See
  [Calling another test](#calling-another-test-test-calls).
- **Misspellings view.** Scan every test case for spelling errors and jump to the
  offending test.
- **Duplicate preconditions.** The Duplicates feature now detects preconditions
  with matching content, not just tests.
- **Cucumber and Generic test types**, and the ability to **link an existing bug**
  and add **remarks** on a Test Execution run.
- **Faster sync** (container sync is concurrent and rate-limited) with a **bug-sync
  progress bar**, plus container, import, and Manage Profiles refinements.

---

## What's new in 1.8.0

Version 1.8.0 is a major feature release. Highlights:

- **Requirements suite.** Create, import, and link requirements, edit their full
  detail (priority, components, fix versions, sprint, and a markdown
  description), and choose which Jira link type connects Tests to Requirements.
  See [Requirements & coverage](#requirements-coverage).
- **Coverage module (opt-in).** A new **Coverage** tab measures parameter-level
  test coverage and lets you define a functional requirement once and reuse it
  across customer projects, with per-version coverage, change requests, and a
  visual Coverage Map. Enable it from **More → Show Coverage tab**. See
  [Coverage module](#coverage-module).
- **Preconditions** gain a dedicated **Condition** field and a read-only detail
  with explicit Edit, Save, and Cancel.
- **Dashboard** adds a **By requirement** breakdown beside By component.
- **Traceability** adds an **Epic** layer to the requirement flow.
- **Gap Analysis** and the profile switcher got UI polish, and profile switching
  is now locked while a sync runs.

---

## What's new in 1.7.0a (alpha)

- **Test run history.** Each test now has a **Run history** section: every
  execution it ran in, with the result, date, who ran it, environment, Test
  Plan(s), Fix Version(s), and linked defects. Test Executions gain run date /
  tester / environment columns, and Test Plans / Sets show a run roll-up. See
  [Run history](#run-history) and [Containers](#test-sets-plans-executions-containers).
- **View sessions.** Switching tabs no longer resets a view: your selection,
  filters, search, pagination, and sub-tab are restored when you return to it
  (until you restart the app or change profile).
- **Richer bug detail.** A bug's affected-tests table gains a **Project** column
  and an expandable per-test breakdown (fix version / execution / plan /
  environment / date / tester / defects). From there you can open a test's
  **read-only detail** in a side panel without leaving the Bugs view, or add the
  bug's tests to an **existing** Test Execution. See
  [Defect tracking](#defect-tracking).
- **Read-only detail side panels.** The read-only test detail also opens beside
  the Test Execution member list in Containers, and the same side-panel layout is
  used from the **Requirements** view's covering-tests table (it no longer takes
  over the whole screen). See [Requirements & coverage](#requirements-coverage).
- **Import JUnit XML results.** Update a Test Execution's run results by importing
  a JUnit-compatible report, matching each `<testcase name>` to a test by summary.
  Import into the **selected execution**, or **create a new execution** from the
  report (optionally creating any tests it does not find). See
  [Containers](#test-sets-plans-executions-containers).
- **Per-test Fix Version on executions.** Each member of a Test Execution shows
  its **own** Jira Fix Version(s) in a column, and the execution's Fix Version
  chips are clickable to filter the member list. See
  [Containers](#test-sets-plans-executions-containers).
- **Run history nested by Test Plan, sortable.** A bug's per-test run breakdown
  groups runs under each **Test Plan** (a plan can span several executions and fix
  versions), adds **Created / Updated** timestamps, and the run rows sort by them.
  See [Defect tracking](#defect-tracking).
- **Traceability detail.** The Requirement flow gains a **Test** column
  (requirement → coverage → Test plan → **test** → result), and the Sub-task flow
  labels each parent with its **summary**. See [Traceability](#traceability).
- **Bugs panel polish.** A scrollable bug list with a **select-all** checkbox,
  working rows-per-page (default 5), and full-colour run-status highlighting.
- **macOS fixes.** Copy/paste and right-click now work in text inputs (a standard
  Edit menu is wired in); a profile can supply a **custom CA certificate** or
  **allow an untrusted certificate** to connect through an internal CA; and the
  Personal Access Token field has a **show/hide** toggle. You can also **test the
  connection while editing** a saved profile (it uses the stored token). See
  [Connecting to Jira](#connecting-to-jira-profiles).

> This is an **alpha** build (`1.7.0a`) for validation. The live Xray run data
> and the macOS TLS / menu fixes still need verifying against a real instance.

## What's new in 1.6.0

- **Test Case Gap Analysis.** A new **Gap Analysis** tab compares a project (or
  an imported CSV/XLSX list) against a target list and reports what is missing on
  each side, with an optional folder-mismatch check, create-tests-from-gaps, and
  a formatted Excel report. See [Gap analysis](#gap-analysis).
- **Cross-project Test Executions.** Member tests that live in a *different* Jira
  project now appear on the execution board and in the traceability flow (with an
  **include cross-project members** toggle), and their linked bugs are harvested.
  See [Containers](#test-sets-plans-executions-containers) and
  [Traceability](#traceability).
- **Execution Type.** The Xray **Test Type** (Manual / Automated / Generic /
  Cucumber) is now a Browse column and filter, and is editable per test and in
  bulk. See [Browsing tests](#browsing-tests).
- **Test Environments & Fix Version(s) on executions.** A Test Execution shows
  its **Test Environments** (editable, single and bulk) and its read-only Jira
  **Fix Version(s)**. See [Containers](#test-sets-plans-executions-containers).
- **Swap and multi-pick links.** Add several requirements or preconditions to a
  test at once, or **Replace** (swap) the whole set in one step — per test and in
  bulk. See [Bulk operations](#bulk-operations).
- **Add tests to a requirement.** Link covering tests straight from the
  Requirements view. See [Requirements & coverage](#requirements-coverage).
- **Export to XLSX everywhere.** Export the **Dashboard** and the active
  **Traceability** tab to Excel, on top of the existing CSV/XLSX exports, and
  filter the Dashboard by folder / component / status.
- **Duplicates: compare summaries.** Compare duplicate members' summaries side by
  side (in addition to steps), with a scan progress bar. See
  [Finding duplicates](#finding-duplicates).
- **Live Jira/Xray wiring.** Every operation now runs against a live Jira Data
  Center / Xray Server instance, not just demo data (see the note below).

> **Live wiring (Phase 7).** The real Jira/Xray REST calls are now implemented
> for every operation. A few instance-specific shapes are still being verified
> against live servers; demo mode remains fully functional throughout.

## What's new in 1.5.0

- **Defect tracking.** Raise a bug straight from a failed test in a Test
  Execution, browse every bug linked to your tests in a new **Bugs** panel, and
  see a test's linked bugs — including ones filed in other projects — on its
  detail panel. See [Defect tracking](#defect-tracking).
- **Sub-task Test Executions.** Xray sub-task Test Executions (an execution that
  is a sub-task of a parent issue) now sync and sit alongside standalone ones —
  filter by Standalone or Sub-task and open the parent issue from the container
  card. See [Containers](#test-sets-plans-executions-containers).
- **Traceability view.** The traceability Sankeys moved out of the Dashboard
  into their own **Traceability** tab with three diagrams — Requirement,
  Execution, and the new **Sub-task** (parent → execution → result). See
  [Traceability](#traceability).
- **Searchable, filterable lists.** Type-to-filter dropdowns and sort/filter
  controls on the Requirements, Preconditions and Bugs panels and the container
  picker (filter executions by Standalone / Sub-task too).
- **Jira colour in text.** `{color:#hex}…{color}` macros in descriptions and
  steps now render in colour; editing still shows the raw macro so it
  round-trips back to Jira unchanged.
- **Editing niceties.** Duplicate a step, clone a whole test, native spell-check
  on test prose, and the test key in the detail header links to its Jira issue.
- **Smarter sync.** Per-view syncs (Containers, Requirements, Bugs) now show
  progress in the status bar; bug sync respects the profile scope.

Earlier releases added commit **conflict management** (three-way merge),
**macOS** support, and the **Test Calls** view (1.3.0–1.4.0). See
[`CHANGELOG.md`](../../../xtm/CHANGELOG.md) for the full history.

---

## Contents

1. [Installation & first launch](#installation-first-launch)
2. [Connecting to Jira (profiles)](#connecting-to-jira-profiles)
   - [Working with a Zephyr Essential profile](#working-with-a-zephyr-essential-profile)
   - [Demo mode, without a Jira connection](#demo-mode-try-it-without-jira)
3. [The main window](#the-main-window)
4. [Browsing tests](#browsing-tests)
5. [Viewing & editing a test](#viewing-editing-a-test)
6. [Creating & importing tests](#creating-importing-tests)
7. [Bulk operations](#bulk-operations)
8. [Preconditions](#preconditions)
9. [Requirements & coverage](#requirements-coverage)
10. [Finding duplicates](#finding-duplicates)
11. [Test Sets, Plans & Executions (Containers)](#test-sets-plans-executions-containers)
12. [Datasets](#datasets)
13. [Defect tracking](#defect-tracking)
14. [Dashboard](#dashboard)
15. [Traceability](#traceability)
16. [Gap analysis](#gap-analysis)
17. [Coverage module](#coverage-module)
18. [Spellcheck](#spellcheck)
19. [Committing changes to Jira](#committing-changes-to-jira)
20. [Syncing](#syncing)
21. [Settings & profile management](#settings-profile-management)
22. [Diagnostics & troubleshooting](#diagnostics-troubleshooting)
23. [Glossary](#glossary)

---

## 1. Installation & first launch

Xray Test Manager ships as a single Windows executable. There are two artifacts:

| Artifact | Use it when |
| --- | --- |
| `xray-test-manager-<version>-windows-amd64.exe` | **Portable.** Run it directly — no install. |
| `xray-test-manager-<version>-windows-amd64-installer.exe` | **Installer.** Adds a Start-menu entry and uninstaller. |

**Prerequisite:** Microsoft **WebView2** runtime. It is already present on
up-to-date Windows 10/11. If the window is blank on first launch, install the
"Evergreen" WebView2 runtime from Microsoft and relaunch.

On first launch the app opens the connection screen, because no profile exists
yet.

![Figure 1: First-launch connection screen](images/01-first-launch.png)
*Figure 1 — First launch: the app asks you to connect a Jira project.*

---

## 2. Connecting to Jira (profiles)

A **profile** stores how to reach one Jira project: a name, the Jira base URL,
the project key, and a **Personal Access Token (PAT)**. You can keep several
profiles and switch between them from the top bar.

To create one, fill in the connection form:

- **Name** — any label, e.g. "Payments — Prod".
- **Jira URL** — your Jira Data Center base URL, e.g. `https://jira.example.com`.
- **Project key** — the Xray test project, e.g. `QA`.
- **Personal Access Token** — created in Jira under *Profile → Personal Access
  Tokens*. Requires Jira DC 8.14+.

![Figure 2: New-profile form](images/02-profile-form.png)
*Figure 2 — Creating a profile. The token is stored in the Windows Credential
Manager, never in the local database or logs.*

> **Security:** your PAT is saved to the **Windows Credential Manager** only. It
> is never written to the local database, exported profile files, or logs.

The PAT field has a **show/hide** toggle so you can reveal what you typed while
pasting or checking it.

![Figure 48: PAT show/hide toggle](images/48-pat-toggle.png)
*Figure 48 — The Personal Access Token field with its show/hide toggle.*

**Advanced: TLS / certificate settings.** If your Jira uses an internal or
self-signed certificate and the connection test fails with a certificate error
(`x509: certificate signed by unknown authority`), expand **Advanced: TLS /
certificate settings** and either paste a **CA certificate (PEM)** to trust, or
tick **Allow untrusted certificate (skip TLS verification)** (insecure; only for
a trusted internal server).

![Figure 49: TLS / certificate settings](images/49-tls-settings.png)
*Figure 49 — Advanced TLS settings: a custom CA certificate, or the
allow-untrusted escape hatch.*

### Reporting Kiwi defects into Jira

Kiwi TCMS has no issue type of its own, so a defect raised against a Kiwi test
has nowhere to live. Point the profile at a Jira project instead.

On a Kiwi profile (**Backend: Kiwi TCMS**), open **Edit profile** and fill in
the **Bug tracker** section: the Jira URL, the bug project key, the issue type
(usually `Bug`), and a personal access token for that Jira. This connection is
separate from the Kiwi connection itself, with its own **Advanced: TLS /
certificate settings (bug tracker)** for a CA certificate or an
allow-untrusted-certificate option. The token is stored in the Windows
Credential Manager, never in the local database. Leave the URL and project key
blank to keep bug reporting off.

Once both fields are filled in, **Create Bug** appears on a failed test the
same way it does for an Xray profile. Committing creates the issue in Jira and
adds a hyperlink to it on the Kiwi Test Execution the bug was raised from. The
next sync reads those hyperlinks and fetches only the issues they name,
filling in each one's summary, status, and priority, so the Bugs view shows
what relates to this product instead of every defect in the Jira project.

If Jira creates the issue but the link back to Kiwi fails, the issue is kept
and the commit reports which link is missing. Commit again to add it; the
existing issue is reused instead of a second one being filed. If Jira creates
the issue and the link succeeds but Jira later deletes it (or the linked key
turns out to be malformed), the next sync drops it from the Bugs view rather
than showing a blank row.

If the bug tracker is set up but its credential cannot be loaded, sync
reports the failure on the bug stage instead of emptying the Bugs view; what
was cached from the last successful sync stays in place.

To turn bug reporting off again, clear both the bug tracker URL and the
project key and save. This asks for confirmation, since it deletes the stored
connection and its credential.

### Working with a Zephyr Essential profile

Zephyr Essential is SmartBear's lighter Jira app, the one that used to be
called Zephyr Squad. Choose **Backend: Zephyr Essential** and give the profile
the same Jira base URL and personal access token an Xray profile takes: a
Zephyr test is an ordinary Jira issue, so there is no second connection to set
up. Only Data Center and Server are supported; Zephyr Essential Cloud
authenticates differently and is not covered.

Connecting checks two things, your token and whether the Zephyr app is
installed, so a profile pointed at a plain Jira is refused at that point rather
than syncing into an empty project. It also reads the instance's own result
vocabulary, because Zephyr lets each instance define its statuses. A run on
your instance may read UNEXECUTED or PASS WARNING where an Xray profile reads
TODO or PASS.

What a Zephyr profile shows: the project's tests with their fields, labels,
components, steps and workflow, the requirements they cover, the defects raised
against them, and the test cycles each test has been executed in with its
result in each one. Editing a test's own fields and committing them works as it
does for Xray, since those are Jira writes.

What Zephyr Essential does not have, and what therefore stays empty or hidden:

- **No test repository.** Essential has no folder tree for test cases, so
  Browse's folder sidebar says so and the grouping falls back accordingly.
  Zephyr does have folders, but they group executions inside a cycle rather
  than organising test cases.
- **No preconditions.** The Preconditions tab is not offered.
- **No datasets or parameterized runs.** The Datasets tab is not offered, and
  a test has no iterations.
- **No Test Sets and no Test Plans.** A cycle is the only container kind, so
  the Containers view offers Test Execution alone and Browse drops those two
  groupings.

**Custom fields come from two places.** Jira's own custom fields sit on the
test issue and you can edit them. Zephyr keeps its own custom fields on an
execution and on a test step, and those are a different set: the detail panel
lists them under **On an execution** and **On a step** and leaves them
read-only, because an edit here is committed as a change to the Jira issue and
there is nothing on the issue for them to change. With a single set, which is
what an Xray profile has, the panel shows no group labels at all.

Two limits to know about this release. Setting a result, creating a cycle or
editing a step on a Zephyr profile is not available yet, so results are shown
rather than changed. And the searches look for a Jira issue type named `Test`:
if your instance renamed the type Zephyr uses, connecting reports it by name
instead of leaving you with a project that looks empty.

### Demo mode — try it without Jira

To explore the app with realistic data and **no Jira connection**, create a
profile with the **Jira URL set to `demo`** (any project key and any token will
do). The app then serves ~5,000 generated tests plus sample Test Sets, Plans,
Executions, folders, preconditions, and requirements entirely offline. A yellow
**`DEMO`** chip appears in the top bar. This is the recommended way to capture
the screenshots for this guide.

---

## 3. The main window

After connecting, the main window appears. It has four regions:

1. **Top bar** — profile selector and menu (left), view tabs (center), and
   pending/New Test/More/Sync actions (right).
2. **View tabs** — switch between Browse, Preconditions, Requirements,
   Duplicates, Gap Analysis, Test Calls, Dashboard, Traceability, and Containers
   (plus an opt-in **Coverage** tab, enabled from **More → Show Coverage tab**).
3. **Work area** — the content of the selected view.
4. **Status bar** — whether the local store came up, sync progress, the test
   count and last-sync time, and the theme in force at the right end. A failed
   sync takes the whole bar as a red ribbon.

![Figure 3: Annotated main window](images/03-main-window.png)
*Figure 3 — The main window (Browse view) with the four regions labeled.*

The top bar holds the most-used controls:

![Figure 4: Top bar close-up](images/04-topbar.png)
*Figure 4 — Top bar: profile selector, **Profile** menu, view tabs, the
**pending changes** badge, **New Test**, the **More** menu, and **Sync**.*

> On a narrow window the view tabs move onto their own centered row so they never
> overlap the profile controls.

---

## 4. Browsing tests

The **Browse** tab is the primary workspace: a grouping sidebar on the left, the
test grid in the middle, and a detail panel that opens on the right when you
select a test.

### Grouping sidebar

Use **Group by** to organize the left sidebar by **Folder** (the Test Repository
tree), **Test Set**, **Test Plan**, or **Component**. Selecting an entry filters
the grid to that group. Drag the divider on the sidebar's right edge to resize
it; the width is remembered.

![Figure 5: Grouping sidebar](images/05-browse-sidebar.png)
*Figure 5 — The **Group by** selector and (here) the Test Repository folder
tree. Hover a folder for New-subfolder, rename, delete, and New-test actions.*

### The test grid

The grid lists tests with sortable columns. Above it sits the toolbar:

- **Search** — matches key, summary, and description.
- **Status filter** — filter by workflow status.
- **Execution Type filter** — narrow to a Xray **Test Type** (Manual / Automated
  / Generic / Cucumber). Execution Type is also available as a sortable,
  toggleable grid column (add it from **Columns**) and is editable per test and
  in bulk.
- **Saved views** — apply a saved filter; **Save view** stores the current
  filter + status combination for reuse.
- **Export** — write the *currently filtered* tests to CSV or XLSX.
- **Columns** — show, hide, and reorder columns (the layout is remembered).

Pagination controls sit at the bottom: rows-per-page, first/prev/next/last, and
a jump-to-page box. A dirty-dot marks rows with uncommitted edits.

![Figure 6: Test grid and toolbar](images/06-test-grid.png)
*Figure 6 — The test grid with the search box, status filter, Saved views,
Export, and Columns controls.*

To act on many tests at once, tick their checkboxes — or use **Select all
matching this filter** to select every test the current filter returns, across
all pages. See [Bulk operations](#bulk-operations).

![Figure 7: Columns panel](images/07-columns-panel.png)
*Figure 7 — The **Columns** panel: toggle visibility and reorder with the
up/down controls.*

---

## 5. Viewing & editing a test

Click a row to open the **detail panel** on the right. Edits here are **local
only** — they are queued as pending changes and pushed to Jira when you commit.
A dot marks each field that has an uncommitted edit.

![Figure 8: Test detail — fields](images/08-test-detail.png)
*Figure 8 — The detail panel: summary, status, priority, labels, components, and
other fields. Click a field to edit it inline.*

### Labels

The Labels field shows each label as a chip. Start typing and it suggests the
labels already used on your synced tests; press Enter or click one to add it.
If what you typed is not in the list, the last suggestion reads **Create
"…"**, and Enter adds it as a new label. Remove a label with its **×**, or
press Backspace in the empty field to remove the last one. Pasting several
labels separated by spaces adds them all.

Jira labels cannot contain spaces and are limited to 255 characters, so the
field refuses those with a message. Labels are case-sensitive: `Smoke` and
`smoke` are different labels. Each add or remove is queued as a pending
change straight away. Text you typed but did not confirm with Enter is added
when you leave the field.

#### Renaming, merging and deleting labels

**More → Labels…** lists every label on your synced tests with the number of
tests that carry it. Filter the list by typing, and sort it by label or by
test count.

- **Rename…** asks for a new name. If a label with that name already exists,
  the rename becomes a merge: the old label is replaced by the existing one,
  and a test that already had both keeps a single copy.
- **Delete** removes the label from every test that carries it.

Jira has no way to manage labels on their own, because a label exists only
as a value on issues. Each action is therefore an edit to every test carrying
the label, one pending change per test, and it reaches Jira on Commit as that
many issue updates. Before anything is queued, the confirmation says how many
changes it will be. Renaming a label used by 2,000 tests queues 2,000 pending
changes.

Only tests you have synced are changed. A test in Jira that is not in your
local copy keeps the old label. The app also refuses to rename or delete a
label while pending label edits on tests that use it are waiting to be
committed, and says how many. Commit or discard those first. A new name with
a space in it, or longer than 255 characters, is refused as in the Labels
field.

### Components

The Components field works like Labels, with three differences. It only
offers the project's components, so you cannot attach a name Jira does not
know. Names keep their spaces: typing or pasting "User Management" adds one
component, not two. If your Jira account can manage components (the
Components tab is visible), typing a new name and pressing Enter asks whether
to create it in the project, then adds it once Jira has. Each change is queued
as a pending change straight away, and the pending list shows it as a plain
list of names.

### Steps

Test steps load when you first open a test (to keep sync fast). You can add,
edit, reorder, and remove steps; each has Action, Data, and Expected Result, and
all three fields support **markdown** (here and in the New Test panel). Jira
`{color:#hex}…{color}` macros render in colour in the read view, while editing
shows the raw macro so it round-trips back to Jira unchanged.

![Figure 9: Test steps](images/09-test-steps.png)
*Figure 9 — Editing the step table: action / data / expected result, with
add, reorder, and delete.*

You can also **clone steps from another test**. The **Clone from…** action — in
the Steps header here, and in the **New Test** panel — opens a picker: choose a
source test, then tick which of its steps to copy (all by default, or a subset).
The chosen steps are appended and queued as pending like any manual edit. Handy
for seeding a test from a similar existing one.

![Figure 36: Clone steps from another test](images/36-clone-steps.png)
*Figure 36 — "Clone from…": after picking a source test, select which steps to
copy (whole or selective) before appending them.*

### Calling another test (test calls)

Instead of manual Action / Data / Expected content, a step can **call another
test** — Xray's "test call". In the Steps header click **+ Call test**, search
for and pick the test to call, and the step is appended as **⮡ Calls KEY**. Call
steps reorder, delete, and commit like any other step.

The **Test Calls** tab gives a project-wide view of these relationships:

- callers are grouped with the tests they call;
- caller rows in the Browse grid show a **⮡ calls** badge;
- a call whose target isn't in the local cache is classified as either
  **cross-project** (blue — the target lives in another Jira project, which is
  expected) or **missing** (amber — deleted in Jira or not synced yet). A legend
  explains the badges, and a note at the bottom of the view explains the
  cross-project calls and how to browse them (add the project as a cross-project
  source) or fix a mistyped key (re-sync);
- when tests call each other in a loop, every test in the cycle is flagged
  **cycle** (a cyclic call would recurse forever when executed);
- use **Expand all / Collapse all** and the pager when there are many callers.

![Figure 55: Test Calls cross-project and missing calls](images/55-testcalls-crossproject.png)
*Figure 55 — The Test Calls view: the "missing" and "cross-project" tiles, per-row
badges, and the cross-project explanation pinned at the bottom.*

### Custom fields

Xray and project custom fields also load on first open and can be edited inline.

![Figure 10: Custom fields](images/10-custom-fields.png)
*Figure 10 — Custom-field values in the detail panel.*

### Preconditions & requirements on a test

The detail panel also shows the **preconditions** the test depends on and the
**requirements** it covers, with controls to link or unlink them. Use the
searchable picker to **add several at once**, or **Replace** to swap the whole
set (unlink the current ones and link the chosen ones) in a single step. The same
applies in bulk — see [Bulk operations](#bulk-operations).

![Figure 11: Preconditions & requirements sections](images/11-detail-links.png)
*Figure 11 — The preconditions and requirement-coverage sections of a test.*

### Workflow transitions

Move a test through its workflow with a transition control from the detail
panel; the transition is queued for commit like any other edit.

### Linked bugs

If a test has defects linked to it (including bugs filed in a different Jira
project), they appear in a **Linked bugs** section on the detail panel as
clickable keys that open in the browser. See [Defect tracking](#defect-tracking).

### Run history

The detail panel's **Run history** section lists every Test Execution this test
ran in, with the result, date, who ran it, environment, Test Plan(s), Fix
Version(s), and any linked defects. It is pulled from Xray during sync, so you
see a test's run trajectory without opening Xray; execution and defect keys open
in the browser.

![Figure 42: Run history](images/42-run-history.png)
*Figure 42 — The Run history section: each execution run with result, date,
tester, environment, plan, fix versions, and defects.*

---

## 6. Creating & importing tests

### New Test

Click **New Test** (top-right, Browse view). The New Test panel collects the
summary, folder, steps, and fields. The new test gets a temporary key locally
and is created in Jira on the next commit.

If your Jira project marks other fields as required when a test is created,
the panel asks for them below the summary, marked with an asterisk, and
**Create** stays disabled until each one is filled. The values are sent when
the test is created on commit, so the commit does not fail for want of them.

![Figure 12: New Test panel](images/12-new-test.png)
*Figure 12 — Creating a test. If a folder is selected in the sidebar it is
pre-filled.*

### Import from CSV / XLSX

To bulk-create tests from a spreadsheet, open **More → Import tests…**. Map your
columns to test fields and import; the rows are queued as new tests for commit.

![Figure 13: Import tests modal](images/13-import-tests.png)
*Figure 13 — Importing tests from CSV or XLSX with column mapping.*

---

## 7. Bulk operations

Select two or more tests (checkboxes, or **Select all matching**) and the **bulk
toolbar** appears above the grid:

- **Bulk edit…** — set a field across all selected tests, including the
  **Execution Type** (Xray Test Type), priority, labels, components, and custom
  fields.
- **Labels…** — add and remove several labels at once. Pick labels to add
  (existing ones, or new ones you type) and labels to remove (existing ones
  only). A line under the pickers tells you how many of the selected tests will
  change. A label can't be in both lists. Tests that would end up with the same
  labels are left alone, so they get no pending change.
- **Components…** — change components on all selected tests. **Add and
  remove** works like Labels…. **Replace all** sets every selected test to the
  components you pick; picking none clears them. The line under the pickers
  says how many tests will change.
- **Replace step text…** finds a string in the steps of every selected test
  and replaces it across them. Tick which of **Action**, **Data** and
  **Expected** to search, and narrow it with **Match case** and **Whole word**.
  **Find matches** writes nothing: it lists one row per matching step field
  with the text now and the text after, and any row can be unticked before you
  click **Replace**. Tests with no match are counted below the list and are
  never written to.
- **Bulk transition…** — move all selected tests through a workflow transition.
- **Allocate…** — add the selected tests to a Test Set / Plan / Execution.
- **Move to folder…** — re-file into a Test Repository folder.
- **Preconditions…** — link, unlink, or **Replace** (swap) preconditions in bulk.
- **Requirements…** — link, unlink, or **Replace** requirement coverage in bulk.

![Figure 14: Bulk toolbar](images/14-bulk-toolbar.png)
*Figure 14 — The bulk toolbar after selecting several tests.*

![Figure 15: A bulk modal](images/15-bulk-edit.png)
*Figure 15 — Example: the Bulk edit dialog. Every bulk action queues pending
changes rather than writing straight to Jira.*

---

## 8. Preconditions

The **Preconditions** tab is a dedicated surface for managing Xray
Preconditions. The left side is a filterable **card list**; selecting a card
opens its detail on the right. Each card carries the precondition's key, its
type, its summary, its workflow status and the number of tests that use it, and
**Sort** orders the list by Key, Type, Status or Usage.

![Figure 16: Preconditions view](images/16-preconditions.png)
*Figure 16 — Preconditions: card list (left) with a detail pane (right) for the
summary, type, description, and the tests that use it.*

The detail opens **read-only**; use **Edit**, **Save**, and **Cancel** to change
it. In edit mode you can:

- Edit the **summary**, **type** (Manual / Generic / Cucumber), the
  **description** (markdown-aware), and a dedicated **Condition** field (the
  precondition's given-state text). The Condition and Description sections are
  collapsible.
- See and paginate the **tests that use** the precondition, and unlink any.
- **+ Add tests** to link more tests.
- Delete the precondition (queued for commit).

Create a new precondition with **New precondition**:

![Figure 17: New precondition dialog](images/17-precondition-create.png)
*Figure 17 — Creating a precondition. It is created in Jira on the next commit.*

The type you pick (Manual, Generic or Cucumber) is the precondition's type in
Jira once committed. If your Jira project marks other fields as required when a
precondition is created, the dialog asks for them under Description, and
**Create** stays disabled until each one is filled.

### Importing preconditions

**Import…**, under the **⋯** beside **New precondition**, reads a `.csv` or
`.xlsx` file with
the columns Summary, Type, Condition, Description and Labels. **Download
template** saves a CSV with those columns and two example rows. After you
pick a file, each field is matched to a column by its header name; change
any match with its dropdown. Only Summary is required.

- **Type** is Manual, Generic or Cucumber, in any case. A blank Type means
  Manual.
- **Labels** are separated by spaces, as in the Labels field.
- If Jira requires other fields when a precondition is created, the dialog
  asks for them once and applies the answers to every row.

**Validate** checks the file without changing anything. It reports how many
rows are new, how many already exist, and the rows that need fixing. A row is
treated as already existing when its summary matches an existing precondition,
ignoring case and extra spaces, or matches an earlier row in the same file.
Those rows are skipped, so importing the same file twice creates nothing the
second time.

**Import** queues each new row as a pending create, committed from the Pending
list. If any row has an error, such as an empty summary or a Type Xray does
not have, nothing is imported, whether or not you pressed Validate first. Fix
the rows and import again.

---

## 9. Requirements & coverage

The **Requirements** tab shows requirements (e.g. Stories/Epics, possibly from a
different project) and their **derived coverage status** from the tests that
cover them.

![Figure 18: Requirements view](images/18-requirements.png)
*Figure 18 — Requirements: a card list with coverage badges and filter pills
(Failed / Not run / Passed / Uncovered), and a detail pane listing the covering
tests with their run results.*

- **Coverage pills** at the top filter the list by status; the count on each
  pill updates live.
- Each card shows the requirement key, issue type, summary, coverage badge, and
  test count.
- The detail pane lists the **covering tests** (paginated) with run results, and
  lets you edit the summary or delete the requirement.
- The detail also shows the requirement's **priority, component(s), fix
  version(s), sprint**, and a collapsible **markdown description**. **Edit** mode
  edits every field, and Components and Fix Versions offer the project's
  available values (select-or-add).
- Clicking a covering test opens its detail **read-only in a side panel** to the
  right, so you can inspect its steps and fields without leaving the Requirements
  view (the same side-panel layout used in the Bugs view).
- **+ Add tests** links covering tests to the selected requirement directly from
  this view (a searchable multi-pick), queued for commit like any coverage edit.
- **Export audit…** writes the coverage / sign-off audit to CSV or XLSX.

![Figure 53: Requirement covering-test detail](images/53-requirement-test-detail.png)
*Figure 53 — A covering test opened read-only in a side panel beside the
requirement detail.*

### Creating, importing & linking requirements

- **New requirement** drafts a requirement in a left-docked panel: pick the
  project and issue type and write a markdown summary. It gets a temporary key
  locally and is created in Jira on the next commit.
- **⋯** beside **New requirement** holds the actions used less often:
  **Sources…**, **Import…** and **Export audit…**.
- **Import…** loads requirements from a CSV or XLSX file, de-duplicating by
  summary (like Gap Analysis) and offering a downloadable template.
- **Link…** relates the selected requirement to other requirements (for example
  **Requires**).
- **Link type.** Which Jira issue-link type connects a Test to a Requirement on
  commit is configurable in **Settings**. The default is **"Tested By"**, chosen
  from the link types your instance defines.

### Requirement sources

Requirements can come from projects other than your test project. Configure
which projects and issue types are pulled with **⋯ → Sources…**:

![Figure 19: Requirement sources modal](images/19-requirement-sources.png)
*Figure 19 — The Requirement Sources dialog: add a source project, the issue
types to include, and an optional scope. Sync afterward to pull them in.*

---

## 10. Finding duplicates

The **Duplicates** tab surfaces likely-duplicate tests so you can consolidate
them. Results are grouped; each group lists its members with a detail panel.

![Figure 20: Duplicates view](images/20-duplicates.png)
*Figure 20 — Duplicate groups with a paginated list and a balanced detail panel
for the selected group.*

Use **Compare steps** to see members' steps **side by side**, with differing
rows highlighted — useful when deciding which copy to keep. **Compare summaries**
does the same for the members' summaries. Re-running the scan (**Scan**) shows a
progress bar in the status bar while it works.

![Figure 21: Step comparison](images/21-duplicates-compare.png)
*Figure 21 — Side-by-side step comparison across the duplicated tests
(**Compare summaries** offers the same view for summaries).*

---

## 11. Test Sets, Plans & Executions (Containers)

The **Containers** tab manages the three Xray container types. Pick a **kind**
(Test Set / Test Plan / Test Execution), then choose a container from a
**type-to-filter dropdown** that you can narrow by keyword, status, and — for
executions — Standalone vs Sub-task. A **Bugs** toggle at the top of the tab
switches to the defect view (see [Defect tracking](#defect-tracking)).

![Figure 22: Containers view](images/22-containers.png)
*Figure 22 — The Containers view: kind selector, container list, and member
tests. Tables default to 15 rows per page.*

You can:

- Create a container with **New Test Set**, **New Test Plan** or **New Test
  Execution** (the button follows the kind selector), and allocate tests to it.
- Add or remove member tests.
- For a **Test Execution**, set each member's **run result** inline (e.g. PASS /
  FAIL / TODO), or apply one result to many selected members at once. A compact
  bar summarizes the run-status mix.

![Figure 23: Run results per test](images/23-execution-runstatus.png)
*Figure 23 — Each member's run result shows in the Execution column (PASS /
FAIL / TODO / ABORTED / EXECUTING), summarized by the bar above. In a Test
Execution these are editable inline, and one result can be applied to many
selected tests at once.*

**Sub-task Test Executions.** Xray lets a Test Execution be a *sub-task* of a
parent issue (often a Story). These sync and appear in the same execution list
as standalone ones; the **Standalone / Sub-task** filter narrows to either kind,
sub-task entries are tinted in the picker, and the selected execution's card
shows a **↳ parent** link that opens the parent issue in the browser. They use
all the same run-result and create-bug features.

**Test Environments & Fix Version(s).** A Test Execution's detail shows its Xray
**Test Environments** as chips you can edit — add or remove an environment on the
selected execution, or apply environments to several executions at once — and the
execution list can be filtered to a chosen environment. The execution's Jira
**Fix Version(s)** appear alongside as read-only chips. Both standalone and
sub-task executions are covered.

![Figure 41: Test Environments and Fix Versions on an execution](images/41-execution-environments.png)
*Figure 41 — A sub-task Test Execution showing its editable Test Environments
chips and the batch environment controls. (Fix Version(s) render as read-only
chips when the execution has them.)*

**Cross-project members.** A Test Execution can include member tests that live in
a *different* Jira project. These are cached locally and shown on the board with
their run results just like in-project members, and their linked bugs are
harvested into the [Bugs panel](#defect-tracking) and the test detail. The
[Traceability](#traceability) Execution flow can include or exclude them with
the **include cross-project members** toggle.

**Run details on executions.** A Test Execution's member table also shows, per
test, the run **Date**, the **tester**, and the **environment** the run used,
beside the editable run-result control.

![Figure 43: Execution run details](images/43-execution-runs.png)
*Figure 43 — A Test Execution's members with run date, tester, and environment
columns alongside the editable result.*

**Plan / Set run roll-up.** Selecting a Test Plan or Test Set shows a **run
roll-up** bar above the board: passed / failed / not-run / executing / aborted /
blocked counts across the executions that ran its tests.

![Figure 44: Plan run roll-up](images/44-plan-rollup.png)
*Figure 44 — The run roll-up on a Test Plan, summarizing results across its
executions.*

**Open a test read-only.** The open-detail icon on a member test opens that
test's **read-only** detail in a side panel beside the board (the same panel the
Bugs view uses, see [Defect tracking](#defect-tracking)), so you can read its
steps and fields without leaving Containers.

**Per-test Fix Version filter.** Each member of a Test Execution shows its **own**
Jira Fix Version(s) in a **Fix Version** column (members can differ from one
another). The execution's Fix Version chips are clickable: select one to filter
the member list to the tests carrying it, and click it again (or **All**) to
clear.

![Figure 52: Per-test Fix Version on an execution](images/52-exec-fixversion-filter.png)
*Figure 52 — A Test Execution's member table with the per-test **Fix Version**
column and the run-status results highlighted in full colour.*

**Defects and remarks on a run.** Each member of a Test Execution also has
**Defects** and **Remarks** columns for recording notes on that specific run:

- **Link a defect.** Click the **＋** in the Defects column to open a picker.
  Search the cached bug list by key or summary, or type a bug key directly
  (e.g. `PROJECT-123`) and confirm to link it even if it is not yet cached
  locally. Linked defects show as removable chips; click a chip's **✕** to
  unlink it from the run.
- **Add or edit a remark.** Click into the Remarks cell and type; the remark
  saves when you press **Enter** or move focus away (on blur).

Both actions are staged as local pending changes, just like the run-status
control, and are pushed to Jira/Xray when you **Commit** (a run-defect link
and a run-comment update).

![Figure 55: Test Execution run defects and remarks](images/55-exec-defects-remarks.png)
*Figure 55 — A Test Execution's member table with the **Defects** column (linked
bug chips and the ＋ picker) and the editable **Remarks** column.*

### A test with a dataset

A member test that uses a dataset expands into one result per parameter
combination, on a live Xray profile as well as on a demo one. The header says
how many iterations are finished and which dataset the run took its rows from.
Steps arrive one call per iteration, so a long dataset takes a moment to fill
in.

Each iteration row carries the values it ran with and its result, and expanding
one shows its steps with those values already substituted. Pick a result in an
iteration's **Result** column to record it.

To set several at once, tick the iterations you mean and a bar appears above the
table: choose a result, then **Apply**. The tick in the header row selects every
iteration, and **Clear** drops the selection without applying anything. An
expanded iteration has the same for its steps. This matters most on a
combinatorial dataset, where three parameters over three rows run as nine
iterations rather than three.

A bulk apply is the same edit as picking each row by hand, so it journals the
same way and commits the same way. If one row cannot take the result the others
still apply, and the message above the table names the ones that did not.

- A result is recorded locally and sent to Jira when you commit, like every
  other edit. Nothing reaches Xray as you pick it, so a result picked by
  mistake can be discarded from Pending changes instead of having to be put
  back. An uncommitted result is marked **not committed** beside it.
- Committing a result Xray already holds clears the row without writing again,
  or holds it as a **Dataset iteration result** conflict if somebody moved that
  iteration to a third result in between. A commit refused with a 403 that
  names a permission usually means the Test Execution is closed, where the
  cause is the execution's state rather than the account.
- Xray derives the test's own run result from its iterations, by a rule XTM
  does not own. The **Execution** column therefore shows what Xray last
  reported, and does not move until your results are committed.
- A step's result is recorded locally on any profile, but only a demo profile
  can send one. Xray gives a step inside an iteration no id of its own, so
  there is nothing for a step write to address, and committing one on a live
  profile reports it as skipped and leaves it pending.
- A dataset of one row produces no iterations, because Xray keeps that run's
  parameters on the run itself. Record its result in the test's own
  **Execution** column.
- Refreshing or syncing the run brings Xray's results for everything you have
  not touched, and leaves an uncommitted result alone.

A backend that reports no write support marks the panel **Read-only results**
and shows each result as a badge instead of a dropdown.

### Import JUnit XML results

You can update run results in bulk by importing a **JUnit-compatible XML** report.
Each `<testcase name>` is matched to a test by its **summary** (normalized), and
the result maps as: no failure/error → **PASS**, a `<failure>`/`<error>` →
**FAIL**, a `<skipped>` is left unchanged. Two entry points on the Test Execution
view:

- **Import results (JUnit XML)** updates the **selected** execution. Matched
  member tests get the imported result; unmatched or ambiguous testcases are
  reported and skipped.
- **New exec from JUnit XML** **creates a new Test Execution** from the report:
  enter the execution summary, optionally tick **Create missing tests** (any
  testcase with no matching test is created first), then pick the file. All tests
  are allocated to the new execution and given their results.

For a parameterized test, add one JUnit `<property>` for every dataset
parameter, so the testcase names a full parameter combination. XTM matches that
signature to one iteration and records the result against it. If the signature
matches more than one iteration, or the dataset has changed since the run, the
result is kept as a conflict rather than written to the wrong iteration. A new
execution has no server run ids until it exists, so the signature is stored and
resolved on commit.

Either way the work is queued as pending changes and a preview shows what will be
applied before you commit. See [Committing changes](#committing-changes-to-jira).

![Figure 50: Create a Test Execution from a JUnit report](images/50-junit-new-exec.png)
*Figure 50 — The "New execution from JUnit XML" dialog: an execution summary, the
**Create missing tests** option, and the file picker.*

### Generate a pytest scaffold

From a Test Plan or Execution you can generate a **pytest** scaffold for its
tests — either plain `@pytest.mark.xray` functions or a `unittest`-style class.

![Figure 24: Generate pytest](images/24-pytest.png)
*Figure 24 — The Generate pytest menu and the resulting scaffold file.*

### Project components (Components tab)

On a Jira profile, the **Components** tab (also under **View → Components**)
manages the project's Jira components. It is hidden on Kiwi profiles.

The table lists each component with its description, lead, and the number of
synced tests that carry it. The test count comes from your local copy, so it
reflects the last sync.

- **New component** opens a form for the name, a description, a lead, and the
  default assignee (project default, component lead, project lead, or
  unassigned). Find a lead by typing at least two characters of their name or
  username and picking from the results.
- **Bulk create…** creates several components at once. Type the names one per
  line, or load an `.xlsx` workbook with Name, Description, Lead username and
  Default assignee columns. **Download template** saves a workbook with those
  columns and two example rows. Default assignee takes the wording the form
  uses (Project default, Component lead, Project lead, Unassigned). Older
  `.xls` files are not read; save them as `.xlsx` first.

  Before anything reaches Jira, a preview lists every row and marks the ones
  that will be skipped: a blank name, a name the project already has, a name
  another row also creates, or an unknown default assignee. Names compare
  without regard to case, as Jira's do. **Create** then creates the rest, one
  at a time. If Jira refuses a row, for example because the lead username does
  not exist, the other rows are still created, and the result lists each
  refusal with Jira's reason.
- **Edit** opens the same form for an existing component. **Clear lead**
  removes the lead.
- **Delete** first asks Jira how many issues use the component. If any do, you
  can move them to another component as part of the delete, or just remove the
  component from them.

These changes go to Jira straight away. They do not wait for a commit and do
not appear in pending changes. After each one the app reloads the component
list, and a rename or delete also updates the components shown on your synced
tests.

The app refuses to rename or delete a component while pending changes edit
components on tests that use it, because those edits would name a component
Jira no longer has. Commit or discard them first. If your Jira account lacks
project admin rights, Jira refuses the change and the tab says so.

---

## 12. Datasets

The **Datasets** tab lists the parameterized tests in the selected profile, one
row per test, with the test's own default dataset, its Plan and Execution
overrides, and the containers that use it. The tab appears only on a backend
that reports dataset support.

### Build the catalog

Open **View → Datasets** and click **Scan datasets**. A normal profile sync
does not build this catalog: XTM has to inspect the project's tests, Plans and
Executions one at a time, so the scan is a step of its own.

The scan only reads. It edits no test and queues no pending change. The previous
catalog stays usable while a new scan runs, and a scan that fails or is
cancelled leaves the last complete snapshot in place rather than replacing it
with partial results. A view saying the project has not been scanned means this
profile has no successful snapshot yet.

The timestamp on the view is when that snapshot was taken. It does not track
Jira live, so a change made since then shows up only after another scan.

### Read a test group

Each row is one test, not a reusable project-level dataset. Expand it for one
card per scope that exists:

- **Default dataset**: the parameters and rows defined on the test.
- **Plan overrides**: a different dataset for that test, defined in a Test
  Plan.
- **Execution overrides**: a different dataset for that test, defined in a Test
  Execution.
- **Used by**: every synced Plan and Execution that holds the test. Click a
  container key to open it.

A usage reading **Inherited default** uses the test's own dataset, **Override**
has its own definition for that container, and **Override only** means the test
has no default while a Plan or Execution defines one.

Plan exports carry no parameter type metadata, so a type XTM cannot read is
shown as unknown.

### Profile and project boundaries

The selected profile decides what the catalog holds: tests owned by the
profile's Jira project, plus tests from another project that one of the
profile's Plans or Executions uses. Those are marked **External**.

An external test may not be in the local Browse cache. Its dataset preview and
its Jira action still work, but **Open Test** is hidden, because it would open
an empty local view.

### Find and open related work

Search by test key, summary, Plan key or Execution key. **Dataset scope**
narrows the list to defaults, Plan overrides or Execution overrides, and **Test
ownership** switches between project-owned and external tests.

From an expanded row, **Open Test** opens it in Browse when XTM holds it
locally, a Plan or Execution link opens that container in Containers, and
**Open _key_ in Jira** opens the issue on the configured Jira site.

### Where editing and results happen

The catalog is read-only. Edit a test's default dataset from its detail panel in
Browse, below the steps, where **Add row** adds an iteration and the arrows at
the end of a row set the order the iterations run in. Plan and Execution
overrides cannot be edited from XTM at all.

The editor takes Text parameters and ad-hoc List parameters, and stores a list's
options with the test's dataset. A parameter bound to an existing predefined
Jira list keeps that binding until you confirm converting it to an ad-hoc list;
XTM never creates, edits or deletes a project list. A CSV import matches columns
to existing parameters by name, so those keep their list metadata, and a column
matching no parameter becomes Text.

Run results are not recorded here. A dataset test's row in a Test Execution
expands into one result per iteration: see
[A test with a dataset](#a-test-with-a-dataset).

---

## 13. Defect tracking

When a test fails you can raise and track a defect (bug) for it without leaving
the app. Defect tracking spans three places:

- **Create a bug from a failed run.** In a Test Execution, a failed member row
  shows a **🐞** action that opens a create-bug form (summary, description,
  priority, labels). The bug is queued as a pending change, filed in Jira on the
  next commit, and linked to the test.
- **The Bugs panel.** The **Bugs** toggle at the top of the Containers tab lists
  every defect linked to this profile's tests — a paginated master list on the
  left and, for the selected bug, its details plus the affected tests (with their
  run status) on the right. Bug keys open in the browser; test keys open the
  test.
- **Linked bugs on a test.** A test's detail panel shows its linked bugs,
  including ones filed in a different project, as clickable keys.

![Figure 37: Bugs panel](images/37-bugs-panel.png)
*Figure 37 — The Bugs panel (Containers → Bugs): linked defects with the selected
bug's details and the tests it affects.*

Which Jira project a new bug lands in, and its issue type, are configurable per
profile (the test project, the execution's project, or a dedicated defect
project). Bug sync respects the profile's scope and shows progress in the status
bar.

On a Kiwi profile, bugs are filed into a separate Jira bug tracker configured
on the profile rather than into the same connection. See [Reporting Kiwi
defects into Jira](#reporting-kiwi-defects-into-jira).

**Affected-tests breakdown.** The affected-tests table shows each test's
**Project**, and each row expands to a per-test run breakdown — execution,
result, fix version, environment, date, tester, **Created / Updated**, and
defects — drawn from the test's run history, so you can see how each affected
test fared without opening Xray. The runs are **grouped by Test Plan** (a plan
can span several executions and fix versions), and the run rows **sort** by
result, Created, or Updated.

![Figure 45: Affected-tests breakdown](images/45-bug-breakdown.png)
*Figure 45 — A bug's affected tests with the Project column and the expanded
per-test run breakdown grouped by Test Plan, with Created / Updated columns.*

**Open a test read-only.** The open-detail icon on an affected test opens that
test's **read-only** detail in a side panel beside the bug detail, mirroring the
Duplicates description/steps panel — so you can inspect its steps and fields
without leaving the Bugs view. The test key still opens the test in Browse, and
your Bugs selection and filters are restored when you return.

![Figure 46: Read-only test detail in the Bugs view](images/46-bug-test-detail.png)
*Figure 46 — The read-only test detail opened to the right of the bug detail.*

**Add affected tests to an execution.** Alongside **Create Test Execution**
(which builds a new execution from the checked bugs' tests), **Add to
execution…** allocates those tests to an **existing** Test Execution picked from
a searchable list.

![Figure 47: Add tests to an existing execution](images/47-add-to-execution.png)
*Figure 47 — Adding the checked bugs' affected tests to an existing Test
Execution.*

---

## 14. Dashboard

The **Dashboard** tab summarizes the cached project from the local cache: total
tests, pending changes, and breakdowns by status, priority, folder, label,
component, and **requirement**, plus execution coverage, requirement coverage,
a **By requirement** chart beside By component, Test Set / Plan /
Execution counts, a duplicates card, and a recently-updated trend. Filter the
whole dashboard by **folder**, **component**, and **status** to scope every card
to a slice of the project. **Refresh** recomputes it from the cache without a
sync, and **Export XLSX** writes a workbook with a summary sheet plus one sheet
per breakdown (honouring the active filters). The traceability diagrams live in
their own [Traceability](#traceability) tab.

![Figure 25: Dashboard](images/25-dashboard.png)
*Figure 25 — The statistics dashboard with summary cards and breakdowns.*

---

## 15. Traceability

The **Traceability** tab holds three Sankey diagrams behind a tab bar; each keeps
its own filters and only one shows at a time. Hover any node to trace its
threads.

- **Execution** (default) — flows **Test Plan → Test Execution → run status**.
  The plan and execution filters are multi-select (the execution list narrows to
  the chosen plans), and a **Cross-project only** toggle keeps just the runs
  whose Test Execution lives in a *different* Jira project than the current
  profile; while it's on, a **cross-project bugs** list appears beside the
  diagram.
- **Requirement** — flows **Requirement → Epic → Coverage → Test plan → Test →
  Test result** (the covering test case is shown explicitly), with a multi-select
  requirement filter. The **Epic** layer appears when a requirement has an epic
  and is skipped otherwise, falling back to the direct Requirement → Coverage
  flow.
- **Sub-task** — flows **Parent issue → Test Execution → run result** over
  sub-task Test Executions, with a **Parent** filter (each parent is shown as
  *key — summary*). Use it to see, per parent
  issue, which sub-task executions ran its tests and how they turned out.

Each tab's toolbar offers **Export XLSX**, which writes the current diagram as a
flow sheet plus a flat table of the same data. The **include cross-project
members** toggle controls whether member tests that live in other Jira projects
are drawn in the flow.

![Figure 27: Execution traceability](images/27-execution-sankey.png)
*Figure 27 — The Execution tab: Test Plan → Test Execution → run status.*

![Figure 26: Requirement traceability](images/26-requirement-sankey.png)
*Figure 26 — The Requirement tab: requirement → coverage → Test plan → test →
run result. The **Test** column shows the covering test case between its plan and
its result.*

![Figure 38: Sub-task traceability](images/38-subtask-sankey.png)
*Figure 38 — The Sub-task tab: parent issue (key — summary) → execution → run
result, with the
Parent filter.*

---

## 16. Gap analysis

The **Gap Analysis** tab compares two sets of tests and reports what each one is
missing, so coverage gaps — between two projects, or between a spreadsheet and a
project — are easy to spot.

Set up the comparison at the top of the tab:

- **Reference** — the baseline to compare against: the **active project** (its
  cached tests) or an **uploaded** CSV / XLSX file.
- **Target** — the CSV / XLSX file to check against the reference.
- **Compare by** — match tests by **summary**, or by **summary + folder** (which
  also flags tests that match by summary but sit in a different folder).
- **Three-way** (file reference only) — also compare both files against the
  active project, for a complete-project view.
- **Download template** — get a ready-made CSV / XLSX template (full, summary, or
  summary + folder) to fill in as the reference or target.

Click **Run analysis** to produce the result.

![Figure 39: Gap Analysis setup](images/39-gap-analysis.png)
*Figure 39 — The Gap Analysis tab: choose the reference and target, the match
mode, and run.*

The result shows summary cards (**Matched**, **Orphaned in reference**, **Orphaned
in target**, **Total gap**) plus two lists:

- **Target → Reference** — tests in the target that are missing from the
  reference. These are addable: tick the ones you want and **Add selected as
  tests** to create them as pending new tests (commit them later from the Pending
  list).
- **Reference → Target** — tests in the reference that are missing from the target
  (report only).
- **Export CSV** / **Export Excel** writes a formatted report with a section per
  list (plus the folder mismatches, when comparing by summary + folder).

![Figure 40: Gap Analysis results](images/40-gap-analysis-result.png)
*Figure 40 — Gap Analysis results: the summary cards, the Target → Reference list
(addable) and Reference → Target list (report only), with the export actions.*

---

## 17. Coverage module

The **Coverage** tab is an opt-in, local-only workspace for **parameter-level
test coverage** and **cross-project functional-requirement reuse**. It is hidden
by default; turn it on with **More → Show Coverage tab**.

Requirement coverage (section 9) answers "does a test cover this Jira
requirement". The Coverage module goes further: it measures how thoroughly a
*function* is tested by breaking it into the parameter **values** that must be
exercised, and it lets you define that once and reuse it across many customer
projects. Everything is stored in the local cache, and nothing here needs Jira
admin.

### Functions, parameters, and values

- Define a **functional requirement** (a function under test) and decompose it
  into **parameter groups** and **parameter values** (valid values, error codes,
  and boundaries).
- Map your existing Tests to each value. Coverage is the share of **required
  values that have at least one mapped test** (the parameter-level definition,
  not an exhaustive combinatorial one).
- Values with no test are **gaps**; a mapping whose Test was later deleted is
  flagged **stale** and kept, never removed automatically.

### Reuse across projects

- Group equivalent requirement issues from many customer projects under one
  local **canonical functional requirement**, so coverage is defined once and
  reused. The module shows which projects reuse each requirement.

### Versions and change requests

- A functional requirement can hold multiple **versions** (for example 2.40 and
  3.0); coverage is measured per version, and a version can be **cloned** to
  start a new line.
- Customer requirements are **locked to a version**, and **change requests**
  record each customer's accept, reject, or pending decision, with
  version-distribution and adoption dashboards.

### Coverage Map

- The **Coverage Map** sub-tab (open it without picking a function) relates your
  in-scope projects to the canonical requirements: a per-project coverage panel
  (coverage percentage, requirement count, functions reused) and a **Sankey**
  flowing from project to function to covered or gap.
- The in-scope projects are **configuration** (a per-profile source and customer
  list) that seeds itself from your existing members.

### Seeding and export

- **Import** the PKCS#11 parameter-extraction Excel workbook to seed a model, or
  create a profile with the Jira URL **`demo-pkcs`** for a fully populated
  example across every view.
- **Export** a styled Excel report with Summary, per-group, Gaps, By-project,
  and Reuse-map sheets for Jira or Confluence.

---

## 18. Spellcheck

**Spellcheck** scans your synced tests for typos in the summary, the
description, and the Gherkin or Generic definition text. Every fix is queued as
a pending change, so nothing reaches Jira until you commit. Open it from the
**Spellcheck** tab or the **View** menu.

### Scan for typos

Click **Scan for typos**; the button reads **Scanning…** while it runs. When
it finishes, the number of findings appears beside the button and the table
fills in. Each row is one finding: the test, the field, the word, a snippet of
its context, and the suggested replacements.

![Figure 56: Spellcheck findings](images/spellcheck-02-results.png)
*Figure 56: the Spellcheck table after a scan, one row per finding.*

Technical terms such as `csv` or `api` are reported too. Ignore one once and it
stops appearing.

### Fix one finding

Clicking a suggestion in the **Suggestions** column replaces the word and queues
the change. To see more first, click the row: a panel opens on the right with
the suggestions, a **Replace with** box, the full context, and **Ignore word**.
Type a replacement and the panel shows the text before and after it;
**Replace** queues it.

![Figure 57: A replacement previewed](images/spellcheck-04-replace-preview.png)
*Figure 57: the finding panel, previewing the text before and after the
replacement.*

### Fix or ignore many findings

Tick one finding and XTM selects every finding of the same word, so a single
tick can select hundreds of rows. The bar above the table says how many
findings and how many words are selected.

With one word selected, pick a suggestion beside **Replace all:**, or type into
**Replace all with…** and click **Replace all**. **Ignore selected** ignores
the selected words instead, and **Clear** deselects everything.

![Figure 58: Every csv finding selected](images/spellcheck-05-bulk.png)
*Figure 58: one tick selected every finding of the same word.*

### The ignore list

**Ignore** on a row, **Ignore word** in the panel and **Ignore selected** in the
bar all add words to the ignore list, and an ignored word is skipped by every
later scan. **Ignore list** opens it: **Add a word to ignore…** adds a word,
**Filter…** finds one, a word's remove button takes it off, and **Done**
closes the dialog. The ignore list is shared by all of your profiles.

![Figure 59: The ignore list](images/spellcheck-06-ignore-list.png)
*Figure 59: the ignore list, which every profile shares.*

The table is a snapshot of the last scan, so scan again after fixing a batch. A
word you fixed is still the old word in Jira until you commit.

---

## 19. Committing changes to Jira

All edits — field changes, steps, transitions, links, bugs, new entities,
deletions — are queued locally as **pending changes**. Nothing reaches Jira
until you commit.

The **pending badge** in the top bar shows the count. Click it to review:

![Figure 28: Pending changes](images/28-pending-changes.png)
*Figure 28 — The Pending Changes dialog: review each queued edit, discard
individual ones (or **Discard all** to revert everything), jump to the affected
test, **Commit** everything, or commit a selected subset.*

On **Commit**, the app pushes changes to Jira grouped by test. Results are
reported per test:

- **Succeeded** — applied; the pending rows are cleared.
- **Failed** — kept so you can retry or discard.
- **Conflict** — the test changed in Jira since your edit was based on it. The
  test is held back so you can **sync**, then either **override** (re-base your
  edit onto the latest remote) or **keep remote** (discard your local edit).
  Conflicts surface in this same dialog with the override / keep-remote actions.

---

## 20. Syncing

**Sync** pulls the latest tests from Jira into the local cache. The status bar
shows progress per phase (tests, folders, preconditions, containers,
requirements, bugs, custom fields).

When a sync fails, the whole bar turns into a red ribbon saying what Jira
answered and what to change: the token, the Jira URL and project key, or
nothing at all when Jira itself failed. **More → Sync history** has the run and
**More → Diagnostics** has the log.

![Figure 30: Sync in progress](images/30-sync-progress.png)
*Figure 30 — A sync in progress, with the phase and item counts in the status
bar.*

> **Profile switching is locked during a sync.** While any sync is running (a
> full sync or a per-view refresh), the profile selector in the top bar is
> disabled, so you can't switch projects mid-pull and race the in-flight writes.
> It re-enables automatically as soon as the sync finishes.

![Figure 54: Profile selector locked during sync](images/54-profile-locked-sync.png)
*Figure 54 — During a sync the profile selector is greyed out and the **Sync**
button reads "Syncing…"; switching profiles is blocked until it completes.*

- **Sync** (top-right) — incremental pull since the last watermark.
- **More → Full resync (re-pull folders)** — ignores the watermark and re-maps
  Test Repository folder membership. Slower; use after big folder reshuffles.
- **More → Sync history** — past syncs with timing and counts.

**Per-view refresh.** You don't always need a full sync. Some views refresh just
their own data:

- **Requirements → Sync** — pulls only requirement coverage from Jira.
- **Containers → Sync** — pulls only Test Sets / Plans / Executions.
- **Containers → Bugs → Sync** — pulls only the defects linked to your tests.
- **Dashboard → Refresh** — recomputes the dashboard from the local cache.
- **Duplicates → Scan** — re-scans the cache for duplicates.

These per-view syncs now show their progress in the status bar, the same as a
full sync.

![Figure 31: Sync history](images/31-sync-history.png)
*Figure 31 — The paginated sync history.*

---

## 21. Settings & profile management

The **Profile** menu (top-left) manages the active profile:

- **Set as default** — auto-select this profile at launch.
- **Edit profile…** — name, URL, project key, scope.
- **Set scope…** — a JQL scope that narrows which tests sync (blank = all).
- **Set token…** — set or rotate the PAT.
- **Export profile…** / **Import profile…** — share a profile's config (without
  its token).
- **New profile…**

![Figure 32: Profile menu](images/32-profile-menu.png)
*Figure 32 — The Profile menu.*

Switch the **color theme** from **More → Theme: Light / Dark / System**.

![Figure 33: Dark theme](images/33-dark-theme.png)
*Figure 33 — The app in dark theme.*

---

## 22. Diagnostics & troubleshooting

**More → Diagnostics** shows the test cache database path (your profiles live
in the shared `agile-suite/profiles.db`, whose path appears in the exported
diagnostics), log path, schema version, and environment details, useful when
reporting an issue.

![Figure 34: Diagnostics](images/34-diagnostics.png)
*Figure 34 — The Diagnostics dialog.*

| Symptom | What to try |
| --- | --- |
| Blank window on launch | Install the Microsoft WebView2 runtime, relaunch. |
| "Backend failed to start" | Note the DB/log path shown, check the log; try removing the test cache database (`xtm.db`) and relaunching; this does not touch your profiles, which live in the shared `agile-suite/profiles.db`. |
| Sync shows 0 tests | Check the project key and that the PAT has access; confirm the scope JQL isn't excluding everything. |
| Edits not in Jira | They are local until you **Commit** (see §19). Check the pending badge. |
| A commit reports **Conflict** | Sync, then override or keep-remote in the Pending Changes dialog. |
| Token rejected | Rotate it with **Profile → Set token…**. |

**About** (native menu / **More**) shows the version.

![Figure 35: About](images/35-about.png)
*Figure 35 — The About dialog with the version number.*

---

## Glossary

- **PAT** — Personal Access Token; how the app authenticates to Jira DC.
- **Pending change** — a local edit queued in the app, not yet in Jira.
- **Commit** — push all (or selected) pending changes to Jira.
- **Sync** — pull the latest from Jira into the local cache.
- **Coverage** — a requirement's status derived from its covering tests' results
  (Passed / Failed / Not run / Uncovered).
- **Container** — a Test Set, Test Plan, or Test Execution.
- **Run result** — a test's status within a Test Execution (e.g. PASS/FAIL/TODO).
- **Demo profile** — a profile with Jira URL `demo`; runs fully offline on
  generated data.

---

*This guide is distributed with Xray Test Manager. Screenshots are captured from
a demo profile; your data and exact field set may differ by Jira/Xray
configuration.*

---

<!-- Include directive (docfilter.lua): a block-quote that is exactly one link
     to a readable file is replaced in place by that file's parsed content.
     Keep this as the sole content of the block-quote. -->

> [Screenshot capture guide](references/SCREENSHOTS.md)
