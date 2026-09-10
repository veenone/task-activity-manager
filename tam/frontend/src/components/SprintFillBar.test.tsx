import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Sprint } from "../api";
import { SprintFillBar } from "./SprintFillBar";

const SPRINTS: Sprint[] = [
  { id: 12, boardId: 1, name: "Sprint 12", state: "active", startDate: "", endDate: "", goal: "" },
  { id: 13, boardId: 1, name: "Sprint 13", state: "future", startDate: "", endDate: "", goal: "" },
];

function renderBar(over: Partial<Parameters<typeof SprintFillBar>[0]> = {}) {
  const onFill = vi.fn();
  const onClear = vi.fn();
  render(
    <SprintFillBar count={3} sprints={SPRINTS} busy={false} onFill={onFill} onClear={onClear} {...over} />,
  );
  return { onFill, onClear };
}

describe("SprintFillBar", () => {
  it("cannot send anywhere until a destination is chosen", async () => {
    const user = userEvent.setup();
    const { onFill } = renderBar();
    expect(screen.getByText("3 cards selected")).toBeInTheDocument();
    const move = screen.getByRole("button", { name: "Move 3 cards" });
    expect(move).toBeDisabled();
    await user.selectOptions(screen.getByRole("combobox", { name: "Move the checked cards to" }), "13");
    expect(move).toBeEnabled();
    await user.click(move);
    expect(onFill).toHaveBeenCalledWith("13");
  });

  it("sends the backlog as an empty sprint id, since it is a destination and not an absence", async () => {
    const user = userEvent.setup();
    const { onFill } = renderBar();
    await user.selectOptions(screen.getByRole("combobox", { name: "Move the checked cards to" }), "backlog");
    await user.click(screen.getByRole("button", { name: "Move 3 cards" }));
    expect(onFill).toHaveBeenCalledWith("");
  });

  it("offers every open sprint, including the one a checked card may already be in", () => {
    renderBar();
    const picker = screen.getByRole("combobox", { name: "Move the checked cards to" });
    // A selection here can span sprints, so there is no one sprint to leave
    // out of the list the way the board's own bar leaves out the sprint on
    // screen.
    expect(within(picker).getByRole("option", { name: "Sprint 12" })).toBeInTheDocument();
    expect(within(picker).getByRole("option", { name: "Sprint 13" })).toBeInTheDocument();
  });

  it("does nothing on a second press while the first is still in flight", async () => {
    const user = userEvent.setup();
    const { onFill } = renderBar({ busy: true });
    const move = screen.getByRole("button", { name: "Move 3 cards" });
    expect(move).toBeDisabled();
    await user.click(move);
    expect(onFill).not.toHaveBeenCalled();
  });
});
