import { expect, it } from "vitest";
import type { query } from "../wailsjs/go/models";
import type { Edge } from "./linkData";
import { projectGraph } from "./projectGraph";

const issue = (key: string) =>
  ({ key, project: key.split("-")[0] }) as query.Issue;
const issues = new Map(
  ["A-1", "A-2", "B-1", "C-1", "D-1"].map((k) => [k, issue(k)]),
);
// A clones B twice and blocks C once; C relates to D; B blocks B (inside).
const web: Edge[] = [
  { from: "A-1", to: "B-1", type: "Cloners" },
  { from: "A-2", to: "B-1", type: "Cloners" },
  { from: "A-1", to: "C-1", type: "Blocks" },
  { from: "C-1", to: "D-1", type: "Relates" },
  { from: "B-1", to: "B-1", type: "Blocks" },
];
const all = { hidden: new Set<string>(), min: 1, secondRing: false };

it("puts the centre's neighbours around it, one edge per pair and type with its count", () => {
  const g = projectGraph(web, issues, "A", all);
  expect(g.nodes.map((n) => [n.project, n.ring, n.issues])).toEqual([
    ["A", 0, 2],
    ["B", 1, 1],
    ["C", 1, 1],
  ]);
  expect(g.edges).toEqual([
    { id: "A>B:Cloners", source: "A", target: "B", type: "Cloners", count: 2 },
    { id: "A>C:Blocks", source: "A", target: "C", type: "Blocks", count: 1 },
  ]);
});

it("reaches a second ring on request, and drops hidden types and thin lines", () => {
  expect(
    projectGraph(web, issues, "A", { ...all, secondRing: true }).nodes.map(
      (n) => [n.project, n.ring],
    ),
  ).toEqual([
    ["A", 0],
    ["B", 1],
    ["C", 1],
    ["D", 2],
  ]);
  expect(
    projectGraph(web, issues, "A", { ...all, min: 2 }).nodes.map(
      (n) => n.project,
    ),
  ).toEqual(["A", "B"]);
  expect(
    projectGraph(web, issues, "A", {
      ...all,
      hidden: new Set(["Cloners"]),
    }).edges.map((e) => e.id),
  ).toEqual(["A>C:Blocks"]);
});

it("keeps a centre with no links left, alone", () => {
  expect(projectGraph(web, issues, "A", { ...all, min: 5 })).toEqual({
    nodes: [{ project: "A", ring: 0, issues: 2 }],
    edges: [],
  });
});
