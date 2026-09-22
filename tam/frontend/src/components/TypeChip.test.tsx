import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TypeChip } from "./TypeChip";

describe("TypeChip", () => {
  it("draws a logical type with its own palette class and short label", () => {
    render(<TypeChip type="story" />);
    const chip = screen.getByTitle("Story");
    expect(chip).toHaveTextContent("Story");
    expect(chip.className).toContain("chip-type-story");
  });

  it("calls the sub-task level whatever the instance calls it", () => {
    render(<TypeChip type="subtask" subtaskLabel="Technical task" />);
    expect(screen.getByTitle("Technical task")).toHaveTextContent("Technical task");
  });

  // Issue #65 item 2. The New issue dialog offers the types the project
  // really has, so a draft can carry a type TAM has no palette for. It shows
  // the project's own name, on the neutral chip: a chip-type-Improvement
  // class no stylesheet defines would render as unstyled text.
  it("names a type TAM has no palette for and takes the neutral chip", () => {
    render(<TypeChip type="Improvement" />);
    const chip = screen.getByTitle("Improvement");
    expect(chip).toHaveTextContent("Improvement");
    expect(chip.className).toContain("chip-type-none");
  });
});
