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

  it("closes the popover when the toolbar becomes disabled while it is open", async () => {
    // Controller ruling F2: a toolbar that goes disabled while the link
    // popover is open must close it, so it cannot reappear and steal focus
    // from the editor when the lock lifts.
    const user = userEvent.setup();
    const onApply = vi.fn((_href: string): string | null => null);
    const items = linkGroups({ onApply });
    const { rerender } = render(<EditorToolbar label="Formatting" groups={items} />);
    const button = screen.getByRole("button", { name: "Link" });
    await user.click(button);
    expect(screen.getByRole("textbox", { name: "Link address" })).toBeInTheDocument();

    rerender(<EditorToolbar label="Formatting" groups={items} disabled />);

    expect(screen.queryByRole("textbox", { name: "Link address" })).toBeNull();
    expect(button).toHaveAttribute("aria-expanded", "false");
  });
});
