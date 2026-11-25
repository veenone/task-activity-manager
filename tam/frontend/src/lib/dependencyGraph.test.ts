import { describe, expect, it } from "vitest";
import type { DependencyGraph, DependencyLink, Issue } from "../api";
import { NO_DEP_FILTER, canvasData, chain, chainExport, closesCycle, defaultTypes, edgeId, visibleGraph, wholeExport } from "./dependencyGraph";

const issue = (key: string, over: Partial<Issue> = {}): Issue =>
  ({ key, summary: `Summary of ${key}`, type: "story", status: "To Do", statusCategory: "new", assignee: "", sprintId: "", sprintName: "", ...over }) as Issue;
const link = (outward: string, inward: string, over: Partial<DependencyLink> = {}): DependencyLink => ({
  id: `${outward}>${inward}`, type: "Blocks", outward, inward, owner: outward, direction: "outward", pending: false, pendingId: 0, removal: false, ...over,
});

// A blocks B blocks C, D relates to C, and C is blocked by OPS-9 elsewhere.
const graph: DependencyGraph = {
  issues: [
    issue("A", { sprintId: "12", assignee: "R. Anand" }),
    issue("B", { sprintId: "12", status: "In Progress", statusCategory: "indeterminate" }),
    issue("C", { sprintId: "13" }),
    issue("D"),
  ],
  external: [{ key: "OPS-9", summary: "Firewall rule", issueType: "Task" }],
  links: [link("A", "B"), link("B", "C"), link("D", "C", { type: "Relates" }), link("OPS-9", "C")],
};

describe("the dependency graph's filters (#612)", () => {
  it("starts on Blocks when the instance has it, and on every type when not", () => {
    expect(defaultTypes(["Relates", "Blocks", "Cloners"])).toEqual(["Blocks"]);
    expect(defaultTypes(["Relates", "Cloners"])).toEqual(["Relates", "Cloners"]);
  });

  it("keeps the links of the chosen types and the issues they join", () => {
    const v = visibleGraph(graph, { ...NO_DEP_FILTER, types: ["Blocks"] });
    expect(v.links.map(edgeId)).toEqual(["A>B", "B>C", "OPS-9>C"]);
    expect([...v.keys].sort()).toEqual(["A", "B", "C", "OPS-9"]);
  });

  it("keeps a link when one of its ends matches the sprint, status or assignee", () => {
    expect(visibleGraph(graph, { ...NO_DEP_FILTER, types: ["Blocks"], sprintId: "13" }).links.map(edgeId)).toEqual(["B>C", "OPS-9>C"]);
    expect(visibleGraph(graph, { ...NO_DEP_FILTER, types: ["Blocks"], status: "In Progress" }).links.map(edgeId)).toEqual(["A>B", "B>C"]);
    expect(visibleGraph(graph, { ...NO_DEP_FILTER, types: ["Blocks"], assignee: "R. Anand" }).links.map(edgeId)).toEqual(["A>B"]);
  });
});

describe("a picked issue's chain (#612)", () => {
  it("finds what it waits on upstream and what waits on it downstream", () => {
    const c = chain(graph.links.filter((l) => l.type === "Blocks"), "B");
    expect([...c.up].sort()).toEqual(["A"]);
    expect([...c.down].sort()).toEqual(["C"]);
    expect([...c.edges].sort()).toEqual(["A>B", "B>C"]);
    const end = chain(graph.links.filter((l) => l.type === "Blocks"), "C");
    expect([...end.up].sort()).toEqual(["A", "B", "OPS-9"]);
  });

  it("lights the chain and fades the rest on the canvas", () => {
    const v = visibleGraph(graph, { ...NO_DEP_FILTER, types: ["Blocks"] });
    const { nodes, edges } = canvasData(graph, v, "B", "");
    expect(nodes.find((n) => n.id === "A")).toMatchObject({ lit: true, faded: false });
    expect(nodes.find((n) => n.id === "OPS-9")).toMatchObject({ faded: true, dashed: true });
    expect(edges.find((e) => e.id === "OPS-9>C")).toMatchObject({ faded: true });
    expect(edges.find((e) => e.id === "A>B")).toMatchObject({ lit: true, directed: true });
  });
});

describe("pending links on the canvas (#612)", () => {
  it("draws a pending add and a pending removal dashed, and says which", () => {
    const g: DependencyGraph = {
      ...graph,
      links: [link("A", "B", { pending: true, pendingId: 4, id: "" }), link("B", "C", { pending: true, removal: true, pendingId: 5 })],
    };
    const v = visibleGraph(g, { ...NO_DEP_FILTER, types: ["Blocks"] });
    const { edges } = canvasData(g, v, null, "");
    expect(edges.find((e) => e.id === "p4")).toMatchObject({ dashed: true, label: "pending" });
    expect(edges.find((e) => e.id === "B>C")).toMatchObject({ dashed: true, label: "removal pending" });
  });
});

describe("a blocks link that closes a cycle (#612)", () => {
  it("is caught when the new blocked issue already blocks the blocker", () => {
    expect(closesCycle(graph.links, "C", "A")).toBe(true);
    expect(closesCycle(graph.links, "A", "C")).toBe(false);
    // Relates links are not part of a chain, and a removal on its way out
    // no longer holds one together.
    expect(closesCycle(graph.links, "C", "D")).toBe(false);
    const leaving = graph.links.map((l) => (l.outward === "B" ? { ...l, pending: true, removal: true } : l));
    expect(closesCycle(leaving, "C", "A")).toBe(false);
  });
});

describe("what an export sends (#612)", () => {
  it("sends a picked issue's chain alone, with the types it was followed through", () => {
    const v = visibleGraph(graph, { ...NO_DEP_FILTER, types: ["Blocks"] });
    const out = chainExport(graph, v, "B", ["Blocks"]);
    expect(out.scope.focus).toBe("B");
    expect([...out.scope.keys].sort()).toEqual(["A", "B", "C"]);
    expect(out.scope.types).toEqual(["Blocks"]);
    expect(out.edges.map((e) => e.id).sort()).toEqual(["A>B", "B>C"]);
    // The picture is of the chain alone, so nothing in it is faded.
    expect(out.nodes.some((n) => n.faded)).toBe(false);
  });

  it("follows the chain both ways and leaves out what it does not reach", () => {
    const v = visibleGraph(graph, { ...NO_DEP_FILTER, types: ["Blocks"] });
    // C waits on B, A through B, and OPS-9; D only relates to it.
    expect(chainExport(graph, v, "C", ["Blocks"]).scope.keys.sort()).toEqual(["A", "B", "C", "OPS-9"]);
    const relates = visibleGraph(graph, { ...NO_DEP_FILTER, types: ["Relates"] });
    expect(chainExport(graph, relates, "D", ["Relates"]).scope.keys.sort()).toEqual(["C", "D"]);
  });

  it("sends the whole profile with no focus, drawing every link of every type", () => {
    const out = wholeExport(graph);
    expect(out.scope).toEqual({ focus: "", keys: [], types: [] });
    expect(out.edges.map((e) => e.id)).toEqual(["A>B", "B>C", "D>C", "OPS-9>C"]);
    expect(out.nodes.map((n) => n.id)).toEqual(["A", "B", "C", "D", "OPS-9"]);
  });
});
