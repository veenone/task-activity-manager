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

// Full sync moved from the Sync menu into More (#540). It still has to ask
// before it starts (#534), so this runs through the shell, where the top bar
// hands the More entry to the guarded runner.

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    Health: vi.fn(), ListProfiles: vi.fn(), GetSettings: vi.fn(), SetDefaultProfile: vi.fn(),
    GetSyncState: vi.fn(), ListIssues: vi.fn(), ListSprints: vi.fn(), ListOpenSprints: vi.fn(),
    ListFilterOptions: vi.fn(), GetSubtaskTypeName: vi.fn(), ListProjectTypes: vi.fn(),
    GetProfileSetting: vi.fn(), ListEpics: vi.fn(), ListPendingChanges: vi.fn(), SyncIssues: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
  };
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

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.Health).mockResolvedValue({ ok: true, error: "", dbPath: "", sharedPath: "", logPath: "" });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", showNavRail: false, tourSeenVersion: TOUR_VERSION });
  vi.mocked(api.SetDefaultProfile).mockResolvedValue();
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0, projectTotal: 0 });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.ListFilterOptions).mockResolvedValue({ statuses: [], assignees: [] });
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
  vi.mocked(api.ListProjectTypes).mockResolvedValue([]);
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
  vi.mocked(api.SyncIssues).mockResolvedValue({ fetched: 0, upserted: 0, projectTotal: 0, full: true, elapsed: "1s" });
});

async function chooseFullSync(user: ReturnType<typeof userEvent.setup>) {
  renderApp();
  const more = await screen.findByRole("button", { name: "More" });
  await waitFor(() => expect(screen.getByRole("button", { name: "Sync" })).toBeEnabled());
  await user.click(more);
  await user.click(screen.getByRole("menuitem", { name: "Full sync" }));
  return screen.findByRole("alertdialog", { name: "Full sync" });
}

describe("the More menu's Full sync", () => {
  it("asks first and runs a full sync once confirmed", async () => {
    const user = userEvent.setup();
    const dialog = await chooseFullSync(user);
    expect(api.SyncIssues).not.toHaveBeenCalled();
    const confirm = within(dialog).getAllByRole("button", { name: "Full sync" });
    await user.click(confirm[confirm.length - 1]);
    await waitFor(() => expect(api.SyncIssues).toHaveBeenCalledWith("p1", true));
  });

  it("starts nothing when the question is cancelled", async () => {
    const user = userEvent.setup();
    const dialog = await chooseFullSync(user);
    const cancel = within(dialog).getAllByRole("button", { name: "Cancel" });
    await user.click(cancel[cancel.length - 1]);
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(api.SyncIssues).not.toHaveBeenCalled();
  });
});
