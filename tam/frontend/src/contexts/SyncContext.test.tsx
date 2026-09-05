import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { SyncProgress } from "../api";
import { profileBackend } from "../profileBackend";
import { SyncProvider, useSync } from "./SyncContext";
import { useSyncState } from "../queries/issues";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    SyncIssues: vi.fn(),
    GetSyncState: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
  };
});

let progressListener: ((p: SyncProgress) => void) | null = null;

beforeEach(() => {
  vi.clearAllMocks();
  progressListener = null;
  vi.mocked(api.EventsOn).mockImplementation((name: string, cb: (p: SyncProgress) => void) => {
    if (name === "tam:sync-progress") progressListener = cb;
    return () => {};
  });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0 });
});

function Probe() {
  const { status, progress, syncError, canSync, runSync } = useSync();
  const state = useSyncState("p1");
  return (
    <div>
      <span data-testid="status">{status}</span>
      <span data-testid="progress">{progress ? `${progress.fetched}/${progress.total}` : "none"}</span>
      <span data-testid="error">{syncError}</span>
      <span data-testid="count">{state.data?.issueCount ?? "?"}</span>
      <button onClick={() => void runSync(false)} disabled={!canSync}>Sync</button>
      <button onClick={() => void runSync(true)}>Full sync</button>
    </div>
  );
}

// ProfileProvider does not load on mount (the shell calls reload after
// Health), so the test loads the profile the same way.
function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderProbe() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <SyncProvider>
            <Loader />
            <Probe />
          </SyncProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

describe("SyncProvider", () => {
  it("runs a sync, shows progress frames, and refreshes the sync state", async () => {
    let finish: (v: api.SyncSummary) => void = () => {};
    vi.mocked(api.SyncIssues).mockImplementation(
      () => new Promise<api.SyncSummary>((resolve) => { finish = resolve; }),
    );
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));
    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));
    expect(screen.getByRole("button", { name: "Sync" })).toBeDisabled();
    act(() => progressListener?.({ phase: "issues", fetched: 25, total: 60, done: false, stage: "Fetching issues" }));
    expect(screen.getByTestId("progress")).toHaveTextContent("25/60");
    vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "2026-09-05T10:42:00Z", lastFull: "", lastError: "", issueCount: 60 });
    await act(async () => { finish({ fetched: 60, upserted: 60, skipped: 0, full: false, elapsed: "1s" }); });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("60"));
    expect(api.SyncIssues).toHaveBeenCalledWith("p1", false);
  });

  it("runs one sync when the button is clicked twice in the same tick", async () => {
    vi.mocked(api.SyncIssues).mockImplementation(
      () => new Promise<api.SyncSummary>(() => {}),
    );
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));
    const button = screen.getByRole("button", { name: "Sync" });
    // Both clicks land before React re-renders, so the disabled attribute
    // cannot be what stops the second one.
    act(() => {
      fireEvent.click(button);
      fireEvent.click(button);
    });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));
    expect(api.SyncIssues).toHaveBeenCalledTimes(1);
  });

  it("records a failure and returns to idle", async () => {
    vi.mocked(api.SyncIssues).mockRejectedValue(new Error("connection test failed: 401"));
    renderProbe();
    await userEvent.click(screen.getByRole("button", { name: "Full sync" }));
    await waitFor(() => expect(screen.getByTestId("error")).toHaveTextContent("connection test failed: 401"));
    expect(screen.getByTestId("status")).toHaveTextContent("idle");
    expect(api.SyncIssues).toHaveBeenCalledWith("p1", true);
    // The shared notice dialog is an alertdialog, whatever the tone.
    expect(screen.getByRole("alertdialog")).toHaveTextContent("Sync failed");
  });
});
