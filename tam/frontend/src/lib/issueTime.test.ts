import { describe, it, expect } from "vitest";
import type { Issue } from "../api";
import { hasTime, issueTime, timeBar, timeCell, timeCellTitle, timeUnestimated } from "./issueTime";

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
  // Compact: the bar beside it carries the proportion, so the cell
  // holds the two numbers rather than a sentence.
  it("marks a family and leaves a leaf unmarked", () => {
    expect(timeCell(issue([21600, 7200, 21600], [21600, 7200, 21600]))).toBe("6h/6h");
    expect(timeCell(issue([null, null, null], [144000, 100800, 43200]))).toBe("Σ 12h/40h");
  });

  it("says what it knows when only one figure is there", () => {
    expect(timeCell(issue([28800, null, null], [28800, null, null]))).toBe("8h est");
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

describe("the time bar", () => {
  it("fills by what is logged against what was estimated", () => {
    const bar = timeBar(issueTime(issue([40 * 3600, null, 10 * 3600], [40 * 3600, null, 10 * 3600])));
    expect(bar).not.toBeNull();
    expect(bar!.value).toBe(10 * 3600);
    expect(bar!.max).toBe(40 * 3600);
  });

  it("is under way while there is estimate left", () => {
    const bar = timeBar(issueTime(issue([40 * 3600, null, 10 * 3600], [40 * 3600, null, 10 * 3600])));
    expect(bar!.tone).toBe("progress");
  });

  // Logged exactly to the estimate is the one state worth its own
  // colour: the work is done to plan.
  it("is complete when the logged time met the estimate", () => {
    const bar = timeBar(issueTime(issue([8 * 3600, null, 8 * 3600], [8 * 3600, null, 8 * 3600])));
    expect(bar!.tone).toBe("complete");
    expect(bar!.value).toBe(bar!.max);
  });

  // Past the estimate the bar turns over: the track becomes what was
  // logged and the fill becomes the estimate inside it, so the overrun
  // is the part of the track the fill does not reach.
  it("turns over when the work overran, and measures the estimate inside the logged", () => {
    const bar = timeBar(issueTime(issue([2 * 3600, null, 6 * 3600], [2 * 3600, null, 6 * 3600])));
    expect(bar!.tone).toBe("over");
    expect(bar!.value).toBe(2 * 3600);
    expect(bar!.max).toBe(6 * 3600);
    expect(bar!.valueText).toBe("6h logged against 2h estimated, over by 4h");
  });

  // Nothing to measure against: a bar with no scale would be a shape
  // saying something it does not know.
  it("is nothing without an estimate", () => {
    expect(timeBar(issueTime(issue([null, null, 3600], [null, null, 3600])))).toBeNull();
    expect(timeBar(issueTime(issue([null, null, null], [null, null, null])))).toBeNull();
  });

  // An estimate nobody has worked against is an empty track, not an
  // absent one: it says the work is planned and untouched.
  it("is an empty track for an estimate with nothing logged", () => {
    const bar = timeBar(issueTime(issue([8 * 3600, null, null], [8 * 3600, null, null])));
    expect(bar!.value).toBe(0);
    expect(bar!.max).toBe(8 * 3600);
  });

  it("names itself and says its value in words", () => {
    const bar = timeBar(issueTime(issue([null, null, null], [144000, 100800, 43200])));
    expect(bar!.label).toMatch(/time/i);
    expect(bar!.valueText).toBe("12h of 40h, including sub-tasks");
    expect(bar!.tone).toBe("progress");
  });
});

describe("work logged against no estimate", () => {
  // There is no target, so there is no proportion and no bar. The row
  // still has something worth seeing at a glance: somebody is spending
  // time on work nobody sized.
  it("is marked, not measured", () => {
    const t = issueTime(issue([null, null, 3600], [null, null, 3600]));
    expect(timeBar(t)).toBeNull();
    expect(timeUnestimated(t)).toBe(true);
  });

  it("is not marked when there is an estimate to measure against", () => {
    expect(timeUnestimated(issueTime(issue([7200, null, 3600], [7200, null, 3600])))).toBe(false);
  });

  it("is not marked for an issue nobody has logged against either", () => {
    expect(timeUnestimated(issueTime(issue([null, null, null], [null, null, null])))).toBe(false);
    // An estimate with nothing logged is measured, not marked.
    expect(timeUnestimated(issueTime(issue([7200, null, null], [7200, null, null])))).toBe(false);
  });

  // A parent whose children logged hours nobody estimated is the same
  // case, read off the family's figures.
  it("reads the family's figures like any other", () => {
    expect(timeUnestimated(issueTime(issue([null, null, null], [null, null, 43200])))).toBe(true);
  });
});
