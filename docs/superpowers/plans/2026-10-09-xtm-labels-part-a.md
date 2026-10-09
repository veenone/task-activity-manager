# XTM labels, part A: token picker and Bulk Labels — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Label editing in XTM gets autocomplete from labels already in use, inline creation of new labels, and a dedicated Bulk Labels action that adds and removes several labels across the selected tests in one pass.

**Architecture:** A `ListLabels` read and a `BulkEditLabels` write join `internal/testrepo` in a new `labels.go`, exposed as Wails bindings in `app.go`. On the frontend, a new `TokenPicker` combobox replaces the label text inputs in `TestDetail` and `BulkEditModal`, and a new `BulkLabelsModal` opens from the bulk toolbar. All writes go through the existing pending-change journal as `labels` field edits, so commit code does not change.

**Tech Stack:** Go 1.x with SQLite (`internal/testrepo`), Wails v2 bindings, React and TypeScript, TanStack Query, Vitest with Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-09-xtm-labels-components-design.md`, part A.

## Global Constraints

- Run Go commands from `xtm/`; run npm commands from the repo root.
- Never hand-edit `xtm/frontend/wailsjs`. Regenerate with `wails generate module` from `xtm/`.
- No `console` calls in `src`. No em dash in any user-visible string.
- No hex colors or px font sizes in new CSS; use the `var(--…)` tokens from `style.css`.
- Modals use the `Modal` component already imported by `BulkEditModal`.
- Every class a component sets must be defined in a stylesheet.
- Test assertions check values and visible text, never element existence or child count alone (C6).
- New Go code goes in new files; `testrepo.go` is already far over 400 lines and must not grow beyond the small `applyBulkOperation` edit in Task 1.
- The stored and journaled label value stays a space-separated string.
- Label validation: no whitespace, at most 255 characters.
- Commits reference the issue for part A, and carry no AI attribution.

## Review Focus

1. Labels that differ only in case (`Smoke` versus `smoke`). Jira treats them as different labels, so suggestions, adds and removes must be case-sensitive. Task 1 and Task 3 each pin this with a test.
2. Pasting `a b c` into the picker. The user expects three chips, not an error about whitespace. Task 3 pins this.
3. A selection that includes a key the profile no longer has (deleted by a sync while the modal is open). It must be reported as failed while the rest apply. Task 1 pins this.
4. Add and remove lists that share a label. The modal must refuse before calling the backend, and the backend must refuse too. Tasks 1 and 5 pin this.
5. A read-only profile opening a test. It must show the labels as text with no picker, so nothing can be edited. Task 4 pins this. (An unchanged blur is already kept from queueing a change by the `value === backendValue` early return in `TestDetail`'s `saveField`, which this plan does not touch.)

---

### Task 1: ListLabels and BulkEditLabels in testrepo

**Files:**
- Create: `xtm/internal/testrepo/labels.go`
- Create: `xtm/internal/testrepo/labels_test.go`
- Modify: `xtm/internal/testrepo/testrepo.go` (the `add_label` and `remove_label` cases in `applyBulkOperation`)

**Interfaces:**
- Produces: `func (r *Repository) ListLabels(profileID string) ([]Bucket, error)`, sorted by label, case-sensitive.
- Produces: `func (r *Repository) BulkEditLabels(profileID string, testKeys []string, add, remove []string) (BulkEditResult, error)`.
- Produces: `add_label` and `remove_label` in `BulkEditTests` accept several space-separated labels in `Value`.

- [ ] **Step 1: Write the failing tests**

`xtm/internal/testrepo/labels_test.go`:

```go
package testrepo_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

const lblProfile = "p1"

func seedLabelTests(t *testing.T, repo *testrepo.Repository) {
	t.Helper()
	tests := []testrepo.TestCase{
		{Key: "QA-1", Summary: "a", Labels: []string{"smoke", "login"}},
		{Key: "QA-2", Summary: "b", Labels: []string{"smoke"}},
		{Key: "QA-3", Summary: "c", Labels: []string{"Smoke"}},
		{Key: "QA-4", Summary: "d"},
	}
	if err := repo.UpsertTests(lblProfile, tests); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestListLabelsCountsCaseSensitive(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	got, err := repo.ListLabels(lblProfile)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []testrepo.Bucket{
		{Label: "Smoke", Count: 1},
		{Label: "login", Count: 1},
		{Label: "smoke", Count: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func labelsOf(t *testing.T, repo *testrepo.Repository, key string) []string {
	t.Helper()
	tc, err := repo.GetTest(lblProfile, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	return tc.Labels
}

func TestBulkEditLabelsAddsAndRemoves(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	res, err := repo.BulkEditLabels(lblProfile,
		[]string{"QA-1", "QA-2", "QA-4"},
		[]string{"regression", "login"},
		[]string{"smoke"})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if len(res.Failed) != 0 || len(res.Succeeded) != 3 {
		t.Fatalf("result %+v", res)
	}
	if got := labelsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"login", "regression"}) {
		t.Fatalf("QA-1 labels %v", got)
	}
	if got := labelsOf(t, repo, "QA-2"); !reflect.DeepEqual(got, []string{"regression", "login"}) {
		t.Fatalf("QA-2 labels %v", got)
	}
	if got := labelsOf(t, repo, "QA-4"); !reflect.DeepEqual(got, []string{"regression", "login"}) {
		t.Fatalf("QA-4 labels %v", got)
	}
}

func TestBulkEditLabelsSkipsUnchangedAndKeepsCase(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	// Removing "smoke" must not touch QA-3's "Smoke".
	res, err := repo.BulkEditLabels(lblProfile, []string{"QA-3"}, nil, []string{"smoke"})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if !reflect.DeepEqual(res.Succeeded, []string{"QA-3"}) {
		t.Fatalf("result %+v", res)
	}
	pending, err := repo.ListPendingChanges(lblProfile)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("unchanged test queued %d pending changes: %+v", len(pending), pending)
	}
}

func TestBulkEditLabelsReportsMissingKey(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	res, err := repo.BulkEditLabels(lblProfile, []string{"QA-1", "QA-99"}, []string{"x"}, nil)
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if !reflect.DeepEqual(res.Succeeded, []string{"QA-1"}) {
		t.Fatalf("succeeded %v", res.Succeeded)
	}
	if len(res.Failed) != 1 || res.Failed[0].TestKey != "QA-99" || res.Failed[0].Error != "not found" {
		t.Fatalf("failed %+v", res.Failed)
	}
}

func TestBulkEditLabelsRejectsBadInput(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	cases := []struct {
		name        string
		add, remove []string
	}{
		{"overlap", []string{"smoke"}, []string{"smoke"}},
		{"whitespace", []string{"two words"}, nil},
		{"empty", nil, nil},
	}
	for _, c := range cases {
		if _, err := repo.BulkEditLabels(lblProfile, []string{"QA-1"}, c.add, c.remove); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

func TestBulkEditTestsAddLabelAcceptsSeveral(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	_, err := repo.BulkEditTests(lblProfile, []string{"QA-2"},
		testrepo.BulkEdit{Operation: "add_label", Value: "a b smoke"})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if got := labelsOf(t, repo, "QA-2"); !reflect.DeepEqual(got, []string{"smoke", "a", "b"}) {
		t.Fatalf("labels %v", got)
	}
	_, err = repo.BulkEditTests(lblProfile, []string{"QA-2"},
		testrepo.BulkEdit{Operation: "remove_label", Value: "a smoke"})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if got := labelsOf(t, repo, "QA-2"); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("labels %v", got)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/testrepo/ -run 'Labels|AddLabelAcceptsSeveral' -count=1`
Expected: build failure, `repo.ListLabels undefined` and `repo.BulkEditLabels undefined`. After Step 3a alone, `TestBulkEditTestsAddLabelAcceptsSeveral` fails on an assertion, so it is proven red against existing code.

- [ ] **Step 3a: Accept several labels in add_label and remove_label**

In `applyBulkOperation` in `testrepo.go`, replace the two label cases:

```go
	case "add_label":
		want := strings.Fields(op.Value)
		if len(want) == 0 {
			return "", fmt.Errorf("label value is required")
		}
		return strings.Join(addLabels(strings.Fields(current), want), " "), nil

	case "remove_label":
		drop := strings.Fields(op.Value)
		if len(drop) == 0 {
			return "", fmt.Errorf("label value is required")
		}
		return strings.Join(removeLabels(strings.Fields(current), drop), " "), nil
```

The no-op check in `BulkEditTests` compares strings, and `strings.Join(strings.Fields(current), " ")` equals `current` for any value the app wrote, so a no-op still reports success without queueing.

- [ ] **Step 3b: Write labels.go**

`xtm/internal/testrepo/labels.go`:

```go
package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// maxLabelLen is Jira's limit on a single label.
const maxLabelLen = 255

// ListLabels returns the distinct labels across a profile's Tests with a count
// each, sorted by label. Jira labels are case-sensitive, so "Smoke" and
// "smoke" are separate entries. It backs the label picker's suggestions.
func (r *Repository) ListLabels(profileID string) ([]Bucket, error) {
	rows, err := r.db.Query(
		`SELECT labels FROM test_case WHERE profile_id = ? AND labels <> ''`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			return nil, err
		}
		for _, l := range strings.Fields(stored) {
			counts[l]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Bucket, 0, len(counts))
	for l, n := range counts {
		out = append(out, Bucket{Label: l, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

// BulkEditLabels adds and removes labels on each given Test, queueing one
// "labels" field edit per Test whose labels change. Tests that would not
// change are reported as succeeded without a pending change. Each Test is
// edited on its own through EditTestField, as BulkEditTests does, so one
// failure does not block the others.
func (r *Repository) BulkEditLabels(profileID string, testKeys []string, add, remove []string) (BulkEditResult, error) {
	result := BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}
	if err := validateLabelEdit(add, remove); err != nil {
		return result, fmt.Errorf("bulk labels: %w", err)
	}

	for _, key := range testKeys {
		var current string
		err := r.db.QueryRow(
			`SELECT labels FROM test_case WHERE profile_id = ? AND jira_key = ?`,
			profileID, key,
		).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: "not found"})
			continue
		}
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		before := strings.Fields(current)
		after := addLabels(removeLabels(before, remove), add)
		next := strings.Join(after, " ")
		if next == strings.Join(before, " ") {
			result.Succeeded = append(result.Succeeded, key)
			continue
		}
		if err := r.EditTestField(profileID, key, "labels", next); err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, key)
	}
	return result, nil
}

// validateLabelEdit rejects an empty request, a label Jira would refuse, and
// a label that appears in both lists.
func validateLabelEdit(add, remove []string) error {
	if len(add) == 0 && len(remove) == 0 {
		return fmt.Errorf("nothing to add or remove")
	}
	adding := map[string]bool{}
	for _, l := range add {
		if err := validateLabel(l); err != nil {
			return err
		}
		adding[l] = true
	}
	for _, l := range remove {
		if err := validateLabel(l); err != nil {
			return err
		}
		if adding[l] {
			return fmt.Errorf("label %q is in both add and remove", l)
		}
	}
	return nil
}

func validateLabel(l string) error {
	if l == "" {
		return fmt.Errorf("label is empty")
	}
	if strings.IndexFunc(l, unicode.IsSpace) >= 0 {
		return fmt.Errorf("label %q contains whitespace", l)
	}
	if len([]rune(l)) > maxLabelLen {
		return fmt.Errorf("label %q is longer than %d characters", l, maxLabelLen)
	}
	return nil
}

// addLabels appends each label in add that labels does not already hold,
// keeping the existing order.
func addLabels(labels, add []string) []string {
	have := make(map[string]bool, len(labels))
	out := append([]string{}, labels...)
	for _, l := range labels {
		have[l] = true
	}
	for _, l := range add {
		if !have[l] {
			out = append(out, l)
			have[l] = true
		}
	}
	return out
}

// removeLabels drops every label in remove, keeping the order of the rest.
func removeLabels(labels, remove []string) []string {
	drop := make(map[string]bool, len(remove))
	for _, l := range remove {
		drop[l] = true
	}
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if !drop[l] {
			out = append(out, l)
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `go test ./internal/testrepo/ -count=1`
Expected: PASS, including the existing `BulkEditTests` tests in `testrepo_test.go`.

- [ ] **Step 5: Commit**

```bash
git add xtm/internal/testrepo/labels.go xtm/internal/testrepo/labels_test.go xtm/internal/testrepo/testrepo.go
git commit -m "feat(xtm): list labels and bulk add or remove them (#<issue>)"
```

---

### Task 2: Bindings, API exports and the labels query

**Files:**
- Modify: `xtm/app.go` (next to `BulkEditTests` and `ListComponents`)
- Regenerate: `xtm/frontend/wailsjs/go/main/App.{js,d.ts}`, `models.ts`
- Modify: `xtm/frontend/src/api.ts`
- Modify: `xtm/frontend/src/queries/keys.ts`, `xtm/frontend/src/queries/invalidate.ts`, `xtm/frontend/src/queries/app.ts`
- Test: `xtm/frontend/src/queries/app.test.tsx`

**Interfaces:**
- Consumes: `Repository.ListLabels`, `Repository.BulkEditLabels` from Task 1.
- Produces: `App.ListLabels(profileID string) ([]testrepo.Bucket, error)` and `App.BulkEditLabels(profileID string, testKeys, add, remove []string) (testrepo.BulkEditResult, error)`; TS exports `ListLabels`, `BulkEditLabels` from `../api`; `keys.labels(profileId)`; `useLabels(profileId: string): UseQueryResult<Bucket[]>`.

- [ ] **Step 1: Write the failing query test**

Add to `xtm/frontend/src/queries/app.test.tsx`, following the existing `ListComponents` mock there: add `ListLabels: vi.fn()` to the `vi.mock("../api", …)` factory, import it, and add:

```tsx
describe("useLabels", () => {
  it("returns the profile's labels from ListLabels", async () => {
    vi.mocked(ListLabels).mockResolvedValue([
      { label: "login", count: 1 },
      { label: "smoke", count: 2 },
    ]);
    const { result } = renderHook(() => useLabels("p1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(vi.mocked(ListLabels)).toHaveBeenCalledWith("p1");
    expect(result.current.data?.map((b) => b.label)).toEqual(["login", "smoke"]);
  });
});
```

Use the `wrapper` and imports the file already defines for its other hooks.

- [ ] **Step 2: Run it to see it fail**

Run: `npx vitest run src/queries/app.test.tsx` from `xtm/frontend`
Expected: FAIL, `useLabels` is not exported.

- [ ] **Step 3: Add the Go bindings**

In `xtm/app.go`, after `ListComponents`:

```go
// ListLabels returns the distinct labels across a profile's Tests with a count
// each, for the label picker's suggestions.
func (a *App) ListLabels(profileID string) ([]testrepo.Bucket, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListLabels(profileID)
}
```

After `BulkEditTests`:

```go
// BulkEditLabels adds and removes labels across a batch of Tests, queueing one
// labels edit per Test that changes. The Bulk Labels modal calls it.
func (a *App) BulkEditLabels(profileID string, testKeys, add, remove []string) (result testrepo.BulkEditResult, err error) {
	defer recoverToError("BulkEditLabels", &err)
	empty := testrepo.BulkEditResult{Succeeded: []string{}, Failed: []testrepo.BulkFailure{}}
	if err := a.requireStore(); err != nil {
		return empty, err
	}
	return a.repo.BulkEditLabels(profileID, testKeys, add, remove)
}
```

- [ ] **Step 4: Regenerate the bindings**

Run from `xtm/`: `wails generate module`
Then `git diff --stat xtm/frontend/wailsjs` must show only `App.js` and `App.d.ts` gaining `ListLabels` and `BulkEditLabels`. Revert any unrelated wailsjs churn (runtime line endings) with `git checkout -- <file>` before committing.

- [ ] **Step 5: Export and wire the query**

`api.ts`: add `ListLabels,` after `ListComponents,` and `BulkEditLabels,` after `BulkEditTests,` in the re-export list.

`queries/keys.ts`, after `components`:

```ts
  labels: (profileId: string) => [profileId, "labels"] as const,
```

`queries/invalidate.ts`: add `keys.labels(profileId),` after `keys.components(profileId),`, so a commit, sync or bulk edit refreshes the suggestions.

`queries/app.ts`: add `ListLabels` to the import and:

```ts
// useLabels loads the distinct labels on the profile's Tests, the label
// picker's suggestions.
export function useLabels(profileId: string) {
  return useQuery({
    queryKey: keys.labels(profileId),
    queryFn: () => call(() => ListLabels(profileId)),
    enabled: !!profileId,
    placeholderData: (prev) => prev,
  });
}
```

- [ ] **Step 6: Run the tests**

Run from `xtm/frontend`: `npx vitest run src/queries` and from `xtm/`: `go vet ./... && go build ./...`
Expected: PASS and a clean build.

- [ ] **Step 7: Commit**

```bash
git add xtm/app.go xtm/frontend/wailsjs/go/main/App.js xtm/frontend/wailsjs/go/main/App.d.ts xtm/frontend/src/api.ts xtm/frontend/src/queries
git commit -m "feat(xtm): expose label list and bulk label edit to the frontend (#<issue>)"
```

---

### Task 3: TokenPicker

**Files:**
- Create: `xtm/frontend/src/components/TokenPicker.tsx`
- Create: `xtm/frontend/src/components/TokenPicker.test.tsx`
- Modify: `xtm/frontend/src/App.css` (append a `/* TokenPicker */` block next to the `.multiselect` rules)

**Interfaces:**
- Produces:

```ts
export interface TokenPickerProps {
  value: string[];
  onChange: (next: string[]) => void;
  suggestions: string[];
  allowCreate: boolean;
  onCreate?: (v: string) => Promise<boolean>;
  validate?: (v: string) => string | null;
  label: string;
  placeholder?: string;
  onBlur?: () => void;
}
export function TokenPicker(props: TokenPickerProps): JSX.Element;
export function validateLabel(v: string): string | null;
```

`onBlur` fires when focus leaves the whole picker; `TestDetail` uses it to save. `validateLabel` lives in this file so every label picker shares one rule.

- [ ] **Step 1: Write the failing tests**

`TokenPicker.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TokenPicker, validateLabel } from "./TokenPicker";

function Harness(props: {
  initial?: string[];
  suggestions?: string[];
  allowCreate?: boolean;
  onCreate?: (v: string) => Promise<boolean>;
  spy?: (v: string[]) => void;
}) {
  const [value, setValue] = useState(props.initial ?? []);
  return (
    <TokenPicker
      label="Labels"
      value={value}
      onChange={(v) => {
        setValue(v);
        props.spy?.(v);
      }}
      suggestions={props.suggestions ?? ["smoke", "Smoke", "login"]}
      allowCreate={props.allowCreate ?? true}
      onCreate={props.onCreate}
      validate={validateLabel}
    />
  );
}

const input = () => screen.getByRole("combobox", { name: "Labels" });

describe("TokenPicker", () => {
  it("filters suggestions, keeping case variants, and picks with the keyboard", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await userEvent.type(input(), "smo");
    const options = screen.getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual(["smoke", "Smoke", 'Create "smo"']);
    await userEvent.keyboard("{ArrowDown}{Enter}");
    expect(spy).toHaveBeenLastCalledWith(["Smoke"]);
  });

  it("hides suggestions already chosen", async () => {
    render(<Harness initial={["smoke"]} />);
    await userEvent.type(input(), "s");
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Smoke",
      'Create "s"',
    ]);
  });

  it("creates an unknown value on Enter", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await userEvent.type(input(), "regression{Enter}");
    expect(spy).toHaveBeenLastCalledWith(["regression"]);
    expect(screen.getByRole("button", { name: "Remove regression" })).toBeTruthy();
  });

  it("splits pasted whitespace into separate values", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await userEvent.click(input());
    await userEvent.paste("a b  c");
    expect(spy).toHaveBeenLastCalledWith(["a", "b", "c"]);
  });

  it("shows the validation error and refuses the add", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} />);
    await userEvent.type(input(), "x".repeat(256) + "{Enter}");
    expect(screen.getByRole("alert").textContent).toBe(
      "A label can be at most 255 characters.",
    );
    expect(spy).not.toHaveBeenCalled();
  });

  it("does not add when onCreate resolves false", async () => {
    const spy = vi.fn();
    const onCreate = vi.fn(async () => false);
    render(<Harness spy={spy} onCreate={onCreate} />);
    await userEvent.type(input(), "newthing{Enter}");
    expect(onCreate).toHaveBeenCalledWith("newthing");
    expect(spy).not.toHaveBeenCalled();
  });

  it("without allowCreate, Enter on an unknown value adds nothing", async () => {
    const spy = vi.fn();
    render(<Harness spy={spy} allowCreate={false} />);
    await userEvent.type(input(), "nope{Enter}");
    expect(spy).not.toHaveBeenCalled();
    expect(screen.queryByText('Create "nope"')).toBeNull();
  });

  it("removes the last chip on Backspace in an empty input", async () => {
    const spy = vi.fn();
    render(<Harness initial={["a", "b"]} spy={spy} />);
    await userEvent.click(input());
    await userEvent.keyboard("{Backspace}");
    expect(spy).toHaveBeenLastCalledWith(["a"]);
  });

  it("validateLabel rejects whitespace and over-long labels", () => {
    expect(validateLabel("ok")).toBeNull();
    expect(validateLabel("two words")).toBe("A label cannot contain spaces.");
    expect(validateLabel("x".repeat(256))).toBe(
      "A label can be at most 255 characters.",
    );
  });
});
```

- [ ] **Step 2: Run them to see them fail**

Run from `xtm/frontend`: `npx vitest run src/components/TokenPicker.test.tsx`
Expected: FAIL, module `./TokenPicker` not found.

- [ ] **Step 3: Implement TokenPicker**

`TokenPicker.tsx`:

```tsx
import { useId, useMemo, useRef, useState } from "react";
import type { ClipboardEvent, FocusEvent, KeyboardEvent } from "react";

export interface TokenPickerProps {
  value: string[];
  onChange: (next: string[]) => void;
  suggestions: string[];
  allowCreate: boolean;
  onCreate?: (v: string) => Promise<boolean>;
  validate?: (v: string) => string | null;
  label: string;
  placeholder?: string;
  onBlur?: () => void;
}

const MAX_LABEL = 255;

// validateLabel is the one rule every label picker uses: Jira rejects labels
// with whitespace and labels longer than 255 characters.
export function validateLabel(v: string): string | null {
  if (/\s/.test(v)) return "A label cannot contain spaces.";
  if ([...v].length > MAX_LABEL) return "A label can be at most 255 characters.";
  return null;
}

interface Option {
  value: string;
  create: boolean;
}

// TokenPicker is a multi-value combobox: chosen values render as removable
// chips, typing filters the suggestions, and with allowCreate an unknown value
// can be added. Matching is case-sensitive for membership because Jira labels
// and components are, and case-insensitive for filtering so "smo" finds both
// "smoke" and "Smoke".
export function TokenPicker({
  value,
  onChange,
  suggestions,
  allowCreate,
  onCreate,
  validate,
  label,
  placeholder,
  onBlur,
}: TokenPickerProps) {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [error, setError] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  const listId = useId();
  const errorId = useId();

  const options = useMemo<Option[]>(() => {
    const q = query.trim();
    if (!q) return [];
    const chosen = new Set(value);
    const lower = q.toLowerCase();
    const matches = suggestions
      .filter((s) => !chosen.has(s) && s.toLowerCase().includes(lower))
      .map((s) => ({ value: s, create: false }));
    const exact = suggestions.includes(q) || chosen.has(q);
    if (allowCreate && !exact) matches.push({ value: q, create: true });
    return matches;
  }, [query, suggestions, value, allowCreate]);

  async function add(raw: string[], create: boolean) {
    const next = [...value];
    for (const v of raw) {
      const msg = validate?.(v) ?? null;
      if (msg) {
        setError(msg);
        return;
      }
      if (next.includes(v)) continue;
      const known = suggestions.includes(v);
      if (!known && !allowCreate) return;
      if (!known && create && onCreate && !(await onCreate(v))) return;
      next.push(v);
    }
    setError("");
    setQuery("");
    setActive(0);
    if (next.length !== value.length) onChange(next);
  }

  function pick(o: Option) {
    void add([o.value], o.create);
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setActive((i) => Math.min(i + 1, Math.max(options.length - 1, 0)));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const o = options[active];
      if (o) pick(o);
    } else if (e.key === "Escape") {
      setOpen(false);
    } else if (e.key === "Backspace" && query === "" && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  }

  function onPaste(e: ClipboardEvent<HTMLInputElement>) {
    const parts = e.clipboardData.getData("text").split(/\s+/).filter(Boolean);
    if (parts.length < 2) return;
    e.preventDefault();
    void add(parts, true);
  }

  function onWrapperBlur(e: FocusEvent<HTMLDivElement>) {
    if (e.currentTarget.contains(e.relatedTarget as Node | null)) return;
    setOpen(false);
    onBlur?.();
  }

  const showList = open && options.length > 0;

  return (
    <div className="token-picker" onBlur={onWrapperBlur}>
      <div className="token-picker-field" onClick={() => inputRef.current?.focus()}>
        {value.map((v) => (
          <span key={v} className="token-chip">
            {v}
            <button
              type="button"
              className="token-chip-remove"
              aria-label={`Remove ${v}`}
              onClick={() => onChange(value.filter((x) => x !== v))}
            >
              ×
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          className="token-picker-input"
          role="combobox"
          aria-label={label}
          aria-expanded={showList}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={showList ? `${listId}-${active}` : undefined}
          aria-describedby={error ? errorId : undefined}
          value={query}
          placeholder={value.length === 0 ? placeholder : undefined}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            setActive(0);
            setError("");
          }}
          onKeyDown={onKeyDown}
          onPaste={onPaste}
        />
      </div>
      {showList && (
        <ul id={listId} role="listbox" className="token-picker-list">
          {options.map((o, i) => (
            <li
              key={`${o.create ? "new:" : ""}${o.value}`}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={i === active}
              className={i === active ? "token-option token-option-active" : "token-option"}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => pick(o)}
            >
              {o.create ? `Create "${o.value}"` : o.value}
            </li>
          ))}
        </ul>
      )}
      {error && (
        <p id={errorId} role="alert" className="token-picker-error">
          {error}
        </p>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Add the styles**

Append to `App.css` after the `.multiselect` block. Check each `var(--…)` name exists in `style.css` before using it; swap for the nearest existing token if one does not.

```css
/* TokenPicker */
.token-picker {
  position: relative;
}
.token-picker-field {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1, 4px);
  align-items: center;
  padding: var(--space-1, 4px);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm, 4px);
  background: var(--surface);
  cursor: text;
}
.token-picker-field:focus-within {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
}
.token-chip {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1, 4px);
  padding: 0 var(--space-2, 6px);
  border-radius: var(--radius-sm, 4px);
  background: var(--surface-2);
  font-size: var(--font-sm);
}
.token-chip-remove {
  border: 0;
  background: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0;
}
.token-picker-input {
  flex: 1;
  min-width: 8ch;
  border: 0;
  background: transparent;
  color: inherit;
  outline: none;
}
.token-picker-list {
  position: absolute;
  z-index: 5;
  left: 0;
  right: 0;
  margin: 2px 0 0;
  padding: var(--space-1, 4px) 0;
  list-style: none;
  max-height: 14rem;
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm, 4px);
  background: var(--surface);
}
.token-option {
  padding: var(--space-1, 4px) var(--space-2, 6px);
  cursor: pointer;
}
.token-option-active {
  background: var(--surface-2);
}
.token-picker-error {
  margin: var(--space-1, 4px) 0 0;
  color: var(--danger);
  font-size: var(--font-sm);
}
/* .bulk-row input gives every input in a bulk row a border and full width;
   the picker's inner input sits inside the picker's own border. */
.bulk-row .token-picker-input {
  border: 0;
  width: auto;
}
.bulk-row .token-picker {
  flex: 1;
}
```

- [ ] **Step 5: Run the tests and the lints**

Run from `xtm/frontend`: `npx vitest run src/components/TokenPicker.test.tsx`
Run from the repo root: `npm run lint` then `bash scripts/ratchet.sh`
Expected: tests PASS; no ratchet counter rises (watch `eslint_a11y`, `hardcoded_hex`, `hardcoded_px_font`, `ui_em_dashes`). If `hardcoded_px_font` rises, the `font-size` tokens above are wrong; use the token `style.css` defines.

- [ ] **Step 6: Commit**

```bash
git add xtm/frontend/src/components/TokenPicker.tsx xtm/frontend/src/components/TokenPicker.test.tsx xtm/frontend/src/App.css
git commit -m "feat(xtm): add a token picker with suggestions and inline create (#<issue>)"
```

---

### Task 4: Picker in TestDetail and BulkEditModal

**Files:**
- Create: `xtm/frontend/src/components/LabelsField.tsx`
- Create: `xtm/frontend/src/components/LabelsField.test.tsx`
- Modify: `xtm/frontend/src/components/TestDetail.tsx` (the Labels `<dd>` uses `LabelsField`)
- Modify: `xtm/frontend/src/components/BulkEditModal.tsx` (the value input for the Labels field)
- Create: `xtm/frontend/src/components/BulkEditModal.test.tsx`

`TestDetail` has no test file and is 2,675 lines with many providers, so the label control is pulled out into `LabelsField` and tested on its own. That also takes a little weight off `TestDetail`.

**Interfaces:**
- Consumes: `TokenPicker`, `validateLabel` (Task 3); `useLabels` (Task 2).
- Produces:

```ts
interface LabelsFieldProps {
  profileId: string;
  value: string;            // space-joined, as TestDetail stores it
  onChange: (v: string) => void;
  onSave: () => void;       // called when focus leaves the picker
  readOnly: boolean;
}
export function LabelsField(props: LabelsFieldProps): JSX.Element;
```

- [ ] **Step 1: Write the failing BulkEditModal test**

`BulkEditModal.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BulkEditModal } from "./BulkEditModal";

const bulkEdit = vi.fn();

vi.mock("../api", () => ({
  BulkEditTests: (...a: unknown[]) => bulkEdit(...a),
  ListLabels: vi.fn(async () => [
    { label: "smoke", count: 2 },
    { label: "login", count: 1 },
  ]),
  errMsg: (e: unknown) => String(e),
}));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

function renderModal() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <BulkEditModal testKeys={["QA-1", "QA-2"]} onComplete={() => {}} onCancel={() => {}} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  bulkEdit.mockReset();
  bulkEdit.mockResolvedValue({ succeeded: ["QA-1", "QA-2"], failed: [] });
});

describe("BulkEditModal labels", () => {
  it("adds several labels picked from suggestions in one call", async () => {
    renderModal();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Field" }), "labels");
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Operation" }), "add_label");
    const picker = screen.getByRole("combobox", { name: "Labels" });
    await userEvent.type(picker, "smo");
    await userEvent.click(await screen.findByRole("option", { name: "smoke" }));
    await userEvent.type(picker, "fresh{Enter}");
    await userEvent.click(screen.getByRole("button", { name: /apply/i }));
    expect(bulkEdit).toHaveBeenCalledWith("p1", ["QA-1", "QA-2"], {
      operation: "add_label",
      field: "labels",
      value: "smoke fresh",
    });
  });
});
```

The `<select>`s in `BulkEditModal` sit inside `<label>` elements with "Field" and "Operation" text, which gives them those accessible names. If the Apply button's text differs, match the text in the file.

- [ ] **Step 2: Write the failing LabelsField test**

`LabelsField.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { LabelsField } from "./LabelsField";

vi.mock("../api", () => ({
  ListLabels: vi.fn(async () => [
    { label: "smoke", count: 2 },
    { label: "login", count: 1 },
  ]),
}));

function Harness({ readOnly = false, onSave = vi.fn(), seen = vi.fn() }) {
  const [value, setValue] = useState("smoke");
  seen(value);
  return (
    <LabelsField
      profileId="p1"
      value={value}
      onChange={setValue}
      onSave={() => onSave(value)}
      readOnly={readOnly}
    />
  );
}

function renderField(props: Parameters<typeof Harness>[0] = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Harness {...props} />
      <button>elsewhere</button>
    </QueryClientProvider>,
  );
}

describe("LabelsField", () => {
  it("adds a suggested label and saves the joined value on blur", async () => {
    const onSave = vi.fn();
    renderField({ onSave });
    const picker = screen.getByRole("combobox", { name: "Labels" });
    await userEvent.type(picker, "log");
    await userEvent.click(await screen.findByRole("option", { name: "login" }));
    await userEvent.click(screen.getByRole("button", { name: "elsewhere" }));
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith("smoke login");
  });

  it("read-only shows the labels as text with no picker", () => {
    renderField({ readOnly: true });
    expect(screen.getByText("smoke").tagName).toBe("SPAN");
    expect(screen.queryByRole("combobox", { name: "Labels" })).toBeNull();
  });
});
```

- [ ] **Step 3: Run them to see them fail**

Run from `xtm/frontend`: `npx vitest run src/components/BulkEditModal.test.tsx src/components/LabelsField.test.tsx`
Expected: FAIL, no combobox named "Labels" in the modal, and `./LabelsField` not found.

- [ ] **Step 4: Use the picker in BulkEditModal**

In `BulkEditModal.tsx`:
- Import `TokenPicker`, `validateLabel` from `./TokenPicker` and `useLabels` from `../queries/app`.
- Add `const { data: labelBuckets = [] } = useLabels(profileId);` and `const labelSuggestions = labelBuckets.map((b) => b.label);`.
- Before the `useTextarea ? …` branch of the Value control, add a branch for `field === "labels"`:

```tsx
              ) : field === "labels" ? (
                <TokenPicker
                  label="Labels"
                  value={value.split(/\s+/).filter(Boolean)}
                  onChange={(next) => setValue(next.join(" "))}
                  suggestions={labelSuggestions}
                  allowCreate={operation !== "remove_label"}
                  validate={validateLabel}
                  placeholder="Type to search or create"
                />
```

- Change `useTextarea` to `field === "description"`.
- In `apply()`, keep the empty-value check limited to `add_label` and `remove_label`, and change its text to "Pick at least one label." "Replace all" with nothing picked stays allowed, because clearing every label is a real action.
- With the picker inside a `<label>` element, the outer `<label>` would steal clicks. Change the Value row's wrapper from `<label className="bulk-row bulk-row-value">` to `<div className="bulk-row bulk-row-value">` and give the `<span>Value</span>` sibling no `htmlFor`; the picker carries its own `aria-label`.

- [ ] **Step 5: Write LabelsField and use it in TestDetail**

`LabelsField.tsx`:

```tsx
import { TokenPicker, validateLabel } from "./TokenPicker";
import { useLabels } from "../queries/app";

interface LabelsFieldProps {
  profileId: string;
  value: string;
  onChange: (v: string) => void;
  onSave: () => void;
  readOnly: boolean;
}

// LabelsField is the test detail's label control. It keeps the space-joined
// string TestDetail stores, so saveField and the dirty marker are unchanged.
export function LabelsField({ profileId, value, onChange, onSave, readOnly }: LabelsFieldProps) {
  const { data: buckets = [] } = useLabels(profileId);
  if (readOnly) return <span>{value || "—"}</span>;
  return (
    <TokenPicker
      label="Labels"
      value={value.split(/\s+/).filter(Boolean)}
      onChange={(next) => onChange(next.join(" "))}
      onBlur={onSave}
      suggestions={buckets.map((b) => b.label)}
      allowCreate
      validate={validateLabel}
      placeholder="Type to search or create"
    />
  );
}
```

The "—" placeholder for no labels is what `TestDetail` shows today. It is not a sentence, so the em-dash ratchet counts it either way; check `bash scripts/ratchet.sh` does not rise, and if it does, the old occurrence in `TestDetail` went away in the same change and the count holds.

In `TestDetail.tsx`, replace the whole Labels `<dd>` body (the `readOnly ? <span> : <input>` expression) with:

```tsx
              <LabelsField
                profileId={profileId}
                value={labels}
                onChange={setLabels}
                onSave={() => saveField("labels", labels)}
                readOnly={readOnly}
              />
```

`onSave` closes over `labels` from the latest render. Picking an option updates state and re-renders before focus can leave, so the blur sees the new value. `saveField` returns early when the value equals the backend's, so an unchanged blur queues nothing.

- [ ] **Step 6: Run the tests**

Run from `xtm/frontend`: `npx vitest run`
Expected: PASS, including the existing TestDetail tests.

- [ ] **Step 7: Commit**

```bash
git add xtm/frontend/src/components/TestDetail.tsx xtm/frontend/src/components/LabelsField.tsx xtm/frontend/src/components/LabelsField.test.tsx xtm/frontend/src/components/BulkEditModal.tsx xtm/frontend/src/components/BulkEditModal.test.tsx
git commit -m "feat(xtm): pick labels from suggestions in test detail and bulk edit (#<issue>)"
```

---

### Task 5: Bulk Labels modal and toolbar button

**Files:**
- Create: `xtm/frontend/src/components/BulkLabelsModal.tsx`
- Create: `xtm/frontend/src/components/BulkLabelsModal.test.tsx`
- Modify: `xtm/frontend/src/contexts/ModalContext.tsx` (add `"bulkLabels"` to the modal name union)
- Modify: `xtm/frontend/src/App.tsx` (toolbar button after "Bulk edit…", render block after the `bulkEdit` block)
- Modify: `xtm/internal/testrepo/labels.go`, `labels_test.go` (add `ListTestLabels`)
- Modify: `xtm/app.go`, regenerated `wailsjs`, `xtm/frontend/src/api.ts` (expose it)

No CSS changes: the modal reuses `bulk-modal`, `pending-head`, `bulk-body`, `bulk-row`, `bulk-preview`, `error-text`, `commit-fail-list`, `mono` and `pending-actions`, which `BulkEditModal` already uses.

**Interfaces:**
- Consumes: `BulkEditLabels`, `useLabels` (Task 2); `TokenPicker`, `validateLabel` (Task 3).
- Produces: `Repository.ListTestLabels(profileID string, testKeys []string) (map[string][]string, error)`, `App.ListTestLabels` with the same signature, TS export `ListTestLabels`. The preview needs each selected test's current labels, and no existing read returns labels by key.
- Produces: `BulkLabelsModal({ testKeys, onComplete, onCancel })`, the same props as `BulkEditModal`; `countChanged(current, add, remove): number`.

- [ ] **Step 1: Write the failing Go test for ListTestLabels**

Append to `labels_test.go`:

```go
func TestListTestLabelsReturnsRequestedKeys(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	got, err := repo.ListTestLabels(lblProfile, []string{"QA-1", "QA-4", "QA-99"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := map[string][]string{"QA-1": {"smoke", "login"}, "QA-4": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 2: Write the failing modal tests**

`BulkLabelsModal.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BulkLabelsModal } from "./BulkLabelsModal";

const bulkLabels = vi.fn();

vi.mock("../api", () => ({
  BulkEditLabels: (...a: unknown[]) => bulkLabels(...a),
  ListLabels: vi.fn(async () => [
    { label: "smoke", count: 2 },
    { label: "login", count: 1 },
  ]),
  ListTestLabels: vi.fn(async () => ({
    "QA-1": ["smoke", "login"],
    "QA-2": ["smoke"],
    "QA-3": [],
  })),
  errMsg: (e: unknown) => String(e),
}));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

function renderModal(onComplete = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <BulkLabelsModal testKeys={["QA-1", "QA-2", "QA-3"]} onComplete={onComplete} onCancel={() => {}} />
    </QueryClientProvider>,
  );
  return onComplete;
}

async function pick(name: string, text: string) {
  await userEvent.type(screen.getByRole("combobox", { name }), `${text}{Enter}`);
}

beforeEach(() => {
  bulkLabels.mockReset();
  bulkLabels.mockResolvedValue({ succeeded: ["QA-1", "QA-2", "QA-3"], failed: [] });
});

describe("BulkLabelsModal", () => {
  it("previews how many selected tests change", async () => {
    renderModal();
    await pick("Remove labels", "login");
    expect(await screen.findByText("1 of 3 selected tests will change.")).toBeTruthy();
    await pick("Add labels", "smoke");
    expect(await screen.findByText("2 of 3 selected tests will change.")).toBeTruthy();
  });

  it("sends add and remove lists and completes", async () => {
    const onComplete = renderModal();
    await pick("Add labels", "regression");
    await pick("Remove labels", "smoke");
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(bulkLabels).toHaveBeenCalledWith("p1", ["QA-1", "QA-2", "QA-3"], ["regression"], ["smoke"]);
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it("refuses a label in both lists without calling the backend", async () => {
    renderModal();
    await pick("Add labels", "smoke");
    await pick("Remove labels", "smoke");
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(screen.getByText('"smoke" is in both Add and Remove.')).toBeTruthy();
    expect(bulkLabels).not.toHaveBeenCalled();
  });

  it("lists failed tests and stays open", async () => {
    bulkLabels.mockResolvedValue({
      succeeded: ["QA-1"],
      failed: [{ testKey: "QA-2", error: "not found" }],
    });
    const onComplete = renderModal();
    await pick("Add labels", "x");
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(await screen.findByText("QA-2: not found")).toBeTruthy();
    expect(onComplete).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 3: Run them to see them fail**

Run from `xtm/`: `go test ./internal/testrepo/ -run ListTestLabels -count=1` (build failure)
Run from `xtm/frontend`: `npx vitest run src/components/BulkLabelsModal.test.tsx` (module not found)

- [ ] **Step 4: Add ListTestLabels**

Append to `labels.go`:

```go
// ListTestLabels returns the labels of each requested Test, keyed by Jira key,
// for the Bulk Labels preview. Keys the profile does not have are omitted.
func (r *Repository) ListTestLabels(profileID string, testKeys []string) (map[string][]string, error) {
	out := make(map[string][]string, len(testKeys))
	for _, key := range testKeys {
		var stored string
		err := r.db.QueryRow(
			`SELECT labels FROM test_case WHERE profile_id = ? AND jira_key = ?`,
			profileID, key,
		).Scan(&stored)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("list test labels %s: %w", key, err)
		}
		labels := strings.Fields(stored)
		if labels == nil {
			labels = []string{}
		}
		out[key] = labels
	}
	return out, nil
}
```

In `app.go`, after `ListLabels`:

```go
// ListTestLabels returns the labels of each given Test, for the Bulk Labels
// preview.
func (a *App) ListTestLabels(profileID string, testKeys []string) (map[string][]string, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListTestLabels(profileID, testKeys)
}
```

Regenerate with `wails generate module` from `xtm/` (same churn check as Task 2), and add `ListTestLabels,` to `api.ts`.

- [ ] **Step 5: Implement BulkLabelsModal**

`BulkLabelsModal.tsx`:

```tsx
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useProfile } from "../contexts/ProfileContext";
import { Modal } from "./Modal";
import { TokenPicker, validateLabel } from "./TokenPicker";
import { useLabels } from "../queries/app";
import { BulkEditLabels, ListTestLabels, errMsg } from "../api";
import type { BulkEditResult } from "../api";
import { call } from "../lib/apiCall";

interface Props {
  testKeys: string[];
  onComplete: (result: BulkEditResult) => void;
  onCancel: () => void;
}

// countChanged reports how many tests a given add/remove would change, using
// the same rule the backend applies: remove first, then add what is missing.
export function countChanged(
  current: Record<string, string[]>,
  add: string[],
  remove: string[],
): number {
  const drop = new Set(remove);
  let n = 0;
  for (const labels of Object.values(current)) {
    const kept = labels.filter((l) => !drop.has(l));
    const next = [...kept, ...add.filter((l) => !kept.includes(l))];
    if (next.join(" ") !== labels.join(" ")) n++;
  }
  return n;
}

export function BulkLabelsModal({ testKeys, onComplete, onCancel }: Props) {
  const { activeId: profileId } = useProfile();
  const [add, setAdd] = useState<string[]>([]);
  const [remove, setRemove] = useState<string[]>([]);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<BulkEditResult | null>(null);

  const { data: buckets = [] } = useLabels(profileId);
  const suggestions = buckets.map((b) => b.label);
  const { data: current } = useQuery({
    queryKey: [profileId, "tests", "labels", testKeys],
    queryFn: () => call(() => ListTestLabels(profileId, testKeys)),
    enabled: !!profileId,
  });

  const changed = useMemo(
    () => (current ? countChanged(current, add, remove) : null),
    [current, add, remove],
  );
  const overlap = add.find((l) => remove.includes(l));

  async function apply() {
    if (overlap) {
      setError(`"${overlap}" is in both Add and Remove.`);
      return;
    }
    if (add.length === 0 && remove.length === 0) {
      setError("Pick at least one label to add or remove.");
      return;
    }
    setApplying(true);
    setError("");
    try {
      const r = await BulkEditLabels(profileId, testKeys, add, remove);
      setResult(r);
      if (r.failed.length === 0) onComplete(r);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setApplying(false);
    }
  }

  const n = testKeys.length;
  return (
    <Modal onClose={onCancel} className="modal bulk-modal" labelledBy="bulk-labels-title">
      <div className="pending-head">
        <h2 id="bulk-labels-title">
          Labels ({n} {n === 1 ? "test" : "tests"})
        </h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close">
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <div className="bulk-row bulk-row-value">
          <span>Add</span>
          <TokenPicker
            label="Add labels"
            value={add}
            onChange={setAdd}
            suggestions={suggestions}
            allowCreate
            validate={validateLabel}
            placeholder="Type to search or create"
          />
        </div>
        <div className="bulk-row bulk-row-value">
          <span>Remove</span>
          <TokenPicker
            label="Remove labels"
            value={remove}
            onChange={setRemove}
            suggestions={suggestions}
            allowCreate={false}
            validate={validateLabel}
            placeholder="Type to search"
          />
        </div>
        {changed !== null && (
          <p className="muted bulk-preview">
            {changed} of {n} selected tests will change.
          </p>
        )}
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

The test fixture's "Remove labels" picker has `allowCreate={false}`, and "login" is in the suggestions, so Enter adds it.

- [ ] **Step 6: Wire the modal into App**

`ModalContext.tsx`: add `| "bulkLabels"` after `| "bulkEdit"`.

`App.tsx`, in the bulk toolbar after the "Bulk edit…" button:

```tsx
          <button
            className="btn btn-primary"
            onClick={() => openModal("bulkLabels")}
          >
            Labels…
          </button>
```

After the `isOpen("bulkEdit")` block:

```tsx
      {isOpen("bulkLabels") && (
        <BulkLabelsModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}
```

and import `BulkLabelsModal` beside `BulkEditModal`.

- [ ] **Step 7: Run everything this touches**

Run from `xtm/`: `go test ./internal/testrepo/ -count=1 && go vet ./...`
Run from `xtm/frontend`: `npx vitest run`
Run from the repo root: `npm run typecheck --workspaces --if-present && npm run lint && bash scripts/ratchet.sh`
Expected: all PASS; no ratchet counter rises.

- [ ] **Step 8: Commit**

```bash
git add xtm/internal/testrepo/labels.go xtm/internal/testrepo/labels_test.go xtm/app.go xtm/frontend/wailsjs/go/main/App.js xtm/frontend/wailsjs/go/main/App.d.ts xtm/frontend/src
git commit -m "feat(xtm): add a Bulk Labels action that adds and removes labels (#<issue>)"
```

---

### Task 6: User guide, full gates, PR

**Files:**
- Modify: `docs/user-guide/USER_GUIDE.md` (the section on editing tests and the bulk actions list)

- [ ] **Step 1: Write the user-guide text**

Find the bulk actions section (`grep -n -i "bulk" docs/user-guide/USER_GUIDE.md`). Add a "Labels" entry describing: the Labels… button appears when tests are selected; Add takes existing or new labels, Remove takes existing ones; the preview line counts the tests that change; changes are queued as pending and reach Jira on commit; a label cannot contain spaces. In the test detail section, replace the "space-separated" wording with a description of the picker: type to filter, Enter or click to pick, Enter on an unknown label creates it, × or Backspace removes one. Write it with the humanizer skill in file mode, per the prose rule in `AGENTS.project.md`.

- [ ] **Step 2: Run the full gate set**

Run from the repo root: `make gates`
Expected: PASS. Read any failure and fix the cause (P4); do not commit after a failed gate.

- [ ] **Step 3: Commit the docs**

```bash
git add docs/user-guide/USER_GUIDE.md
git commit -m "docs(xtm): describe the label picker and Bulk Labels (#<issue>)"
```

- [ ] **Step 4: Open the PR**

Push the branch and open a PR against `main` whose body quotes the part A issue's acceptance lines, links the spec under `## Spec`, and lists the gates run. No AI attribution in the body or commits.
