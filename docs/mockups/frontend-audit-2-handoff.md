# Frontend audit handoff (take 2)

Audit date: 2026-10-10. Scope: `frontend/core`, `tam/frontend`, `xtm/frontend`,
their stylesheets, and the mockups in `docs/mockups/agile-suite-frontends.pen`.
The visual version of this file is the frame `2. Frontend Audit` in that .pen.

Read `AGENTS.md` and `AGENTS.project.md` first. Every task below names the
rule it serves (C2, C3, I3 and so on) and the gate that measures it. Run the
gates for what you touch before each commit, and the full set before a push.

## How the numbers were taken

Counts come from `grep`, `wc -l`, `eslint-report.json` and `.ratchet-counters`
on the working tree at `main` (8bcc0b9). Comment stripping follows the
ratchet's own perl pass for hex and px counts, so a raw grep runs a little
higher than the baseline. Re-run before you quote a number in a PR.

| Measure | Value |
|---|---|
| CSS lines | 11,349 total, 8,212 of them in `xtm/frontend/src/App.css` |
| Hex literals in app CSS | XTM 439, TAM 0, `primitives.css` 47 |
| `font-size: Npx` literals | XTM 396, TAM 58, `primitives.css` 68 |
| `[data-theme="dark"]` overrides in app CSS | XTM 28, TAM 0 |
| Core stylesheets loaded | TAM both, XTM none |
| Source files over 400 lines (non-test) | 27; `TestDetail.tsx` 2,702, `ContainersView.tsx` 1,969, `App.tsx` 1,774 |
| ESLint problems | 196, of which `jsx-a11y` 95 |
| Files with bespoke modal markup | 38, all in XTM |
| Component test files vs component files | XTM 11 of 81, TAM 37 of 75 |
| Mockup nodes | 19,247; 0 components, 0 variables, 2,251 named `div`/`header`/`main`/`footer` |

## Status on 2026-10-15

Waves 0 to 3 are done; T4.1 (#248) and T4.2 (#249) are in review. The plan
and its progress chart live in Outline, collection "Frontend Audit". The same
measures, taken the same way on `main` (75a875b):

| Measure | Audit | Now |
|---|---|---|
| Hex literals in XTM CSS | 439 | 30, all commented dark overrides (#215, #216) |
| `font-size: Npx` literals in XTM CSS | 396 | 0, on the type scale (#218) |
| `jsx-a11y` findings | 95 | 0 (#194, #220) |
| Files with bespoke modal markup | 38 | 0, core `Modal` everywhere (#200, #201) |
| XTM source files over 400 lines (non-test) | 27 | 11; largest `api.ts` 1,730, largest component 872 (#233 to #247) |
| Component test files vs component files | XTM 11 of 81, TAM 37 of 75 | XTM 28 of 116, TAM 42 of 74 |
| Mockup nodes | 19,247, 0 components, 0 variables | 13 components and 153 variables; screens built from instances (#248) |
| Dialogs drawn in the mockups | XTM 7 of 36, TAM 3 of 15 | XTM 16 of 36, TAM 9 of 15 (#249) |

Open decision 1 is settled: T1.3 added a 10px `--fs-2xs` step for dense
tables rather than an 11px floor. Decisions 2 and 3 are still open.

## 1. Improvement points

| ID | Improvement | Where | Effort | Wave |
|---|---|---|---|---|
| IP-01 | Make each ratchet counter measure what its name says | `.ratchet-counters` | S | 0 |
| IP-02 | Define the custom properties XTM uses but nobody declares, and gate it | `xtm/.../App.css`, `tokens.css` | S | 0 |
| IP-03 | Load the core stylesheets in XTM and delete its hand-kept copies | `xtm/frontend/src/main.tsx`, `App.css` | M | 1 |
| IP-04 | Move XTM's 439 hex literals onto tokens, then drop the 28 dark overrides | `xtm/.../App.css` | L | 1 |
| IP-05 | Add a type scale and a z-index scale as tokens | `tokens.css` | M | 1 |
| IP-06 | Replace the 38 bespoke modals with core `Modal` | 38 XTM components | L | 2 |
| IP-07 | Clear the 95 `jsx-a11y` findings and label tables and icon buttons | XTM mostly | L | 2 |
| IP-08 | Shared focus ring, reduced-motion and breakpoints for XTM | `primitives.css`, `App.css` | M | 2 |
| IP-09 | Move the five duplicated app components into core | `AboutModal`, `DiagnosticsModal`, `ProfilesModal`, `ProfileForm`, `PendingChangesModal` | L | 3 |
| IP-10 | Split the over-long XTM files | see T3.2 | L | 3 |
| IP-11 | Raise XTM component test coverage where files are touched | `xtm/frontend` | M | 3 |
| IP-12 | Turn the .pen into a system: variables, components, real node names | `agile-suite-frontends.pen` | M | 4 |

## 2. Gap analysis

| Area | Today | Target | Gap |
|---|---|---|---|
| Colour | XTM writes hex in rules and re-colours it per theme in 28 override blocks | Every colour is a token that both themes define | 439 literals, 28 overrides |
| Type | 18 distinct px sizes in XTM, 41 declarations at 9 or 10px, no size tokens | One scale in `tokens.css`, 11px floor | scale does not exist |
| Shared UI | Core is imported by 96 TAM files and 24 XTM files; 5 components exist twice (3,619 lines) | One implementation per component | 5 duplicates, 38 bespoke modals |
| Stylesheets | XTM loads no core stylesheet; `.modal`, `.sr-only` and the token blocks are copied by hand | One tokens file, one primitives file | XTM's light `:root` block has 25 properties, core has 174 |
| Accessibility | 28 `autoFocus`, 59 click handlers on non-interactive elements, 35 tables against 9 `caption` or `scope="col"` matches, 8 `outline: none` | 0 `jsx-a11y` findings, visible focus everywhere | 95 findings |
| Responsive and motion | XTM has 4 media queries and no reduced-motion rule over 8,212 lines | Core breakpoints and a reduced-motion block | XTM missing both |
| Gates | `hardcoded_px_font` reads TAM only; core CSS and core TSX sit outside `hardcoded_hex`, `ui_em_dashes`, `bespoke_modals` | Counters describe what they claim (M2) | 4 counters |
| Tests | XTM 11 test files beside 81 component files | Behaviour test beside each file that changes | `TestDetail`, `TestTable`, `App.tsx` have none |
| Mockups | 12 of 12 XTM views, 8 of 8 TAM views; XTM 7 of 36 dialog or panel components, TAM 3 of 15 | Every dialog a user can reach | 29 XTM and 12 TAM dialogs |
| Mockup build | Static HTML import, 98 distinct fills, Roboto where the app uses the system stack | Variables from `tokens.css`, reusable pieces | no components or variables |

## 3. Identified issues

Severity is the effect on a user or on a gate, not the effort to fix.

| ID | Sev | Issue | Evidence | Rule |
|---|---|---|---|---|
| ISS-01 | High | `window.confirm` used for a bulk environment replace; every other confirm goes through `useConfirm` | `xtm/frontend/src/components/ContainersView.tsx:369`; `usePrompt.tsx` records that `window.prompt` returns null in the webview | I2, Modals contract |
| ISS-02 | High | `hardcoded_px_font` counts TAM only, so XTM's 396 declarations are unmeasured; `primitives.css` (47 hex, 68 px) and core TSX are outside every counter | `.ratchet-counters` pathspecs | M2 |
| ISS-03 | High | Custom properties used and never defined: `--bg` (no fallback), `--bg-hover` (falls back to light `#f5f5f5`), `--muted`, `--hover-bg`, `--mono` | `App.css:7890`, `7903`, `1833`, `1855`, `6332`, `6391`, `7279` | Colour and type contract |
| ISS-04 | High | Dark mode in XTM depends on 28 per-selector overrides, so any new literal ships light-only | `grep -c 'data-theme="dark"\]' xtm/.../App.css` = 28 | Colour and type contract |
| ISS-05 | High | 28 `autoFocus` attributes and 59 clickable non-interactive elements | `eslint-report.json`; worst files by lint problems `ContainersView` (17), `App.tsx` (12), `RequirementsView` (11) | I3 |
| ISS-06 | Med | XTM carries its own copy of modal, token and `.sr-only` rules that drift from core | `App.css:3936`, `3950` against `primitives.css:186`, `200`; note at `App.css:69` | Modals contract |
| ISS-07 | Med | Text under 11px: 41 declarations in XTM CSS, 716 text nodes in the mockups | `font-size: 9px` x8, `10px` x33 | I3 |
| ISS-08 | Med | 8 `outline: none` in XTM CSS against 3 `:focus-visible` rules | `App.css:1519, 2065, 2282, 3286, 3390, 3393, 3692, 4722` | I3 |
| ISS-09 | Med | 35 tables, 9 matches for `caption` or `scope="col"` | grep over `*.tsx` | I3 |
| ISS-10 | Med | No `prefers-reduced-motion` rule in XTM for 12 transitions or animations | `App.css` | I3 |
| ISS-11 | Med | Five components exist in both apps and have diverged (`ProfileForm` 938 vs 592 lines, `PendingChangesModal` 681 vs 388) | `components/` in both apps | C1 |
| ISS-12 | Med | 27 source files over 400 lines | `files_over_400` | C2 |
| ISS-13 | Low | Em dashes in UI strings: 54 XTM files, 4 TAM files; 24 mockup text nodes | `ui_em_dashes` baseline 113 | UI copy contract |
| ISS-14 | Low | Close buttons named only by `title` in three XTM places, TAM uses `aria-label` | `ContainerDetailPanel.tsx:112`, `CoverageView.tsx:854`, `962` | I3 |
| ISS-15 | Low | Two warning families (`--warn-*`, `--warning-*`) and three aliases (`--card-border`, `--bg-subtle`, `--accent-strong`) | `tokens.css` | C1 |
| ISS-16 | Low | Mockups: empty 800x600 frame `bi8Au`, 446 nodes clipped by their parent, Roboto in 7,537 text nodes | `.pen` scan | none |

## 4. Implementation plan

Waves run in order. Inside a wave, tasks are independent unless a `Needs` line
says otherwise. One logical change per conventional commit (P5), each with a
failing test first where behaviour changes (P2). Raising a ratchet baseline
needs the reason in the commit message (C7); T0.1 is the one planned raise,
because the counters get wider.

### Wave 0: make the gates honest, fix the cheap defects

**T0.1 Widen the counters (IP-01, ISS-02).**
Edit `.ratchet-counters`.
- `hardcoded_px_font`: scan `tam/frontend/src/*.css` and `xtm/frontend/src/*.css`.
- `hardcoded_hex` and `hardcoded_px_font`: add `frontend/core/styles/primitives.css` (not `tokens.css`, which is where literals belong).
- `ui_em_dashes` and `bespoke_modals`: add `frontend/core/src/*.tsx` and `*.ts`.
- Run `bash scripts/ratchet.sh --update` and say in the message that the counters grew in scope, not that the code got worse.
Done when: the reported `hardcoded_px_font` is about 396 + 58 + 68 and each number can be reproduced with the command in the file.

**T0.2 Declare the missing properties and gate them (IP-02, ISS-03).**
- `--muted` becomes `var(--text-muted)` (lines 1833, 1855); `--bg` becomes `var(--surface)` (7890); `--bg-hover, #f5f5f5` becomes `var(--row-hover)` (7903); `--hover-bg` becomes `var(--ghost-hover)` (6332, 6391).
- Add `--font-mono` to `tokens.css` and use it where `--mono` is read.
- Drop the literal fallbacks beside it (`#f5f5f5`, `#4a90e2` at 7904).
- Add a case to `frontend/core/src/instruction-gate.test.ts`: every `var(--x)` in a tracked stylesheet resolves to a declaration in `tokens.css`, `primitives.css`, or an allowlist of names set from TSX (`--board-cols`, `--row-lead-depth`, `--type-col-w`, `--issue-key-w`, `--sprint-key-w`, `--epic-key-w`; confirm each by grep before listing it).
Test first: the gate case fails on the current tree, then passes.

**T0.3 One confirm path (ISS-01).**
Replace `window.confirm` at `ContainersView.tsx:369` with `useConfirm`, as lines 413 and 549 of the same file already do. The call sits in a synchronous guard, so the handler becomes async.
Test first: cancelling the dialog leaves the executions unchanged; accepting sets the environment on every selected key.

### Wave 1: XTM on the shared design language

**T1.1 Load the core stylesheets (IP-03, ISS-06).**
Needs T0.2.
- In `xtm/frontend/src/main.tsx` import `@agile-suite/core/styles/tokens.css` and `primitives.css` before `./App.css`, the way `tam/frontend/src/main.tsx:5-7` does.
- Diff XTM's `:root` blocks (App.css lines 3 to 63) against `tokens.css`. Where a value differs, that is a decision: list the differences in the PR, do not pick silently.
- Delete the rules core now provides: the two `:root` blocks, `.sr-only`, `.modal-overlay`, `.modal`, `.modal.profiles-modal`, then whatever the diff shows is identical.
Verify in demo mode (`agents/project/xtm-architecture.md`), light and dark, on Browse, Test detail, a Bulk dialog and Pending changes.

**T1.2 Hex to tokens (IP-04, ISS-04).**
Needs T1.1. Work through `App.css` by section, about 50 literals per commit.
1. Script a count of distinct values; most of the 439 repeat.
2. Map each to the nearest token; when none fits, add a token to `tokens.css` with a light and a dark value, never a literal in the rule.
3. When a rule had a `:root[data-theme="dark"]` sibling only to re-colour a literal, delete the sibling in the same commit.
Done when: `hardcoded_hex` is at or under the TAM figure plus the intentional literals, and the 28 overrides are gone or each has a comment saying why it stays.

**T1.3 Type and layer scales (IP-05, ISS-07).**
Proposed type tokens: `--fs-xs: 11px`, `--fs-sm: 12px`, `--fs-md: 13px` (TAM's body size), `--fs-lg: 15px`, `--fs-xl: 18px`, `--fs-2xl: 26px`. Map 9, 10, 10.5, 11 and 11.5 to xs; 12 and 12.5 to sm; 13 to md; 14 and 15 to lg; 16, 17 and 18 to xl; 26 to 2xl.
Proposed layer tokens: `--z-sticky`, `--z-menu`, `--z-modal`, `--z-toast`, replacing the literals 1, 2, 5, 20, 21, 40, 41, 60, 70 in app and core CSS.
**Decision for the owner before this lands (P7):** moving 9 and 10px text to 11px changes chip and table widths. Review the Browse table and the Coverage map for overflow.

### Wave 2: modals and accessibility

**T2.1 Core `Modal` everywhere (IP-06).**
Four batches, one PR each, deleting the matching CSS as you go:
- Bulk family: `BulkAllocateModal`, `BulkEditModal`, `BulkMoveModal`, `BulkPreconditionsModal`, `BulkRenameModal`, `BulkRequirementsModal`, `BulkReviewModal`, `BulkTransitionModal`.
- Pickers and links: `PickTestModal`, `PickPreconditionModal`, `LinkBugPicker`, `LinkRequirementsModal`, `CoverageTestPicker`, `AddTestsModal`, `CloneStepsModal`.
- Import and run: `ImportTestsModal`, `ImportRequirementsModal`, `JUnitImportModal`, `JUnitNewExecModal`, `CreateBugModal`, `RollupBreakdownModal`.
- Connection and system: `ConnectionsModal`, `ProfilesModal`, `SyncHistoryModal`, `DiagnosticsModal`, `PendingChangesModal`, `RequirementSourcesModal`, `BridgeWizard`.
- The remaining files in the `bespoke_modals` list (views and panels that embed a modal) go with the view they belong to in T3.2.
Done when: `bespoke_modals` falls by the batch size and `grep -c 'className="modal'` in the touched files is 0.

**T2.2 Focus without `autoFocus` (IP-07, ISS-05).**
Core `Modal` already owns initial focus; pass the control that should take it. For popovers that open on a click (`VersionBar` x4, `SearchableSelect`, `MultiSelect`, `MultiAddSelect`), focus the input from an effect that runs on open. The rule flags the attribute, not the behaviour.

**T2.3 Keyboard access for clickable rows (IP-07, ISS-05).**
Start from the rows that already take focus (`tbody tr:focus-visible`, `App.css:2805`) and copy their handler. Where a row has a natural primary control, put the click on a `<button>` in the key cell and leave the row alone. Order by lint count: `ContainersView`, `App.tsx`, `RequirementsView`, `TestDetail`, `FolderTree`.

**T2.4 Tables and icon buttons (IP-07, ISS-09, ISS-14).**
`scope="col"` on header cells and an accessible name (`caption` or `aria-label`) on each of the 35 tables; `aria-label` on the three icon-only close buttons.

**T2.5 Focus ring, motion, breakpoints (IP-08, ISS-08, ISS-10).**
For each of the 8 `outline: none` declarations, either add the `:focus-visible` ring from `primitives.css` or note that the widget draws its own indicator. Wrap the 12 transitions in a reduced-motion rule (core already has one at `primitives.css:904`). Reuse the 560, 720 and 900px breakpoints TAM and core use.

### Wave 3: structure

**T3.1 Move the duplicated components into core (IP-09, ISS-11).**
Create `frontend/core/src/components/profile/` and `.../about/` (C5). Order: `AboutModal`, `DiagnosticsModal`, `ProfilesModal` (near twins, 97 to 121 lines), then `PendingChangesModal` and `ProfileForm`, which need slots for the app-specific part. Each app keeps a thin adapter; no new `@agile-suite/core` export without a consumer in both apps.
Test first: move the existing app tests to core and run them against both adapters.

**T3.2 Split the long XTM files (IP-10, ISS-12).**
Order by lint count and risk: `ContainersView.tsx`, `App.tsx` (shell, topbar, nav rail, dialog host), `TestDetail.tsx` (one file per tab), `BugsPanel.tsx`, `TestTable.tsx`, `CoverageView.tsx`, `PreconditionsView.tsx`, `ProfileForm.tsx`. Domain folders (C5), under 400 lines (C2). Pin behaviour with a test through the public component before moving code.

**T3.3 Tests where code moves (IP-11).**
No separate test project. Each T2 and T3 task adds the behaviour test it needs; the goal is that no file touched in this plan stays untested.

### Wave 4: mockups

**T4.1 Make the .pen a system (IP-12, ISS-16).**
- Create variables from `tokens.css` with a theme axis for light and dark, then rebind fills (98 distinct literals to the token set).
- Build reusable `Topbar`, `NavRail`, `TableRow`, `Chip`, `Button` and `Modal shell`, and rebuild the frames from instances.
- Rename the 2,251 generic nodes; remove the empty frame `bi8Au` after confirming nothing links to it.
- Use the app's font stack on Windows (Segoe UI) rather than Roboto, set a text floor of 11px, and remove the 24 em dashes.

**T4.2 Draw the missing dialogs (gap list in section 2).**
XTM first: `ConnectionsModal`, `ProfilesModal`, `SyncHistoryModal`, `BridgeWizard`, `CreateBugModal`, `ImportTestsModal`, `JUnitImportModal`, `BulkMoveModal`, `BulkTransitionModal`. TAM: `SettingsModal`, `ProfilesModal`, `StartSprintModal`, `CompleteSprintModal`, `ImportIssuesModal`, `CreateSprintModal`.

## Open decisions

1. Text floor: 11px, which moves roughly 41 declarations up, or keep 10px for dense tables.
2. Whether `--warn-*` and `--warning-*` merge, and which name wins.
3. Whether XTM keeps its top tab bar plus rail or follows TAM's View menu.

## Do not

- Hand-edit anything under `wailsjs` (Generated bindings contract).
- Raise a ratchet number to make a branch pass; lower it with `--update` in the same PR as the cleanup.
- Add a hex colour or a `px` font size to a rule to match a mockup; add the token.
- Merge the default branch without approval (P8).
- Put AI attribution in commits or PR text (`.githooks/commit-msg`).

## Verification before a push

```
make gates
npm test --workspaces --if-present
npm run typecheck --workspaces --if-present
npm run lint
```

Report which of these ran. A task is done when its gate moved the way its
"Done when" line says, and the change is visible in demo mode in both themes.
