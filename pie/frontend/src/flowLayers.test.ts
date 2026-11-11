import { expect, it } from "vitest";
import { type FlowPath, layered } from "./flowLayers";

// Three issues from two baselines; X-1 and X-2 share every box.
const paths: FlowPath[] = [
  { key: "X-1", steps: ["b:X", "p:X", "s:missing"] },
  { key: "X-2", steps: ["b:X", "p:X", "s:missing"] },
  { key: "Y-1", steps: ["b:Y", "p:Y", "s:open"] },
];

const lines = (r: ReturnType<typeof layered>) =>
  r.lines.map((l) => `${l.source}>${l.target} ${l.keys.join(",")}`);

it("groups the paths into one line per pair of boxes, counting the keys", () => {
  const r = layered(paths, { hidden: new Set(), min: 1 });
  expect(lines(r)).toEqual([
    "b:X>p:X X-1,X-2",
    "p:X>s:missing X-1,X-2",
    "b:Y>p:Y Y-1",
    "p:Y>s:open Y-1",
  ]);
  expect(r.boxes.map((b) => `${b.id} ${b.keys.length}`)).toEqual([
    "b:X 2",
    "b:Y 1",
    "p:X 2",
    "p:Y 1",
    "s:missing 2",
    "s:open 1",
  ]);
});

it("joins the layers either side of a hidden one", () => {
  const r = layered(paths, { hidden: new Set([1]), min: 1 });
  expect(lines(r)).toEqual(["b:X>s:missing X-1,X-2", "b:Y>s:open Y-1"]);
});

it("drops the lines under the minimum count, and the boxes left without a line", () => {
  const r = layered(paths, { hidden: new Set(), min: 2 });
  expect(lines(r)).toEqual(["b:X>p:X X-1,X-2", "p:X>s:missing X-1,X-2"]);
  expect(r.boxes.map((b) => b.id)).toEqual(["b:X", "p:X", "s:missing"]);
});

it("splits a pair into a line per type, and counts a key once", () => {
  const typed: FlowPath[] = [
    { key: "A-1>B-1", steps: ["f:A", "o:B"], type: "Cloners" },
    { key: "A-2>B-1", steps: ["f:A", "o:B"], type: "Blocks" },
    { key: "A-2>B-1", steps: ["f:A", "o:B"], type: "Blocks" },
  ];
  const r = layered(typed, { hidden: new Set(), min: 1 });
  expect(r.lines.map((l) => `${l.type} ${l.keys.length}`)).toEqual([
    "Cloners 1",
    "Blocks 1",
  ]);
});
