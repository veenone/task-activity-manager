import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { EpicNode, EpicTreeData, Issue } from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { EpicsView } from "./EpicsView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    GetEpicTree: vi.fn(),
    ListEpics: vi.fn(),
    ListSprints: vi.fn(),
    ListOpenSprints: vi.fn(),
    MoveIssueToSprint: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
    ListActivity: vi.fn(),
    EditIssue: vi.fn(),
    GetLinkTypes: vi.fn(),
    GetSubtaskTypeName: vi.fn(),
    SearchUsers: vi.fn(),
    ListPriorities: vi.fn(),
    CreateIssue: vi.fn(),
  };
});

// A selected row's IssueDetailPanel reads useSync to hold Save during a sync
// or commit; its provider needs more wiring than this view otherwise uses,
// so it is mocked here the same way BacklogView.test.tsx mocks it.
vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ status: "idle" }) }));

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
    ...over,
  };
}

function epicNode(over: Partial<EpicNode> & { issue: Issue }): EpicNode {
  return { children: [], total: 0, done: 0, points: 0, donePoints: 0, ...over };
}

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderView() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ModalProvider>
            <Loader />
            <EpicsView />
          </ModalProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const lastTreeQuery = () => vi.mocked(api.GetEpicTree).mock.calls.at(-1)?.[1];

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListSprints).mockResolvedValue([{ id: "12", name: "Sprint 12" }]);
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: "PLAT-101", description: "", links: [], fields: {} });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
  vi.mocked(api.ListActivity).mockResolvedValue([]);
  vi.mocked(api.GetLinkTypes).mockResolvedValue([]);
  vi.mocked(api.EditIssue).mockResolvedValue(undefined);
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.MoveIssueToSprint).mockResolvedValue();
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Technical task");
  vi.mocked(api.SearchUsers).mockResolvedValue([]);
  vi.mocked(api.ListPriorities).mockResolvedValue(["High"]);
});

describe("EpicsView", () => {
  it("renders epics with their progress and children", async () => {
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp", status: "In Progress" }),
          children: [
            issue({ key: "PLAT-101", type: "story", summary: "Apply promo code", status: "Done", storyPoints: 5 }),
            issue({ key: "PLAT-102", type: "task", summary: "Add coupon banner", status: "To Do", storyPoints: 5 }),
          ],
          total: 2, done: 1, points: 10, donePoints: 5,
        }),
      ],
      orphans: [],
      truncated: false,
    });
    renderView();
    expect(await screen.findByRole("treeitem", { name: "PLAT-100 Checkout revamp" })).toBeInTheDocument();
    expect(screen.getByText("1 of 2 done, 10 pts")).toBeInTheDocument();
    expect(screen.getByText("Apply promo code")).toBeInTheDocument();
    expect(screen.getByText("Add coupon banner")).toBeInTheDocument();
  });

  // The reported path: open an epic, select a story under it, draft a
  // technical task from the panel. The parent is the story, not the epic and
  // not nothing.
  it("drafts a technical task under the story the panel is showing", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [issue({ key: "PLAT-101", type: "story", summary: "Apply promo code" })],
          total: 1, done: 0, points: 5, donePoints: 0,
        }),
      ],
      orphans: [],
      truncated: false,
    });
    renderView();
    await user.click(await screen.findByRole("treeitem", { name: "PLAT-100 Checkout revamp" }));
    await user.click(await screen.findByText("Apply promo code"));
    await user.click(await screen.findByRole("button", { name: "+ Technical task" }));
    const dialog = await screen.findByRole("dialog", { name: /^New / });
    expect(within(dialog).getByText("PLAT-101")).toBeInTheDocument();
    expect(within(dialog).queryByText("none")).not.toBeInTheDocument();
  });

  it("shows the No epic node with its count and a line past 200 orphans", async () => {
    const orphans = Array.from({ length: 205 }, (_, i) => issue({ key: `PLAT-${900 + i}`, summary: `Orphan ${i}` }));
    vi.mocked(api.GetEpicTree).mockResolvedValue({ epics: [], orphans, truncated: false });
    renderView();
    expect(await screen.findByText("No epic")).toBeInTheDocument();
    expect(screen.getByText("205")).toBeInTheDocument();
    expect(screen.getByText("and 5 more. Use the search to narrow.")).toBeInTheDocument();
  });

  it("shows the count line", async () => {
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [issue({ key: "PLAT-101" }), issue({ key: "PLAT-102" })],
          total: 2, done: 0, points: 0, donePoints: 0,
        }),
      ],
      orphans: [issue({ key: "PLAT-200", summary: "Orphan issue" })],
      truncated: false,
    });
    renderView();
    expect(await screen.findByText("1 epics, 3 issues, 1 without an epic")).toBeInTheDocument();
  });

  it("collapses and expands an epic's children via the caret", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [issue({ key: "PLAT-101", summary: "Apply promo code" })],
          total: 1, done: 0, points: 0, donePoints: 0,
        }),
      ],
      orphans: [],
      truncated: false,
    });
    renderView();
    expect(await screen.findByText("Apply promo code")).toBeInTheDocument();
    await user.click(screen.getByText("▾"));
    expect(screen.queryByText("Apply promo code")).not.toBeInTheDocument();
    await user.click(screen.getByText("▸"));
    expect(await screen.findByText("Apply promo code")).toBeInTheDocument();
  });

  it("round-trips visibility with Collapse all then Expand all", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [issue({ key: "PLAT-101", summary: "Apply promo code" })],
          total: 1, done: 0, points: 0, donePoints: 0,
        }),
      ],
      orphans: [],
      truncated: false,
    });
    renderView();
    expect(await screen.findByText("Apply promo code")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Collapse all" }));
    expect(screen.queryByText("Apply promo code")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Expand all" }));
    expect(await screen.findByText("Apply promo code")).toBeInTheDocument();
  });

  it("selects a child on click and opens the panel with its key", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [issue({ key: "PLAT-101", summary: "Apply promo code" })],
          total: 1, done: 0, points: 0, donePoints: 0,
        }),
      ],
      orphans: [],
      truncated: false,
    });
    renderView();
    await user.click(await screen.findByText("Apply promo code"));
    expect(await screen.findByRole("heading", { name: "PLAT-101" })).toBeInTheDocument();
  });

  it("offers the profile's open sprints on a child issue and journals a move", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [issue({ key: "PLAT-101", summary: "Apply promo code", sprintId: "12", sprintName: "Sprint 12" })],
          total: 1, done: 0, points: 0, donePoints: 0,
        }),
      ],
      orphans: [],
      truncated: false,
    });
    vi.mocked(api.ListOpenSprints).mockResolvedValue([
      { id: 12, name: "Sprint 12", boardName: "Platform board", state: "active" },
      { id: 13, name: "Sprint 13", boardName: "Platform board", state: "future" },
    ]);
    renderView();
    await user.click(await screen.findByText("Apply promo code"));
    // The filter bar has its own select with the same "Sprint" label, so the
    // query is scoped to the detail panel, the element the test is about.
    const panel = await screen.findByRole("complementary");
    const picker = await within(panel).findByRole("combobox", { name: "Sprint" });
    expect(within(picker).getByRole("option", { name: "Sprint 13" })).toBeInTheDocument();
    await user.selectOptions(picker, "13");
    await waitFor(() => expect(api.MoveIssueToSprint).toHaveBeenCalledWith("p1", "PLAT-101", "13"));
  });

  it("highlights the moved row for two seconds after a parentKey edit", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const childV1 = issue({ key: "PLAT-101", summary: "Apply promo code", parentKey: "PLAT-100" });
    const treeWith = (child: Issue): EpicTreeData => ({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [child],
          total: 1, done: 0, points: 0, donePoints: 0,
        }),
      ],
      orphans: [],
      truncated: false,
    });
    vi.mocked(api.GetEpicTree)
      .mockResolvedValueOnce(treeWith(childV1))
      .mockResolvedValue(treeWith({ ...childV1, parentKey: "PLAT-200" }));
    vi.mocked(api.ListEpics).mockResolvedValue([
      issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
      issue({ key: "PLAT-200", type: "epic", summary: "Search relevance rework" }),
    ]);

    renderView();
    const treeNav = await screen.findByRole("tree", { name: "Epics" });
    await user.click(within(treeNav).getByText("Apply promo code"));
    const epicSelect = await screen.findByLabelText("Epic");
    await user.selectOptions(epicSelect, "PLAT-200");
    await user.click(screen.getByRole("button", { name: "Save edit" }));

    const row = () => within(treeNav).getByText("Apply promo code").closest('[role="treeitem"]') as HTMLElement;
    await waitFor(() => expect(row()).toHaveClass("epic-row-moved"));
    await act(async () => {
      vi.advanceTimersByTime(2100);
    });
    await waitFor(() => expect(row()).not.toHaveClass("epic-row-moved"));

    vi.useRealTimers();
  });

  it("sends the search text in the tree query after the debounce", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue({ epics: [], orphans: [], truncated: false });
    renderView();
    await screen.findByText(/without an epic/);
    await user.type(screen.getByRole("searchbox", { name: "Search epics and issues" }), "promo");
    await waitFor(() => expect(lastTreeQuery()?.text).toBe("promo"));
  });

  it("flips showDone from the checkbox", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue({ epics: [], orphans: [], truncated: false });
    renderView();
    await screen.findByText(/without an epic/);
    await user.click(screen.getByRole("checkbox", { name: "Show done" }));
    await waitFor(() => expect(lastTreeQuery()?.showDone).toBe(true));
  });

  it("treats an Approved requirement as open in its chip and the epic's progress", async () => {
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [
            issue({ key: "PLAT-103", type: "requirement", summary: "Promo codes single-use", status: "Approved" }),
            issue({ key: "PLAT-101", type: "story", summary: "Apply promo code", status: "Done" }),
          ],
          total: 2, done: 1, points: 0, donePoints: 0,
        }),
      ],
      orphans: [],
      truncated: false,
    });
    renderView();
    expect(await screen.findByText("1 of 2 done")).toBeInTheDocument();
    const approvedRow = screen.getByText("Promo codes single-use").closest('[role="treeitem"]') as HTMLElement;
    expect(within(approvedRow).getByText("Approved")).toHaveClass("chip-status-todo");
  });

  it("shows Loading epics then Refreshing during a filter refetch", async () => {
    const user = userEvent.setup();
    const tree: EpicTreeData = {
      epics: [epicNode({ issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }) })],
      orphans: [],
      truncated: false,
    };
    let resolveFirst!: (v: EpicTreeData) => void;
    vi.mocked(api.GetEpicTree).mockReturnValueOnce(new Promise<EpicTreeData>((res) => { resolveFirst = res; }));
    renderView();
    expect(await screen.findByText("Loading epics")).toBeInTheDocument();
    resolveFirst(tree);
    expect(await screen.findByRole("treeitem", { name: "PLAT-100 Checkout revamp" })).toBeInTheDocument();

    let resolveSecond!: (v: EpicTreeData) => void;
    vi.mocked(api.GetEpicTree).mockReturnValueOnce(new Promise<EpicTreeData>((res) => { resolveSecond = res; }));
    await user.click(screen.getByRole("checkbox", { name: "Show done" }));
    expect(await screen.findByText(/refreshing/)).toBeInTheDocument();
    resolveSecond(tree);
    await waitFor(() => expect(screen.queryByText(/refreshing/)).not.toBeInTheDocument());
  });

  it("shows the empty state when nothing is cached", async () => {
    vi.mocked(api.GetEpicTree).mockResolvedValue({ epics: [], orphans: [], truncated: false });
    renderView();
    expect(await screen.findByText("No issues cached yet. Use Sync in the topbar.")).toBeInTheDocument();
  });

  it("shows the narrowing hint when the tree is truncated", async () => {
    vi.mocked(api.GetEpicTree).mockResolvedValue({
      epics: [epicNode({ issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }) })],
      orphans: [],
      truncated: true,
    });
    renderView();
    expect(await screen.findByText("Showing the first 5,000 issues. Narrow the filter.")).toBeInTheDocument();
  });
});

describe("EpicTree keyboard", () => {
  function singleEpicTree(): EpicTreeData {
    return {
      epics: [
        epicNode({
          issue: issue({ key: "PLAT-100", type: "epic", summary: "Checkout revamp" }),
          children: [issue({ key: "PLAT-101", summary: "Apply promo code" })],
          total: 1, done: 0, points: 0, donePoints: 0,
        }),
      ],
      orphans: [],
      truncated: false,
    };
  }

  it("moves focus to the next visible row on ArrowDown", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue(singleEpicTree());
    renderView();
    const treeNav = await screen.findByRole("tree", { name: "Epics" });
    await screen.findByText("Apply promo code");
    const allEpicsRow = within(treeNav).getByText("All epics").closest('[role="treeitem"]') as HTMLElement;
    await user.click(allEpicsRow);
    expect(document.activeElement).toBe(allEpicsRow);

    await user.keyboard("{ArrowDown}");
    const epicRow = screen.getByRole("treeitem", { name: "PLAT-100 Checkout revamp" });
    expect(document.activeElement).toBe(epicRow);
  });

  it("expands a collapsed epic on ArrowRight and collapses it on ArrowLeft", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue(singleEpicTree());
    renderView();
    await screen.findByText("Apply promo code");
    await user.click(screen.getByRole("button", { name: "Collapse all" }));
    expect(screen.queryByText("Apply promo code")).not.toBeInTheDocument();

    const epicRow = screen.getByRole("treeitem", { name: "PLAT-100 Checkout revamp" });
    await user.click(epicRow);
    await user.keyboard("{ArrowRight}");
    expect(await screen.findByText("Apply promo code")).toBeInTheDocument();

    await user.keyboard("{ArrowLeft}");
    expect(screen.queryByText("Apply promo code")).not.toBeInTheDocument();
  });

  it("moves focus from a child to its epic on ArrowLeft", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue(singleEpicTree());
    renderView();
    const childRow = (await screen.findByText("Apply promo code")).closest('[role="treeitem"]') as HTMLElement;
    await user.click(childRow);
    expect(document.activeElement).toBe(childRow);

    await user.keyboard("{ArrowLeft}");
    const epicRow = screen.getByRole("treeitem", { name: "PLAT-100 Checkout revamp" });
    expect(document.activeElement).toBe(epicRow);
  });

  it("selects a focused child on Enter", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue(singleEpicTree());
    renderView();
    const treeNav = await screen.findByRole("tree", { name: "Epics" });
    await screen.findByText("Apply promo code");
    const allEpicsRow = within(treeNav).getByText("All epics").closest('[role="treeitem"]') as HTMLElement;
    await user.click(allEpicsRow);
    await user.keyboard("{ArrowDown}{ArrowDown}");
    const childRow = screen.getByText("Apply promo code").closest('[role="treeitem"]') as HTMLElement;
    expect(document.activeElement).toBe(childRow);

    await user.keyboard("{Enter}");
    expect(await screen.findByRole("heading", { name: "PLAT-101" })).toBeInTheDocument();
  });

  it("selects a focused child on Space", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue(singleEpicTree());
    renderView();
    const treeNav = await screen.findByRole("tree", { name: "Epics" });
    await screen.findByText("Apply promo code");
    const allEpicsRow = within(treeNav).getByText("All epics").closest('[role="treeitem"]') as HTMLElement;
    await user.click(allEpicsRow);
    await user.keyboard("{ArrowDown}{ArrowDown}");
    const childRow = screen.getByText("Apply promo code").closest('[role="treeitem"]') as HTMLElement;
    expect(document.activeElement).toBe(childRow);

    await user.keyboard(" ");
    expect(await screen.findByRole("heading", { name: "PLAT-101" })).toBeInTheDocument();
  });

  it("keeps exactly one focusable row after collapsing the selected child's epic", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetEpicTree).mockResolvedValue(singleEpicTree());
    const { container } = renderView();
    const treeNav = await screen.findByRole("tree", { name: "Epics" });
    const childRow = within(treeNav).getByText("Apply promo code").closest('[role="treeitem"]') as HTMLElement;
    await user.click(childRow);
    expect(await screen.findByRole("heading", { name: "PLAT-101" })).toBeInTheDocument();

    await user.click(within(treeNav).getByText("▾"));
    expect(within(treeNav).queryByText("Apply promo code")).not.toBeInTheDocument();

    const focusable = container.querySelectorAll('[tabindex="0"]');
    expect(focusable.length).toBe(1);

    (focusable[0] as HTMLElement).focus();
    await user.keyboard("{ArrowDown}");
    const epicRow = screen.getByRole("treeitem", { name: "PLAT-100 Checkout revamp" });
    expect(document.activeElement).toBe(epicRow);
  });
});
