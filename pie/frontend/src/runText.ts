import type { query } from "../wailsjs/go/models";

// countText is the status bar's summary of the rows a run matched.
export function countText(res: query.Result, matched: number): string {
  if (res.truncated) return `Showing the first ${matched} of ${res.total} issues`;
  return res.total === 1 ? "1 issue" : `${res.total} issues`;
}

// runAtText gives a stored run's time in the analyst's own time zone.
export function runAtText(iso: string): string {
  const d = new Date(iso);
  const two = (n: number) => String(n).padStart(2, "0");
  return `Last run ${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())} ${two(d.getHours())}:${two(d.getMinutes())}`;
}
