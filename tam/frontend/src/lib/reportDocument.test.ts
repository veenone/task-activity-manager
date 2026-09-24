import { describe, it, expect } from "vitest";
import type { SprintReport } from "../api";
import { reportDocument } from "./reportDocument";
import {
  emptyDaysLine,
  emptyVelocityLine,
  floorLine,
  methodLine,
  mixedUnitsLine,
  modeLine,
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

  it("says a single row is not a trend, and names mixed units", () => {
    const one = reportDocument(report({ velocity: [report().velocity[0]] }))!;
    expect(one.sections[2].notes).toContain(singleSprintLine());
    const mixed = report();
    mixed.velocity[1].unit = "cards";
    expect(reportDocument(mixed)!.sections[2].notes).toContain(mixedUnitsLine());
  });
});

describe("nothingToPublishLine", () => {
  it("says there is nothing to render without repeating the reason beside it", () => {
    expect(nothingToPublishLine()).toBe("There is no report to publish or export yet.");
    expect(nothingToPublishLine()).not.toContain(unavailableLine("sprintHasNoDates"));
  });
});
