import type { query } from "../wailsjs/go/models";

// statistics counts a run's matched issues for the Statistics page. Issues
// reached only through a link count towards reach and nothing else, so the
// totals match what the analyst asked Jira for. Everything is computed from
// the run already on screen; nothing here asks Jira.

const DAY = 86_400_000;

export interface Filter {
  project: string; // "" for every project
  withinDays: number; // 0 for all time
}

export interface Bar {
  label: string;
  value: number;
}

export interface ProjectRow {
  project: string;
  bases: string[];
  issues: number;
  open: number;
  medianAge: number | null;
}

export interface StatusRow {
  status: string;
  issues: number;
  dated: number;
  medianDays: number | null;
}

export interface Stats {
  issues: number;
  open: number;
  medianAge: number | null;
  multiProject: number;
  multiBase: number;
  outOfStep: number;
  created: Bar[];
  ages: Bar[];
  reach: Bar[];
  projects: ProjectRow[];
  finished: StatusRow[];
}

const AGE_BANDS: [string, number][] = [
  ["< 30", 30],
  ["30-90", 90],
  ["90-365", 365],
  ["1-2 yrs", 730],
  ["> 2 yrs", Infinity],
];

const ageBand = (age: number) => AGE_BANDS.findIndex(([, below]) => age < below);

// median rounds to whole days. An empty list has none, and the page shows a
// dash rather than a zero that would read as a measurement.
export function median(xs: number[]): number | null {
  if (xs.length === 0) return null;
  const s = [...xs].sort((a, b) => a - b);
  const m = Math.floor(s.length / 2);
  return s.length % 2 ? s[m] : Math.round((s[m - 1] + s[m]) / 2);
}

// Jira writes offsets as +0700, which Date.parse only reads as +07:00.
function time(jira: string): number {
  return Date.parse(jira.replace(/([+-]\d\d)(\d\d)$/, "$1:$2"));
}

function days(from: string, to: number): number {
  return Math.floor((to - time(from)) / DAY);
}

const open = (i: query.Issue) => i.statusCategory !== "done";

export function statistics(res: query.Result, filter: Filter, now: number): Stats {
  const shown = res.issues.filter(
    (i) =>
      i.matched &&
      (filter.project === "" || i.project === filter.project) &&
      (filter.withinDays === 0 || days(i.created, now) <= filter.withinDays),
  );
  const byKey = new Map(res.issues.map((i) => [i.key, i]));
  const setOf = new Map<string, query.Set>();
  for (const s of res.sets ?? []) for (const k of s.keys) setOf.set(k, s);

  const reachOf = (i: query.Issue) => {
    const set = setOf.get(i.key);
    const members = set ? set.keys.map((k) => byKey.get(k)).filter((m) => m !== undefined) : [i];
    return { projects: set ? set.projects.length : 1, bases: new Set(members.flatMap((m) => m.bases ?? [])).size };
  };
  const reaches = shown.map(reachOf);
  const unfinished = shown.filter(open);
  const ages = unfinished.map((i) => days(i.created, now));

  const reach: Bar[] = [];
  const most = Math.max(0, ...reaches.map((r) => r.projects));
  for (let n = 1; n <= most; n += 1) reach.push({ label: String(n), value: reaches.filter((r) => r.projects === n).length });

  return {
    issues: shown.length,
    open: unfinished.length,
    medianAge: median(ages),
    multiProject: reaches.filter((r) => r.projects > 1).length,
    multiBase: reaches.filter((r) => r.bases > 1).length,
    outOfStep: (res.sets ?? []).filter((s) => s.outOfStep && shown.some((i) => s.keys.includes(i.key))).length,
    created: months(shown),
    ages: AGE_BANDS.map(([label], b) => ({ label, value: ages.filter((a) => ageBand(a) === b).length })),
    reach,
    projects: projectRows(shown, now),
    finished: statusRows(shown.filter((i) => !open(i))),
  };
}

// months runs from the oldest issue's month to the newest's, so a month with
// nothing created still has its place on the axis.
function months(issues: query.Issue[]): Bar[] {
  const keys = issues.map((i) => i.created.slice(0, 7)).sort();
  if (keys.length === 0) return [];
  const out: Bar[] = [];
  let [y, m] = keys[0].split("-").map(Number);
  const last = keys[keys.length - 1];
  for (;;) {
    const label = `${y}-${String(m).padStart(2, "0")}`;
    out.push({ label, value: keys.filter((k) => k === label).length });
    if (label >= last) return out;
    [y, m] = m === 12 ? [y + 1, 1] : [y, m + 1];
  }
}

function projectRows(issues: query.Issue[], now: number): ProjectRow[] {
  const names = [...new Set(issues.map((i) => i.project))].sort();
  return names.map((project) => {
    const mine = issues.filter((i) => i.project === project);
    const unfinished = mine.filter(open);
    return {
      project,
      bases: [...new Set(mine.flatMap((i) => i.bases ?? []))].sort(),
      issues: mine.length,
      open: unfinished.length,
      medianAge: median(unfinished.map((i) => days(i.created, now))),
    };
  });
}

// statusRows takes the status names from Jira, so Closed and Rejected stay
// apart and Corrected, which Jira never dates, is counted with no time.
function statusRows(finished: query.Issue[]): StatusRow[] {
  const names = [...new Set(finished.map((i) => i.status))];
  return names
    .map((status) => {
      const dated = finished.filter((i) => i.status === status && i.resolved);
      return {
        status,
        issues: finished.filter((i) => i.status === status).length,
        dated: dated.length,
        medianDays: median(dated.map((i) => days(i.created, time(i.resolved)))),
      };
    })
    .sort((a, b) => b.issues - a.issues || a.status.localeCompare(b.status));
}
