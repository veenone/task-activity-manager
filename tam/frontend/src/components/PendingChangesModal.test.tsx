import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { PendingChange } from "../api";
import { profileBackend } from "../profileBackend";
import { SyncProvider } from "../contexts/SyncContext";
import { PendingChangesModal } from "./PendingChangesModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    SyncIssues: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
    ListPendingChanges: vi.fn(),
    DiscardPendingChange: vi.fn(),
    DiscardAllPendingChanges: vi.fn(),
    CommitPendingChanges: vi.fn(),
    ResolveConflictOverride: vi.fn(),
    ResolveConflictKeepRemote: vi.fn(),
  };
});

const rows: PendingChange[] = [
  { id: 3, entityType: "issue_create", entityKey: "TAM-NEW-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "2026-09-06T10:00:00Z",
    afterVal: JSON.stringify({ type: "task", summary: "Add a retry to the payment webhook consumer", description: "", priority: "Medium", labels: [], assignee: "M. Ortiz", storyPoints: 3, extra: {} }) },
  { id: 2, entityType: "issue", entityKey: "PLAT-409", field: "priority", beforeVal: "Medium", afterVal: "High", baseVersion: "v1", createdAt: "2026-09-06T09:59:00Z" },
  { id: 1, entityType: "issue", entityKey: "PLAT-409", field: "assignee", beforeVal: "", afterVal: "M. Ortiz", baseVersion: "v1", createdAt: "2026-09-06T09:58:00Z" },
];

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderModal(onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <SyncProvider>
            <Loader />
            <PendingChangesModal onClose={onClose} />
          </SyncProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
  return onClose;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListPendingChanges).mockResolvedValue(rows);
  vi.mocked(api.DiscardPendingChange).mockResolvedValue();
  vi.mocked(api.DiscardAllPendingChanges).mockResolvedValue(3);
});

describe("PendingChangesModal", () => {
  it("groups the journal by issue, drafts first, with before and after per field", async () => {
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    expect(await within(dialog).findByText("3 changes on 2 issues, 1 of them new")).toBeInTheDocument();
    const cards = within(dialog).getAllByRole("group");
    expect(cards).toHaveLength(2);
    expect(within(cards[0]).getByText("TAM-NEW-1")).toBeInTheDocument();
    expect(within(cards[0]).getByText("Draft")).toBeInTheDocument();
    expect(within(cards[0]).getByText("Add a retry to the payment webhook consumer")).toBeInTheDocument();
    expect(within(cards[0]).getByText("New Task in PLAT, priority Medium, assignee M. Ortiz, 3 points")).toBeInTheDocument();
    expect(within(cards[1]).getByText("PLAT-409")).toBeInTheDocument();
    const rowsOfCard = within(cards[1]).getAllByRole("listitem");
    expect(rowsOfCard[0]).toHaveTextContent("Priority Medium to High");
    expect(rowsOfCard[1]).toHaveTextContent("Assignee (none) to M. Ortiz");
    expect(within(dialog).getByRole("button", { name: "Commit (2)" })).toBeEnabled();
  });

  it("discards one row and all rows", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Discard priority on PLAT-409" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 2));
    expect(api.ListPendingChanges).toHaveBeenCalledTimes(2);

    await user.click(within(dialog).getByRole("button", { name: "Discard all" }));
    const confirm = await screen.findByRole("alertdialog");
    await user.click(within(confirm).getByRole("button", { name: "Discard all" }));
    await waitFor(() => expect(api.DiscardAllPendingChanges).toHaveBeenCalledWith("p1"));
  });

  it("commits and shows the result banner with the key mapping", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: ["PLAT-409"], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501" }], conflicts: [], failures: [], remaining: 0,
    });
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce(rows).mockResolvedValue([]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    await waitFor(() => expect(api.CommitPendingChanges).toHaveBeenCalledWith("p1"));
    expect(await within(dialog).findByText("Last commit: 1 issue pushed, 1 created (TAM-NEW-1 is now PLAT-501).")).toBeInTheDocument();
    expect(await within(dialog).findByText("Nothing pending. Edit an issue or create one and it shows up here.")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Commit (0)" })).toBeDisabled();
  });

  it("names failures in the banner and keeps their rows", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], conflicts: [],
      failures: [{ key: "PLAT-409", error: "PUT failed: 400 priority is invalid" }, { key: "TAM-NEW-1", error: "Severity is required" }],
      remaining: 3,
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    expect(await within(dialog).findByText("Last commit: nothing pushed, 2 failed.")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-409: PUT failed: 400 priority is invalid")).toBeInTheDocument();
    expect(within(dialog).getByText("TAM-NEW-1: Severity is required")).toBeInTheDocument();
    expect(within(dialog).getAllByRole("group")).toHaveLength(2);
  });
});
