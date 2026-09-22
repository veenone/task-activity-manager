import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RowLead, BRANCH_GLYPH, DETACHED_TEXT } from "./RowLead";

// leadOf reads the one lead a row draws. Every assertion below is about what
// the lead reserves and how deep it sits, which is the whole contract: the
// pixels come from one custom property the stylesheet resolves, and jsdom
// applies no stylesheet.
function leadOf(container: HTMLElement): HTMLElement {
  const lead = container.querySelector<HTMLElement>(".row-lead");
  if (!lead) throw new Error("the row drew no lead");
  return lead;
}

const depthOf = (container: HTMLElement) => Number(leadOf(container).dataset.rowDepth);
const slotTextOf = (container: HTMLElement) =>
  leadOf(container).querySelector<HTMLElement>(".row-lead-slot")?.textContent ?? null;

describe("RowLead", () => {
  it("puts a child deeper than its parent whether or not the parent has a toggle", () => {
    const { container: withToggle } = render(<RowLead place="root" toggle={<button type="button">2</button>} />);
    const { container: childless } = render(<RowLead place="root" />);
    const { container: child } = render(<RowLead place="child" />);
    expect(depthOf(withToggle)).toBe(depthOf(childless));
    expect(depthOf(child)).toBeGreaterThan(depthOf(withToggle));
  });

  it("reserves the toggle's place on a row that has no toggle", () => {
    const { container: withToggle } = render(<RowLead place="root" toggle={<button type="button">2</button>} />);
    const { container: childless } = render(<RowLead place="root" />);
    expect(slotTextOf(withToggle)).toBe("2");
    // Present and empty, not absent: the spacer is what keeps a childless
    // parent's summary level with the summary of a parent that expands.
    expect(slotTextOf(childless)).toBe("");
  });

  it("draws the branch only under a parent that is there", () => {
    const { container: root } = render(<RowLead place="root" />);
    const { container: child } = render(<RowLead place="child" />);
    expect(leadOf(root).textContent).not.toContain(BRANCH_GLYPH);
    expect(leadOf(child).textContent).toContain(BRANCH_GLYPH);
  });

  it("says so when the parent is not in the list", () => {
    const { container: child } = render(<RowLead place="child" />);
    expect(child.textContent).not.toContain(DETACHED_TEXT);
    render(<RowLead place="detached" />);
    expect(screen.getByText(DETACHED_TEXT)).toBeInTheDocument();
  });

  it("drops the toggle's place on a surface that has no toggles", () => {
    const { container } = render(<RowLead place="child" slot={false} />);
    expect(slotTextOf(container)).toBeNull();
    expect(leadOf(container).textContent).toContain(BRANCH_GLYPH);
  });
});
