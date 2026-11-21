import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import * as api from "./api";
import { TOUR_VERSION } from "./tour/steps";
import App from "./App";
import { profileBackend } from "./profileBackend";
import { ViewProvider } from "./nav";
import { ModalProvider } from "./modals";
import { SyncProvider } from "./contexts/SyncContext";

// TAM mounts only the active view, so whatever a view keeps has to outlive
// the view itself. These go through the shell, switching tabs the way a
// user does, because the bug was the unmount (#524).

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    Health: vi.fn(), ListProfiles: vi.fn(), GetSettings: vi.fn(), SetDefaultProfile: vi.fn(),
    GetSyncState: vi.fn(), ListIssues: vi.fn(), GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn(),
    ListSprints: vi.fn(), ListOpenSprints: vi.fn(), ListFilterOptions: vi.fn(), GetSubtaskTypeName: vi.fn(),
    ListProjectTypes: vi.fn(), GetProfileSetting: vi.fn(), ListBoards: vi.fn(), ListBoardSprints: vi.fn(),
    GetBoard: vi.fn(), BoardAllocation: vi.fn(), CancelSprintReport: vi.fn(), EventsOn: vi.fn(() => () => {}),
    ListPendingChanges: vi.fn(),
  };
});

const issue = (key: string, summary: string): api.Issue => ({
  key, id: key, project: "DEMO", type: "story", summary, status: "To Do",
  assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "",
  storyPoints: null, rank: "", created: "", updated: "",
});

function renderApp() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <SyncProvider>
            <ViewProvider>
              <ModalProvider>
                <App />
              </ModalProvider>
            </ViewProvider>
          </SyncProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const tab = (name: string) => within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name });
const search = () => screen.findByRole("searchbox", { name: "Search issues" });

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.Health).mockResolvedValue({ ok: true, error: "", dbPath: "", sharedPath: "", logPath: "" });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
    { id: "p2", name: "Other team", jiraUrl: "demo", projectKey: "OTH", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", showNavRail: false, tourSeenVersion: TOUR_VERSION });
  vi.mocked(api.SetDefaultProfile).mockResolvedValue();
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0, projectTotal: 0 });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [issue("DEMO-1", "Apply promo"), issue("DEMO-2", "Refund flow")], total: 2 });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.ListFilterOptions).mockResolvedValue({ statuses: [], assignees: [] });
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
  vi.mocked(api.ListProjectTypes).mockResolvedValue([]);
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.ListBoards).mockResolvedValue([
    { id: 1, name: "Team board", type: "scrum" },
    { id: 2, name: "Support flow", type: "kanban" },
  ]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue([]);
  vi.mocked(api.GetBoard).mockResolvedValue({
    boardId: 2, sprintId: "", swimlane: "none", columns: [], lanes: [], donePoints: 0,
    unmapped: 0, unmappedStatuses: [], notSynced: 0, capped: false, needsStatusSync: false,
  });
  vi.mocked(api.BoardAllocation).mockResolvedValue([]);
  vi.mocked(api.CancelSprintReport).mockResolvedValue();
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
});

describe("TAM view state", () => {
  it("brings the Backlog's search, sort and open issue back after another view", async () => {
    const user = userEvent.setup();
    renderApp();
    await user.type(await search(), "promo");
    await user.click(screen.getByRole("button", { name: "Sort by Summary" }));
    await user.click(await screen.findByRole("row", { name: /DEMO-1/ }));

    await user.click(tab("Boards"));
    await screen.findByRole("region", { name: "Boards" });
    await user.click(tab("Backlog"));

    expect(await search()).toHaveValue("promo");
    expect(screen.getByRole("columnheader", { name: /Summary/ })).toHaveAttribute("aria-sort", "ascending");
    expect(await screen.findByRole("complementary", { name: "DEMO-1" })).toBeInTheDocument();
  });

  it("keeps the board picked in Reports when Open Boards is followed", async () => {
    const user = userEvent.setup();
    renderApp();
    await user.click(tab("Reports"));
    const reports = await screen.findByRole("region", { name: /Reports/ });
    await user.selectOptions(await within(reports).findByRole("combobox", { name: "Board" }), "2");
    await user.click(await within(reports).findByRole("button", { name: "Open Boards" }));

    const boards = await screen.findByRole("region", { name: "Boards" });
    await waitFor(() => expect(within(boards).getByRole("combobox", { name: "Board" })).toHaveValue("2"));
  });

  it("forgets a profile's view state when the profile is switched", async () => {
    const user = userEvent.setup();
    renderApp();
    await user.type(await search(), "promo");
    // Away from the Backlog, so the switch happens while it is not mounted
    // and nothing inside it can see the profile change.
    await user.click(tab("Boards"));
    await screen.findByRole("region", { name: "Boards" });

    const profile = screen.getByRole("combobox", { name: "Profile" });
    await waitFor(() => expect(profile).toBeEnabled());
    await user.selectOptions(profile, "p2");
    await waitFor(() => expect(profile).toHaveValue("p2"));
    await user.selectOptions(profile, "p1");
    await waitFor(() => expect(profile).toHaveValue("p1"));

    await user.click(tab("Backlog"));
    expect(await search()).toHaveValue("");
  });
});
