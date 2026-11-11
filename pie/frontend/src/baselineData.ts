import type { cases, query } from "../wailsjs/go/models";
import { edgesOf, type GEdge } from "./graphModel";

// The baseline view's model (#465): which issues sit inside a baseline, which
// links stay inside it or cross its boundary, the blocker and impact walks,
// and the health numbers. Free of G6, so it is tested without a canvas.

export type BEdge = GEdge & { crossing: boolean };
export type Health = {
  inside: number;
  finishedShare: number;
  blockedFromOutside: number;
  external: number;
  outOfStep: number;
  medianAge: number | null;
};

// members are the baseline's own project and the projects derived from it.
export const members = (b: cases.Baseline) => new Set([b.key, ...(b.descendants ?? [])]);

const done = (is: query.Issue | undefined) => is?.statusCategory === "done";

// insideIssues are the run's issues in the baseline's projects, carrying the
// version when one is chosen ("" for every version).
export function insideIssues(issues: query.Issue[], projects: Set<string>, version: string): query.Issue[] {
  return issues.filter((i) => projects.has(i.project) && (!version || (i.fixVersions ?? []).includes(version)));
}

// versionsOf lists the Fix Versions the baseline's issues carry, newest first
// as people number them (3.10 after 3.9).
export function versionsOf(issues: query.Issue[], projects: Set<string>): string[] {
  const all = new Set(insideIssues(issues, projects, "").flatMap((i) => i.fixVersions ?? []));
  return [...all].sort((a, b) => b.localeCompare(a, undefined, { numeric: true }));
}

// baselineEdges are the links of the inside issues whose other end the run
// holds, each once, marked crossing when exactly one end is inside.
export function baselineEdges(inside: query.Issue[], run: Map<string, query.Issue>): BEdge[] {
  const keys = new Set(inside.map((i) => i.key));
  const out = new Map<string, BEdge>();
  for (const is of inside) {
    for (const e of edgesOf(is)) {
      if (!run.has(e.source) || !run.has(e.target)) continue;
      const had = out.get(e.id);
      if (had && had.words) continue;
      out.set(e.id, { ...e, crossing: keys.has(e.source) !== keys.has(e.target) });
    }
  }
  return [...out.values()];
}

// isBlocks is a link that says one issue holds another up: Jira's Blocks
// type, or any type worded "blocks". Clones, duplicates and relates-to say
// nothing about order, so the walks do not follow them.
export const isBlocks = (e: GEdge) => e.directed && (/block/i.test(e.type) || /^blocks$/i.test(e.words));

// walk follows blocks links from start: back to what blocks it ("blockers")
// or on to what it blocks ("impact"), through any number of steps, stopping
// at a cycle. It returns the steps in the order they were reached.
export function walk(start: string, edges: GEdge[], dir: "blockers" | "impact"): { keys: string[]; edges: GEdge[] } {
  const blocks = edges.filter(isBlocks);
  const seen = new Set([start]);
  const keys: string[] = [];
  const used: GEdge[] = [];
  let frontier = [start];
  while (frontier.length > 0) {
    const next: string[] = [];
    for (const k of frontier) {
      for (const e of blocks) {
        const [from, to] = dir === "blockers" ? [e.target, e.source] : [e.source, e.target];
        if (from !== k) continue;
        used.push(e);
        if (!seen.has(to)) {
          seen.add(to);
          keys.push(to);
          next.push(to);
        }
      }
    }
    frontier = next;
  }
  return { keys, edges: used };
}

// health counts the numbers on the baseline's health panel.
export function health(inside: query.Issue[], edges: BEdge[], run: Map<string, query.Issue>, sets: query.Set[], now: number): Health {
  const keys = new Set(inside.map((i) => i.key));
  const open = inside.filter((i) => !done(i));
  const blocked = new Set(
    edges
      .filter((e) => isBlocks(e) && e.crossing && keys.has(e.target) && !done(run.get(e.target)) && !done(run.get(e.source)))
      .map((e) => e.target),
  );
  const external = new Set(edges.filter((e) => e.crossing).map((e) => (keys.has(e.source) ? e.target : e.source)));
  const ages = open
    .map((i) => Math.floor((now - Date.parse(i.created)) / 86_400_000))
    .filter((d) => Number.isFinite(d))
    .sort((a, b) => a - b);
  const mid = ages.length / 2;
  return {
    inside: inside.length,
    finishedShare: inside.length ? (inside.length - open.length) / inside.length : 0,
    blockedFromOutside: blocked.size,
    external: external.size,
    outOfStep: sets.filter((s) => s.outOfStep && s.keys.some((k) => keys.has(k))).length,
    medianAge: ages.length === 0 ? null : ages.length % 2 ? ages[Math.floor(mid)] : Math.round((ages[mid - 1] + ages[mid]) / 2),
  };
}

// fold keeps the inside issues worth drawing: those with a link out of the
// baseline, those on a traced walk, and every issue of a project opened. The
// rest are counted per project, for "+ N more" clusters.
export function fold(
  inside: query.Issue[],
  edges: BEdge[],
  traced: Set<string>,
  opened: Set<string>,
): { shown: query.Issue[]; hidden: Map<string, number> } {
  const linkedOut = new Set(edges.filter((e) => e.crossing).flatMap((e) => [e.source, e.target]));
  const shown: query.Issue[] = [];
  const hidden = new Map<string, number>();
  for (const i of inside) {
    if (linkedOut.has(i.key) || traced.has(i.key) || opened.has(i.project)) shown.push(i);
    else hidden.set(i.project, (hidden.get(i.project) ?? 0) + 1);
  }
  return { shown, hidden };
}
