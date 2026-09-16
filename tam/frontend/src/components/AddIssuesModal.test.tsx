import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Issue, Sprint, SprintDetail } from "../api";
import { AddIssuesModal } from "./AddIssuesModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListIssues: vi.fn(),
    ListBoardSprintDetails: vi.fn(),
    AddIssuesToBoard: vi.fn(),
  };
});

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", statusId: "1",
    assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "",
    storyPoints: null, rank: "", created: "", updated: "",
    ...over,
  };
}

function node(over: Partial<SprintDetail>): SprintDetail {
  return {
    id: 0, boardId: 1, name: "", state: "future", startDate: "", endDate: "", goal: "",
    issues: [], total: 0, done: 0, points: 0, donePoints: 0, membershipCached: true, notSynced: 0, truncated: false,
    ...over,
  };
}

const SPRINTS: Sprint[] = [{ id: 12, boardId: 1, name: "Sprint 12", state: "active", startDate: "", endDate: "", goal: "" }];

const ON_BOARD = issue({ key: "PLAT-409", summary: "Already on this board" });
const NOT_ON_BOARD = issue({ key: "PLAT-500", summary: "Rotate payment gateway API keys" });

function renderModal(onAdded = vi.fn(), onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <AddIssuesModal profileId="p1" boardId={1} boardName="PLAT Scrum" sprints={SPRINTS} onClose={onClose} onAdded={onAdded} />
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onAdded, onClose };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [ON_BOARD, NOT_ON_BOARD], total: 2 });
  vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([
    node({ id: 12, name: "Sprint 12", state: "active", issues: [ON_BOARD] }),
    node({ id: 0, name: "Board backlog", state: "unassigned", issues: [] }),
  ]);
  vi.mocked(api.AddIssuesToBoard).mockResolvedValue();
});

describe("AddIssuesModal", () => {
  it("leaves out a card this board already holds, keeping only what it does not", async () => {
    renderModal();
    // The checkbox for the row appears once the query settles; waiting for
    // the enabled search box first and picking blind from there is the race
    // this repository has already been bitten by.
    expect(await screen.findByRole("checkbox", { name: /PLAT-500/ })).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: /PLAT-409/ })).not.toBeInTheDocument();
  });

  it("Add stays disabled until something is checked", async () => {
    renderModal();
    const checkbox = await screen.findByRole("checkbox", { name: /PLAT-500/ });
    const addBtn = screen.getByRole("button", { name: "Add" });
    expect(addBtn).toBeDisabled();
    await userEvent.setup().click(checkbox);
    expect(addBtn).toBeEnabled();
  });

  it("journals the checked keys onto the sprint chosen under Into, and reports it", async () => {
    const user = userEvent.setup();
    const { onAdded, onClose } = renderModal();
    const checkbox = await screen.findByRole("checkbox", { name: /PLAT-500/ });
    await user.click(checkbox);
    await user.selectOptions(screen.getByLabelText("Into"), "12");
    await user.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() => expect(api.AddIssuesToBoard).toHaveBeenCalledWith("p1", ["PLAT-500"], 1, "12"));
    expect(onAdded).toHaveBeenCalledWith(expect.stringContaining("Sprint 12"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("defaults Into to the backlog", async () => {
    const user = userEvent.setup();
    renderModal();
    const checkbox = await screen.findByRole("checkbox", { name: /PLAT-500/ });
    await user.click(checkbox);
    await user.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() => expect(api.AddIssuesToBoard).toHaveBeenCalledWith("p1", ["PLAT-500"], 1, "backlog"));
  });
});
