import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SprintDetail } from "../api";
import { SprintRow } from "./SprintRow";

function detail(over: Partial<SprintDetail> = {}): SprintDetail {
  return {
    id: 12, boardId: 1, name: "Sprint 12", state: "active",
    startDate: "2026-08-29T09:00:00Z", endDate: "2026-09-12T09:00:00Z", goal: "Ship checkout",
    issues: [], total: 14, done: 8, points: 34, donePoints: 21,
    membershipCached: true, notSynced: 0, truncated: false,
    ...over,
  };
}

function renderRow(over: Partial<SprintDetail> = {}, busy = false) {
  const actions = { onStart: vi.fn(), onComplete: vi.fn(), onEdit: vi.fn(), onDelete: vi.fn() };
  const onToggle = vi.fn();
  render(
    <SprintRow
      detail={detail(over)}
      rowId="sprint:12"
      index={0}
      open={false}
      focused
      flashed={false}
      busy={busy}
      onToggle={onToggle}
      onKeyDown={() => {}}
      actions={actions}
    />,
  );
  return { actions, onToggle };
}

describe("SprintRow", () => {
  it("draws the sprint's name, state, dates and progress, and leaves the goal off the row", () => {
    renderRow();
    expect(screen.getByRole("treeitem", { name: "Sprint 12, Active" })).toBeInTheDocument();
    // "Active", not shouting: a list where one row of twenty is running does
    // not need capitals to say which.
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText("8 of 14 done, 21 of 34 pts")).toBeInTheDocument();
    // The goal is a sentence and every cell here clips, so it is rendered
    // under the sprint when the sprint is open rather than in a cell.
    expect(screen.queryByText("Ship checkout")).not.toBeInTheDocument();
  });

  it("says nothing about progress in a sprint that holds nothing", () => {
    renderRow({ total: 0, done: 0, points: 0, donePoints: 0 });
    expect(screen.queryByText(/done/)).not.toBeInTheDocument();
  });

  it("counts what the sprint holds, not what the view drew of it", () => {
    // The read spends one card budget across every sprint together, so a
    // sprint whose share ran out draws no cards and still holds all of them.
    renderRow({ issues: [], total: 14, truncated: true });
    expect(screen.getByText("8 of 14 done, 21 of 34 pts")).toBeInTheDocument();
  });

  it("offers Complete on a running sprint and Start on one that has not begun", async () => {
    const user = userEvent.setup();
    const { actions } = renderRow();
    await user.click(screen.getByRole("button", { name: "Actions on Sprint 12" }));
    const menu = screen.getByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Complete sprint…" })).toBeInTheDocument();
    expect(within(menu).queryByRole("menuitem", { name: "Start sprint…" })).not.toBeInTheDocument();
    await user.click(within(menu).getByRole("menuitem", { name: "Edit sprint…" }));
    expect(actions.onEdit).toHaveBeenCalled();
  });

  it("closes its own menu while its own write is in flight", async () => {
    const user = userEvent.setup();
    const { actions } = renderRow({}, true);
    await user.click(screen.getByRole("button", { name: "Actions on Sprint 12" }));
    const menu = screen.getByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Delete sprint…" })).toBeDisabled();
    await user.click(within(menu).getByRole("menuitem", { name: "Delete sprint…" }));
    expect(actions.onDelete).not.toHaveBeenCalled();
  });

  it("keeps the menu trigger out of the tab order, the way a card's menu does", () => {
    renderRow();
    // The tree is one tab stop, so a trigger per sprint would make it one
    // stop per sprint; the keyboard opens it through the same class handle
    // the board card's menu uses.
    const trigger = screen.getByRole("button", { name: "Actions on Sprint 12" });
    expect(trigger).toHaveAttribute("tabindex", "-1");
    expect(trigger).toHaveClass("board-card-menu");
  });

  it("gives the board's own unassigned work no state, no dates and no actions", () => {
    renderRow({ id: 0, name: "Board backlog", state: "unassigned", startDate: "", endDate: "", goal: "" });
    expect(screen.getByRole("treeitem", { name: "Board backlog" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Actions on/ })).not.toBeInTheDocument();
    expect(screen.queryByText("Unassigned")).not.toBeInTheDocument();
    // It still counts its work, which is the one thing it has in common
    // with the sprints above it.
    expect(screen.getByText("8 of 14 done, 21 of 34 pts")).toBeInTheDocument();
  });

  it("toggles from the caret without the menu's own clicks reaching the row", async () => {
    const user = userEvent.setup();
    const { onToggle } = renderRow();
    await user.click(screen.getByRole("button", { name: "Actions on Sprint 12" }));
    expect(onToggle).not.toHaveBeenCalled();
    await user.click(screen.getByText("▸"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });
});
