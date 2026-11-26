// MultiSelect came to core in #323 with no test of its own: it had lived in
// xtm/frontend since the Sankey filters and never grew one. TAM's issue type
// filter is its second caller, and a component two apps share is one whose
// behaviour has to be pinned somewhere neither of them owns.
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MultiSelect } from "./MultiSelect";
import { Modal } from "./Modal";

const TYPES = [
  { value: "task", label: "Task" },
  { value: "story", label: "Story" },
  { value: "bug", label: "Bug" },
];

describe("MultiSelect", () => {
  // The button is the whole control while the panel is shut, so it has to
  // report the selection without being opened.
  it("summarises the selection on the button", () => {
    const { rerender } = render(
      <MultiSelect options={TYPES} selected={[]} onChange={vi.fn()} allLabel="All types" />,
    );
    expect(screen.getByRole("button", { name: /All types/ })).toBeInTheDocument();

    rerender(<MultiSelect options={TYPES} selected={["story"]} onChange={vi.fn()} allLabel="All types" />);
    expect(screen.getByRole("button", { name: /Story/ })).toBeInTheDocument();

    rerender(<MultiSelect options={TYPES} selected={["story", "bug"]} onChange={vi.fn()} allLabel="All types" />);
    expect(screen.getByRole("button", { name: /2 selected/ })).toBeInTheDocument();
  });

  // Adding rather than replacing is the whole reason this is not a select.
  it("adds to the selection rather than replacing it", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<MultiSelect options={TYPES} selected={["task"]} onChange={onChange} allLabel="All" />);
    await user.click(screen.getByRole("button", { name: /Task/ }));
    await user.click(screen.getByRole("checkbox", { name: "Bug" }));
    expect(onChange).toHaveBeenCalledWith(["task", "bug"]);
  });

  it("removes a value that was already selected", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<MultiSelect options={TYPES} selected={["task", "bug"]} onChange={onChange} allLabel="All" />);
    await user.click(screen.getByRole("button", { name: /2 selected/ }));
    await user.click(screen.getByRole("checkbox", { name: "Task" }));
    expect(onChange).toHaveBeenCalledWith(["bug"]);
  });

  // Clearing is offered only when there is something to clear, so the panel
  // does not carry a control that would do nothing.
  it("clears everything at once, and offers that only when something is selected", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const { rerender } = render(
      <MultiSelect options={TYPES} selected={[]} onChange={onChange} allLabel="All" />,
    );
    await user.click(screen.getByRole("button", { name: /All/ }));
    expect(screen.queryByRole("button", { name: "Clear all" })).toBeNull();

    rerender(<MultiSelect options={TYPES} selected={["task"]} onChange={onChange} allLabel="All" />);
    await user.click(screen.getByRole("button", { name: "Clear all" }));
    expect(onChange).toHaveBeenCalledWith([]);
  });

  // The caret is decoration beside a summary that already says the same
  // thing, so it stays out of what a screen reader announces.
  it("keeps the caret out of what is read", async () => {
    const user = userEvent.setup();
    render(<MultiSelect options={TYPES} selected={[]} onChange={vi.fn()} allLabel="All types" />);
    const trigger = screen.getByRole("button", { name: /All types/ });
    expect(trigger.querySelector(".multiselect-caret")).toHaveAttribute("aria-hidden", "true");
    await user.click(trigger);
    expect(screen.getAllByRole("checkbox")).toHaveLength(TYPES.length);
  });

  // The 2026-11-23 audit: a trigger that opens a panel says whether it is
  // open, and the panel shuts on Escape with focus back where it started.
  it("reports whether it is open and closes on Escape", async () => {
    const user = userEvent.setup();
    render(<MultiSelect options={TYPES} selected={[]} onChange={vi.fn()} allLabel="All types" />);
    const trigger = screen.getByRole("button", { name: /All types/ });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveAttribute("aria-haspopup", "true");
    await user.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("checkbox", { name: "Task" })).toBeNull();
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });

  // The Backlog's Set release dialog holds this control, and one Escape
  // must shut the panel, not the dialog under it.
  it("closes only its panel on Escape inside a dialog", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <Modal onClose={onClose} label="Set release">
        <MultiSelect options={TYPES} selected={[]} onChange={vi.fn()} allLabel="None" />
      </Modal>,
    );
    await user.click(screen.getByRole("button", { name: /None/ }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("checkbox", { name: "Task" })).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
  });

  // A placeholder is not a label: it goes as soon as the user types.
  it("names the trigger and the search box from its label", async () => {
    const user = userEvent.setup();
    const many = Array.from({ length: 10 }, (_, i) => ({ value: String(i), label: `Option ${i}` }));
    render(<MultiSelect options={many} selected={[]} onChange={vi.fn()} allLabel="None" label="Components" id="pick" />);
    const trigger = screen.getByRole("button", { name: "Components: None" });
    expect(trigger).toHaveAttribute("id", "pick");
    await user.click(trigger);
    expect(screen.getByRole("searchbox", { name: "Search Components" })).toBeInTheDocument();
  });

  // A released version stays on an issue that has it but cannot be added
  // again, so its box can be cleared and never newly checked.
  it("lets a disabled option be cleared but not checked", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const opts = [{ value: "a", label: "A" }, { value: "b", label: "B", disabled: true }, { value: "c", label: "C", disabled: true }];
    render(<MultiSelect options={opts} selected={["c"]} onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: /C/ }));
    expect(screen.getByRole("checkbox", { name: "B" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "C" })).toBeEnabled();
    await user.click(screen.getByRole("checkbox", { name: "C" }));
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it("does not open when disabled", async () => {
    const user = userEvent.setup();
    render(<MultiSelect options={TYPES} selected={[]} onChange={vi.fn()} allLabel="All types" disabled />);
    const trigger = screen.getByRole("button", { name: /All types/ });
    expect(trigger).toBeDisabled();
    await user.click(trigger);
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
  });

  // An empty list is a profile that has never synced, not a broken control.
  it("says so when there is nothing to choose from", async () => {
    const user = userEvent.setup();
    render(<MultiSelect options={[]} selected={[]} onChange={vi.fn()} allLabel="All types" />);
    await user.click(screen.getByRole("button", { name: /All types/ }));
    expect(screen.getByText("No options")).toBeInTheDocument();
  });
});
