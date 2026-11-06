import type { query } from "../wailsjs/go/models";

// An edge is one Jira link, from its outward end to its inward end.
export type Edge = { from: string; to: string; type: string };
export type LinkType = { name: string; outward: string; inward: string };

// projectOf reads the project from a key, for an issue the run did not fetch.
export const projectOf = (key: string, issues: Map<string, query.Issue>) => issues.get(key)?.project ?? key.slice(0, key.lastIndexOf("-"));

// edges turns every issue's links into one edge per link. Jira lists a link on
// both of its issues, so both copies collapse into one.
export function edges(issues: query.Issue[]): { edges: Edge[]; types: LinkType[]; directed: boolean } {
  const seen = new Map<string, Edge>();
  const types = new Map<string, LinkType>();
  let directed = false;
  for (const i of issues) {
    for (const l of i.links ?? []) {
      directed ||= l.outward;
      const t = types.get(l.type) ?? { name: l.type, outward: "", inward: "" };
      if (l.outward) t.outward = l.words;
      else t.inward = l.words;
      types.set(l.type, t);
      const e = l.outward ? { from: i.key, to: l.key, type: l.type } : { from: l.key, to: i.key, type: l.type };
      seen.set(`${e.from}>${e.to}>${e.type}`, e);
    }
  }
  return { edges: [...seen.values()], types: [...types.values()].sort((a, b) => a.name.localeCompare(b.name)), directed };
}
