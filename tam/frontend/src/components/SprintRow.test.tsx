import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SprintDetail } from "../api";
import { calendarDay } from "../lib/format";
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

function renderRow(over: Partial<SprintDetail> = {}, busy = false, waiting = "") {
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
      waiting={waiting}
      onToggle={onToggle}
      onKeyDown={() => {}}
      actions={actions}
    />,
  );
  return { actions, onToggle };
}

// The row draws a relative timeline now, so its wording depends on the
// clock. toFake: ["Date"] and nothing else: faking every timer breaks
// userEvent, and this file clicks.
// Built with the local-time constructor, not from a UTC instant: a sprint's
// dates are civil days, so the clock these fixtures pin has to be one too or
// the row reads a day out for a runner far enough from UTC.
const PINNED = new Date(2026, 8, 3, 12, 0, 0);

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(PINNED);
});
afterEach(() => {
  vi.useRealTimers();
});

describe("SprintRow", () => {
  it("draws the sprint's name, goal, state, dates and timeline", () => {
    renderRow();
    expect(screen.getByRole("treeitem", { name: /^Sprint 12, Active/ })).toBeInTheDocument();
    // "Active" in the accessible name and in the badge, both mixed case:
    // the badge's shout is text-transform in the stylesheet, so a screen
    // reader is not made to spell the state out.
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText("8 of 14 done, 21 of 34 pts")).toBeInTheDocument();
    expect(screen.getByText("Day 6 of 15")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Time elapsed in Sprint 12" })).toBeInTheDocument();
    // The goal used to be kept off the row, because every cell clips and
    // the detail panel narrows the pane. The redesign gives the name column
    // the widest track and lets the goal clip with a title the way the name
    // already does, so a reader can see what a sprint is for without
    // opening it. It is still drawn under an expanded sprint too, where the
    // empty case has wording of its own.
    expect(screen.getByText("Ship checkout")).toHaveAttribute("title", "Ship checkout");
  });

  it("names the row with everything it draws, so the tree says it out loud", () => {
    renderRow();
    // A treeitem with an explicit name is announced by that name and
    // nothing else: a reader arrowing down the tree hears the label and
    // never reaches the cells inside it. Everything the row says about
    // where the sprint is and how much is done has to be in the name, or
    // the bars and the timing are decoration for sighted readers only (I3).
    expect(screen.getByRole("treeitem", { name: /^Sprint 12/ })).toHaveAccessibleName(
      "Sprint 12, Active, Day 6 of 15, 9 days left, 8 of 14 done, 21 of 34 pts",
    );
  });

  it("says in the name that a closed sprint has not been read", () => {
    renderRow({ state: "closed", membershipCached: false, total: 0, done: 0, points: 0, donePoints: 0 });
    expect(screen.getByRole("treeitem", { name: /^Sprint 12/ })).toHaveAccessibleName(
      `Sprint 12, Closed, Closed ${calendarDay("2026-09-12")}, 15 days, cards not read yet`,
    );
  });

  it("says what a draft sprint is waiting for instead of a goal", () => {
    renderRow({ id: -1, name: "Sprint 15", state: "future", draft: true, goal: "", total: 0, done: 0, points: 0, donePoints: 0 });
    expect(screen.getByText("Draft")).toBeInTheDocument();
    expect(screen.getByText("not created in Jira yet")).toBeInTheDocument();
  });

  it("shows the state badge and the waiting chip together", () => {
    // Bundle 04's pending Commit chip has to survive the redesign: a sprint
    // that is future and is about to be started says both things.
    renderRow({ state: "future" }, false, "Starting on Commit");
    expect(screen.getByText("Future")).toBeInTheDocument();
    expect(screen.getByText("Starting on Commit")).toHaveClass("chip-draft");
  });

  it("says a closed sprint has not been read rather than counting it as empty", () => {
    renderRow({ state: "closed", membershipCached: false, total: 0, done: 0, points: 0, donePoints: 0 });
    expect(screen.getByText("cards not read yet")).toBeInTheDocument();
    expect(screen.queryByText(/0 cards/)).not.toBeInTheDocument();
    expect(screen.queryByRole("progressbar", { name: /Points done/ })).not.toBeInTheDocument();
  });

  it("says nothing when nothing waits for Commit", () => {
    renderRow({ state: "future" });
    expect(screen.queryByText(/on Commit/)).not.toBeInTheDocument();
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

  it("counts a sprint's cards once, not once per cell", () => {
    // The scope cell used to print "14 cards" and "8 done" an inch from the
    // timeline's "8 of 14 done, 21 of 34 pts", which is the same two
    // numbers twice. Its second line was also wrong: it called
    // total - done "carried over" while done came from the card's status
    // today, so work that was unfinished at close and got finished
    // afterwards, which is the normal life of carried-over work, counted as
    // done and the figure read 0. The close-time answer lives in the sprint
    // report, which is built from the sprint's own history.
    renderRow();
    expect(screen.queryByText("14 cards")).not.toBeInTheDocument();
    expect(screen.queryByText(/carried over/)).not.toBeInTheDocument();
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

  it("greys out its own actions while its own write is in flight, and stays open", async () => {
    const user = userEvent.setup();
    const { actions } = renderRow({}, true);
    await user.click(screen.getByRole("button", { name: "Actions on Sprint 12" }));
    const menu = screen.getByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Delete sprint…" })).toBeDisabled();
    await user.click(within(menu).getByRole("menuitem", { name: "Delete sprint…" }));
    expect(actions.onDelete).not.toHaveBeenCalled();
    // A disabled item dispatches no click, so the menu is not dismissed
    // either: it stays open with every action greyed, which is what says
    // the sprint is busy rather than the actions having gone away.
    expect(screen.getByRole("menu")).toBeInTheDocument();
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

  it("gives the board's own unassigned work no state, no dates, no timeline and no actions", () => {
    renderRow({ id: 0, name: "Board backlog", state: "unassigned", startDate: "", endDate: "", goal: "" });
    expect(screen.getByRole("treeitem", { name: /^Board backlog/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Actions on/ })).not.toBeInTheDocument();
    expect(screen.queryByText("Unassigned")).not.toBeInTheDocument();
    expect(screen.getByText("no dates")).toBeInTheDocument();
    expect(screen.getByText("no timeline")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    // It still counts its work, which is the one thing it has in common
    // with the sprints above it, and the name carries that count too.
    expect(screen.getByText("8 of 14 done, 21 of 34 pts")).toBeInTheDocument();
    expect(screen.getByRole("treeitem", { name: /^Board backlog/ })).toHaveAccessibleName(
      "Board backlog, no timeline, 8 of 14 done, 21 of 34 pts",
    );
  });

  it("toggles from the caret without the menu's own clicks reaching the row", async () => {
    const user = userEvent.setup();
    const { onToggle } = renderRow();
    await user.click(screen.getByRole("button", { name: "Actions on Sprint 12" }));
    expect(onToggle).not.toHaveBeenCalled();
    await user.click(screen.getByText("▸"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("marks a draft sprint, offers to start it, and holds back Complete until Commit", async () => {
    const user = userEvent.setup();
    const { actions } = renderRow({ id: -1, name: "Sprint 15", state: "future", draft: true, total: 0, done: 0, points: 0, donePoints: 0 });
    expect(screen.getByRole("treeitem", { name: /^Sprint 15, Draft/ })).toBeInTheDocument();
    expect(screen.getByText("Draft").closest(".status-badge")).toHaveClass("status-badge-draft");
    await user.click(screen.getByRole("button", { name: "Actions on Sprint 15" }));
    const menu = await screen.findByRole("menu");
    const complete = within(menu).getByRole("menuitem", { name: "Complete sprint…" });
    expect(complete).toBeDisabled();
    expect(complete).toHaveAttribute("title", "Commit this sprint first");
    const start = within(menu).getByRole("menuitem", { name: "Start sprint…" });
    expect(start).toBeEnabled();
    await user.click(start);
    expect(actions.onStart).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Actions on Sprint 15" }));
    const again = await screen.findByRole("menu");
    expect(within(again).getByRole("menuitem", { name: "Edit sprint…" })).toBeEnabled();
    expect(within(again).getByRole("menuitem", { name: "Delete sprint…" })).toBeEnabled();
  });
});
