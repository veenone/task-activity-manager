import { expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinkMap } from "./LinkMap";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({ RefreshLinks: vi.fn() }));

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

it("draws the links as a flow from project through type to project", async () => {
  const { user } = show();
  await user.click(screen.getByRole("button", { name: "Flow" }));
  const chart = screen.getByRole("img", { name: /Links from each project, by type, to the project they point to/ });
  expect(within(chart).getByText("A to Cloners: 1")).toBeInTheDocument();
  expect(screen.queryByRole("table", { name: "Projects connected by links" })).not.toBeInTheDocument();
});

it("centres the network on the busiest project and recentres on a neighbour", async () => {
  const { user } = show();
  await user.click(screen.getByRole("button", { name: "Network" }));
  expect(screen.getByLabelText("Centre project")).toHaveValue("A");
  const around = screen.getByRole("list", { name: "Projects linked to A" });
  expect(
    within(around)
      .getAllByRole("button")
      .map((b) => b.textContent),
  ).toEqual(["B: Cloners 1 out", "C: Relates 1 out"]);
  await user.click(within(around).getByRole("button", { name: /^B/ }));
  expect(screen.getByLabelText("Centre project")).toHaveValue("B");
  expect(screen.getByRole("img", { name: "Projects linked to B by link type" })).toBeInTheDocument();
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

it("draws a hovered project's name last, on top of the others", async () => {
  const { user } = show();
  await user.click(screen.getByRole("button", { name: "Network" }));
  const chart = screen.getByRole("img", { name: "Projects linked to A by link type" });
  const node = chart.querySelector('[data-project="B"]')!;
  fireEvent.mouseEnter(node);
  const texts = [...chart.querySelectorAll("text")];
  expect(texts.at(-1)).toHaveTextContent("B");
  expect(texts.at(-1)).toHaveClass("pie-network-label-hover");
  fireEvent.mouseLeave(node);
  expect(chart.querySelector(".pie-network-label-hover")).toBeNull();
});

it("moves a dragged project without recentring, and lays out afresh for a new centre", async () => {
  const { user } = show();
  await user.click(screen.getByRole("button", { name: "Network" }));
  const chart = screen.getByRole("img", { name: "Projects linked to A by link type" });
  const node = chart.querySelector('[data-project="B"]')!;
  const before = node.querySelector("circle")!.getAttribute("cx");
  fireEvent.pointerDown(node, { clientX: 0, clientY: 0, pointerId: 1 });
  fireEvent.pointerMove(node, { clientX: 40, clientY: 30, pointerId: 1 });
  fireEvent.pointerUp(node, { clientX: 40, clientY: 30, pointerId: 1 });
  fireEvent.click(node);
  expect(node.querySelector("circle")!.getAttribute("cx")).not.toBe(before);
  expect(screen.getByLabelText("Centre project")).toHaveValue("A");
});
