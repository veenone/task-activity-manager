import type { ReportDocument, ReportTable, SprintReport } from "../api";
import {
  builtAtLine,
  completionLine,
  emptyDaysLine,
  emptyVelocityLine,
  floorLine,
  methodLine,
  modeLine,
  singleSprintLine,
  summarySentence,
  truncationLine,
  unitLine,
  velocityFloorLine,
  velocityPartialLine,
} from "./reportText";
import { burndownTable, outcomeTable, velocityTable } from "./reportTables";
import type { TableSpec } from "./reportTables";

// reportDocument is the sprint report as something other than a screen: the
// Confluence page, the spreadsheet and the deck are all rendered from this
// one value, in Go, and Go words none of it.
//
// That is the decision this module exists to enforce. Every sentence in the
// report is in lib/reportText and every table is in lib/reportTables, both
// of which are TypeScript, so a Go renderer that built its own wording would
// be a second vocabulary for the same figures, which is the failure those
// two modules were written to prevent. So the wording stays here, the
// renderers take a document and lay it out, and the cost is that a report
// can only be published or exported from the running app: there is no
// headless path that could word one on its own.
//
// mixedUnitsLine is deliberately not among them. It says the velocity chart
// splits into one panel per unit, which is true of the screen and false of a
// spreadsheet and a deck that draw no chart at all; the unit rides on every
// figure in the table instead, which is the protection that sentence exists
// to explain.
//
// notes is the caveats, and it is not a footnote. A figure rebuilt from a
// changelog walk can disagree with Jira's own report, and committed is a
// floor, so every section that carries numbers carries the qualification on
// them. They repeat between sections on purpose: a deck shows one section at
// a time, and a slide of numbers with the caveat on a different slide is a
// slide that looks authoritative and is not.

const NO_TABLE: ReportTable = { columns: [], rows: [] };

// cells drops the row keys a TableSpec carries for React and keeps the words.
function cells(spec: TableSpec): ReportTable {
  return { columns: spec.columns, rows: spec.rows.map((r) => r.cells) };
}

// kept drops the sentences that word themselves out of existence: unitLine
// is empty for a report counting points, truncationLine for a report built
// on whole histories.
function kept(lines: string[]): string[] {
  return lines.filter((l) => l !== "");
}

// reportDocument answers null for a report there is nothing to render,
// which is what makes an unavailable report refuse to publish rather than
// publish a page of zeroes. The caller says so with nothingToPublishLine.
export function reportDocument(report: SprintReport, live = false): ReportDocument | null {
  if (report.unavailable) return null;
  const s = report.series;
  const outcome = outcomeTable(s, live);
  const burndown = burndownTable(s.days);
  const velocity = velocityTable(report.velocity);
  const partial = report.velocity.filter((r) => r.truncated).map((r) => r.sprintName);
  return {
    title: `${s.sprintName || "This sprint"} · Report`,
    sections: [
      {
        heading: outcome.caption,
        lines: [modeLine(live), completionLine(s, live), summarySentence(s, live)],
        table: cells(outcome),
        notes: kept([
          floorLine(),
          unitLine(s.unit, s.unitReason),
          truncationLine(s.truncated),
          methodLine(),
          builtAtLine(report.builtAt),
        ]),
      },
      {
        heading: burndown.caption,
        lines: s.days.length === 0 ? [emptyDaysLine()] : [],
        table: s.days.length === 0 ? NO_TABLE : cells(burndown),
        notes: s.days.length === 0 ? [] : kept([methodLine(), truncationLine(s.truncated)]),
      },
      {
        heading: velocity.caption,
        lines: report.velocity.length === 0 ? [emptyVelocityLine()] : [],
        table: report.velocity.length === 0 ? NO_TABLE : cells(velocity),
        notes:
          report.velocity.length === 0
            ? []
            : kept([
                velocityFloorLine(),
                velocityPartialLine(partial),
                report.velocity.length === 1 ? singleSprintLine() : "",
              ]),
      },
    ],
  };
}
