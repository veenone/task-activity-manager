import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Issue, SprintOption } from "../api";
import { SprintField } from "./SprintField";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, MoveIssueToSprint: vi.fn() };
});

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-412", id: "1", project: "PLAT", type: "story", summary: "x", status: "In Progress", assignee: "",
    reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null,
    rank: "", created: "", updated: "",
    ...over,
  };
}

function renderField(sprints: SprintOption[], iss: Issue = issue({}), busy = false) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <SprintField profileId="p1" issue={iss} sprints={sprints} busy={busy} />
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.MoveIssueToSprint).mockResolvedValue();
});

describe("SprintField", () => {
  it("says the empty-list sentence beside the select rather than instead of it", () => {
    renderField([]);
    expect(screen.getByText("No sprints yet, sync a board first")).toBeInTheDocument();
    const select = screen.getByRole("combobox", { name: "Sprint" });
    expect(within(select).getByRole("option", { name: "The backlog" })).toBeInTheDocument();
  });

  it("lets a card already in a sprint still leave it when the list is empty", () => {
    renderField([], issue({ sprintId: "12", sprintName: "Sprint 12" }));
    const select = screen.getByRole("combobox", { name: "Sprint" });
    expect(within(select).getByRole("option", { name: "Sprint 12" })).toBeInTheDocument();
    expect(within(select).getByRole("option", { name: "The backlog" })).toBeInTheDocument();
    expect(screen.getByText("No sprints yet, sync a board first")).toBeInTheDocument();
  });

  it("shows no sentence once the list has sprints", () => {
    renderField([{ id: 12, name: "Sprint 12" }]);
    expect(screen.queryByText(/No sprints yet/)).not.toBeInTheDocument();
  });

  it("names the board beside a sprint only when two sprints share a name", async () => {
    const user = userEvent.setup();
    renderField([
      { id: 12, name: "Sprint 1", boardName: "Platform board" },
      { id: 13, name: "Sprint 1", boardName: "Ops board" },
      { id: 14, name: "Sprint 2", boardName: "Ops board" },
    ]);
    const select = screen.getByRole("combobox", { name: "Sprint" });
    // Both same-named sprints get their board name, not only the second.
    expect(within(select).getByRole("option", { name: "Sprint 1 (Platform board)" })).toBeInTheDocument();
    expect(within(select).getByRole("option", { name: "Sprint 1 (Ops board)" })).toBeInTheDocument();
    // A unique name is shown plain.
    expect(within(select).getByRole("option", { name: "Sprint 2" })).toBeInTheDocument();
    await user.selectOptions(select, "13");
    await waitFor(() => expect(api.MoveIssueToSprint).toHaveBeenCalledWith("p1", "PLAT-412", "13"));
  });
});
