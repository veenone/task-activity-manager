import type { ColumnView, ReportDocument, ReportTable } from "../api";
import { hasLimit, limitBreach } from "./columnLimit";
import { flowModeLine, flowPendingLine, flowScopeLine } from "./flowText";
import { capacityBreachLine, capacityCountLine, noLimitsLine } from "./reportText";
import { capacityTable } from "./reportTables";

// flowDocument is a kanban board's report as the one document shape every
// publisher already renders.
//
// This is the whole reason ReportOutputs, the Confluence publisher, the
// spreadsheet writer and the deck writer need no change to serve a kanban
// board: they take a ReportDocument and this builds one. A figure therefore
// reads the same in the app, on the page, in the sheet and on the slide,
// which is the rule lib/reportDocument was written to and this keeps.
//
// The table is capacityTable, the same spec lib/reportDocument publishes and
// components/CapacityTable draws, so all three surfaces show one table.
//
// Null rather than a document with no sections when there is nothing to
// publish, which is what leaves the three controls disabled with a reason
// beside them rather than letting somebody publish an empty page. The sprint
// side returns null the same way and for the same reason.

const NO_TABLE: ReportTable = { columns: [], rows: [] };

// kept drops the sentences that came back empty, so a note that does not
// apply leaves no blank line behind on the page.
function kept(lines: string[]): string[] {
  return lines.filter((l) => l !== "");
}

export function flowDocument(boardName: string, columns: ColumnView[]): ReportDocument | null {
  if (columns.length === 0) return null;
  const capacity = capacityTable(columns);

  // A board Jira sets no limit on is not an empty table: the columns are
  // known and none of them is limited, which is one sentence and no rows.
  // That is a different fact from a board whose columns nobody has read, and
  // the caller has already refused that one above.
  if (!columns.some(hasLimit)) {
    return {
      title: `${boardName || "This board"} · Report`,
      sections: [{
        heading: capacity.caption,
        lines: [flowModeLine(), noLimitsLine()],
        table: NO_TABLE,
        notes: kept([flowScopeLine(), flowPendingLine()]),
        images: [],
      }],
    };
  }

  const over = columns.filter((c) => limitBreach(c) === "over").map((c) => c.name);
  return {
    title: `${boardName || "This board"} · Report`,
    sections: [{
      heading: capacity.caption,
      // The mode line first, and it is the line that does the work here: a
      // published capacity table is read away from the board, where nothing
      // says whether it describes a moment or a period that closed.
      lines: [flowModeLine(), capacityCountLine(columns[0].constraint ?? "")],
      table: { columns: capacity.columns, rows: capacity.rows.map((r) => r.cells) },
      notes: kept([capacityBreachLine(over), flowScopeLine(), flowPendingLine()]),
      images: [],
    }],
  };
}
