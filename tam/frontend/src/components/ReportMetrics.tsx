import type { ReportSeries } from "../api";
import { unitWord } from "../lib/reportText";

// ReportMetrics puts the five review figures into a shape that can be scanned
// without parsing a sentence. The sentence remains above it for a natural
// reading order; this is the comparison surface.
export function ReportMetrics({ series, live = false }: { series: ReportSeries; live?: boolean }) {
  const unit = (n: number) => unitWord(series.unit, n);
  const metrics = [
    ["Committed", series.committed, unit(series.committed)],
    ["Added", series.added, unit(series.added)],
    ["Removed", series.removed, unit(series.removed)],
    ["Completed", series.completed, unit(series.completed)],
    [live ? "Remaining" : "Carried over", series.carriedOver, unit(series.carriedOver)],
  ] as const;
  return (
    <dl className="report-metrics" aria-label="Sprint report figures">
      {metrics.map(([label, value, noun]) => (
        <div className="report-metric" key={label}>
          <dt>{label}</dt>
          <dd>{value} <span>{noun}</span></dd>
        </div>
      ))}
    </dl>
  );
}
