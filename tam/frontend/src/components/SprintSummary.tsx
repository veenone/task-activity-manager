import type { ReportSeries } from "../api";
import {
  builtAtLine,
  floorLine,
  methodLine,
  summarySentence,
  truncationLine,
  unitLine,
} from "../lib/reportText";

// SprintSummary is the sentence a sprint review starts with, and everything
// that sentence is not allowed to be read without.
//
// The heading names the sprint the backend reported on rather than the one
// the picker holds. A view that titles itself from its own state can show
// one sprint's name over another sprint's numbers, which is exactly the
// failure the api guard on the sprint id exists to prevent, and this is the
// second line of defence against it.
//
// The qualification under the sentence is not a footnote, and it is not
// optional. Committed is a floor and removed sees only the cards that came
// back, so both figures can be low, and a number that might be low and does
// not say so is the failure this whole phase was written to avoid.
export function SprintSummary({ series, builtAt }: { series: ReportSeries; builtAt: string }) {
  const unit = unitLine(series.unit, series.unitReason);
  const truncation = truncationLine(series.truncated);
  const built = builtAtLine(builtAt);
  return (
    <div className="report-summary">
      <h3 className="report-heading">{series.sprintName || "This sprint"}</h3>
      <p className="report-sentence">{summarySentence(series)}</p>
      <p className="muted small">{floorLine()}</p>
      {unit && <p className="muted small">{unit}</p>}
      {truncation && <p className="warn-text small">{truncation}</p>}
      <p className="muted small">{methodLine()}</p>
      {built && <p className="muted small">{built}</p>}
    </div>
  );
}
