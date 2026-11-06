import type { query } from "../wailsjs/go/models";

// A fix state says where an issue stands against the rest of its linked set.
export type FixState = "missing" | "behind" | "everywhere" | "open";

export const FIX_STATES: { id: FixState; label: string }[] = [
  { id: "missing", label: "Fix missing" },
  { id: "behind", label: "Fixed, others behind" },
  { id: "everywhere", label: "Fixed everywhere" },
  { id: "open", label: "Open everywhere" },
];

export function fixState(set: query.Set, issue: query.Issue | undefined): FixState {
  const done = issue?.statusCategory === "done";
  if (set.outOfStep) return done ? "behind" : "missing";
  return done ? "everywhere" : "open";
}

export type FlowNode = {
  id: string;
  label: string;
  x: number;
  y: number;
  h: number;
  value: number;
};
export type FlowBand = {
  from: string;
  to: string;
  keys: string[];
  path: string;
};

export const FLOW_NODE = 12;

// sankey lays columns of nodes left to right and the bands between them. A
// node is as tall as the larger of its incoming and outgoing issues, on one
// scale for every column, and its bands stack in the order of the nodes at
// their other end so they do not cross inside a node. A column too long for
// the height asked for makes the chart taller, so no node is squashed to
// nothing; the height used comes back.
export function sankey(
  columns: { id: string; label: string }[][],
  links: { from: string; to: string; keys: string[] }[],
  size: { width: number; height: number; gap: number },
): { nodes: FlowNode[]; bands: FlowBand[]; height: number } {
  const height = Math.max(size.height, ...columns.map((c) => c.length * size.gap * 2));
  const sum = (id: string, end: "from" | "to") => links.filter((l) => l[end] === id).reduce((n, l) => n + l.keys.length, 0);
  const value = (id: string) => Math.max(sum(id, "from"), sum(id, "to"));
  const unit = Math.min(
    ...columns.map(
      (c) =>
        (height - size.gap * (c.length - 1)) /
        Math.max(
          1,
          c.reduce((n, d) => n + value(d.id), 0),
        ),
    ),
  );
  const step = columns.length > 1 ? (size.width - FLOW_NODE) / (columns.length - 1) : 0;
  const nodes: FlowNode[] = [];
  columns.forEach((c, i) => {
    let y = 0;
    for (const d of c) {
      const h = value(d.id) * unit;
      nodes.push({ ...d, x: i * step, y, h, value: value(d.id) });
      y += h + size.gap;
    }
  });
  const at = new Map(nodes.map((n, i) => [n.id, { node: n, order: i, out: 0, in: 0 }]));
  const ordered = [...links].sort((a, b) => at.get(a.from)!.order - at.get(b.from)!.order || at.get(a.to)!.order - at.get(b.to)!.order);
  const offsets = new Map<(typeof links)[number], number>();
  for (const l of ordered) {
    const from = at.get(l.from)!;
    offsets.set(l, from.out);
    from.out += l.keys.length * unit;
  }
  const into = [...links].sort((a, b) => at.get(a.to)!.order - at.get(b.to)!.order || at.get(a.from)!.order - at.get(b.from)!.order);
  const bands = new Map<(typeof links)[number], FlowBand>();
  for (const l of into) {
    const from = at.get(l.from)!;
    const to = at.get(l.to)!;
    const h = l.keys.length * unit;
    const x0 = from.node.x + FLOW_NODE;
    const x1 = to.node.x;
    const y0 = from.node.y + offsets.get(l)!;
    const y1 = to.node.y + to.in;
    to.in += h;
    const mid = (x0 + x1) / 2;
    const path = `M${x0},${y0} C${mid},${y0} ${mid},${y1} ${x1},${y1} L${x1},${y1 + h} C${mid},${y1 + h} ${mid},${y0 + h} ${x0},${y0 + h} Z`;
    bands.set(l, { from: l.from, to: l.to, keys: l.keys, path });
  }
  return { height, nodes, bands: links.map((l) => bands.get(l)!) };
}
