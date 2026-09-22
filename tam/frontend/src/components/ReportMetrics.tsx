import type { ReportSeries } from "../api";
import { FIGURE_GROUPS, reportFigures } from "../lib/reportFigures";
import { unitWord } from "../lib/reportText";

// ReportMetrics puts the five review figures into a shape that can be scanned
// without parsing a sentence. The sentence remains below it, in the details,
// for anyone who wants it in prose; this is the comparison surface.
//
// The figures are grouped rather than laid out as five equal columns, because
// they are not five peers: committed is the baseline a sprint started from,
// added and removed are the changes made to it, and completed and carried
// over are what came of it. Five equal tiles discarded that.
export function ReportMetrics({ series, live = false }: { series: ReportSeries; live?: boolean }) {
  const figures = reportFigures(series, live);
  return (
    <div className="report-metrics" role="group" aria-label="Sprint report figures">
      {FIGURE_GROUPS.map(({ group, label }) => (
        <section className="report-metric-group" key={group} role="group" aria-labelledby={`metric-group-${group}`}>
          <span className="report-metric-group-label small" id={`metric-group-${group}`}>{label}</span>
          <dl>
            {figures
              .filter((f) => f.group === group)
              .map((f) => (
                <div className="report-metric" key={f.key}>
                  <dt>{f.label}</dt>
                  <dd>{f.value} <span>{unitWord(series.unit, f.value)}</span></dd>
                </div>
              ))}
          </dl>
        </section>
      ))}
    </div>
  );
}
