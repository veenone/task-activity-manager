import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Issue, SprintDetail } from "../api";
import { SprintList } from "./SprintList";
import { issueOrder, rowIdOf } from "../lib/sprintRows";
import { leadDepth, leadSlotText } from "../test/rowLead";

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", statusId: "1",
    assignee: "", reporter: "", priority: "", labels: [], sprintId: "12", sprintName: "Sprint 12",
    parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
    ...over,
  };
}

function detail(over: Partial<SprintDetail>): SprintDetail {
  const base: SprintDetail = {
    id: 12, boardId: 1, name: "Sprint 12", state: "active",
    startDate: "2026-08-29T09:00:00Z", endDate: "2026-09-12T09:00:00Z", goal: "Ship checkout",
    issues: [], total: 0, done: 0, points: 0, donePoints: 0,
    membershipCached: true, notSynced: 0, truncated: false,
    ...over,
  };
  // Total defaults to what was handed in, so a fixture only says so when it
  // is testing the case where the two differ.
  return over.total === undefined ? { ...base, total: base.issues.length } : base;
}

const PROMO = issue({ key: "PLAT-412", summary: "Apply promo code", assignee: "R. Anand", storyPoints: 8 });
const KEYS = issue({ key: "PLAT-409", summary: "Rotate the gateway keys", assignee: "M. Ortiz", storyPoints: 2 });
const RETRO = issue({ key: "PLAT-347", summary: "Write retro notes" });

const ACTIVE = detail({ issues: [PROMO, KEYS, RETRO] });
const FUTURE = detail({ id: 13, name: "Sprint 13", state: "future", goal: "", issues: [issue({ key: "PLAT-500" })] });
const BACKLOG = detail({
  id: 0, name: "Board backlog", state: "unassigned", startDate: "", endDate: "", goal: "",
  issues: [issue({ key: "PLAT-900", summary: "Unscheduled" })],
});

function renderList(details: SprintDetail[] = [ACTIVE, FUTURE, BACKLOG], over: Record<string, unknown> = {}) {
  const on = {
    onSelect: vi.fn(), onCheck: vi.fn(), onExtend: vi.fn(), onClearTo: vi.fn(),
    onStart: vi.fn(), onComplete: vi.fn(), onEdit: vi.fn(), onDelete: vi.fn(),
  };
  render(
    <SprintList
      details={details}
      selectedKey=""
      checked={new Set()}
      movedRowId=""
      busyRowId=""
      {...on}
      {...over}
    />,
  );
  return on;
}

describe("SprintList", () => {
  it("nests a subtask under its parent across assignees and navigates back to the parent", async () => {
    const user = userEvent.setup();
    const child = issue({ key: "TAM-NEW-1", type: "subtask", parentKey: PROMO.key, summary: "Wire input", assignee: "Other owner", draft: true });
    const on = renderList([detail({ issues: [child, KEYS, PROMO] })]);
    const parent = await screen.findByRole("treeitem", { name: `${PROMO.key} ${PROMO.summary}` });
    const subtask = screen.getByRole("treeitem", { name: "TAM-NEW-1 Wire input" });
    expect(parent.nextElementSibling).toBe(subtask);
    expect(subtask).toHaveAttribute("aria-level", "3");
    expect(within(subtask).getByText("Other owner")).toBeInTheDocument();
    await user.click(subtask);
    expect(on.onSelect).toHaveBeenCalledWith(child.key);
    await user.keyboard("{ArrowLeft}");
    expect(parent).toHaveFocus();
    await user.keyboard("{ArrowLeft}");
    expect(screen.queryByRole("treeitem", { name: "TAM-NEW-1 Wire input" })).not.toBeInTheDocument();
    expect(parent).toHaveAttribute("aria-expanded", "false");
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("treeitem", { name: `${KEYS.key} ${KEYS.summary}` })).toHaveFocus();
    await user.click(screen.getByRole("button", { name: `Expand subtasks of ${PROMO.key}` }));
    expect(screen.getByRole("treeitem", { name: "TAM-NEW-1 Wire input" })).toBeInTheDocument();
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("treeitem", { name: "TAM-NEW-1 Wire input" })).toHaveFocus();
    expect(issueOrder([detail({ issues: [child, KEYS, PROMO] })], new Set([PROMO.key])).get(rowIdOf(ACTIVE))).not.toContain(child.key);
  });
  it("leads every card of a family the same way and keeps the subtask's owner out of the clip", async () => {
    const child = issue({ key: "TAM-NEW-1", type: "subtask", parentKey: PROMO.key, assignee: "Other owner", summary: "Wire input" });
    const stray = issue({ key: "TAM-NEW-2", type: "subtask", parentKey: "PLAT-901", summary: "Retire the gateway" });
    renderList([detail({ issues: [PROMO, child, KEYS, stray] })]);
    const rowOf = async (name: string) => await screen.findByRole("treeitem", { name });
    const parent = await rowOf(`${PROMO.key} ${PROMO.summary}`);
    const childless = await rowOf(`${KEYS.key} ${KEYS.summary}`);
    const subtask = await rowOf("TAM-NEW-1 Wire input");
    const strayRow = await rowOf("TAM-NEW-2 Retire the gateway");
    expect(leadSlotText(parent)).toBe("▾ 1");
    expect(leadSlotText(childless)).toBe("");
    expect(leadDepth(childless)).toBe(leadDepth(parent));
    expect(leadDepth(subtask)).toBeGreaterThan(leadDepth(parent));
    // The owner used to sit inside the cell that clips, after a summary
    // long enough to fill it, so it was never on screen. It is a sibling of
    // the text now: the summary is the only part that clips.
    const owner = within(subtask).getByText("Other owner");
    expect(owner.closest(".row-summary-text")).toBeNull();
    expect(within(subtask).getByText("Wire input")).toHaveClass("row-summary-text");
    expect(within(strayRow).getByText("Parent not shown")).toBeInTheDocument();
    expect(strayRow).toHaveAttribute("aria-level", "3");
  });

  it("opens the active sprint and leaves every other one closed", async () => {
    renderList();
    // A board carries every sprint it has ever run, so opening what has not
    // been seen before would open the whole of last quarter.
    expect(await screen.findByText("Apply promo code")).toBeInTheDocument();
    expect(screen.queryByText("PLAT-500")).not.toBeInTheDocument();
    expect(screen.queryByText("Unscheduled")).not.toBeInTheDocument();
  });

  it("renders the goal under the sprint, and says so when the column is still empty", async () => {
    const user = userEvent.setup();
    renderList();
    // The goal is on the row as well as under the expanded sprint now, so
    // an unscoped query finds it twice once a sprint is open. The row's
    // copy is the clipped one; the group's is the one with a line to
    // itself and with wording for a sprint that has no goal at all.
    const group = await screen.findByRole("group");
    expect(within(group).getByText("Ship checkout")).toBeInTheDocument();
    await user.click(screen.getByRole("treeitem", { name: /^Sprint 13, Future/ }));
    // A sprint cached before the goal column existed reads as having no
    // goal, and a refresh is what tells the two apart.
    expect(screen.getByText("No goal recorded yet; refresh the board.")).toBeInTheDocument();
  });

  it("bands the cards by assignee without making a band something to land on", async () => {
    renderList();
    const band = await screen.findByText("R. Anand");
    expect(band.closest("[role='presentation']")).not.toBeNull();
    // The cards are the only tree items under a sprint, so the tree stays
    // two levels deep and nothing lands focus on a heading that does
    // nothing when it is activated.
    expect(screen.queryByRole("treeitem", { name: /R. Anand/ })).not.toBeInTheDocument();
    expect(screen.getByRole("treeitem", { name: "PLAT-412 Apply promo code" })).toBeInTheDocument();
  });

  it("orders the bands by points and puts the cards nobody owns last", async () => {
    renderList();
    await screen.findByText("Apply promo code");
    const bands = screen.getAllByRole("presentation").map((b) => b.textContent);
    expect(bands[0]).toContain("R. Anand");
    expect(bands[1]).toContain("M. Ortiz");
    expect(bands[2]).toContain("Unassigned");
  });

  it("checks a card with Space and selects one with Enter", async () => {
    const user = userEvent.setup();
    const on = renderList();
    const row = await screen.findByRole("treeitem", { name: "PLAT-412 Apply promo code" });
    row.focus();
    await user.keyboard(" ");
    expect(on.onCheck).toHaveBeenCalledWith("PLAT-412");
    expect(on.onSelect).not.toHaveBeenCalled();
    await user.keyboard("{Enter}");
    expect(on.onSelect).toHaveBeenCalledWith("PLAT-412");
  });

  it("extends a shift gesture inside the sprint it was made in", async () => {
    const user = userEvent.setup();
    const on = renderList();
    const row = await screen.findByRole("treeitem", { name: "PLAT-409 Rotate the gateway keys" });
    await user.keyboard("{Shift>}");
    await user.click(row);
    await user.keyboard("{/Shift}");
    // The scope travels with the gesture, which is what keeps a run from
    // crossing into another sprint: the selection measures it against that
    // sprint's own list of keys and nothing else.
    expect(on.onExtend).toHaveBeenCalledWith("PLAT-409", rowIdOf(ACTIVE));
    expect(on.onSelect).not.toHaveBeenCalled();
  });

  it("says how much of a sprint it did not draw, counting what the sprint holds", async () => {
    // The read spends one card budget across every sprint together, so a
    // sprint whose share ran out draws fewer cards than it counted.
    renderList([detail({ issues: [PROMO], total: 40, truncated: true })]);
    expect(await screen.findByText(/39 cards more are in this list than the view draws/)).toBeInTheDocument();
  });

  it("says when a sprint holds keys the cache does not, which is a different gap", async () => {
    renderList([detail({ issues: [PROMO], notSynced: 3 })]);
    expect(await screen.findByText(/3 cards in this list have not been synced/)).toBeInTheDocument();
  });

  it("says a closed sprint has not been read yet rather than calling it empty", async () => {
    const user = userEvent.setup();
    renderList([detail({ id: 11, name: "Sprint 11", state: "closed", issues: [], membershipCached: false })]);
    await user.click(await screen.findByRole("treeitem", { name: /^Sprint 11, Closed/ }));
    // The cards are coming now: the sync reads a closed sprint's membership
    // once and keeps it, so the empty list is a fact about this cache and
    // not about the sprint.
    expect(screen.getByText(/have not been read yet/)).toBeInTheDocument();
    expect(screen.queryByText(/whatever the sprint held/)).not.toBeInTheDocument();
  });

  it("keeps every sprint's row id out of the keys a fill can send", () => {
    // lib/boardSelection's extend slices the order list without looking at
    // what is in it, so a sprint id that could pass for an issue key would
    // end up checked and then sent to Go as one.
    const order = issueOrder([ACTIVE, BACKLOG]);
    expect([...order.values()].flat()).toEqual(["PLAT-412", "PLAT-409", "PLAT-347", "PLAT-900"]);
    expect(rowIdOf(ACTIVE)).toBe("sprint:12");
    expect(rowIdOf(BACKLOG)).toBe("sprint:unassigned");
  });

  it("draws the board's own work last, as a node that is not a sprint", async () => {
    renderList();
    const items = screen.getAllByRole("treeitem").map((i) => i.getAttribute("aria-label"));
    // The row's name carries what it draws, so this checks which node is
    // last rather than the whole label.
    expect(items[items.length - 1]).toMatch(/^Board backlog,/);
    const backlog = screen.getByRole("treeitem", { name: /^Board backlog/ });
    expect(within(backlog).queryByRole("button")).not.toBeInTheDocument();
  });
});
