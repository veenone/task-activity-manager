# TAM Rituals Toolbar and Missing Root Page: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the ritual editor's hand-built buttons with a shared, accessible `EditorToolbar` in `@agile-suite/core`, and turn a Rituals Sync that finds its Confluence root page gone (404) into an offer to create (or adopt) a root page at the top of the space, save its id to the profile, and sync again.

**Architecture:** `EditorToolbar` is a pure React component in `frontend/core` that takes groups of plain items (toggle, action, link) and owns only the keyboard model, tooltips, and the link popover; TAM's `useRitualToolbar` maps TipTap state onto those items. On the Go side, `ritualsync.Run` reports a 404 root as `Result.RootMissing` (a value, never an error, because Wails delivers value or error and never both), carrying the answer of a new `confluence.SpaceProbe`. A new `ritualsync.CreateRoot` creates a top-level page through `CreatePage` with an empty parent (which now omits `ancestors`), and the new binding `App.CreateRitualRoot` runs it, saves the id, and runs the Sync, all under the `"rituals"` lock that the frontend takes through `SyncContext.runRitualRoot`.

**Tech Stack:** Go (Wails v2, modernc SQLite through `core/store`), React 19, TypeScript, TipTap 3.31.3, Vitest + jsdom + Testing Library.

**Spec:** `docs/superpowers/specs/2026-09-15-tam-02-rituals-toolbar-root-page-design.md`

## Global Constraints

- Work in the worktree `C:\tool-projects\task-activity-manager\.claude\worktrees\tam-planning-bundles`, branch `feat/tam-bundles-01-06`. Bundle 01 is planned and may be implemented on the same branch; commit only the files each task names.
- UI text uses no em dashes. Middle dot ` · ` and ellipsis `…` are fine.
- Credentials live only in the OS credential manager (Windows Credential Manager through `core/credentials`). Nothing in this plan reads, logs, or stores a token anywhere else.
- Logic lives in `internal/` (Go) and `lib/` or hooks (frontend); `app*.go` only adapts it to Wails.
- Anything that calls a binding taking `App.acquire` must also take the frontend `SyncContext` lock. `CreateRitualRoot` acquires `"rituals"` in Go, so the frontend reaches it only through `SyncContext.runRitualRoot`, never a bare call.
- `tam/frontend/wailsjs/**` is generated. Regenerate with `cd tam && wails generate module`; never hand-edit it.
- Commit messages use conventional prefixes (`feat(core):`, `feat(tam):`, `fix(tam):`, `docs:`) and must NEVER contain `Co-Authored-By`, `Claude-Session`, or "Generated with Claude Code" lines.
- Test gates: `go test ./...` inside `tam` and inside `core`; `npx vitest run` inside `tam/frontend` and inside `frontend/core`; `npm run typecheck --workspaces --if-present` at the repo root.
- `EditorToolbar` knows nothing about TipTap: no `@tiptap/*` import anywhere under `frontend/core`.
- Editor invariants from `tam/CLAUDE.md` stay exactly as they are: lock is `setEditable` on the same TipTap instance (never a rebuild); saves compare serialized documents; "Add today's entry" refuses a second entry for the same day; a link click never navigates the WebView; allowed link schemes are `http`, `https`, `mailto`.
- Changed on purpose by the spec: while locked for a Sync the toolbar renders **disabled, not hidden**. A read-only page (`readOnly`, or a page that does not parse) still shows no toolbar.
- Sentences, verbatim:
  - Forbidden: `Your token cannot create pages in <SPACE>. Ask a space admin, or set an existing page id in Profile settings.`
  - Link refused: `Links must start with http://, https:// or mailto:.`
  - Root missing: `The Confluence root page <id> could not be found. It may have been deleted, moved out of reach, or mistyped.`
  - Placement: `The new page goes at the top of the <SPACE> space, and its id is saved to this profile as the rituals root.`
  - After: `Sync runs again straight after. This sprint's ritual pages are created under the new root; a page that was under the old root and is gone shows as Gone, and Recreate on next Sync puts it under the new root.`
  - Taken, top level: `A page titled "<title>" is already at the top of <SPACE>. Use that page as the rituals root? Its id is saved to this profile and Sync runs under it.`
  - Taken, nested: `A page titled "<title>" already exists in <SPACE>, below another page. Choose a different title, or set that page's id in Profile settings.`
  - Done, created: `Created "<title>" at the top of <SPACE> and saved it as the rituals root.`
  - Done, adopted: `Using "<title>" in <SPACE> as the rituals root.`
  - Empty title (frontend): `The root page needs a title.` (Go: `The root page needs a title`)
  - Profile field, not a number: `The root page id is a number, such as 123456. You can also paste the page's address.`
  - Profile field, address without id: `That address carries no page id. In Confluence, open the page's Page Information and copy that address, which ends in pageId=123456.`
  - Profile form, save blocked: `Fix the Confluence root page id under Confluence Rituals before saving.`
- Default root title: `<Project key> Rituals` (`Rituals` when the project key is blank).
- Demo: a demo Confluence space asked for root page id `1` (`demo.StagedMissingRootID`) starts without that page.
- The real-instance probe `docs/superpowers/plans/assets/2026-09-15-confluence-root-page-probe.md` is **non-blocking**. Nothing waits for it; its answers may change wording and one status-code check in a follow-up.

## Rulings on the spec

- `CreateRitualRoot(title)` in the spec becomes `CreateRitualRoot(profileID, boardID, title, adopt)`: a binding needs the profile and the board whose Sync runs afterwards, and the duplicate-title adoption is the same call with `adopt` true.
- The spec's `EditorToolbar/` folder is kept (`frontend/core/src/components/EditorToolbar/`) even though the other core components are flat files: it holds five files that change together.
- `BubbleMenu` is not used. The spec names it as available, not required.
- A duplicate title is detected by a 400 **or** 409 create answer followed by a title lookup, never by reading Confluence's message, since which code comes back is a probe question.
- Adoption takes only a page with no ancestors (top of the space). A same-title page below another page is reported, not adopted.
- The permission probe is `GET /rest/api/content?spaceKey=K&limit=1` (401/403/404 is "no") then `GET /rest/api/space/K?expand=operations` ("yes" if `create`/`page` is listed, "no" if operations are listed without it, "unknown" otherwise). Unknown counts as "try".
- `RitualSyncResult.rootMissing` is optional in TypeScript so fixtures written before it stay valid; Go always sends it (null when the root was read).
- The profile form accepts a non-numeric root id when the Confluence URL is `demo` (the demo space's root is `demo-root`), and parses only the `pageId` query value out of a pasted address.
- There is no TAM user guide in the repository (`docs/user-guide/USER_GUIDE.md` is XTM's). The docs task updates `tam/CLAUDE.md` and gives the Outline user guide text to paste where that page exists.

## File map

**Core frontend, created** (`frontend/core/src/components/EditorToolbar/`)
- `types.ts`: `ToolbarItem`, `ToolbarGroup`.
- `icons.tsx`: `IconName`, `ToolbarIcon`.
- `ToolbarButton.tsx`: one button with its tooltip; `tooltipText`.
- `LinkPopover.tsx`: the address box.
- `EditorToolbar.tsx`: groups, roving tabindex, popover placement.
- `EditorToolbar.test.tsx`

**Core frontend, modified**: `frontend/core/src/index.ts`, `frontend/core/styles/primitives.css`.

**Go, created**
- `core/confluence/space.go`, `core/confluence/space_test.go`
- `tam/internal/ritualsync/root.go`, `tam/internal/ritualsync/root_test.go`

**Go, modified**
- `core/confluence/pages.go`, `core/confluence/pages_test.go`
- `tam/internal/demo/confluence.go`, `tam/internal/demo/confluence_test.go`
- `tam/internal/ritualsync/run.go`, `tam/internal/ritualsync/ensure.go`, `tam/internal/ritualsync/run_test.go`
- `tam/internal/ritualtemplate/ritualtemplate.go`, `tam/internal/ritualtemplate/ritualtemplate_test.go`
- `tam/app_rituals.go`, `tam/app_ritualsync_test.go`
- `tam/frontend/wailsjs/**` (regenerated)

**TAM frontend, created** (`tam/frontend/src/`)
- `components/ritual-editor/useRitualToolbar.ts`
- `components/RitualRootDialog.tsx`, `components/RitualRootDialog.test.tsx`
- `lib/confluenceRoot.ts`, `lib/confluenceRoot.test.ts`
- `components/ProfileForm.test.tsx`

**TAM frontend, modified**
- `components/ritual-editor/RitualEditor.tsx`, `RitualEditor.test.tsx`
- `lib/sanitizeHtml.ts`, `lib/sanitizeHtml.test.ts`, `lib/ritualText.ts`, `lib/ritualText.test.ts`
- `api.ts`, `contexts/SyncContext.tsx`, `contexts/SyncContext.test.tsx`
- `components/RitualsView.tsx`, `components/RitualsView.test.tsx`, `components/ProfileForm.tsx`, `App.css`

**Docs, modified**: `tam/CLAUDE.md`.

---

## Part A: the toolbar

### Task 1: `EditorToolbar` in `@agile-suite/core`

**Files:**
- Create: `frontend/core/src/components/EditorToolbar/types.ts`
- Create: `frontend/core/src/components/EditorToolbar/icons.tsx`
- Create: `frontend/core/src/components/EditorToolbar/ToolbarButton.tsx`
- Create: `frontend/core/src/components/EditorToolbar/LinkPopover.tsx`
- Create: `frontend/core/src/components/EditorToolbar/EditorToolbar.tsx`
- Test: `frontend/core/src/components/EditorToolbar/EditorToolbar.test.tsx`
- Modify: `frontend/core/src/index.ts` (append exports)
- Modify: `frontend/core/styles/primitives.css` (append a section at the end)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces (exported from `@agile-suite/core`):
  - `EditorToolbar(props: { label: string; groups: ToolbarGroup[]; disabled?: boolean }): JSX.Element`
  - `type ToolbarItem = ToolbarToggle | ToolbarAction | ToolbarLink` where
    - `ToolbarToggle = { kind: "toggle"; id: string; label: string; icon: IconName; shortcut?: string; active: boolean; disabled?: boolean; onToggle: () => void }`
    - `ToolbarAction = { kind: "action"; id: string; label: string; icon?: IconName; text?: string; shortcut?: string; disabled?: boolean; onRun: () => void }`
    - `ToolbarLink = { kind: "link"; id: string; label: string; href: string | null; onApply: (href: string) => string | null; onRemove: () => void }`
  - `type ToolbarGroup = { id: string; label: string; items: ToolbarItem[] }`
  - `type IconName = "bold" | "italic" | "underline" | "strike" | "heading2" | "heading3" | "bulletList" | "orderedList" | "taskList" | "table" | "link" | "undo" | "redo"`
  - DOM contract later tasks test against: toolbar `role="toolbar"` named by `label`; each group `role="group"` named by its label and carrying `data-group={id}`; each button's accessible name is the item `label`; toggles carry `aria-pressed`; an item with `disabled: true` carries `aria-disabled="true"` and stays focusable; `disabled` on the toolbar sets native `disabled` on every button; the link popover's input is named `Link address`, its buttons `Apply` and `Remove`, and a refusal renders as `role="alert"`.

- [ ] **Step 1: Write the failing test**

`frontend/core/src/components/EditorToolbar/EditorToolbar.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditorToolbar } from "./EditorToolbar";
import type { ToolbarGroup } from "./types";

interface Over {
  boldActive?: boolean;
  onBold?: () => void;
  undoDisabled?: boolean;
  onUndo?: () => void;
}

function groups(over: Over = {}): ToolbarGroup[] {
  return [
    {
      id: "marks",
      label: "Text",
      items: [
        { kind: "toggle", id: "bold", label: "Bold", icon: "bold", shortcut: "Ctrl+B", active: over.boldActive ?? false, onToggle: over.onBold ?? vi.fn() },
        { kind: "toggle", id: "italic", label: "Italic", icon: "italic", shortcut: "Ctrl+I", active: false, onToggle: vi.fn() },
      ],
    },
    {
      id: "history",
      label: "History",
      items: [
        { kind: "action", id: "undo", label: "Undo", icon: "undo", shortcut: "Ctrl+Z", disabled: over.undoDisabled ?? false, onRun: over.onUndo ?? vi.fn() },
        { kind: "action", id: "entry", label: "Add today's entry", text: "+ Add today's entry", onRun: vi.fn() },
      ],
    },
  ];
}

function linkGroups(link: { href?: string | null; onApply?: (href: string) => string | null; onRemove?: () => void } = {}): ToolbarGroup[] {
  return [
    {
      id: "insert",
      label: "Insert",
      items: [
        { kind: "link", id: "link", label: "Link", href: link.href ?? null, onApply: link.onApply ?? (() => null), onRemove: link.onRemove ?? vi.fn() },
      ],
    },
  ];
}

describe("EditorToolbar", () => {
  it("names the toolbar and each of its groups", () => {
    render(<EditorToolbar label="Formatting" groups={groups()} />);
    const bar = screen.getByRole("toolbar", { name: "Formatting" });
    expect(within(bar).getByRole("group", { name: "Text" })).toHaveAttribute("data-group", "marks");
    expect(within(bar).getByRole("group", { name: "History" })).toBeInTheDocument();
    expect(within(bar).getByRole("button", { name: "Add today's entry" })).toHaveTextContent("+ Add today's entry");
  });

  it("is one tab stop, and arrows, Home and End move focus through it with wrap", async () => {
    const user = userEvent.setup();
    render(
      <>
        <button>before</button>
        <EditorToolbar label="Formatting" groups={groups()} />
        <button>after</button>
      </>,
    );
    const bold = screen.getByRole("button", { name: "Bold" });
    const italic = screen.getByRole("button", { name: "Italic" });
    const entry = screen.getByRole("button", { name: "Add today's entry" });
    expect(bold).toHaveAttribute("tabindex", "0");
    expect(italic).toHaveAttribute("tabindex", "-1");

    await user.click(screen.getByRole("button", { name: "before" }));
    await user.tab();
    expect(bold).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(italic).toHaveFocus();
    expect(italic).toHaveAttribute("tabindex", "0");
    expect(bold).toHaveAttribute("tabindex", "-1");
    await user.keyboard("{End}");
    expect(entry).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(bold).toHaveFocus();
    await user.keyboard("{ArrowLeft}");
    expect(entry).toHaveFocus();
    await user.keyboard("{Home}");
    expect(bold).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "after" })).toHaveFocus();
  });

  it("reports a toggle's state with aria-pressed and runs it on click", async () => {
    const onBold = vi.fn();
    render(<EditorToolbar label="Formatting" groups={groups({ boldActive: true, onBold })} />);
    const bold = screen.getByRole("button", { name: "Bold" });
    expect(bold).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Italic" })).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByRole("button", { name: "Undo" })).not.toHaveAttribute("aria-pressed");
    await userEvent.click(bold);
    expect(onBold).toHaveBeenCalledTimes(1);
  });

  it("shows the label and shortcut as a tooltip on hover and on focus", async () => {
    const user = userEvent.setup();
    render(<EditorToolbar label="Formatting" groups={groups()} />);
    const bold = screen.getByRole("button", { name: "Bold" });
    expect(screen.queryByRole("tooltip")).toBeNull();

    await user.hover(bold);
    const tip = screen.getByRole("tooltip");
    expect(tip).toHaveTextContent("Bold (Ctrl+B)");
    expect(bold).toHaveAttribute("aria-describedby", tip.id);
    await user.unhover(bold);
    expect(screen.queryByRole("tooltip")).toBeNull();

    await user.tab();
    expect(bold).toHaveFocus();
    expect(screen.getByRole("tooltip")).toHaveTextContent("Bold (Ctrl+B)");
    await user.keyboard("{End}");
    expect(screen.getByRole("tooltip")).toHaveTextContent("Add today's entry");
  });

  it("renders every button disabled while the toolbar is disabled, and hides none", async () => {
    const onBold = vi.fn();
    render(<EditorToolbar label="Formatting" groups={groups({ onBold })} disabled />);
    const bar = screen.getByRole("toolbar", { name: "Formatting" });
    expect(bar).toHaveAttribute("aria-disabled", "true");
    const buttons = within(bar).getAllByRole("button");
    expect(buttons).toHaveLength(4);
    for (const button of buttons) expect(button).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Bold" }));
    expect(onBold).not.toHaveBeenCalled();
  });

  it("keeps an unavailable item focusable but inert", async () => {
    const user = userEvent.setup();
    const onUndo = vi.fn();
    render(<EditorToolbar label="Formatting" groups={groups({ undoDisabled: true, onUndo })} />);
    const undo = screen.getByRole("button", { name: "Undo" });
    expect(undo).toHaveAttribute("aria-disabled", "true");
    expect(undo).not.toBeDisabled();
    await user.tab();
    await user.keyboard("{ArrowRight}{ArrowRight}");
    expect(undo).toHaveFocus();
    await user.keyboard("{Enter}");
    await user.click(undo);
    expect(onUndo).not.toHaveBeenCalled();
  });
});

describe("the link popover", () => {
  it("opens from the link button, applies on Enter, and returns focus to the button", async () => {
    const user = userEvent.setup();
    const onApply = vi.fn((_href: string): string | null => null);
    render(<EditorToolbar label="Formatting" groups={linkGroups({ onApply })} />);
    const button = screen.getByRole("button", { name: "Link" });
    expect(button).toHaveAttribute("aria-expanded", "false");
    await user.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
    const input = screen.getByRole("textbox", { name: "Link address" });
    expect(input).toHaveFocus();
    await user.type(input, "https://example.com/notes{Enter}");
    expect(onApply).toHaveBeenCalledWith("https://example.com/notes");
    expect(screen.queryByRole("textbox", { name: "Link address" })).toBeNull();
    expect(button).toHaveFocus();
  });

  it("shows a refusal inline and stays open until the address changes", async () => {
    const user = userEvent.setup();
    const refusal = "Links must start with http://, https:// or mailto:.";
    const onApply = vi.fn((href: string): string | null => (href.startsWith("javascript:") ? refusal : null));
    render(<EditorToolbar label="Formatting" groups={linkGroups({ onApply })} />);
    await user.click(screen.getByRole("button", { name: "Link" }));
    const input = screen.getByRole("textbox", { name: "Link address" });
    await user.type(input, "javascript:alert(1){Enter}");
    expect(screen.getByRole("alert")).toHaveTextContent(refusal);
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("textbox", { name: "Link address" })).toBeInTheDocument();
    await user.type(input, "x");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("closes on Escape without applying and returns focus to the link button", async () => {
    const user = userEvent.setup();
    const onApply = vi.fn((_href: string): string | null => null);
    render(<EditorToolbar label="Formatting" groups={linkGroups({ onApply })} />);
    const button = screen.getByRole("button", { name: "Link" });
    await user.click(button);
    await user.type(screen.getByRole("textbox", { name: "Link address" }), "https://x.test");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("textbox", { name: "Link address" })).toBeNull();
    expect(onApply).not.toHaveBeenCalled();
    expect(button).toHaveFocus();
  });

  it("keeps arrow keys inside the address box", async () => {
    const user = userEvent.setup();
    render(<EditorToolbar label="Formatting" groups={linkGroups()} />);
    await user.click(screen.getByRole("button", { name: "Link" }));
    const input = screen.getByRole("textbox", { name: "Link address" });
    await user.type(input, "abc{ArrowLeft}{Home}");
    expect(input).toHaveFocus();
  });

  it("offers Remove only on an existing link, prefilled with its address", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    const { unmount } = render(<EditorToolbar label="Formatting" groups={linkGroups({ href: "https://example.com/a", onRemove })} />);
    const button = screen.getByRole("button", { name: "Link" });
    expect(button).toHaveAttribute("data-active", "true");
    await user.click(button);
    expect(screen.getByRole("textbox", { name: "Link address" })).toHaveValue("https://example.com/a");
    await user.click(screen.getByRole("button", { name: "Remove" }));
    expect(onRemove).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("textbox", { name: "Link address" })).toBeNull();
    unmount();

    render(<EditorToolbar label="Formatting" groups={linkGroups()} />);
    await user.click(screen.getByRole("button", { name: "Link" }));
    expect(screen.queryByRole("button", { name: "Remove" })).toBeNull();
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd frontend/core && npx vitest run src/components/EditorToolbar`
Expected: FAIL, `Failed to resolve import "./EditorToolbar"`.

- [ ] **Step 3: Write the types and icons**

`frontend/core/src/components/EditorToolbar/types.ts`:

```ts
import type { IconName } from "./icons";

// A toolbar item says what it looks like, whether it is on, and what to run.
// It says nothing about which editor it runs against: the caller maps its own
// editor's state onto these, which is what keeps EditorToolbar editor
// agnostic.
export type ToolbarToggle = {
  kind: "toggle";
  id: string;
  label: string;
  icon: IconName;
  shortcut?: string;
  active: boolean;
  // disabled means this one command cannot run here (it stays focusable);
  // disabling the whole toolbar is the toolbar's own prop.
  disabled?: boolean;
  onToggle: () => void;
};

export type ToolbarAction = {
  kind: "action";
  id: string;
  label: string;
  icon?: IconName;
  text?: string;
  shortcut?: string;
  disabled?: boolean;
  onRun: () => void;
};

export type ToolbarLink = {
  kind: "link";
  id: string;
  label: string;
  // href is the link under the caret, or null when there is none.
  href: string | null;
  // onApply answers with a refusal to show under the address box, or null
  // once the link is applied.
  onApply: (href: string) => string | null;
  onRemove: () => void;
};

export type ToolbarItem = ToolbarToggle | ToolbarAction | ToolbarLink;

export type ToolbarGroup = { id: string; label: string; items: ToolbarItem[] };
```

`frontend/core/src/components/EditorToolbar/icons.tsx`:

```tsx
import type { ReactNode } from "react";

export type IconName =
  | "bold" | "italic" | "underline" | "strike"
  | "heading2" | "heading3"
  | "bulletList" | "orderedList" | "taskList"
  | "table" | "link"
  | "undo" | "redo";

// Drawn in currentColor on a 16px grid, so dark mode and forced colors follow
// the text colour the button already has.
const SHAPES: Record<IconName, ReactNode> = {
  bold: <path d="M4.5 2.5h4a2.75 2.75 0 0 1 0 5.5h-4zM4.5 8h4.75a2.75 2.75 0 0 1 0 5.5H4.5z" strokeWidth="2" />,
  italic: <path d="M10 2.5H6.5M9.5 13.5H6M9 2.5 7 13.5" />,
  underline: <path d="M4.5 2.5v5a3.5 3.5 0 0 0 7 0v-5M3.5 13.5h9" />,
  strike: <path d="M2.5 8h11M11 4.5A3 2.25 0 0 0 8 2.75c-1.9 0-3 .9-3 2.1 0 .8.5 1.5 1.5 1.9M5 11.5a3 2.25 0 0 0 3 1.75c1.9 0 3-.9 3-2.1" />,
  heading2: <path d="M2 3.5v9M7 3.5v9M2 8h5M9.5 6a1.75 1.75 0 1 1 3.5 0c0 1.5-3.5 3-3.5 6.5H13" />,
  heading3: <path d="M2 3.5v9M7 3.5v9M2 8h5M9.5 4.5H13l-2 2.5a2 2 0 1 1-1.5 3.5" />,
  bulletList: (
    <>
      <path d="M6 4h7.5M6 8h7.5M6 12h7.5" />
      <circle cx="3" cy="4" r="0.75" fill="currentColor" />
      <circle cx="3" cy="8" r="0.75" fill="currentColor" />
      <circle cx="3" cy="12" r="0.75" fill="currentColor" />
    </>
  ),
  orderedList: <path d="M6.5 4h7M6.5 8h7M6.5 12h7M2.5 2.75l1-.5v3.5M2.25 10.25a.9.9 0 0 1 1.5-.25c.3.45-1.75 1.75-1.75 1.75h1.75" />,
  taskList: (
    <>
      <rect x="2" y="2.5" width="4" height="4" rx="1" />
      <path d="m2.5 11.5 1 1 1.75-2M8.5 4.5h5M8.5 11.5h5" />
    </>
  ),
  table: (
    <>
      <rect x="2" y="2.5" width="12" height="11" rx="1.5" />
      <path d="M2 6.5h12M2 10h12M6.5 6.5v7" />
    </>
  ),
  link: <path d="M6.75 9.25a2.5 2.5 0 0 0 3.5 0l2-2a2.5 2.5 0 0 0-3.5-3.5l-.75.75M9.25 6.75a2.5 2.5 0 0 0-3.5 0l-2 2a2.5 2.5 0 0 0 3.5 3.5l.75-.75" />,
  undo: <path d="M5.5 3.5 2.5 6.5l3 3M2.5 6.5h7a3.5 3.5 0 0 1 0 7H7" />,
  redo: <path d="m10.5 3.5 3 3-3 3M13.5 6.5h-7a3.5 3.5 0 0 0 0 7H9" />,
};

export function ToolbarIcon({ name }: { name: IconName }) {
  return (
    <svg
      className="editor-toolbar-icon"
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {SHAPES[name]}
    </svg>
  );
}
```

- [ ] **Step 4: Write `ToolbarButton` and `LinkPopover`**

`frontend/core/src/components/EditorToolbar/ToolbarButton.tsx`:

```tsx
import { useId, useState } from "react";
import type { ReactNode, Ref } from "react";

interface Props {
  label: string;
  shortcut?: string;
  // pressed is set only on a toggle; an action carries no aria-pressed.
  pressed?: boolean;
  // active marks state that is not a toggle, such as the link button while
  // the caret sits in a link.
  active?: boolean;
  // expanded is set only on a button that opens a popover.
  expanded?: boolean;
  disabled: boolean;
  unavailable?: boolean;
  tabIndex: 0 | -1;
  buttonRef: Ref<HTMLButtonElement>;
  onFocus: () => void;
  onPress: () => void;
  children: ReactNode;
}

export function tooltipText(label: string, shortcut?: string): string {
  return shortcut ? `${label} (${shortcut})` : label;
}

// ToolbarButton is one toolbar button and its tooltip. The tooltip shows on
// hover and on keyboard focus alike, and describes the button whether or not
// it is showing.
export function ToolbarButton({
  label, shortcut, pressed, active, expanded, disabled, unavailable, tabIndex, buttonRef, onFocus, onPress, children,
}: Props) {
  const tipId = useId();
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  return (
    <span className="editor-toolbar-slot" onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)}>
      <button
        ref={buttonRef}
        type="button"
        className="editor-toolbar-button"
        data-toolbar-item=""
        data-active={active ? "true" : undefined}
        aria-label={label}
        aria-pressed={pressed}
        aria-expanded={expanded}
        aria-haspopup={expanded === undefined ? undefined : "dialog"}
        aria-disabled={unavailable ? true : undefined}
        aria-describedby={tipId}
        disabled={disabled}
        tabIndex={tabIndex}
        // A mouse press must not take focus out of the editor, or the
        // selection a formatting command acts on is gone before it runs.
        onMouseDown={(e) => e.preventDefault()}
        onFocus={() => {
          setFocused(true);
          onFocus();
        }}
        onBlur={() => setFocused(false)}
        onClick={() => {
          onFocus();
          if (!unavailable) onPress();
        }}
      >
        {children}
      </button>
      <span role="tooltip" id={tipId} className="editor-toolbar-tooltip" hidden={!(hovered || focused)}>
        {tooltipText(label, shortcut)}
      </span>
    </span>
  );
}
```

`frontend/core/src/components/EditorToolbar/LinkPopover.tsx`:

```tsx
import { useEffect, useId, useRef, useState } from "react";

interface Props {
  href: string | null;
  onApply: (href: string) => string | null;
  onRemove: () => void;
  // onClose is called on Apply, Remove and Escape; the toolbar returns focus
  // to the link button from it.
  onClose: () => void;
}

// LinkPopover is the address box under a toolbar's link button. A refusal
// from onApply stays on screen under the box, rather than the box closing on
// an address that was never applied.
export function LinkPopover({ href, onApply, onRemove, onClose }: Props) {
  const [value, setValue] = useState(href ?? "");
  const [error, setError] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const errorId = useId();

  useEffect(() => {
    input.current?.focus();
    input.current?.select();
  }, []);

  const apply = () => {
    const refusal = onApply(value);
    if (refusal) {
      setError(refusal);
      return;
    }
    onClose();
  };

  return (
    <div
      className="editor-link-popover"
      role="dialog"
      aria-label="Link"
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.preventDefault();
          e.stopPropagation();
          onClose();
        }
      }}
    >
      <input
        ref={input}
        aria-label="Link address"
        value={value}
        placeholder="https://"
        spellCheck={false}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        onChange={(e) => {
          setValue(e.target.value);
          setError("");
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            apply();
          }
        }}
      />
      <button type="button" className="btn btn-primary" onClick={apply}>Apply</button>
      {href !== null && (
        <button
          type="button"
          className="btn btn-ghost"
          onClick={() => {
            onRemove();
            onClose();
          }}
        >
          Remove
        </button>
      )}
      {error && <p id={errorId} className="error-text editor-link-error" role="alert">{error}</p>}
    </div>
  );
}
```

- [ ] **Step 5: Write `EditorToolbar`**

`frontend/core/src/components/EditorToolbar/EditorToolbar.tsx`:

```tsx
import { useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { LinkPopover } from "./LinkPopover";
import { ToolbarButton } from "./ToolbarButton";
import { ToolbarIcon } from "./icons";
import type { ToolbarGroup, ToolbarItem } from "./types";

interface Props {
  label: string;
  groups: ToolbarGroup[];
  // disabled renders every button disabled without hiding any, so a toolbar
  // locked for a while (a Sync running) keeps its place and the page under
  // it does not jump.
  disabled?: boolean;
}

// EditorToolbar is a formatting toolbar for any rich text editor, and knows
// nothing about the editor behind it. The keyboard model is the WAI-ARIA
// toolbar pattern: one tab stop, Left and Right move between items with wrap,
// Home and End jump to either end. The tab stop is remembered by item id, not
// position, so an item that appears or vanishes between renders never moves
// it onto a different button. Item ids must be unique across groups.
export function EditorToolbar({ label, groups, disabled = false }: Props) {
  const items = groups.flatMap((g) => g.items);
  const positions = new Map(items.map((item, i) => [item.id, i]));
  const [focusId, setFocusId] = useState<string | null>(null);
  const [openLink, setOpenLink] = useState<string | null>(null);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const current = (focusId !== null && positions.get(focusId)) || 0;

  const moveTo = (index: number) => {
    if (items.length === 0) return;
    const item = items[(index + items.length) % items.length];
    setFocusId(item.id);
    buttons.current.get(item.id)?.focus();
  };

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    // Keys typed into something inside the toolbar that is not one of its
    // buttons (the link popover's address box) belong to that control.
    if (!(e.target instanceof HTMLElement) || !e.target.hasAttribute("data-toolbar-item")) return;
    const targets: Record<string, number> = { ArrowRight: current + 1, ArrowLeft: current - 1, Home: 0, End: items.length - 1 };
    if (!(e.key in targets)) return;
    e.preventDefault();
    moveTo(targets[e.key]);
  };

  const register = (id: string) => (el: HTMLButtonElement | null) => {
    if (el) buttons.current.set(id, el);
    else buttons.current.delete(id);
  };

  const renderItem = (item: ToolbarItem) => {
    const common = {
      label: item.label,
      disabled,
      tabIndex: (positions.get(item.id) === current ? 0 : -1) as 0 | -1,
      buttonRef: register(item.id),
      onFocus: () => setFocusId(item.id),
    };
    switch (item.kind) {
      case "toggle":
        return (
          <ToolbarButton key={item.id} {...common} shortcut={item.shortcut} pressed={item.active} unavailable={item.disabled} onPress={item.onToggle}>
            <ToolbarIcon name={item.icon} />
          </ToolbarButton>
        );
      case "action":
        return (
          <ToolbarButton key={item.id} {...common} shortcut={item.shortcut} unavailable={item.disabled} onPress={item.onRun}>
            {item.icon && <ToolbarIcon name={item.icon} />}
            {item.text && <span className="editor-toolbar-text">{item.text}</span>}
          </ToolbarButton>
        );
      case "link": {
        const open = openLink === item.id && !disabled;
        return (
          <span key={item.id} className="editor-toolbar-link">
            <ToolbarButton {...common} active={item.href !== null} expanded={open} onPress={() => setOpenLink(open ? null : item.id)}>
              <ToolbarIcon name="link" />
            </ToolbarButton>
            {open && (
              <LinkPopover
                href={item.href}
                onApply={item.onApply}
                onRemove={item.onRemove}
                onClose={() => {
                  setOpenLink(null);
                  buttons.current.get(item.id)?.focus();
                }}
              />
            )}
          </span>
        );
      }
    }
  };

  return (
    <div className="editor-toolbar" role="toolbar" aria-label={label} aria-disabled={disabled || undefined} onKeyDown={onKeyDown}>
      {groups.map((group) => (
        <div key={group.id} className="editor-toolbar-group" role="group" aria-label={group.label} data-group={group.id}>
          {group.items.map(renderItem)}
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 6: Export and style it**

Append to `frontend/core/src/index.ts`:

```ts
export { EditorToolbar } from "./components/EditorToolbar/EditorToolbar";
export type { ToolbarGroup, ToolbarItem, ToolbarToggle, ToolbarAction, ToolbarLink } from "./components/EditorToolbar/types";
export type { IconName } from "./components/EditorToolbar/icons";
```

Append to the end of `frontend/core/styles/primitives.css`:

```css
/* Editor toolbar: grouped icon buttons, a tooltip on each, and the link
   popover. Disabled buttons stay drawn so a locked toolbar keeps its place. */
.editor-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; padding: 4px 8px; border-bottom: 1px solid var(--border); background: var(--surface-2); }
.editor-toolbar-group { display: inline-flex; align-items: center; gap: 2px; padding-right: 4px; border-right: 1px solid var(--border-subtle); }
.editor-toolbar-group:last-child { border-right: 0; }
.editor-toolbar-slot, .editor-toolbar-link { position: relative; display: inline-flex; }
.editor-toolbar-button { display: inline-flex; align-items: center; justify-content: center; gap: 4px; min-width: 28px; height: 28px; padding: 0 6px; border: 1px solid transparent; border-radius: 4px; background: transparent; color: var(--text); font: inherit; font-size: 13px; cursor: pointer; }
.editor-toolbar-button:hover:not(:disabled):not([aria-disabled="true"]) { background: var(--ghost-hover); }
.editor-toolbar-button:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
.editor-toolbar-button[aria-pressed="true"], .editor-toolbar-button[data-active="true"] { background: var(--accent-soft); color: var(--accent); }
.editor-toolbar-button:disabled, .editor-toolbar-button[aria-disabled="true"] { opacity: 0.45; cursor: default; }
.editor-toolbar-tooltip { position: absolute; top: calc(100% + 4px); left: 50%; transform: translateX(-50%); z-index: 20; padding: 2px 6px; border-radius: 4px; background: var(--text-strong); color: var(--surface); font-size: 11px; white-space: nowrap; pointer-events: none; }
.editor-link-popover { position: absolute; top: calc(100% + 4px); left: 0; z-index: 30; display: flex; flex-wrap: wrap; align-items: center; gap: 4px; min-width: 280px; padding: 8px; border: 1px solid var(--border-strong); border-radius: 6px; background: var(--surface); box-shadow: var(--modal-shadow); }
.editor-link-popover input { flex: 1; min-width: 180px; }
.editor-link-error { flex-basis: 100%; margin: 0; font-size: 12px; }
@media (forced-colors: active) {
  .editor-toolbar-button[aria-pressed="true"], .editor-toolbar-button[data-active="true"] { outline: 2px solid CanvasText; }
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd frontend/core && npx vitest run src/components/EditorToolbar`
Expected: PASS, 11 tests.

Run: `cd frontend/core && npx vitest run && npm run typecheck`
Expected: every core test passes; `tsc --noEmit` reports nothing.

Run: `grep -rn "@tiptap" frontend/core/src` (from the repo root)
Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add frontend/core/src/components/EditorToolbar frontend/core/src/index.ts frontend/core/styles/primitives.css
git commit -m "feat(core): an accessible, editor-agnostic EditorToolbar with a link popover"
```

### Task 2: The ritual editor on `EditorToolbar`

**Files:**
- Create: `tam/frontend/src/components/ritual-editor/useRitualToolbar.ts`
- Modify: `tam/frontend/src/components/ritual-editor/RitualEditor.tsx` (imports at lines 1-14, `type Run` at line 45, `act` near line 234, toolbar render near line 300, the `Toolbar` function at the end of the file)
- Modify: `tam/frontend/src/lib/sanitizeHtml.ts` (add `isAllowedLink` after `safeUrl`)
- Modify: `tam/frontend/src/lib/ritualText.ts` (add `LINK_REFUSED`)
- Modify: `tam/frontend/src/App.css` (the five `.ritual-toolbar` / `.ritual-link-input` rules near line 701)
- Test: `tam/frontend/src/lib/sanitizeHtml.test.ts`, `tam/frontend/src/components/ritual-editor/RitualEditor.test.tsx`

**Interfaces:**
- Consumes: `EditorToolbar`, `ToolbarGroup`, `ToolbarItem`, `IconName` from `@agile-suite/core` (Task 1).
- Produces:
  - `useRitualToolbar(editor: Editor | null, options: { standup: boolean; onAddEntry: () => void }): ToolbarGroup[]`
  - `isAllowedLink(value: string): boolean` in `lib/sanitizeHtml.ts`
  - `LINK_REFUSED = "Links must start with http://, https:// or mailto:."` in `lib/ritualText.ts`
  - Groups, in order, each present only when it has items: `marks` "Text" (Bold, Italic, Underline, Strikethrough), `headings` "Headings" (Heading 2, Heading 3), `lists` "Lists" (Bullet list, Numbered list, Task list), `insert` "Insert" (Insert table, Link), `history` "History" (Undo, Redo), `ritual` "Standup" (Add today's entry, standup only).

- [ ] **Step 1: Write the failing tests**

Append to `tam/frontend/src/lib/sanitizeHtml.test.ts`, and change its import line to `import { isAllowedLink, sanitizeHtml } from "./sanitizeHtml";`:

```ts
describe("isAllowedLink", () => {
  it("allows http, https and mailto addresses, in any case", () => {
    for (const href of ["https://example.com", "http://example.com/a?b=1", "MAILTO:team@example.com", "HTTPS://EXAMPLE.COM"]) {
      expect(isAllowedLink(href)).toBe(true);
    }
  });

  it("refuses other schemes, disguised ones, and relative addresses", () => {
    for (const href of ["javascript:alert(1)", "java\tscript:alert(1)", " javascript:alert(1)", "data:text/html,x", "ftp://example.com", "example.com", "/wiki/x", ""]) {
      expect(isAllowedLink(href)).toBe(false);
    }
  });
});
```

In `tam/frontend/src/components/ritual-editor/RitualEditor.test.tsx`:

1. Change the testing-library import to `import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";` and the ritualText import to `import { READ_ONLY_SENTENCE, ENTRY_EXISTS, ENTRY_UNREADABLE, LINK_REFUSED } from "../../lib/ritualText";`.

2. Replace the whole test `it("is not editable while locked and editable again once unlocked, on the same editor", ...)` (it asserted the toolbar was absent while locked, which the spec changes) with:

```tsx
  // A Sync must not meet keystrokes typed after its flush, and must not cost
  // the caret or undo history either, so the lock is setEditable on the same
  // instance rather than a rebuild. The toolbar stays drawn, disabled, so the
  // page does not jump when a Sync starts.
  it("is not editable while locked, with its toolbar disabled not hidden, and editable again on the same editor", async () => {
    const ref = createRef<RitualEditorHandle>();
    const { rerender } = render(<RitualEditor ref={ref} profileId="p1" doc={doc()} locked />);
    await screen.findByText("ship it");
    const before = ref.current!.editor!;
    await waitFor(() => expect(before.isEditable).toBe(false));
    const locked = screen.getByRole("toolbar", { name: "Formatting" });
    for (const button of within(locked).getAllByRole("button")) expect(button).toBeDisabled();
    rerender(<RitualEditor ref={ref} profileId="p1" doc={doc()} locked={false} />);
    await waitFor(() => expect(ref.current!.editor!.isEditable).toBe(true));
    expect(ref.current!.editor).toBe(before);
    expect(within(screen.getByRole("toolbar", { name: "Formatting" })).getByRole("button", { name: "Bold" })).toBeEnabled();
  });
```

3. Add these tests at the end of the `describe("RitualEditor")` block:

```tsx
  it("offers the formatting groups this editor supports", async () => {
    render(<RitualEditor profileId="p1" doc={doc()} />);
    await screen.findByText("ship it");
    const bar = screen.getByRole("toolbar", { name: "Formatting" });
    expect(within(bar).getAllByRole("group").map((g) => g.getAttribute("aria-label"))).toEqual(["Text", "Headings", "Lists", "Insert", "History"]);
    for (const name of ["Bold", "Italic", "Underline", "Strikethrough", "Heading 2", "Heading 3", "Bullet list", "Numbered list", "Task list", "Insert table", "Link", "Undo", "Redo"]) {
      expect(within(bar).getByRole("button", { name })).toBeInTheDocument();
    }
    // A page just opened has nothing to undo.
    expect(within(bar).getByRole("button", { name: "Undo" })).toHaveAttribute("aria-disabled", "true");
  });

  it("puts Add today's entry in its own Standup group", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ ritualType: "standup", body: "<h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>" })} />);
    await screen.findByText("Mon 14 Sep 2026");
    const group = screen.getByRole("group", { name: "Standup" });
    expect(within(group).getByRole("button", { name: "Add today's entry" })).toHaveTextContent("+ Add today's entry");
  });

  it("draws no toolbar on a read-only page", async () => {
    render(<RitualEditor profileId="p1" readOnly doc={doc()} />);
    await screen.findByText("ship it");
    expect(screen.queryByRole("toolbar", { name: "Formatting" })).toBeNull();
  });

  it("bolds the selection from the toolbar, shows it pressed, and saves it", async () => {
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={10} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.selectAll(); });
    const bold = screen.getByRole("button", { name: "Bold" });
    await userEvent.click(bold);
    await waitFor(() => expect(bold).toHaveAttribute("aria-pressed", "true"));
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalled());
    expect(vi.mocked(api.SaveRitualBody).mock.calls.at(-1)![4]).toContain("<strong>");
  });

  it("refuses a link scheme it does not allow and says why, then links an https address", async () => {
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" doc={doc()} />);
    await screen.findByText("ship it");
    act(() => { ref.current!.editor!.commands.selectAll(); });
    await userEvent.click(screen.getByRole("button", { name: "Link" }));
    const input = screen.getByRole("textbox", { name: "Link address" });
    await userEvent.type(input, "javascript:alert(1){Enter}");
    expect(await screen.findByText(LINK_REFUSED)).toBeInTheDocument();
    expect(ref.current!.editor!.getHTML()).not.toContain("javascript");
    await userEvent.clear(input);
    await userEvent.type(input, "https://example.com/notes{Enter}");
    await waitFor(() => expect(screen.queryByRole("textbox", { name: "Link address" })).toBeNull());
    expect(ref.current!.editor!.getHTML()).toContain('href="https://example.com/notes"');
  });
```

The existing tests `adds today's standup entry under the log once, and refuses a second`, the two standup notice tests, and `offers Add today's entry only on the standup` stay unchanged: the button keeps its accessible name `Add today's entry`, and the once-a-day refusal still lives in `addTodaysEntry`. The link click tests stay unchanged: `openLinkOutside` is not touched.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam/frontend && npx vitest run src/lib/sanitizeHtml.test.ts src/components/ritual-editor/RitualEditor.test.tsx`
Expected: FAIL. `isAllowedLink is not a function`; the locked test fails with `Unable to find role="toolbar"`; the group test finds no `role="group"`.

- [ ] **Step 3: Add `isAllowedLink` and `LINK_REFUSED`**

In `tam/frontend/src/lib/sanitizeHtml.ts`, directly after the `safeUrl` function:

```ts
// isAllowedLink is what the editor's link box accepts: an address whose
// scheme, read after compact the way safeUrl reads it, is http, https or
// mailto. Unlike safeUrl it refuses a relative address, which on a page that
// lives in Confluence and in TAM at once points nowhere useful.
export function isAllowedLink(value: string): boolean {
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(compact(value));
  return !!scheme && SAFE_SCHEMES.has(scheme[1].toLowerCase());
}
```

In `tam/frontend/src/lib/ritualText.ts`, after `ENTRY_UNREADABLE`:

```ts
export const LINK_REFUSED = "Links must start with http://, https:// or mailto:.";
```

- [ ] **Step 4: Write `useRitualToolbar`**

`tam/frontend/src/components/ritual-editor/useRitualToolbar.ts`:

```ts
import { useEditorState } from "@tiptap/react";
import type { ChainedCommands, Editor } from "@tiptap/core";
import type { IconName, ToolbarGroup, ToolbarItem } from "@agile-suite/core";
import { LINK_REFUSED } from "../../lib/ritualText";
import { isAllowedLink } from "../../lib/sanitizeHtml";

type Chain = (chain: ChainedCommands) => ChainedCommands;

interface ToggleSpec {
  id: string;
  group: "marks" | "headings" | "lists";
  label: string;
  icon: IconName;
  shortcut: string;
  // requires is the schema mark or node this editor must carry for the
  // button to be offered at all.
  requires: string;
  active: (editor: Editor) => boolean;
  run: Chain;
}

// The shortcuts are the ones the TipTap extensions bind (Mod-b, Mod-Shift-s,
// Mod-Alt-2, Mod-Shift-8 and the rest), spelled for a Windows keyboard.
const TOGGLES: ToggleSpec[] = [
  { id: "bold", group: "marks", label: "Bold", icon: "bold", shortcut: "Ctrl+B", requires: "bold", active: (e) => e.isActive("bold"), run: (c) => c.toggleBold() },
  { id: "italic", group: "marks", label: "Italic", icon: "italic", shortcut: "Ctrl+I", requires: "italic", active: (e) => e.isActive("italic"), run: (c) => c.toggleItalic() },
  { id: "underline", group: "marks", label: "Underline", icon: "underline", shortcut: "Ctrl+U", requires: "underline", active: (e) => e.isActive("underline"), run: (c) => c.toggleUnderline() },
  { id: "strike", group: "marks", label: "Strikethrough", icon: "strike", shortcut: "Ctrl+Shift+S", requires: "strike", active: (e) => e.isActive("strike"), run: (c) => c.toggleStrike() },
  { id: "h2", group: "headings", label: "Heading 2", icon: "heading2", shortcut: "Ctrl+Alt+2", requires: "heading", active: (e) => e.isActive("heading", { level: 2 }), run: (c) => c.toggleHeading({ level: 2 }) },
  { id: "h3", group: "headings", label: "Heading 3", icon: "heading3", shortcut: "Ctrl+Alt+3", requires: "heading", active: (e) => e.isActive("heading", { level: 3 }), run: (c) => c.toggleHeading({ level: 3 }) },
  { id: "bullet", group: "lists", label: "Bullet list", icon: "bulletList", shortcut: "Ctrl+Shift+8", requires: "bulletList", active: (e) => e.isActive("bulletList"), run: (c) => c.toggleBulletList() },
  { id: "ordered", group: "lists", label: "Numbered list", icon: "orderedList", shortcut: "Ctrl+Shift+7", requires: "orderedList", active: (e) => e.isActive("orderedList"), run: (c) => c.toggleOrderedList() },
  { id: "tasks", group: "lists", label: "Task list", icon: "taskList", shortcut: "Ctrl+Shift+9", requires: "taskList", active: (e) => e.isActive("taskList"), run: (c) => c.toggleTaskList() },
];

const GROUP_LABEL: Record<ToggleSpec["group"], string> = { marks: "Text", headings: "Headings", lists: "Lists" };

const insertTable: Chain = (c) => c.insertTable({ rows: 3, cols: 3, withHeaderRow: true });

// Snapshot is plain data, so useEditorState's deep comparison re-renders the
// toolbar only on a transaction that changed something it draws. null in a
// field means this editor does not carry that feature, and the item is not
// offered.
interface Snapshot {
  toggles: { id: string; active: boolean; can: boolean }[];
  table: boolean | null;
  link: { href: string | null } | null;
  undo: boolean | null;
  redo: boolean | null;
}

function read(editor: Editor): Snapshot {
  const has = (name: string) => name in editor.schema.marks || name in editor.schema.nodes;
  const commands = editor.extensionManager.commands;
  return {
    toggles: TOGGLES.filter((t) => has(t.requires)).map((t) => ({ id: t.id, active: t.active(editor), can: t.run(editor.can().chain()).run() })),
    table: has("table") ? insertTable(editor.can().chain()).run() : null,
    link: has("link") ? { href: editor.isActive("link") ? String(editor.getAttributes("link").href ?? "") : null } : null,
    undo: "undo" in commands ? editor.can().undo() : null,
    redo: "redo" in commands ? editor.can().redo() : null,
  };
}

// applyLink is the popover's Apply. An empty address removes the link, an
// address TAM does not allow is refused with the sentence the popover shows,
// and anything else links the selection. The Link mark's own URI check would
// refuse a javascript: address too, but silently; this is what says why.
function applyLink(editor: Editor, href: string): string | null {
  const value = href.trim();
  if (!value) {
    editor.chain().focus().extendMarkRange("link").unsetLink().run();
    return null;
  }
  if (!isAllowedLink(value)) return LINK_REFUSED;
  editor.chain().focus().extendMarkRange("link").setLink({ href: value }).run();
  return null;
}

// useRitualToolbar maps the ritual editor's TipTap state onto the shared
// EditorToolbar's groups, re-rendering on every transaction that changes what
// the toolbar draws. It offers only what this editor's extensions carry.
export function useRitualToolbar(editor: Editor | null, options: { standup: boolean; onAddEntry: () => void }): ToolbarGroup[] {
  const snap = useEditorState({ editor, selector: ({ editor: e }) => (e ? read(e) : null) });
  if (!editor || !snap) return [];

  const run = (chain: Chain) => () => {
    chain(editor.chain().focus()).run();
  };

  const toggles = (group: ToggleSpec["group"]): ToolbarItem[] =>
    snap.toggles.flatMap((state) => {
      const spec = TOGGLES.find((t) => t.id === state.id);
      if (!spec || spec.group !== group) return [];
      return [{ kind: "toggle" as const, id: spec.id, label: spec.label, icon: spec.icon, shortcut: spec.shortcut, active: state.active, disabled: !state.can, onToggle: run(spec.run) }];
    });

  const insert: ToolbarItem[] = [];
  if (snap.table !== null) {
    insert.push({ kind: "action", id: "table", label: "Insert table", icon: "table", disabled: !snap.table, onRun: run(insertTable) });
  }
  if (snap.link) {
    insert.push({
      kind: "link",
      id: "link",
      label: "Link",
      href: snap.link.href,
      onApply: (href) => applyLink(editor, href),
      onRemove: () => {
        editor.chain().focus().extendMarkRange("link").unsetLink().run();
      },
    });
  }

  const history: ToolbarItem[] = [];
  if (snap.undo !== null) history.push({ kind: "action", id: "undo", label: "Undo", icon: "undo", shortcut: "Ctrl+Z", disabled: !snap.undo, onRun: run((c) => c.undo()) });
  if (snap.redo !== null) history.push({ kind: "action", id: "redo", label: "Redo", icon: "redo", shortcut: "Ctrl+Shift+Z", disabled: !snap.redo, onRun: run((c) => c.redo()) });

  const groups: ToolbarGroup[] = [
    { id: "marks", label: GROUP_LABEL.marks, items: toggles("marks") },
    { id: "headings", label: GROUP_LABEL.headings, items: toggles("headings") },
    { id: "lists", label: GROUP_LABEL.lists, items: toggles("lists") },
    { id: "insert", label: "Insert", items: insert },
    { id: "history", label: "History", items: history },
  ];
  if (options.standup) {
    groups.push({ id: "ritual", label: "Standup", items: [{ kind: "action", id: "entry", label: "Add today's entry", text: "+ Add today's entry", onRun: options.onAddEntry }] });
  }
  return groups.filter((g) => g.items.length > 0);
}
```

- [ ] **Step 5: Replace the hand-built toolbar in `RitualEditor.tsx`**

Make these edits, and no others:

1. Imports. Replace

```tsx
import { EditorContent, useEditor, useEditorState } from "@tiptap/react";
import type { ChainedCommands, Editor } from "@tiptap/core";
import { announce, errMsg } from "@agile-suite/core";
```

with

```tsx
import { EditorContent, useEditor } from "@tiptap/react";
import type { Editor } from "@tiptap/core";
import { EditorToolbar, announce, errMsg } from "@agile-suite/core";
```

and after `import { ritualExtensions } from "./extensions";` add

```tsx
import { useRitualToolbar } from "./useRitualToolbar";
```

2. Delete the line `type Run = (chain: ChainedCommands) => ChainedCommands;`.

3. Delete the `act` helper:

```tsx
  const act = (run: Run) => {
    if (!editor) return;
    run(editor.chain().focus()).run();
  };
```

4. Directly above `if (!parsed.ok) {` (so the hook runs on every render, before the early return), add:

```tsx
  const toolbar = useRitualToolbar(editor, { standup: doc.ritualType === "standup", onAddEntry: () => void addTodaysEntry() });
```

5. Replace

```tsx
        {editable && !locked && editor && <Toolbar editor={editor} act={act} standup={doc.ritualType === "standup"} onAddEntry={() => void addTodaysEntry()} />}
```

with

```tsx
        {/* Disabled, not hidden, while a Sync holds the page: the layout stays put. */}
        {editable && editor && <EditorToolbar label="Formatting" groups={toolbar} disabled={locked} />}
```

6. Delete the entire `function Toolbar({ editor, act, standup, onAddEntry }: ...) { ... }` at the end of the file.

In `tam/frontend/src/App.css`, replace these five lines:

```css
.ritual-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 2px; padding: 4px 8px; border-bottom: 1px solid var(--border); background: var(--surface-2); }
.ritual-toolbar .btn[aria-pressed="true"] { background: var(--accent-soft); color: var(--accent); }
.ritual-toolbar .ritual-add-entry { margin-left: auto; }
.ritual-link-input { display: inline-flex; gap: 4px; align-items: center; }
.ritual-link-input input { width: 220px; }
```

with

```css
.ritual-editor .editor-toolbar-group[data-group="ritual"] { margin-left: auto; border-right: 0; }
```

(`.rituals-toolbar`, with an s, is the view's own header row and is not touched.)

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd tam/frontend && npx vitest run src/lib/sanitizeHtml.test.ts src/components/ritual-editor`
Expected: PASS, including the five new editor tests and the rewritten lock test.

Run: `cd tam/frontend && npx vitest run && npm run typecheck`
Expected: all TAM frontend tests pass; `tsc --noEmit` reports nothing.

Run (repo root): `grep -n "ritual-toolbar\|ritual-link-input\|useEditorState" tam/frontend/src/components/ritual-editor/RitualEditor.tsx tam/frontend/src/App.css`
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add tam/frontend/src/components/ritual-editor/useRitualToolbar.ts tam/frontend/src/components/ritual-editor/RitualEditor.tsx tam/frontend/src/components/ritual-editor/RitualEditor.test.tsx tam/frontend/src/lib/sanitizeHtml.ts tam/frontend/src/lib/sanitizeHtml.test.ts tam/frontend/src/lib/ritualText.ts tam/frontend/src/App.css
git commit -m "feat(tam): the ritual editor on the shared EditorToolbar, disabled rather than hidden while a Sync runs"
```

---

## Part B: the missing root page

### Task 3: Confluence creates a top-level page and answers a permission probe

**Files:**
- Modify: `core/confluence/pages.go` (`CreatePage`, lines 124-143)
- Create: `core/confluence/space.go`
- Test: `core/confluence/pages_test.go` (append), `core/confluence/space_test.go` (create)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `(*Client).CreatePage(ctx, spaceKey, parentID, title, body string) (StoredPage, error)`: signature unchanged; an empty `parentID` sends no `ancestors` key.
  - `type Permission string` with `PermissionYes = "yes"`, `PermissionNo = "no"`, `PermissionUnknown = "unknown"`.
  - `type SpaceProbe interface { CanCreatePages(ctx context.Context, spaceKey string) Permission }`, satisfied by `*Client`.

- [ ] **Step 1: Write the failing tests**

Append to `core/confluence/pages_test.go`:

```go
func TestCreatePageWithNoParentSendsNoAncestors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("payload %s: %v", raw, err)
		}
		if _, ok := got["ancestors"]; ok {
			t.Errorf("a top-level create sent ancestors: %s", raw)
		}
		if string(got["title"]) != `"PLAT Rituals"` || string(got["type"]) != `"page"` {
			t.Errorf("payload = %s", raw)
		}
		_, _ = w.Write([]byte(`{"id":"500","title":"PLAT Rituals","version":{"number":1},"ancestors":[]}`))
	}))
	defer srv.Close()

	p, err := NewClient(srv.URL, "secret", "", false).CreatePage(context.Background(), "PLAT", "", "PLAT Rituals", "<p>root</p>")
	if err != nil || p.ID != "500" || len(p.AncestorIDs) != 0 || p.Body != "<p>root</p>" {
		t.Fatalf("created = %+v, %v", p, err)
	}
}
```

`core/confluence/space_test.go`:

```go
package confluence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func probeServer(t *testing.T, listing int, space string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/content":
			if q := r.URL.Query(); q.Get("spaceKey") != "TEAM" || q.Get("limit") != "1" {
				t.Errorf("listing query = %s", r.URL.RawQuery)
			}
			w.WriteHeader(listing)
			if listing == http.StatusOK {
				_, _ = w.Write([]byte(`{"results":[{"id":"1"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"message":"nope"}`))
		case "/rest/api/space/TEAM":
			if r.URL.Query().Get("expand") != "operations" {
				t.Errorf("space query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(space))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
}

func TestCanCreatePagesReadsTheSpaceThenItsOperations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		listing int
		space   string
		want    Permission
	}{
		{"create listed", http.StatusOK, `{"key":"TEAM","operations":[{"operation":"read","targetType":"space"},{"operation":"create","targetType":"page"}]}`, PermissionYes},
		{"operations without create", http.StatusOK, `{"key":"TEAM","operations":[{"operation":"read","targetType":"space"}]}`, PermissionNo},
		{"operations not reported", http.StatusOK, `{"key":"TEAM"}`, PermissionUnknown},
		{"space forbidden", http.StatusForbidden, "", PermissionNo},
		{"space not found", http.StatusNotFound, "", PermissionNo},
		{"server trouble", http.StatusInternalServerError, "", PermissionUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := probeServer(t, tc.listing, tc.space)
			defer srv.Close()
			if got := NewClient(srv.URL, "secret", "", false).CanCreatePages(context.Background(), "TEAM"); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd core && go test ./confluence/`
Expected: FAIL to compile, `undefined: Permission`. (Once `space.go` exists, the top-level create test still fails with `a top-level create sent ancestors` until Step 3's `pages.go` edit.)

- [ ] **Step 3: Implement**

In `core/confluence/pages.go`, replace the comment and the start of `CreatePage`, from `// CreatePage creates a page under parentID.` through the closing `}` of the `payload` literal, with:

```go
// CreatePage creates a page under parentID, or at the top of the space when
// parentID is empty: Confluence places a page sent with no ancestors there,
// and an ancestors entry with an empty id is not that. A response that leaves
// the body out answers with the body that was sent, since that is what now
// exists.
func (c *Client) CreatePage(ctx context.Context, spaceKey, parentID, title, body string) (StoredPage, error) {
	payload := map[string]any{
		"type":  "page",
		"title": title,
		"space": map[string]string{"key": spaceKey},
		"body":  storageBody(body),
	}
	if parentID != "" {
		payload["ancestors"] = []map[string]string{{"id": parentID}}
	}
```

The rest of the function, from `var r rawStoredPage`, stays as it is.

`core/confluence/space.go`:

```go
package confluence

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// Permission is what a probe learned about one thing a token may do in a
// space. Unknown is its own answer, not a polite no: an instance that does
// not report operations says nothing about the token.
type Permission string

const (
	PermissionYes     Permission = "yes"
	PermissionNo      Permission = "no"
	PermissionUnknown Permission = "unknown"
)

// SpaceProbe is the question the ritual sync asks before it offers to create
// a rituals root page. It is its own interface rather than a fifth method on
// Pages because an ordinary pass never needs it; only a missing root does.
type SpaceProbe interface {
	CanCreatePages(ctx context.Context, spaceKey string) Permission
}

var _ SpaceProbe = (*Client)(nil)

// CanCreatePages reads the space the way the token sees it, in two steps.
// First a one-page content listing: a space the token cannot read answers
// 401, 403 or 404, and creating in it is a no. Then the space's operations,
// where an instance that reports them lists create on page for a token that
// may. Anything else (a transport failure, an instance that leaves operations
// out) is unknown, which the caller treats as worth trying, with the create's
// own refusal as the fallback. What a real Data Center answers is recorded in
// docs/superpowers/plans/assets/2026-09-15-confluence-root-page-probe.md.
func (c *Client) CanCreatePages(ctx context.Context, spaceKey string) Permission {
	q := url.Values{}
	q.Set("spaceKey", spaceKey)
	q.Set("limit", "1")
	var listing struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := c.get(ctx, "/rest/api/content?"+q.Encode()).Decode(&listing); err != nil {
		if refused(err) {
			return PermissionNo
		}
		return PermissionUnknown
	}
	var space struct {
		Operations []struct {
			Operation  string `json:"operation"`
			TargetType string `json:"targetType"`
		} `json:"operations"`
	}
	if err := c.get(ctx, "/rest/api/space/"+url.PathEscape(spaceKey)+"?expand=operations").Decode(&space); err != nil || len(space.Operations) == 0 {
		return PermissionUnknown
	}
	for _, op := range space.Operations {
		if op.Operation == "create" && op.TargetType == "page" {
			return PermissionYes
		}
	}
	return PermissionNo
}

// refused is whether an answer says the token may not see the space at all.
func refused(err error) bool {
	var h *HTTPError
	if !errors.As(err, &h) {
		return false
	}
	switch h.Code {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	}
	return false
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd core && go test ./confluence/ -v -run 'TestCreatePage|TestCanCreatePages'`
Expected: PASS, including all six probe subtests and the existing `TestCreatePageSendsSpaceParentAndStorageBody`.

Run: `cd core && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add core/confluence/pages.go core/confluence/pages_test.go core/confluence/space.go core/confluence/space_test.go
git commit -m "feat(core): create a Confluence page at the top of a space, and probe whether a token may"
```

### Task 4: The demo space creates at the top, refuses a denied token, and can stage a missing root

**Files:**
- Modify: `tam/internal/demo/confluence.go`
- Test: `tam/internal/demo/confluence_test.go` (append)

**Interfaces:**
- Consumes: `confluence.Permission`, `confluence.SpaceProbe` (Task 3).
- Produces:
  - `const demo.StagedMissingRootID = "1"`: `NewConfluence(space, StagedMissingRootID, stage)` seeds no root page.
  - `(*demo.Confluence).CreatePage` with `parentID == ""` creates a top-level page (no ancestors).
  - `(*demo.Confluence).DenyCreate()`: every later create answers `*confluence.HTTPError{Code: 403}`; the probe answers `PermissionNo`.
  - `(*demo.Confluence).CanCreatePages(ctx, spaceKey) confluence.Permission`: `PermissionNo` for another space or after `DenyCreate`, `PermissionYes` otherwise.

- [ ] **Step 1: Write the failing tests**

Append to `tam/internal/demo/confluence_test.go`, and add `"net/http"` to its imports:

```go
func TestTheDemoSpaceCreatesAtTheTopAndCanStageAMissingRoot(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", StagedMissingRootID, false)
	if _, err := c.GetPageStorage(ctx, StagedMissingRootID); !errors.Is(err, confluence.ErrNotFound) {
		t.Fatalf("staged root = %v", err)
	}
	if got := c.CanCreatePages(ctx, "DEMO"); got != confluence.PermissionYes {
		t.Fatalf("probe = %s", got)
	}
	if got := c.CanCreatePages(ctx, "OTHER"); got != confluence.PermissionNo {
		t.Fatalf("probe for another space = %s", got)
	}
	root, err := c.CreatePage(ctx, "DEMO", "", "PLAT Rituals", "<p>root</p>")
	if err != nil || len(root.AncestorIDs) != 0 {
		t.Fatalf("top-level create = %+v, %v", root, err)
	}
	if _, err := c.CreatePage(ctx, "DEMO", "", "PLAT Rituals", "<p>again</p>"); err == nil {
		t.Fatal("a duplicate top-level title should be refused")
	}
}

func TestADeniedDemoTokenCannotCreate(t *testing.T) {
	ctx := context.Background()
	c := NewConfluence("DEMO", "root", false)
	c.DenyCreate()
	if got := c.CanCreatePages(ctx, "DEMO"); got != confluence.PermissionNo {
		t.Fatalf("probe = %s", got)
	}
	_, err := c.CreatePage(ctx, "DEMO", "", "PLAT Rituals", "<p/>")
	var h *confluence.HTTPError
	if !errors.As(err, &h) || h.Code != http.StatusForbidden {
		t.Fatalf("err = %v", err)
	}
	if _, err := c.GetPageStorage(ctx, "root"); err != nil {
		t.Fatalf("a root other than the staged id is still seeded: %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam && go test ./internal/demo/ -run 'TestTheDemoSpaceCreatesAtTheTop|TestADeniedDemoToken'`
Expected: FAIL to compile, `undefined: StagedMissingRootID`, `c.CanCreatePages undefined`, `c.DenyCreate undefined`.

- [ ] **Step 3: Implement**

In `tam/internal/demo/confluence.go`:

1. Add `denyCreate bool` as the last field of `type Confluence struct`.

2. After `var _ confluence.Pages = (*Confluence)(nil)` add:

```go
var _ confluence.SpaceProbe = (*Confluence)(nil)

// StagedMissingRootID is the root page id a demo space starts without. A demo
// profile whose Confluence root page id is set to it meets the missing root
// on its first Sync, which is how the create-a-root dialog is walked through
// with no real Confluence.
const StagedMissingRootID = "1"
```

3. In `NewConfluence`, replace the line seeding the root with:

```go
	if rootID != StagedMissingRootID {
		c.pages[rootID] = &fakePage{id: rootID, title: DemoRootTitle, body: "<p>Ritual pages for this team.</p>", version: 1}
	}
```

4. After `FailNext`, add:

```go
// DenyCreate makes the space refuse every page create with 403 from now on,
// and answer the permission probe with no: a token that can read the space
// and write nothing in it.
func (c *Confluence) DenyCreate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.denyCreate = true
}

// CanCreatePages answers the permission probe: no for another space or a
// denied token, yes otherwise.
func (c *Confluence) CanCreatePages(_ context.Context, spaceKey string) confluence.Permission {
	c.mu.Lock()
	defer c.mu.Unlock()
	if spaceKey != c.space || c.denyCreate {
		return confluence.PermissionNo
	}
	return confluence.PermissionYes
}
```

5. In `CreatePage`, change its comment to `// CreatePage creates a page under parentID, or at the top of the space when parentID is empty.`, then directly after the `if spaceKey != c.space { ... }` block insert:

```go
	if c.denyCreate {
		c.mu.Unlock()
		return confluence.StoredPage{}, &confluence.HTTPError{Code: http.StatusForbidden, Status: "403 Forbidden", Message: "Could not create content with type page"}
	}
```

and replace

```go
	if _, ok := c.pages[parentID]; !ok {
		c.mu.Unlock()
		return confluence.StoredPage{}, notFound()
	}
```

with

```go
	if parentID != "" {
		if _, ok := c.pages[parentID]; !ok {
			c.mu.Unlock()
			return confluence.StoredPage{}, notFound()
		}
	}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd tam && go test ./internal/demo/ -v -run 'TestTheDemoSpace|TestADeniedDemoToken'`
Expected: PASS, including the existing duplicate-title and missing-parent demo tests.

Run: `cd tam && go test ./...`
Expected: PASS (no existing test uses root id `1`).

- [ ] **Step 5: Commit**

```bash
git add tam/internal/demo/confluence.go tam/internal/demo/confluence_test.go
git commit -m "feat(tam): the demo Confluence space creates top-level pages, refuses a denied token, and stages a missing root"
```

### Task 5: A Sync that finds its root page gone says so in the result

**Files:**
- Create: `tam/internal/ritualsync/root.go`
- Modify: `tam/internal/ritualsync/run.go` (`Result`, and the root read at lines 62-65)
- Modify: `tam/internal/ritualsync/ensure.go` (`Config`)
- Test: `tam/internal/ritualsync/run_test.go` (append)

**Interfaces:**
- Consumes: `confluence.SpaceProbe`, `confluence.PermissionNo` (Task 3); `demo.Confluence.DenyCreate`, `Remove` (Task 4 and existing).
- Produces:
  - `ritualsync.Config.ProjectKey string`
  - `type RootMissing struct { PageID, SpaceKey string; CanCreate bool; SuggestedTitle string }` with JSON tags `pageId`, `spaceKey`, `canCreate`, `suggestedTitle`.
  - `Result.RootMissing *RootMissing` (JSON `rootMissing`, `null` when the root was read).
  - `SuggestedRootTitle(projectKey string) string`: `"PLAT Rituals"`, or `"Rituals"` for a blank key.
  - A 404 root returns `(Result{Failed: []PageFailure{}, RootMissing: &...}, nil)` with nothing written locally or remotely and `SyncedAt` empty. Any other root read failure keeps the existing error `The Confluence root page <id> could not be read: <reason>`.

- [ ] **Step 1: Write the failing tests**

Append to `tam/internal/ritualsync/run_test.go`:

```go
// pagesOnly hides the demo space's permission probe, the way a transport
// that cannot answer it would.
type pagesOnly struct{ confluence.Pages }

func TestAMissingRootIsReportedInTheResultNotAsAnError(t *testing.T) {
	h := newHarness(t)
	h.cfg.ProjectKey = "PLAT"
	h.fake.Remove("root")
	res, err := Run(h.ctx, h.fake, h.docs, h.cfg, testProfile, testBoard, []Sprint{sprint14})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := RootMissing{PageID: "root", SpaceKey: "PLAT", CanCreate: true, SuggestedTitle: "PLAT Rituals"}
	if res.RootMissing == nil || *res.RootMissing != want {
		t.Fatalf("root missing = %+v", res.RootMissing)
	}
	if res.Created != 0 || res.SyncedAt != "" || len(res.Failed) != 0 {
		t.Fatalf("a pass with no root must do nothing: %+v", res)
	}
	if docs, _ := h.docs.Documents(h.ctx, testProfile, testBoard, 14); len(docs) != 0 {
		t.Fatal("a pass with no root should not have written pages locally")
	}
}

func TestAMissingRootSaysWhenTheTokenCannotCreatePages(t *testing.T) {
	h := newHarness(t)
	h.fake.Remove("root")
	h.fake.DenyCreate()
	res, err := Run(h.ctx, h.fake, h.docs, h.cfg, testProfile, testBoard, []Sprint{sprint14})
	if err != nil || res.RootMissing == nil || res.RootMissing.CanCreate {
		t.Fatalf("result = %+v, %v", res.RootMissing, err)
	}
	if res.RootMissing.SuggestedTitle != "Rituals" {
		t.Fatalf("suggested title with no project key = %q", res.RootMissing.SuggestedTitle)
	}
}

func TestAMissingRootOnATransportWithNoProbeIsWorthTrying(t *testing.T) {
	h := newHarness(t)
	h.fake.Remove("root")
	h.fake.DenyCreate()
	res, err := Run(h.ctx, pagesOnly{h.fake}, h.docs, h.cfg, testProfile, testBoard, []Sprint{sprint14})
	if err != nil || res.RootMissing == nil || !res.RootMissing.CanCreate {
		t.Fatalf("result = %+v, %v", res.RootMissing, err)
	}
}

func TestSuggestedRootTitleNamesTheProject(t *testing.T) {
	for key, want := range map[string]string{"PLAT": "PLAT Rituals", " PLAT ": "PLAT Rituals", "": "Rituals", "  ": "Rituals"} {
		if got := SuggestedRootTitle(key); got != want {
			t.Errorf("SuggestedRootTitle(%q) = %q, want %q", key, got, want)
		}
	}
}
```

The existing `TestAnUnreadableRootRefusesThePass` (a 403 on the root) stays unchanged and must still pass.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam && go test ./internal/ritualsync/ -run 'TestAMissingRoot|TestSuggestedRootTitle|TestAnUnreadableRoot'`
Expected: FAIL to compile, `undefined: RootMissing`, `h.cfg.ProjectKey undefined`, `undefined: SuggestedRootTitle`.

- [ ] **Step 3: Implement**

In `tam/internal/ritualsync/ensure.go`, replace `type Config struct { ... }` with:

```go
// Config is what a pass needs besides its sprints: where the pages live, the
// project a missing root's suggested title names, and the clock and zone the
// templates and timestamps read.
type Config struct {
	SpaceKey   string
	RootID     string
	ProjectKey string
	Location   *time.Location
	Now        func() time.Time
}
```

`tam/internal/ritualsync/root.go`:

```go
package ritualsync

import (
	"context"
	"strings"

	"agile-suite/core/confluence"
)

// RootMissing is a pass that stopped because the rituals root page answered
// 404. It travels in the result, not as an error, for the reason Result does:
// the view needs every field to offer to create a root, and Wails would
// deliver only the sentence. CanCreate is the permission probe's answer with
// unknown read as true, so an instance that reports nothing still gets the
// offer and the create's own 403 speaks for it.
type RootMissing struct {
	PageID         string `json:"pageId"`
	SpaceKey       string `json:"spaceKey"`
	CanCreate      bool   `json:"canCreate"`
	SuggestedTitle string `json:"suggestedTitle"`
}

// SuggestedRootTitle is the title the missing-root dialog starts from.
func SuggestedRootTitle(projectKey string) string {
	return strings.TrimSpace(strings.TrimSpace(projectKey) + " Rituals")
}

// canCreate reads the transport's permission probe. Unknown, and a transport
// with no probe at all, both read as worth trying.
func canCreate(ctx context.Context, pages confluence.Pages, spaceKey string) bool {
	probe, ok := pages.(confluence.SpaceProbe)
	if !ok {
		return true
	}
	return probe.CanCreatePages(ctx, spaceKey) != confluence.PermissionNo
}
```

In `tam/internal/ritualsync/run.go`, add as the last field of `Result`:

```go
	// RootMissing is set, and nothing else is, when the root page answered
	// 404. Nothing was written locally or in Confluence.
	RootMissing *RootMissing `json:"rootMissing"`
```

and replace

```go
	if _, err := pages.GetPageStorage(ctx, cfg.RootID); err != nil {
		return res, fmt.Errorf("The Confluence root page %s could not be read: %s", cfg.RootID, errtext.Line(err))
	}
```

with

```go
	if _, err := pages.GetPageStorage(ctx, cfg.RootID); err != nil {
		if errors.Is(err, confluence.ErrNotFound) {
			res.RootMissing = &RootMissing{
				PageID: cfg.RootID, SpaceKey: cfg.SpaceKey,
				CanCreate: canCreate(ctx, pages, cfg.SpaceKey), SuggestedTitle: SuggestedRootTitle(cfg.ProjectKey),
			}
			return res, nil
		}
		return res, fmt.Errorf("The Confluence root page %s could not be read: %s", cfg.RootID, errtext.Line(err))
	}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd tam && go test ./internal/ritualsync/ -v -run 'TestAMissingRoot|TestSuggestedRootTitle|TestAnUnreadableRoot'`
Expected: PASS.

Run: `cd tam && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/ritualsync/root.go tam/internal/ritualsync/run.go tam/internal/ritualsync/ensure.go tam/internal/ritualsync/run_test.go
git commit -m "feat(tam): a rituals Sync that finds its root page gone reports it in the result"
```

### Task 6: `ritualsync.CreateRoot` creates or adopts a top-level root page

**Files:**
- Modify: `tam/internal/ritualtemplate/ritualtemplate.go` (add `RootBody` after `StandupEntry`)
- Modify: `tam/internal/ritualsync/root.go` (append)
- Test: `tam/internal/ritualtemplate/ritualtemplate_test.go` (append), `tam/internal/ritualsync/root_test.go` (create)

**Interfaces:**
- Consumes: `demo.Confluence.Seed`, `DenyCreate`, `FailNext`, `Page` (Task 4 and existing); `SuggestedRootTitle` (Task 5).
- Produces:
  - `ritualtemplate.RootBody(projectKey string) string`
  - Constants `ritualsync.RootCreated = "created"`, `RootAdopted = "adopted"`, `RootForbidden = "forbidden"`, `RootTitleTaken = "titleTaken"`.
  - `type ritualsync.Root struct { Outcome, PageID, Title, SpaceKey string; TopLevel bool }` with JSON tags `outcome`, `pageId`, `title`, `spaceKey`, `topLevel`.
  - `ritualsync.CreateRoot(ctx context.Context, pages confluence.Pages, spaceKey, projectKey, title string, adopt bool) (Root, error)`. It never writes locally.

- [ ] **Step 1: Write the failing tests**

Append to `tam/internal/ritualtemplate/ritualtemplate_test.go`:

```go
func TestRootBodyNamesTheProjectAndEscapesIt(t *testing.T) {
	want := "<p>Sprint ritual pages for PLAT, kept by Task Activity Manager. Each sprint has a page here, with its Planning, Standup, Review and Retrospective pages beneath it.</p>"
	if got := RootBody("PLAT"); got != want {
		t.Fatalf("RootBody = %q", got)
	}
	if got := RootBody(`A<&"`); !strings.Contains(got, "for A&lt;&amp;&quot;,") {
		t.Fatalf("unescaped: %q", got)
	}
	if got := RootBody(" "); !strings.HasPrefix(got, "<p>Sprint ritual pages, kept by Task Activity Manager.") {
		t.Fatalf("blank project: %q", got)
	}
}
```

`tam/internal/ritualsync/root_test.go`:

```go
package ritualsync

import (
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/ritualtemplate"
)

func TestCreateRootCreatesAtTheTopOfTheSpace(t *testing.T) {
	h := newHarness(t)
	root, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", " PLAT Rituals ", false)
	if err != nil || root.Outcome != RootCreated || root.Title != "PLAT Rituals" || root.SpaceKey != "PLAT" || !root.TopLevel {
		t.Fatalf("root = %+v, %v", root, err)
	}
	page, ok := h.fake.Page(root.PageID)
	if !ok || len(page.AncestorIDs) != 0 || page.Body != ritualtemplate.RootBody("PLAT") || page.Title != "PLAT Rituals" {
		t.Fatalf("page = %+v", page)
	}
}

func TestCreateRootRefusesAnEmptyTitle(t *testing.T) {
	h := newHarness(t)
	if _, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "  ", false); err == nil || err.Error() != "The root page needs a title" {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateRootReportsATokenThatMayNotCreate(t *testing.T) {
	h := newHarness(t)
	h.fake.DenyCreate()
	root, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "PLAT Rituals", false)
	if err != nil || root.Outcome != RootForbidden || root.PageID != "" {
		t.Fatalf("root = %+v, %v", root, err)
	}
}

func TestCreateRootFindsATakenTitleAndSaysWhereItSits(t *testing.T) {
	h := newHarness(t)
	top := h.fake.Seed("", "PLAT Rituals", "<p>by hand</p>")
	root, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "PLAT Rituals", false)
	if err != nil || root.Outcome != RootTitleTaken || root.PageID != top || !root.TopLevel {
		t.Fatalf("top-level taken = %+v, %v", root, err)
	}
	nested := h.fake.Seed("root", "Nested rituals", "<p/>")
	root, err = CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "Nested rituals", false)
	if err != nil || root.Outcome != RootTitleTaken || root.PageID != nested || root.TopLevel {
		t.Fatalf("nested taken = %+v, %v", root, err)
	}
}

func TestCreateRootAdoptsOnlyATopLevelPage(t *testing.T) {
	h := newHarness(t)
	top := h.fake.Seed("", "PLAT Rituals", "<p>by hand</p>")
	root, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "PLAT Rituals", true)
	if err != nil || root.Outcome != RootAdopted || root.PageID != top || !root.TopLevel {
		t.Fatalf("adopted = %+v, %v", root, err)
	}
	nested := h.fake.Seed("root", "Nested rituals", "<p/>")
	root, err = CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "Nested rituals", true)
	if err != nil || root.Outcome != RootTitleTaken || root.PageID != nested || root.TopLevel {
		t.Fatalf("nested adoption = %+v, %v", root, err)
	}
	if _, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "Nobody wrote this", true); err == nil || !strings.HasPrefix(err.Error(), `No page titled "Nobody wrote this" is in PLAT any more`) {
		t.Fatalf("adopting a missing page = %v", err)
	}
}

func TestCreateRootPassesATransportFailureBack(t *testing.T) {
	h := newHarness(t)
	h.fake.FailNext("create", "PLAT Rituals", errors.New("dial tcp: connection refused"))
	if _, err := CreateRoot(h.ctx, h.fake, "PLAT", "PLAT", "PLAT Rituals", false); err == nil || err.Error() != "dial tcp: connection refused" {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam && go test ./internal/ritualtemplate/ ./internal/ritualsync/ -run 'TestRootBody|TestCreateRoot'`
Expected: FAIL to compile, `undefined: RootBody`, `undefined: CreateRoot`.

- [ ] **Step 3: Implement**

In `tam/internal/ritualtemplate/ritualtemplate.go`, after `StandupEntry`:

```go
// RootBody is the body of a rituals root page TAM creates when the configured
// one is missing: one paragraph saying what lives beneath it. Like Render it
// reads no clock, so the same project always gets the same page.
func RootBody(projectKey string) string {
	key := strings.TrimSpace(projectKey)
	subject := "Sprint ritual pages"
	if key != "" {
		subject += " for " + key
	}
	return "<p>" + esc(subject+", kept by Task Activity Manager. Each sprint has a page here, with its Planning, Standup, Review and Retrospective pages beneath it.") + "</p>"
}
```

Append to `tam/internal/ritualsync/root.go`, and extend its imports to `"context"`, `"errors"`, `"fmt"`, `"net/http"`, `"strings"`, `"agile-suite/core/confluence"`, `"agile-suite/tam/internal/ritualtemplate"`:

```go
// Root outcomes. Each is an answer, not an error: forbidden and titleTaken are
// what the dialog words, and Wails delivers a value or an error, never both.
const (
	RootCreated    = "created"
	RootAdopted    = "adopted"
	RootForbidden  = "forbidden"
	RootTitleTaken = "titleTaken"
)

// Root is what an attempt to give the rituals a new root page came to.
type Root struct {
	Outcome  string `json:"outcome"`
	PageID   string `json:"pageId"`
	Title    string `json:"title"`
	SpaceKey string `json:"spaceKey"`
	// TopLevel says whether the page sits at the top of the space. For
	// titleTaken it is the page already holding the title (PageID), and only
	// a top-level one may be adopted.
	TopLevel bool `json:"topLevel"`
}

// CreateRoot creates a rituals root page at the top of the space, or, with
// adopt, takes the top-level page that already carries the title. It writes
// nothing locally: saving the new id to the profile and running the Sync are
// the caller's, under the lock.
//
// A create refused with 403 is forbidden. A create refused with 400 or 409 is
// looked up by title rather than read for its message, since which of the two
// Confluence answers, and in what words, is a probe question
// (docs/superpowers/plans/assets/2026-09-15-confluence-root-page-probe.md);
// a title that turns out to be taken is titleTaken whichever it was.
func CreateRoot(ctx context.Context, pages confluence.Pages, spaceKey, projectKey, title string, adopt bool) (Root, error) {
	out := Root{Title: strings.TrimSpace(title), SpaceKey: spaceKey}
	if out.Title == "" {
		return out, errors.New("The root page needs a title")
	}
	if adopt {
		return adoptRoot(ctx, pages, out)
	}
	created, err := pages.CreatePage(ctx, spaceKey, "", out.Title, ritualtemplate.RootBody(projectKey))
	if err == nil {
		out.Outcome, out.PageID, out.TopLevel = RootCreated, created.ID, true
		return out, nil
	}
	var h *confluence.HTTPError
	if !errors.As(err, &h) {
		return out, err
	}
	switch h.Code {
	case http.StatusForbidden:
		out.Outcome = RootForbidden
		return out, nil
	case http.StatusBadRequest, http.StatusConflict:
		if found, ok, findErr := pages.FindPageByTitle(ctx, spaceKey, out.Title); findErr == nil && ok {
			out.Outcome, out.PageID, out.TopLevel = RootTitleTaken, found.ID, len(found.AncestorIDs) == 0
			return out, nil
		}
	}
	return out, err
}

// adoptRoot takes the page already holding the title, and only from the top
// of the space: the dialog's second confirmation named that placement, and a
// page below another page belongs to somebody else's tree.
func adoptRoot(ctx context.Context, pages confluence.Pages, out Root) (Root, error) {
	found, ok, err := pages.FindPageByTitle(ctx, out.SpaceKey, out.Title)
	if err != nil {
		return out, err
	}
	if !ok {
		return out, fmt.Errorf("No page titled %q is in %s any more. Create the root page instead.", out.Title, out.SpaceKey)
	}
	out.PageID = found.ID
	if len(found.AncestorIDs) != 0 {
		out.Outcome = RootTitleTaken
		return out, nil
	}
	out.Outcome, out.TopLevel = RootAdopted, true
	return out, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd tam && go test ./internal/ritualtemplate/ ./internal/ritualsync/ -v -run 'TestRootBody|TestCreateRoot'`
Expected: PASS. The golden-file tests are untouched since `Render` did not change.

Run: `cd tam && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tam/internal/ritualtemplate/ritualtemplate.go tam/internal/ritualtemplate/ritualtemplate_test.go tam/internal/ritualsync/root.go tam/internal/ritualsync/root_test.go
git commit -m "feat(tam): create or adopt a top-level rituals root page"
```

### Task 7: The `CreateRitualRoot` binding, under the rituals lock

**Files:**
- Modify: `tam/app_rituals.go` (`SyncRituals` at the end of the file; new `syncRitualsLocked`, `RitualRootResult`, `CreateRitualRoot`, `saveRitualRoot`)
- Test: `tam/app_ritualsync_test.go` (append; add imports)
- Regenerate: `tam/frontend/wailsjs/go/main/App.d.ts`, `App.js`, `tam/frontend/wailsjs/go/models.ts`
- Modify: `tam/frontend/src/api.ts` (the rituals block near lines 1086-1129)

**Interfaces:**
- Consumes: `ritualsync.Config.ProjectKey`, `Result.RootMissing` (Task 5); `ritualsync.CreateRoot`, `Root`, `RootCreated`, `RootAdopted` (Task 6); `demo.StagedMissingRootID`, `DenyCreate`, `Seed` (Task 4).
- Produces:
  - Go: `type RitualRootResult struct { Root ritualsync.Root; Sync *ritualsync.Result; SyncError string }` (JSON `root`, `sync`, `syncError`).
  - Go: `(*App).CreateRitualRoot(profileID string, boardID int, title string, adopt bool) (RitualRootResult, error)`, acquiring `"rituals"`.
  - Go: `SyncRituals` records no last-sync time when the result carries `RootMissing`.
  - TS (`api.ts`): `RitualRootMissing`, `RitualSyncResult.rootMissing?: RitualRootMissing | null`, `RitualRootOutcome`, `RitualRoot`, `RitualRootResult`, and `CreateRitualRoot(profileId: string, boardId: number, title: string, adopt: boolean): Promise<RitualRootResult>`.

- [ ] **Step 1: Write the failing tests**

In `tam/app_ritualsync_test.go`, extend the imports with `"agile-suite/tam/internal/demo"` and `"agile-suite/tam/internal/ritualsync"`, then append:

```go
// newMissingRootApp is newRitualSyncApp pointed at the demo space's staged
// missing root, the configuration a stale root page id leaves behind.
func newMissingRootApp(t *testing.T) (*App, profile.Profile) {
	t.Helper()
	a, p := newRitualSyncApp(t)
	if err := a.profiles.SetConfluenceConfig(p.ID, profile.ConfluenceConfig{BaseURL: "demo", SpaceKey: "DEMO", RootPageID: demo.StagedMissingRootID}); err != nil {
		t.Fatal(err)
	}
	return a, p
}

func storedRoot(t *testing.T, a *App, profileID string) profile.ConfluenceConfig {
	t.Helper()
	c, err := a.profiles.ConfluenceConfig(profileID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSyncRitualsReportsAMissingRootWithoutRecordingASync(t *testing.T) {
	a, p := newMissingRootApp(t)
	res, err := a.SyncRituals(p.ID, 1)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := ritualsync.RootMissing{PageID: demo.StagedMissingRootID, SpaceKey: "DEMO", CanCreate: true, SuggestedTitle: "PLAT Rituals"}
	if res.RootMissing == nil || *res.RootMissing != want {
		t.Fatalf("root missing = %+v", res.RootMissing)
	}
	if last, _ := a.LastRitualSync(p.ID, 1); last != "" {
		t.Fatalf("a pass that found no root recorded a sync at %q", last)
	}
	if _, ok := a.busy[p.ID]; ok {
		t.Fatal("the lock was not released")
	}
}

func TestCreateRitualRootSavesTheNewRootAndSyncsUnderIt(t *testing.T) {
	a, p := newMissingRootApp(t)
	out, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false)
	if err != nil || out.Root.Outcome != ritualsync.RootCreated || out.Sync == nil || out.Sync.Created != 5 || out.SyncError != "" {
		t.Fatalf("out = %+v, %v", out, err)
	}
	stored := storedRoot(t, a, p.ID)
	if stored.RootPageID != out.Root.PageID || stored.BaseURL != "demo" || stored.SpaceKey != "DEMO" {
		t.Fatalf("stored = %+v", stored)
	}
	overview := ritualDoc(t, a, p.ID, "_sprint")
	page, ok := a.demoSpace(p.ID, stored).Page(overview.PageID)
	if !ok || len(page.AncestorIDs) != 1 || page.AncestorIDs[0] != out.Root.PageID {
		t.Fatalf("overview page = %+v", page)
	}
	if last, _ := a.LastRitualSync(p.ID, 1); last == "" {
		t.Fatal("the Sync after the create should record its time")
	}
	again, err := a.SyncRituals(p.ID, 1)
	if err != nil || again.RootMissing != nil || again.Created != 0 {
		t.Fatalf("second sync = %+v, %v", again, err)
	}
	if _, ok := a.busy[p.ID]; ok {
		t.Fatal("the lock was not released")
	}
}

func TestCreateRitualRootLeavesTheProfileAloneWhenTheTokenMayNotCreate(t *testing.T) {
	a, p := newMissingRootApp(t)
	a.demoSpace(p.ID, storedRoot(t, a, p.ID)).DenyCreate()
	out, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false)
	if err != nil || out.Root.Outcome != ritualsync.RootForbidden || out.Sync != nil {
		t.Fatalf("out = %+v, %v", out, err)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != demo.StagedMissingRootID {
		t.Fatalf("root page id = %q", got)
	}
}

func TestCreateRitualRootAdoptsATopLevelPageOnlyWhenAsked(t *testing.T) {
	a, p := newMissingRootApp(t)
	existing := a.demoSpace(p.ID, storedRoot(t, a, p.ID)).Seed("", "PLAT Rituals", "<p>by hand</p>")
	out, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false)
	if err != nil || out.Root.Outcome != ritualsync.RootTitleTaken || out.Root.PageID != existing || !out.Root.TopLevel || out.Sync != nil {
		t.Fatalf("taken = %+v, %v", out, err)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != demo.StagedMissingRootID {
		t.Fatalf("a taken title saved root %q", got)
	}
	out, err = a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", true)
	if err != nil || out.Root.Outcome != ritualsync.RootAdopted || out.Sync == nil || out.Sync.Created != 5 {
		t.Fatalf("adopted = %+v, %v", out, err)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != existing {
		t.Fatalf("root page id = %q, want %q", got, existing)
	}
}

func TestCreateRitualRootIsRefusedWhileAnotherOperationHoldsTheLock(t *testing.T) {
	a, p := newMissingRootApp(t)
	a.busy[p.ID] = "sync"
	if _, err := a.CreateRitualRoot(p.ID, 1, "PLAT Rituals", false); err == nil || err.Error() != "a sync is already running for this profile" {
		t.Fatalf("err = %v", err)
	}
	if got := storedRoot(t, a, p.ID).RootPageID; got != demo.StagedMissingRootID {
		t.Fatalf("a refused call saved root %q", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam && go test . -run 'TestSyncRitualsReportsAMissingRoot|TestCreateRitualRoot'`
Expected: FAIL to compile, `a.CreateRitualRoot undefined`. (`TestSyncRitualsReportsAMissingRoot...` would also fail on `last != ""` once it compiles, because `SyncRituals` still records `SyncedAt` unconditionally.)

- [ ] **Step 3: Implement in `tam/app_rituals.go`**

Replace everything in `SyncRituals` from `log.Printf("tam: rituals sync for %s board %d starting", p.ID, boardID)` to the end of the function with:

```go
	return a.syncRitualsLocked(p, boardID, cfg, pages)
}

// syncRitualsLocked is the Sync pass itself, for a caller already holding the
// profile's "rituals" lock: SyncRituals, and CreateRitualRoot once it has
// saved a new root. A pass that found the root missing records no sync time,
// since nothing was synced.
func (a *App) syncRitualsLocked(p profile.Profile, boardID int, cfg profile.ConfluenceConfig, pages confluence.Pages) (ritualsync.Result, error) {
	log.Printf("tam: rituals sync for %s board %d starting", p.ID, boardID)
	cached, err := a.boards.ListSprints(a.ctx, p.ID, boardID)
	if err != nil {
		return ritualsync.Result{}, err
	}
	withRows, err := a.rituals.BoardSprintIDs(a.ctx, p.ID, boardID)
	if err != nil {
		return ritualsync.Result{}, err
	}
	res, err := ritualsync.Run(a.ctx, pages, a.rituals, ritualsync.Config{
		SpaceKey: cfg.SpaceKey, RootID: cfg.RootPageID, ProjectKey: p.ProjectKey, Location: time.Local, Now: time.Now,
	}, p.ID, boardID, ritualsync.Sprints(cached, a.boardName(p.ID, boardID), withRows))
	if err != nil {
		log.Printf("tam: rituals sync for %s board %d refused: %v", p.ID, boardID, err)
		return ritualsync.Result{}, errors.New(errtext.Line(err))
	}
	if res.RootMissing != nil {
		log.Printf("tam: rituals sync for %s board %d stopped: root page %s not found in %s (can create: %v)",
			p.ID, boardID, res.RootMissing.PageID, res.RootMissing.SpaceKey, res.RootMissing.CanCreate)
		return res, nil
	}
	if err := a.repo.SetProfileSetting(a.ctx, p.ID, lastRitualSyncKey(boardID), res.SyncedAt); err != nil {
		log.Printf("tam: record rituals sync time for %s: %v", p.ID, err)
	}
	log.Printf("tam: rituals sync for %s board %d done: %d created, %d pulled, %d pushed, %d conflicts, %d gone, %d failed",
		p.ID, boardID, res.Created, res.Pulled, res.Pushed, res.Conflicts, res.Gone, len(res.Failed))
	return res, nil
}

// RitualRootResult is what CreateRitualRoot came to: the root attempt and,
// when a root was set, the Sync pass run on it straight after. SyncError is
// that pass's refusal in one line. It is not a Go error because by then the
// new root is saved, and a Go error would drop the fact that it was.
type RitualRootResult struct {
	Root      ritualsync.Root    `json:"root"`
	Sync      *ritualsync.Result `json:"sync"`
	SyncError string             `json:"syncError"`
}

// CreateRitualRoot is the missing-root dialog's Create page and sync (adopt
// false) and its Use this page and sync (adopt true). Under the "rituals"
// lock it creates or adopts a top-level page, saves its id as the profile's
// Confluence root page id, and runs the board's Sync on it. Forbidden and a
// taken title come back as the result's outcome, with nothing saved.
// Configuration and credentials are refused before the lock is taken, the way
// SyncRituals refuses them.
func (a *App) CreateRitualRoot(profileID string, boardID int, title string, adopt bool) (RitualRootResult, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return RitualRootResult{}, err
	}
	if err := a.requireRituals(); err != nil {
		return RitualRootResult{}, err
	}
	cfg, pages, err := a.confluencePages(p)
	if err != nil {
		return RitualRootResult{}, err
	}
	if err := a.acquire(p.ID, "rituals"); err != nil {
		return RitualRootResult{}, err
	}
	defer a.release(p.ID)

	root, err := ritualsync.CreateRoot(a.ctx, pages, cfg.SpaceKey, p.ProjectKey, title, adopt)
	if err != nil {
		log.Printf("tam: rituals root for %s refused: %v", p.ID, err)
		return RitualRootResult{}, errors.New(errtext.Line(err))
	}
	out := RitualRootResult{Root: root}
	if root.Outcome != ritualsync.RootCreated && root.Outcome != ritualsync.RootAdopted {
		log.Printf("tam: rituals root for %s in %s: %s", p.ID, cfg.SpaceKey, root.Outcome)
		return out, nil
	}
	if err := a.saveRitualRoot(p.ID, root.PageID); err != nil {
		return RitualRootResult{}, fmt.Errorf("The page %q (%s) is in %s now, but its id could not be saved to the profile: %s. Set it as the root page id in Profile settings.",
			root.Title, root.PageID, cfg.SpaceKey, errtext.Line(err))
	}
	log.Printf("tam: rituals root for %s is now page %s (%s)", p.ID, root.PageID, root.Outcome)
	cfg.RootPageID = root.PageID
	res, err := a.syncRitualsLocked(p, boardID, cfg, pages)
	if err != nil {
		out.SyncError = err.Error()
		return out, nil
	}
	out.Sync = &res
	return out, nil
}

// saveRitualRoot writes a new root page id onto the profile's stored
// Confluence settings, leaving the URL and space key as they are. It reads
// the stored row rather than the configuration the pass ran with, which for a
// demo profile with no Confluence URL is a stand-in never meant to be saved.
func (a *App) saveRitualRoot(profileID, pageID string) error {
	stored, err := a.profiles.ConfluenceConfig(profileID)
	if err != nil {
		return err
	}
	stored.RootPageID = pageID
	return a.profiles.SetConfluenceConfig(profileID, stored)
}
```

`fmt`, `errors`, `log`, `time`, `confluence`, `profile`, `errtext`, `ritualsync` are all already imported by `app_rituals.go`.

- [ ] **Step 4: Run the Go tests to verify they pass**

Run: `cd tam && go test . -run 'TestSyncRituals|TestCreateRitualRoot|TestSaveRitualBody' -v`
Expected: PASS, including the existing `TestSyncRitualsCreatesTheTreeAndRecordsWhen` and `TestSyncRitualsIsRefusedWhileAnotherOperationHoldsTheLock`.

Run: `cd tam && go test ./...`
Expected: PASS.

- [ ] **Step 5: Regenerate the bindings**

Run: `cd tam && wails generate module`
Expected: `git diff --stat tam/frontend/wailsjs` shows `App.d.ts`, `App.js` and `models.ts` changed. `App.d.ts` gains `export function CreateRitualRoot(arg1:string,arg2:number,arg3:string,arg4:boolean):Promise<main.RitualRootResult>;`; `models.ts` gains `ritualsync.RootMissing`, `ritualsync.Root`, a `rootMissing` field on `ritualsync.Result`, and `main.RitualRootResult`. Do not edit these files by hand.

- [ ] **Step 6: Type the binding in `api.ts`**

In `tam/frontend/src/api.ts`, replace

```ts
export interface RitualPageFailure { sprintName: string; title: string; reason: string }
export interface RitualSyncResult {
```

with

```ts
export interface RitualPageFailure { sprintName: string; title: string; reason: string }
// RitualRootMissing is a Sync that stopped because the configured root page
// answered 404. canCreate is the permission probe's answer, unknown read as yes.
export interface RitualRootMissing { pageId: string; spaceKey: string; canCreate: boolean; suggestedTitle: string }
export interface RitualSyncResult {
  // Set, and nothing else is, when the root page is gone. Optional only so
  // fixtures written before it stay valid; Go always sends it, null when the
  // root was read.
  rootMissing?: RitualRootMissing | null;
```

and after the `SyncRituals` export add:

```ts
export type RitualRootOutcome = "created" | "adopted" | "forbidden" | "titleTaken";
export interface RitualRoot { outcome: RitualRootOutcome; pageId: string; title: string; spaceKey: string; topLevel: boolean }
// sync is the pass Go ran on the new root, null when no root was set; syncError
// is that pass's refusal, with the new root saved either way.
export interface RitualRootResult { root: RitualRoot; sync: RitualSyncResult | null; syncError: string }
// CreateRitualRoot takes Go's "rituals" lock. Call it only through
// SyncContext.runRitualRoot, never directly.
export const CreateRitualRoot: (profileId: string, boardId: number, title: string, adopt: boolean) => Promise<RitualRootResult> = App.CreateRitualRoot as any;
```

- [ ] **Step 7: Type-check**

Run (repo root): `npm run typecheck --workspaces --if-present`
Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add tam/app_rituals.go tam/app_ritualsync_test.go tam/frontend/wailsjs tam/frontend/src/api.ts
git commit -m "feat(tam): CreateRitualRoot creates or adopts a root page, saves it, and syncs under the rituals lock"
```

### Task 8: `SyncContext.runRitualRoot` takes the frontend half of the lock

**Files:**
- Modify: `tam/frontend/src/contexts/SyncContext.tsx` (imports at line 16-17, `SyncApi` near line 76, `runRitualsSync` near line 294, the provider value and its dependency list near lines 373-389)
- Test: `tam/frontend/src/contexts/SyncContext.test.tsx`

**Interfaces:**
- Consumes: `CreateRitualRoot`, `RitualRootResult` from `api.ts` (Task 7).
- Produces: `useSync().runRitualRoot(boardId: number, title: string, adopt: boolean): Promise<RitualRootResult>`. It holds the lock as `running === "rituals"`, drives the banner with stage `Creating the rituals root page` (adopt false) or `Using the existing page as the rituals root` (adopt true), and rejects with the busy refusal when the lock is taken. `runRitualsSync` keeps its signature and behaviour.

- [ ] **Step 1: Write the failing tests**

In `tam/frontend/src/contexts/SyncContext.test.tsx`:

1. Add `CreateRitualRoot: vi.fn(),` to the `vi.mock("../api", ...)` object, after `SyncRituals: vi.fn(),`.

2. In `Probe`, add `runRitualRoot` to the `useSync()` destructuring, add state `const [root, setRoot] = React.useState("idle");`, and add inside the returned `<div>`:

```tsx
      <span data-testid="root">{root}</span>
      <button
        onClick={() => {
          setRoot("running");
          void runRitualRoot(1, "PLAT Rituals", false)
            .then((r) => setRoot(r.root.outcome))
            .catch((e) => setRoot(String(e)));
        }}
      >
        Create ritual root
      </button>
```

3. Append inside `describe("SyncProvider")`:

```tsx
  it("holds the lock under rituals while a root page is created", async () => {
    let finish: (v: api.RitualRootResult) => void = () => {};
    vi.mocked(api.CreateRitualRoot).mockImplementation(
      () => new Promise<api.RitualRootResult>((resolve) => { finish = resolve; }),
    );
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));

    await userEvent.click(screen.getByRole("button", { name: "Create ritual root" }));
    await waitFor(() => expect(screen.getByTestId("running")).toHaveTextContent("rituals"));
    expect(screen.getByTestId("status")).toHaveTextContent("syncing");
    expect(screen.getByTestId("stage")).toHaveTextContent("Creating the rituals root page");
    expect(screen.getByRole("button", { name: "Sync" })).toBeDisabled();
    expect(api.CreateRitualRoot).toHaveBeenCalledWith("p1", 1, "PLAT Rituals", false);

    await act(async () => {
      finish({ root: { outcome: "created", pageId: "9001", title: "PLAT Rituals", spaceKey: "DEMO", topLevel: true }, sync: null, syncError: "" });
    });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
    expect(screen.getByTestId("running")).toHaveTextContent("none");
    expect(screen.getByTestId("root")).toHaveTextContent("created");
  });

  it("refuses a root page create while a rituals sync holds the lock", async () => {
    vi.mocked(api.SyncRituals).mockImplementation(() => new Promise<api.RitualSyncResult>(() => {}));
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));

    await userEvent.click(screen.getByRole("button", { name: "Sync rituals" }));
    await waitFor(() => expect(screen.getByTestId("running")).toHaveTextContent("rituals"));
    await userEvent.click(screen.getByRole("button", { name: "Create ritual root" }));
    await waitFor(() => expect(screen.getByTestId("root")).toHaveTextContent(/is already running for this profile/));
    expect(api.CreateRitualRoot).not.toHaveBeenCalled();
  });
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam/frontend && npx vitest run src/contexts/SyncContext.test.tsx`
Expected: FAIL, `runRitualRoot is not a function` (rendered into the `root` test id as a TypeError).

- [ ] **Step 3: Implement**

In `tam/frontend/src/contexts/SyncContext.tsx`:

1. Change the api imports to:

```tsx
import { CommitPendingChanges, CreateRitualRoot, EventsOn, REPORT_PROGRESS_EVENT, SyncBoards, SyncIssues, SyncRituals } from "../api";
import type { BoardSummary, CommitResult, Profile, ReportProgress, RitualRootResult, RitualSyncResult, Settings } from "../api";
```

2. In `interface SyncApi`, directly after `runRitualsSync: (boardId: number) => Promise<RitualSyncResult>;` add:

```tsx
  // runRitualRoot is the missing-root dialog's create (or, with adopt, its
  // adoption) and the Sync Go runs straight after it. CreateRitualRoot
  // acquires Go's "rituals" lock, so this takes the same name and drives the
  // same banner as runRitualsSync; the dialog must never call the binding
  // bare.
  runRitualRoot: (boardId: number, title: string, adopt: boolean) => Promise<RitualRootResult>;
```

3. Replace the whole `const runRitualsSync = useCallback(async (boardId: number): Promise<RitualSyncResult> => { ... }, [activeId, busyRefusal, take, release]);` with:

```tsx
  // Both of the Rituals view's locked calls, the Sync and the root page
  // create that ends in one, hold the lock under "rituals" and drive the
  // banner the same way; only the stage they start it with differs.
  const runRituals = useCallback(async <T,>(stage: string, action: () => Promise<T>): Promise<T> => {
    if (!activeId) throw new Error("no profile selected");
    if (statusRef.current !== "idle") throw busyRefusal();
    take("rituals");
    dispatch({
      type: "SYNC_START",
      clearError: true,
      initialProgress: { phase: "rituals", fetched: 0, total: 0, done: false, stage },
    });
    try {
      return await action();
    } finally {
      release();
      dispatch({ type: "SYNC_END" });
    }
  }, [activeId, busyRefusal, take, release]);

  const runRitualsSync = useCallback(
    (boardId: number): Promise<RitualSyncResult> =>
      runRituals("Syncing rituals with Confluence", () => call(() => SyncRituals(activeId, boardId))),
    [activeId, runRituals],
  );

  const runRitualRoot = useCallback(
    (boardId: number, title: string, adopt: boolean): Promise<RitualRootResult> =>
      runRituals(
        adopt ? "Using the existing page as the rituals root" : "Creating the rituals root page",
        () => call(() => CreateRitualRoot(activeId, boardId, title, adopt)),
      ),
    [activeId, runRituals],
  );
```

4. In the provider value object add `runRitualRoot,` after `runRitualsSync,`, and add `runRitualRoot` to that `useMemo` dependency list after `runRitualsSync`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd tam/frontend && npx vitest run src/contexts/SyncContext.test.tsx`
Expected: PASS, including the existing `holds the lock under rituals while a rituals sync runs`.

Run (repo root): `npm run typecheck --workspaces --if-present`
Expected: no errors. (`RitualsView.test.tsx` mocks `useSync` with an object that lacks `runRitualRoot`; that mock is untyped and still compiles.)

- [ ] **Step 5: Commit**

```bash
git add tam/frontend/src/contexts/SyncContext.tsx tam/frontend/src/contexts/SyncContext.test.tsx
git commit -m "feat(tam): runRitualRoot takes the rituals lock for the root page create"
```

### Task 9: The missing-root dialog and its sentences

**Files:**
- Modify: `tam/frontend/src/lib/ritualText.ts` (append)
- Create: `tam/frontend/src/components/RitualRootDialog.tsx`
- Modify: `tam/frontend/src/App.css` (append after the `.ritual-banner` rules)
- Test: `tam/frontend/src/lib/ritualText.test.ts` (append), `tam/frontend/src/components/RitualRootDialog.test.tsx` (create)

**Interfaces:**
- Consumes: `RitualRoot`, `RitualRootMissing`, `RitualRootResult` from `api.ts` (Task 7); `Modal`, `errMsg` from `@agile-suite/core`.
- Produces:
  - In `ritualText.ts`: `ROOT_TITLE_EMPTY`, `ROOT_AFTER_SENTENCE`, `rootMissingSentence(pageId)`, `rootPlacementSentence(spaceKey)`, `rootForbiddenSentence(spaceKey)`, `rootTakenTopSentence(title, spaceKey)`, `rootTakenNestedSentence(title, spaceKey)`, `rootDoneSentence(root: RitualRoot)`.
  - `RitualRootDialog(props: { missing: RitualRootMissing; create: (title: string, adopt: boolean) => Promise<RitualRootResult>; onDone: () => void; onOpenProfiles: () => void; onClose: () => void })`. Dialog name `Rituals root page not found`; title box named `Page title`; buttons `Create page and sync`, `Use this page and sync`, `Back`, `Open Profile settings`, `Cancel`. `onDone` fires only for `created` or `adopted`, after `create` resolves. `Open Profile settings` calls `onClose` then `onOpenProfiles`.

- [ ] **Step 1: Write the failing tests**

Append to `tam/frontend/src/lib/ritualText.test.ts`, extending its import from `./ritualText` with `ROOT_AFTER_SENTENCE, rootDoneSentence, rootForbiddenSentence, rootMissingSentence, rootPlacementSentence, rootTakenNestedSentence, rootTakenTopSentence`:

```ts
describe("root page text", () => {
  it("words the missing root, where the new one goes, and what Sync does next", () => {
    expect(rootMissingSentence("653264152")).toBe("The Confluence root page 653264152 could not be found. It may have been deleted, moved out of reach, or mistyped.");
    expect(rootPlacementSentence("TEAM")).toBe("The new page goes at the top of the TEAM space, and its id is saved to this profile as the rituals root.");
    expect(ROOT_AFTER_SENTENCE).toBe("Sync runs again straight after. This sprint's ritual pages are created under the new root; a page that was under the old root and is gone shows as Gone, and Recreate on next Sync puts it under the new root.");
  });

  it("words the refusals", () => {
    expect(rootForbiddenSentence("TEAM")).toBe("Your token cannot create pages in TEAM. Ask a space admin, or set an existing page id in Profile settings.");
    expect(rootTakenTopSentence("PLAT Rituals", "TEAM")).toBe('A page titled "PLAT Rituals" is already at the top of TEAM. Use that page as the rituals root? Its id is saved to this profile and Sync runs under it.');
    expect(rootTakenNestedSentence("PLAT Rituals", "TEAM")).toBe(`A page titled "PLAT Rituals" already exists in TEAM, below another page. Choose a different title, or set that page's id in Profile settings.`);
  });

  it("says which root was set", () => {
    const root = { outcome: "created" as const, pageId: "9001", title: "PLAT Rituals", spaceKey: "TEAM", topLevel: true };
    expect(rootDoneSentence(root)).toBe('Created "PLAT Rituals" at the top of TEAM and saved it as the rituals root.');
    expect(rootDoneSentence({ ...root, outcome: "adopted" })).toBe('Using "PLAT Rituals" in TEAM as the rituals root.');
  });

  it("prints no em dash anywhere", () => {
    const all = [rootMissingSentence("1"), rootPlacementSentence("T"), ROOT_AFTER_SENTENCE, rootForbiddenSentence("T"), rootTakenTopSentence("a", "T"), rootTakenNestedSentence("a", "T")];
    for (const s of all) expect(s).not.toContain(String.fromCharCode(0x2014));
  });
});
```

`tam/frontend/src/components/RitualRootDialog.test.tsx`:

```tsx
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { RitualRoot, RitualRootMissing, RitualRootResult } from "../api";
import {
  ROOT_TITLE_EMPTY, rootForbiddenSentence, rootMissingSentence, rootPlacementSentence, rootTakenNestedSentence, rootTakenTopSentence,
} from "../lib/ritualText";
import { RitualRootDialog } from "./RitualRootDialog";

const missing = (over: Partial<RitualRootMissing> = {}): RitualRootMissing => ({
  pageId: "653264152", spaceKey: "TEAM", canCreate: true, suggestedTitle: "PLAT Rituals", ...over,
});

const outcome = (over: Partial<RitualRoot> = {}): RitualRootResult => ({
  root: { outcome: "created", pageId: "9001", title: "PLAT Rituals", spaceKey: "TEAM", topLevel: true, ...over },
  sync: null,
  syncError: "",
});

type Props = ComponentProps<typeof RitualRootDialog>;

function renderDialog(over: Partial<Props> = {}) {
  const props: Props = {
    missing: missing(),
    create: vi.fn(async (_title: string, _adopt: boolean) => outcome()),
    onDone: vi.fn(),
    onOpenProfiles: vi.fn(),
    onClose: vi.fn(),
    ...over,
  };
  render(<RitualRootDialog {...props} />);
  return props;
}

describe("RitualRootDialog", () => {
  it("states the placement and creates the root with the suggested title", async () => {
    const props = renderDialog();
    expect(screen.getByRole("dialog", { name: "Rituals root page not found" })).toBeInTheDocument();
    expect(screen.getByText(rootMissingSentence("653264152"))).toBeInTheDocument();
    expect(screen.getByText(rootPlacementSentence("TEAM"))).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Page title" })).toHaveValue("PLAT Rituals");
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(props.create).toHaveBeenCalledWith("PLAT Rituals", false);
    await waitFor(() => expect(props.onDone).toHaveBeenCalledTimes(1));
  });

  it("sends an edited title trimmed, and refuses an empty one", async () => {
    const props = renderDialog();
    const box = screen.getByRole("textbox", { name: "Page title" });
    await userEvent.clear(box);
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(screen.getByRole("alert")).toHaveTextContent(ROOT_TITLE_EMPTY);
    expect(props.create).not.toHaveBeenCalled();
    await userEvent.type(box, "  Team rituals  {Enter}");
    expect(props.create).toHaveBeenCalledWith("Team rituals", false);
  });

  it("offers Profile settings instead of a create when the probe says the token cannot create", async () => {
    const props = renderDialog({ missing: missing({ canCreate: false }) });
    expect(screen.getByText(rootForbiddenSentence("TEAM"))).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Create page and sync" })).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Open Profile settings" }));
    expect(props.onClose).toHaveBeenCalled();
    expect(props.onOpenProfiles).toHaveBeenCalled();
  });

  it("shows the forbidden sentence when the create itself is refused", async () => {
    const props = renderDialog({ create: vi.fn(async () => outcome({ outcome: "forbidden", pageId: "" })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(await screen.findByText(rootForbiddenSentence("TEAM"))).toBeInTheDocument();
    expect(props.onDone).not.toHaveBeenCalled();
  });

  it("asks a second time before adopting a top-level page with the same title", async () => {
    const create = vi.fn(async (_title: string, adopt: boolean) => outcome(adopt ? { outcome: "adopted", pageId: "77" } : { outcome: "titleTaken", pageId: "77" }));
    const props = renderDialog({ create });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(await screen.findByText(rootTakenTopSentence("PLAT Rituals", "TEAM"))).toBeInTheDocument();
    expect(props.onDone).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Use this page and sync" }));
    expect(create).toHaveBeenLastCalledWith("PLAT Rituals", true);
    await waitFor(() => expect(props.onDone).toHaveBeenCalledTimes(1));
  });

  it("does not offer to adopt a same-title page below another page", async () => {
    renderDialog({ create: vi.fn(async () => outcome({ outcome: "titleTaken", pageId: "77", topLevel: false })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(await screen.findByText(rootTakenNestedSentence("PLAT Rituals", "TEAM"))).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Use this page and sync" })).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByRole("textbox", { name: "Page title" })).toBeInTheDocument();
  });

  it("refuses Escape while its call is in flight, and shows a failure with the title kept", async () => {
    let fail: (e: Error) => void = () => {};
    const props = renderDialog({ create: vi.fn(() => new Promise<RitualRootResult>((_resolve, reject) => { fail = reject; })) });
    await userEvent.click(screen.getByRole("button", { name: "Create page and sync" }));
    expect(screen.getByRole("button", { name: "Creating page…" })).toBeDisabled();
    await userEvent.keyboard("{Escape}");
    expect(props.onClose).not.toHaveBeenCalled();
    await act(async () => { fail(new Error("network down")); });
    expect(await screen.findByRole("alert")).toHaveTextContent("network down");
    expect(screen.getByRole("textbox", { name: "Page title" })).toHaveValue("PLAT Rituals");
    expect(screen.getByRole("button", { name: "Create page and sync" })).toBeEnabled();
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam/frontend && npx vitest run src/lib/ritualText.test.ts src/components/RitualRootDialog.test.tsx`
Expected: FAIL, `rootMissingSentence is not a function` and `Failed to resolve import "./RitualRootDialog"`.

- [ ] **Step 3: Add the sentences**

Change the first line of `tam/frontend/src/lib/ritualText.ts` to `import type { RitualDocument, RitualRoot, RitualStatus, RitualSyncResult } from "../api";` and append:

```ts
// The missing root page. Placement is stated before anything is written, and
// the forbidden sentence is the same whether the probe said so up front or
// the create was refused.
export const ROOT_TITLE_EMPTY = "The root page needs a title.";
export const ROOT_AFTER_SENTENCE = "Sync runs again straight after. This sprint's ritual pages are created under the new root; a page that was under the old root and is gone shows as Gone, and Recreate on next Sync puts it under the new root.";

export function rootMissingSentence(pageId: string): string {
  return `The Confluence root page ${pageId} could not be found. It may have been deleted, moved out of reach, or mistyped.`;
}

export function rootPlacementSentence(spaceKey: string): string {
  return `The new page goes at the top of the ${spaceKey} space, and its id is saved to this profile as the rituals root.`;
}

export function rootForbiddenSentence(spaceKey: string): string {
  return `Your token cannot create pages in ${spaceKey}. Ask a space admin, or set an existing page id in Profile settings.`;
}

export function rootTakenTopSentence(title: string, spaceKey: string): string {
  return `A page titled "${title}" is already at the top of ${spaceKey}. Use that page as the rituals root? Its id is saved to this profile and Sync runs under it.`;
}

export function rootTakenNestedSentence(title: string, spaceKey: string): string {
  return `A page titled "${title}" already exists in ${spaceKey}, below another page. Choose a different title, or set that page's id in Profile settings.`;
}

export function rootDoneSentence(root: RitualRoot): string {
  return root.outcome === "adopted"
    ? `Using "${root.title}" in ${root.spaceKey} as the rituals root.`
    : `Created "${root.title}" at the top of ${root.spaceKey} and saved it as the rituals root.`;
}
```

- [ ] **Step 4: Write the dialog**

`tam/frontend/src/components/RitualRootDialog.tsx`:

```tsx
import { useId, useState } from "react";
import { Modal, errMsg } from "@agile-suite/core";
import type { RitualRootMissing, RitualRootResult } from "../api";
import {
  ROOT_AFTER_SENTENCE, ROOT_TITLE_EMPTY, rootForbiddenSentence, rootMissingSentence, rootPlacementSentence,
  rootTakenNestedSentence, rootTakenTopSentence,
} from "../lib/ritualText";

type Step =
  | { kind: "confirm" }
  | { kind: "working"; adopt: boolean }
  | { kind: "forbidden" }
  | { kind: "taken"; title: string; topLevel: boolean };

interface Props {
  missing: RitualRootMissing;
  // create is the root create (adopt false) or adoption (adopt true) with the
  // Sync Go runs after it. The caller runs it through SyncContext.runRitualRoot.
  create: (title: string, adopt: boolean) => Promise<RitualRootResult>;
  // onDone fires once a root was set and the caller has applied what came back.
  onDone: () => void;
  onOpenProfiles: () => void;
  onClose: () => void;
}

// RitualRootDialog is what a Sync that found its root page gone opens. It
// states where a new page goes before anything is written, offers the create
// only when the permission probe did not say no, and asks a second time
// before adopting a page somebody else wrote. It refuses Escape and the
// overlay while its call is in flight, the way the sprint dialogs do: the call
// holds the profile lock, and closing under it would leave its outcome with
// nowhere to land.
export function RitualRootDialog({ missing, create, onDone, onOpenProfiles, onClose }: Props) {
  const titleId = useId();
  const [title, setTitle] = useState(missing.suggestedTitle);
  const [step, setStep] = useState<Step>(missing.canCreate ? { kind: "confirm" } : { kind: "forbidden" });
  const [error, setError] = useState("");
  const working = step.kind === "working";
  const editing = step.kind === "confirm" || step.kind === "working";

  async function run(value: string, adopt: boolean) {
    const trimmed = value.trim();
    if (!trimmed) {
      setError(ROOT_TITLE_EMPTY);
      return;
    }
    setError("");
    setStep({ kind: "working", adopt });
    try {
      const result = await create(trimmed, adopt);
      switch (result.root.outcome) {
        case "created":
        case "adopted":
          onDone();
          return;
        case "forbidden":
          setStep({ kind: "forbidden" });
          return;
        case "titleTaken":
          setStep({ kind: "taken", title: trimmed, topLevel: result.root.topLevel });
          return;
      }
    } catch (e) {
      setError(errMsg(e));
      setStep({ kind: "confirm" });
    }
  }

  const openProfiles = () => {
    onClose();
    onOpenProfiles();
  };

  return (
    <Modal onClose={onClose} className="modal ritual-root-modal" labelledBy={titleId} closeOnEsc={!working} closeOnOverlayClick={!working}>
      <h2 id={titleId}>Rituals root page not found</h2>
      <p>{rootMissingSentence(missing.pageId)}</p>
      {editing && (
        <>
          <label className="ritual-root-title">
            Page title
            <input
              className="detail-input"
              value={title}
              disabled={working}
              spellCheck={false}
              onChange={(e) => setTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !working) {
                  e.preventDefault();
                  void run(title, false);
                }
              }}
            />
          </label>
          <p className="muted small">{rootPlacementSentence(missing.spaceKey)}</p>
          <p className="muted small">{ROOT_AFTER_SENTENCE}</p>
        </>
      )}
      {step.kind === "forbidden" && <p className="warn-text">{rootForbiddenSentence(missing.spaceKey)}</p>}
      {step.kind === "taken" && (
        <p className="warn-text">
          {step.topLevel ? rootTakenTopSentence(step.title, missing.spaceKey) : rootTakenNestedSentence(step.title, missing.spaceKey)}
        </p>
      )}
      {error && <p className="error-text" role="alert">{error}</p>}
      <div className="form-actions form-actions-end">
        {step.kind === "taken" && <button className="btn" onClick={() => setStep({ kind: "confirm" })}>Back</button>}
        {(step.kind === "taken" || step.kind === "forbidden") && <button className="btn" onClick={openProfiles}>Open Profile settings</button>}
        <button className="btn" onClick={onClose} disabled={working}>Cancel</button>
        {editing && (
          <button className="btn btn-primary" onClick={() => void run(title, false)} disabled={working}>
            {step.kind === "working" ? (step.adopt ? "Using page…" : "Creating page…") : "Create page and sync"}
          </button>
        )}
        {step.kind === "taken" && step.topLevel && (
          <button className="btn btn-primary" onClick={() => void run(step.title, true)}>Use this page and sync</button>
        )}
      </div>
    </Modal>
  );
}
```

Append to `tam/frontend/src/App.css`, after the `.ritual-banner` rules:

```css
/* The missing-root dialog: a title box, where the page goes, what Sync does next. */
.ritual-root-modal { max-width: 480px; }
.ritual-root-modal .ritual-root-title { display: flex; flex-direction: column; gap: 4px; margin: 8px 0; }
.ritual-root-modal p { line-height: 1.45; }
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd tam/frontend && npx vitest run src/lib/ritualText.test.ts src/components/RitualRootDialog.test.tsx`
Expected: PASS.

Run (repo root): `npm run typecheck --workspaces --if-present`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add tam/frontend/src/lib/ritualText.ts tam/frontend/src/lib/ritualText.test.ts tam/frontend/src/components/RitualRootDialog.tsx tam/frontend/src/components/RitualRootDialog.test.tsx tam/frontend/src/App.css
git commit -m "feat(tam): a dialog that offers to create or adopt a missing rituals root page"
```

### Task 10: The Rituals view opens the dialog and applies what it did

**Files:**
- Modify: `tam/frontend/src/components/RitualsView.tsx`
- Test: `tam/frontend/src/components/RitualsView.test.tsx`

**Interfaces:**
- Consumes: `useSync().runRitualRoot` (Task 8); `RitualRootDialog` (Task 9); `rootDoneSentence`, `rootMissingSentence` (Task 9); `RitualRootMissing`, `RitualRootResult` (Task 7); `useModal` from `../modals` (existing, `ModalId` includes `"profiles"`).
- Produces: the view behaviour below. No new exports.
  - A Sync whose result carries `rootMissing` opens `RitualRootDialog`, shows no Sync summary, records no last-sync time, and does not reload documents.
  - The dialog's `create` runs `runRitualRoot(boardId, title, adopt)` for the board the Sync was started on, flushing the editor first and keeping it locked (`syncPressed`) until the reload lands. On `created` or `adopted` it updates the in-memory config's `rootPageID`, announces `rootDoneSentence`, shows the Sync summary or `syncError` when the board and sprint are still current, and reloads documents.
  - `Open Profile settings` closes the dialog and calls `openModal("profiles")`.

- [ ] **Step 1: Write the failing tests**

In `tam/frontend/src/components/RitualsView.test.tsx`:

1. Change `const sync = vi.hoisted(() => ({ running: null as string | null, runRitualsSync: vi.fn() }));` to

```tsx
const sync = vi.hoisted(() => ({ running: null as string | null, runRitualsSync: vi.fn(), runRitualRoot: vi.fn() }));
```

and after the `vi.mock("../contexts/SyncContext", ...)` line add

```tsx
const modal = vi.hoisted(() => ({ openModal: vi.fn() }));
vi.mock("../modals", () => ({ useModal: () => modal }));
```

2. Extend the ritualText import with `rootForbiddenSentence`.

3. Append inside `describe("RitualsView")`:

```tsx
  const rootMissingResult = (canCreate: boolean): api.RitualSyncResult => ({
    created: 0, pulled: 0, pushed: 0, conflicts: 0, gone: 0, failed: [], syncedAt: "",
    rootMissing: { pageId: "653264152", spaceKey: "TEAM", canCreate, suggestedTitle: "PLAT Rituals" },
  });

  it("opens the root dialog when Sync finds the root missing, then creates it and syncs through the lock", async () => {
    sync.runRitualsSync.mockResolvedValue(rootMissingResult(true));
    sync.runRitualRoot.mockResolvedValue({
      root: { outcome: "created", pageId: "9001", title: "PLAT Rituals", spaceKey: "TEAM", topLevel: true },
      sync: { created: 5, pulled: 0, pushed: 0, conflicts: 0, gone: 0, failed: [], syncedAt: "2026-09-15T10:00:00Z", rootMissing: null },
      syncError: "",
    } satisfies api.RitualRootResult);
    renderView();
    await screen.findByTestId("editor");
    await userEvent.click(screen.getByRole("button", { name: "Sync rituals" }));
    const dialog = await screen.findByRole("dialog", { name: "Rituals root page not found" });
    expect(screen.queryByText(/Sync finished/)).toBeNull();
    expect(api.EnsureSprintRituals).toHaveBeenCalledTimes(1);

    await userEvent.click(within(dialog).getByRole("button", { name: "Create page and sync" }));
    expect(sync.runRitualRoot).toHaveBeenCalledWith(1, "PLAT Rituals", false);
    expect(await screen.findByText("Sync finished: 5 created.")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(api.EnsureSprintRituals).toHaveBeenCalledTimes(2);
  });

  it("sends a token that cannot create pages to Profile settings", async () => {
    sync.runRitualsSync.mockResolvedValue(rootMissingResult(false));
    renderView();
    await screen.findByTestId("editor");
    await userEvent.click(screen.getByRole("button", { name: "Sync rituals" }));
    const dialog = await screen.findByRole("dialog", { name: "Rituals root page not found" });
    expect(within(dialog).getByText(rootForbiddenSentence("TEAM"))).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Create page and sync" })).toBeNull();
    await userEvent.click(within(dialog).getByRole("button", { name: "Open Profile settings" }));
    expect(modal.openModal).toHaveBeenCalledWith("profiles");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(sync.runRitualRoot).not.toHaveBeenCalled();
  });

  it("keeps the editor locked while the root is created and until the reload lands", async () => {
    sync.runRitualsSync.mockResolvedValue(rootMissingResult(true));
    let finish: (r: api.RitualRootResult) => void = () => {};
    sync.runRitualRoot.mockImplementation(() => new Promise<api.RitualRootResult>((resolve) => { finish = resolve; }));
    renderView();
    await screen.findByTestId("editor");
    await userEvent.click(screen.getByRole("button", { name: "Sync rituals" }));
    const dialog = await screen.findByRole("dialog", { name: "Rituals root page not found" });
    await waitFor(() => expect(screen.getByTestId("editor")).toHaveAttribute("data-locked", "false"));
    await userEvent.click(within(dialog).getByRole("button", { name: "Create page and sync" }));
    await waitFor(() => expect(screen.getByTestId("editor")).toHaveAttribute("data-locked", "true"));
    await act(async () => {
      finish({
        root: { outcome: "created", pageId: "9001", title: "PLAT Rituals", spaceKey: "TEAM", topLevel: true },
        sync: { created: 5, pulled: 0, pushed: 0, conflicts: 0, gone: 0, failed: [], syncedAt: "2026-09-15T10:00:00Z", rootMissing: null },
        syncError: "",
      });
    });
    await waitFor(() => expect(screen.getByTestId("editor")).toHaveAttribute("data-locked", "false"));
  });
```

Add `act` to the `@testing-library/react` import at the top of the file.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam/frontend && npx vitest run src/components/RitualsView.test.tsx`
Expected: FAIL. The new tests time out on `findByRole("dialog", { name: "Rituals root page not found" })`, and the first also finds `Sync finished. Everything was already in step.` on screen.

- [ ] **Step 3: Implement**

In `tam/frontend/src/components/RitualsView.tsx`:

1. Imports. Change the api type import to

```tsx
import type { Board, ConfluenceConfig, Profile, RitualDocument, RitualRootMissing, RitualRootResult, RitualSyncResult, Settings, Sprint } from "../api";
```

extend the ritualText import with `rootDoneSentence, rootMissingSentence`, and add

```tsx
import { useModal } from "../modals";
import { RitualRootDialog } from "./RitualRootDialog";
```

2. Change `const { runRitualsSync, running } = useSync();` to

```tsx
  const { runRitualsSync, runRitualRoot, running } = useSync();
  const { openModal } = useModal();
```

3. After `const editorRef = useRef<RitualEditorHandle>(null);` add

```tsx
  // rootMissing is set when a Sync found the configured root page gone, and
  // opens the dialog. rootAt is the board and sprint that Sync was started
  // on, so the create and the Sync after it answer for those, not for
  // whatever the pickers moved to while the dialog was open.
  const [rootMissing, setRootMissing] = useState<RitualRootMissing | null>(null);
  const rootAt = useRef<Captured | null>(null);
```

4. In the `[activeId]` effect, change the first line to

```tsx
    setConfig(null); setBoards(null); setBoardId(0); setError(""); setResult(null); setRootMissing(null);
```

5. In `sync()`, replace

```tsx
      const res = await runRitualsSync(at.boardId);
      if (at.current()) {
```

with

```tsx
      const res = await runRitualsSync(at.boardId);
      if (res.rootMissing) {
        // Nothing was synced, so there is no summary, no sync time, and
        // nothing to reload; the dialog is what happens next.
        if (at.current()) {
          rootAt.current = at;
          setRootMissing(res.rootMissing);
        }
        announce(rootMissingSentence(res.rootMissing.pageId));
        return;
      }
      if (at.current()) {
```

6. After the `sync()` function add:

```tsx
  // createRoot is the dialog's create (or adoption), through the rituals lock,
  // and applies the Sync Go ran on the new root the way sync() applies its
  // own. The editor stays locked from the press until the reload lands, for
  // the reason syncPressed exists. A failed reload is shown on the view, not
  // handed back to the dialog: the root is already set by then, and a dialog
  // offering Create again would only meet its own page as a taken title.
  async function createRoot(title: string, adopt: boolean): Promise<RitualRootResult> {
    const at = rootAt.current ?? capture();
    setSyncPressed(true);
    try {
      await editorRef.current?.flush();
      const out = await runRitualRoot(at.boardId, title, adopt);
      if (out.root.outcome === "created" || out.root.outcome === "adopted") {
        setConfig((c) => (c ? { ...c, rootPageID: out.root.pageId } : c));
        announce(rootDoneSentence(out.root));
        if (at.current()) {
          if (out.sync) {
            setResult(out.sync);
            setLastSync(out.sync.syncedAt);
          }
          setError(out.syncError);
        }
        try {
          await reloadDocs(at);
        } catch (e) {
          if (at.current()) setError(errMsg(e));
        }
      }
      return out;
    } finally {
      setSyncPressed(false);
    }
  }

  const closeRootDialog = () => {
    setRootMissing(null);
    rootAt.current = null;
  };
```

7. Directly before the closing `</section>` of the main return (the one rendering `rituals-layout`), add:

```tsx
      {rootMissing && (
        <RitualRootDialog
          missing={rootMissing}
          create={createRoot}
          onDone={closeRootDialog}
          onClose={closeRootDialog}
          onOpenProfiles={() => openModal("profiles")}
        />
      )}
```

`type Captured` is declared later in the component body; TypeScript resolves type aliases regardless of position, so `useRef<Captured | null>` above it compiles.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd tam/frontend && npx vitest run src/components/RitualsView.test.tsx`
Expected: PASS, including every existing Rituals view test (`syncs through the lock, reports what happened, and reloads` still sees `Sync finished: 5 created, 1 failed.`, since its result has no `rootMissing`).

Run: `cd tam/frontend && npx vitest run` and (repo root) `npm run typecheck --workspaces --if-present`
Expected: all pass, no type errors.

- [ ] **Step 5: Manual check on the demo profile**

Run `cd tam && wails dev`. On a demo profile open Manage, set Confluence base URL `demo`, space key `DEMO`, root page ID `1`, save. Open Rituals, press **Sync rituals**. Expected: the dialog opens with title `PLAT Rituals` (or the demo project's key) and the placement sentence; **Create page and sync** closes it and the banner reads `Sync finished: 5 created.`; Manage shows the root page ID changed from `1` to the new page's id; a second **Sync rituals** reports nothing new. While the create runs, the toolbar is drawn disabled and the shell's Sync button is disabled.

- [ ] **Step 6: Commit**

```bash
git add tam/frontend/src/components/RitualsView.tsx tam/frontend/src/components/RitualsView.test.tsx
git commit -m "feat(tam): the Rituals view offers to create a missing root page and syncs under it"
```

### Task 11: Profile settings refuse a root page id that is not a number, and read a pasted address

**Files:**
- Create: `tam/frontend/src/lib/confluenceRoot.ts`
- Modify: `tam/frontend/src/components/ProfileForm.tsx` (after `const urlError` near line 160, `canSave` near line 172, `save()` near line 238, the Root page ID label near line 316, the error block near line 410)
- Test: `tam/frontend/src/lib/confluenceRoot.test.ts` (create), `tam/frontend/src/components/ProfileForm.test.tsx` (create)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `readRootPageInput(value: string, confluenceUrl: string): { id: string; error: string }`
  - `ROOT_ID_NOT_A_NUMBER`, `ROOT_URL_WITHOUT_ID`, `ROOT_FIX_BEFORE_SAVE` (sentences in Global Constraints).
  - Rules: blank is fine (`id: ""`); Confluence URL `demo` (any case) accepts any text; all digits is the id; an `http`/`https` address yields its `pageId` query value when that is all digits, else `ROOT_URL_WITHOUT_ID`; anything else is `ROOT_ID_NOT_A_NUMBER`.

- [ ] **Step 1: Write the failing tests**

`tam/frontend/src/lib/confluenceRoot.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { ROOT_ID_NOT_A_NUMBER, ROOT_URL_WITHOUT_ID, readRootPageInput } from "./confluenceRoot";

const CONF = "https://confluence.example.com";

describe("readRootPageInput", () => {
  it("accepts a page id, trimmed", () => {
    expect(readRootPageInput(" 653264152 ", CONF)).toEqual({ id: "653264152", error: "" });
  });

  it("reads the pageId out of a pasted page address", () => {
    expect(readRootPageInput(`${CONF}/pages/viewpage.action?pageId=653264152`, CONF)).toEqual({ id: "653264152", error: "" });
    expect(readRootPageInput(`${CONF}/pages/viewinfo.action?pageId=42&src=contextnavpagetreemode`, "")).toEqual({ id: "42", error: "" });
  });

  it("refuses an address that carries no page id", () => {
    expect(readRootPageInput(`${CONF}/display/TEAM/Rituals`, CONF)).toEqual({ id: "", error: ROOT_URL_WITHOUT_ID });
    expect(readRootPageInput(`${CONF}/pages/viewpage.action?pageId=abc`, CONF)).toEqual({ id: "", error: ROOT_URL_WITHOUT_ID });
  });

  it("refuses text that is not a number", () => {
    for (const value of ["Team rituals", "12a", "-5", "1.5"]) {
      expect(readRootPageInput(value, CONF)).toEqual({ id: "", error: ROOT_ID_NOT_A_NUMBER });
    }
  });

  it("leaves a blank field alone", () => {
    expect(readRootPageInput("   ", CONF)).toEqual({ id: "", error: "" });
  });

  it("accepts the demo space's own root id", () => {
    expect(readRootPageInput("demo-root", "Demo")).toEqual({ id: "demo-root", error: "" });
  });
});
```

`tam/frontend/src/components/ProfileForm.test.tsx`:

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../api";
import type { Profile } from "../api";
import { ROOT_FIX_BEFORE_SAVE, ROOT_ID_NOT_A_NUMBER } from "../lib/confluenceRoot";
import { ProfileForm } from "./ProfileForm";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    UpdateProfile: vi.fn(),
    TestConnection: vi.fn(),
    TestProfileConnection: vi.fn(),
    GetProfileSetting: vi.fn(),
    SetProfileSetting: vi.fn(),
    GetConfluenceConfig: vi.fn(),
    SetConfluenceConfig: vi.fn(),
  };
});

const acme: Profile = {
  id: "p1",
  name: "Acme Platform",
  jiraUrl: "https://jira.acme.example",
  projectKey: "PLAT",
  backend: "xray",
  createdAt: "",
  scopeJql: "",
  caCert: "",
  allowUntrustedTls: false,
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.SetProfileSetting).mockResolvedValue();
  vi.mocked(api.UpdateProfile).mockResolvedValue(acme);
  vi.mocked(api.GetConfluenceConfig).mockResolvedValue({ baseURL: "https://confluence.example.com", spaceKey: "TEAM", rootPageID: "653264152" });
  vi.mocked(api.SetConfluenceConfig).mockResolvedValue();
});

describe("ProfileForm's Confluence root page id", () => {
  it("reads a pasted page address as its page id and saves the id", async () => {
    const onSaved = vi.fn();
    render(<ProfileForm profile={acme} onSaved={onSaved} />);
    const root = await screen.findByDisplayValue("653264152");
    await userEvent.clear(root);
    await userEvent.type(root, "https://confluence.example.com/pages/viewpage.action?pageId=42");
    await userEvent.tab();
    expect(root).toHaveValue("42");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() =>
      expect(api.SetConfluenceConfig).toHaveBeenCalledWith("p1", { baseURL: "https://confluence.example.com", spaceKey: "TEAM", rootPageID: "42" }, ""),
    );
    expect(onSaved).toHaveBeenCalled();
  });

  it("refuses a root page id that is not a number, and says so beside Save", async () => {
    render(<ProfileForm profile={acme} onSaved={vi.fn()} />);
    const root = await screen.findByDisplayValue("653264152");
    await userEvent.clear(root);
    await userEvent.type(root, "Team rituals");
    expect(screen.getByText(ROOT_ID_NOT_A_NUMBER)).toBeInTheDocument();
    expect(screen.getByText(ROOT_FIX_BEFORE_SAVE)).toBeInTheDocument();
    expect(root).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd tam/frontend && npx vitest run src/lib/confluenceRoot.test.ts src/components/ProfileForm.test.tsx`
Expected: FAIL, `Failed to resolve import "../lib/confluenceRoot"` / `"./confluenceRoot"`.

- [ ] **Step 3: Implement**

`tam/frontend/src/lib/confluenceRoot.ts`:

```ts
// The Confluence root page id field in Profile settings, read the way
// Confluence hands an id to a person: typed as a number, or inside a page
// address copied from the browser. Every sentence the form prints about the
// field lives here, so the rules are testable without rendering the form.

export const ROOT_ID_NOT_A_NUMBER = "The root page id is a number, such as 123456. You can also paste the page's address.";
export const ROOT_URL_WITHOUT_ID = "That address carries no page id. In Confluence, open the page's Page Information and copy that address, which ends in pageId=123456.";
export const ROOT_FIX_BEFORE_SAVE = "Fix the Confluence root page id under Confluence Rituals before saving.";

export interface RootPageInput {
  id: string;
  error: string;
}

const DIGITS = /^\d+$/;

export function readRootPageInput(value: string, confluenceUrl: string): RootPageInput {
  const text = value.trim();
  if (text === "") return { id: "", error: "" };
  // The demo space's own root is "demo-root", and nothing reaches a server.
  if (confluenceUrl.trim().toLowerCase() === "demo") return { id: text, error: "" };
  if (DIGITS.test(text)) return { id: text, error: "" };
  if (/^https?:\/\//i.test(text)) {
    let url: URL;
    try {
      url = new URL(text);
    } catch {
      return { id: "", error: ROOT_URL_WITHOUT_ID };
    }
    const pageId = url.searchParams.get("pageId") ?? "";
    return DIGITS.test(pageId) ? { id: pageId, error: "" } : { id: "", error: ROOT_URL_WITHOUT_ID };
  }
  return { id: "", error: ROOT_ID_NOT_A_NUMBER };
}
```

In `tam/frontend/src/components/ProfileForm.tsx`:

1. After the `../api` import add `import { ROOT_FIX_BEFORE_SAVE, readRootPageInput } from "../lib/confluenceRoot";`.

2. After `const urlError = jiraUrlError(jiraUrl);` add

```tsx
  // A Sync against a root page id that is not one answers 404 and offers to
  // create a root nobody needed, so the id is checked where it is typed.
  const rootInput = readRootPageInput(confluenceRootPageID, confluenceURL);
```

3. In `canSave`, change the last line `tokenSatisfied;` to

```tsx
    tokenSatisfied &&
    rootInput.error === "";
```

4. In `save()`, change `rootPageID: confluenceRootPageID.trim(),` to `rootPageID: rootInput.id,` and `setConfluenceRootPageID(confluenceRootPageID.trim());` to `setConfluenceRootPageID(rootInput.id);`.

5. Replace the Root page ID label:

```tsx
        <label>
          Root page ID (optional)
          <input value={confluenceRootPageID} onChange={(e) => setConfluenceRootPageID(e.target.value)} placeholder="123456" spellCheck={false} />
        </label>
```

with

```tsx
        <label>
          Root page ID (optional)
          <input
            value={confluenceRootPageID}
            onChange={(e) => setConfluenceRootPageID(e.target.value)}
            onBlur={() => {
              if (rootInput.id && !rootInput.error) setConfluenceRootPageID(rootInput.id);
            }}
            placeholder="123456, or paste the page's address"
            spellCheck={false}
            aria-invalid={rootInput.error ? true : undefined}
          />
          {rootInput.error && <span className="field-error">{rootInput.error}</span>}
        </label>
```

6. The field sits inside a collapsed `<details>`, so a stored bad id would disable Save with no visible reason. Directly above the existing `{error && (<div className="error-text" role="alert">...` block add:

```tsx
      {rootInput.error && <div className="field-error">{ROOT_FIX_BEFORE_SAVE}</div>}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd tam/frontend && npx vitest run src/lib/confluenceRoot.test.ts src/components/ProfileForm.test.tsx src/components/ProfilesModal.test.tsx`
Expected: PASS (the existing Profiles modal tests are unaffected: their profiles carry no Confluence root id).

Run (repo root): `npm run typecheck --workspaces --if-present`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add tam/frontend/src/lib/confluenceRoot.ts tam/frontend/src/lib/confluenceRoot.test.ts tam/frontend/src/components/ProfileForm.tsx tam/frontend/src/components/ProfileForm.test.tsx
git commit -m "fix(tam): Profile settings refuse a Confluence root page id that is not a number and read a pasted page address"
```

## Part C: record it

### Task 12: `tam/CLAUDE.md`, the user guide text, and the final gate

**Files:**
- Modify: `tam/CLAUDE.md` ("Rituals, local first" section, "One lock, both ends" section, Layout)

**Interfaces:**
- Consumes: everything above.
- Produces: documentation only.

- [ ] **Step 1: Add two paragraphs to "Rituals, local first"**

In `tam/CLAUDE.md`, insert these two paragraphs directly before the paragraph that starts `Frontend \`lib/storage\` convert storage XHTML to TipTap JSON and back.` (the file's own compressed style, no em dashes):

```markdown
**Editor toolbar = shared `EditorToolbar` in `@agile-suite/core`, and it know
nothing about TipTap.** Items = plain data (toggle, action, link: label, icon,
shortcut, active, disabled, what to run); `useRitualToolbar` in
`ritual-editor/` map TipTap state onto them through `useEditorState`, so
toolbar re-render on transaction and offer only what schema carry (Underline
shown because StarterKit 3 bring it and `lib/storage` write `<u>`). Keyboard =
WAI-ARIA toolbar: one tab stop, Left/Right with wrap, Home/End, tab stop kept
by item id not position. Item that cannot run here = `aria-disabled`, still
focusable; whole toolbar locked = native `disabled` on every button. **Locked
for Sync = disabled, not hidden**, so page not jump when Sync start; editor
itself still `setEditable(false)` on same instance. Read-only page still show
no toolbar. Link popover: Enter apply, Escape close and hand focus back to
link button, refusal (`LINK_REFUSED`, from `isAllowedLink` in
`lib/sanitizeHtml.ts`: absolute http, https, mailto only, scheme read after
dropping control characters) shown under box, never silent close. Button
mouse press `preventDefault`, or selection gone before command run.

**Root page gone = offer, not dead end.** `ritualsync.Run` read root first; 404
come back as `Result.RootMissing` (page id, space, `CanCreate`,
`SuggestedTitle` = `<project key> Rituals`), nil error, nothing written, no
sync time recorded. Any other root failure still Go error. `CanCreate` =
`confluence.SpaceProbe` answer (content listing then space `operations`),
unknown and transport with no probe both read as yes, create own 403 the
fallback. Rituals view open `RitualRootDialog`: placement stated first (top of
space, id saved to profile), Create page and sync, or forbidden sentence plus
Open Profile settings when probe say no. `App.CreateRitualRoot(profile, board,
title, adopt)` run under `"rituals"` lock, reached only through
`SyncContext.runRitualRoot`: `ritualsync.CreateRoot` call `CreatePage` with
empty parent (now omit `ancestors`), 403 = `forbidden`, 400 or 409 = look title
up, `titleTaken` with `TopLevel`, never read Confluence message; adopt (second
confirmation) take only page with no ancestors. On `created`/`adopted`
binding write new id onto stored Confluence config (`saveRitualRoot`, stored
row not stand-in demo config) then run same pass (`syncRitualsLocked`) and
return it in `RitualRootResult`; pass refusal = `SyncError`, not Go error,
since root already saved. Pages under vanished old root go Gone as before;
Recreate on next Sync put them under new root. Demo space asked for root id
`1` (`demo.StagedMissingRootID`) start without it, `DenyCreate` stage token
that cannot create. Profile form refuse non-numeric root id (except Confluence
URL `demo`) and read `pageId` out of pasted address (`lib/confluenceRoot.ts`).
Real-instance answers = `docs/superpowers/plans/assets/2026-09-15-confluence-root-page-probe.md`,
non-blocking.
```

- [ ] **Step 2: Note the second rituals entry point in "One lock, both ends"**

In the paragraph that starts `**Rituals Sync not quiet either.**`, replace its first sentence

```markdown
**Rituals Sync not quiet either.** `SyncContext.runRitualsSync` drive banner
like `runBoardsRefresh`, `running` = `"rituals"`: several requests, no modal
holding focus.
```

with

```markdown
**Rituals Sync not quiet either.** `SyncContext.runRitualsSync` drive banner
like `runBoardsRefresh`, `running` = `"rituals"`: several requests, no modal
holding focus. `runRitualRoot` share same private `runRituals` path, name and
banner, because `CreateRitualRoot` acquire `"rituals"` and end in full Sync.
```

- [ ] **Step 3: Update Layout**

In the Layout block of `tam/CLAUDE.md`:

1. Replace the `app_rituals.go` entry with:

```
    app_rituals.go       the ritual bindings: ensure, list, save, resolve, forget, delete, macro
                          preview, standup entry, last sync, Sync, CreateRitualRoot; all but Sync and
                          CreateRitualRoot under no lock, those two under the "rituals" lock name
```

2. In the `internal/demo/` entry, replace `staging one conflict on the first Standup it creates` with `staging one conflict on the first Standup it creates, a missing root for root id 1, and with DenyCreate a token that cannot create pages`.

3. Replace the `internal/ritualsync/` entry with:

```
    internal/ritualsync/ Ensure (write missing pages from templates, local, no lock), Run
                          (the Sync pass under the "rituals" lock: title match, adopt, create,
                          pull, push, conflict, gone, and a 404 root reported as RootMissing), and
                          root.go: CreateRoot, the top-level root page create or adoption
```

4. In the `internal/ritualtemplate/` entry, after `renders a sprint's five ritual pages (pure, no clock, no I/O)` add `, the body of a root page TAM creates (RootBody),`.

5. After the `src/lib/ritualText.ts` entry add:

```
      src/lib/confluenceRoot.ts  the Profile settings root page id field: a number, or the pageId
                          read out of a pasted page address
```

6. In the `src/components/` list, replace `ritual-editor/RitualEditor (the TipTap editor over one page's storage XHTML, its own save state),` with `ritual-editor/RitualEditor (the TipTap editor over one page's storage XHTML, its own save state), ritual-editor/useRitualToolbar (TipTap state mapped onto @agile-suite/core's EditorToolbar), RitualRootDialog (the missing root page: create, adopt, or Profile settings),`.

7. In the `frontend/` entry line `React app on @agile-suite/core (see ../frontend/core)`, append `; EditorToolbar lives there`.

- [ ] **Step 4: The user guide rituals page**

The repository holds no TAM user guide (`docs/user-guide/USER_GUIDE.md` is Xray Test Manager's), so there is no file to edit here. If the Outline collection "Task Activity Manager" has a user guide page for Rituals, add this section to it through the Outline connector; if it has none, skip this step and say so in the final report:

```markdown
### Formatting a ritual page

The toolbar above a ritual page groups its buttons: Text (bold, italic, underline, strikethrough), Headings, Lists (bullet, numbered, task), Insert (table, link), and History (undo, redo). On the Standup page a Standup group adds today's entry. Tab reaches the toolbar once; the arrow keys, Home and End move between buttons, and each button shows its name and shortcut when you hover or focus it. The link box accepts addresses starting with http://, https:// or mailto:, and says so if you type anything else. While Sync rituals runs, the toolbar stays on screen, greyed out.

### When the rituals root page is missing

If Sync rituals cannot find the root page set in Profile settings, TAM asks whether to create one. It shows the space, a title you can change (your project key followed by "Rituals"), and that the page goes at the top of the space. Create page and sync makes the page, saves its id to your profile, and runs the Sync again under it. If your token cannot create pages there, TAM says so and offers Profile settings, where you can enter the id of an existing page instead. If a page with that title already sits at the top of the space, TAM asks before using it as the root. In Profile settings you can also paste a page's address; TAM keeps only its page id.
```

- [ ] **Step 5: Final gate**

Run each and record the result:

- `cd core && go test ./...` : PASS
- `cd tam && go test ./...` : PASS
- `cd frontend/core && npx vitest run` : PASS
- `cd tam/frontend && npx vitest run` : PASS
- repo root: `npm run typecheck --workspaces --if-present` : no errors
- repo root: `grep -rn $'\xe2\x80\x94' tam/frontend/src/lib/ritualText.ts tam/frontend/src/lib/confluenceRoot.ts tam/frontend/src/components/RitualRootDialog.tsx frontend/core/src/components/EditorToolbar` : no output (no em dash in UI text)
- repo root: `grep -rn "@tiptap" frontend/core/src` : no output
- repo root: `grep -rn "CreateRitualRoot(" tam/frontend/src --include=*.tsx --include=*.ts | grep -v "api.ts\|SyncContext\|\.test\."` : no output (nothing calls the binding bare)

- [ ] **Step 6: Commit**

```bash
git add tam/CLAUDE.md
git commit -m "docs: the rituals toolbar and the missing root page in tam/CLAUDE.md"
```

## Spec coverage

| Spec item | Task |
|---|---|
| E1 shared `EditorToolbar`, editor agnostic, contract | 1 |
| E1 groups, only supported items, `useRitualToolbar`, re-render on transaction | 2 |
| E1 accessibility: toolbar role, roving tabindex, `aria-pressed`, group labels, tooltip with shortcut | 1 |
| E1 inline SVG icons in core, `currentColor`, 16px | 1 |
| E1 LinkPopover: Enter, Escape returns focus, inline refusal, http/https/mailto | 1, 2 |
| E1 locked toolbar disabled, not hidden; `setEditable(false)` kept | 2 |
| E1 "+ Add today's entry" on Standup, once-a-day refusal kept | 2 |
| E2.1 404 root returns `RootMissing` in the result | 5 |
| E2.2 `CanCreate` from content listing plus permissions, unknown = try | 3, 5 |
| E2.3 dialog: space, editable title `<Project key> Rituals`, placement | 9, 10 |
| E2.4 `CreateRitualRoot`: create without ancestors, save id, Sync under `rituals` lock | 3, 6, 7, 8 |
| E2.5 403 sentence with Open Profile settings | 6, 9, 10 |
| E2.6 duplicate title: adopt a top-level page after a second confirmation | 6, 7, 9 |
| Core change: `CreatePage` empty parent omits `ancestors` | 3 |
| Profile validation: numeric id, pasted URL `pageId` | 11 |
| Demo space staged missing root | 4 |
| Testing: Go 404 → RootMissing, no ancestors, setting written, second Sync uses new id | 3, 5, 7 |
| Testing: core keyboard, tooltip shortcut, disabled, link error | 1 |
| Manual demo check | 10 |
| Real-instance probe | `docs/superpowers/plans/assets/2026-09-15-confluence-root-page-probe.md` (non-blocking) |
| Docs: `tam/CLAUDE.md`, user guide rituals page | 12 |

