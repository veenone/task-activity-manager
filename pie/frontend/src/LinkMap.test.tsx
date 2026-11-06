import { expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinkMap } from "./LinkMap";

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
  render(<LinkMap result={res as never} onOpen={onOpen} />);
  return { user: userEvent.setup(), onOpen };
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
