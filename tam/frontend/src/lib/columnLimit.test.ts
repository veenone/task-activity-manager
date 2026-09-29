import { describe, it, expect } from "vitest";
import type { ColumnView } from "../api";
import { counted, hasLimit, limitBreach, limitLine, limitMeter } from "./columnLimit";

function head(over: Partial<ColumnView> = {}): ColumnView {
  return {
    name: "In Progress", statusIds: ["3"], total: 4, points: 0, counted: 4,
    min: null, max: null, constraint: "issueCount", ...over,
  };
}

describe("a column's limit", () => {
  it("is absent for the ordinary column, which has none", () => {
    expect(hasLimit(head())).toBe(false);
    expect(limitLine(head())).toBe("");
    expect(limitBreach(head())).toBe("");
  });

  it("is absent for a board synced before TAM read the limits", () => {
    const old = head();
    delete old.min;
    delete old.max;
    expect(hasLimit(old)).toBe(false);
    expect(limitLine(old)).toBe("");
  });

  it("reads as the count against the maximum", () => {
    expect(limitLine(head({ counted: 4, max: 6 }))).toBe("4 of 6, from Jira");
    expect(limitBreach(head({ counted: 4, max: 6 }))).toBe("");
  });

  it("says so in words when the column is over, not in colour alone", () => {
    expect(limitLine(head({ counted: 7, max: 6 }))).toBe("7 of 6, from Jira, over the limit");
    expect(limitBreach(head({ counted: 7, max: 6 }))).toBe("over");
  });

  // Zero is a limit a board can really set, and it is not the same fact as
  // an unset one: a column limited to nothing is over the moment it holds a
  // card.
  it("treats a maximum of zero as a limit and not as an absent one", () => {
    expect(hasLimit(head({ max: 0 }))).toBe(true);
    expect(limitBreach(head({ counted: 1, max: 0 }))).toBe("over");
    expect(limitBreach(head({ counted: 0, max: 0 }))).toBe("");
  });

  it("reads a minimum on its own, and says when the column is under it", () => {
    expect(limitLine(head({ counted: 3, min: 2 }))).toBe("3, minimum 2, from Jira");
    expect(limitLine(head({ counted: 1, min: 2 }))).toBe("1, minimum 2, from Jira, below the minimum");
    expect(limitBreach(head({ counted: 1, min: 2 }))).toBe("under");
  });

  it("carries both limits when the board sets both", () => {
    expect(limitLine(head({ counted: 4, min: 2, max: 6 }))).toBe("4 of 6, minimum 2, from Jira");
  });

  // Jira's own board counts a column one of two ways, and a head that did
  // not say which would be a number nobody could reconcile with Jira's.
  it("says when the count leaves subtasks out, because Jira's does", () => {
    expect(limitLine(head({ counted: 2, total: 4, max: 6, constraint: "issueCountExclSubs" })))
      .toBe("2 of 6, from Jira, subtasks not counted");
  });

  // The limit a team set in TAM, on a column Jira sets none on, which is most
  // columns of most boards. It reads as a limit in every way Jira's does, so
  // the indicator works on that column instead of staying blank.
  it("reads the limit set in TAM on a column Jira limits not at all", () => {
    const own = head({ counted: 4, localMax: 6 });
    expect(hasLimit(own)).toBe(true);
    expect(limitLine(own)).toBe("4 of 6, set in TAM");
    expect(limitBreach(own)).toBe("");
    expect(limitBreach(head({ counted: 7, localMax: 6 }))).toBe("over");
  });

  // Jira's limit wins, and this is the case that says so with two different
  // numbers in play: the head has to print the board's rule, not the local
  // one, and name it as the board's.
  it("prefers Jira's limit over the local one, and says which it printed", () => {
    const both = head({ counted: 4, max: 3, localMax: 9 });
    expect(limitLine(both)).toBe("4 of 3, from Jira, over the limit");
    expect(limitBreach(both)).toBe("over");
  });

  // The bar under the clause. The scale is the larger of the count and the
  // limit, which is what puts the mark at the end of the track while the
  // column is still within its limit and moves it inside the fill once the
  // column is past it: 4 of 3 draws a bar that runs on beyond the mark
  // rather than one that is merely full.
  it("draws the count against the limit, with the limit as a mark on the track", () => {
    expect(limitMeter(head({ counted: 2, max: 6 }))).toEqual({ count: 2, scale: 6, mark: 1 });
    expect(limitMeter(head({ counted: 6, max: 6 }))).toEqual({ count: 6, scale: 6, mark: 1 });
    expect(limitMeter(head({ counted: 4, max: 3 }))).toEqual({ count: 4, scale: 4, mark: 0.75 });
    // The local limit is measured the same way, and Jira's still wins.
    expect(limitMeter(head({ counted: 4, localMax: 8 }))).toEqual({ count: 4, scale: 8, mark: 1 });
    expect(limitMeter(head({ counted: 4, max: 3, localMax: 9 }))?.scale).toBe(4);
  });

  it("marks the floor on a column the board gives only a minimum", () => {
    expect(limitMeter(head({ counted: 1, min: 4 }))).toEqual({ count: 1, scale: 4, mark: 1 });
    // Both set, and the ceiling is the mark, not the floor: it is the figure
    // the clause above the bar leads with. The floor would put the mark at
    // two ninths of this track instead of eight.
    expect(limitMeter(head({ counted: 9, min: 2, max: 8 }))).toEqual({ count: 9, scale: 9, mark: 8 / 9 });
  });

  it("draws nothing for a column with no limit, and has a scale at zero", () => {
    expect(limitMeter(head())).toBeNull();
    // A column limited to nothing and holding nothing has no scale to
    // divide by; its mark is the end of an empty track, which is where a
    // limit the count has not passed belongs.
    expect(limitMeter(head({ counted: 0, max: 0 }))).toEqual({ count: 0, scale: 0, mark: 1 });
    expect(limitMeter(head({ counted: 2, max: 0 }))).toEqual({ count: 2, scale: 2, mark: 0 });
  });

  it("counts every card the column holds when nothing says otherwise", () => {
    expect(counted(head({ total: 4, counted: 4 }))).toBe(4);
    // A payload from a board cached before the count was split out falls
    // back to the total rather than to nothing.
    const old = head({ total: 5 });
    delete old.counted;
    expect(counted(old)).toBe(5);
  });
});
