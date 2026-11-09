import type { query } from "../wailsjs/go/models";

// An edge is one Jira link, from its outward end to its inward end.
export type Edge = { from: string; to: string; type: string };
export type LinkType = { name: string; outward: string; inward: string };

// projectOf reads the project from a key, for an issue the run did not fetch.
export const projectOf = (key: string, issues: Map<string, query.Issue>) => issues.get(key)?.project ?? key.slice(0, key.lastIndexOf("-"));

// edges turns every issue's links into one edge per link. Jira lists a link on
// both of its issues, so both copies collapse into one. That also tells an old
// run apart: a link with both ends fetched has one copy marked outward, so a
// run with such links and no outward copy was stored before PIE read direction.
export function edges(issues: query.Issue[]): { edges: Edge[]; types: LinkType[]; stale: boolean } {
  const fetched = new Set(issues.map((i) => i.key));
  const seen = new Map<string, Edge>();
  const types = new Map<string, LinkType>();
  let directed = false;
  let paired = false;
  for (const i of issues) {
    for (const l of i.links ?? []) {
      directed ||= l.outward;
      paired ||= fetched.has(l.key);
      const t = types.get(l.type) ?? { name: l.type, outward: "", inward: "" };
      if (l.outward) t.outward = l.words;
      else t.inward = l.words;
      types.set(l.type, t);
      const e = l.outward ? { from: i.key, to: l.key, type: l.type } : { from: l.key, to: i.key, type: l.type };
      seen.set(`${e.from}>${e.to}>${e.type}`, e);
    }
  }
  return { edges: [...seen.values()], types: [...types.values()].sort((a, b) => a.name.localeCompare(b.name)), stale: paired && !directed };
}

// byCell groups edges by "fromProject>toProject" in one pass, so a grid of
// projects reads each cell from the map instead of scanning every edge.
export function byCell(edges: Edge[], issues: Map<string, query.Issue>): Map<string, Edge[]> {
  const out = new Map<string, Edge[]>();
  for (const e of edges) {
    const key = `${projectOf(e.from, issues)}>${projectOf(e.to, issues)}`;
    const list = out.get(key);
    if (list) list.push(e);
    else out.set(key, [e]);
  }
  return out;
}

const ends = (e: Edge, issues: Map<string, query.Issue>) => [projectOf(e.from, issues), projectOf(e.to, issues)] as const;
const sorted = (xs: Iterable<string>) => [...new Set(xs)].sort();

// busiest is the project at either end of the most links, "" when none.
export function busiest(edges: Edge[], issues: Map<string, query.Issue>): string {
  const count = new Map<string, number>();
  for (const e of edges) for (const p of new Set(ends(e, issues))) count.set(p, (count.get(p) ?? 0) + 1);
  let best = "";
  for (const [p, n] of count) if (n > (count.get(best) ?? 0) || (n === count.get(best) && p < best)) best = p;
  return best;
}

export type Spoke = { type: string; out: number; in: number };
export type Neighbour = { project: string; total: number; types: Spoke[] };

// neighbours lists the projects linked to centre, most links first, each with
// a count per link type: out for links starting in centre, in for the rest.
export function neighbours(centre: string, edges: Edge[], issues: Map<string, query.Issue>): Neighbour[] {
  const out = new Map<string, Map<string, Spoke>>();
  for (const e of edges) {
    const [from, to] = ends(e, issues);
    if (from === to || (from !== centre && to !== centre)) continue;
    const other = from === centre ? to : from;
    const types = out.get(other) ?? new Map<string, Spoke>();
    const spoke = types.get(e.type) ?? { type: e.type, out: 0, in: 0 };
    if (from === centre) spoke.out += 1;
    else spoke.in += 1;
    types.set(e.type, spoke);
    out.set(other, types);
  }
  return [...out]
    .map(([project, types]) => {
      const list = [...types.values()].sort((a, b) => a.type.localeCompare(b.type));
      return { project, total: list.reduce((n, s) => n + s.out + s.in, 0), types: list };
    })
    .sort((a, b) => b.total - a.total || a.project.localeCompare(b.project));
}

// flowLinks lays the links out for sankey(): the project a link starts in,
// its type, and the project it points to. Each link is one key, from>to.
export function flowLinks(edges: Edge[], issues: Map<string, query.Issue>) {
  const pairs = new Map<string, { from: string; to: string; keys: string[] }>();
  const add = (from: string, to: string, key: string) => {
    const pair = pairs.get(`${from}|${to}`) ?? { from, to, keys: [] };
    pair.keys.push(key);
    pairs.set(`${from}|${to}`, pair);
  };
  for (const e of edges) {
    const [from, to] = ends(e, issues);
    const key = `${e.from}>${e.to}`;
    add(`f:${from}`, `t:${e.type}`, key);
    add(`t:${e.type}`, `o:${to}`, key);
  }
  const column = (prefix: string, ids: string[]) => ids.map((id) => ({ id: `${prefix}${id}`, label: id }));
  return {
    columns: [
      column("f:", sorted(edges.map((e) => ends(e, issues)[0]))),
      column("t:", sorted(edges.map((e) => e.type))),
      column("o:", sorted(edges.map((e) => ends(e, issues)[1]))),
    ],
    links: [...pairs.values()],
  };
}
