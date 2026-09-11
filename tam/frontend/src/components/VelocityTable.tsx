import type { VelocityRow } from "../api";
import { amount } from "../lib/reportText";

// VelocityTable is the board's last closed sprints as rows, oldest first,
// which is the order the bar chart a later plan draws would be read in. The
// backend sends them in that order and nothing here re-sorts them, so the
// table and the chart can never disagree about it.
//
// The unit is printed in every figure rather than once in a header, because
// it belongs to the row. A board that moved from story points to counting
// cards halfway through the year holds two different quantities, and a
// column of bare numbers would read as one.
export function VelocityTable({ rows }: { rows: VelocityRow[] }) {
  if (rows.length === 0) {
    return (
      <p className="muted">
        No closed sprint on this board has a start and an end date TAM can read, so there is no velocity table.
      </p>
    );
  }
  return (
    <table className="report-table">
      <caption className="sr-only">Velocity, oldest sprint first</caption>
      <thead>
        <tr>
          <th scope="col">Sprint</th>
          <th scope="col">Committed</th>
          <th scope="col">Completed</th>
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
  );
}
