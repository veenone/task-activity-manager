import { describe, it, expect } from "vitest";
import type { ReportDay, ReportSeries, VelocityRow } from "../api";
import { burndownTable, outcomeTable, velocityTable } from "./reportTables";
import { calendarDay, points } from "./format";
import { amount } from "./reportText";

function series(over: Partial<ReportSeries> = {}): ReportSeries {
  return {
    sprintId: 11,
    sprintName: "Sprint 11",
    unit: "points",
    unitReason: "",
    committed: 34,
    added: 5,
    removed: 2,
    completed: 29,
    carriedOver: 10,
    days: [],
    truncated: [],
    ...over,
  };
}

const days: ReportDay[] = [
  { date: "2026-03-02", scope: 34, completed: 0, remaining: 34, ideal: 34 },
  { date: "2026-03-03", scope: 39, completed: 8, remaining: 31, ideal: 27.2 },
];

const rows: VelocityRow[] = [
  { sprintId: 9, sprintName: "Sprint 9", unit: "points", unitReason: "", committed: 30, completed: 28, truncated: false },
  { sprintId: 10, sprintName: "Sprint 10", unit: "cards", unitReason: "nothingEstimated", committed: 12, completed: 11, truncated: true },
];

describe("outcomeTable", () => {
  it("prints the five figures with the unit on every amount", () => {
    const t = outcomeTable(series(), false);
    expect(t.columns).toEqual(["Figure", "Amount"]);
    expect(t.rows.map((r) => r.cells)).toEqual([
      ["Committed", amount(34, "points")],
      ["Added", amount(5, "points")],
      ["Removed", amount(2, "points")],
      ["Completed", amount(29, "points")],
      ["Carried over", amount(10, "points")],
    ]);
  });

  it("names the last figure Remaining while the sprint runs", () => {
    expect(outcomeTable(series(), true).rows[4].cells[0]).toBe("Remaining");
  });
});

describe("burndownTable", () => {
  it("prints a row per day in the day's own words", () => {
    const t = burndownTable(days);
    expect(t.columns).toEqual(["Day", "Scope", "Completed", "Remaining", "Ideal"]);
    expect(t.rows[1].cells).toEqual([
      calendarDay("2026-03-03"),
      points(39),
      points(8),
      points(31),
      points(27.2),
    ]);
  });
});

describe("velocityTable", () => {
  it("prints each row's figures in that row's own unit", () => {
    const t = velocityTable(rows);
    expect(t.columns).toEqual(["Sprint, oldest first", "Committed", "Completed"]);
    expect(t.rows[1].cells).toEqual(["Sprint 10", amount(12, "cards"), amount(11, "cards")]);
  });

  it("takes the caption from the panel it belongs to", () => {
    expect(velocityTable(rows, "Velocity in cards").caption).toBe("Velocity in cards, oldest sprint first");
  });
});
