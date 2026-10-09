# XTM components, part B: edit components on tests — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Components on an existing test become editable, one test at a time in the detail panel and across a selection in a Bulk Components dialog, through the pending-change journal like labels.

**Architecture:** The newline-bounded component encoding moves from `internal/testrepo` to a small `internal/fieldcodec` package so `internal/jira` and the Kiwi adapter can decode it. `components` joins the editable test fields; `FieldsForJira` sends Jira `[{"name": …}]`. New testrepo calls set, bulk-edit and read components; the frontend gets a `ComponentsField` in the detail panel and a `BulkComponentsModal`, both on the `TokenPicker` from part A. Creating an unknown component from either picker uses part C's `CreateComponent`.

**Tech Stack:** Go, SQLite, Wails v2, React and TypeScript, TanStack Query, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-09-xtm-labels-components-design.md`, part B (on `main`).

**Issue:** #158. **Branch:** `feat/xtm-test-components`, cut from `feat/xtm-components-admin` (PR #162) because the inline create needs part C. Rebase onto `main` once #162 merges, before opening the PR.

## Deviations from the spec

1. **No `add_component` / `remove_component` / `replace_components` in `applyBulkOperation`.** A dedicated `BulkEditComponents(testKeys, add, remove, replace)` covers all three, the way `BulkEditLabels` covers labels. Bulk Edit's generic field list stays as it is.
2. **A `SetTestComponents(testKey, names)` binding** encodes the value in Go. The frontend never builds the newline form, so there is one encoder.
3. **Kiwi's `parseComponentSet` switches to the newline form.** It was written for a comma convention before any code queued component edits. `EditTestField` writes the queued value into `test_case.components`, which must keep the newline form the rest of the code decodes, so the journal value has to be that form for both backends.
4. **`TokenPicker` gains a `separator` prop.** Labels split pasted and unconfirmed text on whitespace; component names contain spaces ("User Management"), so the component pickers pass `null` and keep text whole.
5. **The pending-changes list shows component edits as `A, B`** instead of the stored `\nA\nB\n`.

## Global Constraints

- Run Go commands from `xtm/`; npm commands from the repo root. Regenerate `wailsjs` with `wails generate module`; commit only content changes.
- Backend logic in `internal/`; `app.go` / `app_components.go` only adapt.
- No `console`, no em dash in UI strings, no hex or px fonts in new CSS, `Modal` from `@agile-suite/core` in any file that sets a modal class, every class defined in a stylesheet, new files under 400 lines.
- Component name rules: not empty after trimming, no newline, at most 255 characters.
- Commits reference #158; no AI attribution.

## Review Focus

1. **A component name with a space** ("User Management") typed, pasted and blurred in either picker stays one component. Task 4 pins it.
2. **A test whose only component is removed** commits `components: []` to Jira (clears the field), not an omitted key. Task 2 pins it.
3. **Kiwi profile:** a components edit commits through `applyComponentDiff` with the right names, and the picker offers no create row (no `supportsComponentAdmin`). Tasks 2 and 6 pin it.
4. **Replace all with an empty list** clears components on every selected test, and the preview says how many change. Tasks 3 and 6 pin it.
5. **Creating a component from the picker when the user declines the confirm, or Jira refuses,** adds no chip and shows the reason. Task 5 pins it.

---

### Task 1: `internal/fieldcodec`

**Files:**
- Create: `xtm/internal/fieldcodec/components.go`, `xtm/internal/fieldcodec/components_test.go`
- Modify: `xtm/internal/testrepo/testrepo.go` (the three helpers delegate)

**Interfaces:**
- Produces: `fieldcodec.EncodeComponents([]string) string`, `fieldcodec.DecodeComponents(string) []string`, `fieldcodec.ComponentFilterPattern(string) string`.

- [ ] **Step 1: Write the failing test**

```go
package fieldcodec

import (
	"reflect"
	"testing"
)

func TestComponentsRoundTrip(t *testing.T) {
	enc := EncodeComponents([]string{" User Management ", "", "API"})
	if enc != "\nUser Management\nAPI\n" {
		t.Fatalf("encode %q", enc)
	}
	if got := DecodeComponents(enc); !reflect.DeepEqual(got, []string{"User Management", "API"}) {
		t.Fatalf("decode %v", got)
	}
	if EncodeComponents(nil) != "" {
		t.Fatal("empty list should encode to empty string")
	}
	if got := DecodeComponents(""); got == nil || len(got) != 0 {
		t.Fatalf("decode empty %#v", got)
	}
	if ComponentFilterPattern("API") != "%\nAPI\n%" {
		t.Fatalf("pattern %q", ComponentFilterPattern("API"))
	}
}
```

- [ ] **Step 2: Run it** — `go test ./internal/fieldcodec/ -count=1` — Expected: build failure, package has no Go files besides the test.

- [ ] **Step 3: Move the helpers**

`components.go`: move `componentSep`, `encodeComponents`, `decodeComponents` and `componentFilterPattern` from `testrepo.go` (with their comments) and export them as `EncodeComponents`, `DecodeComponents`, `ComponentFilterPattern`. Package comment:

```go
// Package fieldcodec holds the storage encodings shared by the local store,
// the Jira client and the backends. Components are stored newline-bounded
// ("\nA\nB\n") so a LIKE '%\nName\n%' filter matches one whole name.
package fieldcodec
```

In `testrepo.go`, replace the moved block with thin delegates so the 22 existing call sites stay as they are:

```go
func encodeComponents(names []string) string    { return fieldcodec.EncodeComponents(names) }
func decodeComponents(stored string) []string   { return fieldcodec.DecodeComponents(stored) }
func componentFilterPattern(name string) string { return fieldcodec.ComponentFilterPattern(name) }
```

Add the `agile-suite/xtm/internal/fieldcodec` import; drop `strings` only if nothing else in `testrepo.go` uses it (it does; leave it).

- [ ] **Step 4: Run** — `go test ./internal/fieldcodec/ ./internal/testrepo/ -count=1` — Expected: PASS.

- [ ] **Step 5: Commit** — `refactor(xtm): move the component encoding to internal/fieldcodec (#158)`

---

### Task 2: Components are an editable field; commit sends them

**Files:**
- Modify: `xtm/internal/testrepo/testrepo.go` (`editableFields`)
- Modify: `xtm/internal/jira/edit.go` (`FieldsForJira`)
- Modify: `xtm/internal/backend/kiwi/write.go` (`parseComponentSet`), `xtm/internal/backend/kiwi/write_test.go`
- Create: `xtm/internal/jira/edit_components_test.go`
- Create: `xtm/internal/testrepo/components_edit.go`, `xtm/internal/testrepo/components_edit_test.go`

**Interfaces:**
- Produces: `func (r *Repository) SetTestComponents(profileID, testKey string, names []string) error` — validates each name, dedupes keeping order, and queues one `components` edit via `EditTestField` (no-op when unchanged).
- Produces: `func validateComponentName(n string) error` (package-private, reused in Task 3).

- [ ] **Step 1: Write the failing tests**

`internal/jira/edit_components_test.go`:

```go
package jira

import (
	"reflect"
	"testing"
)

func TestFieldsForJiraComponents(t *testing.T) {
	got := FieldsForJira(map[string]string{"components": "\nUser Management\nAPI\n"})
	want := []map[string]string{{"name": "User Management"}, {"name": "API"}}
	if !reflect.DeepEqual(got["components"], want) {
		t.Fatalf("components %#v", got["components"])
	}
	cleared := FieldsForJira(map[string]string{"components": ""})
	v, ok := cleared["components"]
	if !ok || !reflect.DeepEqual(v, []map[string]string{}) {
		t.Fatalf("an empty value must clear: %#v (present %v)", v, ok)
	}
}
```

`internal/testrepo/components_edit_test.go`:

```go
package testrepo_test

import (
	"reflect"
	"strings"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func TestSetTestComponentsQueuesOneEdit(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", Summary: "a", Components: []string{"API"}}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetTestComponents("p1", "QA-1", []string{"User Management", "API", "API"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	tc, _ := repo.GetTest("p1", "QA-1")
	if !reflect.DeepEqual(tc.Components, []string{"User Management", "API"}) {
		t.Fatalf("components %v", tc.Components)
	}
	pending, _ := repo.ListPendingChanges("p1")
	if len(pending) != 1 || pending[0].Field != "components" ||
		pending[0].BeforeVal != "\nAPI\n" || pending[0].AfterVal != "\nUser Management\nAPI\n" {
		t.Fatalf("pending %+v", pending)
	}
	// Same set again: nothing new queued.
	if err := repo.SetTestComponents("p1", "QA-1", []string{"User Management", "API"}); err != nil {
		t.Fatal(err)
	}
	if again, _ := repo.ListPendingChanges("p1"); len(again) != 1 {
		t.Fatalf("unchanged set queued another edit: %+v", again)
	}
}

func TestSetTestComponentsRejectsBadNames(t *testing.T) {
	repo := newRepo(t)
	_ = repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", Summary: "a"}})
	for _, bad := range []string{"  ", "two\nlines", strings.Repeat("x", 256)} {
		if err := repo.SetTestComponents("p1", "QA-1", []string{bad}); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}
```

In `kiwi/write_test.go`, change the two `"components": "Login, Backend"` inputs to `"components": "\nLogin\nBackend\n"`, and the single `"components": "Login"` to `"\nLogin\n"`; the expected names stay the same.

- [ ] **Step 2: Run them** — `go test ./internal/jira/ -run FieldsForJiraComponents -count=1; go test ./internal/testrepo/ -run SetTestComponents -count=1; go test ./internal/backend/kiwi/ -count=1` — Expected: the jira test fails on the assertion (no `components` key), testrepo fails to build (`SetTestComponents` undefined), the Kiwi tests fail on the names (the comma split sees one name with newlines trimmed differently). Note the actual Kiwi failure text in the ledger.

- [ ] **Step 3: Implement**

`edit.go`, in `FieldsForJira`'s switch:

```go
		case "components":
			names := fieldcodec.DecodeComponents(v)
			objs := make([]map[string]string, 0, len(names))
			for _, n := range names {
				objs = append(objs, map[string]string{"name": n})
			}
			out["components"] = objs
```

and add the import. Update the function comment's list of handled fields if it has one.

`kiwi/write.go`: replace `parseComponentSet`'s body and comment:

```go
// parseComponentSet splits a FieldsForJira "components" value back into
// names. The value is the newline-bounded form test_case.components uses
// (internal/fieldcodec), because EditTestField writes the queued value into
// that column; component names can contain spaces and commas.
func parseComponentSet(v string) []string {
	return fieldcodec.DecodeComponents(v)
}
```

`testrepo.go`: add `"components": "components",` to `editableFields`.

`components_edit.go`:

```go
package testrepo

import (
	"fmt"
	"strings"
)

const maxComponentLen = 255

// validateComponentName applies the rules Jira enforces on a component name,
// plus no newline, which the stored encoding uses as its separator.
func validateComponentName(n string) error {
	switch {
	case strings.TrimSpace(n) == "":
		return fmt.Errorf("component name is empty")
	case strings.ContainsAny(n, "\r\n"):
		return fmt.Errorf("component %q contains a line break", n)
	case len([]rune(n)) > maxComponentLen:
		return fmt.Errorf("component %q is longer than %d characters", n, maxComponentLen)
	}
	return nil
}

// cleanComponents validates, trims and dedupes names, keeping first-seen order.
func cleanComponents(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		if err := validateComponentName(n); err != nil {
			return nil, err
		}
		n = strings.TrimSpace(n)
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}

// SetTestComponents replaces one test's components, queueing a single
// pending edit. Setting the same list again queues nothing.
func (r *Repository) SetTestComponents(profileID, testKey string, names []string) error {
	clean, err := cleanComponents(names)
	if err != nil {
		return fmt.Errorf("set components on %s: %w", testKey, err)
	}
	var current string
	if err := r.db.QueryRow(
		`SELECT components FROM test_case WHERE profile_id = ? AND jira_key = ?`,
		profileID, testKey).Scan(&current); err != nil {
		return fmt.Errorf("set components on %s: %w", testKey, err)
	}
	next := encodeComponents(clean)
	if next == current {
		return nil
	}
	return r.EditTestField(profileID, testKey, "components", next)
}
```

- [ ] **Step 4: Run** — `go test ./internal/jira/ ./internal/testrepo/ ./internal/backend/... ./internal/syncer/ -count=1` — Expected: PASS. If a syncer commit test enumerates editable fields or `FieldsForJira` output and now sees `components`, update its expectation and ledger it.

- [ ] **Step 5: Commit** — `feat(xtm): components are an editable test field and commit to Jira (#158)`

---

### Task 3: Bulk components in testrepo

**Files:**
- Modify: `xtm/internal/testrepo/components_edit.go`, `components_edit_test.go`

**Interfaces:**
- Produces: `func (r *Repository) BulkEditComponents(profileID string, testKeys, add, remove []string, replace bool) (BulkEditResult, error)` and `func (r *Repository) ListTestComponents(profileID string, testKeys []string) (map[string][]string, error)`.

Rules: with `replace`, every test's components become `add` (cleaned) and `remove` must be empty; an empty `add` clears. Without `replace`, at least one of `add`/`remove` is non-empty and no name is in both. Unchanged tests succeed without a pending change; a missing key fails with `"not found"`, the rest still apply.

- [ ] **Step 1: Write the failing tests**

```go
func seedBulkComponents(t *testing.T, repo *testrepo.Repository) {
	t.Helper()
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", Summary: "a", Components: []string{"API", "Core"}},
		{Key: "QA-2", Summary: "b", Components: []string{"Core"}},
		{Key: "QA-3", Summary: "c"},
	}); err != nil {
		t.Fatal(err)
	}
}

func compsOf(t *testing.T, repo *testrepo.Repository, key string) []string {
	t.Helper()
	tc, err := repo.GetTest("p1", key)
	if err != nil {
		t.Fatal(err)
	}
	return tc.Components
}

func TestBulkEditComponentsAddRemove(t *testing.T) {
	repo := newRepo(t)
	seedBulkComponents(t, repo)
	res, err := repo.BulkEditComponents("p1", []string{"QA-1", "QA-2", "QA-3", "QA-9"},
		[]string{"User Management"}, []string{"Core"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Succeeded, []string{"QA-1", "QA-2", "QA-3"}) ||
		len(res.Failed) != 1 || res.Failed[0].TestKey != "QA-9" || res.Failed[0].Error != "not found" {
		t.Fatalf("result %+v", res)
	}
	if got := compsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"API", "User Management"}) {
		t.Fatalf("QA-1 %v", got)
	}
	if got := compsOf(t, repo, "QA-3"); !reflect.DeepEqual(got, []string{"User Management"}) {
		t.Fatalf("QA-3 %v", got)
	}
}

func TestBulkEditComponentsReplaceAndClear(t *testing.T) {
	repo := newRepo(t)
	seedBulkComponents(t, repo)
	if _, err := repo.BulkEditComponents("p1", []string{"QA-1", "QA-3"}, []string{"Core"}, nil, true); err != nil {
		t.Fatal(err)
	}
	if got := compsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"Core"}) {
		t.Fatalf("QA-1 %v", got)
	}
	if _, err := repo.BulkEditComponents("p1", []string{"QA-1", "QA-2"}, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	if got := compsOf(t, repo, "QA-2"); len(got) != 0 {
		t.Fatalf("QA-2 should be cleared, got %v", got)
	}
	pending, _ := repo.ListPendingChanges("p1")
	if len(pending) != 3 { // QA-1 (one row, updated twice), QA-2, QA-3
		t.Fatalf("pending %d: %+v", len(pending), pending)
	}
}

func TestBulkEditComponentsRejects(t *testing.T) {
	repo := newRepo(t)
	seedBulkComponents(t, repo)
	cases := []struct {
		name        string
		add, remove []string
		replace     bool
	}{
		{"nothing", nil, nil, false},
		{"overlap", []string{"Core"}, []string{"Core"}, false},
		{"replace with remove", []string{"A"}, []string{"Core"}, true},
		{"bad name", []string{"a\nb"}, nil, false},
	}
	for _, c := range cases {
		if _, err := repo.BulkEditComponents("p1", []string{"QA-1"}, c.add, c.remove, c.replace); err == nil {
			t.Errorf("%s: want error", c.name)
		}
	}
}

func TestListTestComponents(t *testing.T) {
	repo := newRepo(t)
	seedBulkComponents(t, repo)
	got, err := repo.ListTestComponents("p1", []string{"QA-2", "QA-3", "QA-9"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"QA-2": {"Core"}, "QA-3": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}
```

The pending count in `TestBulkEditComponentsReplaceAndClear` relies on `pending_change`'s `UNIQUE(profile_id, entity_type, entity_key, field)`: QA-1's second edit updates its row. Check `EditTestField` upserts that way (it must, since labels re-edits do) before trusting the 3.

- [ ] **Step 2: Run** — `go test ./internal/testrepo/ -run 'BulkEditComponents|ListTestComponents' -count=1` — Expected: build failure.

- [ ] **Step 3: Implement** (append to `components_edit.go`; add `database/sql` and `errors` imports)

```go
// BulkEditComponents changes components across tests, one pending edit per
// test that changes. Without replace it removes then adds; with replace each
// test's components become add, and an empty add clears them.
func (r *Repository) BulkEditComponents(profileID string, testKeys, add, remove []string, replace bool) (BulkEditResult, error) {
	result := BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}
	addClean, err := cleanComponents(add)
	if err != nil {
		return result, fmt.Errorf("bulk components: %w", err)
	}
	removeClean, err := cleanComponents(remove)
	if err != nil {
		return result, fmt.Errorf("bulk components: %w", err)
	}
	switch {
	case replace && len(removeClean) > 0:
		return result, fmt.Errorf("bulk components: replace takes no remove list")
	case !replace && len(addClean) == 0 && len(removeClean) == 0:
		return result, fmt.Errorf("bulk components: nothing to add or remove")
	}
	adding := map[string]bool{}
	for _, n := range addClean {
		adding[n] = true
	}
	for _, n := range removeClean {
		if adding[n] {
			return result, fmt.Errorf("bulk components: %q is in both add and remove", n)
		}
	}

	for _, key := range testKeys {
		var current string
		err := r.db.QueryRow(
			`SELECT components FROM test_case WHERE profile_id = ? AND jira_key = ?`,
			profileID, key).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: "not found"})
			continue
		}
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		var next []string
		if replace {
			next = addClean
		} else {
			next = addLabels(removeLabels(decodeComponents(current), removeClean), addClean)
		}
		encoded := encodeComponents(next)
		if encoded == current {
			result.Succeeded = append(result.Succeeded, key)
			continue
		}
		if err := r.EditTestField(profileID, key, "components", encoded); err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, key)
	}
	return result, nil
}

// ListTestComponents returns each requested test's components, keyed by Jira
// key, for the Bulk Components preview. Unknown keys are omitted.
func (r *Repository) ListTestComponents(profileID string, testKeys []string) (map[string][]string, error) {
	out := make(map[string][]string, len(testKeys))
	for _, key := range testKeys {
		var stored string
		err := r.db.QueryRow(
			`SELECT components FROM test_case WHERE profile_id = ? AND jira_key = ?`,
			profileID, key).Scan(&stored)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("list test components %s: %w", key, err)
		}
		out[key] = decodeComponents(stored)
	}
	return out, nil
}
```

`addLabels` / `removeLabels` (from part A's `labels.go`) are generic order-keeping set helpers; reusing them is deliberate (C1). If a reviewer finds the names misleading here, rename them to `addNames` / `removeNames` in `labels.go` in this task.

- [ ] **Step 4: Run** — `go test ./internal/testrepo/ -count=1` — Expected: PASS.

- [ ] **Step 5: Commit** — `feat(xtm): bulk add, remove and replace components on tests (#158)`

---

### Task 4: Bindings, query hook, TokenPicker separator, pending display

**Files:**
- Modify: `xtm/app_components.go`, `xtm/app_components_test.go`
- Regenerate: `wailsjs` (`App.js`, `App.d.ts`)
- Modify: `xtm/frontend/src/api.ts`, `xtm/frontend/src/queries/keys.ts`, `xtm/frontend/src/queries/components.ts`, `components.test.tsx`
- Modify: `xtm/frontend/src/components/TokenPicker.tsx`, `TokenPicker.test.tsx`
- Create: `xtm/frontend/src/lib/components.ts`, `xtm/frontend/src/lib/components.test.ts`
- Modify: `xtm/frontend/src/components/PendingChangesModal.tsx`

**Interfaces:**
- Go bindings: `SetTestComponents(profileID, testKey string, names []string) error`, `BulkEditComponents(profileID string, testKeys, add, remove []string, replace bool) (testrepo.BulkEditResult, error)`, `ListTestComponents(profileID string, testKeys []string) (map[string][]string, error)`.
- TS: `keys.componentOptions(profileId, projectKey)` = `[profileId, "components", "options", projectKey]`; `useComponentOptions(profileId, projectKey)` returns `string[]` from `ListProjectComponents`.
- `TokenPicker` prop `separator?: RegExp | null` (default `/\s+/`; `null` keeps text whole).
- `formatComponentsValue(stored: string): string` → `"A, B"`; `validateComponentName(v: string): string | null`.

- [ ] **Step 1: Write the failing tests**

`app_components_test.go`, append:

```go
func TestSetTestComponentsBindingQueuesEdit(t *testing.T) {
	a := newTestApp(t)
	p, err := a.CreateProfile("Demo", "demo", "DEMO", "", "", "", "", "tok", "", false, "xray")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.repo.UpsertTests(p.ID, []testrepo.TestCase{{Key: "DEMO-1", Summary: "a"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.SetTestComponents(p.ID, "DEMO-1", []string{"User Management"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := a.ListTestComponents(p.ID, []string{"DEMO-1"})
	if err != nil || len(got["DEMO-1"]) != 1 || got["DEMO-1"][0] != "User Management" {
		t.Fatalf("got %v err %v", got, err)
	}
}
```

(add the `testrepo` import; `a.repo` is the App's repository field name, check it in `app.go`).

`lib/components.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { formatComponentsValue, validateComponentName } from "./components";

describe("component helpers", () => {
  it("formats the stored form as a readable list", () => {
    expect(formatComponentsValue("\nUser Management\nAPI\n")).toBe("User Management, API");
    expect(formatComponentsValue("")).toBe("");
  });
  it("validates names", () => {
    expect(validateComponentName("User Management")).toBeNull();
    expect(validateComponentName("   ")).toBe("A component needs a name.");
    expect(validateComponentName("a".repeat(256))).toBe("A component name can be at most 255 characters.");
  });
});
```

`TokenPicker.test.tsx`, add (inside the existing `describe`, extending `Harness` with a `separator` prop passed through):

```tsx
  it("with separator null, keeps pasted and blurred text whole", async () => {
    const spy = vi.fn();
    render(
      <>
        <Harness spy={spy} separator={null} suggestions={["User Management"]} />
        <button>elsewhere</button>
      </>,
    );
    await userEvent.click(input());
    await userEvent.paste("Data Platform");
    await userEvent.click(screen.getByRole("button", { name: "elsewhere" }));
    expect(spy).toHaveBeenLastCalledWith(["Data Platform"]);
  });
```

and in `queries/components.test.tsx`, add `ListProjectComponents: vi.fn()` to the mock and:

```tsx
  it("useComponentOptions returns the cached project components", async () => {
    (api.ListProjectComponents as ReturnType<typeof vi.fn>).mockResolvedValue(["API", "Core"]);
    const { result } = renderHook(() => useComponentOptions("p1", "QA"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.ListProjectComponents).toHaveBeenCalledWith("p1", "QA");
    expect(result.current.data).toEqual(["API", "Core"]);
  });
```

- [ ] **Step 2: Run them** — Go: `go test . -run SetTestComponentsBinding -count=1` (build failure). Frontend: `npx vitest run src/lib/components.test.ts src/components/TokenPicker.test.tsx src/queries/components.test.tsx` (missing module, failing paste test, missing hook).

- [ ] **Step 3: Implement**

`app_components.go`:

```go
// SetTestComponents replaces one test's components, queued for the next
// commit.
func (a *App) SetTestComponents(profileID, testKey string, names []string) (err error) {
	defer recoverToError("SetTestComponents", &err)
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.SetTestComponents(profileID, testKey, names)
}

// BulkEditComponents adds, removes or replaces components across tests,
// queued for the next commit.
func (a *App) BulkEditComponents(profileID string, testKeys, add, remove []string, replace bool) (result testrepo.BulkEditResult, err error) {
	defer recoverToError("BulkEditComponents", &err)
	empty := testrepo.BulkEditResult{Succeeded: []string{}, Failed: []testrepo.BulkFailure{}}
	if err := a.requireStore(); err != nil {
		return empty, err
	}
	return a.repo.BulkEditComponents(profileID, testKeys, add, remove, replace)
}

// ListTestComponents returns each given test's components, for the Bulk
// Components preview.
func (a *App) ListTestComponents(profileID string, testKeys []string) (map[string][]string, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListTestComponents(profileID, testKeys)
}
```

Regenerate `wailsjs`; add the three to `api.ts`'s re-export list.

`keys.ts`, after `projectComponents`:

```ts
  // Cached option list for pickers; under "components" so creates refresh it.
  componentOptions: (profileId: string, projectKey: string) =>
    [profileId, "components", "options", projectKey] as const,
```

`queries/components.ts`:

```ts
// useComponentOptions loads the project's cached component names for the
// component pickers.
export function useComponentOptions(profileId: string, projectKey: string) {
  return useQuery({
    queryKey: keys.componentOptions(profileId, projectKey),
    queryFn: () => call(() => ListProjectComponents(profileId, projectKey)),
    enabled: !!profileId && !!projectKey,
  });
}
```

`lib/components.ts`:

```ts
// The stored components value is newline-bounded ("\nA\nB\n"); see
// internal/fieldcodec. These helpers only read and check names; encoding
// stays in Go.

export function formatComponentsValue(stored: string): string {
  return stored
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean)
    .join(", ");
}

export function validateComponentName(v: string): string | null {
  if (!v.trim()) return "A component needs a name.";
  if (/[\r\n]/.test(v)) return "A component name cannot contain a line break.";
  if ([...v].length > 255) return "A component name can be at most 255 characters.";
  return null;
}
```

`TokenPicker.tsx`: add `separator?: RegExp | null;` to the props (destructure with default `separator = /\s+/`). Introduce:

```ts
  const splitText = (t: string) =>
    separator ? t.split(separator).filter(Boolean) : t.trim() ? [t.trim()] : [];
```

Use it in `onPaste` (with `separator === null`, let the browser paste into the input as normal: `if (!separator) return;` at the top of `onPaste`) and in `onWrapperBlur` for `pending`.

`PendingChangesModal.tsx`, at the top of `describeChange`'s switch:

```ts
    case "test_case":
      if (c.field === "components") {
        return {
          field: "components",
          before: formatComponentsValue(c.beforeVal),
          after: formatComponentsValue(c.afterVal),
        };
      }
      return { field: c.field, before: c.beforeVal, after: c.afterVal };
```

Check the entity type string for test field edits is `"test_case"` (`entityTestCase` in Go).

- [ ] **Step 4: Run** — `go test . -run 'SetTestComponents|ComponentBindings' -count=1 && go vet .`; `npx vitest run src/lib src/components/TokenPicker.test.tsx src/queries src/components/PendingChangesModal* && npx tsc --noEmit -p .` — Expected: PASS.

- [ ] **Step 5: Commit** — `feat(xtm): component bindings, picker separator and readable pending values (#158)`

---

### Task 5: Components field in the test detail panel

**Files:**
- Create: `xtm/frontend/src/components/components-admin/useCreateComponentFromPicker.ts`
- Create: `xtm/frontend/src/components/ComponentsField.tsx`, `ComponentsField.test.tsx`
- Modify: `xtm/frontend/src/components/TestDetail.tsx`

**Interfaces:**
- `useCreateComponentFromPicker(profileId: string, projectKey: string): { onCreate?: (name: string) => Promise<boolean>; error: string; clearError: () => void }` — `onCreate` is `undefined` when the profile lacks `supportsComponentAdmin`; otherwise it confirms through `useConfirm`, calls `CreateComponent`, invalidates `keys.components(profileId)`, and resolves `true`; on decline it resolves `false`; on failure it sets `error` and resolves `false`.
- `ComponentsField({ profileId, projectKey, value: string[], readOnly, onSave: (names: string[]) => void })`.

- [ ] **Step 1: Write the failing test**

`ComponentsField.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ComponentsField } from "./ComponentsField";

const api = vi.hoisted(() => ({
  ListProjectComponents: vi.fn(async () => ["API", "User Management"]),
  CreateComponent: vi.fn(),
}));
vi.mock("../api", () => ({ ...api, errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)) }));
const confirm = vi.hoisted(() => vi.fn(async () => true));
vi.mock("@agile-suite/core", async (orig) => ({
  ...(await orig<typeof import("@agile-suite/core")>()),
  useConfirm: () => ({ confirm }),
}));
const caps = vi.hoisted(() => ({ supportsComponentAdmin: true }));
vi.mock("../features", async (orig) => ({
  ...(await orig<typeof import("../features")>()),
  useCapabilities: () => caps,
}));

function Harness({ onSave = vi.fn(), readOnly = false }) {
  const [value, setValue] = useState<string[]>(["API"]);
  return (
    <ComponentsField
      profileId="p1"
      projectKey="QA"
      value={value}
      readOnly={readOnly}
      onSave={(names) => {
        setValue(names);
        onSave(names);
      }}
    />
  );
}

function renderField(props: Parameters<typeof Harness>[0] = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Harness {...props} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  api.CreateComponent.mockReset();
  confirm.mockReset();
  confirm.mockResolvedValue(true);
  caps.supportsComponentAdmin = true;
});

const picker = () => screen.getByRole("combobox", { name: "Components" });

describe("ComponentsField", () => {
  it("adds a component whose name has a space and saves the list", async () => {
    const onSave = vi.fn();
    renderField({ onSave });
    await userEvent.type(picker(), "User Man");
    await userEvent.click(await screen.findByRole("option", { name: "User Management" }));
    expect(onSave).toHaveBeenLastCalledWith(["API", "User Management"]);
  });

  it("creates an unknown component after confirming", async () => {
    api.CreateComponent.mockResolvedValue({ id: "9", name: "Data Platform" });
    const onSave = vi.fn();
    renderField({ onSave });
    await userEvent.type(picker(), "Data Platform{Enter}");
    expect(confirm).toHaveBeenCalled();
    expect(api.CreateComponent).toHaveBeenCalledWith("p1", expect.objectContaining({ name: "Data Platform" }));
    expect(onSave).toHaveBeenLastCalledWith(["API", "Data Platform"]);
  });

  it("adds nothing when the confirm is declined or Jira refuses", async () => {
    const onSave = vi.fn();
    renderField({ onSave });
    confirm.mockResolvedValueOnce(false);
    await userEvent.type(picker(), "Nope{Enter}");
    expect(api.CreateComponent).not.toHaveBeenCalled();
    api.CreateComponent.mockRejectedValueOnce(new Error("You need project admin rights in Jira to change components."));
    await userEvent.clear(picker());
    await userEvent.type(picker(), "Denied{Enter}");
    expect(await screen.findByText("You need project admin rights in Jira to change components.")).toBeTruthy();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("offers no create row without component admin", async () => {
    caps.supportsComponentAdmin = false;
    renderField();
    await userEvent.type(picker(), "Zzz");
    expect(screen.queryByText('Create "Zzz"')).toBeNull();
  });

  it("read-only shows the components as text", () => {
    renderField({ readOnly: true });
    expect(screen.getByText("API").tagName).toBe("SPAN");
    expect(screen.queryByRole("combobox", { name: "Components" })).toBeNull();
  });
});
```

Check how `useConfirm` and `useCapabilities` are imported where they are used (`useConfirm` may come from `./useConfirm` re-export, `useCapabilities` from `../features`), and mock the module the new code actually imports from.

- [ ] **Step 2: Run** — `npx vitest run src/components/ComponentsField.test.tsx` — Expected: module not found.

- [ ] **Step 3: Implement**

`useCreateComponentFromPicker.ts`:

```ts
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useConfirm } from "@agile-suite/core";
import { CreateComponent, errMsg } from "../../api";
import { useCapabilities } from "../../features";
import { keys } from "../../queries/keys";

// useCreateComponentFromPicker backs a component picker's inline create: ask,
// create in Jira, refresh the component lists. Without component admin the
// picker gets no onCreate and offers no create row.
export function useCreateComponentFromPicker(profileId: string, projectKey: string) {
  const caps = useCapabilities(profileId);
  const { confirm } = useConfirm();
  const qc = useQueryClient();
  const [error, setError] = useState("");

  async function onCreate(name: string): Promise<boolean> {
    setError("");
    const ok = await confirm({
      title: "Create component",
      message: `Create component "${name}" in ${projectKey}?`,
      confirmLabel: "Create",
    });
    if (!ok) return false;
    try {
      await CreateComponent(profileId, { name, description: "", leadUserName: "", assigneeType: "PROJECT_DEFAULT" });
      await qc.invalidateQueries({ queryKey: keys.components(profileId) });
      return true;
    } catch (e) {
      setError(errMsg(e));
      return false;
    }
  }

  return {
    onCreate: caps.supportsComponentAdmin ? onCreate : undefined,
    error,
    clearError: () => setError(""),
  };
}
```

`ComponentsField.tsx`:

```tsx
import { TokenPicker } from "./TokenPicker";
import { useComponentOptions } from "../queries/components";
import { validateComponentName } from "../lib/components";
import { useCreateComponentFromPicker } from "./components-admin/useCreateComponentFromPicker";

interface Props {
  profileId: string;
  projectKey: string;
  value: string[];
  readOnly: boolean;
  onSave: (names: string[]) => void;
}

// ComponentsField is the test detail's component control. Each change saves
// at once, like LabelsField. Only the project's components are offered; with
// component admin an unknown name can be created after a confirm.
export function ComponentsField({ profileId, projectKey, value, readOnly, onSave }: Props) {
  const options = useComponentOptions(profileId, projectKey);
  const create = useCreateComponentFromPicker(profileId, projectKey);
  if (readOnly) return <span>{value.join(", ")}</span>;
  return (
    <>
      <TokenPicker
        label="Components"
        value={value}
        onChange={(next) => {
          create.clearError();
          onSave(next);
        }}
        suggestions={options.data ?? []}
        allowCreate={!!create.onCreate}
        onCreate={create.onCreate}
        validate={validateComponentName}
        separator={null}
        placeholder="Type to search"
      />
      {create.error && <p className="token-picker-error">{create.error}</p>}
    </>
  );
}
```

The read-only fallback for no components renders an empty span rather than an em dash (UI copy contract).

`TestDetail.tsx`: after the Labels `<dd>`, add:

```tsx
            <dt>
              Components {isDirty("components") && <DirtyDot />}
            </dt>
            <dd>
              <ComponentsField
                profileId={profileId}
                projectKey={activeProfile?.projectKey ?? ""}
                value={test.components ?? []}
                readOnly={!!readOnly}
                onSave={saveComponents}
              />
            </dd>
```

and, beside `saveField`:

```tsx
  // saveComponents queues a components edit. The value is encoded in Go
  // (SetTestComponents), so it bypasses saveField's string path.
  async function saveComponents(names: string[]) {
    if (readOnly || !test) return;
    if (names.join("\n") === (test.components ?? []).join("\n")) return;
    setSaveError("");
    try {
      await SetTestComponents(profileId, testKey, names);
      queryClient.setQueryData(keys.test(profileId, testKey), { ...test, components: names });
      onEdited();
    } catch (e) {
      setSaveError(`Save failed: ${errMsg(e)}`);
    }
  }
```

Check `isDirty` accepts `"components"` (widen its parameter type if it is `EditableField`), and get `activeProfile` the way TestDetail already reads profile data (`useProfile()`); if TestDetail already has the project key under another name, use that.

- [ ] **Step 4: Run** — `npx vitest run src/components/ComponentsField.test.tsx && npx tsc --noEmit -p .`; then the full `npx vitest run`. Expected: PASS.

- [ ] **Step 5: Commit** — `feat(xtm): edit a test's components in the detail panel (#158)`

---

### Task 6: Bulk Components dialog

**Files:**
- Create: `xtm/frontend/src/components/BulkComponentsModal.tsx`, `BulkComponentsModal.test.tsx`
- Modify: `xtm/frontend/src/contexts/ModalContext.tsx` (`"bulkComponents"`), `xtm/frontend/src/App.tsx` (toolbar button after **Labels…**, render block)

**Interfaces:**
- `BulkComponentsModal({ testKeys, onComplete, onCancel })`, same props as `BulkLabelsModal`.
- Consumes `countChanged` from `./BulkLabelsModal` for the add/remove preview.

- [ ] **Step 1: Write the failing test**

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BulkComponentsModal } from "./BulkComponentsModal";

const api = vi.hoisted(() => ({
  BulkEditComponents: vi.fn(),
  ListProjectComponents: vi.fn(async () => ["API", "Core", "User Management"]),
  ListTestComponents: vi.fn(async () => ({ "QA-1": ["API", "Core"], "QA-2": ["Core"], "QA-3": [] })),
  CreateComponent: vi.fn(),
}));
vi.mock("../api", () => ({ ...api, errMsg: (e: unknown) => String(e) }));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1", activeProfile: { projectKey: "QA" } }),
}));
vi.mock("../features", async (orig) => ({
  ...(await orig<typeof import("../features")>()),
  useCapabilities: () => ({ supportsComponentAdmin: false }),
}));

function renderModal(onComplete = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <BulkComponentsModal testKeys={["QA-1", "QA-2", "QA-3"]} onComplete={onComplete} onCancel={() => {}} />
    </QueryClientProvider>,
  );
  return onComplete;
}

async function pick(name: string, text: string) {
  await userEvent.type(screen.getByRole("combobox", { name }), text);
  await userEvent.click(await screen.findByRole("option", { name: text }));
}

beforeEach(() => {
  api.BulkEditComponents.mockReset();
  api.BulkEditComponents.mockResolvedValue({ succeeded: ["QA-1", "QA-2", "QA-3"], failed: [] });
});

describe("BulkComponentsModal", () => {
  it("adds and removes with a preview", async () => {
    const onComplete = renderModal();
    await pick("Add components", "User Management");
    await pick("Remove components", "Core");
    expect(await screen.findByText("3 of 3 selected tests will change.")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(api.BulkEditComponents).toHaveBeenCalledWith("p1", ["QA-1", "QA-2", "QA-3"], ["User Management"], ["Core"], false);
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it("replace all with nothing clears every selected test", async () => {
    renderModal();
    await userEvent.click(screen.getByRole("radio", { name: "Replace all" }));
    expect(screen.queryByRole("combobox", { name: "Remove components" })).toBeNull();
    expect(await screen.findByText("2 of 3 selected tests will change.")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(api.BulkEditComponents).toHaveBeenCalledWith("p1", ["QA-1", "QA-2", "QA-3"], [], [], true);
  });

  it("offers no create row on a profile without component admin", async () => {
    renderModal();
    await userEvent.type(screen.getByRole("combobox", { name: "Add components" }), "Zzz");
    expect(screen.queryByText('Create "Zzz"')).toBeNull();
  });
});
```

- [ ] **Step 2: Run** — `npx vitest run src/components/BulkComponentsModal.test.tsx` — Expected: module not found.

- [ ] **Step 3: Implement** `BulkComponentsModal.tsx`, mirroring `BulkLabelsModal.tsx` (read it first and keep the same class names and layout):

```tsx
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Modal } from "@agile-suite/core";
import { useProfile } from "../contexts/ProfileContext";
import { TokenPicker } from "./TokenPicker";
import { countChanged } from "./BulkLabelsModal";
import { useComponentOptions } from "../queries/components";
import { validateComponentName } from "../lib/components";
import { useCreateComponentFromPicker } from "./components-admin/useCreateComponentFromPicker";
import { BulkEditComponents, ListTestComponents, errMsg } from "../api";
import type { BulkEditResult } from "../api";
import { call } from "../lib/apiCall";

interface Props {
  testKeys: string[];
  onComplete: (result: BulkEditResult) => void;
  onCancel: () => void;
}

// BulkComponentsModal adds and removes components across the selection, or
// replaces them outright. Each changed test gets one pending edit.
export function BulkComponentsModal({ testKeys, onComplete, onCancel }: Props) {
  const { activeId: profileId, activeProfile } = useProfile();
  const projectKey = activeProfile?.projectKey ?? "";
  const [mode, setMode] = useState<"edit" | "replace">("edit");
  const [add, setAdd] = useState<string[]>([]);
  const [remove, setRemove] = useState<string[]>([]);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<BulkEditResult | null>(null);

  const options = useComponentOptions(profileId, projectKey);
  const create = useCreateComponentFromPicker(profileId, projectKey);
  const { data: current } = useQuery({
    queryKey: [profileId, "tests", "components", testKeys],
    queryFn: () => call(() => ListTestComponents(profileId, testKeys)),
    enabled: !!profileId,
  });

  const replace = mode === "replace";
  const changed = useMemo(() => {
    if (!current) return null;
    if (!replace) return countChanged(current, add, remove);
    return Object.values(current).filter((c) => c.join("\n") !== add.join("\n")).length;
  }, [current, add, remove, replace]);

  async function apply() {
    const overlap = add.find((n) => remove.includes(n));
    if (!replace && overlap) {
      setError(`"${overlap}" is in both Add and Remove.`);
      return;
    }
    if (!replace && add.length === 0 && remove.length === 0) {
      setError("Pick at least one component to add or remove.");
      return;
    }
    setApplying(true);
    setError("");
    try {
      const r = await BulkEditComponents(profileId, testKeys, add, replace ? [] : remove, replace);
      setResult(r);
      if (r.failed.length === 0) onComplete(r);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setApplying(false);
    }
  }

  const n = testKeys.length;
  const suggestions = options.data ?? [];
  return (
    <Modal onClose={onCancel} className="modal bulk-modal" labelledBy="bulk-components-title">
      <div className="pending-head">
        <h2 id="bulk-components-title">
          Components ({n} {n === 1 ? "test" : "tests"})
        </h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close">
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <fieldset className="bulk-row">
          <legend className="sr-only">Mode</legend>
          <label>
            <input type="radio" checked={!replace} onChange={() => setMode("edit")} /> Add and remove
          </label>
          <label>
            <input type="radio" checked={replace} onChange={() => setMode("replace")} /> Replace all
          </label>
        </fieldset>
        <div className="bulk-row">
          <span>{replace ? "Set to" : "Add"}</span>
          <TokenPicker
            label="Add components"
            value={add}
            onChange={setAdd}
            suggestions={suggestions}
            allowCreate={!!create.onCreate}
            onCreate={create.onCreate}
            validate={validateComponentName}
            separator={null}
            placeholder="Type to search"
          />
        </div>
        {!replace && (
          <div className="bulk-row">
            <span>Remove</span>
            <TokenPicker
              label="Remove components"
              value={remove}
              onChange={setRemove}
              suggestions={suggestions}
              allowCreate={false}
              validate={validateComponentName}
              separator={null}
              placeholder="Type to search"
            />
          </div>
        )}
        {changed !== null && (
          <p className="muted bulk-preview">
            {changed} of {n} selected tests will change.
          </p>
        )}
        {create.error && <div className="error-text">{create.error}</div>}
        {error && <div className="error-text">{error}</div>}
        {result && result.failed.length > 0 && (
          <div className="error-text">
            <ul className="commit-fail-list">
              {result.failed.map((f) => (
                <li key={f.testKey}>
                  <span className="mono">{f.testKey}</span>: {f.error}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={applying}>
          Cancel
        </button>
        <button className="btn btn-primary" onClick={apply} disabled={applying}>
          {applying ? "Applying…" : "Apply"}
        </button>
      </div>
    </Modal>
  );
}
```

Check `fieldset.bulk-row` renders acceptably (the `.bulk-row` rule targets a flex row; a `fieldset` needs `border: 0; margin: 0; padding: 0;` — add a `.bulk-row-radios` class with that rule to `App.css` if the default fieldset border shows) and that `.bulk-row span` styling does not catch the radio labels badly.

`ModalContext.tsx`: add `| "bulkComponents"`. `App.tsx`: a **Components…** button right after **Labels…** in the bulk toolbar (`openModal("bulkComponents")`), and the render block after the `bulkLabels` one, same shape, importing `BulkComponentsModal`.

- [ ] **Step 4: Run** — `npx vitest run && npx tsc --noEmit -p .`; from the root `npm run lint && bash scripts/ratchet.sh`. Expected: PASS, no counter up.

- [ ] **Step 5: Commit** — `feat(xtm): a Bulk Components dialog to add, remove or replace components (#158)`

---

### Task 7: Docs, gates, PR

- [ ] **Step 1:** User guide: in "Viewing & editing a test", a "Components" paragraph after "Labels" (picker, names with spaces, only project components offered, create with confirm when you have component admin, saved as pending at once); in "Bulk operations", a **Components…** entry (add and remove, or replace all; empty replace clears; preview). Run the humanizer skill on the new text in file mode.
- [ ] **Step 2:** Rebase onto `main` if #162 has merged (`git fetch origin && git rebase origin/main`), then `make gates`. The known locale-dependent TAM test may fail on this machine; name it if so.
- [ ] **Step 3:** Commit `docs(xtm): describe editing components on tests (#158)`, push, and open the PR against `main` with #158's acceptance lines, the spec link plus the five deviations above, and the checks run. If #162 has not merged yet, open it as a draft and say it is stacked on #162.
