import type { VelocityRow } from "../../api";
import { band, linear, niceTicks } from "../../lib/chartScale";
import { points } from "../../lib/format";
import { mixedUnitsLine, singleSprintLine, velocityLine } from "../../lib/reportText";
import { velocityTable } from "../../lib/reportTables";
import { ChartFrame, Tooltip } from "./frame";
import { BAR_HEIGHT, BAR_PADDING, LABEL_OFFSET, MARGIN, TICKS } from "./geometry";
import { usePointFocus } from "./usePointFocus";

// VelocityChart draws the velocity rows as a committed and a completed bar
// per sprint, in the order the table beside it prints them, oldest first,
// and over the same rows: nothing here re-sorts or trims them, so the two
// can never disagree.
//
// A row's unit is its own. When the rows do not all share one, each unit
// gets a small chart of its own rather than one axis that would read a
// card as a point, and a sentence says so.

const LEGEND = [
  { label: "Committed", swatch: "committed" },
  { label: "Completed", swatch: "completed" },
];

// tabled says the same rows are in a visible table beside the chart. The
// Reports view draws both, and a hidden table there would read the six
// figures to a screen reader twice.
export function VelocityChart({ rows, busy, tabled }: { rows: VelocityRow[]; busy?: boolean; tabled?: boolean }) {
  // Nothing to draw and nothing to say: the table's own sentence covers an
  // empty board.
  if (rows.length === 0) return null;
  const units = [...new Set(rows.map((r) => r.unit))];
  if (units.length === 1) {
    return <Panel rows={rows} title="Velocity" note={rows.length === 1 ? singleSprintLine() : undefined} busy={busy} tabled={tabled} />;
  }
  return (
    <div className="chart-multiples">
      {units.map((unit) => (
        <Panel key={unit} rows={rows.filter((r) => r.unit === unit)} title={`Velocity in ${unit}`} busy={busy} tabled={tabled} />
      ))}
      <p className="muted small chart-note">{mixedUnitsLine()}</p>
    </div>
  );
}

function Panel({ rows, title, note, busy, tabled }: { rows: VelocityRow[]; title: string; note?: string; busy?: boolean; tabled?: boolean }) {
  const table = velocityTable(rows, title);
  return (
    <ChartFrame title={title} legend={LEGEND} table={tabled ? undefined : table} note={note} busy={busy}>
      {(width, titleId) => <Bars rows={rows} width={width} titleId={titleId} />}
    </ChartFrame>
  );
}

function Bars({ rows, width, titleId }: { rows: VelocityRow[]; width: number; titleId: string }) {
  const { shown, hit } = usePointFocus(rows.length);
  const plotRight = width - MARGIN.right;
  const plotBottom = BAR_HEIGHT - MARGIN.bottom;
  const bands = band(rows.length, plotRight - MARGIN.left, BAR_PADDING);
  const half = bands.width / 2;
  const most = Math.max(0, ...rows.flatMap((r) => [r.committed, r.completed]));
  const ticks = niceTicks(most, TICKS);
  const y = linear([0, Math.max(ticks[ticks.length - 1], 1)], [plotBottom, MARGIN.top]);
  const left = (i: number) => MARGIN.left + bands.x(i);

  return (
    <>
      <svg
        className="chart-svg"
        width={width}
        height={BAR_HEIGHT}
        viewBox={`0 0 ${width} ${BAR_HEIGHT}`}
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
        {rows.map((r, i) => (
          <g key={r.sprintId}>
            <rect
              className="chart-bar chart-bar-committed"
              x={left(i)}
              y={y(r.committed)}
              width={half}
              height={plotBottom - y(r.committed)}
            />
            <rect
              className="chart-bar chart-bar-completed"
              x={left(i) + half}
              y={y(r.completed)}
              width={half}
              height={plotBottom - y(r.completed)}
            />
            <text className="chart-value chart-value-up" x={left(i) + half / 2} y={y(r.committed) - LABEL_OFFSET}>
              {points(r.committed)}
            </text>
            <text className="chart-value chart-value-up" x={left(i) + half * 1.5} y={y(r.completed) - LABEL_OFFSET}>
              {points(r.completed)}
            </text>
            <text className="chart-x-label" x={left(i) + half} y={BAR_HEIGHT - LABEL_OFFSET}>{r.sprintName}</text>
          </g>
        ))}
        {rows.map((r, i) => (
          <rect
            key={r.sprintId}
            className="chart-hit"
            x={left(i)}
            y={MARGIN.top}
            width={bands.width}
            height={plotBottom - MARGIN.top}
            aria-label={velocityLine(r)}
            {...hit<SVGRectElement>(i)}
          />
        ))}
      </svg>
      {shown !== null && shown < rows.length && (
        <Tooltip
          x={left(shown) + half}
          y={y(Math.max(rows[shown].committed, rows[shown].completed))}
          text={velocityLine(rows[shown])}
        />
      )}
    </>
  );
}
