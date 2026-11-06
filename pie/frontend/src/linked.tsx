import type { BadgeTone } from "@agile-suite/core";
import type { query } from "../wailsjs/go/models";

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

export function allProjects(sets: query.Set[]): string[] {
  return [...new Set(sets.flatMap((s) => s.projects))].sort();
}

export function OutOfStep({ outOfStep }: { outOfStep: boolean }) {
  return <span className={outOfStep ? "pie-out-of-step" : "muted"}>{outOfStep ? "Out of step" : "In step"}</span>;
}
