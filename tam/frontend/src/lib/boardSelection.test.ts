import { describe, it, expect } from "vitest";
import { NONE, checkedIn, clear, extend, isChecked, replace, toggle } from "./boardSelection";

// ORDER is a board's reading order: lane by lane, column by column, card by
// card, which is the list every shift gesture is measured against.
const ORDER = ["PLAT-1", "PLAT-2", "PLAT-3", "PLAT-4", "PLAT-5"];

describe("boardSelection", () => {
  it("replaces the whole selection with one card", () => {
    const sel = replace("PLAT-3");
    expect([...sel.keys]).toEqual(["PLAT-3"]);
    expect(sel.anchor).toBe("PLAT-3");
    // A second plain gesture does not accumulate.
    expect([...replace("PLAT-1").keys]).toEqual(["PLAT-1"]);
  });

  it("toggles one card in and back out, leaving the rest alone", () => {
    const two = toggle(toggle(NONE, "PLAT-2"), "PLAT-4");
    expect(checkedIn(two, ORDER)).toEqual(["PLAT-2", "PLAT-4"]);
    const one = toggle(two, "PLAT-2");
    expect(checkedIn(one, ORDER)).toEqual(["PLAT-4"]);
    // The anchor follows the card the gesture was made on, whether it was
    // being added or taken out.
    expect(one.anchor).toBe("PLAT-2");
  });

  it("extends from the anchor in the board's own order", () => {
    const sel = extend(replace("PLAT-2"), ORDER, "PLAT-4");
    expect(checkedIn(sel, ORDER)).toEqual(["PLAT-2", "PLAT-3", "PLAT-4"]);
    expect(sel.anchor).toBe("PLAT-2");
  });

  it("flips the run when a shift gesture crosses the anchor", () => {
    const down = extend(replace("PLAT-3"), ORDER, "PLAT-5");
    expect(checkedIn(down, ORDER)).toEqual(["PLAT-3", "PLAT-4", "PLAT-5"]);
    // Back past the anchor: the run is on the other side of it, not the
    // union of both, and the anchor has not moved.
    const up = extend(down, ORDER, "PLAT-1");
    expect(checkedIn(up, ORDER)).toEqual(["PLAT-1", "PLAT-2", "PLAT-3"]);
    expect(up.anchor).toBe("PLAT-3");
  });

  it("extends like a plain gesture when the anchor has left the board", () => {
    const stale = { keys: new Set(["PLAT-9"]), anchor: "PLAT-9" };
    expect(checkedIn(extend(stale, ORDER, "PLAT-2"), ORDER)).toEqual(["PLAT-2"]);
  });

  it("clears the cards but keeps the card given as the next anchor", () => {
    const sel = clear("PLAT-4");
    expect(sel.keys.size).toBe(0);
    expect(extend(sel, ORDER, "PLAT-5").keys.size).toBe(2);
    expect(clear().anchor).toBe("");
  });

  it("reads back only the cards the board is still drawing, in its order", () => {
    const sel = toggle(toggle(toggle(NONE, "PLAT-4"), "PLAT-1"), "PLAT-9");
    expect(checkedIn(sel, ORDER)).toEqual(["PLAT-1", "PLAT-4"]);
    expect(isChecked(sel, "PLAT-9")).toBe(true);
    expect(isChecked(sel, "PLAT-2")).toBe(false);
  });
});
