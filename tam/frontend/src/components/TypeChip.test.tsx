import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TypeChip } from "./TypeChip";
import { TYPE_CHIP_ALT_CLASSES, typeChipClass } from "../lib/typeChip";

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

  // Issue #65 item 2, revised by #79. A row can carry a type TAM has no
  // concept of, and since #68 most of them do. It still shows the project's
  // own name, but it no longer shares one grey chip with every other such
  // type: lib/typeChip picks a defined palette class, stably per name. The
  // class has to be one a stylesheet defines, which is what made the
  // original chip-type-Improvement wrong.
  it("names a type TAM has no palette for and gives it a colour of its own", () => {
    render(<TypeChip type="Improvement" />);
    const chip = screen.getByTitle("Improvement");
    expect(chip).toHaveTextContent("Improvement");
    expect(chip.className).not.toContain("chip-type-none");
    expect(chip.className).toContain(`chip-type-${typeChipClass("Improvement")}`);
    expect(TYPE_CHIP_ALT_CLASSES).toContain(typeChipClass("Improvement"));
  });
});
