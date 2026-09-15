import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { FieldSpec } from "../api";
import { META_INPUTS, MetaField, splitMetaFields } from "./MetaField";

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
  it("draws long text as a text area, through the table later inputs replace", async () => {
    const onChange = vi.fn();
    render(<MetaField spec={spec({ name: "Acceptance criteria", type: "textarea" })} value="" invalid={false} onChange={onChange} />);
    const box = screen.getByLabelText("Acceptance criteria");
    expect(box.tagName).toBe("TEXTAREA");
    await userEvent.type(box, "G");
    expect(onChange).toHaveBeenCalledWith("G");
    expect(Object.keys(META_INPUTS)).toContain("textarea");
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

  it("offers a select when Jira listed values, a multi-select for an array", () => {
    render(
      <MetaField
        spec={spec({ name: "Components", type: "array", required: true, allowedValues: [{ id: "10", value: "Frontend" }, { id: "11", value: "Backend" }] })}
        value="10"
        invalid
        onChange={vi.fn()}
      />,
    );
    const select = screen.getByLabelText("Components *");
    expect(select).toHaveAttribute("multiple");
    expect(select).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("Pick one or more.")).toBeInTheDocument();
  });
});
