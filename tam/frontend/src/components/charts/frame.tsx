import { useId, useRef } from "react";
import type { CSSProperties, ReactNode } from "react";
import { useChartWidth } from "./useChartWidth";

// ChartFrame is what the three report charts share: a figure with a caption
// that names it, the box the SVG is measured against, a legend in HTML so it
// wraps, and a visually hidden table of the same data for a screen reader,
// which reads the table and never the marks.
//
// The SVG itself is the chart's own, drawn through children with the width
// it has to fill and the id of the caption it takes its name from.

export interface LegendItem {
  label: string;
  // swatch is the colour role, one of the chart-swatch-* rules in App.css.
  swatch: string;
}

export interface TableSpec {
  caption: string;
  columns: string[];
  rows: { key: string; cells: string[] }[];
}

interface Props {
  title: string;
  legend?: LegendItem[];
  table?: TableSpec;
  // note is a sentence under the chart that qualifies it, from reportText.
  note?: string;
  // empty replaces the chart with a sentence when there is nothing to draw.
  empty?: string;
  busy?: boolean;
  children?: (width: number, titleId: string) => ReactNode;
}

export function ChartFrame({ title, legend, table, note, empty, busy, children }: Props) {
  const titleId = useId();
  const box = useRef<HTMLDivElement>(null);
  const width = useChartWidth(box);
  return (
    <figure className="chart-figure" aria-labelledby={titleId} aria-busy={busy || undefined}>
      <figcaption id={titleId} className="chart-title">{title}</figcaption>
      {empty ? (
        <p className="muted chart-empty">{empty}</p>
      ) : (
        <>
          <div className="chart-box" ref={box}>{children?.(width, titleId)}</div>
          {legend && (
            <ul className="chart-legend" aria-label={`${title} legend`}>
              {legend.map((item) => (
                <li key={item.swatch}>
                  <span className={`chart-swatch chart-swatch-${item.swatch}`} aria-hidden="true" />
                  {item.label}
                </li>
              ))}
            </ul>
          )}
          {table && <HiddenTable table={table} />}
        </>
      )}
      {note && <p className="muted small chart-note">{note}</p>}
    </figure>
  );
}

function HiddenTable({ table }: { table: TableSpec }) {
  return (
    <table className="sr-only">
      <caption>{table.caption}</caption>
      <thead>
        <tr>
          {table.columns.map((c) => <th key={c} scope="col">{c}</th>)}
        </tr>
      </thead>
      <tbody>
        {table.rows.map((r) => (
          <tr key={r.key}>
            {r.cells.map((cell, i) => (i === 0 ? <th key={i} scope="row">{cell}</th> : <td key={i}>{cell}</td>))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// Tooltip is the sentence over a hovered or focused point. It is HTML,
// positioned over the SVG at the point's own coordinates, so it can wrap
// and is read by a live region rather than being part of the picture.
export function Tooltip({ x, y, text }: { x: number; y: number; text: string }) {
  const at: CSSProperties = { left: x, top: y };
  return <div className="chart-tooltip" role="status" style={at}>{text}</div>;
}
