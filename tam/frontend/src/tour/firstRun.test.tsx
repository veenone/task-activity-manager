import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { ViewProvider } from "../nav";
import { SyncProvider } from "../contexts/SyncContext";
import { TOUR_VERSION } from "./steps";
import App from "../App";

// The first-run tour has its own file rather than a corner of
// App.test.tsx: that file is a shell test and a popover over the shell
// answers to half its queries, and it is also a few lines under the
// 400-line gate.

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    Health: vi.fn(), ListProfiles: vi.fn(), GetSettings: vi.fn(), SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(), SetNavRailVisible: vi.fn(), SetTourSeenVersion: vi.fn(),
    GetSyncState: vi.fn(), ListIssues: vi.fn(), ListSprints: vi.fn(), ListOpenSprints: vi.fn(),
    ListPendingChanges: vi.fn(), GetProfileSetting: vi.fn(), ListProjectTypes: vi.fn(),
    GetSubtaskTypeName: vi.fn(), EventsOn: vi.fn(() => () => {}), BrowserOpenURL: vi.fn(),
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

// synced is the state after a profile has pulled its project in, which is
// the moment the tour is allowed to open.
const synced = { lastSynced: new Date().toISOString(), lastFull: "", lastError: "", issueCount: 60, projectTotal: 60 };

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.Health).mockResolvedValue({ ok: true, error: "", dbPath: "C:/tam.db", sharedPath: "C:/p.db", logPath: "C:/tam.log" });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", tourSeenVersion: 0 });
  vi.mocked(api.GetSyncState).mockResolvedValue(synced);
  vi.mocked(api.SetTourSeenVersion).mockResolvedValue();
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.ListProjectTypes).mockResolvedValue([]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
  vi.mocked(api.SetNavRailVisible).mockResolvedValue();
});

// driver.js appends its popover to the body, outside the tree testing
// library unmounts, so one test's tour would still be on screen during the
// next one's queries.
afterEach(() => {
  document.querySelectorAll(".driver-popover").forEach((n) => n.remove());
});

describe("the first-run tour", () => {
  it("opens once the profile has synced, for somebody who has not seen it", async () => {
    renderApp();
    expect(await screen.findByText("Your connection")).toBeInTheDocument();
  });

  // A tour of an empty backlog explains nothing, so it waits for the sync
  // rather than opening at launch.
  it("waits for the first sync", async () => {
    vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0, projectTotal: 0 });
    renderApp();
    await waitFor(() => expect(api.GetSyncState).toHaveBeenCalled());
    expect(screen.queryByText("Your connection")).toBeNull();
  });

  it("leaves somebody who has been through this version alone", async () => {
    vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", tourSeenVersion: TOUR_VERSION });
    renderApp();
    await waitFor(() => expect(api.GetSyncState).toHaveBeenCalled());
    expect(screen.queryByText("Your connection")).toBeNull();
  });
});
