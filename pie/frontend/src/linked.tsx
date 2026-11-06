import type { BadgeTone } from "@agile-suite/core";
import type { cases, query } from "../wailsjs/go/models";

// tone reads Jira's status category, so no status name is listed here.
export function tone(statusCategory: string): BadgeTone {
  if (statusCategory === "done") return "closed";
  if (statusCategory === "indeterminate") return "active";
  return "future";
}

// ageText counts whole days from the date Jira stamped, ignoring its time
// zone: an age in days does not need it.
export function ageText(created: string, now = new Date()): string {
  const day = Date.parse(created.slice(0, 10));
  if (Number.isNaN(day)) return "";
  const days = Math.max(0, Math.floor((now.getTime() - day) / 86_400_000));
  return days === 1 ? "1 day old" : `${days} days old`;
}

export function byKey(result: query.Result): Map<string, query.Issue> {
  return new Map(result.issues.map((i) => [i.key, i]));
}

// projectNames maps each project key in the run to the name Jira gives it. A
// run stored before names were read has none, and its keys stand alone.
export function projectNames(result: query.Result): Map<string, string> {
  return new Map(result.issues.filter((i) => i.projectName).map((i) => [i.project, i.projectName]));
}

export function projectLabel(key: string, name?: string): string {
  return name ? `${name} (${key})` : key;
}

export function allProjects(sets: query.Set[]): string[] {
  return [...new Set(sets.flatMap((s) => s.projects))].sort();
}

// baselineGroups orders projects by the analyst's baselines: each baseline,
// then its descendants, then the projects under none (baseline null). A
// baseline with no project in the list is left out.
export function baselineGroups(projects: string[], baselines: cases.Baseline[]) {
  const here = new Set(projects);
  const placed = new Set<string>();
  const groups: { baseline: string | null; projects: string[] }[] = [];
  for (const b of baselines) {
    const keys = [b.key, ...b.descendants].filter((k) => here.has(k));
    keys.forEach((k) => placed.add(k));
    if (keys.length > 0) groups.push({ baseline: b.key, projects: keys });
  }
  const rest = projects.filter((p) => !placed.has(p));
  if (rest.length > 0) groups.push({ baseline: null, projects: rest });
  return groups;
}

export function OutOfStep({ outOfStep }: { outOfStep: boolean }) {
  return <span className={outOfStep ? "pie-out-of-step" : "muted"}>{outOfStep ? "Out of step" : "In step"}</span>;
}
