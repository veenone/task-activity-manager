# XTM labels and components

Three related changes to XTM. They ship as three PRs in the order A, C, B,
because B's inline "create component" calls the CRUD that C adds.

- A. A shared token picker, used for labels on one test and in a new Bulk
  Labels action.
- C. A Components view that creates, edits and deletes Jira project
  components.
- B. Editing components on existing tests, singly and in bulk.

## Current state

Labels are editable today. `TestDetail` has a space-separated text input that
saves through `EditTestField`, and `BulkEditModal` offers `replace`,
`add_label` and `remove_label`, which `applyBulkOperation` in
`internal/testrepo` turns into a full label list queued in the pending-change
journal. On commit, `FieldsForJira` sends `{"fields": {"labels": [...]}}`.
Nobody gets suggestions, so typos create new labels without warning.

Components are read-only. Sync stores each test's components in
`test_case.components`, using the newline-bounded form from
`encodeComponents`, and caches the project's list through
`ReplaceProjectFieldOptions(..., "component", ...)`. `ProjectComponents` in
`internal/jira/projectfields.go` returns names only. `components` is not in
`editableFields` or `FieldsForJira`, and no code calls the Jira component
endpoints for writing.

## A. Token picker and Bulk Labels

### TokenPicker

A new component, `xtm/frontend/src/components/TokenPicker.tsx`, with its rules
in the XTM stylesheet. It lives in XTM rather than `frontend/core` because
core has no stylesheet, and the Class names contract requires every class to
be defined in one. It can move to core when TAM needs it.

Props:

```ts
interface TokenPickerProps {
  value: string[];
  onChange: (next: string[]) => void;
  suggestions: string[];          // candidates shown while typing
  allowCreate: boolean;           // Enter on an unknown value adds it
  onCreate?: (v: string) => Promise<boolean>; // B only; false keeps it out
  validate?: (v: string) => string | null;    // error text, or null if valid
  label: string;                  // accessible name
  placeholder?: string;
}
```

Behavior:

- Chosen values render as chips with a remove button. Backspace in an empty
  input removes the last chip.
- Typing filters `suggestions` case-insensitively, excluding values already
  chosen. Arrow keys move through the list; Enter or a click picks one.
- With `allowCreate`, Enter on text that matches no suggestion adds it, and
  the list shows a "Create "x"" row so the user can see it is new. When
  `onCreate` is set, the value is added only if it resolves true.
- `validate` errors render under the input and block the add.
- It works as an ARIA combobox: `role="combobox"`, `aria-expanded`,
  `aria-activedescendant`, and a `listbox` of `option`s. The eslint a11y
  counters must not rise.

### Label suggestions

Suggestions come from the labels already on the profile's synced tests,
through a new `Repository.ListLabels(profileID) ([]Bucket, error)`, the same
shape as `ListComponents`. It is exposed as `App.ListLabels` and a TanStack
Query hook. Jira has no endpoint for creating a label, so creating one means
putting it on a test; the picker always runs with `allowCreate`.

Label validation: no whitespace (Jira rejects it) and at most 255
characters. The error text lives in the component, per the UI copy contract.

### Where the picker goes

- `TestDetail`: replaces the space-separated label input. Each change saves
  through `saveField("labels")` with the list joined by spaces, so the stored
  and journaled form stays as it is now.
- `BulkEditModal`: the `add_label`, `remove_label` and `replace` value input
  becomes a TokenPicker. The backend operations accept several labels
  separated by spaces, which `applyBulkOperation` already splits.
- New `BulkLabelsModal`, opened by a **Labels** button in the bulk toolbar
  next to Edit. It has an "Add labels" picker and a "Remove labels" picker,
  and a preview line ("12 of 20 selected tests will change") worked out on
  the client from the loaded tests. Apply calls a new
  `BulkEditLabels(testKeys, add, remove []string)` binding, which queues one
  labels edit per changed test and returns the existing `BulkEditResult`.
  Each test is edited on its own, as `BulkEditTests` does, so one failure
  does not block the rest. A label in both lists is an error the modal reports
  before calling the backend.

## C. Components view

### Jira client

New methods in `xtm/internal/jira/components.go`:

| Method | Endpoint |
|---|---|
| `ProjectComponentDetails(ctx, projectKey) ([]Component, error)` | `GET /rest/api/2/project/{key}/components` |
| `CreateComponent(ctx, ComponentInput) (Component, error)` | `POST /rest/api/2/component` |
| `UpdateComponent(ctx, id, ComponentInput) (Component, error)` | `PUT /rest/api/2/component/{id}` |
| `DeleteComponent(ctx, id, moveIssuesTo string) error` | `DELETE /rest/api/2/component/{id}?moveIssuesTo={id}` |
| `ComponentIssueCount(ctx, id) (int, error)` | `GET /rest/api/2/component/{id}/relatedIssueCounts` |
| `SearchUsers(ctx, query) ([]User, error)` | `GET /rest/api/2/user/search?username={q}` |

`Component` carries `ID`, `Name`, `Description`, `LeadName`,
`LeadDisplayName` and `AssigneeType`. `ComponentInput` carries `Project`,
`Name`, `Description`, `LeadUserName` and `AssigneeType`. Jira DC takes
`leadUserName`; the field names are checked against a live instance before
the PR leaves draft, and the existing `NOTE(xtm)` on `ProjectComponents` is
resolved at the same time.

Each method gets a demo branch. The demo keeps components in a
mutex-guarded map on the client, seeded from `demoComponentList`, so creates,
renames and deletes show up during a demo session. The other two demo lists,
`demoComponentNames` and the one in `customfields.go`, are switched to read
from `demoComponentList` so the demo's tests and its component list agree.

The methods join the `backend.Backend` interface. The Xray adapter delegates
to the client. The Kiwi adapter returns `backend.ErrUnsupported` and leaves
a new component-management capability off in `Capabilities`, which the
frontend reads to hide the view.

### Live writes, not the journal

Component CRUD calls Jira straight away. The journal holds pending edits to
issues; a component belongs to the project, and nothing local can preview a
component that Jira has not created. After each successful write the app
re-fetches the project's components and replaces the cached option list.

Renames and deletes also change component names on tests that Jira has
already moved. To keep the local cache honest without rewriting the
journal:

- Rename and delete are refused while any pending change edits the
  `components` field of a test carrying that component. The message tells
  the user to commit or discard those changes first.
- After a rename, the app rewrites the old name to the new one in
  `test_case.components` for the profile, in one transaction.
- After a delete with "move issues to", the app rewrites the name the same
  way. After a plain delete it removes the name.

### UI

`ComponentsView.tsx` is added to the `View` union, the tab bar, and the
native View menu (`menu:view-components`). It follows `ContainersView`: a
table of name, description, lead and test count (the test count comes from
`ListComponents`, local and free), plus New, Edit and Delete.

- New and Edit open one form modal built on the `Modal` primitives: name
  (required, unique within the project, checked case-insensitively on the
  client and again by Jira), description, lead (a single-value search box
  over `SearchUsers`, debounced) and assignee type (project default,
  component lead, project lead, unassigned).
- Delete goes through `useConfirm`. Before it opens, the app fetches
  `ComponentIssueCount`. When the count is above zero, the dialog offers a
  "Move issues to" select of the other components, defaulting to none.
- Errors render in the view's existing error area. A 403 reads "You need
  project admin rights in Jira to change components."

## B. Components on tests

### Storage and journal

`components` joins `editableFields`. The value stored and journaled is the
newline-bounded form `encodeComponents` already writes, because component
names can contain spaces. `encodeComponents` and `decodeComponents` move from
`internal/testrepo` to a small package, `internal/fieldcodec`, so that
`FieldsForJira` in `internal/jira` can decode them without importing
`testrepo`. `FieldsForJira` maps `components` to
`[{"name": "..."}, ...]`, and an empty value to `[]`, which clears the field.

Bulk operations gain `add_component`, `remove_component` and
`replace_components` in `applyBulkOperation`, mirroring the label cases, and
a `BulkEditComponents(testKeys, add, remove []string)` binding mirroring
`BulkEditLabels`.

### UI

- `TestDetail` gets a Components TokenPicker below Labels. Suggestions come
  from the cached project component list (`ListProjectComponents`).
- A **Components** bulk toolbar button opens `BulkComponentsModal`, which has
  the same layout as `BulkLabelsModal`.
- Both pickers set `allowCreate` with an `onCreate` that calls
  `CreateComponent`. Before creating, the picker asks through `useConfirm`
  ("Create component "x" in PROJ?"). If the create fails, the chip is not
  added and the error shows under the picker.

### Commit

Commit already sends whatever `FieldsForJira` returns, so components need no
new commit code. If Jira rejects a component name (deleted elsewhere since
the last sync), the commit fails for that test the way other field errors
do, and the pending change stays for the user to fix or discard.

## Testing

Each part is written test first (P2).

- Go, `internal/jira`: an httptest case per new endpoint, asserting the
  method, path, query and decoded request body, plus a 403 case and a demo
  case that creates, renames and deletes and then lists.
- Go, `internal/testrepo`: `ListLabels` counts; `BulkEditLabels` and
  `BulkEditComponents` add, remove, skip unchanged tests and report failures;
  the rename and delete cache rewrites; the refusal when pending changes
  touch the component.
- Go, `internal/jira`: `FieldsForJira` turns `components` into name objects
  and an empty value into `[]`.
- Vitest: TokenPicker keyboard flow, create row, validation error and
  `onCreate` returning false; `BulkLabelsModal` preview count and
  add/remove conflict; `ComponentsView` create, edit, the delete confirm with
  a move target, and the 403 text. Assertions check calls with values and
  rendered text (C6).

## Docs

`docs/user-guide/USER_GUIDE.md` gains sections on bulk labels, components on
tests, and the Components view, each shipped with its PR (P10).

## Out of scope

- Renaming or deleting a label across the project. Jira has no label entity
  to rename, and doing it means editing every test that carries the label.
- Components on requirements or containers.
- Component CRUD on the Kiwi backend.
