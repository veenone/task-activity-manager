import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import * as api from "./api";
import App from "./App";
import { profileBackend } from "./profileBackend";
import { ViewProvider } from "./nav";
import { ModalProvider } from "./modals";
import { SyncProvider } from "./contexts/SyncContext";

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
    SyncIssues: vi.fn(),
    GetSyncState: vi.fn(),
    ListIssues: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
    ListSprints: vi.fn(),
    GetProfileSetting: vi.fn(),
    SetProfileSetting: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
    BrowserOpenURL: vi.fn(),
    ListPendingChanges: vi.fn(),
    DiscardPendingChange: vi.fn(),
    DiscardAllPendingChanges: vi.fn(),
    CommitPendingChanges: vi.fn(),
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
  vi.mocked(api.Health).mockResolvedValue({
    ok: true, error: "", dbPath: "C:/tam.db", sharedPath: "C:/profiles.db", logPath: "C:/tam.log",
  });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0 });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
});

describe("App shell", () => {
  it("shows the title, the demo chip, and the active profile", async () => {
    renderApp();
    expect(screen.getByText("Task Activity Manager")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: /profile/i })).toHaveValue("p1");
  });

  it("switches views from the nav rail and names the phase", async () => {
    renderApp();
    await userEvent.click(screen.getByRole("button", { name: "Epics" }));
    expect(screen.getByRole("heading", { name: "Epics" })).toBeInTheDocument();
    expect(screen.getByText(/arrives in Phase 2/)).toBeInTheDocument();
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

  it("shows the Commit chip with the pending count and opens the dialog", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 1, entityType: "issue", entityKey: "PLAT-409", field: "priority", beforeVal: "Medium", afterVal: "High", baseVersion: "v1", createdAt: "" },
      { id: 2, entityType: "issue", entityKey: "PLAT-409", field: "assignee", beforeVal: "", afterVal: "M. Ortiz", baseVersion: "v1", createdAt: "" },
      { id: 3, entityType: "issue_create", entityKey: "TAM-NEW-1", field: "create", beforeVal: "", afterVal: '{"type":"task","summary":"x","description":"","priority":"","labels":[],"assignee":"","storyPoints":null,"extra":{}}', baseVersion: "", createdAt: "" },
    ]);
    renderApp();
    const chip = await screen.findByRole("button", { name: "3 pending changes: Commit" });
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
