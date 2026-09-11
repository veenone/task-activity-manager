import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import * as api from "./api";
import App from "./App";
import { profileBackend } from "./profileBackend";
import { ViewProvider } from "./nav";
import { ModalProvider } from "./modals";
import { SyncProvider } from "./contexts/SyncContext";

// The native menu is TAM's primary navigation, and it reaches the frontend
// only as Wails events. This stands in for the Wails event bus so a test can
// fire what the menu fires. vi.hoisted, because the vi.mock factory below is
// hoisted above every other binding in the file.
const menuBus = vi.hoisted(() => {
  const listeners = new Map<string, Set<(...args: never[]) => void>>();
  return {
    on(name: string, cb: (...args: never[]) => void) {
      const set = listeners.get(name) ?? new Set();
      set.add(cb);
      listeners.set(name, set);
      return () => set.delete(cb);
    },
    async emit(name: string, ...args: never[]) {
      await act(async () => {
        for (const cb of [...(listeners.get(name) ?? [])]) cb(...args);
      });
    },
    reset() {
      listeners.clear();
    },
  };
});

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    Health: vi.fn(),
    GetDiagnostics: vi.fn(),
    ListProfiles: vi.fn(),
    CreateProfile: vi.fn(),
    DeleteProfile: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    SetNavRailVisible: vi.fn(),
    SyncIssues: vi.fn(),
    GetSyncState: vi.fn(),
    ListIssues: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
    ListSprints: vi.fn(),
    GetEpicTree: vi.fn(),
    ListEpics: vi.fn(),
    GetProfileSetting: vi.fn(),
    ListBoards: vi.fn(),
    ListBoardSprints: vi.fn(),
    ListBoardSprintDetails: vi.fn(),
    GetBoard: vi.fn(),
    GetSprintReport: vi.fn(),
    CancelSprintReport: vi.fn(),
    SyncBoards: vi.fn(),
    SetProfileSetting: vi.fn(),
    EventsOn: vi.fn(menuBus.on),
    BrowserOpenURL: vi.fn(),
    ListPendingChanges: vi.fn(),
    DiscardPendingChange: vi.fn(),
    DiscardAllPendingChanges: vi.fn(),
    CommitPendingChanges: vi.fn(),
    PreviewImport: vi.fn(),
    AutoMapImport: vi.fn(),
    ImportIssues: vi.fn(),
    SaveImportTemplate: vi.fn(),
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
  menuBus.reset();
  vi.mocked(api.SetNavRailVisible).mockResolvedValue();
  vi.mocked(api.Health).mockResolvedValue({
    ok: true, error: "", dbPath: "C:/tam.db", sharedPath: "C:/profiles.db", logPath: "C:/tam.log",
  });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", showNavRail: false });
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0 });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.GetEpicTree).mockResolvedValue({ epics: [], orphans: [], truncated: false });
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.ListBoards).mockResolvedValue([]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue([]);
  vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([]);
  // The Reports view cancels whatever report it left running when it goes
  // away, whether or not it ever started one, so the shell's own test needs
  // the binding stood in for even though no board here has a sprint.
  vi.mocked(api.CancelSprintReport).mockResolvedValue();
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
});

describe("App shell", () => {
  it("shows the title, the demo chip, and the active profile", async () => {
    renderApp();
    expect(screen.getByText("Task Activity Manager")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: /profile/i })).toHaveValue("p1");
  });

  it("hides the nav rail until the stored setting asks for it", async () => {
    renderApp();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    expect(screen.queryByRole("navigation", { name: "Navigation rail" })).not.toBeInTheDocument();
    // The view tabs still offer every view; only the rail is gone.
    expect(within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name: "Boards" })).toBeInTheDocument();
  });

  it("shows the nav rail when the stored setting asks for it", async () => {
    vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", showNavRail: true });
    renderApp();
    expect(await screen.findByRole("navigation", { name: "Navigation rail" })).toBeInTheDocument();
  });

  it("switches views from the View menu", async () => {
    renderApp();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    await menuBus.emit("menu:view", "boards" as never);
    expect(screen.getByRole("region", { name: /Boards/ })).toBeInTheDocument();
    expect(
      await screen.findByText("This project has no boards in Jira, or the sync has not run"),
    ).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name: "Boards" }))
      .toHaveAttribute("aria-current", "page");
  });

  it("switches to the Sprints view, which the View menu now carries too", async () => {
    renderApp();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    await menuBus.emit("menu:view", "sprints" as never);
    expect(screen.getByRole("region", { name: "Sprints" })).toBeInTheDocument();
    expect(
      await screen.findByText("No scrum board has been synced for this project, so there are no sprints to show."),
    ).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name: "Sprints" }))
      .toHaveAttribute("aria-current", "page");
  });

  it("ignores a view id the frontend does not know", async () => {
    renderApp();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    await menuBus.emit("menu:view", "nonesuch" as never);
    expect(screen.getByRole("region", { name: "Backlog" })).toBeInTheDocument();
  });

  it("shows and hides the nav rail from the View menu's checkbox", async () => {
    renderApp();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    await menuBus.emit("menu:nav-rail", true as never);
    expect(screen.getByRole("navigation", { name: "Navigation rail" })).toBeInTheDocument();
    await menuBus.emit("menu:nav-rail", false as never);
    expect(screen.queryByRole("navigation", { name: "Navigation rail" })).not.toBeInTheDocument();
  });

  it("hides the rail from its own close button and persists that", async () => {
    vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", showNavRail: true });
    renderApp();
    await userEvent.click(await screen.findByRole("button", { name: "Hide the navigation rail" }));
    expect(screen.queryByRole("navigation", { name: "Navigation rail" })).not.toBeInTheDocument();
    // Persisting rebuilds the native menu, so the View menu's tick follows.
    await waitFor(() => expect(api.SetNavRailVisible).toHaveBeenCalledWith(false));
  });

  it("switches views from the view tabs", async () => {
    renderApp();
    const tabs = screen.getByRole("navigation", { name: "Views" });
    await userEvent.click(within(tabs).getByRole("button", { name: "Boards" }));
    expect(screen.getByRole("region", { name: /Boards/ })).toBeInTheDocument();
    expect(
      await screen.findByText("This project has no boards in Jira, or the sync has not run"),
    ).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name: "Boards" }))
      .toHaveAttribute("aria-current", "page");
  });

  it("switches views from the nav rail, and Reports is the real view now", async () => {
    vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", showNavRail: true });
    renderApp();
    const rail = await screen.findByRole("navigation", { name: "Navigation rail" });
    await userEvent.click(within(rail).getByRole("button", { name: "Reports" }));
    expect(screen.getByRole("region", { name: /Reports/ })).toBeInTheDocument();
    // No board is synced in this fixture, which is ReportsView's own empty
    // state rather than the placeholder that used to stand here.
    expect(await screen.findByText(/No scrum board has been synced for this project/)).toBeInTheDocument();
    expect(screen.queryByText(/arrives in Phase/)).not.toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name: "Reports" }))
      .toHaveAttribute("aria-current", "page");
  });

  it("names the phase of a view a later plan delivers", async () => {
    vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", showNavRail: true });
    renderApp();
    const rail = await screen.findByRole("navigation", { name: "Navigation rail" });
    await userEvent.click(within(rail).getByRole("button", { name: "Rituals" }));
    expect(screen.getByText(/arrives in Phase 5/)).toBeInTheDocument();
  });

  it("renders the epic tree when the View menu opens Epics", async () => {
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        {
          issue: { key: "PLAT-1", id: "1", project: "PLAT", type: "epic", summary: "Checkout revamp", status: "In Progress", assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "", created: "", updated: "" },
          children: [],
          total: 0,
          done: 0,
          points: 0,
          donePoints: 0,
        },
      ],
      orphans: [],
      truncated: false,
    });
    renderApp();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    await menuBus.emit("menu:view", "epics" as never);
    expect(screen.getByRole("region", { name: "Epics" })).toBeInTheDocument();
    expect(await screen.findByText("Checkout revamp")).toBeInTheDocument();
    expect(screen.getByRole("tree", { name: "Epics" })).toBeInTheDocument();
  });

  it("says so when the profiles cannot be loaded", async () => {
    vi.mocked(api.ListProfiles).mockRejectedValue(new Error("profiles.db is locked"));
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    renderApp();
    await waitFor(() =>
      expect(
        screen.getByText(/Profiles could not be loaded: profiles.db is locked/),
      ).toBeInTheDocument(),
    );
    spy.mockRestore();
  });

  it("syncs from the topbar and refreshes the status bar", async () => {
    vi.mocked(api.SyncIssues).mockImplementation(async () => {
      vi.mocked(api.GetSyncState).mockResolvedValue({
        lastSynced: new Date().toISOString(), lastFull: "", lastError: "", issueCount: 60,
      });
      return { fetched: 60, upserted: 60, skipped: 0, full: false, elapsed: "1s" };
    });
    renderApp();
    await waitFor(() => expect(screen.getByTestId("sync-summary")).toHaveTextContent("Not synced yet"));
    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Sync changes" }));
    await waitFor(() => expect(screen.getByTestId("sync-summary")).toHaveTextContent(/60 issues, last synced today/));
    expect(api.SyncIssues).toHaveBeenCalledWith("p1", false);
  });

  it("shows the pending button beside Sync and opens the dialog", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 1, entityType: "issue", entityKey: "PLAT-409", field: "priority", beforeVal: "Medium", afterVal: "High", baseVersion: "v1", createdAt: "" },
      { id: 2, entityType: "issue", entityKey: "PLAT-409", field: "assignee", beforeVal: "", afterVal: "M. Ortiz", baseVersion: "v1", createdAt: "" },
      { id: 3, entityType: "issue_create", entityKey: "TAM-NEW-1", field: "create", beforeVal: "", afterVal: '{"type":"task","summary":"x","description":"","priority":"","labels":[],"assignee":"","storyPoints":null,"extra":{}}', baseVersion: "", createdAt: "" },
    ]);
    renderApp();
    const chip = await screen.findByRole("button", { name: "3 pending" });
    await user.click(chip);
    expect(await screen.findByRole("dialog", { name: "Pending changes" })).toBeInTheDocument();
  });

  it("shows the last sync error in the status bar", async () => {
    vi.mocked(api.GetSyncState).mockResolvedValue({
      lastSynced: "2026-09-05T10:42:00Z", lastFull: "", lastError: "jira: 502 Bad Gateway", issueCount: 12,
    });
    renderApp();
    await waitFor(() => expect(screen.getByTestId("sync-error")).toHaveTextContent("jira: 502 Bad Gateway"));
  });

  it("surfaces a startup failure instead of a blank page", async () => {
    vi.mocked(api.Health).mockResolvedValue({
      ok: false, error: "open local store: disk full", dbPath: "", sharedPath: "", logPath: "",
    });
    renderApp();
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("disk full"),
    );
  });
});
