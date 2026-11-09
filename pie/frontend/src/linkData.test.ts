import { expect, it } from "vitest";
import type { query } from "../wailsjs/go/models";
import { busiest, byCell, type Edge, flowLinks, neighbours } from "./linkData";

const issue = (key: string, project: string) => ({ key, project }) as query.Issue;

it("groups each edge under its pair of projects once, keeping the edge order", () => {
  const issues = new Map([issue("A-1", "A"), issue("A-2", "A"), issue("B-1", "B")].map((i) => [i.key, i]));
  const edges: Edge[] = [
    { from: "A-1", to: "B-1", type: "Blocks" },
    { from: "B-1", to: "A-2", type: "Relates" },
    { from: "A-2", to: "B-1", type: "Relates" },
    { from: "A-1", to: "C-9", type: "Blocks" },
  ];
  const cells = byCell(edges, issues);
  expect(cells.get("A>B")?.map((e) => e.from)).toEqual(["A-1", "A-2"]);
  expect(cells.get("B>A")?.map((e) => e.to)).toEqual(["A-2"]);
  expect(cells.get("A>C")?.map((e) => e.to)).toEqual(["C-9"]);
  expect(cells.get("B>B")).toBeUndefined();
});

const projects = new Map([issue("A-1", "A"), issue("A-2", "A"), issue("B-1", "B"), issue("C-1", "C")].map((i) => [i.key, i]));
const web: Edge[] = [
  { from: "A-1", to: "B-1", type: "Cloners" },
  { from: "A-2", to: "B-1", type: "Cloners" },
  { from: "B-1", to: "A-2", type: "Relates" },
  { from: "C-1", to: "A-1", type: "Blocks" },
];

it("names the project with the most links as the busiest", () => {
  expect(busiest(web, projects)).toBe("A");
  expect(busiest([], projects)).toBe("");
});

it("lists a project's neighbours with each link type's count both ways, most links first", () => {
  expect(neighbours("A", web, projects)).toEqual([
    {
      project: "B",
      total: 3,
      types: [
        { type: "Cloners", out: 2, in: 0 },
        { type: "Relates", out: 0, in: 1 },
      ],
    },
    { project: "C", total: 1, types: [{ type: "Blocks", out: 0, in: 1 }] },
  ]);
});

it("runs each link from its project through its type to the other project", () => {
  const { columns, links } = flowLinks(web, projects);
  expect(columns.map((c) => c.map((n) => n.id))).toEqual([
    ["f:A", "f:B", "f:C"],
    ["t:Blocks", "t:Cloners", "t:Relates"],
    ["o:A", "o:B"],
  ]);
  expect(links.find((l) => l.from === "f:A" && l.to === "t:Cloners")?.keys).toEqual(["A-1>B-1", "A-2>B-1"]);
  expect(links.find((l) => l.from === "t:Cloners" && l.to === "o:B")?.keys).toHaveLength(2);
});
