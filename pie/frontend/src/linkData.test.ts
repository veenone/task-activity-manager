import { expect, it } from "vitest";
import type { query } from "../wailsjs/go/models";
import { byCell, type Edge } from "./linkData";

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
