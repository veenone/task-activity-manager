import { expect, it } from "vitest";
import type { query } from "../wailsjs/go/models";
import { coverage, leastCovered } from "./coverage";

const issue = (key: string, statusCategory: string) => ({ key, project: key.split("-")[0], statusCategory }) as query.Issue;
const issues = new Map(
  [issue("A-1", "done"), issue("B-1", "new"), issue("A-2", "done"), issue("C-2", "done"), issue("D-3", "new"), issue("D-4", "new")].map((i) => [i.key, i]),
);
const set = (keys: string[], outOfStep = false) => ({ keys, projects: [], outOfStep }) as unknown as query.Set;
// Baseline A holds A, B and C; D is under none.
const groups = [
  { baseline: "A", projects: ["A", "B", "C"] },
  { baseline: null, projects: ["D"] },
];

it("marks a gap only where the set reaches the baseline through another project", () => {
  const { rows } = coverage([set(["A-1", "B-1"], true), set(["D-3"])], groups, issues);
  expect(rows[0].cells).toEqual([{ keys: ["A-1"] }, { keys: ["B-1"] }, "gap", "none"]);
  expect(rows[1].cells).toEqual(["none", "none", "none", { keys: ["D-3"] }]);
});

it("counts what a set covers and names its state", () => {
  const { rows } = coverage([set(["A-1", "B-1"], true), set(["A-2", "C-2"]), set(["A-1", "B-1", "C-2"], true), set(["D-3", "D-4"])], groups, issues);
  expect(rows.map((r) => [r.covered, r.of, r.state])).toEqual([
    [2, 3, "missing"],
    [2, 3, "missing"],
    [3, 3, "behind"],
    [1, 1, "open"],
  ]);
  const { rows: done } = coverage([set(["A-2"])], [{ baseline: "A", projects: ["A"] }], issues);
  expect(done[0].state).toBe("everywhere");
});

it("gives each baseline project the share of the baseline's sets it is in", () => {
  const { columns } = coverage([set(["A-1", "B-1"]), set(["A-2", "C-2"]), set(["D-3"])], groups, issues);
  expect(columns).toEqual([
    { project: "A", share: 1 },
    { project: "B", share: 0.5 },
    { project: "C", share: 0.5 },
    { project: "D", share: null },
  ]);
});

it("puts the least covered sets first and keeps the run's order between equals", () => {
  const { rows } = coverage([set(["A-2", "C-2"]), set(["D-3"]), set(["A-1"]), set(["B-1", "C-2"])], groups, issues);
  expect(leastCovered(rows).map((r) => r.set.keys[0])).toEqual(["A-1", "A-2", "B-1", "D-3"]);
});
