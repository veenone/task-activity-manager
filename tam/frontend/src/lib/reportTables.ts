import type { ReportDay, ReportSeries, VelocityRow } from "../api";
import { calendarDay, points } from "./format";
import { reportFigures } from "./reportFigures";
import { amount } from "./reportText";

// reportTables is the three tables the sprint report is made of, built once
// and read by everything that draws them: the charts' hidden tables, the
// velocity table on screen, and the Confluence page, the spreadsheet and the
// deck that lib/reportDocument assembles out of them.
//
// It sits beside lib/reportText for the same reason that file exists. The
// sentences are there because a figure worded two ways is a figure the
// reader cannot reconcile; a table is a figure too, so its caption, its
// column names and its cells are here rather than in each surface that
// prints them. Every cell goes through format or reportText, so a row in
// the deck reads exactly as the row on screen does.

export interface TableSpec {
  caption: string;
  columns: string[];
  rows: { key: string; cells: string[] }[];
}

// outcomeTable is the five review figures, the same list and the same order
// the metric tiles and the outcome chart draw.
export function outcomeTable(series: ReportSeries, live = false): TableSpec {
  return {
    caption: "Sprint outcome",
    columns: ["Figure", "Amount"],
    rows: reportFigures(series, live).map((f) => ({
      key: f.key,
      cells: [f.label, amount(f.value, series.unit)],
    })),
  };
}

// burndownTable is the day by day line behind the totals. The cells are bare
// numbers rather than amounts because the unit is the same in every one of
// them, and a column of "34 points" reads as five columns of noise.
export function burndownTable(days: ReportDay[]): TableSpec {
  return {
    caption: "Burndown, day by day",
    columns: ["Day", "Scope", "Completed", "Remaining", "Ideal"],
    rows: days.map((d) => ({
      key: d.date,
      cells: [calendarDay(d.date), points(d.scope), points(d.completed), points(d.remaining), points(d.ideal)],
    })),
  };
}

// velocityTable is the board's closed sprints, oldest first, in the order the
// backend sent them. title is the panel the rows belong to, which is
// "Velocity" for a board estimating in one unit and names the unit when the
// chart splits into one panel per unit.
//
// Every figure carries its own unit, because a board that moved from points
// to cards holds two different quantities in one column.
export function velocityTable(rows: VelocityRow[], title = "Velocity"): TableSpec {
  return {
    caption: `${title}, oldest sprint first`,
    columns: ["Sprint", "Committed", "Completed"],
    rows: rows.map((r) => ({
      key: String(r.sprintId),
      cells: [r.sprintName, amount(r.committed, r.unit), amount(r.completed, r.unit)],
    })),
  };
}
