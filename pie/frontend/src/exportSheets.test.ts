import { expect, it } from "vitest";
import type { query } from "../wailsjs/go/models";
import { caseSheet, cycleSheets, linkSheets, mapSheet, resultsSheet, setSheet, statisticsSheets } from "./exportSheets";

const issue = (key: string, status: string, statusCategory: string, matched = true) =>
  ({ key, project: key.split("-")[0], status, statusCategory, summary: `About ${key}`, matched }) as query.Issue;
const issues = new Map([issue("A-1", "Closed", "done"), issue("B-1", "Open", "new", false)].map((i) => [i.key, i]));
const sets = [{ keys: ["A-1", "B-1"], projects: ["A", "B"], outOfStep: true }] as query.Set[];
const label = (p: string) => (p === "A" ? "Alpha (A)" : p);

it("lists each issue of each linked set on its own row", () => {
  expect(setSheet(sets, issues, label)).toEqual({
    name: "Linked sets",
    header: ["Linked set", "Key", "Project", "Status", "Summary", "In the query", "Out of step"],
    rows: [
      ["A-1", "A-1", "Alpha (A)", "Closed", "About A-1", "yes", "yes"],
      ["A-1", "B-1", "B", "Open", "About B-1", "no", "yes"],
    ],
  });
});

it("writes the coverage matrix, with a gap where the baseline misses the set", () => {
  const groups = [
    { baseline: "A", projects: ["A", "B", "C"] },
    { baseline: null, projects: ["D"] },
  ];
  expect(mapSheet(sets, groups, issues, label)).toEqual({
    name: "Coverage",
    header: ["Linked set", "Alpha (A)", "B", "C", "D", "Covered", "State"],
    rows: [
      ["Coverage", "100%", "100%", "0%", "", "", ""],
      ["A-1", "A-1 Closed", "B-1 Open", "gap", "", "2 of 3", "Fix missing"],
    ],
  });
});

it("counts links per project pair and lists every link", () => {
  const edges = [{ from: "A-1", to: "B-1", type: "Cloners" }];
  const [grid, links] = linkSheets(["A", "B"], (f, t) => edges.filter((e) => f === "A" && t === "B" && e), edges, issues, label);
  expect(grid).toEqual({
    name: "Link grid",
    header: ["From \\ to", "Alpha (A)", "B", "Total"],
    rows: [
      ["Alpha (A)", "", "Cloners 1", "1"],
      ["B", "", "", "0"],
    ],
  });
  expect(links.rows).toEqual([["A-1", "Alpha (A)", "Closed", "Cloners", "B-1", "B", "Open"]]);
});

it("writes the statistics as numbers, with an empty cell for no median", () => {
  const [headline, perProject, finished] = statisticsSheets(
    {
      issues: 2,
      open: 1,
      medianAge: 4,
      multiProject: 1,
      multiBase: 0,
      outOfStep: 1,
      created: [],
      ages: [],
      reach: [],
      projects: [{ project: "A", bases: ["X"], issues: 2, open: 1, medianAge: null }],
      finished: [{ status: "Closed", issues: 1, dated: 1, medianDays: 3 }],
    },
    label,
  );
  expect(headline.rows[0]).toEqual(["Issues the query matched", "2"]);
  expect(perProject.rows).toEqual([["Alpha (A)", "X", "2", "1", ""]]);
  expect(finished.rows).toEqual([["Closed", "1", "1", "3"]]);
});

it("lists each case's projects with verdict and reason, and a cycle's changes", () => {
  const detail = [
    { key: "ACME-1", projects: [{ key: "A", verdict: "affected", reason: "ships it", assessedAt: "2026-11-07T10:00:00Z" }] },
    { key: "ACME-2", projects: [] },
  ];
  expect(caseSheet(detail as never, label).rows).toEqual([
    ["ACME-1", "Alpha (A)", "affected", "ships it", "2026-11-07"],
    ["ACME-2", "", "", "", ""],
  ]);
  const [changes] = cycleSheets(
    {
      name: "Week 45",
      cases: detail,
      changes: {
        since: "Week 44",
        new: ["ACME-2"],
        gone: ["ACME-0"],
        verdicts: [{ case: "ACME-1", project: "A", from: "investigating", to: "affected" }],
      },
    } as never,
    label,
  );
  expect(changes.rows).toEqual([
    ["New case", "ACME-2", "", "", ""],
    ["Case gone", "ACME-0", "", "", ""],
    ["Verdict changed", "ACME-1", "Alpha (A)", "investigating", "affected"],
  ]);
});

it("writes the Results rows in the table's columns", () => {
  const rows = [
    { key: "A-1", project: "A", type: "Bug", summary: "Crash", status: "Open", priority: "High", clientVisibility: "Yes", updated: "2026-10-02T00:00:00.000+0000" },
  ] as query.Issue[];
  expect(resultsSheet(rows, label, { visibility: true, caseKeys: new Set(["A-1"]) })).toEqual({
    name: "Results",
    header: ["Key", "Project", "Type", "Summary", "Status", "Priority", "Client visibility", "Case", "Updated"],
    rows: [["A-1", "Alpha (A)", "Bug", "Crash", "Open", "High", "Yes", "yes", "2026-10-02"]],
  });
  expect(resultsSheet(rows, label, { visibility: false, caseKeys: null }).header).toEqual(["Key", "Project", "Type", "Summary", "Status", "Priority", "Updated"]);
});
