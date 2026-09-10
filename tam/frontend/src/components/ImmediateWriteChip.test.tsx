import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ImmediateWriteChip } from "./ImmediateWriteChip";

describe("ImmediateWriteChip", () => {
  it("says in words that the write does not wait for Commit", () => {
    render(<ImmediateWriteChip />);
    // The sentence itself is the point: a chip that only tests as "present"
    // could say anything at all and still pass.
    expect(screen.getByText("Sends to Jira now")).toBeInTheDocument();
  });

  it("is not painted in the colour that means waiting for Commit", () => {
    render(<ImmediateWriteChip />);
    const chip = screen.getByText("Sends to Jira now");
    expect(chip).toHaveClass("chip", "chip-now");
    // Amber is spoken for by the pending marks, and this chip is their
    // opposite, so it must not borrow one of their classes.
    expect(chip.className).not.toMatch(/warn|pending|draft/);
  });
});
