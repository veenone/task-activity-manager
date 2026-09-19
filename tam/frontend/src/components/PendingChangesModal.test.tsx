import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { PendingChange } from "../api";
import { groupPending } from "../queries/pending";
import { profileBackend } from "../profileBackend";
import { SyncProvider } from "../contexts/SyncContext";
import { PendingChangesModal, countPushable, sprintChangeLine } from "./PendingChangesModal";

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
    ListUnpushableEdits: vi.fn(),
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
  vi.mocked(api.ListUnpushableEdits).mockResolvedValue([]);
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

  // Issue #52: an edit journalled for a field Jira will not take is named
  // here and kept. Discarding it stays the user's own act.
  it("names an edit Jira will not take and leaves the row alone", async () => {
    vi.mocked(api.ListUnpushableEdits).mockResolvedValue([{ id: 2, key: "PLAT-409", field: "priority" }]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    const card = (await within(dialog).findAllByRole("group"))[1];
    const note = await within(card).findByText(/Jira will not take Priority on PLAT-409/);
    // The explanation comes before the action it qualifies, in the order a
    // screen reader walks the row rather than only where the eye lands.
    const discard = within(card).getByRole("button", { name: "Discard priority on PLAT-409" });
    expect(note.compareDocumentPosition(discard) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // Not the held treatment: an unpushable row wears no Waiting chip, and
    // borrowing the colour that always comes with one would say it does.
    expect(note).not.toHaveClass("pending-held");
    expect(within(card).queryByText("Waiting")).not.toBeInTheDocument();
    // The row is still there, with the value the user typed, and still only
    // theirs to discard.
    expect(within(card).getAllByRole("listitem")[0]).toHaveTextContent("Priority Medium to High");
    expect(within(card).getByRole("button", { name: "Discard priority on PLAT-409" })).toBeEnabled();
    expect(api.DiscardPendingChange).not.toHaveBeenCalled();
    // The other row says nothing: Jira takes it.
    expect(within(card).queryByText(/Jira will not take Assignee/)).not.toBeInTheDocument();
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

  // A create that could not confirm one of the draft's extra fields leaves it
  // out. The user typed that value, so the banner has to say which field did
  // not go: the issue exists in Jira without it, and nothing else on screen
  // would ever show that.
  it("names the fields a create left out", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501", leftOut: ["Team (customfield_10253)", "Squad (customfield_10777)"] }],
      linked: [], conflicts: [], failures: [], remaining: 0,
    });
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce(rows).mockResolvedValue([]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    await waitFor(() => expect(api.CommitPendingChanges).toHaveBeenCalledWith("p1"));
    expect(
      await within(dialog).findByText(
        "PLAT-501 was created without Team (customfield_10253) and Squad (customfield_10777). TAM could not confirm they are on the create screen, so it left them out. Set them in Jira if the issue needs them.",
      ),
    ).toBeInTheDocument();
  });

  it("says nothing about left out fields when a create sent everything", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501", leftOut: [] }],
      linked: [], conflicts: [], failures: [], remaining: 0,
    });
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce(rows).mockResolvedValue([]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    await waitFor(() => expect(api.CommitPendingChanges).toHaveBeenCalledWith("p1"));
    expect(within(dialog).queryByText(/was created without/)).not.toBeInTheDocument();
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
      { id: 3, entityType: "issue_transition", entityKey: "PLAT-412", field: "statusId", beforeVal: "1|To Do", afterVal: "3|In Progress", baseVersion: "v1", createdAt: "" },
      ...rows,
    ];
    // The first read shows every row; every read after the commit shows only
    // the held issue's rows, as the store would.
    vi.mocked(api.ListPendingChanges).mockResolvedValueOnce(conflictRows).mockResolvedValue(conflictRows.slice(0, 3));
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: ["PLAT-409"], created: [{ tempKey: "TAM-NEW-1", key: "PLAT-501" }], linked: [],
      conflicts: [{ key: "PLAT-412", summary: "Checkout: apply promo code at payment step", remoteVersion: "2026-09-06T11:00:00Z", fields: [
        { field: "storyPoints", base: "5", mine: "8", remote: "13" },
        { field: "labels", base: "checkout, promo", mine: "checkout, promo, q3", remote: "checkout, promo" },
        { field: "statusId", base: "To Do", mine: "In Progress", remote: "Done" },
      ] }],
      failures: [], remaining: 3,
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
    // A held board write reads as the move it is. Without a label of its
    // own the row printed the journal's raw field name, "statusId".
    expect(within(bodyRows[2]).getAllByRole("cell").map((c) => c.textContent)).toEqual(["Status", "To Do", "In Progress", "Done"]);
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

  it("shows a draft sprint as its own card, first, and discards it", async () => {
    const user = userEvent.setup();
    const sprintRow: PendingChange = {
      id: 9, entityType: "sprint_create", entityKey: "-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "2026-09-15T10:00:00Z",
      afterVal: JSON.stringify({ boardId: 1, boardName: "PLAT Scrum", name: "Sprint 15", goal: "", startDate: "2026-09-16T09:00:00.000+0000", endDate: "2026-09-30T09:00:00.000+0000" }),
    };
    vi.mocked(api.ListPendingChanges).mockResolvedValue([sprintRow, ...rows]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    expect(await within(dialog).findByText("4 changes on 2 issues, 1 of them new, and 1 new sprint")).toBeInTheDocument();
    const cards = within(dialog).getAllByRole("group");
    expect(cards.map((c) => c.getAttribute("aria-label"))).toEqual(["Sprint 15", "TAM-NEW-1", "PLAT-409"]);
    expect(within(cards[0]).getByText("Draft sprint")).toBeInTheDocument();
    expect(within(cards[0]).getByText("New sprint on PLAT Scrum, 2026-09-16 to 2026-09-30")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Commit (3)" })).toBeEnabled();
    await user.click(within(cards[0]).getByRole("button", { name: "Discard Sprint 15" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 9));
  });

  it("shows a start and a completion in words, a draft's start on its own card, and discards each", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 42, entityType: "sprint_complete", entityKey: "12", field: "complete", beforeVal: "", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ boardId: 1, name: "Sprint 12", moveTo: "13", moveToName: "Sprint 13" }) },
      { id: 41, entityType: "sprint_start", entityKey: "13", field: "start", beforeVal: "", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ boardId: 1, name: "Sprint 13", goal: "", startDate: "2026-09-14T09:00:00.000+0000", endDate: "2026-09-28T09:00:00.000+0000" }) },
      { id: 40, entityType: "sprint_start", entityKey: "-1", field: "start", beforeVal: "", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ boardId: 1, name: "Sprint 15", goal: "", startDate: "2026-09-29T09:00:00.000+0000", endDate: "2026-10-12T09:00:00.000+0000" }) },
      { id: 39, entityType: "sprint_create", entityKey: "-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ boardId: 1, boardName: "PLAT Scrum", name: "Sprint 15", goal: "", startDate: "2026-09-29T09:00:00.000+0000", endDate: "2026-10-12T09:00:00.000+0000" }) },
    ]);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [], conflicts: [], failures: [], remaining: 0,
      sprintsChanged: ["Sprint 12 completed, 45 unfinished cards moved to Sprint 13"],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    expect(await within(dialog).findByText("4 changes: 1 new sprint, 2 sprint changes")).toBeInTheDocument();
    const cards = within(dialog).getAllByRole("group");
    expect(cards.map((c) => c.getAttribute("aria-label"))).toEqual(["Sprint 15", "Sprint 12", "Sprint 13"]);
    expect(cards[0]).toHaveTextContent("Start sprint Sprint 15, 2026-09-29 to 2026-10-12");
    expect(cards[2]).toHaveTextContent("Start sprint Sprint 13, 2026-09-14 to 2026-09-28");
    expect(cards[2]).toHaveTextContent("Commit starts it in Jira.");
    expect(cards[1]).toHaveTextContent("Complete sprint Sprint 12, unfinished cards to Sprint 13");
    expect(cards[1]).toHaveTextContent("Commit works out the unfinished cards from Jira again");
    await user.click(within(cards[0]).getByRole("button", { name: "Discard the change to Sprint 15" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 40));
    await user.click(within(cards[1]).getByRole("button", { name: "Discard the change to Sprint 12" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 42));
    await user.click(within(dialog).getByRole("button", { name: "Commit (3)" }));
    expect(await within(dialog).findByText("Last commit: 1 sprint change pushed (Sprint 12 completed, 45 unfinished cards moved to Sprint 13).")).toBeInTheDocument();
  });

  // The number on the button is what Commit will deliver. A refused field
  // fails that issue's whole update, so an issue carrying one delivers
  // nothing and must not be counted. Tested where the number is worked out.
  describe("countPushable", () => {
    const groups = groupPending(rows);

    it("counts every group when nothing is known to be refused", () => {
      expect(countPushable(groups, new Set(), new Set())).toBe(2);
    });

    it("leaves out an issue carrying a row Jira will refuse", () => {
      expect(countPushable(groups, new Set(), new Set(["PLAT-409"]))).toBe(1);
    });

    it("still counts an issue nothing is known about", () => {
      // The refusal set only ever holds issues whose screen TAM has read.
      // An issue missing from it is unknown, not known to be fine, and an
      // unknown issue may well push, so it stays in the number.
      expect(countPushable(groups, new Set(), new Set(["OTHER-1"]))).toBe(2);
    });

    it("does not count a conflicted issue twice out", () => {
      expect(countPushable(groups, new Set(["PLAT-409"]), new Set(["PLAT-409"]))).toBe(1);
    });

    it("leaves a sprint or board alone when an issue shares its key", () => {
      // A sprint group's key is its numeric id and a draft board's is a
      // negative one, so only an issue group may be matched by issue key.
      const sprintRows: PendingChange[] = [
        { id: 9, entityType: "sprint_start", entityKey: "12", field: "start", beforeVal: "", baseVersion: "", createdAt: "",
          afterVal: JSON.stringify({ boardId: 1, name: "Sprint 12", startDate: "2026-09-14T09:00:00.000+0000", endDate: "2026-09-28T09:00:00.000+0000" }) },
      ];
      expect(countPushable(groupPending(sprintRows), new Set(), new Set(["12"]))).toBe(1);
    });
  });

  it("drops a refused issue out of the Commit count and says why", async () => {
    vi.mocked(api.ListUnpushableEdits).mockResolvedValue([{ id: 2, key: "PLAT-409", field: "priority" }]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    // Three rows on two issues, one of which Jira will refuse: the draft is
    // all Commit can deliver.
    await waitFor(() => expect(within(dialog).getByRole("button", { name: "Commit (1)" })).toBeInTheDocument());
    expect(await within(dialog).findByText(/1 issue is not in that count/)).toBeInTheDocument();
    // The work itself is untouched: both cards are still there, and the
    // refused row is still the user's to discard.
    expect(within(dialog).getAllByRole("group")).toHaveLength(2);
    expect(within(dialog).getByRole("button", { name: "Discard priority on PLAT-409" })).toBeEnabled();
  });

  it("accounts for a Commit count of zero rather than leaving an inert button", async () => {
    vi.mocked(api.ListPendingChanges).mockResolvedValue(rows.filter((r) => r.entityKey === "PLAT-409"));
    vi.mocked(api.ListUnpushableEdits).mockResolvedValue([{ id: 2, key: "PLAT-409", field: "priority" }]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    const commit = await within(dialog).findByRole("button", { name: "Commit (0)" });
    expect(commit).toBeDisabled();
    // A disabled button with no account of why is the thing to avoid.
    expect(await within(dialog).findByText(/1 issue is not in that count/)).toBeInTheDocument();
  });

  it("reads a completion into the backlog as such", () => {
    expect(sprintChangeLine({ id: 1, entityType: "sprint_complete", entityKey: "12", field: "complete", beforeVal: "", baseVersion: "", createdAt: "",
      afterVal: JSON.stringify({ boardId: 1, name: "Sprint 12", moveTo: "", moveToName: "" }) }))
      .toBe("Complete sprint Sprint 12, unfinished cards to the backlog");
  });

  it("shows an edit and a delete of real sprints in words, and discards each", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 21, entityType: "sprint_delete", entityKey: "13", field: "delete", beforeVal: "Sprint 13", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ boardId: 1, name: "Sprint 13" }) },
      { id: 20, entityType: "sprint_edit", entityKey: "12", field: "edit", baseVersion: "", createdAt: "",
        beforeVal: JSON.stringify({ name: "Sprint 12", goal: "Ship", startDate: "2026-09-01T09:00:00.000+0000", endDate: "2026-09-14T09:00:00.000+0000" }),
        afterVal: JSON.stringify({ boardId: 1, name: "Sprint 12b", goal: "", startDate: "2026-09-01T09:00:00.000+0000", endDate: "2026-09-15T09:00:00.000+0000", clearGoal: true }) },
      ...rows,
    ]);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], sprintsChanged: ["Sprint 12b edited"], linked: [], conflicts: [], failures: [], remaining: 0,
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    expect(await within(dialog).findByText("5 changes on 2 issues, 1 of them new, and 2 sprint changes")).toBeInTheDocument();
    const cards = within(dialog).getAllByRole("group");
    expect(cards.map((c) => c.getAttribute("aria-label"))).toEqual(["Sprint 13", "Sprint 12", "TAM-NEW-1", "PLAT-409"]);
    expect(cards[1]).toHaveTextContent("Edit sprint Sprint 12: name to Sprint 12b, goal removed, dates to 2026-09-01 to 2026-09-15");
    expect(cards[0]).toHaveTextContent("Delete sprint Sprint 13");
    await user.click(within(cards[1]).getByRole("button", { name: "Discard the change to Sprint 12" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 20));
    await user.click(within(cards[0]).getByRole("button", { name: "Discard the change to Sprint 13" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 21));
    await user.click(within(dialog).getByRole("button", { name: "Commit (4)" }));
    expect(await within(dialog).findByText("Last commit: 1 sprint change pushed (Sprint 12b edited).")).toBeInTheDocument();
  });

  it("shows a draft board as its own card, first, apart from a draft sprint with the same id", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 31, entityType: "sprint_create", entityKey: "-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ boardId: -1, boardName: "Checkout", name: "Sprint 1", goal: "", startDate: "2026-09-16T09:00:00.000+0000", endDate: "2026-09-30T09:00:00.000+0000" }) },
      { id: 30, entityType: "board_create", entityKey: "-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "",
        afterVal: JSON.stringify({ name: "Checkout", type: "scrum", filterName: "Checkout filter", jql: "project = PLAT" }) },
    ]);
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    expect(await within(dialog).findByText("2 changes: 1 new board, 1 new sprint")).toBeInTheDocument();
    const cards = within(dialog).getAllByRole("group");
    expect(cards.map((c) => c.getAttribute("aria-label"))).toEqual(["Checkout", "Sprint 1"]);
    expect(cards[0]).toHaveTextContent("New board Checkout");
    expect(cards[0]).toHaveTextContent("Scrum board on the filter Checkout filter: project = PLAT");
    await user.click(within(cards[0]).getByRole("button", { name: "Discard Checkout" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 30));
  });

  it("says what the last Commit held and what each held row waits for", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], createdSprints: [{ draftId: -1, id: 100, name: "Sprint 15" }], linked: [], conflicts: [], remaining: 3,
      failures: [{ key: "TAM-NEW-1", error: "POST failed: 400 Epic Name is required", entityType: "issue_create", rowId: 3, retryable: true }],
      held: [{ key: "PLAT-409", entityType: "issue", rowId: 0, waitsFor: "TAM-NEW-1", reason: "waits for TAM-NEW-1, which Jira refused" }],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    expect(await within(dialog).findByText("Last commit: 1 sprint created (Sprint 15 is sprint 100), 1 failed, 1 waiting.")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-409 waits for TAM-NEW-1, which Jira refused.")).toBeInTheDocument();
    const card = within(dialog).getByRole("group", { name: "PLAT-409" });
    expect(within(card).getByText("Waiting")).toBeInTheDocument();
    expect(within(card).getByText("Waits for TAM-NEW-1, which Jira refused.")).toBeInTheDocument();
    expect(within(dialog).getByText("Commit again to retry the failures.")).toBeInTheDocument();
  });

  it("offers Commit again for held rows even when nothing failed for good", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [], conflicts: [], remaining: 2,
      failures: [{ key: "Sprint 15", error: "the draft sprint could not be decoded", entityType: "sprint_create", rowId: 9, retryable: false }],
      held: [{ key: "TAM-NEW-1", entityType: "issue_create", rowId: 3, waitsFor: "-1", reason: "waits for sprint \"Sprint 15\", which could not be read" }],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (2)" }));
    expect(await within(dialog).findByText("Commit again once what they wait for is in Jira.")).toBeInTheDocument();
    expect(within(dialog).queryByText("Commit again to retry the failures.")).not.toBeInTheDocument();
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
    expect(await within(dialog).findByText("Last commit: 2 cards moved (PLAT-412 to In Progress, PLAT-412 before PLAT-409).")).toBeInTheDocument();
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
      await within(dialog).findByText("Last commit: 1 card moved (PLAT-412 to In Progress), 1 already in place."),
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

  it("drops a failed move's line once the Undo has taken its journal row", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue(moveRows);
    vi.mocked(api.CommitPendingChanges).mockResolvedValue({
      committed: [], created: [], linked: [], moved: [], conflicts: [], remaining: 3,
      failures: [{
        key: "PLAT-412", entityType: "issue_transition", rowId: 12, retryable: true,
        error: "PLAT-412 cannot move to In Progress: Jira said no",
      }],
    });
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "Pending changes" });
    await user.click(await within(dialog).findByRole("button", { name: "Commit (1)" }));
    expect(await within(dialog).findByText(/Jira said no/)).toBeInTheDocument();

    // lastCommit stands until the next Commit, so the row going is the only
    // thing that can tell the banner the failure has been answered.
    vi.mocked(api.ListPendingChanges).mockResolvedValue(moveRows.filter((r) => r.id !== 12));
    await user.click(within(dialog).getByRole("button", { name: "Undo this move" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 12));
    await waitFor(() => expect(within(dialog).queryByText(/Jira said no/)).not.toBeInTheDocument());
    expect(within(dialog).queryByRole("button", { name: "Undo this move" })).not.toBeInTheDocument();
    expect(within(dialog).queryByText("Commit again to retry the failures.")).not.toBeInTheDocument();
  });
});
