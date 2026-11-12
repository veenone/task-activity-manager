import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectMap } from "./ProjectMap";
import { filterSets } from "./mapData";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({ Baselines: vi.fn() }));
// jsdom has no canvas, so G6 is replaced by lists of what it was given.
vi.mock("./G6Canvas", () => ({
  default: ({
    nodes,
    edges,
    label,
    onSelect,
  }: {
    nodes: { id: string; label: string; sub?: string }[];
    edges: { id: string; source: string; target: string; label?: string; faded?: boolean }[];
    label: string;
    onSelect: (id: string) => void;
  }) => (
    <div>
      <ul aria-label={label}>
        {nodes.map((n) => (
          <li key={n.id}>
            <button type="button" onClick={() => onSelect(n.id)}>{`${n.label} · ${n.sub}`}</button>
          </li>
        ))}
      </ul>
      <ul aria-label="Lines">
        {edges.map((e) => (
          <li key={e.id}>{`${e.source}>${e.target} ${e.label}${e.faded ? " faded" : ""}`}</li>
        ))}
      </ul>
    </div>
  ),
}));

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

const lines = async () =>
  within(await screen.findByRole("list", { name: "Lines" }))
    .getAllByRole("listitem")
    .map((l) => l.textContent);

it("lays issues out from baseline to project to fix state, and lists the issues missing a fix", async () => {
  const { user, onOpen } = show();
  const flow = await screen.findByRole("list", { name: /Issues by baseline, project and fix state/ });
  expect(await within(flow).findByRole("button", { name: "Baseline A · 3 issues" })).toBeVisible();
  expect(await lines()).toEqual([
    "b:A>p:A 2",
    "p:A>s:behind 1",
    "b:A>p:B 1",
    "p:B>s:missing 1",
    "p:A>s:everywhere 1",
    "b:>p:C 1",
    "p:C>s:everywhere 1",
  ]);
  const panel = screen.getByRole("complementary", { name: "Everything shown" });
  await user.click(within(panel).getByRole("button", { name: "B-1 Open" }));
  expect(onOpen).toHaveBeenCalledWith("B-1");
});

it("lights the paths through a chosen box and gives its fix states", async () => {
  const { user } = show();
  const flow = await screen.findByRole("list", { name: /Issues by baseline/ });
  await user.click(await within(flow).findByRole("button", { name: "C · 1 issue" }));
  const panel = screen.getByRole("complementary", { name: "Box C" });
  const items = (name: string) =>
    within(within(panel).getByRole("list", { name }))
      .getAllByRole("listitem")
      .map((l) => l.textContent);
  expect(items("Fix states")).toEqual(["Fixed everywhere 1"]);
  expect(items("Comes from")).toEqual(["No baseline 1"]);
  expect(within(panel).queryByRole("button", { name: "B-1 Open" })).not.toBeInTheDocument();
  expect((await lines()).filter((l) => !l.endsWith("faded"))).toEqual(["b:>p:C 1", "p:C>s:everywhere 1"]);
});

it("joins the layers either side of a hidden one, and drops lines under the minimum", async () => {
  const { user } = show();
  await screen.findByRole("list", { name: /Issues by baseline/ });
  await user.click(screen.getByRole("checkbox", { name: "Project layer" }));
  expect(await lines()).toContain("b:A>s:missing 1");
  await user.click(screen.getByRole("checkbox", { name: "Project layer" }));
  await user.clear(screen.getByLabelText("Hide lines with fewer issues than"));
  await user.type(screen.getByLabelText("Hide lines with fewer issues than"), "2");
  expect(await lines()).toEqual(["b:A>p:A 2"]);
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
  await screen.findByRole("list", { name: /Issues by baseline/ });
  await user.click(screen.getByLabelText("Out of step only"));
  expect((await lines()).some((l) => l.includes("everywhere"))).toBe(false);
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
  await user.type(screen.getByRole("combobox", { name: "Project" }), "C");
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
  await user.selectOptions(await screen.findByRole("combobox", { name: "Baseline" }), "A");
  fireEvent.focus(screen.getByRole("combobox", { name: "Project" }));
  expect(within(screen.getByRole("listbox")).getAllByRole("option").map((o) => o.textContent)).toEqual(["A", "B"]);
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
  await user.type(screen.getByRole("combobox", { name: "Project" }), "C");
  const next = { ...result, sets: [result.sets[0]] };
  view.rerender(<ProjectMap profileId="p1" result={next as never} onOpen={vi.fn()} />);
  expect(screen.getByRole("combobox", { name: "Project" })).toHaveValue("");
  expect(screen.queryByText("No linked sets to show.")).not.toBeInTheDocument();
});

it("empties project text that named nothing when filters are cleared", async () => {
  const { user } = show();
  await screen.findByRole("option", { name: "Baseline A" });
  await user.type(screen.getByRole("combobox", { name: "Project" }), "zzz");
  await user.click(screen.getByLabelText("Out of step only"));
  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(screen.getByRole("combobox", { name: "Project" })).toHaveValue("");
  expect(screen.queryByText("No project matches")).not.toBeInTheDocument();
});

it("groups each project's issues under their status when asked, and back", async () => {
  const { user } = show();
  await user.click(screen.getByRole("button", { name: "Tree" }));
  const tree = screen.getByRole("list", { name: "Project tree" });
  await within(tree).findByText("Baseline A");
  expect(within(tree).getByRole("button", { name: "A-2 Closed" })).toBeVisible();

  await user.click(screen.getByRole("checkbox", { name: "Group by status" }));
  // A's two Closed issues fold under one Closed node with their count.
  const closed = within(tree).getAllByText("Closed", { selector: "summary > span" });
  expect(closed.map((s) => s.closest("summary")!.textContent)).toEqual(["Closed2"]);
  const group = closed[0].closest("details")!;
  expect(group).not.toHaveAttribute("open");
  await user.click(within(group).getByText("Closed", { selector: "summary > span" }));
  expect(within(group).getByRole("button", { name: "A-1 Closed" })).toBeVisible();
  expect(within(group).getByRole("button", { name: "A-2 Closed" })).toBeVisible();

  await user.click(screen.getByRole("checkbox", { name: "Group by status" }));
  expect(within(tree).queryByText("Closed", { selector: "summary > span" })).not.toBeInTheDocument();
});
