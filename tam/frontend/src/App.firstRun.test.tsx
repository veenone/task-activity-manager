import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import * as api from "./api";
import { profileBackend } from "./profileBackend";
import { ModalProvider } from "./modals";
import { ViewProvider } from "./nav";
import { SyncProvider } from "./contexts/SyncContext";
import { TOUR_VERSION } from "./tour/steps";
import App from "./App";

// A new install with no profile (#526), and the loading line the Backlog
// shows before its first page arrives. Its own file because App.test.tsx
// sits at the 400-line gate.

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
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

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.Health).mockResolvedValue({ ok: true, error: "", dbPath: "C:/tam.db", sharedPath: "C:/p.db", logPath: "C:/tam.log" });
  vi.mocked(api.ListProfiles).mockResolvedValue([]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "", theme: "light", tourSeenVersion: TOUR_VERSION });
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0, projectTotal: 0 });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.ListProjectTypes).mockResolvedValue([]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
  vi.mocked(api.SetNavRailVisible).mockResolvedValue();
});

describe("first run", () => {
  it("shows a first-run screen in place of the views when there is no profile", async () => {
    renderApp();
    const firstRun = await screen.findByRole("region", { name: "Get started" });
    expect(firstRun).toHaveTextContent("Connect to your Jira Data Center project to get started.");
    // The views each had an empty state of their own that blamed the sync.
    expect(screen.queryByRole("region", { name: "Backlog" })).not.toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Views" })).not.toBeInTheDocument();
  });

  it("opens Manage Profiles from the first-run screen", async () => {
    const user = userEvent.setup();
    renderApp();
    await user.click(await screen.findByRole("button", { name: "Create a profile" }));
    expect(await screen.findByRole("dialog", { name: "Manage Profiles" })).toBeInTheDocument();
  });

  it("draws the views, not the first-run screen, once a profile exists", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([
      { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
    ]);
    renderApp();
    expect(await screen.findByRole("region", { name: "Backlog" })).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Get started" })).not.toBeInTheDocument();
  });

  // Before the profiles are read there is no telling a new install from one
  // that is still starting, so neither the first-run screen nor a view's
  // empty state is drawn.
  it("does not flash the first-run screen while the profiles are being read", async () => {
    vi.mocked(api.ListProfiles).mockReturnValue(new Promise(() => {}));
    renderApp();
    expect(await screen.findByRole("main")).toHaveTextContent("Reading the profiles");
    expect(screen.queryByRole("region", { name: "Get started" })).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Backlog" })).not.toBeInTheDocument();
  });

  it("says the Backlog is reading while its first page loads", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([
      { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
    ]);
    vi.mocked(api.ListIssues).mockReturnValue(new Promise(() => {}));
    renderApp();
    const backlog = await screen.findByRole("region", { name: "Backlog" });
    await waitFor(() => expect(backlog).toHaveTextContent("Reading the issues"));
  });
});
