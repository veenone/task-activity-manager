import type { query } from "../wailsjs/go/models";
import { type Edge, projectOf } from "./linkData";

// RING caps each ring of the project network, so a busy centre stays readable;
// the details panel still lists every linked project.
export const RING = 24;

export type ProjectNode = { project: string; ring: 0 | 1 | 2; issues: number };
export type ProjectEdge = {
  id: string;
  source: string;
  target: string;
  type: string;
  count: number;
};
export type ProjectFilter = {
  hidden: Set<string>;
  min: number;
  secondRing: boolean;
};

// projectGraph turns links between issues into the project network around
// centre (#473): one edge per pair of projects and link type, counting its
// links, and the projects one ring out, or two on request. Links inside one
// project, hidden types and lines thinner than min are left out.
export function projectGraph(
  edges: Edge[],
  issues: Map<string, query.Issue>,
  centre: string,
  f: ProjectFilter,
): { nodes: ProjectNode[]; edges: ProjectEdge[] } {
  const pairs = new Map<string, ProjectEdge>();
  for (const e of edges) {
    const [source, target] = [
      projectOf(e.from, issues),
      projectOf(e.to, issues),
    ];
    if (source === target || f.hidden.has(e.type)) continue;
    const id = `${source}>${target}:${e.type}`;
    const pair = pairs.get(id) ?? {
      id,
      source,
      target,
      type: e.type,
      count: 0,
    };
    pair.count += 1;
    pairs.set(id, pair);
  }
  const kept = [...pairs.values()].filter((p) => p.count >= f.min);
  // ringAround lists the projects linked to any of from and not yet placed,
  // most links first.
  const placed = new Set([centre]);
  const ringAround = (from: Set<string>) => {
    const total = new Map<string, number>();
    for (const p of kept)
      for (const [a, b] of [
        [p.source, p.target],
        [p.target, p.source],
      ])
        if (from.has(a) && !placed.has(b))
          total.set(b, (total.get(b) ?? 0) + p.count);
    const ring = [...total]
      .sort(([a, x], [b, y]) => y - x || a.localeCompare(b))
      .slice(0, RING)
      .map(([p]) => p);
    ring.forEach((p) => placed.add(p));
    return ring;
  };
  const first = ringAround(new Set([centre]));
  const second = f.secondRing ? ringAround(new Set(first)) : [];
  const count = new Map<string, number>();
  for (const is of issues.values())
    count.set(is.project, (count.get(is.project) ?? 0) + 1);
  const node = (project: string, ring: 0 | 1 | 2): ProjectNode => ({
    project,
    ring,
    issues: count.get(project) ?? 0,
  });
  return {
    nodes: [
      node(centre, 0),
      ...first.map((p) => node(p, 1)),
      ...second.map((p) => node(p, 2)),
    ],
    edges: kept.filter((p) => placed.has(p.source) && placed.has(p.target)),
  };
}
