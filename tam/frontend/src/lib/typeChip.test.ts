import { describe, expect, it } from "vitest";
import { ISSUE_TYPES } from "../api";
import { TYPE_CHIP_ALT_CLASSES, typeChipClass, typeChipLabel } from "./typeChip";

describe("typeChipLabel", () => {
  it("shows the short label for a type TAM models", () => {
    expect(typeChipLabel("story")).toBe("Story");
    expect(typeChipLabel("requirement")).toBe("Req");
  });

  it("shows the instance's own word for the sub-task level", () => {
    expect(typeChipLabel("subtask", "Technical task")).toBe("Technical task");
  });

  it("falls back to TAM's word when the instance's is not known yet", () => {
    expect(typeChipLabel("subtask")).toBe("Sub");
  });

  it("shows the raw name for a type TAM has no concept of", () => {
    expect(typeChipLabel("Improvement")).toBe("Improvement");
  });
});

describe("typeChipClass", () => {
  it("gives each modelled type its own palette class", () => {
    const classes = ISSUE_TYPES.map((t) => typeChipClass(t.id));
    expect(classes).toEqual(["task", "epic", "story", "bug", "requirement", "subtask"]);
  });

  // The bug: every type outside the six shared chip-type-none, so since #68
  // widened the sync a whole project's vocabulary read as one grey chip.
  it("gives a type TAM does not model a colour of its own", () => {
    expect(typeChipClass("Improvement")).not.toBe("none");
    expect(TYPE_CHIP_ALT_CLASSES).toContain(typeChipClass("Improvement"));
  });

  it("gives the same name the same colour every time", () => {
    expect(typeChipClass("Improvement")).toBe(typeChipClass("Improvement"));
    expect(typeChipClass("Technical task")).toBe(typeChipClass("Technical task"));
  });

  // Not "every name gets a different colour", which three colours cannot
  // promise. Only that the palette is actually being spread rather than
  // every name landing on one entry, which a broken hash would do.
  it("spreads a project's vocabulary across the palette", () => {
    const names = ["Improvement", "Technical task", "Change Request", "Incident", "Spike", "Support"];
    expect(new Set(names.map(typeChipClass)).size).toBeGreaterThan(1);
  });

  it("keeps the neutral chip for a row with no type at all", () => {
    expect(typeChipClass("")).toBe("none");
  });
});
