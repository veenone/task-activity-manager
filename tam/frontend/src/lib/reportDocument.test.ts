import { describe, it, expect } from "vitest";
import type { ColumnView, ReportImage, SprintReport } from "../api";
import { reportDocument } from "./reportDocument";
import {
  capacityBreachLine,
  capacityCountLine,
  emptyDaysLine,
  emptyVelocityLine,
  floorLine,
  methodLine,
  mixedUnitsLine,
  modeLine,
  noLimitsLine,
  nothingToPublishLine,
  singleSprintLine,
  truncationLine,
  unavailableLine,
  unitLine,
  velocityFloorLine,
  velocityPartialLine,
} from "./reportText";

function report(over: Partial<SprintReport> = {}): SprintReport {
  return {
    series: {
      sprintId: 11,
      sprintName: "Sprint 11",
      unit: "points",
      unitReason: "",
      committed: 34,
      added: 5,
      removed: 2,
      completed: 29,
      carriedOver: 10,
      days: [{ date: "2026-03-02", scope: 34, completed: 0, remaining: 34, ideal: 34 }],
      truncated: [],
    },
    velocity: [
      { sprintId: 10, sprintName: "Sprint 10", unit: "points", unitReason: "", committed: 30, completed: 28, truncated: false },
      { sprintId: 11, sprintName: "Sprint 11", unit: "points", unitReason: "", committed: 34, completed: 29, truncated: false },
    ],
    builtAt: "2026-03-16T09:00:00Z",
    unavailable: "",
    ...over,
  };
}

// A picture as the rasteriser hands one over: a name, a description and the
// PNG's bytes, which nothing here reads.
const picture = (n: number): ReportImage => ({ name: `chart-${n}.png`, alt: `Chart ${n}`, data: "iVBOR" });

const headings = (r: SprintReport, live = false) => (reportDocument(r, live) ?? { sections: [] }).sections.map((s) => s.heading);

describe("reportDocument", () => {
  it("has nothing to render for an unavailable report", () => {
    expect(reportDocument(report({ unavailable: "sprintHasNoDates" }))).toBeNull();
  });

  it("titles itself after the sprint the backend reported on", () => {
    expect(reportDocument(report())?.title).toBe("Sprint 11 · Report");
  });

  it("carries the outcome, the burndown and the velocity", () => {
    expect(headings(report())).toEqual(["Sprint outcome", "Burndown, day by day", "Velocity, oldest sprint first"]);
  });

  it("puts the five figures under the outcome with the floor caveat and the method", () => {
    const outcome = reportDocument(report())!.sections[0];
    expect(outcome.table.columns).toEqual(["Figure", "Amount"]);
    expect(outcome.table.rows[0]).toEqual(["Committed", "34 points"]);
    expect(outcome.lines[0]).toBe(modeLine(false));
    expect(outcome.notes).toContain(floorLine());
    expect(outcome.notes).toContain(methodLine());
  });

  it("says the figures are provisional while the sprint runs", () => {
    expect(reportDocument(report(), true)!.sections[0].lines[0]).toBe(modeLine(true));
  });

  it("carries the reconstruction caveat onto the burndown too", () => {
    const burndown = reportDocument(report())!.sections[1];
    expect(burndown.table.rows).toHaveLength(1);
    expect(burndown.notes).toContain(methodLine());
  });

  it("carries the partial changelog caveat into every section that rests on one", () => {
    const r = report();
    r.series.truncated = ["PLAT-7"];
    r.velocity[0].truncated = true;
    const doc = reportDocument(r)!;
    expect(doc.sections[0].notes).toContain(truncationLine(["PLAT-7"]));
    expect(doc.sections[1].notes).toContain(truncationLine(["PLAT-7"]));
    expect(doc.sections[2].notes).toContain(velocityPartialLine(["Sprint 10"]));
  });

  it("carries the unit caveat when the report counts cards", () => {
    const r = report();
    r.series.unit = "cards";
    r.series.unitReason = "nothingEstimated";
    expect(reportDocument(r)!.sections[0].notes).toContain(unitLine("cards", "nothingEstimated"));
  });

  it("says why a section is empty rather than printing an empty table", () => {
    const r = report({ velocity: [] });
    r.series.days = [];
    const empty = reportDocument(r)!;
    expect(empty.sections[1].lines).toEqual([emptyDaysLine()]);
    expect(empty.sections[1].table.rows).toEqual([]);
    expect(empty.sections[2].lines).toEqual([emptyVelocityLine()]);
    expect(empty.sections[2].table.rows).toEqual([]);
  });

  it("keeps the velocity caveats the table on screen carries", () => {
    const doc = reportDocument(report())!;
    expect(doc.sections[2].notes).toContain(velocityFloorLine());
  });

  it("says a single row is not a trend", () => {
    const one = reportDocument(report({ velocity: [report().velocity[0]] }))!;
    expect(one.sections[2].notes).toContain(singleSprintLine());
  });

  it("hands each chart's picture to the section whose figures it draws", () => {
    const doc = reportDocument(report(), false, {
      outcome: [picture(1)],
      burndown: [picture(2)],
      velocity: [picture(3)],
    })!;
    expect(doc.sections.map((s) => s.images.map((i) => i.name))).toEqual([
      ["chart-1.png"],
      ["chart-2.png"],
      ["chart-3.png"],
    ]);
  });

  it("carries no picture when no chart was drawn", () => {
    expect(reportDocument(report())!.sections.map((s) => s.images)).toEqual([[], [], []]);
  });

  it("carries no picture for a section with nothing to draw", () => {
    const r = report({ velocity: [] });
    r.series.days = [];
    const doc = reportDocument(r, false, { burndown: [picture(1)], velocity: [picture(2)] })!;
    expect(doc.sections[1].images).toEqual([]);
    expect(doc.sections[2].images).toEqual([]);
  });

  it("explains one chart per unit once the velocity section carries several", () => {
    const mixed = report();
    mixed.velocity[1].unit = "cards";
    const doc = reportDocument(mixed, false, { velocity: [picture(1), picture(2)] })!;
    expect(doc.sections[2].notes).toContain(mixedUnitsLine());
  });

  it("leaves that line out for a velocity section carrying one chart or none", () => {
    expect(reportDocument(report())!.sections[2].notes).not.toContain(mixedUnitsLine());
    const one = reportDocument(report(), false, { velocity: [picture(1)] })!;
    expect(one.sections[2].notes).not.toContain(mixedUnitsLine());
  });
});

describe("nothingToPublishLine", () => {
  it("says there is nothing to render without repeating the reason beside it", () => {
    expect(nothingToPublishLine()).toBe("There is no report to publish or export yet.");
    expect(nothingToPublishLine()).not.toContain(unavailableLine("sprintHasNoDates"));
  });
});

// The capacity section is the WIP limits, which reach Confluence, the
// spreadsheet and the deck because all three renderers walk the document's
// sections and this is one of them.
describe("the capacity section", () => {
  const head = (over: Partial<ColumnView>): ColumnView => ({
    name: "In Progress", statusIds: ["3"], total: 4, points: 0, counted: 4,
    min: null, max: null, constraint: "issueCount", ...over,
  });

  it("is left out for a report carrying no column heads at all", () => {
    expect(headings(report())).toHaveLength(3);
  });

  it("says a board with no limit set has none rather than printing zeroes", () => {
    const doc = reportDocument(report({ capacity: [head({}), head({ name: "Done" })] }))!;
    const capacity = doc.sections[3];
    expect(capacity.lines).toEqual([noLimitsLine()]);
    expect(capacity.table.rows).toEqual([]);
    // The sentence covers both places a limit can come from, since a reader of
    // the published page cannot check whether one was set in TAM either.
    expect(noLimitsLine()).toContain("set in TAM");
  });

  it("carries a row per column with the count against the limit", () => {
    const doc = reportDocument(report({
      capacity: [
        head({ name: "To Do", total: 2, counted: 2 }),
        head({ name: "In Progress", total: 4, counted: 4, max: 3 }),
        head({ name: "Done", total: 9, counted: 9, min: 1 }),
      ],
    }))!;
    const capacity = doc.sections[3];
    expect(capacity.table.rows).toEqual([
      ["To Do", "2", "No limit", "No limit set"],
      ["In Progress", "4", "3, from Jira", "Over the limit"],
      ["Done", "9", "Minimum 1, from Jira", "Within the limit"],
    ]);
  });

  // The published page is read away from the board and away from TAM, so a
  // limit a team set for themselves has to be attributed there above all: a
  // reader who takes it for the board's rule will go looking for it in Jira.
  it("says which limit came from Jira and which was set in TAM", () => {
    const doc = reportDocument(report({
      capacity: [
        head({ name: "To Do", total: 2, counted: 2, localMax: 4 }),
        head({ name: "In Progress", total: 4, counted: 4, max: 3, localMax: 9 }),
      ],
    }))!;
    const capacity = doc.sections[3];
    expect(capacity.table.rows).toEqual([
      ["To Do", "2", "4, set in TAM", "Within the limit"],
      // Jira's limit wins, so the row prints the board's 3 and not the 9.
      ["In Progress", "4", "3, from Jira", "Over the limit"],
    ]);
  });

  it("names the columns that are over, and says what the cards are counted by", () => {
    const doc = reportDocument(report({
      capacity: [head({ name: "In Progress", counted: 4, max: 3, constraint: "issueCountExclSubs" })],
    }))!;
    const capacity = doc.sections[3];
    expect(capacity.lines).toEqual([capacityCountLine("issueCountExclSubs")]);
    expect(capacity.notes).toContain(capacityBreachLine(["In Progress"]));
    expect(capacity.images).toEqual([]);
  });
});
