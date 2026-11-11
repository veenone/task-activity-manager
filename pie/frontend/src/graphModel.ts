import type { query } from "../wailsjs/go/models";

// The issue graph's model (#464): the issues on the canvas and one edge per
// relationship, however many times an issue is loaded. Kept free of G6 so it
// can be tested without a canvas.

// MAX_NODES caps the canvas; PER_EXPAND caps the neighbours one expansion adds.
export const MAX_NODES = 300;
export const PER_EXPAND = 50;
export const PARENT = "Parent / sub-task";

export type GNode = { key: string; issue: query.Issue; inRun: boolean; fetchedAt: string };
export type GEdge = { id: string; source: string; target: string; type: string; words: string; directed: boolean };
export type Graph = { nodes: Map<string, GNode>; edges: Map<string, GEdge> };

export const emptyGraph = (): Graph => ({ nodes: new Map(), edges: new Map() });

// edgesOf turns an issue's links, parent and sub-tasks into canonical edges.
// Jira lists a link on both of its issues, worded from each side; both sides
// give the same id here. A directed edge runs from the outward end; a
// symmetric type, worded the same both ways, uses the two keys in order.
export function edgesOf(is: query.Issue): GEdge[] {
  const out: GEdge[] = [];
  for (const l of is.links ?? []) {
    if (l.symmetric) {
      const [a, b] = [is.key, l.key].sort();
      out.push({ id: `${a}~${b}:${l.type}`, source: a, target: b, type: l.type, words: l.words, directed: false });
    } else {
      const [source, target] = l.outward ? [is.key, l.key] : [l.key, is.key];
      out.push({ id: `${source}>${target}:${l.type}`, source, target, type: l.type, words: l.outward ? l.words : "", directed: true });
    }
  }
  const parentEdge = (parent: string, child: string): GEdge => ({
    id: `${parent}>${child}:${PARENT}`,
    source: parent,
    target: child,
    type: PARENT,
    words: "parent of",
    directed: true,
  });
  if (is.parent) out.push(parentEdge(is.parent, is.key));
  for (const s of is.subtasks ?? []) out.push(parentEdge(is.key, s));
  return out;
}

// neighbourKeys are the keys an issue links to, its parent and sub-tasks.
export function neighbourKeys(is: query.Issue): string[] {
  return [...new Set([...(is.links ?? []).map((l) => l.key), is.parent, ...(is.subtasks ?? [])])].filter(
    (k): k is string => !!k && k !== is.key,
  );
}

// add puts issues on the canvas, up to MAX_NODES, and every edge whose two
// ends are now on it. An issue already there is updated in place. It returns
// a new Graph and how many issues did not fit.
export function add(g: Graph, issues: { issue: query.Issue; inRun: boolean; fetchedAt: string }[]): { graph: Graph; left: number } {
  const nodes = new Map(g.nodes);
  const edges = new Map(g.edges);
  let left = 0;
  for (const n of issues) {
    const had = nodes.get(n.issue.key);
    if (!had && nodes.size >= MAX_NODES) {
      left++;
      continue;
    }
    // A wording seen from the outward side replaces an empty inward one.
    nodes.set(n.issue.key, { key: n.issue.key, issue: n.issue, inRun: n.inRun || !!had?.inRun, fetchedAt: n.fetchedAt || had?.fetchedAt || "" });
  }
  for (const n of nodes.values()) {
    for (const e of edgesOf(n.issue)) {
      if (!nodes.has(e.source) || !nodes.has(e.target)) continue;
      const had = edges.get(e.id);
      edges.set(e.id, had && had.words ? had : e);
    }
  }
  return { graph: { nodes, edges }, left };
}

// unloaded counts an issue's neighbours that are not on the canvas.
export function unloaded(g: Graph, key: string): number {
  const n = g.nodes.get(key);
  return n ? neighbourKeys(n.issue).filter((k) => !g.nodes.has(k)).length : 0;
}

export type Filter = { hiddenTypes: Set<string>; outside: boolean; finished: boolean };

// visible applies the filters. The issue in focus always stays.
// The issues in keep, the focus and the centre, stay whatever the filters say.
export function visible(g: Graph, f: Filter, ...keep: (string | null)[]): { nodes: GNode[]; edges: GEdge[] } {
  const nodes = [...g.nodes.values()].filter(
    (n) => keep.includes(n.key) || ((f.outside || n.inRun) && (f.finished || n.issue.statusCategory !== "done")),
  );
  const shown = new Set(nodes.map((n) => n.key));
  const edges = [...g.edges.values()].filter((e) => !f.hiddenTypes.has(e.type) && shown.has(e.source) && shown.has(e.target));
  return { nodes, edges };
}

// typeCounts counts the canvas's edges by type, for the filter list.
export function typeCounts(g: Graph): [string, number][] {
  const counts = new Map<string, number>();
  for (const e of g.edges.values()) counts.set(e.type, (counts.get(e.type) ?? 0) + 1);
  return [...counts].sort(([a], [b]) => a.localeCompare(b));
}

// History is the issues visited, with the place in it.
export type History = { keys: string[]; at: number };

export const visit = (h: History, key: string): History =>
  h.keys[h.at] === key ? h : { keys: [...h.keys.slice(0, h.at + 1), key], at: h.at + 1 };
export const back = (h: History): History => ({ ...h, at: Math.max(0, h.at - 1) });
export const forward = (h: History): History => ({ ...h, at: Math.min(h.keys.length - 1, h.at + 1) });

// search finds loaded issues whose key or summary contains the text.
export function search(g: Graph, text: string): GNode[] {
  const t = text.trim().toLowerCase();
  if (!t) return [];
  return [...g.nodes.values()].filter((n) => n.key.toLowerCase().includes(t) || n.issue.summary.toLowerCase().includes(t)).slice(0, 20);
}

// looksLikeKey is a full issue key, which Find fetches when it is not loaded.
export const looksLikeKey = (text: string) => /^[A-Z][A-Z0-9_]+-\d+$/i.test(text.trim());
