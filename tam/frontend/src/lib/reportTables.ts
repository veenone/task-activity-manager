import type { ColumnView, ReportDay, ReportSeries, VelocityRow } from "../api";
import { counted, effectiveLimit, hasLimit, limitBreach, limitSourceWords } from "./columnLimit";
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
export function burndownTable(days: ReportDay[], title = "Burndown"): TableSpec {
  return {
    caption: `${title}, day by day`,
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
    // "oldest first" rides in the header rather than in a paragraph above
    // the panel: the header is sticky and the paragraph was not, so the one
    // fact that orders the rows would have scrolled away from them. It also
    // contradicts the sprint picker above, which is newest first, so it is
    // worth saying where the rows are.
    columns: ["Sprint, oldest first", "Committed", "Completed"],
    rows: rows.map((r) => ({
      key: String(r.sprintId),
      cells: [r.sprintName, amount(r.committed, r.unit), amount(r.completed, r.unit)],
    })),
  };
}

// capacityTable is the board's columns against their WIP limits. Every
// column is a row, the ones with no limit included: a table holding only the
// limited columns would leave a reader working out which of the board's
// columns were missing from it and why.
//
// The cards cell is the number the limit is measured against, which on a
// board counting without subtasks is not the column's card total. Which of
// the two it is rides in the section's wording rather than in the cell, since
// a column of "4 cards, subtasks not counted" repeats one clause down the
// whole table.
export function capacityTable(columns: ColumnView[]): TableSpec {
  return {
    caption: "Column capacity",
    columns: ["Column", "Cards", "Limit", "Standing"],
    rows: columns.map((c) => ({
      key: c.name,
      cells: [c.name, String(counted(c)), limitCell(c), standingCell(c)],
    })),
  };
}

// One row of doneAgreementTable: an item the agreement states, and how many
// of the sprint's cards were ticked against those words.
export interface AgreementCount {
  text: string;
  met: number;
}

// doneAgreementTable is the team's own bar for finished against the
// sprint's cards. An item nobody ticked is a row reading zero rather than
// no row at all: this table is read as evidence at a review, where a
// missing row and a zero row say different things about an item.
//
// The cell is a bare "9 of 20" and the column head carries what is being
// counted, the way the burndown's cells drop the unit. Every row counts the
// same cards out of the same total, so printing "cards" once per row down
// the column explains nothing.
export function doneAgreementTable(rows: AgreementCount[], cards: number): TableSpec {
  return {
    caption: "Done agreement",
    columns: ["Item", "Cards that met it"],
    rows: rows.map((r) => ({ key: r.text, cells: [r.text, `${r.met} of ${cards}`] })),
  };
}

// limitCell is the limit itself, worded rather than left as a bare number: an
// empty cell reads as a limit of nothing and a zero reads worse.
//
// The cell names where the limit came from, because this table is read away
// from the board and away from TAM: a reader who takes a number the team set
// for itself as the board's rule will go looking for it in Jira and not find
// it. The pair and the precedence come from lib/columnLimit, so the cell and
// the column head on screen can never attribute the same figure differently.
function limitCell(c: ColumnView): string {
  const { min, max } = effectiveLimit(c);
  if (max === null && min === null) return "No limit";
  const figure = max === null ? `Minimum ${min}` : min === null ? String(max) : `${max}, minimum ${min}`;
  return `${figure}, ${limitSourceWords(c)}`;
}

// standingCell is the column against its limit, in words. It is a column of
// the table rather than a colour on the row, because two of the three
// surfaces this table reaches cannot carry a colour that means anything.
function standingCell(c: ColumnView): string {
  if (!hasLimit(c)) return "No limit set";
  const breach = limitBreach(c);
  if (breach === "over") return "Over the limit";
  if (breach === "under") return "Below the minimum";
  return "Within the limit";
}
