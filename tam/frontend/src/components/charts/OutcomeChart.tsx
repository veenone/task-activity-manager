import type { ReportSeries } from "../../api";
import { band, linear, niceTicks } from "../../lib/chartScale";
import { reportFigures } from "../../lib/reportFigures";
import { amount, outcomeLine } from "../../lib/reportText";
import { ChartFrame, Tooltip } from "./frame";
import { BAR_PADDING, LABEL_OFFSET, OUTCOME_MARGIN, OUTCOME_ROW, TICKS } from "./geometry";
import { usePointFocus } from "./usePointFocus";

// OutcomeChart is the five review figures as horizontal bars, each with
// its amount written at the bar's end. It draws the same list the metric
// tiles print, from reportFigures, so the two never name a figure two
// ways. Committed is a floor and removed sees only what came back; the
// caveat line the view prints under the charts says so.

const TITLE = "Sprint outcome";

export function OutcomeChart({ series, live = false, busy }: { series: ReportSeries; live?: boolean; busy?: boolean }) {
  const figures = reportFigures(series, live);
  const table = {
    caption: TITLE,
    columns: ["Figure", "Amount"],
    rows: figures.map((f) => ({ key: f.key, cells: [f.label, amount(f.value, series.unit)] })),
  };
  return (
    <ChartFrame title={TITLE} table={table} busy={busy}>
      {(width, titleId) => <Bars series={series} live={live} width={width} titleId={titleId} />}
    </ChartFrame>
  );
}

function Bars({ series, live, width, titleId }: { series: ReportSeries; live: boolean; width: number; titleId: string }) {
  const figures = reportFigures(series, live);
  const { shown, hit } = usePointFocus(figures.length);
  const height = OUTCOME_MARGIN.top + figures.length * OUTCOME_ROW + OUTCOME_MARGIN.bottom;
  const plotRight = width - OUTCOME_MARGIN.right;
  const most = Math.max(0, ...figures.map((f) => f.value));
  const ticks = niceTicks(most, TICKS);
  const x = linear([0, Math.max(ticks[ticks.length - 1], 1)], [OUTCOME_MARGIN.left, plotRight]);
  const rows = band(figures.length, figures.length * OUTCOME_ROW, BAR_PADDING);
  const top = (i: number) => OUTCOME_MARGIN.top + rows.x(i);
  const middle = (i: number) => top(i) + rows.width / 2;

  return (
    <>
      <svg className="chart-svg" width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="img" aria-labelledby={titleId}>
        <line className="chart-axis" x1={x(0)} x2={x(0)} y1={OUTCOME_MARGIN.top} y2={height - OUTCOME_MARGIN.bottom} />
        {figures.map((f, i) => (
          <g key={f.key}>
            <text className="chart-y-label chart-row-label" x={x(0) - LABEL_OFFSET} y={middle(i)}>{f.label}</text>
            <rect
              className={`chart-bar chart-bar-${f.key}`}
              x={x(0)}
              y={top(i)}
              width={x(f.value) - x(0)}
              height={rows.width}
            />
            <text className="chart-value" x={x(f.value) + LABEL_OFFSET} y={middle(i)}>{amount(f.value, series.unit)}</text>
          </g>
        ))}
        {figures.map((f, i) => (
          <rect
            key={f.key}
            className="chart-hit"
            x={OUTCOME_MARGIN.left}
            y={top(i)}
            width={plotRight - OUTCOME_MARGIN.left}
            height={rows.width}
            aria-label={outcomeLine(f.label, f.value, series.unit)}
            {...hit<SVGRectElement>(i)}
          />
        ))}
      </svg>
      {shown !== null && shown < figures.length && (
        <Tooltip
          x={x(figures[shown].value)}
          y={top(shown)}
          text={outcomeLine(figures[shown].label, figures[shown].value, series.unit)}
        />
      )}
    </>
  );
}
