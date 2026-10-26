# Datasets

The Datasets view lists the parameterized Tests in the selected profile. Each
Test appears once. Under it you see the Test's default dataset, its Plan or
Execution overrides, and the containers that use it.

The view is available for Xray profiles. Backends that do not support datasets
do not show it.

## Build the catalog

Open **View → Datasets**, then click **Scan datasets**. A normal profile sync
does not build this catalog. The scan is a separate step because XTM has to
inspect the project's Tests, Plans and Executions one at a time.

The scan reads Jira and updates XTM's local catalog. It does not edit any Test
or send pending changes to Jira. You can keep using the previous catalog while
a new scan runs.

If a scan fails, XTM shows the error and keeps the last successful catalog. A
failed or cancelled scan never replaces a complete snapshot with partial
results. If the view says the project has not been scanned, that profile has
no successful snapshot yet.

## Read a Test group

Each row is one Test, not a reusable project-level dataset. Expand a row to see
a definition card for each scope that exists:

- **Default dataset**: the parameters and rows defined on the Test.
- **Plan overrides**: a different dataset for that Test, defined in a Test
  Plan.
- **Execution overrides**: a different dataset for that Test, defined in a
  Test Execution.
- **Used by**: a compact table of every synced Plan and Execution that
  contains the Test. Click a container key to open that Plan or Execution.

A usage marked **Inherited default** uses the Test's default dataset. A usage
marked **Override** has its own definition for that container. **Override
only** means the Test has no default, but a Plan or Execution defines one.

Plan exports do not include all parameter type metadata. When the type is
missing, XTM shows it as unknown.

Long summaries wrap inside the definition cards. When the view is narrow, the
**Used by** table scrolls sideways so you can still reach its Type, Container,
Summary and Dataset columns.

## Profile and project boundaries

The selected profile sets what the catalog includes: Tests owned by the
profile's Jira project, plus Tests from another project that one of the
profile's Plans or Executions uses. Those referenced Tests are marked
**External**.

An external Test may not be in the local Browse cache. Its dataset preview and
Jira action still work, but XTM hides **Open Test**, because it would lead to
an empty local view.

## Find and open related work

Search by Test key, summary, Plan key or Execution key. **Dataset scope**
narrows the list to defaults, Plan overrides or Execution overrides. **Test
ownership** switches between project-owned and external Tests.

From an expanded row:

- **Open Test** opens that Test in Browse when XTM has it locally.
- A Plan link opens that Plan in Containers.
- An Execution link opens that Execution in Containers, even if Containers
  was last showing Plans or Bugs.
- **Open _key_ in Jira** opens the issue on the configured Jira site.

The catalog is read-only. Edit a default dataset from the Test detail panel.
You cannot edit Plan or Execution overrides from this view.

Results are not recorded here either. A dataset test's row in a Test Execution
expands into one result per iteration, in
[Containers](09-containers.md#a-test-with-a-dataset).

The editor supports Text parameters and ad-hoc List parameters. List options
are stored with the Test dataset. A parameter bound to an existing predefined
Jira list keeps that binding unless you confirm converting it to an ad-hoc
list. XTM does not create, edit or delete project lists. CSV import matches
columns to existing parameters by name, so those keep their list metadata.
Columns with no match become Text.

## Troubleshooting

- **The Datasets view is missing.** The active backend does not report dataset
  support.
- **The view says it has not been scanned.** Click **Scan datasets** for the
  selected profile.
- **A recent Jira change is missing.** Run another dataset scan. The timestamp
  shows when the last complete snapshot was taken. It does not track Jira
  live.
- **A scan fails.** Read the error above the catalog. The rows you see are the
  last successful snapshot, and you can keep using them while you fix the
  connection or permission problem.
