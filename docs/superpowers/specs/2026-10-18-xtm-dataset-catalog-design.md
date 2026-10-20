# XTM dataset catalog

Issue: [#300](https://github.com/veenone/task-activity-manager/issues/300)

XTM exposes datasets only while a Test or Test Execution is already open.
Those reads are lazy, so the cache cannot answer which datasets exist across a
profile. This design adds a complete, explicitly synchronized inventory for
the selected profile.

The catalog follows Xray's ownership model. A dataset is defined by a Test,
with optional Test Plan + Test or Test Execution + Test overrides. It is not a
project-level reusable object. Project and global parameter lists remain
administrator-owned settings and are not catalog entries.

This extends the parameterized-test work in #267 and #277. It does not broaden
#277's editor into context-level writes.

## Goals

- Inventory every dataset definition relevant to the selected profile.
- Show a Test dataset once and link every Plan and Execution that consumes it.
- Distinguish inherited use from a real context override.
- Include external-project Tests referenced by the profile's Plans or
  Executions.
- Preserve the last complete snapshot when a refresh fails or is cancelled.
- Navigate from an inventory entry to the exact Test, Plan, Execution or Jira
  issue.

## Non-goals

- Editing Test Plan or Test Execution overrides from the catalog.
- Managing project or global parameter lists.
- Treating inherited contexts as separate dataset definitions.
- Adding datasets to Kiwi TCMS.
- Folding discovery into every full profile sync.
- Inventorying Test Sets, which are not an Xray dataset override scope.

## Product model

The primary object in the view is a Test group, not a standalone dataset. A
group exists when the Test has at least one definition at any supported scope:

- Test default;
- Test Plan + Test override; or
- Test Execution + Test override.

The Test default may be absent. This matters for a Test that is parameterized
only inside a Plan or Execution.

Every Plan and Execution membership is a usage. The catalog also records the
Plans linked to each Execution, because that link is part of Xray's resolution
order. It stores and renders the relationships without copying an inherited
table into another dataset entry.

The active profile defines the boundary:

- all Tests owned by the profile's Jira project are candidates;
- all Plans and Executions owned by that project are contexts; and
- Tests from other projects join the candidate set when one of those contexts
  references them.

External Tests carry their owning project and an **External project** badge.

## Datasets view

`datasets` joins the shared `View` registry, so the tab bar and optional
navigation rail cannot drift. It is shown only when the active backend reports
dataset support.

The view header shows:

- the number of Test groups and dataset definitions;
- the last successful dataset sync time;
- a **Sync datasets** action;
- search by Test key, Test summary, Plan key or Execution key; and
- filters for definition scope and project ownership.

The primary table has one row per Test group. A row shows the Test key and
summary, owning project, parameter and row counts for its default definition
when present, linked Plan and Execution counts, and the number of overrides.

Expanding a row shows:

- the default definition, when present;
- Plans and Executions using it through Xray's resolution order;
- separate child entries for actual Plan and Execution overrides;
- a read-only parameter-and-row preview for each definition; and
- **Open Test**, **Open Container**, and **Open in Jira** actions.

Test-level edits continue through the existing Test detail editor. The catalog
never offers context-level editing. A local pending edit is not merged into the
last Jira snapshot; the catalog may mark that local changes are pending, but
the displayed inventory remains explicitly identified by its successful sync
time.

### View states

- **Never synced:** explain that complete discovery requires **Sync datasets**.
- **Syncing:** retain the previous snapshot, show completed and total probes,
  and disable another catalog sync.
- **Empty:** after a successful scan, say that no datasets were found for the
  profile.
- **Failed or cancelled:** retain the previous snapshot and show why the new
  snapshot was not published.
- **Successful:** show the new snapshot and its completion time.

An unsupported or missing dataset API is not reported as an empty inventory.

## Discovery

Dataset sync is a dedicated action because complete context discovery costs
one remote read for each Test and each Plan/Execution membership. Opening the
view never starts that work implicitly, and a normal profile sync does not pay
the cost.

The service performs three stages.

### 1. Enumerate candidates

Read current Jira state. The lazy dataset cache is not a candidate source:

1. List all Tests owned by the profile project.
2. List all Test Plans and Test Executions owned by that project, with their
   Test memberships and each Execution's linked Plans.
3. Fetch basic identity for referenced Tests missing from the project-owned
   set.
4. Form the union of owned and referenced Tests and de-duplicate memberships.

The candidate list and relationship list form the boundary for this scan. Jira
can change while a long scan runs, so "complete" means complete for that
enumerated boundary, with every required probe succeeding.

### 2. Probe definitions

For each candidate Test, read the Test-level JSON dataset model. For every
Plan or Execution membership, read what that context itself defines:

- Test default: the existing JSON `TEST` model;
- Test Plan override: the documented dataset export with `testIssueKey`,
  `contextIssueKey`, and `resolved=false`;
- Test Execution override: the existing Test Run JSON `TEST_RUN` model, which
  exposes `entityDataset` separately from inherited `resolvedDataset`.

`resolved=false` is mandatory for Plan reads. A resolved export can return the
Test's inherited table and would turn every usage into a false override.

The Plan export is CSV and therefore cannot preserve list-parameter type and
allowed-value metadata. The catalog renders the parameter names and values it
can prove and labels unavailable type information as unknown. It never guesses
that a parameter is text.

The current adapter has no Test Plan dataset path. Before the catalog service
is implemented, a fixture-backed contract test and a live-instance check must
confirm the `resolved=false` response for both an override and an inherited
Plan membership. The checked behavior then moves into
`agents/project/domain-context.md` through the repository's self-improvement
protocol if it adds or corrects a platform fact.

Negative answers are retained during the scan so completion can be proven,
but only non-empty definitions become catalog entries. Requests run with
bounded concurrency, progress, cancellation, and the existing profile-switch
lock. The progress total is the number of Test probes plus relationship
probes.

### 3. Publish atomically

No catalog row changes while network work is incomplete. The completed scan
is published as one new generation in a SQLite transaction. Only after every
row is written does the catalog state point to that generation. Older
generations are removed after the switch.

Any fetch, validation, cancellation, or transaction error leaves the active
generation unchanged. A profile switch cancels the service and cannot publish
under the newly selected profile.

## Storage

The existing `test_dataset` table remains the editor's cache. It contains lazy
negative reads and locally edited defaults, so using it as the catalog would
make an incomplete cache look complete and could overwrite pending work.

The catalog uses a separate read model:

### `dataset_catalog_state`

- `profile_id` primary key;
- `active_generation`;
- `last_synced_at`;
- owned-Test, external-Test, relationship, Test-group and definition counts.

### `dataset_catalog_test`

- `profile_id`, `generation`, `test_key` primary key;
- Test summary and owning project;
- external-project flag.

Only Tests with at least one definition need to survive publication, but the
service counts all probes before filtering them.

### `dataset_catalog_definition`

- `profile_id`, `generation`, `test_key`, `scope_kind`, `context_key` primary
  key;
- context summary where applicable;
- parameters and rows as validated JSON;
- metadata-completeness marker for Plan CSV reads.

`scope_kind` is one of the dedicated semantic constants for Test, Plan, and
Execution. A Test definition uses an empty `context_key`.

### `dataset_catalog_usage`

- `profile_id`, `generation`, `test_key`, `container_kind`, `container_key`
  primary key;
- container summary.

Usages include inherited contexts and override contexts. A definition at the
same `(test_key, container_key)` distinguishes an override.

### `dataset_catalog_exec_plan`

- `profile_id`, `generation`, `exec_key`, `plan_key` primary key.

This table preserves the Plan links returned for each Execution. The UI can
therefore show the relevant Plan and Execution links without inferring a
relationship from shared Test membership. When an Execution links several
Plans that define different overrides for the same Test, the catalog lists
the definitions and links without claiming which one Xray selected. The live
transport check must settle that resolution rule before XTM labels one as the
effective source.

All tables are profile-keyed, join the profile purge list, and receive a store
migration. Repository queries read only the active generation and page at the
Test-group level.

## Application and backend boundaries

The application binding exposes:

```text
SyncDatasetCatalog(profileID)
ListDatasetCatalog(profileID, query) -> paged Test groups + sync state
GetDatasetCatalogDetail(profileID, testKey) -> definitions + usages
```

Trust-boundary validation rejects empty profile and Test keys, unknown scope
or container kinds, malformed JSON, invalid row widths, and definitions beyond
the server-reported limits.

The backend gains the smallest scope-aware read needed for a Plan context. The
Xray adapter implements it through the unresolved export. Kiwi returns the
existing unsupported error and keeps dataset capability off. Test and
Execution reads continue through their richer existing paths.

The repository owns generation publication and catalog queries. The sync
service owns remote enumeration, bounded scheduling, cancellation, progress,
and conversion into the repository's publish input.

## Navigation

**Open Test** selects the Test in Browse. **Open Container** switches to the
Containers view, selects `testplan` or `testexec`, and selects the exact
container key. A small navigation intent in `NavContext` carries that target;
`ContainersView` consumes it into its existing profile-scoped persisted state.
The new context field and call flow are documented with the implementation.

**Open in Jira** reuses `BrowserOpenURL` and the existing Jira URL builder.
Keys are encoded and the configured Jira base URL remains the only allowed
origin.

An external Test that cannot be opened safely from the local Browse cache
still has a catalog preview and Jira link. The UI does not offer a broken
internal action.

## Errors and recovery

- A dataset absence is data and does not fail the scan.
- A missing dataset route is an unsupported-instance error, not absence.
- Authentication, authorization, malformed responses, invalid dataset shapes,
  and failed individual probes abort publication.
- Cancellation is reported separately from failure.
- The service does not silently retry an unexplained failure. The user can run
  **Sync datasets** again after the cause is visible.
- A storage failure rolls back the generation transaction.
- Pending local Test dataset edits are untouched because the catalog never
  writes `test_dataset` or the pending journal.

## Accessibility and interaction

The expandable Test rows use real buttons with `aria-expanded` and a stable
controlled region. Search and filters have persistent labels. Scope and
ownership are conveyed in text, not color alone. Sync progress is announced
through the existing polite live region, while errors receive the established
alert treatment. Keyboard users can expand a Test and reach every navigation
action without entering the read-only grid cells.

## Testing

Every behavior change starts with a failing test, following P2.

### Jira and adapter

- Test default, Plan override, Plan inheritance, Execution override and
  Execution inheritance fixtures.
- The Plan request includes `resolved=false`.
- A missing dataset and a missing API route remain distinguishable.
- CSV metadata loss is explicit and never reconstructed as text metadata.

### Repository

- Publish and query one Test with many inherited usages.
- Publish an override-only Test.
- De-duplicate repeated memberships and external Tests.
- Scope and ownership filtering plus paging at the Test-group boundary.
- A failed publish leaves the previous active generation readable.
- Profile purge removes state, Tests, definitions, usages and Execution-to-Plan
  links.
- Catalog publication never changes `test_dataset` or pending changes.

### Sync service and bindings

- Enumerate project Tests, externally referenced Tests and Execution-to-Plan
  links.
- Probe every Test and every Plan/Execution relationship once.
- Abort on one failed or malformed probe without publishing.
- Cancellation and profile changes cannot publish.
- Progress totals and completion counts describe the actual probe set.
- Binding inputs reject invalid profile, Test, scope and filter values.

### Frontend

- The capability-gated view appears in both navigation surfaces.
- Never-synced, syncing, empty, failed, cancelled and successful states render
  distinct user-visible outcomes.
- One Test row lists many containers without inherited duplicates.
- Override-only and external Tests are labeled correctly.
- Search and filters send and render real values.
- Expansion exposes the proven dataset preview.
- Test and container actions select the exact internal target.
- Jira navigation is offered when internal navigation is unavailable.
- Keyboard expansion, focus order and accessible names pass the a11y ratchet.

## Documentation

Update the XTM user guide with the profile boundary, explicit sync, inheritance
model, external Tests, last-successful-snapshot behavior, filters and links.
Update the developer call-flow documentation for discovery, generation
publication and navigation intent. Record only newly verified Xray behavior in
the domain playbook.

## Implementation map

The shipped implementation keeps the App binding thin and puts discovery and
publication in their domain packages, following the application boundary in
`AGENTS.project.md` and trust-boundary rule I1.

### Scan and publication

```text
DatasetsView
  -> useDatasetCatalogState.scan
  -> SyncDatasetCatalog
  -> App.SyncDatasetCatalog
  -> datasetcatalog.Service.Sync
  -> testrepo.Repository.PublishDatasetCatalog
  -> active catalog generation changes in one SQLite transaction
```

`DatasetsView` starts the scan only after the user clicks **Scan datasets**.
`App.SyncDatasetCatalog` resolves the selected profile and backend, forwards
progress through `emitDatasetCatalogProgress`, and delegates discovery to
`datasetcatalog.Service.Sync`. The service enumerates Test and container
relationships, runs the Test, Plan and Execution probes, and calls
`Repository.PublishDatasetCatalog` only after all probes succeed. Publication
writes a new generation and switches `dataset_catalog_state.active_generation`
in the same transaction. An error or cancellation leaves the previous
generation active, satisfying I2's recovery requirement without touching the
dataset editor cache or pending journal.

### Catalog reads

```text
DatasetsView
  -> useDatasetCatalog
  -> ListDatasetCatalog
  -> App.ListDatasetCatalog
  -> testrepo.Repository.ListDatasetCatalog

DatasetCatalogDetail
  -> useDatasetCatalogDetail
  -> GetDatasetCatalogDetail
  -> App.GetDatasetCatalogDetail
  -> testrepo.Repository.GetDatasetCatalogDetail
```

The query keys include the profile and complete filter or Test identity. List
queries retain the previous page while a new page or scan is loading. Detail
queries remain disabled until a row is expanded. Both App methods validate
binding input before the repository reads the active generation.

### Exact navigation

```text
DatasetCatalogDetail
  -> DatasetsView callbacks
  -> MainView
  -> NavContext.openContainer({ kind, key })
  -> ContainersView / useContainerState
  -> consumeContainerTarget()
```

Opening a Test sets the exact Browse selection. Opening a Plan or Execution
stores a one-shot `ContainerNavigationTarget` and switches to Containers.
`useContainerState` leaves Bugs mode, applies the target kind, waits until the
matching list has replaced any retained placeholder data, selects the exact
key, and consumes the target. External Tests whose local destination is not
safe expose the Jira action only.

### Plan transport evidence

The Plan reader is based on Xray's checked-in WADL contract and fixture-backed
tests for `resolved=false`. No connected Xray instance was available during
this implementation, so the live override and inheritance response check
remains a pre-merge validation item. This change therefore adds no new live
platform claim to `agents/project/domain-context.md`.

## Delivery sequence

1. Verify and pin the Plan override transport.
2. Add the catalog schema, migration, purge coverage and atomic repository
   publication.
3. Add backend enumeration and the cancellable sync service.
4. Add read bindings and query hooks.
5. Add the Datasets view and its states, filters and detail expansion.
6. Add exact Test/container navigation.
7. Update user and developer documentation, then run the full repository
   gates.

The work lands through issue #300. Its PR body quotes the issue's acceptance
lines. The existing uncommitted dataset editor work is not folded into this
feature's commits.

## Risks

- Probe count grows with both Tests and their container relationships.
  Bounded concurrency and explicit sync keep the cost visible and controlled.
- The public Plan export loses list metadata. The UI exposes the limitation
  instead of inventing data.
- Remote Jira state can change during a scan. The published snapshot is
  complete for its initial enumerated boundary, not a Jira-wide transaction.
- External Tests may not have a safe local Browse destination. Jira remains
  the guaranteed link.
- Existing worktree changes overlap dataset UI files. Implementation must
  preserve them and isolate catalog commits by logical slice.
