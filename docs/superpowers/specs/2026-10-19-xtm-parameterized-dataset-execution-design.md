# XTM parameterized dataset authoring and execution

Issue: [#301](https://github.com/veenone/task-activity-manager/issues/301)

XTM can store an Xray dataset and inventory datasets across a profile, but the
authoring and execution flows stop short of Xray's model. Dataset controls are
cramped, list parameters cannot be created, step authors must type references
from memory, and Test Executions flatten a parameterized Test into one result.

This design completes that path without guessing an undocumented Xray write
contract. It follows Xray's published behavior: a dataset belongs to a Test or
to that Test in a Plan or Execution context; rows and combinatorial values
produce iterations inside one Test Run; step results determine an iteration
result; and iteration results determine the Test Run result.

The product references are Xray's
[Parameterized Tests](https://docs.getxray.app/space/XRAY/159809712/Parameterized+Tests)
and [Execute Tests](https://docs.getxray.app/space/XRAY/160170368/Execute+Tests)
documentation.

## Goals

- Make the catalog detail and Test dataset editor look and behave like the
  rest of XTM.
- Present dataset usage as a compact table with exact container links.
- Let an author create Text and ad-hoc List parameters without losing Xray
  metadata on existing parameters.
- Insert `${PARAMETER}` at the caret in every text-based manual-step field.
- Show the dataset iterations nested under one Test Run, with resolved values
  and substituted steps.
- Import JUnit results into the correct dataset iterations with a preview for
  any mapping the report cannot prove.
- Exercise the whole flow offline with DEMO2-1 in DEMO2-STE-1.
- Keep live iteration editing read-only until its Xray transport is captured
  and tested.

## Non-goals

- Creating or administering global parameter lists.
- Rebuilding project-list administration inside XTM.
- Treating an iteration as a separate Test Run.
- Guessing an Xray endpoint or payload from UI behavior alone.
- Adding parameterized execution support to a backend that does not advertise
  datasets.
- Editing Plan or Execution dataset overrides from the catalog.

## Dataset catalog layout

The catalog remains a table with one master row per Test. Expanding a row
reveals a single detail surface inside the table rather than three unrelated
columns on the page.

The detail surface contains:

1. A scope summary with separate Default, Plan override and Execution override
   cards. Empty scopes are omitted except for a missing default, which is
   stated plainly.
2. A compact **Used by** table.
3. The existing Test and Jira actions in one footer row.

Definition cards use the same surfaces, border, radius, spacing and muted text
tokens as other XTM detail cards. Parameter names are static badges, not
buttons. Combinatorial parameters include a text label and symbol, so color is
never the only distinction. Unknown Plan metadata is a short note attached to
that definition instead of a page-wide warning.

The usage table has these columns:

| Column | Content |
|---|---|
| Type | Test Plan or Test Execution |
| Container | Linked Jira key that opens the exact XTM container |
| Summary | Container summary, allowed to take the remaining width |
| Dataset | Override or Inherited default |

The table sits in an overflow container. It keeps semantic table markup and a
caption available to assistive technology. Compact labels remain on one line;
unpredictably long summaries wrap instead of clipping actions. The expanded
row owns a stable region referenced by the master row's `aria-controls`, and
the expand button reports `aria-expanded`.

Loading, empty and error states occupy the same padded detail surface, so the
master table does not jump between unrelated layouts. Opening a detail never
clears the selected Test or the last successful catalog page.

## Dataset editor

The editor becomes one bordered card with three parts: a toolbar, the value
grid, and a footer. It reuses XTM's existing button, input, select, Menu, Modal
and focus styles. No new component library or color token is introduced.

The toolbar contains **Add parameter**, **Import CSV**, and **Export CSV**.
Import remains reversible until Save. Export is available without entering an
edit sub-flow.

The value grid reserves its first column for row numbers and its last for row
actions. Parameter headers show the name, a Text or List label, and a
Combinatorial label when applicable. Reorder, edit and remove actions live in
the shared Menu component instead of appearing as raw `<`, `>`, and Remove
links. Row movement and removal use compact labeled controls with accessible
names.

**Add parameter** and **Edit parameter** open the shared Modal with:

- Name;
- Type: Text or List;
- Combinatorial;
- list source and options when Type is List.

For this issue, a newly created List uses the ad-hoc source. Options are an
ordered collection of non-empty, unique strings. Existing project/predefined
List parameters show their source and options, but the binding itself is
read-only until XTM has a verified API for enumerating and changing project
lists. A user may keep that binding, or explicitly convert the parameter to
Text or a new ad-hoc List. Conversion requires confirmation because it changes
the Xray list identity.

Text cells use an input. List cells use a select whose choices come from the
parameter options. Changing list options does not silently discard a cell
value. A removed option that is still in use is blocked with the affected row
numbers until those cells are changed.

The footer groups **Add row** separately from **Save** and **Cancel**. Save is
the only action that journals the dataset. Parameter metadata remains part of
the whole-table journal value, matching the existing replacement write.

### Parameter metadata

The existing `DatasetParameter` fields stay authoritative:

- `type` is `TEXT` or `LIST`;
- `options` contains ordered List values;
- `listScope` identifies ad-hoc or predefined ownership;
- `listId` and the parameter `id` retain Xray identity.

A new Text parameter is written as `TEXT` with no options or list binding. A
new ad-hoc List parameter is written as `LIST`, with its options and Xray's
ad-hoc scope marker. The adapter test must pin the exact new-list payload
before that write is enabled for a live profile. Existing list ids and scopes
are always round-tripped.

CSV has no place for type or list identity. Import matches parameters by name
and retains metadata for matches, as it does today. An imported column with no
match becomes Text. Export remains Xray's name/value format and does not claim
to preserve list configuration.

## Step macro helper

Manual step Action, Data and Expected fields gain the same dataset reference
helper. The helper reads parameter names from the Test's default dataset,
because that is the dataset available while the Test definition is authored.
It never inserts values.

There are two entry paths:

- A `${` toolbar button opens the parameter menu.
- Typing `${` while editing opens the same filtered menu.

Choosing a parameter inserts `${NAME}` at the current selection, replaces a
partially typed macro when appropriate, returns focus to the field, and places
the caret after the closing brace. Arrow keys move through suggestions, Enter
selects, and Escape closes without changing text. The button has a persistent
accessible name; the suggestion list uses the established combobox/listbox
pattern.

`MarkdownField` gains a small insertion contract rather than dataset logic.
It reports and restores the current selection and accepts inserted text. A
dataset-aware wrapper owns trigger detection, suggestions and reference
styling. This keeps description fields and non-Xray backends unchanged.

Rendered step text distinguishes known and undefined `${NAME}` references.
Undefined references carry text and an accessible explanation, not color
alone. XTM does not remove them: Xray allows a reference to be written before
its parameter exists.

Renaming a parameter keeps the existing rewrite confirmation. The macro menu
reflects the edited dataset only after Save, so a cancelled dataset edit does
not leak temporary names into steps.

## Parameterized Test Runs

One Test Execution member remains one Test Run. When its effective dataset
produces iterations, the member row gains an expand control and a concise
progress summary. Expanding it reveals the iterations underneath that row.

Each iteration header contains:

- a stable sequence label;
- its status;
- parameter names and resolved values; and
- an expand control for its steps.

Expanded steps show the Action, Data and Expected text after parameter
substitution, plus the recorded step status and result fields the transport
can prove. The original Test definition remains unchanged.

The nearest dataset wins in this order:

```text
Test Execution + Test > Test Plan + Test > Test default
```

The read model records the scope and context used to produce the iterations.
The UI shows that source beside the iteration summary. It never infers an
effective Plan override when several linked Plans make the answer ambiguous;
the server's Test Run detail must identify the resolved iteration data.

### Combination expansion

Xray expands combinatorial parameters at execution time. The live adapter
reads Xray's generated iterations rather than recreating them. Demo mode and
pure unit tests use a deterministic expansion helper:

1. Non-combinatorial columns form the seed rows.
2. Each combinatorial column contributes its distinct values in stored order.
3. The cartesian product of those value sets is applied to each seed row.
4. With no seed columns, one empty seed row is used.
5. Expansion stops with a validation error above Xray's 1,000-iteration limit.

This helper is not used to invent live execution ids or results.

### Result ownership and roll-up

Xray owns live result calculation. A live read carries the Test Run id,
iteration ids, step-result ids and the statuses returned by Xray. When a
verified write changes a step result, Xray's response or the following read
supplies the recalculated iteration and Test Run statuses. XTM does not impose
a guessed precedence on custom statuses.

Demo mode implements the native status rules needed for its deterministic
fixture. It is clearly adapter-owned behavior and does not become a fallback
for an incomplete live response.

## JUnit import into dataset iterations

The existing JUnit importer matches `<testcase name>` to a member Test summary
and queues one overall Test Run status. That remains the behavior for a Test
without iterations. A parameterized member instead targets an iteration.

JUnit has no standard field that means "Xray dataset row." XTM accepts
testcase-level properties whose names exactly match the effective dataset
parameters:

```xml
<testcase name="Login accepts valid credentials">
  <properties>
    <property name="USERNAME" value="alice"/>
    <property name="REGION" value="eu-west"/>
    <property name="PRIORITY" value="high"/>
  </properties>
</testcase>
```

Property names use Xray's case-sensitive parameter names after trimming XML
whitespace. All parameters in the resolved iteration must be present and have
the same values. Unrelated properties are ignored. A partial signature is not
enough to choose an iteration, even when it happens to match only one row in
the current dataset; a later dataset edit could make that inference wrong.

The analyzer follows this order:

1. Match the testcase to exactly one Execution member using the existing
   normalized-summary rule.
2. If that Test has no iterations, propose the existing overall result.
3. If it has iterations and the testcase carries a complete parameter
   signature, match that signature to exactly one iteration.
4. Otherwise mark the testcase **Needs mapping** and offer the Test's
   iterations in the preview.

Report order is never used as iteration identity. Duplicate iteration values,
two testcases targeting the same iteration, a signature that matches nothing,
and a signature that matches several iterations remain unresolved until the
user changes the mapping or excludes the testcase.

Skipped JUnit cases keep their current result unchanged, matching today's
import behavior. Passed and failed cases map to PASS and FAIL. Failure and
error text may be retained as source detail when the verified iteration write
contract supports it; it is not silently discarded into an unrelated Test Run
comment.

### Preview and application

The import preview groups rows by Test. A parameterized group shows the
testcase, result, parameter signature, target iteration and current iteration
status. A row that needs mapping has an accessible iteration select whose
options show the full parameter signature. The UI exposes full values without
depending on truncated chip text or hover.

**Apply** remains disabled while any included row is ambiguous, duplicated or
unmapped. The user may exclude such a row explicitly. The backend revalidates
Execution membership, effective dataset identity and iteration identity when
Apply runs; a stale preview cannot write into a changed dataset.

`JUnitMatch` becomes a discriminated target: either the existing Test Run
target or a stable iteration target. The parser retains testcase-level
properties in a validated named type. `ApplyJUnitImport` queues iteration
results through the same journal and capability boundary as manual iteration
editing. It never falls back to setting the overall Test Run when a requested
iteration write is unsupported.

The **New exec from JUnit XML** flow applies the same rules for an existing
parameterized Test, using the default dataset that will resolve into the new
Execution. A newly created Test has no dataset and keeps the existing overall
result behavior. If a context override later changes the new Execution's
iterations, Apply detects the stale mapping instead of carrying results across
by row number.

## Transport capability boundary

The current Xray adapter can resolve a Test Run and set its overall status,
but it has no fixture-backed iteration detail or iteration-step update
contract. The public documentation describes the screen behavior, not those
REST payloads.

The backend therefore advertises two separate capabilities:

- parameterized Test Run detail;
- parameterized Test Run result writes.

Demo supports both. Xray supports detail or writes only after each endpoint,
identifier and payload has a checked fixture and an adapter contract test. A
live profile without the read capability keeps the current Test Run row and
states that iteration details are unavailable. A profile with reads but no
writes shows nested results read-only and disables status controls with an
explanation.

No pending change is created when writes are unsupported. Once the write
contract is verified, iteration-step changes join the ordinary local journal
and Commit flow. Their entity key contains stable server ids, not row indexes.
Conflict detection compares the remote iteration/step result version or value
captured by the verified read contract.

## Data model and boundaries

Named backend DTOs carry parameterized runs across the Wails boundary:

```text
ParameterizedRun
  run identity, Test identity, dataset source, aggregate status
  Iterations[]
    stable identity, rank, status, Parameters[]
    Steps[]
      stable identity, definition identity, resolved text, result status
```

Arrays are non-null and ordered. Trust-boundary validation rejects blank ids,
duplicate ranks or ids, unknown parameter names, invalid status values,
iterations beyond the server limit, and malformed step references.

The local cache adds profile-scoped iteration and iteration-step tables keyed
by stable server identity. Both join `profileScopedTables`; the schema version
and migration are updated together. Repository replacement happens in one
transaction per Test Execution sync so a partial response cannot leave half
an iteration tree.

The App binding validates profile, Execution and Test keys, resolves the
backend, then delegates. Combination expansion, substitution, validation and
roll-up logic live under `internal/`, never in `app.go`.

Dataset and step queries use profile and Test identity in their keys.
Parameterized-run queries also include the Execution key. Closing or switching
a Test cancels stale reads, and a late response cannot populate another Test's
panel.

## Demo2 behavior

DEMO2-1 keeps its USERNAME, REGION and combinatorial PRIORITY parameters. Its
manual steps include `${USERNAME}`, `${REGION}` and `${PRIORITY}` across Action,
Data and Expected so macro rendering is visible.

DEMO2-STE-1 returns one Test Run for DEMO2-1 with deterministic generated
iterations. The fixture includes at least one Passed, one Failed and one Todo
iteration, with step results that explain each aggregate. Repeated reads return
the same ids and ordering. Demo writes update the local simulated response in
the same way other demo writes do; refresh restores the deterministic source
fixture.

No demo row is seeded into a real profile's database.

## Errors and recovery

- Invalid dataset names, duplicate rows, duplicate list options and values
  outside server limits are rejected before journaling.
- A failed dataset save leaves the editor and its unsaved values open.
- Removing a referenced list option reports the affected rows.
- A failed macro insertion leaves the field value and selection intact.
- A malformed iteration response leaves the previous complete run tree in the
  cache and reports the error.
- Unsupported live reads or writes are explicit capability states, not empty
  data or successful no-ops.
- A failed iteration commit keeps its pending changes for retry or discard.

## Testing

Every feature or fix begins with a failing test and its red result is recorded
for the proven-red gate.

Frontend tests cover the catalog detail hierarchy, semantic usage table,
responsive overflow, editor Modal, Text/List cells, list-option validation,
parameter menus, keyboard macro insertion, caret restoration, known and
undefined references, nested iterations, status progress, and unsupported
transport states. JUnit tests cover automatic property matching, manual
mapping, exclusion, duplicate targets, full-value disclosure and disabled
Apply for unresolved rows. Assertions use visible values and interactions
rather than element counts.

Go tests cover dataset metadata round trips, new ad-hoc List payloads,
combination expansion, substitution, DTO validation, atomic repository
replacement, purge, deterministic Demo2 runs, capability reporting, malformed
transport responses and pending-change conflicts. JUnit parser and repository
tests cover complete, partial, unknown, duplicate and ambiguous signatures,
stale dataset revalidation, skipped cases, non-parameterized compatibility and
new-Execution imports. A live Xray transport claim is added only with a
captured fixture and direct contract test.

Manual verification runs `wails dev` with the `demo2` profile at normal and
narrow window widths. The walkthrough creates Text and List parameters,
inserts macros into all three step fields, opens DEMO2-STE-1, expands DEMO2-1,
and checks that resolved steps and result roll-up agree.

## Documentation

The XTM user guide explains Text and List parameters, ad-hoc options, macro
insertion, dataset resolution order, nested iterations and read-only live
results. It also documents the testcase property convention and manual JUnit
mapping preview. Developer documentation records the new component and binding
call flows. Newly verified Xray endpoints belong in
`agents/project/domain-context.md` through the self-improvement protocol.

## Delivery slices

1. Restyle the catalog detail and usage table.
2. Rebuild the dataset editor shell and parameter Modal.
3. Add Text and ad-hoc List authoring with adapter contract tests.
4. Add the step macro insertion contract and dataset-aware helper.
5. Add iteration DTOs, validation, schema and repository reads.
6. Add deterministic Demo2 iterations and the nested Test Execution UI.
7. Extend existing and new-Execution JUnit imports with property matching and
   the manual iteration mapping preview.
8. Add live Xray iteration reads and writes only when their contracts are
   captured; otherwise ship the explicit unsupported states.
9. Update user and developer documentation and run the full repository gates.

Each slice is a separate conventional commit referencing #301. The PR quotes
the issue's acceptance lines and links this specification.

