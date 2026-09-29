import type { ColumnView } from "../api";
import { limitBreach } from "../lib/columnLimit";
import { capacityBreachLine, capacityCountLine, noLimitsLine } from "../lib/reportText";
import { capacityTable } from "../lib/reportTables";

// CapacityTable is the board's columns against the limits Jira sets on them,
// on screen.
//
// The rows are lib/reportTables' capacityTable, which is the same spec
// lib/reportDocument publishes to Confluence, the spreadsheet and the deck.
// That is the point of drawing it from the spec rather than from the columns
// directly: the table a team reads on screen and the table it reads on the
// published page are one table, and a cell worded two ways is a figure nobody
// can reconcile.
//
// Until now the section existed only in the published document, so a reader
// could publish a capacity table they had never seen.
//
// The standing is a column of words and not a colour on the row, for the
// reason standingCell gives: two of the three surfaces this reaches cannot
// carry a colour that means anything, and a reader who cannot see the colour
// is the reader the number matters most to.
export function CapacityTable({ columns, headingId }: { columns: ColumnView[]; headingId: string }) {
  const spec = capacityTable(columns);
  const over = columns.filter((c) => limitBreach(c) === "over").map((c) => c.name);
  const breach = capacityBreachLine(over);
  // Every column of the board is a row, limited or not, so "no limits at all"
  // is a fact about the board rather than an empty table.
  const limited = columns.some((c) => (c.max ?? null) !== null || (c.min ?? null) !== null);

  if (columns.length === 0) return null;
  return (
    <>
      {/* The breach line first, because it is the one sentence that survives
          being read from the back of a room, and because a reader who only
          reads one line should read this one. */}
      {breach && <p className="warn-text small">{breach}</p>}
      {!limited && <p className="muted small">{noLimitsLine()}</p>}
      <div className="report-table-wrap" role="group" aria-labelledby={headingId}
        // A scrolling region has to be focusable or its rows cannot be
        // reached without a mouse, which is WCAG 2.1.1. The rule allows
        // tabIndex only on role="tabpanel" and does not know about scroll
        // containers, which is the exception VelocityTable makes too.
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
        tabIndex={0}
      >
        <table className="report-table">
          <caption className="sr-only">{spec.caption}</caption>
          <thead>
            <tr>
              {spec.columns.map((c) => <th key={c} scope="col">{c}</th>)}
            </tr>
          </thead>
          <tbody>
            {spec.rows.map((r) => (
              <tr key={r.key}>
                <th scope="row">{r.cells[0]}</th>
                {r.cells.slice(1).map((cell, i) => (
                  <td key={spec.columns[i + 1]}>{cell}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {/* What the cards were counted by is Jira's own constraint, and it
          rides under the table rather than in every row: a column reading
          "4 cards, subtasks not counted" repeats one clause down the whole
          table. */}
      <p className="muted small">{capacityCountLine(columns[0].constraint ?? "")}</p>
    </>
  );
}
