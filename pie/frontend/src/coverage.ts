import type { query } from "../wailsjs/go/models";
import type { FixState } from "./mapData";

type Group = { baseline: string | null; projects: string[] };

// A cell holds the set's issues in that project, or says the project is a gap
// (its baseline has the set elsewhere, it has none), or none (the set never
// reaches its baseline, or the project is under no baseline).
export type Cell = { keys: string[] } | "gap" | "none";
export type CoverageRow = {
  set: query.Set;
  cells: Cell[];
  covered: number;
  of: number;
  state: FixState;
};
// share is null for a project under no baseline: it has no siblings to miss.
export type CoverageColumn = { project: string; share: number | null };

// coverage lays the sets against the projects of each baseline (#473). A
// set covers a project it has an issue in, out of the projects of every
// baseline it reaches. Its state is Fix missing when it has a gap, then
// behind when out of step, then fixed or open everywhere.
export function coverage(sets: query.Set[], groups: Group[], issues: Map<string, query.Issue>) {
  const reach = new Map<string, number>();
  const hits = new Map<string, number>();
  const rows: CoverageRow[] = sets.map((set) => {
    const at = (p: string) => set.keys.filter((k) => issues.get(k)?.project === p);
    const cells = groups.flatMap((g) => {
      const here = g.projects.map(at);
      const reached = g.baseline !== null && here.some((k) => k.length > 0);
      return here.map((keys, i): Cell => {
        const p = g.projects[i];
        if (reached) reach.set(p, (reach.get(p) ?? 0) + 1);
        if (keys.length > 0 && reached) hits.set(p, (hits.get(p) ?? 0) + 1);
        return keys.length > 0 ? { keys } : reached ? "gap" : "none";
      });
    });
    const covered = cells.filter((c) => typeof c === "object").length;
    const gaps = cells.filter((c) => c === "gap").length;
    const done = set.keys.every((k) => issues.get(k)?.statusCategory === "done");
    const state = gaps > 0 ? "missing" : set.outOfStep ? "behind" : done ? "everywhere" : "open";
    return { set, cells, covered, of: covered + gaps, state };
  });
  const columns: CoverageColumn[] = groups.flatMap((g) =>
    g.projects.map((project) => {
      const n = reach.get(project) ?? 0;
      return {
        project,
        share: g.baseline === null || n === 0 ? null : (hits.get(project) ?? 0) / n,
      };
    }),
  );
  return { rows, columns };
}

export const percent = (share: number) => `${Math.round(share * 100)}%`;

// leastCovered sorts by the share of its projects a set covers, lowest first.
// Array sort is stable, so equal sets keep the run's order.
export function leastCovered(rows: CoverageRow[]): CoverageRow[] {
  const share = (r: CoverageRow) => (r.of === 0 ? 1 : r.covered / r.of);
  return [...rows].sort((a, b) => share(a) - share(b));
}
