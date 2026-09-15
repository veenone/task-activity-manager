import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { Board, Sprint } from "../api";
import { BoardsToolbar, sprintOption } from "./BoardsToolbar";

const board: Board = { id: 1, name: "PLAT Scrum", type: "scrum" };
const draft: Sprint = { id: -1, boardId: 1, name: "Sprint 15", state: "future", startDate: "", endDate: "", goal: "", draft: true };

function renderToolbar(sprint: Sprint) {
  render(
    <BoardsToolbar
      boards={[board]} board={board} onBoard={vi.fn()}
      sprints={[sprint]} sprint={sprint} onSprint={vi.fn()}
      swimlane="none" onSwimlane={vi.fn()}
      refreshing={false} canRefresh onRefresh={vi.fn()}
      filter="" onFilter={vi.fn()}
      onStart={vi.fn()} onComplete={vi.fn()} onCreate={vi.fn()}
    />,
  );
}

describe("BoardsToolbar", () => {
  it("labels a draft sprint in the picker and holds Start back", () => {
    renderToolbar(draft);
    expect(sprintOption(draft)).toBe("Sprint 15 (draft)");
    const start = screen.getByRole("button", { name: "Start sprint" });
    expect(start).toBeDisabled();
    expect(start).toHaveAttribute("title", "Commit this sprint first");
  });

  it("starts a future sprint Jira holds", () => {
    renderToolbar({ ...draft, id: 14, name: "Sprint 14", draft: false });
    expect(screen.getByRole("button", { name: "Start sprint" })).toBeEnabled();
    expect(sprintOption({ ...draft, id: 14, name: "Sprint 14", draft: false })).toBe("Sprint 14 (Future)");
  });
});
