import type { ReportPerson } from "../api";
import { ALLOCATION_TIME, allocationSection } from "../lib/reportPeople";
import { PeoplePanel } from "./PeoplePanel";

// AllocationPanel is the board's open work per assignee (#288), the section
// the outputs carry, drawn the way the People tab draws its own: a row per
// person with the time bar, opening to the cards behind it. The rows come
// from the cache, so the caller hands over what useBoardAllocation read.
export function AllocationPanel({ rows, error, onRetry }: { rows?: ReportPerson[]; error?: string; onRetry: () => void }) {
  if (error) {
    return (
      <p className="error-text" role="alert">
        Could not read the board allocation: {error}{" "}
        <button type="button" className="btn" onClick={onRetry}>Retry</button>
      </p>
    );
  }
  if (!rows) return <p className="muted" role="status">Reading the board allocation</p>;
  const [section] = allocationSection(rows);
  if (rows.length === 0) {
    return (
      <section className="report-people" role="group" aria-labelledby="report-allocation-heading">
        <h3 className="report-heading" id="report-allocation-heading">{section.heading}</h3>
        {section.lines.map((l) => <p className="muted" key={l}>{l}</p>)}
      </section>
    );
  }
  return <PeoplePanel section={section} people={rows} headingId="report-allocation-heading" time={ALLOCATION_TIME} />;
}
