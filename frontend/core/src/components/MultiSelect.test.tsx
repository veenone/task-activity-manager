// MultiSelect came to core in #323 with no test of its own: it had lived in
// xtm/frontend since the Sankey filters and never grew one. TAM's issue type
// filter is its second caller, and a component two apps share is one whose
// behaviour has to be pinned somewhere neither of them owns.
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MultiSelect } from "./MultiSelect";

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

  // An empty list is a profile that has never synced, not a broken control.
  it("says so when there is nothing to choose from", async () => {
    const user = userEvent.setup();
    render(<MultiSelect options={[]} selected={[]} onChange={vi.fn()} allLabel="All types" />);
    await user.click(screen.getByRole("button", { name: /All types/ }));
    expect(screen.getByText("No options")).toBeInTheDocument();
  });
});
