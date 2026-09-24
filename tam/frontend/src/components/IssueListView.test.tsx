import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { IssueListView } from "./IssueListView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    ListIssues: vi.fn(),
    ListSprints: vi.fn(),
    ListOpenSprints: vi.fn(),
    GetSubtaskTypeName: vi.fn(),
    ListProjectTypes: vi.fn(),
  };
});

vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ status: "idle" }) }));

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderView() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ModalProvider>
            <Loader />
            <IssueListView viewId="t" label="Backlog" emptyNote="No issues" />
          </ModalProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const lastQuery = () => vi.mocked(api.ListIssues).mock.calls.at(-1)?.[1];

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListIssues).mockResolvedValue({
    issues: [{
      key: "PLAT-412", id: "1", project: "PLAT", type: "story", summary: "Apply promo", status: "To Do",
      assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "",
      storyPoints: null, rank: "", created: "", updated: "",
    }],
    total: 1,
  });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Technical task");
  // PLAT's own types: the task level under a name of its own as well as the
  // built-in one, a type TAM has no concept of, and a sub-task level called
  // something else again.
  vi.mocked(api.ListProjectTypes).mockResolvedValue([
    { id: "1", name: "Task", subtask: false, logical: "task" },
    { id: "2", name: "Todo", subtask: false, logical: "task" },
    { id: "3", name: "Story", subtask: false, logical: "story" },
    { id: "4", name: "Improvement", subtask: false, logical: "" },
    { id: "5", name: "Technical task", subtask: true, logical: "subtask" },
  ]);
});

describe("IssueListView's type filter", () => {
  // A project with ten types gets ten chips, under its own names for the
  // ones TAM has no concept of, and filtering for one of those has to reach
  // the query (#68). Two Jira types that mean the same logical type share
  // one chip: two chips selecting the same rows is a bar claiming two
  // filters it does not have.
  it("offers the project's own types, one TAM does not model included", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    const bar = screen.getByRole("group", { name: "Issue types" });
    const chip = await within(bar).findByRole("button", { name: "Improvement" });
    expect(chip.className).toContain("chip-type-none");
    await userEvent.click(chip);
    await waitFor(() => expect(lastQuery()?.types).toEqual(["Improvement"]));
    expect(within(bar).getAllByRole("button", { name: "Task" })).toHaveLength(1);
    // Nothing offers a type this project does not have.
    expect(within(bar).queryByRole("button", { name: "Bug" })).not.toBeInTheDocument();
  });

  // A profile that has never synced has no type list to read, and the bar
  // falls back to the six TAM models rather than offering nothing.
  it("falls back to the types TAM models when the project's are unknown", async () => {
    vi.mocked(api.ListProjectTypes).mockResolvedValue([]);
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    const bar = screen.getByRole("group", { name: "Issue types" });
    await within(bar).findByRole("button", { name: "Bug" });
    await userEvent.click(within(bar).getByRole("button", { name: "Sub" }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["subtask"]));
  });
});
