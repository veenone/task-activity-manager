import type { ReportSection } from "../api";

// CapacityPanel is the report's capacity section on screen: the board's columns
// against their WIP limits, with each limit attributed to Jira or to this team.
//
// It takes the section lib/reportDocument already built rather than composing
// one of its own. That section is what the Confluence page, the spreadsheet
// and the deck are laid out from, so a team comparing the screen with the page
// it published reads one set of rows worded one way. Building a second version
// here is exactly the failure lib/reportText and lib/reportTables exist to
// prevent.
//
// Which section it is differs. A sprint report's is capacitySection, scoped to
// the sprint's own cards; a kanban board's is the only section kanbanDocument
// carries, scoped to the board. Neither difference reaches the markup, which
// is why this draws both.
//
// No section draws nothing. For a sprint that is a report from a build before
// the column heads travelled, or one whose board the cache has nothing for,
// and a panel reading "no limits" there would claim a fact about a board
// nobody read.
export function CapacityPanel({ section }: { section?: ReportSection }) {
  if (!section) return null;
  const { columns, rows } = section.table;
  return (
    <section
      className="report-capacity"
      // A scroller has to be reachable without a mouse, and a group with no
      // name is announced as nothing in particular. This is the velocity
      // panel's rule, for the same reasons on the same page.
      role="group"
      aria-labelledby="report-capacity-heading"
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
      tabIndex={0}
    >
      <h3 className="report-heading" id="report-capacity-heading">{section.heading}</h3>
      {section.lines.map((line) => <p className="muted small" key={line}>{line}</p>)}
      {rows.length > 0 && (
        <table className="report-table">
          <caption className="sr-only">{section.heading}</caption>
          <thead>
            <tr>{columns.map((c) => <th key={c} scope="col">{c}</th>)}</tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row[0]}>
                {/* The column's name heads its row, so every cell beside it is
                    announced with the column it belongs to. */}
                <th scope="row">{row[0]}</th>
                {row.slice(1).map((cell, i) => <td key={columns[i + 1]}>{cell}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {section.notes.map((note) => <p className="muted small" key={note}>{note}</p>)}
    </section>
  );
}
