import { useMemo } from "react";
import type { Sankey } from "../api";

interface Props {
  data: Sankey;
}

const NODE_W = 13;
const GAP = 9; // vertical gap between stacked nodes in a column
const PAD_Y = 8;
const HEIGHT = 440;
const LABEL_PAD = 8;

interface Placed {
  id: string;
  label: string;
  layer: number;
  value: number;
  x: number;
  y: number;
  h: number;
}

// SankeyChart hand-renders a 3-layer Sankey (Plan -> Execution -> run status)
// in SVG — no charting dependency. Because the backend balances the layers
// (every run counted once per layer), a single global value->pixel scale keeps
// link ribbons consistent on both ends.
export function SankeyChart({ data }: Props) {
  const layout = useMemo(() => buildLayout(data), [data]);

  if (!layout) {
    return (
      <p className="muted sankey-empty">
        No execution data to trace yet — sync or generate sample executions.
      </p>
    );
  }

  const { placed, links, width } = layout;
  const byId = new Map(placed.map((p) => [p.id, p]));

  return (
    <svg
      className="sankey"
      viewBox={`0 0 ${width} ${HEIGHT}`}
      preserveAspectRatio="xMidYMid meet"
      role="img"
      aria-label="Traceability from test plans through executions to run status"
    >
      <g className="sankey-links">
        {links.map((lk, i) => {
          const s = byId.get(lk.source);
          const t = byId.get(lk.target);
          if (!s || !t) return null;
          const sx = s.x + NODE_W;
          const tx = t.x;
          const sy = s.y + lk.sy;
          const ty = t.y + lk.ty;
          const xm = (sx + tx) / 2;
          const d = `M${sx},${sy} C${xm},${sy} ${xm},${ty} ${tx},${ty} L${tx},${ty + lk.thick} C${xm},${ty + lk.thick} ${xm},${sy + lk.thick} ${sx},${sy + lk.thick} Z`;
          return (
            <path
              key={i}
              d={d}
              className={`sankey-link ${linkClass(t)}`}
            >
              <title>
                {labelOf(s)} → {labelOf(t)}: {lk.value}
              </title>
            </path>
          );
        })}
      </g>
      <g className="sankey-nodes">
        {placed.map((p) => (
          <g key={p.id}>
            <rect
              x={p.x}
              y={p.y}
              width={NODE_W}
              height={p.h}
              rx={2}
              className={`sankey-node sankey-node-l${p.layer}`}
            >
              <title>
                {labelOf(p)}: {p.value}
              </title>
            </rect>
            {nodeLabel(p, width)}
          </g>
        ))}
      </g>
    </svg>
  );
}

interface PlacedLink {
  source: string;
  target: string;
  value: number;
  thick: number;
  sy: number; // offset within source node
  ty: number; // offset within target node
}

function buildLayout(data: Sankey) {
  if (!data || data.nodes.length === 0) return null;
  const layers = [0, 1, 2];
  const byLayer = layers.map((L) => data.nodes.filter((n) => n.layer === L));
  if (byLayer.some((col) => col.length === 0)) return null;

  const layerTotal = Math.max(
    ...byLayer.map((col) => col.reduce((a, n) => a + n.value, 0)),
    1,
  );
  const maxNodes = Math.max(...byLayer.map((col) => col.length));
  const avail = HEIGHT - PAD_Y * 2 - (maxNodes - 1) * GAP;
  const scale = avail / layerTotal;

  // Reserve horizontal room for labels on both sides; columns sit between.
  const labelW = 150;
  const width = labelW * 2 + 320;
  const colX = [labelW, (width - NODE_W) / 2, width - labelW - NODE_W];

  const placed: Placed[] = [];
  const place = new Map<string, Placed>();
  byLayer.forEach((col, L) => {
    let y = PAD_Y;
    for (const n of col) {
      const h = Math.max(2, n.value * scale);
      const p: Placed = { ...n, x: colX[L], y, h };
      placed.push(p);
      place.set(n.id, p);
      y += h + GAP;
    }
  });

  // Ribbon offsets: walk source out-cursors and target in-cursors in node order.
  const outCursor = new Map<string, number>();
  const inCursor = new Map<string, number>();
  const links: PlacedLink[] = [];
  for (const lk of data.links) {
    const s = place.get(lk.source);
    const t = place.get(lk.target);
    if (!s || !t) continue;
    const thick = Math.max(1, lk.value * scale);
    const sy = outCursor.get(lk.source) ?? 0;
    const ty = inCursor.get(lk.target) ?? 0;
    outCursor.set(lk.source, sy + thick);
    inCursor.set(lk.target, ty + thick);
    links.push({ ...lk, thick, sy, ty });
  }

  return { placed, links, width };
}

function nodeLabel(p: Placed, width: number) {
  const cy = p.y + p.h / 2;
  const text = truncate(p.label, 26);
  if (p.layer === 2) {
    return (
      <text
        x={p.x - LABEL_PAD}
        y={cy}
        className="sankey-label"
        textAnchor="end"
        dominantBaseline="middle"
      >
        {text} ({p.value})
      </text>
    );
  }
  // Layers 0 and 1: label to the right of the node.
  return (
    <text
      x={p.x + NODE_W + LABEL_PAD}
      y={cy}
      className="sankey-label"
      textAnchor="start"
      dominantBaseline="middle"
    >
      {truncate(p.label, p.layer === 0 ? 22 : 18)}
    </text>
  );
}

function linkClass(target: Placed): string {
  if (target.layer === 2) return `link-status link-${target.label.toLowerCase()}`;
  return "link-plan";
}

function labelOf(p: Placed): string {
  return p.label;
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n - 1) + "…" : s;
}
