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
      committed: ["PLAT-409"], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501" }], linked: [], conflicts: [], failures: [], remaining: 0,
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
      committed: [], created: [], linked: [], conflicts: [],
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
    expect(within(dialog).getByText("Commit again to retry the failures.")).toBeInTheDocument();
  });

  it("shows the retry line only when a commit reported failures", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue(rows);
    vi.mocked(api.CommitPendingChanges)
      .mockResolvedValueOnce({
        committed: ["PLAT-409"], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501" }], linked: [], conflicts: [], failures: [], remaining: 0,
      })
      .mockResolvedValueOnce({
        committed: [], created: [], linked: [], conflicts: [],
        failures: [{ key: "PLAT-409", error: "PUT failed: 400 priority is invalid" }],
        remaining: 1,
      });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    await within(dialog).findByText("Last commit: 1 issue pushed, 1 created (TAM-NEW-1 is now PLAT-501).");
    expect(within(dialog).queryByText("Commit again to retry the failures.")).not.toBeInTheDocument();

    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    expect(await within(dialog).findByText("Commit again to retry the failures.")).toBeInTheDocument();
  });

  it("orders a conflict group before a draft group", async () => {
    const user = userEvent.setup();
    const draftAndEditRows: PendingChange[] = [
      { id: 6, entityType: "issue", entityKey: "PLAT-412", field: "storyPoints", beforeVal: "5", afterVal: "8", baseVersion: "v1", createdAt: "2026-09-06T09:00:00Z" },
      rows[0],
    ];
    vi.mocked(api.ListPendingChanges).mockResolvedValue(draftAndEditRows);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [],
      conflicts: [{ key: "PLAT-412", summary: "Promo", remoteVersion: "v2", fields: [{ field: "storyPoints", base: "5", mine: "8", remote: "13" }] }],
      failures: [], remaining: 2,
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    await within(dialog).findByText("Last commit: nothing pushed, 1 held back.");
    const cards = within(dialog).getAllByRole("group");
    expect(cards.map((c) => c.getAttribute("aria-label"))).toEqual(["PLAT-412", "TAM-NEW-1"]);
  });

  it("shows a held issue with base, mine, and remote and resolves it either way", async () => {
    const user = userEvent.setup();
    const conflictRows: PendingChange[] = [
      { id: 5, entityType: "issue", entityKey: "PLAT-412", field: "storyPoints", beforeVal: "5", afterVal: "8", baseVersion: "v1", createdAt: "" },
      { id: 4, entityType: "issue", entityKey: "PLAT-412", field: "labels", beforeVal: "checkout, promo", afterVal: "checkout, promo, q3", baseVersion: "v1", createdAt: "" },
      ...rows,
    ];
    // The first read shows every row; every read after the commit shows only
    // the held issue's rows, as the store would.
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce(conflictRows).mockResolvedValue(conflictRows.slice(0, 2));
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: ["PLAT-409"], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501" }], linked: [],
      conflicts: [{ key: "PLAT-412", summary: "Checkout: apply promo code at payment step", remoteVersion: "2026-09-06T11:00:00Z", fields: [
        { field: "storyPoints", base: "5", mine: "8", remote: "13" },
        { field: "labels", base: "checkout, promo", mine: "checkout, promo, q3", remote: "checkout, promo" },
      ] }],
      failures: [], remaining: 2,
    });
    vi.mocked(api.ResolveConflictOverride).mockResolvedValue();
    vi.mocked(api.ResolveConflictKeepRemote).mockResolvedValue();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (3)" }));
    expect(await within(dialog).findByText("Last commit: 1 issue pushed, 1 created (TAM-NEW-1 is now PLAT-501), 1 held back.")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-412 changed in Jira since you edited it. Resolve it below, then commit again.")).toBeInTheDocument();

    const card = await within(dialog).findByRole("group", { name: "PLAT-412" });
    expect(within(card).getByText("Conflict")).toBeInTheDocument();
    const table = within(card).getByRole("table");
    const bodyRows = within(table).getAllByRole("row").slice(1);
    expect(within(bodyRows[0]).getAllByRole("cell").map((c) => c.textContent)).toEqual(["Story points", "5", "8", "13"]);
    expect(within(bodyRows[1]).getAllByRole("cell").map((c) => c.textContent)).toEqual(["Labels", "checkout, promo", "checkout, promo, q3", "checkout, promo"]);
    expect(within(dialog).getByRole("button", { name: "Commit (0)" })).toBeDisabled();

    await user.click(within(card).getByRole("button", { name: "Override" }));
    await waitFor(() => expect(api.ResolveConflictOverride).toHaveBeenCalledWith("p1", "PLAT-412", "2026-09-06T11:00:00Z"));
    await waitFor(() => expect(within(dialog).queryByText("Conflict")).not.toBeInTheDocument());
    expect(await within(dialog).findByRole("button", { name: "Commit (1)" })).toBeEnabled();
  });

  it("keep remote drops the edits", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 5, entityType: "issue", entityKey: "PLAT-412", field: "storyPoints", beforeVal: "5", afterVal: "8", baseVersion: "v1", createdAt: "" },
    ]);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [],
      conflicts: [{ key: "PLAT-412", summary: "Promo", remoteVersion: "v2", fields: [{ field: "storyPoints", base: "5", mine: "8", remote: "13" }] }],
      failures: [], remaining: 1,
    });
    vi.mocked(api.ResolveConflictKeepRemote).mockResolvedValue();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (1)" }));
    const card = await within(dialog).findByRole("group", { name: "PLAT-412" });
    vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
    await user.click(within(card).getByRole("button", { name: "Keep remote" }));
    await waitFor(() => expect(api.ResolveConflictKeepRemote).toHaveBeenCalledWith("p1", "PLAT-412"));
    expect(await within(dialog).findByText("Nothing pending. Edit an issue or create one and it shows up here.")).toBeInTheDocument();
  });

  it("shows a notice when discarding fails", async () => {
    const user = userEvent.setup();
    vi.mocked(api.DiscardPendingChange).mockRejectedValue(new Error("row is gone"));
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Discard priority on PLAT-409" }));
    expect(await screen.findByText(/row is gone/)).toBeInTheDocument();
  });

  it("shows a link card and counts pushed links in the banner", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce([
      { id: 9, entityType: "link", entityKey: "PLAT-412", field: "Relates|outward|XT-1018", beforeVal: "", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ type: "Relates", direction: "outward", toKey: "XT-1018", toSummary: "Promo code applies discount", toType: "Test" }) },
    ]).mockResolvedValue([]);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({ committed: [], created: [], linked: [{ key: "PLAT-412", toKey: "XT-1018", type: "Relates" }], conflicts: [], failures: [], remaining: 0 });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    const card = await within(dialog).findByRole("group", { name: "PLAT-412" });
    expect(card).toHaveTextContent("Relates (outward) XT-1018 Promo code applies discount");
    expect(within(card).getByRole("button", { name: "Discard link to XT-1018" })).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Commit (1)" }));
    expect(await within(dialog).findByText("Last commit: 1 link pushed.")).toBeInTheDocument();
  });
});

describe("PendingChangesModal board moves", () => {
  const moveRows: PendingChange[] = [
    { id: 12, entityType: "issue_transition", entityKey: "PLAT-412", field: "statusId", beforeVal: "1|To Do", afterVal: "3|In Progress", baseVersion: "v1", createdAt: "" },
    { id: 11, entityType: "issue_sprint", entityKey: "PLAT-412", field: "sprintId", beforeVal: "12|Sprint 12", afterVal: "13|Sprint 13", baseVersion: "v1", createdAt: "" },
    { id: 10, entityType: "issue_rank", entityKey: "PLAT-412", field: "rank", beforeVal: "", afterVal: "before|PLAT-409|1", baseVersion: "v1", createdAt: "" },
  ];

  it("reads each move in words rather than in ids, and discards one of them", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue(moveRows);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    const card = await within(dialog).findByRole("group", { name: "PLAT-412" });
    const rows = within(card).getAllByRole("listitem");
    expect(rows[0]).toHaveTextContent("Status To Do to In Progress");
    expect(rows[1]).toHaveTextContent("Sprint Sprint 12 to Sprint 13");
    expect(rows[2]).toHaveTextContent("Rank before PLAT-409");
    expect(within(card).queryByText("statusId")).not.toBeInTheDocument();

    await user.click(within(card).getByRole("button", { name: "Discard the status move on PLAT-412" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 12));
  });

  it("counts the cards a Commit moved, rather than reporting nothing pushed", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue(moveRows);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [], conflicts: [], failures: [], remaining: 0,
      moved: [
        { key: "PLAT-412", entityType: "issue_transition", target: "In Progress", side: "", satisfied: false },
        { key: "PLAT-412", entityType: "issue_rank", target: "PLAT-409", side: "before", satisfied: false },
      ],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (1)" }));
    expect(await within(dialog).findByText("Last commit: 2 cards moved.")).toBeInTheDocument();
  });

  it("does not count a move Jira had already made as a card this Commit moved", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue(moveRows);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [], conflicts: [], failures: [], remaining: 0,
      moved: [
        { key: "PLAT-412", entityType: "issue_transition", target: "In Progress", side: "", satisfied: false },
        { key: "PLAT-412", entityType: "issue_rank", target: "PLAT-409", side: "before", satisfied: true },
      ],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (1)" }));
    expect(
      await within(dialog).findByText("Last commit: 1 card moved, 1 already in place."),
    ).toBeInTheDocument();
  });

  it("offers an Undo on a board failure, and no retry where a retry cannot help", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue(moveRows);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [], moved: [], conflicts: [], remaining: 3,
      failures: [{
        key: "PLAT-412", entityType: "issue_transition", rowId: 12, retryable: false,
        error: "PLAT-412 cannot reach In Progress; it can reach Done",
      }],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (1)" }));
    expect(await within(dialog).findByText(/cannot reach In Progress/)).toBeInTheDocument();
    expect(within(dialog).queryByText("Commit again to retry the failures.")).not.toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Undo this move" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 12));
  });
});
