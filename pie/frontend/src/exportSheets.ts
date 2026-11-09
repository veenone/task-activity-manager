import type { cases, cycles, query } from "../wailsjs/go/models";
import type { Edge } from "./linkData";
import { projectOf } from "./linkData";
import type { Stats } from "./stats";

// A Sheet is what ExportWorkbook writes: every cell is text, and Go stores a
// whole number as a number. Each builder below takes what its view shows,
// filters already applied, so the workbook matches the screen (#441).
export type Sheet = { name: string; header: string[]; rows: string[][] };

type Label = (project: string) => string;
const yes = (b: boolean) => (b ? "yes" : "no");
const num = (n: number | null) => (n === null ? "" : String(n));
const day = (stamp: string) => stamp.slice(0, 10);

// setSheet lists each issue of each linked set on a row of its own.
export function setSheet(sets: query.Set[], issues: Map<string, query.Issue>, label: Label): Sheet {
  return {
    name: "Linked sets",
    header: ["Linked set", "Key", "Project", "Status", "Summary", "In the query", "Out of step"],
    rows: sets.flatMap((s) =>
      s.keys.map((k) => {
        const i = issues.get(k);
        return [s.keys[0], k, label(projectOf(k, issues)), i?.status ?? "", i?.summary ?? "", yes(i?.matched ?? false), yes(s.outOfStep)];
      }),
    ),
  };
}

// mapSheet is the Project map's table: a row per set, a column per project.
export function mapSheet(sets: query.Set[], projects: string[], issues: Map<string, query.Issue>, label: Label): Sheet {
  return {
    name: "Project map",
    header: ["Linked set", ...projects.map(label), "State"],
    rows: sets.map((s) => [
      s.keys[0],
      ...projects.map((p) =>
        s.keys
          .filter((k) => issues.get(k)?.project === p)
          .map((k) => `${k} ${issues.get(k)?.status ?? ""}`.trim())
          .join("; "),
      ),
      s.outOfStep ? "Out of step" : "In step",
    ]),
  };
}

// linkSheets is the Link map's grid, with a total per row, and every link it
// counts.
export function linkSheets(
  projects: string[],
  between: (from: string, to: string) => Edge[],
  edges: Edge[],
  issues: Map<string, query.Issue>,
  label: Label,
): [Sheet, Sheet] {
  const cell = (here: Edge[]) => {
    const counts = new Map<string, number>();
    here.forEach((e) => counts.set(e.type, (counts.get(e.type) ?? 0) + 1));
    return [...counts].map(([t, n]) => `${t} ${n}`).join("; ");
  };
  const grid = {
    name: "Link grid",
    header: ["From \\ to", ...projects.map(label), "Total"],
    rows: projects.map((from) => [
      label(from),
      ...projects.map((to) => cell(between(from, to))),
      String(projects.reduce((n, to) => n + between(from, to).length, 0)),
    ]),
  };
  const end = (key: string) => [key, label(projectOf(key, issues)), issues.get(key)?.status ?? ""];
  const links = {
    name: "Links",
    header: ["From issue", "From project", "From status", "Link type", "To issue", "To project", "To status"],
    rows: edges.map((e) => [...end(e.from), e.type, ...end(e.to)]),
  };
  return [grid, links];
}

// statisticsSheets is the headline numbers and both tables, for the project
// and time range chosen.
export function statisticsSheets(s: Stats, label: Label): [Sheet, Sheet, Sheet] {
  return [
    {
      name: "Headline",
      header: ["Measure", "Value"],
      rows: [
        ["Issues the query matched", num(s.issues)],
        ["Not finished", num(s.open)],
        ["Median age not finished, days", num(s.medianAge)],
        ["Reach more than one project", num(s.multiProject)],
        ["Reach more than one product base", num(s.multiBase)],
        ["Linked sets out of step", num(s.outOfStep)],
      ],
    },
    {
      name: "Per project",
      header: ["Project", "Product base", "Issues", "Not finished", "Median age not finished, days"],
      rows: s.projects.map((p) => [label(p.project), p.bases.join(", "), num(p.issues), num(p.open), num(p.medianAge)]),
    },
    {
      name: "Finished by status",
      header: ["Status", "Issues", "With a resolution date", "Median days to resolve"],
      rows: s.finished.map((f) => [f.status, num(f.issues), num(f.dated), num(f.medianDays)]),
    },
  ];
}

// caseSheet lists each case's projects with verdict, reason and date. A case
// with no project still gets a row.
export function caseSheet(rows: cases.Detail[], label: Label): Sheet {
  return {
    name: "Cases",
    header: ["Case", "Project", "Verdict", "Reason", "Assessed"],
    rows: rows.flatMap((c) =>
      c.projects.length === 0
        ? [[c.key, "", "", "", ""]]
        : c.projects.map((p) => [c.key, label(p.key), p.verdict, p.reason, day(p.assessedAt)]),
    ),
  };
}

// cycleSheets is what changed since the last published cycle, and the
// cycle's own copy of the cases.
export function cycleSheets(d: cycles.Detail, label: Label): [Sheet, Sheet] {
  const ch = d.changes;
  return [
    {
      name: "Changes",
      header: ["Change", "Case", "Project", "Was", "Now"],
      rows: [
        ...ch.new.map((k) => ["New case", k, "", "", ""]),
        ...ch.gone.map((k) => ["Case gone", k, "", "", ""]),
        ...ch.verdicts.map((v) => ["Verdict changed", v.case, label(v.project), v.from, v.to]),
      ],
    },
    caseSheet(d.cases, label),
  ];
}

// resultsSheet is the Results table: every row its filters leave, across all
// pages, in its columns. Client visibility and Case appear when the table
// shows them.
export function resultsSheet(rows: query.Issue[], label: Label, show: { visibility: boolean; caseKeys: Set<string> | null }): Sheet {
  return {
    name: "Results",
    header: [
      "Key",
      "Project",
      "Type",
      "Summary",
      "Status",
      "Priority",
      ...(show.visibility ? ["Client visibility"] : []),
      ...(show.caseKeys ? ["Case"] : []),
      "Updated",
    ],
    rows: rows.map((i) => [
      i.key,
      label(i.project),
      i.type,
      i.summary,
      i.status,
      i.priority,
      ...(show.visibility ? [i.clientVisibility ?? ""] : []),
      ...(show.caseKeys ? [yes(show.caseKeys.has(i.key))] : []),
      day(i.updated ?? ""),
    ]),
  };
}
