import { describe, it, expect } from "vitest";
import type { Issue } from "../api";
import { hasTime, issueTime, timeCell, timeCellTitle } from "./issueTime";

// The same six-field shape the Go side's test uses, in the same order:
// the issue's own three, then its family's three. The two
// implementations are pinned against the same cases, so either drifting
// fails where it is written.
function issue(own: (number | null)[], family: (number | null)[]): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "story", summary: "x", status: "To Do",
    assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "",
    parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
    originalEstimateSeconds: own[0], remainingEstimateSeconds: own[1], timeSpentSeconds: own[2],
    aggregateEstimateSeconds: family[0], aggregateRemainingSeconds: family[1], aggregateTimeSpentSeconds: family[2],
  };
}

describe("issueTime", () => {
  it("reads a leaf as itself", () => {
    const t = issueTime(issue([28800, 7200, 21600], [28800, 7200, 21600]));
    expect(t.family).toBe(false);
    expect(t.estimateSeconds).toBe(28800);
    expect(t.remainingSeconds).toBe(7200);
    expect(t.spentSeconds).toBe(21600);
  });

  // The issue this was raised for: everything is on the sub-tasks, so
  // reading the issue's own shows nothing at all.
  it("reads a parent estimated through its children as the family", () => {
    const t = issueTime(issue([null, null, null], [144000, 100800, 43200]));
    expect(t.family).toBe(true);
    expect(t.estimateSeconds).toBe(144000);
    expect(t.remainingSeconds).toBe(100800);
    expect(t.spentSeconds).toBe(43200);
  });

  it("keeps the issue's own beside the family's when both carry work", () => {
    const t = issueTime(issue([7200, 3600, 3600], [144000, 100800, 43200]));
    expect(t.family).toBe(true);
    expect(t.spentSeconds).toBe(43200);
    expect(t.ownSpentSeconds).toBe(3600);
  });

  it("shows nothing for an issue nobody tracked", () => {
    expect(hasTime(issueTime(issue([null, null, null], [null, null, null])))).toBe(false);
  });

  // A row cached before the aggregates were stored: its own figures are
  // all there is, and they are not a family of nothing.
  it("reads a row with no aggregates as its own", () => {
    const t = issueTime(issue([28800, 7200, 21600], [null, null, null]));
    expect(t.family).toBe(false);
    expect(t.estimateSeconds).toBe(28800);
  });
});

describe("the Backlog's time cell", () => {
  it("marks a family and leaves a leaf unmarked", () => {
    expect(timeCell(issue([21600, 7200, 21600], [21600, 7200, 21600]))).toBe("6h of 6h");
    expect(timeCell(issue([null, null, null], [144000, 100800, 43200]))).toBe("Σ 12h of 40h");
  });

  it("says what it knows when only one figure is there", () => {
    expect(timeCell(issue([28800, null, null], [28800, null, null]))).toBe("8h estimated");
    expect(timeCell(issue([null, null, 3600], [null, null, 3600]))).toBe("1h logged");
    expect(timeCell(issue([null, null, null], [null, null, null]))).toBe("");
  });

  it("spells the mark out in the cell's title, remaining included", () => {
    const title = timeCellTitle(issue([null, null, null], [144000, 100800, 43200]));
    expect(title).toContain("40h estimated");
    expect(title).toContain("28h remaining");
    expect(title).toContain("12h logged");
    expect(title).toContain("including sub-tasks");
    // A leaf says the same three without the sub-task clause.
    expect(timeCellTitle(issue([28800, 7200, 21600], [28800, 7200, 21600]))).not.toContain("sub-tasks");
  });
});
