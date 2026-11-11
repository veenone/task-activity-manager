import { expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { query } from "../wailsjs/go/models";
import type { Edge } from "./linkData";
import { ProjectGraphView } from "./ProjectGraphView";

// jsdom has no canvas, so G6 is replaced by lists of what it was given.
vi.mock("./G6Canvas", () => ({
  default: ({
    nodes,
    edges,
    layout,
    onSelect,
  }: {
    nodes: { id: string; label: string; sub?: string }[];
    edges: { id: string; width?: number; label?: string; directed: boolean }[];
    layout?: { focusNode?: string };
    onSelect: (id: string) => void;
  }) => (
    <div>
      <p>{`Centred on ${layout?.focusNode}`}</p>
      <ul aria-label="Nodes">
        {nodes.map((n) => (
          <li key={n.id}>
            <button type="button" onClick={() => onSelect(n.id)}>
              {`${n.label} ${n.sub}`}
            </button>
          </li>
        ))}
      </ul>
      <ul aria-label="Edges">
        {edges.map((e) => (
          <li
            key={e.id}
          >{`${e.id} ${e.label}${e.directed ? "" : " undirected"}`}</li>
        ))}
      </ul>
    </div>
  ),
}));

const issue = (key: string) =>
  ({ key, project: key.split("-")[0] }) as query.Issue;
const issues = new Map(
  ["A-1", "A-2", "B-1", "C-1", "D-1"].map((k) => [k, issue(k)]),
);
// A clones B twice and relates to C; C blocks D.
const edges: Edge[] = [
  { from: "A-1", to: "B-1", type: "Cloners" },
  { from: "A-2", to: "B-1", type: "Cloners" },
  { from: "A-1", to: "C-1", type: "Relates" },
  { from: "C-1", to: "D-1", type: "Blocks" },
];
const types = [
  { name: "Cloners", outward: "clones", inward: "is cloned by" },
  { name: "Relates", outward: "relates to", inward: "relates to" },
  { name: "Blocks", outward: "blocks", inward: "is blocked by" },
];
const names: Record<string, string> = {
  A: "Alpha",
  B: "Bravo",
  C: "Charlie",
  D: "Delta",
};

function show() {
  const onIssues = vi.fn();
  const onGrid = vi.fn();
  render(
    <ProjectGraphView
      edges={edges}
      issues={issues}
      types={types}
      label={(p) => `${p} ${names[p]}`}
      name={(p) => names[p]}
      onIssues={onIssues}
      onGrid={onGrid}
    />,
  );
  return { user: userEvent.setup(), onIssues, onGrid };
}
const list = (name: string) =>
  within(screen.getByRole("list", { name }))
    .queryAllByRole("listitem")
    .map((l) => l.textContent);

it("centres on the busiest project, with line labels for the counts and arrows only where a link has a direction", async () => {
  show();
  expect(await screen.findByText("Centred on A")).toBeVisible();
  expect(list("Nodes")).toEqual([
    "A · 2 issues Alpha",
    "B · 1 issue Bravo",
    "C · 1 issue Charlie",
  ]);
  expect(list("Edges")).toEqual(["A>B:Cloners 2", "A>C:Relates 1 undirected"]);
  const details = screen.getByRole("complementary", { name: "Project A" });
  expect(details).toHaveTextContent("Links out3");
  expect(details).toHaveTextContent("BCloners 2 out");
});

it("centres on a project chosen by key or name, on Centre here, and walks back and forward", async () => {
  const { user } = show();
  await screen.findByText("Centred on A");
  await user.click(
    within(screen.getByRole("list", { name: "Nodes" })).getByRole("button", {
      name: /^C /,
    }),
  );
  // Choosing a box selects it; the centre stays.
  expect(screen.getByText("Centred on A")).toBeVisible();
  expect(
    screen.getByRole("complementary", { name: "Project C" }),
  ).toHaveTextContent("DBlocks 1 out");
  await user.click(screen.getByRole("button", { name: "Centre here" }));
  expect(screen.getByText("Centred on C")).toBeVisible();
  expect(list("Nodes")).toEqual([
    "C · 1 issue Charlie",
    "A · 2 issues Alpha",
    "D · 1 issue Delta",
  ]);
  await user.click(screen.getByRole("button", { name: "‹ Back" }));
  expect(screen.getByText("Centred on A")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Forward ›" }));
  expect(screen.getByText("Centred on C")).toBeVisible();
  const box = screen.getByLabelText("Centre on a project");
  expect(box).toHaveValue("C");
  await user.clear(box);
  await user.type(box, "B");
  expect(screen.getByText("Centred on B")).toBeVisible();
});

it("hides a link type and thin lines, and reaches a second ring", async () => {
  const { user } = show();
  await screen.findByText("Centred on A");
  await user.click(screen.getByRole("checkbox", { name: /Relates/ }));
  expect(list("Edges")).toEqual(["A>B:Cloners 2"]);
  await user.click(screen.getByRole("checkbox", { name: /Relates/ }));
  await user.click(screen.getByRole("checkbox", { name: "Two rings" }));
  expect(list("Nodes")).toContain("D · 1 issue Delta");
  const min = screen.getByRole("spinbutton", {
    name: "Hide lines with fewer links than",
  });
  await user.clear(min);
  await user.type(min, "2");
  expect(list("Edges")).toEqual(["A>B:Cloners 2"]);
});

it("opens the chosen project's issues, or its row of the grid", async () => {
  const { user, onIssues, onGrid } = show();
  await user.click(
    await screen.findByRole("button", { name: "Show its issues" }),
  );
  expect(onIssues).toHaveBeenCalledWith("A");
  await user.click(screen.getByRole("button", { name: "Open in Grid" }));
  expect(onGrid).toHaveBeenCalledWith(["A", "B", "C"]);
});

it("drops a chosen project a filter hides from the details", async () => {
  const { user } = show();
  await screen.findByText("Centred on A");
  await user.click(
    within(screen.getByRole("list", { name: "Nodes" })).getByRole("button", {
      name: /^C /,
    }),
  );
  // C is linked to A only by Relates; hiding it takes C off the canvas.
  await user.click(screen.getByRole("checkbox", { name: /Relates/ }));
  expect(
    screen.getByRole("complementary", { name: "Project A" }),
  ).toBeVisible();
});

it("offers no issues for a project seen only at the far end of a link", async () => {
  const onIssues = vi.fn();
  const only = new Map([["A-1", issue("A-1")]]);
  render(
    <ProjectGraphView
      edges={[{ from: "A-1", to: "Z-9", type: "Cloners" }]}
      issues={only}
      types={types}
      label={(p) => p}
      name={() => ""}
      onIssues={onIssues}
      onGrid={vi.fn()}
    />,
  );
  const user = userEvent.setup();
  await user.click(
    within(await screen.findByRole("list", { name: "Nodes" })).getByRole(
      "button",
      { name: /^Z / },
    ),
  );
  expect(
    screen.getByRole("button", { name: "Show its issues" }),
  ).toBeDisabled();
});
