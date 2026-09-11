import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { ReportProgress, SyncProgress } from "../api";
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
    SyncBoards: vi.fn(),
    GetSyncState: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
  };
});

let progressListener: ((p: SyncProgress) => void) | null = null;
// The report's frames arrive on their own event, so the provider registers a
// second listener and this is the handle on it.
let reportListener: ((p: ReportProgress) => void) | null = null;
// finishReport resolves the Probe's own report promise, for the same reason
// finishQuiet exists: runReport wraps an arbitrary action rather than one
// bound method.
let finishReport: () => void = () => {};
// finishQuiet resolves the Probe's own quiet-write promise, the way finish
// resolves a mocked api call in the other tests below; runQuietLock wraps
// an arbitrary action rather than one particular bound method, so there is
// no api mock to hang a controllable promise off here.
let finishQuiet: () => void = () => {};

beforeEach(() => {
  vi.clearAllMocks();
  progressListener = null;
  reportListener = null;
  finishQuiet = () => {};
  finishReport = () => {};
  vi.mocked(api.EventsOn).mockImplementation((name: string, cb: (p: SyncProgress) => void) => {
    if (name === "tam:sync-progress") progressListener = cb;
    if (name === "tam:report-progress") reportListener = cb as (p: ReportProgress) => void;
    return () => {};
  });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0 });
});

function Probe() {
  const { status, progress, syncError, canSync, runSync, runBoardsRefresh, runReport, runQuietLock, lastBoards } = useSync();
  const state = useSyncState("p1");
  const [quiet, setQuiet] = React.useState("idle");
  return (
    <div>
      <span data-testid="status">{status}</span>
      <span data-testid="progress">{progress ? `${progress.fetched}/${progress.total}` : "none"}</span>
      <span data-testid="error">{syncError}</span>
      <span data-testid="count">{state.data?.issueCount ?? "?"}</span>
      <span data-testid="boards">{lastBoards ? lastBoards.dropped.join(", ") || "none dropped" : "no pass"}</span>
      <span data-testid="quiet">{quiet}</span>
      <span data-testid="stage">{progress?.stage ?? "none"}</span>
      <button onClick={() => void runSync(false)} disabled={!canSync}>Sync</button>
      <button onClick={() => void runSync(true)}>Full sync</button>
      <button onClick={() => void runBoardsRefresh().catch(() => {})}>Refresh boards</button>
      <button
        onClick={() => {
          setQuiet("running");
          void runQuietLock(() => new Promise<void>((resolve) => { finishQuiet = resolve; }))
            .then(() => setQuiet("done"))
            .catch((e) => setQuiet(String(e)));
        }}
      >
        Quiet write
      </button>
      <button
        onClick={() => {
          void runReport(() => new Promise<void>((resolve) => { finishReport = resolve; })).catch(() => {});
        }}
      >
        Report
      </button>
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
  // A boards refresh holds the same per-profile lock in Go that a sync does,
  // so the UI has to know one is running. It used to be a plain mutation
  // outside this reducer: the shell stayed idle, offered Sync, and Go refused
  // it with "a sync is already running for this profile" on a profile whose
  // status still read "not synced yet".
  it("locks sync while a boards refresh is running", async () => {
    let finish: (v: api.BoardSummary) => void = () => {};
    vi.mocked(api.SyncBoards).mockImplementation(
      () => new Promise<api.BoardSummary>((resolve) => { finish = resolve; }),
    );
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));

    await userEvent.click(screen.getByRole("button", { name: "Refresh boards" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));
    expect(screen.getByRole("button", { name: "Sync" })).toBeDisabled();

    await act(async () => {
      finish({ boards: 2, columns: 6, sprints: 3, cards: 40, dropped: [], unavailable: false, elapsed: "4s" });
    });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
    expect(screen.getByRole("button", { name: "Sync" })).toBeEnabled();
    // The pass lands where the sync's own boards half lands, so the Boards
    // view reports on it through one channel.
    expect(screen.getByTestId("boards")).toHaveTextContent("none dropped");
  });

  it("releases the lock when a boards refresh fails", async () => {
    vi.mocked(api.SyncBoards).mockRejectedValue(new Error("GET failed: 503"));
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));
    await userEvent.click(screen.getByRole("button", { name: "Refresh boards" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
    expect(screen.getByRole("button", { name: "Sync" })).toBeEnabled();
  });

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

  it("keeps the boards pass of the sync that just finished", async () => {
    vi.mocked(api.SyncIssues).mockResolvedValue({
      fetched: 60, upserted: 60, skipped: 0, full: false, elapsed: "1s",
      boards: { boards: 1, columns: 3, sprints: 2, cards: 12, dropped: ["Ops Kanban: 403 Forbidden"], unavailable: false, elapsed: "2s" },
    });
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));
    expect(screen.getByTestId("boards")).toHaveTextContent("no pass");
    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    await waitFor(() => expect(screen.getByTestId("boards")).toHaveTextContent("Ops Kanban: 403 Forbidden"));
  });

  it("has no boards pass to report when the sync did not run one", async () => {
    vi.mocked(api.SyncIssues).mockResolvedValue({ fetched: 1, upserted: 1, skipped: 0, full: false, elapsed: "1s" });
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));
    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
    expect(screen.getByTestId("boards")).toHaveTextContent("no pass");
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

  // runQuietLock is what a sprint create, rename, or delete takes Go's lock
  // through: it must guard against a sync exactly as runBoardsRefresh does,
  // but without reading as one, since none of those three writes is a
  // ceremony every other view needs to announce.
  it("guards a quiet write against a sync in both directions, without ever driving the sync banner", async () => {
    let finishSync: (v: api.SyncSummary) => void = () => {};
    vi.mocked(api.SyncIssues).mockImplementation(
      () => new Promise<api.SyncSummary>((resolve) => { finishSync = resolve; }),
    );
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));

    // A quiet write in flight takes the guard, but the reducer never moves:
    // no "syncing" status, and Sync still reads enabled, exactly as it would
    // if nothing were running at all.
    await userEvent.click(screen.getByRole("button", { name: "Quiet write" }));
    expect(screen.getByTestId("status")).toHaveTextContent("idle");
    expect(screen.getByRole("button", { name: "Sync" })).toBeEnabled();

    // The guard is real even though the banner never moved: a sync attempted
    // while the quiet write is in flight is silently refused, the same
    // no-op every other overlapping SYNC_START already is.
    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    expect(api.SyncIssues).not.toHaveBeenCalled();

    await act(async () => { finishQuiet(); });
    await waitFor(() => expect(screen.getByTestId("quiet")).toHaveTextContent("done"));

    // The other direction: a quiet write attempted while a real sync is
    // running is refused in words the caller can show, not swallowed.
    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));
    await userEvent.click(screen.getByRole("button", { name: "Quiet write" }));
    await waitFor(() => expect(screen.getByTestId("quiet")).toHaveTextContent(/already running/));

    await act(async () => { finishSync({ fetched: 1, upserted: 1, skipped: 0, full: false, elapsed: "1s" }); });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
  });
  // The sharpest thing in the Reports view is this: a report takes the same
  // per-profile lock a sync does, so the shell has to say a report is
  // running rather than staying idle with Sync offered and inert.
  it("locks sync while a report is being built", async () => {
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));

    await userEvent.click(screen.getByRole("button", { name: "Report" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));
    expect(screen.getByRole("button", { name: "Sync" })).toBeDisabled();
    // What the shell says before the first frame lands has to be true too.
    expect(screen.getByTestId("stage")).toHaveTextContent("Building the sprint report");

    await act(async () => { finishReport(); });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
    expect(screen.getByRole("button", { name: "Sync" })).toBeEnabled();
    expect(screen.getByTestId("stage")).toHaveTextContent("none");
  });

  it("says a sprint is being read for a report rather than that a sync is running", async () => {
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));
    await userEvent.click(screen.getByRole("button", { name: "Report" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));

    await act(async () => {
      reportListener?.({ phase: "velocity", sprintId: 10, sprintName: "Sprint 10", fetched: 25, total: 200, done: false });
    });
    expect(screen.getByTestId("stage")).toHaveTextContent("Reading Sprint 10 for the velocity table");
    expect(screen.getByTestId("progress")).toHaveTextContent("25/200");

    await act(async () => { finishReport(); });
  });

  // sprintreport.Progress marks the end of one sprint's fetch, and a report
  // covering six sprints sends six of those. The reducer reads done as "the
  // pull is over" and clears the bar, so a report that forwarded it would go
  // silent after its first sprint.
  it("keeps the bar up when one sprint of a report finishes", async () => {
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));
    await userEvent.click(screen.getByRole("button", { name: "Report" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));

    await act(async () => {
      reportListener?.({ phase: "sprint", sprintId: 11, sprintName: "Sprint 11", fetched: 200, total: 200, done: true });
    });
    expect(screen.getByTestId("stage")).toHaveTextContent("Reading Sprint 11");
    expect(screen.getByTestId("progress")).toHaveTextContent("200/200");

    await act(async () => { finishReport(); });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
  });

  it("refuses a report while a sync is running, and names what is holding the lock", async () => {
    let finishSync: (v: api.SyncSummary) => void = () => {};
    vi.mocked(api.SyncIssues).mockImplementation(
      () => new Promise<api.SyncSummary>((resolve) => { finishSync = resolve; }),
    );
    renderProbe();
    await waitFor(() => expect(screen.getByTestId("count")).toHaveTextContent("0"));

    await userEvent.click(screen.getByRole("button", { name: "Sync" }));
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("syncing"));

    // The Probe's Report button swallows the rejection, so what is asserted
    // is the outcome a user sees: the report never starts and the sync's own
    // stage is still what the shell is showing.
    await userEvent.click(screen.getByRole("button", { name: "Report" }));
    expect(screen.getByTestId("stage")).toHaveTextContent("Starting");

    await act(async () => { finishSync({ fetched: 1, upserted: 1, skipped: 0, full: false, elapsed: "1s" }); });
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("idle"));
  });
});
