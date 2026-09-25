import { describe, expect, it } from "vitest";
import { typeColumnWidth } from "./typeColumn";

// The number of em in a width string, for comparing two of them without
// pinning the coefficient this module is free to retune.
const em = (w: string) => Number(/([\d.]+)\s*\*/.exec(w)?.[1] ?? NaN);

describe("typeColumnWidth", () => {
  it("is wider for a longer label", () => {
    expect(em(typeColumnWidth(["Improvement"]))).toBeGreaterThan(em(typeColumnWidth(["Bug"])));
  });

  it("sizes to the longest label on the page, not the first or the last", () => {
    const mixed = typeColumnWidth(["Bug", "Sub Test Execution", "Task"]);
    expect(mixed).toBe(typeColumnWidth(["Sub Test Execution"]));
  });

  // The bug this exists for: a fixed 104px track ellipsised "Improvement",
  // and the 44px track in the sprint and epic trees cut it to two letters.
  it("holds a name the fixed tracks used to clip", () => {
    const declared = typeColumnWidth(["Improvement"]);
    expect(em(declared)).toBeGreaterThanOrEqual("Improvement".length * 0.5);
  });

  it("does not collapse when nothing is on the page", () => {
    expect(em(typeColumnWidth([]))).toBeGreaterThan(0);
  });

  it("does not let one pathological name eat the row", () => {
    const absurd = typeColumnWidth(["A".repeat(200)]);
    expect(em(absurd)).toBe(em(typeColumnWidth(["B".repeat(300)])));
  });

  it("leaves room for the chip's own padding and border", () => {
    expect(typeColumnWidth(["Bug"])).toMatch(/\+\s*\d+px/);
  });
});
