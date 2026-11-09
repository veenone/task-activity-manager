import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectMap } from "./ProjectMap";
import { filterSets, sankey } from "./mapData";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({ Baselines: vi.fn() }));

const issue = (key: string, status: string, statusCategory: string) => ({
  key,
  project: key.split("-")[0],
  status,
  statusCategory,
});

// A-1 is fixed and its clone B-1 is not; A-2 and C-2 are both finished.
const result = {
  issues: [issue("A-1", "Closed", "done"), issue("B-1", "Open", "new"), issue("A-2", "Closed", "done"), issue("C-2", "Rejected", "done")],
  sets: [
    { keys: ["A-1", "B-1"], projects: ["A", "B"], outOfStep: true },
    { keys: ["A-2", "C-2"], projects: ["A", "C"], outOfStep: false },
  ],
};

function show() {
  const onOpen = vi.fn();
  render(<ProjectMap profileId="p1" result={result as never} onOpen={onOpen} />);
  return { user: userEvent.setup(), onOpen };
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.Baselines).mockResolvedValue([{ key: "A", descendants: ["B"] }]);
});

it("counts what the map shows above the views", async () => {
  show();
  const counts = screen.getByRole("list", { name: "Map counts" });
  expect(await within(counts).findByText("baselines, and 1 project under none")).toBeVisible();
  expect(
    within(counts)
      .getAllByRole("listitem")
      .map((l) => l.textContent),
  ).toEqual(["1 of 2 linked sets out of step", "1 issues still to fix", "4 issues across 3 projects", "1 baselines, and 1 project under none"]);
});

it("flows issues from baseline to project to fix state, and lists the sets out of step", async () => {
  const { user, onOpen } = show();
  const flow = screen.getByRole("img", {
    name: /Issues by baseline, project and fix state/,
  });
  expect(await within(flow).findByText("Baseline A")).toBeInTheDocument();
  expect(within(flow).getByText("B to Fix missing: 1 (B-1)")).toBeInTheDocument();
  expect(within(flow).getByText("C to Fixed everywhere: 1 (C-2)")).toBeInTheDocument();
  expect(within(flow).getByText("Fixed, others behind 1")).toBeInTheDocument();

  const behind = screen.getByRole("list", { name: "Sets out of step" });
  await user.click(within(behind).getByRole("button", { name: "B-1 Open" }));
  expect(onOpen).toHaveBeenCalledWith("B-1");
});

it("branches baselines into projects and issues, and marks a chosen set across the tree", async () => {
  const { user, onOpen } = show();
  await user.click(screen.getByRole("button", { name: "Tree" }));
  const tree = screen.getByRole("list", { name: "Project tree" });
  expect(await within(tree).findByText("Baseline A")).toBeVisible();
  expect(within(tree).getByText("0 of 1 fixed")).toBeVisible();

  await user.click(screen.getByRole("button", { name: "A-1 Out of step" }));
  const lit = within(tree)
    .getAllByRole("listitem")
    .filter((li) => li.getAttribute("aria-current") === "true");
  expect(lit.map((li) => li.textContent)).toEqual(["A-1 Closed", "B-1 Open"]);

  await user.click(within(tree).getByRole("button", { name: "C-2 Rejected" }));
  expect(onOpen).toHaveBeenCalledWith("C-2");
});

it("applies Out of step only to the flow", async () => {
  const { user } = show();
  await user.click(screen.getByLabelText("Out of step only"));
  const flow = screen.getByRole("img", { name: /Issues by baseline/ });
  expect(within(flow).queryByText(/Fixed everywhere/)).not.toBeInTheDocument();
});

it("stacks a node's bands without overlap and sizes nodes by their issues", () => {
  const { nodes, bands } = sankey(
    [
      [{ id: "x", label: "x" }],
      [
        { id: "p", label: "p" },
        { id: "q", label: "q" },
      ],
    ],
    [
      { from: "x", to: "p", keys: ["1", "2", "3"] },
      { from: "x", to: "q", keys: ["4"] },
    ],
    { width: 200, height: 110, gap: 10 },
  );
  const [x, p, q] = nodes;
  expect([x.value, p.value, q.value]).toEqual([4, 3, 1]);
  expect([x.y, x.h, p.y, p.h, q.y, q.h]).toEqual([0, 100, 0, 75, 85, 25]);
  // The band to q leaves x where the band to p ends.
  expect(bands[1].path.startsWith(`M${x.x + 12},75 `)).toBe(true);
});

it("grows the chart rather than squash a column of many nodes", () => {
  const column = Array.from({ length: 30 }, (_, i) => ({ id: `n${i}`, label: `n${i}` }));
  const { nodes, height } = sankey(
    [[{ id: "x", label: "x" }], column],
    column.map((c) => ({ from: "x", to: c.id, keys: [c.id] })),
    { width: 200, height: 100, gap: 10 },
  );
  expect(nodes.every((n) => n.h > 0)).toBe(true);
  expect(nodes.at(-1)!.y + nodes.at(-1)!.h).toBeLessThanOrEqual(height);
});

const groups = [
  { baseline: "A", projects: ["A", "B"] },
  { baseline: null, projects: ["C"] },
];

it("keeps the sets that pass every filter", () => {
  const sets = result.sets as never[];
  const keys = (f: Parameters<typeof filterSets>[2]) => filterSets(sets, groups, f).map((s: { keys: string[] }) => s.keys[0]);
  expect(keys({ baseline: "", project: "", key: "" })).toEqual(["A-1", "A-2"]);
  expect(keys({ baseline: "A", project: "", key: "" })).toEqual(["A-1", "A-2"]);
  expect(keys({ baseline: "-", project: "", key: "" })).toEqual(["A-2"]);
  expect(keys({ baseline: "", project: "B", key: "" })).toEqual(["A-1"]);
  expect(keys({ baseline: "", project: "", key: "c-2" })).toEqual(["A-2"]);
  expect(keys({ baseline: "A", project: "C", key: "" })).toEqual(["A-2"]);
  expect(keys({ baseline: "-", project: "B", key: "" })).toEqual([]);
});

it("filters every view by project and key, says how many show, and clears", async () => {
  const { user } = show();
  await screen.findByRole("option", { name: "Baseline A" });
  await user.type(screen.getByLabelText("Project"), "C");
  expect(screen.getByText("Showing 1 of 2 linked sets")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Table" }));
  const table = screen.getByRole("table", { name: "Project map" });
  expect(within(table).getAllByRole("rowheader").map((h) => h.textContent)).toEqual(["A-2"]);

  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  await user.type(screen.getByLabelText("Key"), "b-1");
  expect(within(table).getAllByRole("rowheader").map((h) => h.textContent)).toEqual(["A-1"]);
  const counts = screen.getByRole("list", { name: "Map counts" });
  expect(within(counts).getByText("1 of 2")).toBeVisible();
});

it("offers only the chosen baseline's projects", async () => {
  const { user } = show();
  await user.selectOptions(await screen.findByLabelText("Baseline"), "A");
  expect([...document.querySelectorAll(`#${screen.getByLabelText("Project").getAttribute("list")} option`)].map((o) => o.textContent)).toEqual(["A", "B"]);
});

it("drops the Showing line once every filter is back to its default", async () => {
  const { user } = show();
  await screen.findByRole("option", { name: "Baseline A" });
  await user.type(screen.getByLabelText("Key"), "x");
  expect(screen.getByText("Showing 0 of 2 linked sets")).toBeVisible();
  await user.clear(screen.getByLabelText("Key"));
  expect(screen.queryByText(/^Showing/)).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Clear filters" })).not.toBeInTheDocument();
});

it("forgets a chosen project the next run no longer reaches, instead of showing nothing", async () => {
  const user = userEvent.setup();
  const view = render(<ProjectMap profileId="p1" result={result as never} onOpen={vi.fn()} />);
  await screen.findByRole("option", { name: "Baseline A" });
  await user.type(screen.getByLabelText("Project"), "C");
  const next = { ...result, sets: [result.sets[0]] };
  view.rerender(<ProjectMap profileId="p1" result={next as never} onOpen={vi.fn()} />);
  expect(screen.getByText("No project matches")).toBeVisible();
  expect(screen.queryByText("No linked sets to show.")).not.toBeInTheDocument();
});
