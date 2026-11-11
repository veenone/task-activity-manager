import { expect, it } from "vitest";
import type { query } from "../wailsjs/go/models";
import { add, back, edgesOf, emptyGraph, forward, looksLikeKey, MAX_NODES, PARENT, search, typeCounts, unloaded, visible, visit } from "./graphModel";

const issue = (key: string, more: Partial<query.Issue> = {}) =>
  ({ key, project: key.split("-")[0], summary: `About ${key}`, statusCategory: "new", links: [], subtasks: [], parent: "", ...more }) as unknown as query.Issue;
const link = (key: string, type: string, outward: boolean, symmetric = false, words = "") => ({ key, type, outward, symmetric, words });
const on = (...issues: query.Issue[]) => issues.map((i) => ({ issue: i, inRun: true, fetchedAt: "" }));

it("gives a link the same edge from both of its issues, directed from the outward end", () => {
  const a = issue("A-1", { links: [link("B-1", "Blocks", true, false, "blocks")] });
  const b = issue("B-1", { links: [link("A-1", "Blocks", false, false, "is blocked by")] });
  expect(edgesOf(a)[0].id).toBe(edgesOf(b)[0].id);
  const { graph } = add(emptyGraph(), on(b, a));
  expect([...graph.edges.values()]).toEqual([
    { id: "A-1>B-1:Blocks", source: "A-1", target: "B-1", type: "Blocks", words: "blocks", directed: true },
  ]);
});

it("draws a symmetric type once and without a direction", () => {
  const a = issue("B-1", { links: [link("A-1", "Relates", true, true, "relates to")] });
  const b = issue("A-1", { links: [link("B-1", "Relates", true, true, "relates to")] });
  const { graph } = add(emptyGraph(), on(a, b));
  expect([...graph.edges.values()].map((e) => [e.id, e.directed])).toEqual([["A-1~B-1:Relates", false]]);
});

it("adds parent edges from either side once", () => {
  const p = issue("A-1", { subtasks: ["A-2"] });
  const c = issue("A-2", { parent: "A-1" });
  const { graph } = add(emptyGraph(), on(p, c));
  expect([...graph.edges.values()].map((e) => e.id)).toEqual([`A-1>A-2:${PARENT}`]);
});

it("never adds an issue twice, keeps it in the run once it was, and counts unloaded neighbours", () => {
  const a = issue("A-1", { links: [link("B-1", "Blocks", true), link("C-1", "Blocks", true)] });
  let { graph } = add(emptyGraph(), on(a));
  ({ graph } = add(graph, [{ issue: a, inRun: false, fetchedAt: "2026-11-10T09:00:00Z" }]));
  expect(graph.nodes.size).toBe(1);
  expect(graph.nodes.get("A-1")!.inRun).toBe(true);
  expect(unloaded(graph, "A-1")).toBe(2);
  ({ graph } = add(graph, on(issue("B-1"))));
  expect(unloaded(graph, "A-1")).toBe(1);
});

it("stops at the canvas limit and says how many did not fit", () => {
  const many = Array.from({ length: MAX_NODES + 5 }, (_, i) => issue(`A-${i}`));
  const { graph, left } = add(emptyGraph(), on(...many));
  expect(graph.nodes.size).toBe(MAX_NODES);
  expect(left).toBe(5);
});

it("filters by type, by run and by finished, but keeps the issue in focus", () => {
  const a = issue("A-1", { links: [link("B-1", "Blocks", true), link("C-1", "Cloners", true)] });
  const { graph } = add(emptyGraph(), [
    { issue: a, inRun: true, fetchedAt: "" },
    { issue: issue("B-1", { statusCategory: "done" }), inRun: true, fetchedAt: "" },
    { issue: issue("C-1"), inRun: false, fetchedAt: "" },
  ]);
  const all = { hiddenTypes: new Set<string>(), outside: true, finished: true };
  expect(visible(graph, all, null).edges).toHaveLength(2);
  expect(visible(graph, { ...all, hiddenTypes: new Set(["Cloners"]) }, null).edges.map((e) => e.type)).toEqual(["Blocks"]);
  expect(visible(graph, { ...all, outside: false }, null).nodes.map((n) => n.key)).toEqual(["A-1", "B-1"]);
  expect(visible(graph, { ...all, finished: false }, "B-1").nodes.map((n) => n.key)).toEqual(["A-1", "B-1", "C-1"]);
  expect(typeCounts(graph)).toEqual([["Blocks", 1], ["Cloners", 1]]);
});

it("walks back and forward through the issues visited, dropping the forward path on a new visit", () => {
  let h = visit(visit(visit({ keys: [], at: -1 }, "A-1"), "B-1"), "C-1");
  h = back(back(h));
  expect(h.keys[h.at]).toBe("A-1");
  expect(back(h).at).toBe(0);
  h = visit(forward(h), "D-1");
  expect(h.keys).toEqual(["A-1", "B-1", "D-1"]);
  expect(forward(h).at).toBe(2);
});

it("finds loaded issues by key or summary, and tells a full key", () => {
  const { graph } = add(emptyGraph(), on(issue("A-1"), issue("B-12", { summary: "Reader resets" })));
  expect(search(graph, "reader").map((n) => n.key)).toEqual(["B-12"]);
  expect(looksLikeKey(" rft-91 ")).toBe(true);
  expect(looksLikeKey("reader")).toBe(false);
});
