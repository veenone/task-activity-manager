import { expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinkMap } from "./LinkMap";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({ RefreshLinks: vi.fn() }));
// jsdom has no canvas, so G6 is replaced by a list of what it was given.
vi.mock("./G6Canvas", () => ({
  default: ({
    nodes,
    edges,
    label,
    onSelect,
  }: {
    nodes: { id: string }[];
    edges: { id: string; source: string; target: string; label?: string }[];
    label: string;
    onSelect: (id: string) => void;
  }) => (
    <>
      <ul aria-label={label}>
        {nodes.map((n) => (
          <li key={n.id}>
            <button type="button" onClick={() => onSelect(n.id)}>
              {n.id}
            </button>
          </li>
        ))}
      </ul>
      <ul aria-label="Lines">
        {edges.map((e) => (
          <li key={e.id}>{`${e.source}>${e.target} ${e.label}`}</li>
        ))}
      </ul>
    </>
  ),
}));

const link = (key: string, type: string, words: string, outward: boolean) => ({ key, type, words, outward });
const issue = (key: string, status: string, links: object[]) => ({
  key,
  project: key.split("-")[0],
  status,
  summary: `Summary of ${key}`,
  links,
});

// A-1 clones B-1 and relates to C-1; A-2 blocks A-3 inside A; B-2 clones Z-9,
// which the run never fetched.
const result = {
  issues: [
    issue("A-1", "Closed", [link("B-1", "Cloners", "clones", true), link("C-1", "Relates", "relates to", true)]),
    issue("B-1", "Open", [link("A-1", "Cloners", "is cloned by", false)]),
    issue("C-1", "Open", [link("A-1", "Relates", "relates to", false)]),
    issue("A-2", "Open", [link("A-3", "Blocks", "blocks", true)]),
    issue("A-3", "Open", [link("A-2", "Blocks", "is blocked by", false)]),
    issue("B-2", "Open", [link("Z-9", "Cloners", "clones", true)]),
  ],
  sets: [],
};

function show(res: object = result) {
  const onOpen = vi.fn();
  const onResult = vi.fn();
  const view = render(
    <LinkMap result={res as never} onOpen={onOpen} refresh={{ profileId: "p1", queryId: 7, runAt: "2026-11-06T10:00:00Z", onResult }} />,
  );
  return { user: userEvent.setup(), onOpen, onResult, view };
}

const cell = (from: string, to: string) => {
  const grid = screen.getByRole("table", { name: "Projects connected by links" });
  const row = within(grid).getByRole("rowheader", { name: from }).closest("tr")!;
  const cols = within(grid)
    .getAllByRole("columnheader")
    .map((h) => h.textContent);
  return within(row).getAllByRole("cell")[cols.indexOf(to) - 1];
};

it("counts links by type from each row's project to each column's project", () => {
  show();
  expect(cell("A", "B")).toHaveTextContent("Cloners 1");
  expect(cell("A", "C")).toHaveTextContent("Relates 1");
  expect(cell("B", "A")).toHaveTextContent("-");
  // Inside one project and to issues outside the run are hidden at first.
  expect(cell("A", "A")).toHaveTextContent("-");
  expect(screen.queryByRole("columnheader", { name: "Z" })).not.toBeInTheDocument();
});

it("lists each link type with its wording both ways, and hides an unticked type", async () => {
  const { user } = show();
  const rail = screen.getByRole("group", { name: "Link types" });
  expect(within(rail).getByText("clones")).toBeVisible();
  expect(within(rail).getByText("is cloned by")).toBeVisible();
  expect(within(rail).getByText("relates to (both ways)")).toBeVisible();

  await user.click(within(rail).getByRole("checkbox", { name: "Cloners 1" }));
  expect(cell("A", "B")).toHaveTextContent("-");
  await user.click(within(rail).getByRole("button", { name: "None" }));
  expect(cell("A", "C")).toHaveTextContent("-");
  await user.click(within(rail).getByRole("button", { name: "All" }));
  expect(cell("A", "B")).toHaveTextContent("Cloners 1");
});

it("adds links inside a project and links to issues outside the run when asked", async () => {
  const { user } = show();
  await user.click(screen.getByLabelText("Show links inside one project"));
  expect(cell("A", "A")).toHaveTextContent("Blocks 1");
  await user.click(screen.getByLabelText("Only links where both ends are in the run"));
  expect(cell("B", "Z")).toHaveTextContent("Cloners 1");
});

it("lists a cell's links, switches to the reverse direction, and opens a key in Trace", async () => {
  const { user, onOpen } = show();
  await user.click(within(cell("A", "B")).getByRole("button", { name: "A to B: 1 link" }));
  const links = screen.getByRole("table", { name: "Links from A to B" });
  const row = within(links).getAllByRole("row")[1];
  expect(
    within(row)
      .getAllByRole("cell")
      .map((c) => c.textContent),
  ).toEqual(["A-1", "Closed", "clones", "B-1", "Open", "Summary of A-1"]);

  await user.click(within(row).getByRole("button", { name: "B-1" }));
  expect(onOpen).toHaveBeenCalledWith("B-1");
  await user.click(screen.getByRole("button", { name: "B to A (0)" }));
  expect(screen.getByRole("table", { name: "Links from B to A" })).toBeVisible();
});

it("asks for a new run when the stored one has no link direction", () => {
  show({ ...result, issues: result.issues.map((i) => ({ ...i, links: i.links.map((l) => ({ ...l, outward: false })) })) });
  expect(screen.getByRole("alert")).toHaveTextContent("Run the query again");
});

it("does not take a run whose links all point to unfetched outward ends for an old one", () => {
  show({ issues: [issue("B-1", "Open", [link("A-1", "Cloners", "is cloned by", false)])], sets: [] });
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("ends each row with its total", () => {
  show();
  const grid = screen.getByRole("table", { name: "Projects connected by links" });
  expect(within(grid).getAllByRole("columnheader").at(-1)).toHaveTextContent("Total");
  expect(cell("A", "Total")).toHaveTextContent("2");
  expect(cell("C", "Total")).toHaveTextContent("0");
});

it("refreshes the links without running the query", async () => {
  const fresh = { ...result, issues: result.issues.slice(0, 2) };
  vi.mocked(api.RefreshLinks).mockResolvedValue(fresh as never);
  const { user, onResult } = show();
  await user.click(screen.getByRole("button", { name: "Refresh links" }));
  expect(api.RefreshLinks).toHaveBeenCalledWith("p1", 7, "2026-11-06T10:00:00Z", result);
  expect(onResult).toHaveBeenCalledWith(fresh);
});

it("keeps the run and says why when a refresh fails", async () => {
  vi.mocked(api.RefreshLinks).mockRejectedValue("the links could not be refreshed: Jira is down");
  const { user, onResult } = show();
  await user.click(screen.getByRole("button", { name: "Refresh links" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Jira is down");
  expect(onResult).not.toHaveBeenCalled();
});

it("drops a refresh that answers after the run on screen changed", async () => {
  let answer: (r: unknown) => void = () => undefined;
  vi.mocked(api.RefreshLinks).mockReturnValue(new Promise((r) => (answer = r)) as never);
  const { user, onResult, view } = show();
  await user.click(screen.getByRole("button", { name: "Refresh links" }));
  const other = { ...result, issues: result.issues.slice(3) };
  view.rerender(<LinkMap result={other as never} onOpen={vi.fn()} refresh={{ profileId: "p1", queryId: 8, runAt: "", onResult }} />);
  await act(async () => answer(result));
  expect(onResult).not.toHaveBeenCalled();
});

const lines = async () =>
  within(await screen.findByRole("list", { name: "Lines" }))
    .getAllByRole("listitem")
    .map((l) => l.textContent);

it("lays the links out from project through type to project", async () => {
  const { user } = show();
  await user.click(screen.getByRole("button", { name: "Flow" }));
  const chart = await screen.findByRole("list", { name: /Links from each project, by type, to the project they point to/ });
  expect(within(chart).getByRole("button", { name: "t:Cloners" })).toBeVisible();
  expect(await lines()).toEqual(["f:A>t:Cloners 1", "t:Cloners>o:B 1", "f:A>t:Relates 1", "t:Relates>o:C 1"]);
  expect(screen.queryByRole("table", { name: "Projects connected by links" })).not.toBeInTheDocument();
});

it("keeps the links with an end whose key contains the text", async () => {
  const { user } = show();
  await user.type(screen.getByLabelText("Key"), "c-1");
  expect(cell("A", "C")).toHaveTextContent("Relates 1");
  const grid = screen.getByRole("table", { name: "Projects connected by links" });
  expect(within(grid).queryByRole("rowheader", { name: "B" })).not.toBeInTheDocument();
});

it("keeps only the links between the projects chosen", async () => {
  const { user } = show();
  const box = screen.getByLabelText("Add a project");
  await user.type(box, "A{Enter}");
  await user.type(box, "B{Enter}");
  const grid = screen.getByRole("table", { name: "Projects connected by links" });
  expect(within(grid).getAllByRole("rowheader").map((h) => h.textContent)).toEqual(["A", "B"]);
  expect(cell("A", "B")).toHaveTextContent("Cloners 1");
  await user.click(screen.getByRole("button", { name: "Remove A" }));
  expect(screen.getByText("No links between projects to show.")).toBeVisible();
});

it("sorts the grid's rows and columns by project key or by project name", async () => {
  const named = {
    ...result,
    issues: result.issues.map((i) => ({ ...i, projectName: { A: "Zeta", B: "Alpha", C: "Mid" }[i.project] })),
  };
  const { user } = show(named);
  const grid = () => screen.getByRole("table", { name: "Projects connected by links" });
  const cols = () =>
    within(grid())
      .getAllByRole("columnheader")
      .slice(1, -1)
      .map((h) => h.textContent);
  const rows = () => within(grid()).getAllByRole("rowheader").map((h) => h.textContent);
  expect(cols()).toEqual(["A", "B", "C"]);
  await user.click(within(grid()).getByRole("button", { name: "Sort by project name" }));
  expect(cols()).toEqual(["B", "C", "A"]);
  expect(rows()).toEqual(["Alpha (B)", "Mid (C)", "Zeta (A)"]);
  await user.click(within(grid()).getByRole("button", { name: "Sort by project name" }));
  expect(cols()).toEqual(["A", "C", "B"]);
  await user.click(within(grid()).getByRole("button", { name: "Sort by project key" }));
  expect(cols()).toEqual(["A", "B", "C"]);
});

it("keeps only the chosen projects on the flow's left side", async () => {
  // C-1 also blocks B-1, so A and C both start links.
  const res = {
    ...result,
    issues: result.issues.map((i) =>
      i.key === "C-1" ? { ...i, links: [...i.links, link("B-1", "Blocks", "blocks", true)] } : i,
    ),
  };
  const { user } = show(res);
  await user.click(screen.getByRole("button", { name: "Flow" }));
  expect(await lines()).toContain("f:C>t:Blocks 1");
  await user.type(screen.getByRole("combobox", { name: "Left side" }), "A{Enter}");
  expect(await lines()).toContain("f:A>t:Cloners 1");
  expect(await lines()).not.toContain("f:C>t:Blocks 1");
});

it("draws the projects around the busiest one with G6, and hands a project to the Issues view or the grid", async () => {
  const { user } = show();
  await user.click(screen.getByRole("button", { name: "Network" }));
  const around = await screen.findByRole("list", { name: "Projects linked to A" });
  expect(within(around).getAllByRole("button").map((b) => b.textContent)).toEqual(["A", "B", "C"]);
  await user.click(within(around).getByRole("button", { name: "B" }));
  await user.click(screen.getByRole("button", { name: "Show its issues" }));
  // The issue graph starts at B's busiest issue.
  expect(screen.getByRole("button", { name: "Issues", pressed: true })).toBeVisible();
  expect(await screen.findByRole("complementary", { name: "Issue B-1" })).toBeVisible();

  await user.click(screen.getByRole("button", { name: "Projects" }));
  await user.click(await screen.findByRole("button", { name: "Open in Grid" }));
  // The grid keeps A and the projects linked to it.
  const grid = screen.getByRole("table", { name: "Projects connected by links" });
  expect(within(grid).getAllByRole("rowheader").map((h) => h.textContent)).toEqual(["A", "B", "C"]);
});

it("starts the issue graph at the run's busiest issue again when Issues is chosen by hand", async () => {
  const { user } = show();
  await user.click(screen.getByRole("button", { name: "Network" }));
  await user.click(within(await screen.findByRole("list", { name: "Projects linked to A" })).getByRole("button", { name: "B" }));
  await user.click(screen.getByRole("button", { name: "Show its issues" }));
  expect(await screen.findByRole("complementary", { name: "Issue B-1" })).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Projects" }));
  await user.click(screen.getByRole("button", { name: "Issues" }));
  expect(await screen.findByRole("complementary", { name: "Issue A-1" })).toBeVisible();
});
