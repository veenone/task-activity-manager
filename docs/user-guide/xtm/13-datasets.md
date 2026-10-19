# Datasets

The Datasets view is an inventory of parameterized Tests in the selected
profile. It shows each Test once, then groups the Test's default dataset, its
Plan or Execution overrides, and the containers that use it.

The view is available for Xray profiles. It does not appear for backends that
do not support datasets.

## Build the catalog

Open **View → Datasets**, then click **Scan datasets**. A normal profile sync
does not build this catalog. The scan is separate because XTM must inspect the
Tests, Plans and Executions in the project one at a time.

The scan reads Jira and updates XTM's local catalog. It does not edit a Test or
send pending changes to Jira. You can keep using the previous catalog while a
new scan runs.

If a scan fails, XTM shows the error and keeps the last successful catalog. A
failed or cancelled scan never replaces a complete snapshot with partial
results. If the view says the project has not been scanned, no successful
snapshot exists for that profile yet.

## Read a Test group

Each row represents one Test, not a reusable project-level dataset. Expand a
row to see definition cards for each available scope:

- **Default dataset**: parameters and rows defined on the Test.
- **Plan overrides**: a different dataset defined for that Test in a Test
  Plan.
- **Execution overrides**: a different dataset defined for that Test in a
  Test Execution.
- **Used by**: a compact table of every synced Plan and Execution that
  contains the Test. Its container keys open the corresponding Plan or
  Execution.

A usage marked **Inherited default** uses the Test's default dataset. A usage
marked **Override** has its own context-specific definition. **Override only**
means the Test has no default but does have a Plan or Execution definition.

Plan exports do not include all parameter type metadata. XTM labels that
metadata as unknown instead of guessing a type.

Long summaries wrap inside the definition cards. The **Used by** table scrolls
horizontally when the view is narrow, so its Type, Container, Summary and
Dataset columns remain reachable.

## Profile and project boundaries

The selected profile sets the catalog boundary. XTM includes Tests owned by
that profile's Jira project, plus Tests from another project when one of the
profile's Plans or Executions uses them. Those referenced Tests are marked
**External**.

An external Test may not exist in the local Browse cache. In that case, its
dataset preview and Jira action remain available, but XTM does not show an
**Open Test** action that would lead to an empty local view.

## Find and open related work

Use search for a Test key, summary, Plan key or Execution key. **Dataset
scope** narrows the list to defaults, Plan overrides or Execution overrides.
**Test ownership** switches between project-owned and external Tests.

From an expanded row:

- **Open Test** opens the exact Test in Browse when XTM has a safe local
  destination.
- A Plan link opens that Plan in Containers.
- An Execution link opens that Execution in Containers, even if Containers
  previously showed Plans or Bugs.
- **Open _key_ in Jira** opens the issue in the configured Jira site.

The catalog is read-only. Edit a default dataset from the Test detail panel;
Plan and Execution overrides are not editable from this view.

The editor supports Text parameters and ad-hoc List parameters. List options
are stored with the Test dataset. Existing predefined Jira list bindings are
preserved unless you confirm conversion to an ad-hoc list; XTM does not create,
edit or delete project lists. CSV import matches existing columns by name to
retain their list metadata, and treats unmatched columns as Text.

## Troubleshooting

- **The Datasets view is missing.** The active backend does not report dataset
  support.
- **The view says it has not been scanned.** Click **Scan datasets** for the
  selected profile.
- **A recent Jira change is missing.** Run another dataset scan. The timestamp
  describes the last complete snapshot, not live Jira state.
- **A scan fails.** Read the error shown above the catalog. Existing rows are
  the last successful snapshot and remain safe to use while you fix the
  connection or permission problem.
