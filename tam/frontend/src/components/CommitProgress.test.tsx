import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { CommitProgress, CommitResult, PendingChange } from "../api";
import { profileBackend } from "../profileBackend";
import { SyncProvider } from "../contexts/SyncContext";
import { PendingChangesModal } from "./PendingChangesModal";
import { commitStageLine } from "./CommitProgress";

const listeners = new Map<string, (p: CommitProgress) => void>();

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    EventsOn: vi.fn(),
    ListPendingChanges: vi.fn(),
    CommitPendingChanges: vi.fn(),
    ListUnpushableEdits: vi.fn(),
  };
});

const rows: PendingChange[] = [
  { id: 2, entityType: "issue", entityKey: "PLAT-409", field: "priority", beforeVal: "Medium", afterVal: "High", baseVersion: "v1", createdAt: "2026-09-06T09:59:00Z" },
  { id: 1, entityType: "issue", entityKey: "PLAT-410", field: "summary", beforeVal: "a", afterVal: "b", baseVersion: "v1", createdAt: "2026-09-06T09:58:00Z" },
];

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

beforeEach(() => {
  vi.clearAllMocks();
  listeners.clear();
  vi.mocked(api.EventsOn).mockImplementation((name: string, cb: (p: CommitProgress) => void) => {
    listeners.set(name, cb);
    return () => listeners.delete(name);
  });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListPendingChanges).mockResolvedValue(rows);
  vi.mocked(api.ListUnpushableEdits).mockResolvedValue([]);
});

describe("commit progress in the Pending changes dialog", () => {
  it("draws done of total and the issue being pushed, then gives way to the result", async () => {
    let finish: (r: CommitResult) => void = () => {};
    vi.mocked(api.CommitPendingChanges).mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(
      <QueryClientProvider client={createQueryClient()}>
        <DialogProvider>
          <ProfileProvider backend={profileBackend}>
            <SyncProvider>
              <Loader />
              <PendingChangesModal onClose={vi.fn()} />
            </SyncProvider>
          </ProfileProvider>
        </DialogProvider>
      </QueryClientProvider>,
    );
    const dialog = await screen.findByRole("dialog", { name: "Pending changes (2)" });
    const commit = within(dialog).getByRole("button", { name: "Commit (2)" });
    await waitFor(() => expect(commit).toBeEnabled());
    await userEvent.click(commit);

    const bar = await within(dialog).findByRole("progressbar", { name: "Commit progress" });
    expect(within(dialog).getByText("Starting the commit")).toBeInTheDocument();

    act(() => listeners.get(api.COMMIT_PROGRESS_EVENT)!({ done: 1, total: 2, stage: "edits", key: "PLAT-410", finished: false }));
    expect(bar).toHaveAttribute("aria-valuenow", "1");
    expect(bar).toHaveAttribute("aria-valuetext", "1 of 2 changes");
    expect(within(dialog).getByText("Committing edits, PLAT-410")).toBeInTheDocument();

    act(() => listeners.get(api.COMMIT_PROGRESS_EVENT)!({ done: 2, total: 2, stage: "worklogs", key: "", finished: true }));
    expect(bar).toHaveAttribute("aria-valuenow", "2");

    vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
    await act(async () => finish({ committed: ["PLAT-409", "PLAT-410"], created: [], linked: [], conflicts: [], failures: [], remaining: 0 }));
    await waitFor(() => expect(within(dialog).queryByRole("progressbar", { name: "Commit progress" })).not.toBeInTheDocument());
  });

  it("names the phase alone when it pushes no single issue", () => {
    expect(commitStageLine({ done: 3, total: 9, stage: "board moves", key: "", finished: false })).toBe("Committing board moves");
    expect(commitStageLine({ done: 9, total: 9, stage: "worklogs", key: "", finished: true })).toBe("Sent, finishing up");
  });
});
