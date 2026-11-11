import { expect, it } from "vitest";
import type { query } from "../wailsjs/go/models";
import { baselineEdges, fold, health, insideIssues, isBlocks, members, versionsOf, walk } from "./baselineData";

const link = (key: string, type: string, outward: boolean, words: string) => ({ key, type, outward, words, symmetric: false });
const NOW = Date.parse("2026-11-10T00:00:00Z");
const ago = (days: number) => new Date(NOW - days * 86_400_000).toISOString();
const issue = (key: string, more: Partial<query.Issue> = {}) =>
  ({ key, project: key.split("-")[0], statusCategory: "new", created: ago(10), links: [], subtasks: [], parent: "", fixVersions: [], ...more }) as unknown as query.Issue;

// Baseline COS with derived COL. RFT-91 (outside) blocks COS-210, which blocks
// COS-188, which blocks COL-45. COS-150 clones TLS-12 (outside).
const run = [
  issue("RFT-91", { links: [link("COS-210", "Blocks", true, "blocks")] }),
  issue("COS-210", { fixVersions: ["3.4.0"], created: ago(30), links: [link("RFT-91", "Blocks", false, "is blocked by"), link("COS-188", "Blocks", true, "blocks")] }),
  issue("COS-188", { fixVersions: ["3.4.0"], created: ago(20), links: [link("COS-210", "Blocks", false, "is blocked by"), link("COL-45", "Blocks", true, "blocks")] }),
  issue("COL-45", { fixVersions: ["3.4.0", "3.10.0"], created: ago(5), links: [link("COS-188", "Blocks", false, "is blocked by")] }),
  issue("COS-150", { fixVersions: ["3.3.0"], statusCategory: "done", links: [link("TLS-12", "Cloners", true, "clones")] }),
  issue("COS-231", { fixVersions: ["3.3.0"] }),
  issue("TLS-12"),
];
const byKey = new Map(run.map((i) => [i.key, i]));
const scope = members({ key: "COS", descendants: ["COL"] } as never);

it("finds the baseline's issues, by version, and lists its versions newest first", () => {
  expect(insideIssues(run, scope, "").map((i) => i.key)).toEqual(["COS-210", "COS-188", "COL-45", "COS-150", "COS-231"]);
  expect(insideIssues(run, scope, "3.4.0").map((i) => i.key)).toEqual(["COS-210", "COS-188", "COL-45"]);
  expect(versionsOf(run, scope)).toEqual(["3.10.0", "3.4.0", "3.3.0"]);
});

it("keeps each link once and marks those crossing the boundary", () => {
  const edges = baselineEdges(insideIssues(run, scope, ""), byKey);
  const crossing = edges.filter((e) => e.crossing).map((e) => e.id).sort();
  expect(crossing).toEqual(["COS-150>TLS-12:Cloners", "RFT-91>COS-210:Blocks"]);
  expect(edges.filter((e) => !e.crossing).map((e) => e.id).sort()).toEqual(["COS-188>COL-45:Blocks", "COS-210>COS-188:Blocks"]);
  expect(edges.find((e) => e.id === "RFT-91>COS-210:Blocks")!.words).toBe("");
  expect(edges.filter(isBlocks)).toHaveLength(3);
});

it("walks back to every blocker and on to every issue affected, along blocks only", () => {
  const edges = baselineEdges(insideIssues(run, scope, ""), byKey);
  expect(walk("COL-45", edges, "blockers").keys).toEqual(["COS-188", "COS-210", "RFT-91"]);
  expect(walk("COS-210", edges, "impact").keys).toEqual(["COS-188", "COL-45"]);
  // Clones say nothing about order.
  expect(walk("COS-150", edges, "impact").keys).toEqual([]);
});

it("stops at a cycle", () => {
  const loop = [
    issue("COS-1", { links: [link("COS-2", "Blocks", true, "blocks")] }),
    issue("COS-2", { links: [link("COS-1", "Blocks", true, "blocks")] }),
  ];
  const edges = baselineEdges(loop, new Map(loop.map((i) => [i.key, i])));
  expect(walk("COS-1", edges, "impact").keys).toEqual(["COS-2"]);
});

it("counts the baseline's health", () => {
  const inside = insideIssues(run, scope, "");
  const edges = baselineEdges(inside, byKey);
  const sets = [
    { keys: ["COS-150", "TLS-12"], projects: ["COS", "TLS"], outOfStep: true },
    { keys: ["RFT-91"], projects: ["RFT"], outOfStep: true },
  ] as never;
  expect(health(inside, edges, byKey, sets, NOW)).toEqual({
    inside: 5,
    finishedShare: 0.2,
    blockedFromOutside: 1,
    external: 2,
    outOfStep: 1,
    // Open ages 30, 20, 5, 10: the middle two are 10 and 20.
    medianAge: 15,
  });
  expect(health([], [], byKey, [], NOW).medianAge).toBeNull();
});

it("folds the issues with no link out, unless traced or their project is open", () => {
  const inside = insideIssues(run, scope, "");
  const edges = baselineEdges(inside, byKey);
  let f = fold(inside, edges, new Set(), new Set());
  expect(f.shown.map((i) => i.key)).toEqual(["COS-210", "COS-150"]);
  expect([...f.hidden]).toEqual([["COS", 2], ["COL", 1]]);
  f = fold(inside, edges, new Set(["COL-45"]), new Set(["COS"]));
  expect(f.shown.map((i) => i.key)).toEqual(["COS-210", "COS-188", "COL-45", "COS-150", "COS-231"]);
  expect(f.hidden.size).toBe(0);
});
