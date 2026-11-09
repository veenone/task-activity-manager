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
