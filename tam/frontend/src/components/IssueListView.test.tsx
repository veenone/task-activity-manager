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
    ListFilterOptions: vi.fn(),
    ExportBacklog: vi.fn(),
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
  vi.mocked(api.ExportBacklog).mockResolvedValue("C:/exports/tam-backlog.xlsx");
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.ListFilterOptions).mockResolvedValue({
    statuses: ["In Progress", "To Do"],
    assignees: ["R. Anand", "S. Kim"],
  });
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
  // One control, not one per type. A project with five types spent five
  // controls on one filter, and the action buttons were what fell to the
  // second row once #321 added two selects beside it (#323).
  //
  // The dropdown still selects several types at once, which the chips did and
  // a plain select would not: narrowing to "stories and bugs" is a real
  // question and losing it would be a worse bar, not a tidier one.
  it("offers the project's own types in one dropdown, one TAM does not model included", async () => {
    const user = userEvent.setup();
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());

    const types = screen.getByRole("button", { name: /All types/ });
    await user.click(types);
    await user.click(await screen.findByRole("checkbox", { name: "Improvement" }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["Improvement"]));

    // Two Jira types meaning the same logical type share one entry: two rows
    // selecting the same issues is a filter claiming two filters it has not.
    expect(screen.getAllByRole("checkbox", { name: "Task" })).toHaveLength(1);
    expect(screen.queryByRole("checkbox", { name: "Bug" })).toBeNull();

    // A second pick adds to the first rather than replacing it.
    await user.click(screen.getByRole("checkbox", { name: "Story" }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["Improvement", "story"]));
  });

  // The button is the only thing on screen when the panel is shut, so it has
  // to say what the filter is doing without being opened.
  it("says what is selected on the button", async () => {
    const user = userEvent.setup();
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: /All types/ }));
    await user.click(await screen.findByRole("checkbox", { name: "Story" }));
    expect(await screen.findByRole("button", { name: /Story/ })).toBeInTheDocument();
    await user.click(screen.getByRole("checkbox", { name: "Improvement" }));
    expect(await screen.findByRole("button", { name: /2 selected/ })).toBeInTheDocument();
  });

  // A profile that has never synced has no type list to read, and the filter
  // falls back to the six TAM models rather than offering nothing.
  it("falls back to the types TAM models when the project's are unknown", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListProjectTypes).mockResolvedValue([]);
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: /All types/ }));
    await user.click(await screen.findByRole("checkbox", { name: "Sub" }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["subtask"]));
  });
});

describe("IssueListView's export", () => {
  it("exports the filter the grid is showing, not the page", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Apply promo");
    await user.click(screen.getByRole("button", { name: /All types/ }));
    await user.click(await screen.findByRole("checkbox", { name: "Story" }));
    await user.click(screen.getByRole("button", { name: /Export/ }));
    await waitFor(() => expect(api.ExportBacklog).toHaveBeenCalled());
    const sent = vi.mocked(api.ExportBacklog).mock.calls.at(-1)?.[1];
    expect(sent?.types).toEqual(["story"]);
    // Offset and limit are the grid's paging, which an export of the
    // whole filter has no use for.
    expect(sent?.limit).toBe(0);
    expect(sent?.offset).toBe(0);
  });

  it("says where the file went", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Apply promo");
    await user.click(screen.getByRole("button", { name: /Export/ }));
    expect(await screen.findByText(/tam-backlog.xlsx/)).toBeInTheDocument();
  });

  // A cancelled dialog answers with no path, which is not a failure and
  // not something to announce.
  it("says nothing when the save dialog was cancelled", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ExportBacklog).mockResolvedValue("");
    renderView();
    await screen.findByText("Apply promo");
    await user.click(screen.getByRole("button", { name: /Export/ }));
    await waitFor(() => expect(api.ExportBacklog).toHaveBeenCalled());
    expect(screen.queryByText(/Saved/)).toBeNull();
  });

  it("reports an export that failed", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ExportBacklog).mockRejectedValue(new Error("the drive is full"));
    renderView();
    await screen.findByText("Apply promo");
    await user.click(screen.getByRole("button", { name: /Export/ }));
    expect(await screen.findByText(/the drive is full/)).toBeInTheDocument();
  });
});

describe("IssueListView's status and assignee filters", () => {
  // The options are the profile's own, read from the cache, so the bar never
  // offers a status or a person this project has no work in. Both reach the
  // query, which is what makes them filters rather than decoration: the grid
  // is paged, so a client-side narrowing would filter one page of many.
  it("offers what the cache holds and sends the choice to the query", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Apply promo");

    const status = await screen.findByRole("combobox", { name: "Status" });
    expect(within(status).getByRole("option", { name: "In Progress" })).toBeInTheDocument();
    expect(within(status).queryByRole("option", { name: "Blocked" })).toBeNull();
    await user.selectOptions(status, "In Progress");
    await waitFor(() => expect(lastQuery()?.statuses).toEqual(["In Progress"]));

    const assignee = screen.getByRole("combobox", { name: "Assignee" });
    await user.selectOptions(assignee, "S. Kim");
    await waitFor(() => expect(lastQuery()?.assignees).toEqual(["S. Kim"]));
    // The two narrow together, the way the store ANDs them.
    expect(lastQuery()?.statuses).toEqual(["In Progress"]);
  });

  // Nobody is a choice of its own. The query carries assignees as a list so
  // that the empty name can mean unassigned, because an empty string on its
  // own would read as no filter at all.
  it("asks for unassigned work as its own choice", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Apply promo");
    await user.selectOptions(screen.getByRole("combobox", { name: "Assignee" }), api.ASSIGNEE_NONE);
    await waitFor(() => expect(lastQuery()?.assignees).toEqual([""]));
  });

  // Page four of an unfiltered backlog is almost never page four of a
  // filtered one, so a filter that left the offset alone would show an empty
  // grid over a non-zero total.
  it("returns to the first page when a filter changes", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 400 });
    renderView();
    await waitFor(() => expect(api.ListIssues).toHaveBeenCalled());
    await user.click(await screen.findByRole("button", { name: "Next page" }));
    await waitFor(() => expect(lastQuery()?.offset).toBe(25));
    await user.selectOptions(screen.getByRole("combobox", { name: "Status" }), "To Do");
    await waitFor(() => expect(lastQuery()?.statuses).toEqual(["To Do"]));
    expect(lastQuery()?.offset).toBe(0);
  });
});
