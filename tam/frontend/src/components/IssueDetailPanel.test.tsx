import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import { profileBackend } from "../profileBackend";
import * as api from "../api";
import type { Issue } from "../api";
import { useSync } from "../contexts/SyncContext";
import { IssueDetailPanel } from "./IssueDetailPanel";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn(), EditIssue: vi.fn(), ListActivity: vi.fn(), DiscardPendingChange: vi.fn(), GetLinkTypes: vi.fn(), ListEpics: vi.fn(), SearchUsers: vi.fn(), ListPriorities: vi.fn(), GetSubtaskTypeName: vi.fn(), CreateIssue: vi.fn(), MoveIssueToSprint: vi.fn() };
});

// The panel reads useSync to hold Save while a sync or commit runs. Its
// provider needs a profile and a dialog host the panel does not otherwise
// use, so it is mocked here rather than wired up for every test.
vi.mock("../contexts/SyncContext", () => ({ useSync: vi.fn(() => ({ status: "idle" })) }));

const story: Issue = {
  key: "PLAT-412", id: "1", project: "PLAT", type: "story", summary: "Checkout: apply promo code at payment step",
  status: "In Progress", assignee: "R. Anand", reporter: "PO", priority: "High", labels: ["checkout", "promo"],
  sprintId: "12", sprintName: "Sprint 12 - Checkout polish", parentKey: "PLAT-350", storyPoints: 5, rank: "",
  created: "2026-08-01T09:00:00Z", updated: "2026-09-05T09:58:00Z",
};

const longEpicSummary = "Modernize the checkout experience across web, mobile, and every partner integration";

const epics: Issue[] = [
  { ...story, key: "PLAT-350", id: "2", type: "epic", summary: longEpicSummary, parentKey: "" },
  { ...story, key: "PLAT-320", id: "3", type: "epic", summary: "Search relevance rework", parentKey: "" },
];

// The panel opens the create dialog for a sub-task, and that dialog reads the
// active profile, so the harness carries a provider the way the app does.
function renderPanel(onClose = vi.fn(), sprints?: api.Sprint[], issue: Issue = story, emptyNote?: string) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <IssueDetailPanel profileId="p1" issue={issue} sprints={sprints} emptyNote={emptyNote} onClose={onClose} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
  return onClose;
}

// BOARD_SPRINTS are what a caller with a board hands down. Away from one
// there is no such list, and the panel prints the sprint's name instead.
const BOARD_SPRINTS: api.Sprint[] = [
  { id: 12, boardId: 1, name: "Sprint 12", state: "active", startDate: "", endDate: "", goal: "" },
  { id: 13, boardId: 1, name: "Sprint 13", state: "future", startDate: "", endDate: "", goal: "" },
];

beforeEach(() => {
  vi.clearAllMocks();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  vi.mocked(useSync).mockReturnValue({ status: "idle" } as any);
  vi.mocked(api.GetIssueDetail).mockResolvedValue({
    key: "PLAT-412",
    description: "As a shopper I can enter a promo code on the payment step.",
    links: [
      { direction: "inward", type: "Tested By", key: "XT-1018", summary: "Promo code applies discount", issueType: "Test" },
      { direction: "outward", type: "Relates", key: "PLAT-388", summary: "Promo codes must be single-use", issueType: "Requirement" },
    ],
    fields: {},
  });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([
    { key: "XT-1018", summary: "Promo code applies discount", linkType: "Tested By" },
    { key: "XT-1019", summary: "Expired promo code rejected", linkType: "Tested By" },
  ]);
  vi.mocked(api.EditIssue).mockResolvedValue();
  vi.mocked(api.MoveIssueToSprint).mockResolvedValue();
  vi.mocked(api.GetLinkTypes).mockResolvedValue([]);
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.SearchUsers).mockResolvedValue([{ name: "ranand", displayName: "R. Anand" }]);
  vi.mocked(api.ListPriorities).mockResolvedValue(["Highest", "High", "Medium", "Low"]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Technical task");
  vi.mocked(api.ListActivity).mockResolvedValue([
    { id: 5, occurredAt: "2026-09-06T10:10:00Z", actor: "araha", entityType: "issue_create", entityKey: "PLAT-412", action: "commit", field: "create", beforeVal: "", afterVal: "{\"summary\":\"x\"}", note: "" },
    { id: 4, occurredAt: "2026-09-06T10:07:00Z", actor: "araha", entityType: "issue_create", entityKey: "PLAT-412", action: "discard", field: "create", beforeVal: "{\"summary\":\"x\"}", afterVal: "", note: "" },
    { id: 3, occurredAt: "2026-09-06T10:06:00Z", actor: "araha", entityType: "link", entityKey: "PLAT-412", action: "link", field: "Relates|outward|XT-1018", beforeVal: "", afterVal: "XT-1018", note: "" },
    { id: 2, occurredAt: "2026-09-06T10:05:00Z", actor: "araha", entityType: "issue", entityKey: "PLAT-412", action: "edit", field: "storyPoints", beforeVal: "5", afterVal: "8", note: "" },
    { id: 1, occurredAt: "2026-09-06T10:00:00Z", actor: "araha", entityType: "issue", entityKey: "PLAT-412", action: "edit", field: "summary", beforeVal: "Checkout: apply promo code", afterVal: "Checkout: apply promo code at payment step", note: "" },
  ]);
});

// The panel stacks collapsible sections now instead of tabs. Fields is open
// on mount; the rest are opened by their heading, which is what a reader
// does, and the region the heading controls is what the assertions read.
async function openSection(title: string): Promise<HTMLElement> {
  const toggle = await screen.findByRole("button", { name: new RegExp(`^${title}`) });
  if (toggle.getAttribute("aria-expanded") !== "true") {
    await userEvent.click(toggle);
  }
  const id = toggle.getAttribute("aria-controls");
  const region = id ? document.getElementById(id) : null;
  if (!region) throw new Error(`no region for section ${title}`);
  return region;
}

function section(title: string): HTMLElement {
  const toggle = screen.getByRole("button", { name: new RegExp(`^${title}`) });
  const id = toggle.getAttribute("aria-controls");
  const region = id ? document.getElementById(id) : null;
  if (!region) throw new Error(`no region for section ${title}`);
  return region;
}

describe("IssueDetailPanel", () => {
  it("drafts a sub-task from the body, under the issue it is about", async () => {
    renderPanel();
    // The action sits with the content it produces, not in the crowded head.
    const draft = await screen.findByRole("button", { name: "+ Technical task" });
    expect(draft).toBeInTheDocument();
    await userEvent.click(draft);
    const dialog = await screen.findByRole("dialog", { name: /^New / });
    // The parent is the issue the panel is about, stated rather than chosen.
    expect(within(dialog).getByText("PLAT-412")).toBeInTheDocument();
    expect(within(dialog).queryByLabelText("Type")).not.toBeInTheDocument();
  });

  it("is resizeable, and remembers the width", async () => {
    renderPanel();
    const panel = await screen.findByRole("complementary");
    expect(panel).toHaveStyle({ width: "352px" });
    const grip = panel.querySelector(".detail-resizer");
    expect(grip).toBeInTheDocument();
  });

  it("shows the cached fields at once and the description once fetched", async () => {
    renderPanel();
    expect(screen.getByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
    expect(screen.getByText("Checkout: apply promo code at payment step")).toBeInTheDocument();
    // Status reads as a chip on the panel's head, beside the key, the way
    // XTM's does; the sprint and the other read-only facts sit in the grid
    // above the editable fields.
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.getByText("Sprint 12 - Checkout polish")).toBeInTheDocument();
    const details = section("Fields");
    // The assignee input shows the display name it was synced with; what it
    // stores once a person is picked is the username.
    expect(within(details).getByLabelText("Assignee")).toHaveValue("R. Anand");
    expect(within(details).getByLabelText("Story points")).toHaveValue("5");
    expect(within(details).getByLabelText("Labels")).toHaveValue("checkout, promo");
    await waitFor(() => expect(screen.getByLabelText("Description")).toHaveValue("As a shopper I can enter a promo code on the payment step."));
    expect(api.GetIssueDetail).toHaveBeenCalledWith("p1", "PLAT-412");
  });

  it("switches to Links and Tests", async () => {
    renderPanel();
    const links = await openSection("Links");
    await waitFor(() => expect(within(links).getByText("XT-1018")).toBeInTheDocument());
    expect(within(links).getByText("Tested By")).toBeInTheDocument();
    expect(within(links).getByText("PLAT-388")).toBeInTheDocument();
    const tests = await openSection("Covered by tests");
    await waitFor(() => expect(within(tests).getByText("XT-1019")).toBeInTheDocument());
    expect(within(tests).getByText(/via XTM, link: Tested By/)).toBeInTheDocument();
    expect(api.ListLinkedTests).toHaveBeenCalledWith("p1", "PLAT-412");
  });

  it("keeps the cached fields and offers a retry when the detail fetch fails", async () => {
    vi.mocked(api.GetIssueDetail).mockRejectedValueOnce(new Error("jira: 502 Bad Gateway"));
    renderPanel();
    await waitFor(() => expect(screen.getByTestId("detail-error")).toHaveTextContent("jira: 502 Bad Gateway"));
    expect(screen.getByLabelText("Assignee")).toHaveValue("R. Anand");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.getByLabelText("Description")).toHaveValue("As a shopper I can enter a promo code on the payment step."));
    expect(api.GetIssueDetail).toHaveBeenCalledTimes(2);
  });

  it("retries the linked tests after a failure", async () => {
    vi.mocked(api.ListLinkedTests).mockRejectedValueOnce(new Error("jira: 503"));
    renderPanel();
    const tests = await openSection("Covered by tests");
    await waitFor(() => expect(within(tests).getByTestId("tests-error")).toHaveTextContent("jira: 503"));
    await userEvent.click(within(tests).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(within(tests).getByText("XT-1018")).toBeInTheDocument());
    expect(api.ListLinkedTests).toHaveBeenCalledTimes(2);
  });

  it("keeps the Links tab recoverable", async () => {
    vi.mocked(api.GetIssueDetail).mockRejectedValueOnce(new Error("jira: 502"));
    renderPanel();
    const links = await openSection("Links");
    await waitFor(() => expect(within(links).getByTestId("links-error")).toHaveTextContent("jira: 502"));
    await userEvent.click(within(links).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(within(links).getByText("XT-1018")).toBeInTheDocument());
  });

  it("says when there are no linked tests and closes", async () => {
    vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
    const onClose = renderPanel();
    await openSection("Covered by tests");
    await waitFor(() => expect(screen.getByText("No linked tests.")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });
});

describe("IssueDetailPanel write path", () => {
  it("prefills the editable fields and saves only what changed, in field order", async () => {
    const user = userEvent.setup();
    renderPanel();
    const summary = await screen.findByLabelText("Summary");
    expect(summary).toHaveValue("Checkout: apply promo code at payment step");
    expect(screen.getByLabelText("Labels")).toHaveValue("checkout, promo");
    expect(screen.getByLabelText("Story points")).toHaveValue("5");
    const description = await screen.findByLabelText("Description");
    await waitFor(() => expect(description).toHaveValue("As a shopper I can enter a promo code on the payment step."));
    const save = screen.getByRole("button", { name: "Save edit" });
    expect(save).toBeDisabled();

    await user.clear(screen.getByLabelText("Story points"));
    await user.type(screen.getByLabelText("Story points"), "8");
    await user.clear(summary);
    await user.type(summary, "Checkout: promo code at payment");
    expect(save).toBeEnabled();
    await user.click(save);
    await waitFor(() => expect(api.EditIssue).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.EditIssue).mock.calls[0]).toEqual(["p1", "PLAT-412", "summary", "Checkout: promo code at payment"]);
    expect(vi.mocked(api.EditIssue).mock.calls[1]).toEqual(["p1", "PLAT-412", "storyPoints", "8"]);
    expect(await screen.findByText("Saved. Commit pushes it to Jira.")).toBeInTheDocument();
  });

  it("keeps Save disabled while a sync or commit is running", async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(useSync).mockReturnValue({ status: "committing" } as any);
    const user = userEvent.setup();
    renderPanel();
    const summary = await screen.findByLabelText("Summary");
    await user.clear(summary);
    await user.type(summary, "Checkout: promo code at payment");
    expect(screen.getByRole("button", { name: "Save edit" })).toBeDisabled();
    expect(api.EditIssue).not.toHaveBeenCalled();
  });

  it("refuses a blank summary and non-numeric points before calling the backend", async () => {
    const user = userEvent.setup();
    renderPanel();
    const summary = await screen.findByLabelText("Summary");
    await user.clear(summary);
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText("Summary cannot be empty.")).toBeInTheDocument();
    await user.type(summary, "x");
    await user.clear(screen.getByLabelText("Story points"));
    await user.type(screen.getByLabelText("Story points"), "eight");
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText("Story points must be a number.")).toBeInTheDocument();
    expect(api.EditIssue).not.toHaveBeenCalled();
  });

  it("shows the backend's error and keeps the edits in the form", async () => {
    const user = userEvent.setup();
    vi.mocked(api.EditIssue).mockRejectedValueOnce(new Error("field \"priority\" cannot be edited"));
    renderPanel();
    // Priority is the instance's own list now, not a text box.
    const priority = await screen.findByLabelText("Priority");
    await waitFor(() => expect(within(priority as HTMLElement).getByRole("option", { name: "Highest" })).toBeInTheDocument());
    await user.selectOptions(priority, "Highest");
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText(/cannot be edited/)).toBeInTheDocument();
    expect(priority).toHaveValue("Highest");
  });

  it("lists the activity newest first on the Activity tab", async () => {
    const user = userEvent.setup();
    renderPanel();
    await openSection("Activity");
    const items = await screen.findAllByRole("listitem");
    expect(items).toHaveLength(5);
    expect(items[0]).toHaveTextContent("araha pushed the draft to Jira");
    expect(items[1]).toHaveTextContent("araha discarded the draft");
    expect(items[2]).toHaveTextContent("araha added a link: Relates (outward) to XT-1018");
    expect(items[3]).toHaveTextContent("araha edited Story points: 5 to 8");
    expect(items[4]).toHaveTextContent("araha edited Summary");
    expect(api.ListActivity).toHaveBeenCalledWith("p1", "PLAT-412", 200);
  });

  it("reads a board move as a place rather than as a field and an id", async () => {
    vi.mocked(api.ListActivity).mockResolvedValue([
      { id: 4, occurredAt: "2026-09-06T10:20:00Z", actor: "araha", entityType: "issue_rank", entityKey: "PLAT-412", action: "move", field: "rank", beforeVal: "", afterVal: "before|PLAT-409|1", note: "" },
      { id: 3, occurredAt: "2026-09-06T10:15:00Z", actor: "araha", entityType: "issue_sprint", entityKey: "PLAT-412", action: "commit", field: "sprintId", beforeVal: "12|Sprint 12", afterVal: "13|Sprint 13", note: "" },
      { id: 2, occurredAt: "2026-09-06T10:10:00Z", actor: "araha", entityType: "issue_transition", entityKey: "PLAT-412", action: "discard", field: "statusId", beforeVal: "3|In Progress", afterVal: "1|To Do", note: "" },
      { id: 1, occurredAt: "2026-09-06T10:05:00Z", actor: "araha", entityType: "issue_transition", entityKey: "PLAT-412", action: "move", field: "statusId", beforeVal: "1|To Do", afterVal: "3|In Progress", note: "" },
    ]);
    renderPanel();
    await openSection("Activity");
    const items = await screen.findAllByRole("listitem");
    expect(items[0]).toHaveTextContent("araha moved this card before PLAT-409");
    expect(items[1]).toHaveTextContent("araha pushed the move to Sprint 13");
    expect(items[2]).toHaveTextContent("araha put this card back in To Do");
    expect(items[3]).toHaveTextContent("araha moved this card to In Progress");
    expect(screen.queryByText(/statusId/)).not.toBeInTheDocument();
  });

  it("marks a draft in the panel head", async () => {
    render(
      <QueryClientProvider client={createQueryClient()}>
        <DialogProvider>
          <IssueDetailPanel profileId="p1" issue={{ ...story, key: "TAM-NEW-1", status: "Draft", draft: true, pending: true }} onClose={vi.fn()} />
        </DialogProvider>
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Draft", { selector: "span.chip-draft" })).toBeInTheDocument();
    expect(screen.getByText("Commit creates this issue in Jira and gives it a real key.")).toBeInTheDocument();
  });

  it("marks a pending link on the Links tab and discards it", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetIssueDetail).mockResolvedValue({
      key: "PLAT-412", description: "d", fields: {},
      links: [
        { direction: "inward", type: "Tested By", key: "XT-1018", summary: "Promo code applies discount", issueType: "Test" },
        { direction: "outward", type: "Relates", key: "XT-1031", summary: "Retried payment is not charged twice", issueType: "Test", pending: true, pendingId: 41 },
      ],
    });
    vi.mocked(api.DiscardPendingChange).mockResolvedValue();
    renderPanel();
    await openSection("Links");
    const row = (await screen.findByText("XT-1031")).closest("li")!;
    expect(row).toHaveTextContent("pending");
    await user.click(within(row).getByRole("button", { name: "Discard link to XT-1031" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 41));
    expect(screen.getByRole("heading", { name: "Add link" })).toBeInTheDocument();
  });

  it("surfaces a failed link discard", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetIssueDetail).mockResolvedValue({
      key: "PLAT-412", description: "d", fields: {},
      links: [
        { direction: "outward", type: "Relates", key: "XT-1031", summary: "Retried payment is not charged twice", issueType: "Test", pending: true, pendingId: 41 },
      ],
    });
    vi.mocked(api.DiscardPendingChange).mockRejectedValueOnce(new Error("row is gone"));
    renderPanel();
    await openSection("Links");
    const row = (await screen.findByText("XT-1031")).closest("li")!;
    await user.click(within(row).getByRole("button", { name: "Discard link to XT-1031" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 41));
    expect(await screen.findByText(/row is gone/)).toBeInTheDocument();
  });
});

describe("IssueDetailPanel Epic field", () => {
  it("offers the epics preselected to the story's parent, with a long summary truncated", async () => {
    vi.mocked(api.ListEpics).mockResolvedValue(epics);
    renderPanel();
    // getByLabelText throws on more than one match, so this alone proves
    // there is exactly one element labelled Epic.
    const select = await screen.findByLabelText("Epic");
    expect(select.tagName).toBe("SELECT");
    await waitFor(() => expect(within(select).getAllByRole("option")).toHaveLength(3));
    expect(select).toHaveValue("PLAT-350");
    const options = within(select).getAllByRole("option");
    expect(options[0]).toHaveTextContent("(none)");
    expect(options[1]).toHaveTextContent(`PLAT-350 ${longEpicSummary.slice(0, 60)}…`);
    expect(options[2]).toHaveTextContent("PLAT-320 Search relevance rework");
  });

  it("saves the chosen epic through the same edit path as every other field", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListEpics).mockResolvedValue(epics);
    renderPanel();
    const select = await screen.findByLabelText("Epic");
    await waitFor(() => expect(within(select).getAllByRole("option")).toHaveLength(3));
    await user.selectOptions(select, "PLAT-320");
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    await waitFor(() => expect(api.EditIssue).toHaveBeenCalledWith("p1", "PLAT-412", "parentKey", "PLAT-320"));
  });

  it("hides the Epic select entirely when the issue is an epic", async () => {
    vi.mocked(api.ListEpics).mockResolvedValue(epics);
    render(
      <QueryClientProvider client={createQueryClient()}>
        <DialogProvider>
          <IssueDetailPanel profileId="p1" issue={{ ...story, type: "epic" }} onClose={vi.fn()} />
        </DialogProvider>
      </QueryClientProvider>,
    );
    const details = await openSection("Fields");
    expect(screen.queryByLabelText("Epic")).not.toBeInTheDocument();
    expect(within(details).queryByText("Epic")).not.toBeInTheDocument();
    expect(within(details).queryByText("Parent")).not.toBeInTheDocument();
  });
});

describe("IssueDetailPanel sprint field", () => {
  it("prints the sprint as a fact away from a board", async () => {
    renderPanel();
    expect(await screen.findByText("Sprint 12 - Checkout polish")).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Sprint" })).not.toBeInTheDocument();
  });

  it("journals a sprint move through the binding the card menu uses", async () => {
    renderPanel(vi.fn(), BOARD_SPRINTS);
    const picker = await screen.findByRole("combobox", { name: "Sprint" });
    expect(picker).toHaveValue("12");
    await userEvent.selectOptions(picker, "13");
    await waitFor(() => expect(api.MoveIssueToSprint).toHaveBeenCalledWith("p1", "PLAT-412", "13"));
    // Nothing else is touched: a sprint move is its own journal row, not an
    // edit riding on the field form.
    expect(api.EditIssue).not.toHaveBeenCalled();
  });

  it("offers the backlog as a destination, which is a place and not an absence", async () => {
    renderPanel(vi.fn(), BOARD_SPRINTS);
    const picker = await screen.findByRole("combobox", { name: "Sprint" });
    expect(within(picker).getByRole("option", { name: "The backlog" })).toBeInTheDocument();
    await userEvent.selectOptions(picker, "");
    await waitFor(() => expect(api.MoveIssueToSprint).toHaveBeenCalledWith("p1", "PLAT-412", ""));
  });

  it("keeps a closed sprint the picker never offers as its own option", async () => {
    // The issue sits in sprint 12; a board whose picker has moved on to 13
    // and 14 would otherwise show this card as being in the backlog.
    renderPanel(vi.fn(), [BOARD_SPRINTS[1]]);
    const picker = await screen.findByRole("combobox", { name: "Sprint" });
    expect(picker).toHaveValue("12");
    expect(within(picker).getByRole("option", { name: "Sprint 12 - Checkout polish" })).toBeInTheDocument();
  });

  // A sub-task has no sprint of its own in Jira: it follows its parent, and
  // the Agile move endpoint refuses one aimed at it, so the panel keeps the
  // read-only fact even where a board hands down a list to pick from.
  it("keeps the sprint a read-only fact for a sub-task, even with a list to pick from", async () => {
    renderPanel(vi.fn(), BOARD_SPRINTS, { ...story, type: "subtask" });
    expect(await screen.findByText("Sprint 12 - Checkout polish")).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Sprint" })).not.toBeInTheDocument();
  });

  it("passes the caller's empty-list note through to the select", async () => {
    renderPanel(vi.fn(), [], story, "No sprints yet, sync a board first");
    await screen.findByRole("combobox", { name: "Sprint" });
    expect(screen.getByText("No sprints yet, sync a board first")).toBeInTheDocument();
  });
});
