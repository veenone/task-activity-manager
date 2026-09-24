import type { ReportDay } from "../../api";
import { dayIndex, labelEvery, linear, niceTicks, spread } from "../../lib/chartScale";
import { calendarDay, points } from "../../lib/format";
import { amount, dayLine, emptyDaysLine } from "../../lib/reportText";
import { burndownTable } from "../../lib/reportTables";
import { ChartFrame, Tooltip } from "./frame";
import { HIT_RADIUS, LABEL_OFFSET, LINE_HEIGHT, MARGIN, POINT_RADIUS, TICKS, VALUE_GAP } from "./geometry";
import { usePointFocus } from "./usePointFocus";

// BurndownChart draws the day by day line behind the sprint's totals: the
// work remaining, the guide it was meant to follow, and the scope it was
// measured against. The days are the backend's own, which stop at today
// for a running sprint, so nothing after now is drawn.
//
// A single-day sprint has nothing to join, so it draws its points and no
// line. The values at the end of each line are written beside it, so the
// picture states its numbers and colour is never the only thing that
// tells the lines apart.

const LEGEND = [
  { label: "Remaining", swatch: "remaining" },
  { label: "Ideal", swatch: "ideal" },
  { label: "Scope", swatch: "scope" },
];

export function BurndownChart({ days, unit, busy }: { days: ReportDay[]; unit: string; busy?: boolean }) {
  const table = burndownTable(days);
  return (
    <ChartFrame
      title="Burndown"
      legend={LEGEND}
      table={table}
      empty={days.length === 0 ? emptyDaysLine() : undefined}
      busy={busy}
    >
      {(width, titleId) => <Lines days={days} unit={unit} width={width} titleId={titleId} />}
    </ChartFrame>
  );
}

function Lines({ days, unit, width, titleId }: { days: ReportDay[]; unit: string; width: number; titleId: string }) {
  const { shown, hit } = usePointFocus(days.length);
  const idx = dayIndex(days);
  const plotRight = width - MARGIN.right;
  const plotBottom = LINE_HEIGHT - MARGIN.bottom;
  const x = linear([0, idx[idx.length - 1]], [MARGIN.left, plotRight]);
  const most = Math.max(0, ...days.flatMap((d) => [d.scope, d.remaining, d.ideal]));
  const ticks = niceTicks(most, TICKS);
  const y = linear([0, Math.max(ticks[ticks.length - 1], 1)], [plotBottom, MARGIN.top]);
  const every = labelEvery(days.length, plotRight - MARGIN.left);
  const last = days[days.length - 1];
  const joined = days.length > 1;

  const remaining = days.map((d, i) => `${x(idx[i])},${y(d.remaining)}`).join(" ");
  const ideal = days.map((d, i) => `${x(idx[i])},${y(d.ideal)}`).join(" ");
  // Scope is a step: it holds its value until the day it changes.
  const scope = days
    .map((d, i) => (i === 0 ? `M${x(idx[i])} ${y(d.scope)}` : `H${x(idx[i])} V${y(d.scope)}`))
    .join(" ");

  // The two end labels, pushed apart when the lines end near each other.
  const [remainingY, scopeY] = spread([y(last.remaining), y(last.scope)], VALUE_GAP);
  const endX = x(idx[idx.length - 1]) + LABEL_OFFSET;

  return (
    <>
      <svg
        className="chart-svg"
        width={width}
        height={LINE_HEIGHT}
        viewBox={`0 0 ${width} ${LINE_HEIGHT}`}
        role="img"
        aria-labelledby={titleId}
      >
        {ticks.map((t) => (
          <g key={t}>
            <line className="chart-grid" x1={MARGIN.left} x2={plotRight} y1={y(t)} y2={y(t)} />
            <text className="chart-y-label" x={MARGIN.left - LABEL_OFFSET} y={y(t)}>{points(t)}</text>
          </g>
        ))}
        <line className="chart-axis" x1={MARGIN.left} x2={plotRight} y1={plotBottom} y2={plotBottom} />
        {days.map((d, i) =>
          i % every === 0 ? (
            <text key={d.date} className="chart-x-label" x={x(idx[i])} y={LINE_HEIGHT - LABEL_OFFSET}>
              {calendarDay(d.date)}
            </text>
          ) : null,
        )}
        {joined && (
          <>
            <path className="chart-line chart-line-scope" d={scope} />
            <polyline className="chart-line chart-line-ideal" points={ideal} />
            <polyline className="chart-line chart-line-remaining" points={remaining} />
          </>
        )}
        {days.map((d, i) => (
          <circle
            key={d.date}
            className={i === days.length - 1 ? "chart-point chart-point-final" : "chart-point"}
            cx={x(idx[i])}
            cy={y(d.remaining)}
            r={POINT_RADIUS}
          />
        ))}
        <text className="chart-value chart-value-remaining" x={endX} y={remainingY}>{amount(last.remaining, unit)}</text>
        <text className="chart-value chart-value-scope" x={endX} y={scopeY}>{amount(last.scope, unit)}</text>
        {days.map((d, i) => (
          <circle
            key={d.date}
            className="chart-hit"
            cx={x(idx[i])}
            cy={y(d.remaining)}
            r={HIT_RADIUS}
            aria-label={dayLine(d, unit)}
            {...hit<SVGCircleElement>(i)}
          />
        ))}
      </svg>
      {shown !== null && shown < days.length && (
        <Tooltip x={x(idx[shown])} y={y(days[shown].remaining)} text={dayLine(days[shown], unit)} />
      )}
    </>
  );
}
