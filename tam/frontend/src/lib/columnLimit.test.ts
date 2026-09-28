import { describe, it, expect } from "vitest";
import type { ColumnView } from "../api";
import { counted, hasLimit, limitBreach, limitLine } from "./columnLimit";

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
    expect(limitLine(head({ counted: 4, max: 6 }))).toBe("4 of 6");
    expect(limitBreach(head({ counted: 4, max: 6 }))).toBe("");
  });

  it("says so in words when the column is over, not in colour alone", () => {
    expect(limitLine(head({ counted: 7, max: 6 }))).toBe("7 of 6, over the limit");
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
    expect(limitLine(head({ counted: 3, min: 2 }))).toBe("3, minimum 2");
    expect(limitLine(head({ counted: 1, min: 2 }))).toBe("1, minimum 2, below the minimum");
    expect(limitBreach(head({ counted: 1, min: 2 }))).toBe("under");
  });

  it("carries both limits when the board sets both", () => {
    expect(limitLine(head({ counted: 4, min: 2, max: 6 }))).toBe("4 of 6, minimum 2");
  });

  // Jira's own board counts a column one of two ways, and a head that did
  // not say which would be a number nobody could reconcile with Jira's.
  it("says when the count leaves subtasks out, because Jira's does", () => {
    expect(limitLine(head({ counted: 2, total: 4, max: 6, constraint: "issueCountExclSubs" })))
      .toBe("2 of 6, subtasks not counted");
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
