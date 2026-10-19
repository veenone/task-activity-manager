# XTM Dataset Macro Helper Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Insert and validate `${PARAMETER}` references at the caret in each editable manual-step field.

**Architecture:** Add a generic insertion handle to `MarkdownField`, then put dataset trigger and suggestion behavior in a focused wrapper. Share the Test default dataset through a query hook so the dataset section and step editor cannot race separate fetches.

**Tech Stack:** React 19, TypeScript, TanStack Query, Vitest, Testing Library, Wails v2.

**Spec:** `docs/superpowers/specs/2026-10-19-xtm-parameterized-dataset-execution-design.md`

## Global Constraints

- Helper applies only to Xray object-step Action, Data and Expected fields.
- Use the Test default dataset, not a Plan or Execution override.
- Support button, `${` typing, mouse and keyboard selection, Escape, and focus restoration.
- Preserve undefined macros and explain them in text as well as color.
- Add no dependency; reuse shared menu/focus primitives and existing design tokens.
- Start each behavior with a failing test and commit each task with #301.

## Review Focus

- Inserting over a selected substring replaces only that selection; Task 1 pins selection boundaries.
- Clicking the suggestion menu does not trigger a blur save before insertion; Task 2 tests one save with the final value.
- A parameter name containing spaces inserts unchanged inside braces; Task 2 covers `${ACCOUNT REGION}`.
- Switching Tests while a dataset read is pending cannot show the old Test's macros; Task 3 tests query keys and cancellation.
- A removed parameter leaves an undefined reference visible and keyboard-readable; Task 2 covers rendered output.

---

### Task 1: Generic caret insertion contract

**Files:**
- Modify: `xtm/frontend/src/components/MarkdownField.tsx`
- Create: `xtm/frontend/src/components/MarkdownField.test.tsx`

**Interfaces:**
- Produces: `MarkdownFieldHandle` with `insertText(text: string, range?: { start: number; end: number }): void` and `focus(): void`; `MarkdownField` uses `forwardRef` without changing existing props.

- [ ] **Step 1: Write failing handle tests**

Assert insertion at the caret, replacement of a selection, caret placement after inserted text, focus restoration, and no `onCommit` until the final blur.

- [ ] **Step 2: Run the test and record red**

Run: `npm test -w xtm/frontend -- MarkdownField.test.tsx`

Expected: FAIL because no imperative handle exists.

- [ ] **Step 3: Implement `MarkdownFieldHandle`**

Use the native selection range from the active input/textarea. Update through `onChange` semantics, then restore focus and the new caret after React renders.

- [ ] **Step 4: Run the focused test**

Run: `npm test -w xtm/frontend -- MarkdownField.test.tsx`

Expected: PASS.

- [ ] **Step 5: Run the XTM frontend typecheck**

Run: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add xtm/frontend/src/components/MarkdownField.tsx xtm/frontend/src/components/MarkdownField.test.tsx
git commit -m "feat(xtm): expose markdown caret insertion (#301)"
```

### Task 2: Dataset-aware macro field

**Files:**
- Create: `xtm/frontend/src/components/test-detail/DatasetMacroField.tsx`
- Create: `xtm/frontend/src/components/test-detail/DatasetMacroField.test.tsx`
- Create: `xtm/frontend/src/components/test-detail/StepRow.test.tsx`
- Create: `xtm/frontend/src/components/test-detail/dataset-macro.css`
- Modify: `xtm/frontend/src/components/test-detail/StepRow.tsx`

**Interfaces:**
- Consumes: `MarkdownFieldHandle`, `DatasetParameter[]`, and the existing MarkdownField value/commit props.
- Produces: `DatasetMacroField({ parameters, value, onChange, onCommit, ...fieldProps })`; undefined-reference rendering helper `renderDatasetReferences(value, parameterNames)`.

- [ ] **Step 1: Write failing interaction tests**

Cover toolbar insertion at selection, suggestions after `${`, prefix filtering, arrow/Enter selection, Escape, `${ACCOUNT REGION}`, one final commit, and an undefined `${MISSING}` exposed with accessible text.

- [ ] **Step 2: Run focused tests to prove red**

Run: `npm test -w xtm/frontend -- DatasetMacroField.test.tsx`

Expected: FAIL because the component does not exist.

- [ ] **Step 3: Implement the wrapper and use it in all three fields**

Keep trigger parsing and menu state in the wrapper. Replace only the active partial macro. In `StepRow`, use it for Action, Data and Expected; leave call-test rows and read-only fields unchanged.

- [ ] **Step 4: Run macro and step tests**

Run: `npm test -w xtm/frontend -- DatasetMacroField.test.tsx StepRow.test.tsx`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add xtm/frontend/src/components/test-detail
git commit -m "feat(xtm): insert dataset macros in steps (#301)"
```

### Task 3: Share the default dataset query with steps

**Files:**
- Modify: `xtm/frontend/src/queries/datasets.ts`
- Modify: `xtm/frontend/src/queries/datasets.test.tsx`
- Modify: `xtm/frontend/src/components/test-detail/DatasetSection.tsx`
- Modify: `xtm/frontend/src/components/test-detail/useTestDetailState.ts`
- Modify: `xtm/frontend/src/components/test-detail/StepsSection.tsx`
- Create: `xtm/frontend/src/components/test-detail/StepsSection.test.tsx`
- Modify: `xtm/frontend/src/components/test-detail/DatasetEditor.tsx`

**Interfaces:**
- Produces: `useTestDataset(profileId: string, testKey: string, enabled: boolean)` keyed by `['test-dataset', profileId, testKey]`; successful editor Save updates that cache.
- Consumes: `DatasetMacroField(parameters=dataset.parameters)`.

- [ ] **Step 1: Write failing query and panel tests**

Assert disabled queries make no call, profile/Test identity is in the key, a late QA-1 response cannot seed QA-2, Save updates the shared cache, and Steps receives only the current Test's parameters.

- [ ] **Step 2: Run focused tests to prove red**

Run: `npm test -w xtm/frontend -- datasets.test.tsx DatasetSection.test.tsx StepsSection.test.tsx`

Expected: FAIL on the missing shared hook.

- [ ] **Step 3: Refactor reads onto the query hook**

Keep execution override loading separate because macro authoring deliberately uses only the default. Invalidate or patch the exact default key after Save and refresh.

- [ ] **Step 4: Run the XTM frontend suite**

Run: `npm test -w xtm/frontend`

Expected: PASS without lowering the test count.

- [ ] **Step 5: Run the XTM frontend typecheck**

Run: `npm run typecheck -w xtm/frontend`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add xtm/frontend/src/queries/datasets* xtm/frontend/src/components/test-detail
git commit -m "refactor(xtm): share test dataset state (#301)"
```

