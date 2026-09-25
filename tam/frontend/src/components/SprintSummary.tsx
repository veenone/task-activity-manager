import type { ReportSeries } from "../api";
import {
  builtAtLine,
  completionLine,
  floorLine,
  methodLine,
  modeLine,
  summarySentence,
  truncationLine,
  unitLine,
} from "../lib/reportText";
import { ReportMetrics } from "./ReportMetrics";

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
export function SprintSummary({ series, builtAt, live = false }: { series: ReportSeries; builtAt: string; live?: boolean }) {
  const unit = unitLine(series.unit, series.unitReason);
  const truncation = truncationLine(series.truncated);
  const built = builtAtLine(builtAt);
  return (
    <div className="report-summary">
      <h3 className="report-heading">{series.sprintName || "This sprint"}</h3>
      <p className={`report-mode ${live ? "report-mode-live" : "report-mode-closed"}`} role={live ? "status" : undefined}>
        {modeLine(live)}
      </p>
      <p className="report-completion">{completionLine(series, live)}</p>
      <ReportMetrics series={series} live={live} />
      {/* Both caveats govern the figures directly above them, so they are
          read with them or they are not read at all.

          floorLine was moved out of the details below once already, because
          leaving it there meant the numbers were read without it. unitLine
          was left behind, and it is the stronger of the two: it says the
          five figures count cards rather than points, which changes what
          every one of them means. It is empty for a report counting points,
          so having it here costs nothing in the ordinary case. */}
      {unit && <p className="muted small report-caveats">{unit}</p>}
      <p className="muted small report-caveats">{floorLine()}</p>
      {truncation && <p className="warn-text small">{truncation}</p>}
      <details className="report-details">
        <summary>How this report is calculated</summary>
        {/* The sentence says what the tiles say. It stays for anyone who
            wants the figures in prose, below the surface that is scanned. */}
        <p className="report-sentence">{summarySentence(series, live)}</p>
        {/* The one line here that is not a qualification on a named figure:
            it answers the argument that starts when Jira shows a different
            number, which is a thing to look up rather than to read every
            time. */}
        <p className="muted small">{methodLine()}</p>
      </details>
      {built && <p className="muted small">{built}</p>}
    </div>
  );
}
