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
  return { ...actual, GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn(), EditIssue: vi.fn(), ListActivity: vi.fn(), DiscardPendingChange: vi.fn(), GetLinkTypes: vi.fn(), ListEpics: vi.fn(), SearchUsers: vi.fn(), ListPriorities: vi.fn(), GetSubtaskTypeName: vi.fn(), CreateIssue: vi.fn(), MoveIssueToSprint: vi.fn(), BrowserOpenURL: vi.fn() };
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
function renderPanel(onClose = vi.fn(), sprints?: api.Sprint[], issue: Issue = story, emptyNote?: string, jiraUrl?: string) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <IssueDetailPanel profileId="p1" issue={issue} jiraUrl={jiraUrl} sprints={sprints} emptyNote={emptyNote} onClose={onClose} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

// detail is the fetched half of one issue, with only the parts a test cares
// about spelled out.
function detailOf(over: Partial<api.IssueDetail> = {}): api.IssueDetail {
  return {
    key: "PLAT-412",
    description: "As a shopper I can enter a promo code on the payment step.",
    links: [],
    fields: {},
    comments: [],
    commentTotal: 0,
    commentsTruncated: false,
    fetchedAt: "",
    ...over,
  };
}

function commentOf(over: Partial<api.IssueComment> = {}): api.IssueComment {
  return {
    id: "1",
    author: "ranand",
    authorName: "R. Anand",
    created: "2026-09-13T10:14:00Z",
    updated: "2026-09-13T10:14:00Z",
    body: "Looks right to me.",
    restriction: "",
    ...over,
  };
}

// The read view has no form control, so the description is reached by its
// text now and only through Edit as a textarea.
async function openDescriptionEditor(user: ReturnType<typeof userEvent.setup>): Promise<HTMLElement> {
  const edit = await screen.findByRole("button", { name: "Edit" });
  await waitFor(() => expect(edit).toBeEnabled());
  await user.click(edit);
  return screen.getByLabelText("Description");
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
  vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({
    links: [
      { direction: "inward", type: "Tested By", key: "XT-1018", summary: "Promo code applies discount", issueType: "Test" },
      { direction: "outward", type: "Relates", key: "PLAT-388", summary: "Promo codes must be single-use", issueType: "Requirement" },
    ],
  }));
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

  // A story still drafted as TAM-NEW-2 can hold a technical task: Commit
  // creates the story first and the sub-task after it has a real key.
  it("drafts a sub-task under a draft", async () => {
    renderPanel(vi.fn(), undefined, { ...story, key: "TAM-NEW-2", status: "Draft", draft: true, pending: true });
    expect(await screen.findByRole("button", { name: "+ Technical task" })).toBeInTheDocument();
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
    // The description reads as rendered markup now, not as a textarea.
    await waitFor(() => expect(within(details).getByText("As a shopper I can enter a promo code on the payment step.")).toBeInTheDocument());
    expect(api.GetIssueDetail).toHaveBeenCalledWith("p1", "PLAT-412");
  });

  // Fix round 3, Minor: D6 chose one Refresh for the whole view. The Fields
  // section's own went through GetIssueDetail, so it did nothing at all
  // while the cached detail was still fresh, and nothing ever at
  // detail_cache_minutes = 0.
  it("leaves the Fields section without a Refresh of its own", () => {
    renderPanel();
    const heading = (title: string) =>
      screen.getByRole("button", { name: new RegExp(`^${title}`) }).closest("section") as HTMLElement;
    expect(within(heading("Fields")).queryByRole("button", { name: /Refresh/ })).toBeNull();
    // The linked tests are read by their own binding, not the detail cache,
    // so that section keeps its Refresh: this is a removal, not a sweep.
    expect(within(heading("Covered by tests")).getByRole("button", { name: /Refresh/ })).toBeInTheDocument();
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
    await waitFor(() => expect(screen.getByText("As a shopper I can enter a promo code on the payment step.")).toBeInTheDocument());
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
    const onClose = vi.fn();
    renderPanel(onClose);
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
    await waitFor(() => expect(screen.getByText("As a shopper I can enter a promo code on the payment step.")).toBeInTheDocument());
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
      key: "PLAT-412", description: "d", fields: {}, comments: [], commentTotal: 0, commentsTruncated: false, fetchedAt: "",
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
      key: "PLAT-412", description: "d", fields: {}, comments: [], commentTotal: 0, commentsTruncated: false, fetchedAt: "",
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

describe("IssueDetailPanel description", () => {
  // The picked syntax lives for the app run, so this test works on its own
  // two issues: picking Markdown for PLAT-412 here would reach every later
  // test in this file, which is exactly the memory being asserted.
  const picky: Issue = { ...story, key: "PLAT-500" };

  it("renders the markup, and remembers a picked syntax for that issue alone", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({ description: "h3. Acceptance criteria\n* one" }));
    const first = renderPanel(vi.fn(), undefined, picky);
    const fields = section("Fields");
    await waitFor(() => expect(within(fields).getByRole("heading", { name: "Acceptance criteria" })).toBeInTheDocument());
    expect(within(fields).getByRole("listitem")).toHaveTextContent("one");
    expect(screen.queryByLabelText("Description")).not.toBeInTheDocument();

    // Detection guessed wiki; the toggle overrides it and the read view
    // re-renders at once, with the heading now ordinary text.
    await user.click(within(fields).getByRole("button", { name: "Markdown" }));
    expect(within(fields).queryByRole("heading", { name: "Acceptance criteria" })).not.toBeInTheDocument();
    expect(within(fields).getByText(/h3\. Acceptance criteria/)).toBeInTheDocument();
    first.unmount();

    // Reopening the same issue keeps the choice; another issue starts on
    // auto, since the memory is keyed by profile, key and field.
    const second = renderPanel(vi.fn(), undefined, picky);
    await waitFor(() => expect(within(section("Fields")).getByRole("button", { name: "Markdown" })).toHaveAttribute("aria-pressed", "true"));
    second.unmount();
    renderPanel(vi.fn(), undefined, { ...story, key: "PLAT-501" });
    await waitFor(() => expect(within(section("Fields")).getByRole("heading", { name: "Acceptance criteria" })).toBeInTheDocument());
  });

  it("holds Edit closed until the detail has loaded", async () => {
    vi.mocked(api.GetIssueDetail).mockReturnValue(new Promise(() => {}));
    renderPanel();
    expect(await screen.findByRole("button", { name: "Edit" })).toBeDisabled();
  });

  it("edits the raw text through Write and journals exactly what was typed", async () => {
    const user = userEvent.setup();
    renderPanel();
    const box = await openDescriptionEditor(user);
    expect(box).toHaveValue("As a shopper I can enter a promo code on the payment step.");
    await user.clear(box);
    await user.type(box, "h3. Steps");
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    await waitFor(() => expect(api.EditIssue).toHaveBeenCalledWith("p1", "PLAT-412", "description", "h3. Steps"));
    // Saving closes the editor and hands the reader the rendered text back.
    await waitFor(() => expect(screen.queryByLabelText("Description")).not.toBeInTheDocument());
  });

  it("restores the read view on Cancel with nothing journaled", async () => {
    const user = userEvent.setup();
    renderPanel();
    const box = await openDescriptionEditor(user);
    await user.clear(box);
    await user.type(box, "throw this away");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByLabelText("Description")).not.toBeInTheDocument();
    expect(screen.getByText("As a shopper I can enter a promo code on the payment step.")).toBeInTheDocument();
    expect(api.EditIssue).not.toHaveBeenCalled();
  });

  it("says so when an issue has no description", async () => {
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({ description: "" }));
    renderPanel();
    expect(await screen.findByText("No description.")).toBeInTheDocument();
  });

  // D5: the summary is one line Jira never wiki-renders, so marks stay
  // exactly as typed and only inline code and links are unwrapped.
  it("renders inline code in the summary heading and leaves marks alone", async () => {
    renderPanel(vi.fn(), undefined, { ...story, summary: "apply at {{/payment}} step for 2*3*4 items" });
    const summary = document.querySelector(".detail-summary") as HTMLElement;
    expect(within(summary).getByText("/payment").tagName).toBe("CODE");
    expect(summary).toHaveTextContent("2*3*4 items");
    expect(summary.querySelector("strong")).toBeNull();
  });
});

describe("IssueDetailPanel comments", () => {
  const wikiComment = commentOf({ id: "1", body: "Blocked on the gateway sandbox.\nbq. Sandbox back Thursday" });
  const mdComment = commentOf({
    id: "2",
    author: "msoto",
    authorName: "M. Soto",
    created: "2026-09-14T16:40:00Z",
    updated: "2026-09-14T17:02:00Z",
    body: "Rounding fixed:\n\n- **2 decimals** everywhere",
  });

  it("counts the comments in the heading and reads each one in its own syntax", async () => {
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({
      comments: [wikiComment, mdComment],
      commentTotal: 2,
      fetchedAt: "2026-09-16T14:02:00Z",
    }));
    renderPanel();
    const toggle = await screen.findByRole("button", { name: /^Comments/ });
    await waitFor(() => expect(toggle).toHaveTextContent("2"));
    // The cached stamp rides beside the heading, so a stale offline detail
    // says how old it is.
    expect(screen.getByText(/^cached /)).toBeInTheDocument();

    const region = await openSection("Comments");
    const cards = within(region).getAllByRole("article");
    expect(cards).toHaveLength(2);
    expect(within(cards[0]).getByRole("heading", { name: "R. Anand" })).toBeInTheDocument();
    expect(within(cards[1]).getByRole("heading", { name: "M. Soto" })).toBeInTheDocument();
    // Only the second was edited, and only the second reads in a syntax the
    // description does not.
    expect(within(region).getAllByText("edited")).toHaveLength(1);
    expect(within(cards[1]).getByText("edited")).toBeInTheDocument();
    expect(within(cards[1]).getByText("Markdown")).toBeInTheDocument();
    expect(within(cards[0]).queryByText("Markdown")).not.toBeInTheDocument();
    expect(within(cards[1]).getByText("2 decimals").tagName).toBe("STRONG");
  });

  it("shows the newest five until Show all is pressed", async () => {
    const many = Array.from({ length: 7 }, (_, i) =>
      commentOf({ id: String(i + 1), authorName: `Person ${i + 1}`, body: `note ${i + 1}` }),
    );
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({ comments: many, commentTotal: 7 }));
    renderPanel();
    const region = await openSection("Comments");
    await waitFor(() => expect(within(region).getAllByRole("article")).toHaveLength(5));
    expect(within(region).getByText("note 3")).toBeInTheDocument();
    expect(within(region).queryByText("note 2")).not.toBeInTheDocument();
    await userEvent.click(within(region).getByRole("button", { name: "Show all 7" }));
    expect(within(region).getAllByRole("article")).toHaveLength(7);
  });

  it("says how many of the issue's comments it holds when the read was cut short", async () => {
    const held = Array.from({ length: 500 }, (_, i) => commentOf({ id: String(i + 1), body: `note ${i + 1}` }));
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({ comments: held, commentTotal: 812, commentsTruncated: true }));
    renderPanel();
    const region = await openSection("Comments");
    await waitFor(() =>
      expect(within(region).getByText("Showing 500 of 812 comments. Open in Jira for the rest.")).toBeInTheDocument());
  });

  // The DTO's rule: truncated with nothing in hand is a failed read, never
  // an issue with no comments (that answers total 0 and truncated false).
  it("says the comments could not be read rather than that there are none", async () => {
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({ comments: [], commentTotal: 0, commentsTruncated: true }));
    renderPanel();
    const region = await openSection("Comments");
    await waitFor(() => expect(within(region).getByText(/could not be read/)).toBeInTheDocument());
    expect(within(region).queryByText("No comments.")).not.toBeInTheDocument();
  });

  it("says when an issue has no comments", async () => {
    renderPanel();
    const region = await openSection("Comments");
    await waitFor(() => expect(within(region).getByText("No comments.")).toBeInTheDocument());
  });

  it("names an authorless comment and marks a restricted one", async () => {
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({
      comments: [
        commentOf({ id: "1", author: "", authorName: "", body: "left by nobody" }),
        commentOf({ id: "2", restriction: "jira-developers", body: "internal note" }),
      ],
      commentTotal: 2,
    }));
    renderPanel();
    const region = await openSection("Comments");
    await waitFor(() => expect(within(region).getByRole("heading", { name: "Unknown user" })).toBeInTheDocument());
    expect(within(region).getByText("Restricted: jira-developers")).toBeInTheDocument();
  });

  it("opens a comment's link in the browser, and refuses one that is not a web address", async () => {
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({
      comments: [commentOf({ body: "see [Figma|https://figma.example/flow] and [Bad|javascript:alert(1)]" })],
      commentTotal: 1,
    }));
    renderPanel();
    const region = await openSection("Comments");
    const link = await within(region).findByRole("link", { name: "Figma" });
    await userEvent.click(link);
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://figma.example/flow");
    expect(within(region).queryByRole("link", { name: "Bad" })).not.toBeInTheDocument();
    expect(within(region).getByText(/Bad/)).toBeInTheDocument();
  });

  it("opens an issue key mentioned in a comment on the instance", async () => {
    vi.mocked(api.GetIssueDetail).mockResolvedValue(detailOf({
      comments: [commentOf({ body: "Blocked on PLAT-409." })],
      commentTotal: 1,
    }));
    renderPanel(vi.fn(), undefined, story, undefined, "https://jira.example");
    const region = await openSection("Comments");
    await userEvent.click(await within(region).findByRole("button", { name: "PLAT-409" }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://jira.example/browse/PLAT-409");
  });
});
