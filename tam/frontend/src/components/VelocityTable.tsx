import type { VelocityRow } from "../api";
import { amount, emptyVelocityLine, velocityFloorLine } from "../lib/reportText";
import { velocityTable } from "../lib/reportTables";

// VelocityTable is the board's last closed sprints as rows, oldest first,
// which is the order VelocityChart reads them in beside it. The backend
// sends them in that order and nothing here re-sorts them, so the table and
// the chart can never disagree about it. The chart drops its own hidden
// table when this one is on screen, so the figures are read once.
//
// The unit is printed in every figure rather than once in a header, because
// it belongs to the row. A board that moved from story points to counting
// cards halfway through the year holds two different quantities, and a
// column of bare numbers would read as one.
//
// The Committed column carries its qualification here rather than borrowing
// the summary's, which is a paragraph away and in a pane that scrolls. The
// rule this phase works to is that the qualification goes on the surface
// that shows the number.
export function VelocityTable({ rows }: { rows: VelocityRow[] }) {
  const spec = velocityTable(rows);
  if (rows.length === 0) {
    return (
      <p className="muted">{emptyVelocityLine()}</p>
    );
  }
  return (
    <>
      {/* The view's only scroller, so it has to be reachable: WebView2 does
          not focus a scrolling div on its own, and a group with no name is
          announced as nothing in particular. */}
      <div
        className="report-table-wrap"
        role="group"
        aria-labelledby="report-velocity-heading"
        // A scrollable region has to be focusable or its content cannot be
        // reached without a mouse, which is WCAG 2.1.1. The rule allows
        // tabIndex only on role="tabpanel" out of the box and does not know
        // about scroll containers, so this is the exception it should make.
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
          {rows.map((r) => (
            <tr key={r.sprintId}>
              <th scope="row">
                {r.sprintName}
                {/* A row built on a changelog Jira cut short says so on the
                    row, since the figures beside it are the ones that are not
                    exact. */}
                {r.truncated && (
                  <span className="muted small report-row-note">Built on a partial changelog</span>
                )}
              </th>
              <td>{amount(r.committed, r.unit)}</td>
              <td>{amount(r.completed, r.unit)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      </div>
      <p className="muted small">{velocityFloorLine()}</p>
    </>
  );
}
