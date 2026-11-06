import { useId } from "react";
import { band, labelEvery, linear, niceTicks } from "@agile-suite/core";
import type { Bar } from "./stats";

// BarChart draws one series of counts. The SVG scales to its box from a fixed
// viewBox, and a visually hidden table carries the same numbers for a screen
// reader, which reads the table and never the marks.
const W = 480;
const H = 200;
const M = { top: 18, right: 8, bottom: 24, left: 34 };
// Past this many bars a number over each one would collide; the table and each
// bar's own tooltip still carry them.
const LABELLED = 12;

export function BarChart({ title, bars }: { title: string; bars: Bar[] }) {
  const id = useId();
  const plotWidth = W - M.left - M.right;
  const bottom = H - M.bottom;
  const bands = band(bars.length, plotWidth, 0.2);
  const ticks = niceTicks(Math.max(0, ...bars.map((b) => b.value)), 4);
  const y = linear([0, Math.max(ticks[ticks.length - 1], 1)], [bottom, M.top]);
  const every = labelEvery(bars.length, plotWidth);
  const left = (i: number) => M.left + bands.x(i);

  return (
    <figure className="pie-chart" aria-labelledby={id}>
      <figcaption id={id} className="pie-chart-title">
        {title}
      </figcaption>
      {bars.length === 0 ? (
        <p className="muted">No issues to count.</p>
      ) : (
        <svg className="pie-chart-svg" viewBox={`0 0 ${W} ${H}`} role="img" aria-labelledby={id}>
          {ticks.map((t) => (
            <g key={t}>
              <line className="pie-chart-grid" x1={M.left} x2={W - M.right} y1={y(t)} y2={y(t)} />
              <text className="pie-chart-label pie-chart-y" x={M.left - 6} y={y(t)}>
                {t}
              </text>
            </g>
          ))}
          {bars.map((b, i) => (
            <g key={b.label}>
              <rect className="pie-chart-bar" x={left(i)} y={y(b.value)} width={bands.width} height={bottom - y(b.value)}>
                <title>{`${b.label}: ${b.value}`}</title>
              </rect>
              {bars.length <= LABELLED && b.value > 0 && (
                <text className="pie-chart-label pie-chart-value" x={left(i) + bands.width / 2} y={y(b.value) - 4}>
                  {b.value}
                </text>
              )}
              {i % every === 0 && (
                <text className="pie-chart-label pie-chart-x" x={left(i) + bands.width / 2} y={H - 6}>
                  {b.label}
                </text>
              )}
            </g>
          ))}
        </svg>
      )}
      <table className="sr-only">
        <caption>{title}</caption>
        <tbody>
          <tr>
            <th scope="col">Value</th>
            <th scope="col">Issues</th>
          </tr>
          {bars.map((b) => (
            <tr key={b.label}>
              <th scope="row">{b.label}</th>
              <td>{b.value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
