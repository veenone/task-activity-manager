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

// MapFilter narrows the Project map. baseline is "" for any, "-" for the
// projects under none, or a baseline's key; project and key are "" for any.
export type MapFilter = { baseline: string; project: string; key: string };

// filterSets keeps the sets that reach a project under the baseline, reach
// the project, and hold a key containing the text, ignoring case.
export function filterSets<S extends { keys: string[]; projects: string[] }>(
  sets: S[],
  groups: { baseline: string | null; projects: string[] }[],
  f: MapFilter,
): S[] {
  const under = f.baseline === "" ? null : new Set(groups.find((g) => (g.baseline ?? "-") === f.baseline)?.projects ?? []);
  const text = f.key.trim().toLowerCase();
  return sets.filter(
    (s) =>
      (!under || s.projects.some((p) => under.has(p))) &&
      (!f.project || s.projects.includes(f.project)) &&
      (!text || s.keys.some((k) => k.toLowerCase().includes(text))),
  );
}
