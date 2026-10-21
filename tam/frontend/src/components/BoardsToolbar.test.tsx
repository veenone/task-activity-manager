import type { ReactElement } from "react";
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { ProfileProvider, createQueryClient } from "@agile-suite/core";
import type { Board, Sprint } from "../api";
import { profileBackend } from "../profileBackend";
import { BoardsToolbar, sprintOption } from "./BoardsToolbar";

// The toolbar carries the type, status and assignee bar on a second row, and
// that bar reads the project's types and the cached statuses itself, so the
// toolbar needs both providers now. No profile is loaded here: the bar then
// draws its All options and the types TAM models, which is all these tests
// need it to do without asserting on it.
function wrap(ui: ReactElement) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <ProfileProvider backend={profileBackend}>{ui}</ProfileProvider>
    </QueryClientProvider>,
  );
}

const board: Board = { id: 1, name: "PLAT Scrum", type: "scrum" };
const draft: Sprint = { id: -1, boardId: 1, name: "Sprint 15", state: "future", startDate: "", endDate: "", goal: "", draft: true };

function renderToolbar(sprint: Sprint, over: { boards?: Board[]; board?: Board; onNewBoard?: () => void; onAddIssues?: () => void } = {}) {
  wrap(
    <BoardsToolbar
      boards={over.boards ?? [board]} board={over.board ?? board} onBoard={vi.fn()}
      sprints={[sprint]} sprint={sprint} onSprint={vi.fn()}
      swimlane="none" onSwimlane={vi.fn()}
      refreshing={false} canRefresh onRefresh={vi.fn()}
      filter="" onFilter={vi.fn()}
      issueFilter={{ types: [], status: "", assignee: "" }} onIssueFilter={vi.fn()}
      onStart={vi.fn()} onComplete={vi.fn()} onCreate={vi.fn()}
      onNewBoard={over.onNewBoard ?? vi.fn()} onAddIssues={over.onAddIssues ?? vi.fn()}
    />,
  );
}

describe("BoardsToolbar", () => {
  it("labels a draft sprint in the picker and offers to start it on Commit", () => {
    renderToolbar(draft);
    expect(sprintOption(draft)).toBe("Sprint 15 (draft)");
    const start = screen.getByRole("button", { name: "Start sprint" });
    expect(start).toBeEnabled();
    expect(start).not.toHaveAttribute("title");
  });

  it("labels a draft board in the board picker", () => {
    renderToolbar(draft, { boards: [board, { id: -2, name: "Ops board", type: "scrum", draft: true }] });
    expect(screen.getByRole("option", { name: "PLAT Scrum" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Ops board (draft)" })).toBeInTheDocument();
  });

  it("starts a future sprint Jira holds", () => {
    renderToolbar({ ...draft, id: 14, name: "Sprint 14", draft: false });
    expect(screen.getByRole("button", { name: "Start sprint" })).toBeEnabled();
    expect(sprintOption({ ...draft, id: 14, name: "Sprint 14", draft: false })).toBe("Sprint 14 (Future)");
  });

  it("offers New board with no board on screen, and holds Add issues back until one is", () => {
    wrap(
      <BoardsToolbar
        boards={[]} board={undefined} onBoard={vi.fn()}
        sprints={[]} sprint={undefined} onSprint={vi.fn()}
        swimlane="none" onSwimlane={vi.fn()}
        refreshing={false} canRefresh onRefresh={vi.fn()}
        filter="" onFilter={vi.fn()}
        issueFilter={{ types: [], status: "", assignee: "" }} onIssueFilter={vi.fn()}
        onStart={vi.fn()} onComplete={vi.fn()} onCreate={vi.fn()}
        onNewBoard={vi.fn()} onAddIssues={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "New board" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add issues" })).not.toBeInTheDocument();
  });

  it("reaches New board and Add issues through their own buttons, beside New sprint", () => {
    const onNewBoard = vi.fn();
    const onAddIssues = vi.fn();
    renderToolbar(draft, { onNewBoard, onAddIssues });
    screen.getByRole("button", { name: "New board" }).click();
    screen.getByRole("button", { name: "Add issues" }).click();
    expect(onNewBoard).toHaveBeenCalledTimes(1);
    expect(onAddIssues).toHaveBeenCalledTimes(1);
  });
});
