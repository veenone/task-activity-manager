import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { FieldSpec } from "../api";
import { LongTextInput, META_INPUTS, MetaField, splitMetaFields } from "./MetaField";

const spec = (over: Partial<FieldSpec>): FieldSpec => ({
  id: "customfield_1", name: "Field", type: "string", required: false, allowedValues: [], ...over,
});

describe("splitMetaFields", () => {
  it("drops the form's own fields and splits the rest by required", () => {
    const { required, optional } = splitMetaFields([
      spec({ id: "parent", name: "Parent", required: true }),
      spec({ id: "summary", name: "Summary", required: true }),
      spec({ id: "customfield_10050", name: "Severity", required: true }),
      spec({ id: "customfield_10300", name: "Acceptance criteria", type: "textarea" }),
    ]);
    expect(required.map((s) => s.id)).toEqual(["customfield_10050"]);
    expect(optional.map((s) => s.id)).toEqual(["customfield_10300"]);
  });
});

describe("MetaField", () => {
  it("draws long text with Write and Preview, keeps aria-invalid on the textarea, and its value unchanged", async () => {
    const onChange = vi.fn();
    render(<MetaField spec={spec({ name: "Acceptance criteria", type: "textarea" })} value="" invalid onChange={onChange} />);
    const box = screen.getByLabelText("Acceptance criteria");
    expect(box.tagName).toBe("TEXTAREA");
    expect(box).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("tab", { name: "Write" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Preview" })).toBeInTheDocument();
    await userEvent.type(box, "G");
    expect(onChange).toHaveBeenCalledWith("G");
    expect(META_INPUTS.textarea).toBe(LongTextInput);
  });

  it("draws a date for date and datetime, and says a user field wants a username", () => {
    render(
      <>
        <MetaField spec={spec({ id: "a", name: "Due", type: "date" })} value="" invalid={false} onChange={vi.fn()} />
        <MetaField spec={spec({ id: "b", name: "Starts", type: "datetime" })} value="" invalid={false} onChange={vi.fn()} />
        <MetaField spec={spec({ id: "c", name: "Tester", type: "user" })} value="" invalid={false} onChange={vi.fn()} />
      </>,
    );
    expect(screen.getByLabelText("Due")).toHaveAttribute("type", "date");
    expect(screen.getByLabelText("Starts")).toHaveAttribute("type", "date");
    expect(screen.getByText("A Jira username.")).toBeInTheDocument();
  });

  it("offers a select when Jira listed one value's worth of options", async () => {
    const onChange = vi.fn();
    render(
      <MetaField
        spec={spec({ name: "Severity", required: true, allowedValues: [{ id: "10", value: "Major" }, { id: "11", value: "Minor" }] })}
        value=""
        invalid
        onChange={onChange}
      />,
    );
    const select = screen.getByLabelText("Severity *");
    expect(select).toHaveAttribute("aria-invalid", "true");
    await userEvent.selectOptions(select, "11");
    expect(onChange).toHaveBeenCalledWith("11");
  });
});

// Issue #65 item 3. A field taking several values was a native
// <select multiple>: a scrolling box needing ctrl-click, with the chosen
// values legible only while they happen to be scrolled into view.
describe("MetaField, a field taking several values", () => {
  const components = () => spec({
    name: "Components",
    type: "array",
    required: true,
    allowedValues: [{ id: "10", value: "Frontend" }, { id: "11", value: "Backend" }],
  });

  it("names its chosen values without being opened, and adds one from the list", async () => {
    const onChange = vi.fn();
    render(<MetaField spec={components()} value="10" invalid onChange={onChange} />);
    const trigger = screen.getByLabelText("Components *");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("Frontend")).toBeInTheDocument();

    await userEvent.click(trigger);
    const list = screen.getByRole("listbox", { name: "Components" });
    expect(list).toHaveAttribute("aria-multiselectable", "true");
    expect(within(list).getByRole("option", { name: "Frontend" })).toHaveAttribute("aria-selected", "true");
    await userEvent.click(within(list).getByRole("option", { name: "Backend" }));
    expect(onChange).toHaveBeenCalledWith("10,11");
  });

  it("removes a chosen value from where it is shown", async () => {
    const onChange = vi.fn();
    render(<MetaField spec={components()} value="10,11" invalid={false} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "Remove Frontend" }));
    expect(onChange).toHaveBeenCalledWith("11");
  });

  it("opens and chooses from the keyboard alone", async () => {
    const onChange = vi.fn();
    render(<MetaField spec={components()} value="" invalid={false} onChange={onChange} />);
    await userEvent.tab();
    expect(screen.getByLabelText("Components *")).toHaveFocus();
    await userEvent.keyboard("{ArrowDown}");
    expect(screen.getByRole("listbox", { name: "Components" })).toBeInTheDocument();
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledWith("11");
  });
});
