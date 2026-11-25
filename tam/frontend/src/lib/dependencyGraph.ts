// What the Dependencies view draws (#612), worked out without a canvas: the
// filters, a picked issue's chain, the cycle check, and the nodes and edges
// handed to the shared G6 canvas. A link points from its outward issue to
// its inward one, so "A blocks B" is an arrow from A to B and B waits on A.

import type { CanvasEdge, CanvasLayout, CanvasNode } from "@agile-suite/core";
import { ASSIGNEE_NONE } from "../api";
import type { DependencyGraph, DependencyLink, Issue } from "../api";

export const BLOCKS = "Blocks";

// A chain reads left to right: what an issue waits on sits to its left.
export const DEP_LAYOUT = { type: "antv-dagre", rankdir: "LR", nodesep: 18, ranksep: 70 } as CanvasLayout;

export interface DepFilter {
  types: string[];
  sprintId: string;
  status: string;
  assignee: string;
}

export const NO_DEP_FILTER: DepFilter = Object.freeze({ types: [], sprintId: "", status: "", assignee: "" }) as DepFilter;

export const isBlocks = (type: string) => type.toLowerCase() === BLOCKS.toLowerCase();

// defaultTypes is Jira's Blocks type when the instance has it, which is what
// a dependency means here, and every type when it does not.
export function defaultTypes(names: string[]): string[] {
  const blocks = names.find(isBlocks);
  return blocks ? [blocks] : names;
}

// edgeId names a link on the canvas: Jira's id, or the journal row of a
// pending add, which has none yet.
export function edgeId(l: DependencyLink): string {
  if (l.id) return l.id;
  if (l.pending) return `p${l.pendingId}`;
  return `${l.type}|${l.outward}|${l.inward}`;
}

function matches(iss: Issue | undefined, f: DepFilter): boolean {
  if (!f.sprintId && !f.status && !f.assignee) return true;
  if (!iss) return false;
  if (f.sprintId && iss.sprintId !== f.sprintId) return false;
  if (f.status && iss.status !== f.status) return false;
  if (f.assignee && iss.assignee !== (f.assignee === ASSIGNEE_NONE ? "" : f.assignee)) return false;
  return true;
}

export interface VisibleGraph {
  links: DependencyLink[];
  keys: Set<string>;
}

// visibleGraph keeps the links of the chosen types that have at least one
// end matching the sprint, status and assignee filters, so a sprint's issues
// still show what they wait on outside it, and the issues those links join.
export function visibleGraph(g: DependencyGraph, f: DepFilter): VisibleGraph {
  const byKey = new Map(g.issues.map((i) => [i.key, i]));
  const types = new Set(f.types.map((t) => t.toLowerCase()));
  const links = g.links.filter(
    (l) => types.has(l.type.toLowerCase()) && (matches(byKey.get(l.outward), f) || matches(byKey.get(l.inward), f)),
  );
  const keys = new Set<string>();
  for (const l of links) keys.add(l.outward).add(l.inward);
  return { links, keys };
}

export interface Chain {
  up: Set<string>;
  down: Set<string>;
  edges: Set<string>;
}

// chain follows the links from key both ways: upstream to what it waits on,
// downstream to what waits on it.
export function chain(links: DependencyLink[], key: string): Chain {
  const walk = (from: (l: DependencyLink) => string, to: (l: DependencyLink) => string) => {
    const reached = new Set<string>();
    const edges = new Set<string>();
    const todo = [key];
    while (todo.length) {
      const at = todo.pop()!;
      for (const l of links) {
        if (from(l) !== at) continue;
        edges.add(edgeId(l));
        const next = to(l);
        if (next !== key && !reached.has(next)) {
          reached.add(next);
          todo.push(next);
        }
      }
    }
    return { reached, edges };
  };
  const up = walk((l) => l.inward, (l) => l.outward);
  const down = walk((l) => l.outward, (l) => l.inward);
  return { up: up.reached, down: down.reached, edges: new Set([...up.edges, ...down.edges]) };
}

// closesCycle says whether a new "outward blocks inward" link would close a
// loop of Blocks links: true when inward already blocks outward, directly or
// through others. A link whose removal is pending no longer counts.
export function closesCycle(links: DependencyLink[], outward: string, inward: string): boolean {
  const live = links.filter((l) => isBlocks(l.type) && !l.removal);
  return outward === inward || chain(live, inward).down.has(outward);
}

const short = (s: string) => (s.length > 40 ? `${s.slice(0, 39)}…` : s);

// canvasData is the canvas's nodes and edges. With an issue in focus its
// chain is lit and everything else faded; a pending link is dashed and
// labelled, and an issue that is not cached is drawn dashed.
export function canvasData(g: DependencyGraph, v: VisibleGraph, focus: string | null, selectedLink: string): { nodes: CanvasNode[]; edges: CanvasEdge[] } {
  const byKey = new Map(g.issues.map((i) => [i.key, i]));
  const far = new Map(g.external.map((e) => [e.key, e]));
  const c = focus && v.keys.has(focus) ? chain(v.links, focus) : null;
  const inChain = (k: string) => !c || k === focus || c.up.has(k) || c.down.has(k);
  const nodes: CanvasNode[] = [...v.keys].sort().map((k) => {
    const iss = byKey.get(k);
    return {
      id: k,
      label: k,
      sub: short(iss?.summary ?? far.get(k)?.summary ?? ""),
      status: iss?.statusCategory,
      dashed: !iss,
      lit: !!c && k !== focus && inChain(k),
      faded: !inChain(k),
    };
  });
  const edges: CanvasEdge[] = v.links.map((l) => {
    const id = edgeId(l);
    return {
      id,
      source: l.outward,
      target: l.inward,
      type: l.type,
      directed: true,
      dashed: l.pending,
      label: l.removal ? "removal pending" : l.pending ? "pending" : isBlocks(l.type) ? undefined : l.type,
      lit: id === selectedLink || (!!c && c.edges.has(id)),
      faded: !!c && !c.edges.has(id),
    };
  });
  return { nodes, edges };
}

// DependencyScope is what the backend exports: with a focus, its chain's
// issues and the links of types between them; with none, the whole profile.
export interface DependencyScope {
  focus: string;
  keys: string[];
  types: string[];
}

// GraphExport is an export's scope and the picture to draw for it.
export interface GraphExport {
  scope: DependencyScope;
  nodes: CanvasNode[];
  edges: CanvasEdge[];
}

// chainExport is the picked issue's chain as the view lights it, drawn with
// nothing else, so the picture holds no faded strangers.
export function chainExport(g: DependencyGraph, v: VisibleGraph, focus: string, types: string[]): GraphExport {
  const c = chain(v.links, focus);
  const keys = new Set([focus, ...c.up, ...c.down]);
  const only: VisibleGraph = { links: v.links.filter((l) => c.edges.has(edgeId(l))), keys };
  return { scope: { focus, keys: [...keys], types }, ...canvasData(g, only, focus, "") };
}

// wholeExport is every link of every type, whatever the view's filters; the
// issues with no link go in the workbook, not the picture.
export function wholeExport(g: DependencyGraph): GraphExport {
  return { scope: { focus: "", keys: [], types: [] }, ...canvasData(g, visibleGraph(g, { ...NO_DEP_FILTER, types: g.links.map((l) => l.type) }), null, "") };
}
