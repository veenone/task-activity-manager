import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, LiveRegion, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { BoardView, Issue, PendingChange } from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { BoardsView } from "./BoardsView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    ListBoards: vi.fn(),
    ListBoardSprints: vi.fn(),
    GetBoard: vi.fn(),
    SyncBoards: vi.fn(),
    GetProfileSetting: vi.fn(),
    GetSyncState: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
    ListActivity: vi.fn(),
    ListEpics: vi.fn(),
    GetLinkTypes: vi.fn(),
    EditIssue: vi.fn(),
    ListPendingChanges: vi.fn(),
    DiscardPendingChange: vi.fn(),
    MoveIssueToColumn: vi.fn(),
    MoveIssueToSprint: vi.fn(),
    RankIssue: vi.fn(),
    CanTransition: vi.fn(),
    JournalSprintMoves: vi.fn(),
    StartSprint: vi.fn(),
    CompleteSprint: vi.fn(),
    SuggestSprintDates: vi.fn(),
    PendingInSprint: vi.fn(),
  };
});

// The shared sync state gates Refresh, and a selected card's
// IssueDetailPanel reads it to hold Save. Both come from one hoisted object
// so a test can put the shell into "a sync is running" before it renders.
const sync = vi.hoisted(() => ({
  status: "idle",
  canSync: true,
  lastBoards: null as api.BoardSummary | null,
  lastBoardsAt: 0,
  lastCommit: null as api.CommitResult | null,
  // The Refresh runs through SyncContext now, so that one lock covers a
  // refresh and a sync alike. The stub does what the real one does: call the
  // binding and record the pass where the banner reads it.
  runBoardsRefresh: async () => ({}) as api.BoardSummary,
  // The two ceremonies take Go's per-profile lock, so they run through the
  // reducer as well. The stub is what the real one is once the lock is
  // free: it runs the action and hands the answer back.
  runSprintCeremony: async <T,>(action: () => Promise<T>) => action(),
}));
vi.mock("../contexts/SyncContext", () => ({ useSync: () => sync }));

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "12", sprintName: "Sprint 12", parentKey: "", storyPoints: null, rank: "",
    created: "", updated: "",
    ...over,
  };
}

function board(over: Partial<BoardView>): BoardView {
  return {
    boardId: 1,
    sprintId: "12",
    swimlane: "none",
    columns: [],
    lanes: [],
    donePoints: 0,
    unmapped: 0,
    unmappedStatuses: [],
    notSynced: 0,
    capped: false,
    needsStatusSync: false,
    ...over,
  };
}

const COLUMNS = [
  { name: "To Do", statusIds: ["1"], total: 4, points: 20 },
  { name: "In Progress", statusIds: ["3"], total: 2, points: 0 },
  { name: "Done", statusIds: ["5"], total: 3, points: 27 },
];

const PROMO = issue({ key: "PLAT-412", type: "story", summary: "Checkout: apply promo code", status: "In Progress", assignee: "R. Anand", storyPoints: 8 });
const KEYS = issue({ key: "PLAT-409", summary: "Rotate payment gateway API keys", assignee: "M. Ortiz", storyPoints: 2 });
const RETRO = issue({ key: "PLAT-347", summary: "Write retro notes template", storyPoints: 1 });

// oneLane is the swimlane-none shape: a single catch-all band with one cell
// per column.
function oneLane(cells: Issue[][], overflow = [0, 0, 0]) {
  return board({
    columns: COLUMNS,
    lanes: [{ id: "", label: "All issues", count: cells.flat().length, cells, overflow }],
  });
}

// TRANSITION_ROW is one journaled column move, the row the board reads to
// know a card's position is provisional.
const TRANSITION_ROW: PendingChange = {
  id: 7, entityType: "issue_transition", entityKey: "PLAT-412", field: "statusId",
  beforeVal: "1|To Do", afterVal: "3|In Progress", baseVersion: "v1", createdAt: "",
};

const SPRINTS: api.Sprint[] = [
  { id: 12, boardId: 1, name: "Sprint 12", state: "active", startDate: "2026-08-29T09:00:00Z", endDate: "2026-09-12T09:00:00Z" },
  { id: 13, boardId: 1, name: "Sprint 13", state: "future", startDate: "", endDate: "" },
];

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

// Switcher stands in for the topbar's profile picker, which lives above
// BoardsView in the shell.
function Switcher() {
  const { setActiveId } = useProfile<api.Profile, api.Settings>();
  return <button type="button" onClick={() => setActiveId("p2")}>Switch profile</button>;
}

function renderView() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ModalProvider>
            <Loader />
            <Switcher />
            {/* The board announces every move it makes, so the region
                those announcements land in is part of what these tests
                render. A ceremony's sentence lands in both this region and
                the board's own banner, so anything asserting on one of them
                says which. */}
            <LiveRegion />
            <BoardsView />
          </ModalProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  sync.status = "idle";
  sync.canSync = true;
  sync.lastBoards = null;
  sync.lastBoardsAt = 0;
  sync.runBoardsRefresh = async () => {
    const sum = await api.SyncBoards("p1");
    sync.lastBoards = sum;
    sync.lastBoardsAt = Date.now();
    return sum;
  };
  sync.lastCommit = null;
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListBoards).mockResolvedValue([
    { id: 1, name: "Acme Platform Scrum", type: "scrum" },
    { id: 2, name: "Ops Kanban", type: "kanban" },
  ]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue(SPRINTS);
  vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS], [PROMO], [RETRO]]));
  vi.mocked(api.SyncBoards).mockResolvedValue({
    boards: 2, columns: 6, sprints: 3, cards: 40, dropped: [], unavailable: false, elapsed: "2s",
  });
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.GetSyncState).mockResolvedValue({
    lastSynced: new Date().toISOString(), lastFull: "", lastError: "", issueCount: 61,
  });
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: "PLAT-412", description: "", links: [], fields: {} });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
  vi.mocked(api.ListActivity).mockResolvedValue([]);
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.GetLinkTypes).mockResolvedValue([]);
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
  vi.mocked(api.DiscardPendingChange).mockResolvedValue();
  vi.mocked(api.MoveIssueToColumn).mockResolvedValue();
  vi.mocked(api.MoveIssueToSprint).mockResolvedValue();
  vi.mocked(api.RankIssue).mockResolvedValue();
  vi.mocked(api.CanTransition).mockResolvedValue({ reachable: [], allowed: true });
  vi.mocked(api.JournalSprintMoves).mockResolvedValue(3);
  vi.mocked(api.StartSprint).mockResolvedValue();
  vi.mocked(api.CompleteSprint).mockResolvedValue({ moved: 2, movedTo: "the backlog", failed: [], message: "" });
  vi.mocked(api.SuggestSprintDates).mockResolvedValue({
    name: "Sprint 14", start: "2026-09-14", end: "2026-09-28", length: 14, fromHistory: true,
  });
  vi.mocked(api.PendingInSprint).mockResolvedValue(0);
});

describe("BoardsView toolbar", () => {
  it("offers both boards and picks the first", async () => {
    renderView();
    const picker = await screen.findByRole("combobox", { name: "Board" });
    await waitFor(() => expect(picker).toHaveValue("1"));
    expect(within(picker).getByRole("option", { name: "Acme Platform Scrum" })).toBeInTheDocument();
    expect(within(picker).getByRole("option", { name: "Ops Kanban" })).toBeInTheDocument();
  });

  it("labels sprints with their capitalised state and asks for the active one", async () => {
    renderView();
    const picker = await screen.findByRole("combobox", { name: "Sprint" });
    await waitFor(() => expect(picker).toHaveValue("12"));
    expect(within(picker).getByRole("option", { name: "Sprint 12 (Active)" })).toBeInTheDocument();
    expect(within(picker).getByRole("option", { name: "Sprint 13 (Future)" })).toBeInTheDocument();
    await waitFor(() => expect(api.GetBoard).toHaveBeenCalledWith("p1", 1, "12", "none"));
  });

  it("hides the sprint picker when the chosen board is a kanban one", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("combobox", { name: "Sprint" });
    await user.selectOptions(screen.getByRole("combobox", { name: "Board" }), "2");
    expect(screen.queryByRole("combobox", { name: "Sprint" })).not.toBeInTheDocument();
    await waitFor(() => expect(api.GetBoard).toHaveBeenCalledWith("p1", 2, "", "none"));
  });

  it("reads the sprint dates and the backend's point split in the summary line", async () => {
    const done = issue({ key: "PLAT-501", summary: "Ship the changelog", status: "Done", storyPoints: 5 });
    const blocked = issue({ key: "PLAT-502", summary: "Vendor migration", status: "Blocked", storyPoints: 3 });
    vi.mocked(api.GetBoard).mockResolvedValue(
      board({
        columns: [
          { name: "To Do", statusIds: ["1"], total: 1, points: 3 },
          { name: "In Progress", statusIds: ["3"], total: 1, points: 5 },
          { name: "Blocked", statusIds: ["9"], total: 1, points: 3 },
        ],
        donePoints: 5,
        lanes: [{ id: "", label: "All issues", count: 2, cells: [[], [done], [blocked]], overflow: [0, 0, 0] }],
      }),
    );
    renderView();
    expect(
      await screen.findByText(/^Sprint 12, active, ends .+\. 5 of 11 points done\. Synced today/),
    ).toBeInTheDocument();
  });

  it("keeps the summary's done half whole when a cell is capped", async () => {
    // The Done column holds 900 points and the cell drew a fraction of
    // them. The line reads the backend's count, so it says 900 of 900
    // rather than counting the cards that happened to be rendered.
    const drawn = issue({ key: "PLAT-601", summary: "Ship the changelog", status: "Done", storyPoints: 2 });
    vi.mocked(api.GetBoard).mockResolvedValue(
      board({
        columns: [{ name: "To Do", statusIds: ["1"], total: 0, points: 0 }, { name: "Done", statusIds: ["5"], total: 450, points: 900 }],
        donePoints: 900,
        capped: true,
        lanes: [{ id: "", label: "All issues", count: 450, cells: [[], [drawn]], overflow: [0, 449] }],
      }),
    );
    renderView();
    expect(
      await screen.findByText(/^Sprint 12, active, ends .+\. 900 of 900 points done\./),
    ).toBeInTheDocument();
  });
});

describe("BoardsView board", () => {
  it("renders the columns with their card counts and point sums", async () => {
    renderView();
    expect(await screen.findByText("To Do")).toBeInTheDocument();
    expect(screen.getByText("4 cards, 20 pts")).toBeInTheDocument();
    // A column with no points prints the cards alone rather than "0 pts".
    expect(screen.getByText("2 cards")).toBeInTheDocument();
    expect(screen.getByText("3 cards, 27 pts")).toBeInTheDocument();
  });

  it("puts each card in its own column and names that column on the card", async () => {
    renderView();
    const promo = await screen.findByRole("gridcell", { name: "PLAT-412 Checkout: apply promo code In Progress" });
    expect(promo).toBeInTheDocument();
    expect(screen.getByRole("gridcell", { name: "PLAT-409 Rotate payment gateway API keys To Do" })).toBeInTheDocument();
    expect(screen.getByRole("gridcell", { name: "PLAT-347 Write retro notes template Done" })).toBeInTheDocument();
  });

  it("shows the assignee or Unassigned and the points on the card", async () => {
    renderView();
    const promo = await screen.findByRole("gridcell", { name: /PLAT-412/ });
    expect(within(promo).getByText("R. Anand")).toBeInTheDocument();
    expect(within(promo).getByText("8 pts")).toBeInTheDocument();
    const retro = screen.getByRole("gridcell", { name: /PLAT-347/ });
    expect(within(retro).getByText("Unassigned")).toBeInTheDocument();
  });

  it("lets a card be dragged, and says so nowhere else", async () => {
    renderView();
    const promo = await screen.findByRole("gridcell", { name: /PLAT-412/ });
    expect(promo).toHaveAttribute("draggable", "true");
    expect(screen.queryByText("Read only")).not.toBeInTheDocument();
  });

  it("stops a card being dragged while a commit is pushing", async () => {
    sync.status = "committing";
    renderView();
    const promo = await screen.findByRole("gridcell", { name: /PLAT-412/ });
    expect(promo).toHaveAttribute("draggable", "false");
  });

  it("switches to assignee lanes, keeps Unassigned last, and counts each lane", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    vi.mocked(api.GetBoard).mockResolvedValue(
      board({
        columns: COLUMNS,
        swimlane: "assignee",
        lanes: [
          { id: "R. Anand", label: "R. Anand", count: 1, cells: [[], [PROMO], []], overflow: [0, 0, 0] },
          { id: "M. Ortiz", label: "M. Ortiz", count: 1, cells: [[KEYS], [], []], overflow: [0, 0, 0] },
          { id: "", label: "Unassigned", count: 1, cells: [[], [], [RETRO]], overflow: [0, 0, 0] },
        ],
      }),
    );
    await user.selectOptions(screen.getByRole("combobox", { name: "Swimlanes" }), "assignee");
    await waitFor(() => expect(api.GetBoard).toHaveBeenCalledWith("p1", 1, "12", "assignee"));

    const lanes = await screen.findAllByRole("row", { name: /R\. Anand|M\. Ortiz|Unassigned/ });
    expect(lanes.map((l) => l.getAttribute("aria-label"))).toEqual(["R. Anand", "M. Ortiz", "Unassigned"]);
    expect(screen.getAllByText("1 card")).toHaveLength(3);
  });

  it("opens the detail panel for the card that was clicked", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByText("Checkout: apply promo code"));
    expect(await screen.findByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
  });

  it("names the unmapped statuses and counts what was never synced", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue({
      ...oneLane([[KEYS], [PROMO], [RETRO]]),
      unmapped: 3,
      unmappedStatuses: ["Approved", "Blocked"],
      notSynced: 2,
    });
    renderView();
    expect(await screen.findByText("3 cards are not on the board (Approved, Blocked)")).toBeInTheDocument();
    expect(screen.getByText("2 cards on this board have not been synced")).toBeInTheDocument();
  });

  // The filter is what a standup asks for out loud: where's mine, where are
  // the bugs, what is -124 doing.
  it("filters the cards by key, assignee, or type without losing a column", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText(KEYS.key);
    await user.type(screen.getByRole("searchbox", { name: "Filter cards" }), KEYS.key);
    await waitFor(() => expect(screen.queryByText(PROMO.key)).not.toBeInTheDocument());
    expect(screen.getByText(KEYS.key)).toBeInTheDocument();
    // Every column head stays, so a filter never reads as a lost column.
    for (const name of ["To Do", "In Progress", "Done"]) {
      expect(screen.getByRole("columnheader", { name: new RegExp(name) })).toBeInTheDocument();
    }
  });

  it("shows a capped cell's overflow while its column head still reads the true total", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue({
      ...oneLane([[KEYS], [PROMO], [RETRO]], [41, 0, 0]),
      capped: true,
    });
    renderView();
    expect(await screen.findByText("+41 more")).toBeInTheDocument();
    expect(screen.getByText("4 cards, 20 pts")).toBeInTheDocument();
    expect(screen.getByText("Showing the first 2000 cards of this board")).toBeInTheDocument();
  });
});

describe("BoardsView refresh", () => {
  it("syncs the boards and names the ones that were skipped", async () => {
    const user = userEvent.setup();
    vi.mocked(api.SyncBoards).mockResolvedValue({
      boards: 2, columns: 6, sprints: 3, cards: 40, dropped: ["Ops", "Platform"], unavailable: false, elapsed: "2s",
    });
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(screen.getByRole("button", { name: "Refresh" }));
    await waitFor(() => expect(api.SyncBoards).toHaveBeenCalledWith("p1"));
    expect(await screen.findByText("2 boards were skipped: Ops, Platform")).toBeInTheDocument();
  });

  it("says why a Refresh failed and offers a Retry", async () => {
    const user = userEvent.setup();
    vi.mocked(api.SyncBoards).mockRejectedValueOnce(new Error("a commit is already running for this profile"));
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(screen.getByRole("button", { name: "Refresh" }));

    const banner = await screen.findByText(/Could not refresh the boards: a commit is already running for this profile/);
    expect(banner.closest(".pending-banner-warn")).toBeInTheDocument();
    expect(screen.queryByText("Refreshing")).not.toBeInTheDocument();

    await user.click(within(banner.closest(".pending-banner-warn") as HTMLElement).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(api.SyncBoards).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(screen.queryByText(/Could not refresh the boards/)).not.toBeInTheDocument(),
    );
  });

  it("names the boards an ordinary sync skipped, without a Refresh being pressed", async () => {
    sync.lastBoards = {
      boards: 1, columns: 3, sprints: 2, cards: 12, dropped: ["Ops Kanban: 403 Forbidden"], unavailable: false, elapsed: "2s",
    };
    sync.lastBoardsAt = Date.now();
    renderView();
    expect(await screen.findByText("1 board was skipped: Ops Kanban: 403 Forbidden")).toBeInTheDocument();
    expect(api.SyncBoards).not.toHaveBeenCalled();
  });

  it("says a sync found no Agile API while earlier boards are still on screen", async () => {
    sync.lastBoards = {
      boards: 0, columns: 0, sprints: 0, cards: 0, dropped: [], unavailable: true, elapsed: "1s",
    };
    sync.lastBoardsAt = Date.now();
    renderView();
    expect(
      await screen.findByText("This Jira answered with no boards, so nothing below was refreshed."),
    ).toBeInTheDocument();
  });

  it("lets a Refresh replace what an earlier sync reported", async () => {
    const user = userEvent.setup();
    sync.lastBoards = {
      boards: 1, columns: 3, sprints: 2, cards: 12, dropped: ["Ops Kanban: 403 Forbidden"], unavailable: false, elapsed: "2s",
    };
    sync.lastBoardsAt = Date.now() - 60_000;
    renderView();
    await screen.findByText("1 board was skipped: Ops Kanban: 403 Forbidden");
    await user.click(screen.getByRole("button", { name: "Refresh" }));
    await waitFor(() => expect(screen.queryByText(/was skipped/)).not.toBeInTheDocument());
  });

  it("is disabled while the shared sync is running", async () => {
    sync.canSync = false;
    sync.status = "syncing";
    renderView();
    expect(await screen.findByRole("button", { name: "Refresh" })).toBeDisabled();
  });
});

describe("BoardsView states", () => {
  it("says it is loading the board before anything arrives", async () => {
    vi.mocked(api.GetBoard).mockReturnValue(new Promise<BoardView>(() => {}));
    renderView();
    expect(await screen.findByText("Loading the board")).toBeInTheDocument();
  });

  it("shows Refreshing beside the toolbar and keeps the board drawn", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    let resolve!: (v: BoardView) => void;
    vi.mocked(api.GetBoard).mockReturnValueOnce(new Promise<BoardView>((r) => { resolve = r; }));
    await user.selectOptions(screen.getByRole("combobox", { name: "Swimlanes" }), "epic");
    expect(await screen.findByText("Refreshing")).toBeInTheDocument();
    expect(screen.getByRole("gridcell", { name: /PLAT-412/ })).toBeInTheDocument();
    resolve(oneLane([[KEYS], [PROMO], [RETRO]]));
    await waitFor(() => expect(screen.queryByText("Refreshing")).not.toBeInTheDocument());
  });

  it("says the project has no boards, or no sync has run", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([]);
    renderView();
    expect(
      await screen.findByText("This project has no boards in Jira, or the sync has not run"),
    ).toBeInTheDocument();
  });

  it("says this Jira has no boards when the Agile API answered with none", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([]);
    vi.mocked(api.GetProfileSetting).mockResolvedValue("true");
    renderView();
    expect(await screen.findByText("This Jira has no boards")).toBeInTheDocument();
    expect(screen.queryByText(/the sync has not run/)).not.toBeInTheDocument();
  });

  it("warns when Jira gave the board no columns", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(board({ columns: [], lanes: [] }));
    renderView();
    const banner = await screen.findByText("Jira reports no columns for this board, so there is nothing to draw.");
    expect(banner.closest(".pending-banner-warn")).toBeInTheDocument();
  });

  it("asks for a sync when no cached card carries a status id yet", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(board({ columns: COLUMNS, needsStatusSync: true }));
    renderView();
    expect(await screen.findByText(/Sync to draw this board/)).toBeInTheDocument();
    expect(screen.queryByRole("grid")).not.toBeInTheDocument();
  });

  it("shows the plain empty state when the sprint holds nothing", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(board({ columns: COLUMNS, lanes: [] }));
    renderView();
    expect(await screen.findByText("No cards in this sprint")).toBeInTheDocument();
  });

  it("names no sprint on a board that has none", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetBoard).mockResolvedValue(board({ columns: COLUMNS, lanes: [] }));
    renderView();
    await screen.findByText("No cards in this sprint");
    await user.selectOptions(screen.getByRole("combobox", { name: "Board" }), "2");
    expect(await screen.findByText("No cards on this board")).toBeInTheDocument();
    expect(screen.queryByText(/this sprint/)).not.toBeInTheDocument();
  });

  it("says the uncached cards are uncached, not that they are elsewhere", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(board({ columns: COLUMNS, lanes: [], notSynced: 14 }));
    renderView();
    expect(
      await screen.findByText("No cards in this sprint have been synced. 14 cards are on it but not in this cache."),
    ).toBeInTheDocument();
    expect(screen.queryByText("No cards in this sprint")).not.toBeInTheDocument();
  });

  it("offers a Retry when the board could not be read", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetBoard).mockRejectedValueOnce(new Error("tam.db is locked"));
    renderView();
    expect(await screen.findByText(/Could not load the board: tam.db is locked/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("gridcell", { name: /PLAT-412/ })).toBeInTheDocument();
  });
});

describe("BoardsView keyboard", () => {
  const laneBoard = board({
    columns: COLUMNS,
    swimlane: "assignee",
    lanes: [
      { id: "R. Anand", label: "R. Anand", count: 3, cells: [[KEYS, RETRO], [PROMO], []], overflow: [0, 0, 0] },
      { id: "", label: "Unassigned", count: 1, cells: [[issue({ key: "PLAT-390", summary: "Promo code field accepts whitespace" })], [], []], overflow: [0, 0, 0] },
    ],
  });

  beforeEach(() => {
    vi.mocked(api.GetBoard).mockResolvedValue(laneBoard);
  });

  it("is one tab stop for the whole board", async () => {
    renderView();
    const grid = await screen.findByRole("grid", { name: "Board" });
    expect(grid.querySelectorAll('[tabindex="0"]')).toHaveLength(1);
  });

  it("moves a column to the right on ArrowRight", async () => {
    const user = userEvent.setup();
    renderView();
    const first = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(first);
    expect(document.activeElement).toBe(first);
    await user.keyboard("{ArrowRight}");
    expect(document.activeElement).toBe(screen.getByRole("gridcell", { name: /PLAT-412/ }));
  });

  it("moves down a cell and then into the next lane", async () => {
    const user = userEvent.setup();
    renderView();
    const first = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(first);
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(screen.getByRole("gridcell", { name: /PLAT-347/ }));
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(screen.getByRole("gridcell", { name: /PLAT-390/ }));
    await user.keyboard("{ArrowUp}");
    expect(document.activeElement).toBe(screen.getByRole("gridcell", { name: /PLAT-347/ }));
  });

  it("takes the first and last card of the cell on Home and End", async () => {
    const user = userEvent.setup();
    renderView();
    const first = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(first);
    await user.keyboard("{End}");
    expect(document.activeElement).toBe(screen.getByRole("gridcell", { name: /PLAT-347/ }));
    await user.keyboard("{Home}");
    expect(document.activeElement).toBe(screen.getByRole("gridcell", { name: /PLAT-409/ }));
  });

  it("opens the panel on Enter and checks the card on Space", async () => {
    const user = userEvent.setup();
    renderView();
    const first = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(first);
    await user.keyboard("{ArrowRight}{Enter}");
    expect(await screen.findByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
    // Space is the multi-selection's key now: it checks the focused card
    // and leaves the panel showing whatever it was showing.
    await user.keyboard("{ArrowLeft} ");
    expect(await screen.findByText("1 card selected")).toBeInTheDocument();
    expect(screen.getByRole("gridcell", { name: /PLAT-409/ })).toHaveClass("board-card-checked");
    expect(screen.getByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
  });

  it("extends the checked run with Shift and an arrow", async () => {
    const user = userEvent.setup();
    renderView();
    const first = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(first);
    await user.keyboard("{Shift>}{ArrowDown}{/Shift}");
    expect(await screen.findByText("2 cards selected")).toBeInTheDocument();
    expect(screen.getByRole("gridcell", { name: /PLAT-409/ })).toHaveClass("board-card-checked");
    expect(screen.getByRole("gridcell", { name: /PLAT-347/ })).toHaveClass("board-card-checked");
  });

  it("keeps exactly one focusable card after the swimlane changes under it", async () => {
    const user = userEvent.setup();
    renderView();
    const first = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(first);
    await user.keyboard("{ArrowDown}");
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS], [PROMO], [RETRO]]));
    await user.selectOptions(screen.getByRole("combobox", { name: "Swimlanes" }), "assignee");
    await waitFor(() => expect(screen.queryByRole("gridcell", { name: /PLAT-390/ })).not.toBeInTheDocument());
    const grid = screen.getByRole("grid", { name: "Board" });
    expect(grid.querySelectorAll('[tabindex="0"]')).toHaveLength(1);
  });
});

describe("BoardsView moves", () => {
  // jsdom lays nothing out, so a cell's cards are given forty-pixel boxes
  // stacked from the top of it. The drop arithmetic reads those and a
  // clientY, and nothing else.
  function layOut(cell: HTMLElement) {
    cell.querySelectorAll<HTMLElement>("[data-board-pos]").forEach((card, i) => {
      card.getBoundingClientRect = () => ({
        top: i * 40, bottom: i * 40 + 40, height: 40, width: 300, left: 0, right: 300, x: 0, y: i * 40,
        toJSON: () => "",
      });
    });
  }

  // A DataTransfer that records what the handlers put in it, since jsdom
  // has none of its own.
  function transfer() {
    const held: Record<string, string> = {};
    return {
      dropEffect: "",
      effectAllowed: "",
      setData: (k: string, v: string) => { held[k] = v; },
      getData: (k: string) => held[k] ?? "",
    };
  }

  // jsdom has no DragEvent, so testing-library builds a drag out of
  // window.Event, which drops every init member it does not know and then
  // re-attaches dataTransfer alone. clientY is one of the members it
  // drops, and a drop with no coordinate resolves to the bottom of the
  // cell whatever the test aimed at. A MouseEvent carries the coordinate;
  // the transfer is attached by hand, by reference, so the handlers'
  // writes to it are visible here.
  function fireDrag(el: Element, type: string, dataTransfer: ReturnType<typeof transfer>, clientY = 0) {
    const e = new MouseEvent(type, { bubbles: true, cancelable: true, clientY });
    Object.defineProperty(e, "dataTransfer", { value: dataTransfer });
    fireEvent(el, e);
  }

  function cells(): HTMLElement[] {
    return [...document.querySelectorAll<HTMLElement>(".board-cell")];
  }

  // startDrag picks a card up and lays out the cell it is aimed at, which
  // is everything the drop arithmetic reads.
  async function startDrag(cardName: RegExp, col: number) {
    const card = await screen.findByRole("gridcell", { name: cardName });
    const dataTransfer = transfer();
    fireDrag(card, "dragstart", dataTransfer);
    const cell = cells()[col];
    layOut(cell);
    return { card, cell, dataTransfer };
  }

  async function dragTo(cardName: RegExp, col: number, clientY: number) {
    const { cell, dataTransfer } = await startDrag(cardName, col);
    fireDrag(cell, "dragover", dataTransfer, clientY);
    fireDrag(cell, "drop", dataTransfer, clientY);
    return dataTransfer;
  }

  it("journals a transition when a card is dropped on another column, and repaints it there", async () => {
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-409/ });
    const moved = issue({ ...KEYS, status: "In Progress" });
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[], [PROMO, moved], [RETRO]]));

    await dragTo(/PLAT-409/, 1, 100);
    await waitFor(() => expect(api.MoveIssueToColumn).toHaveBeenCalledWith("p1", "PLAT-409", "3"));
    expect(api.RankIssue).not.toHaveBeenCalled();
    expect(
      await screen.findByRole("gridcell", { name: "PLAT-409 Rotate payment gateway API keys In Progress" }),
    ).toBeInTheDocument();
  });

  it("journals a rank when a card is dropped inside its own cell, with the neighbour and the side", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS, RETRO], [PROMO], []]));
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-347/ });

    await dragTo(/PLAT-347/, 0, 5);
    await waitFor(() => expect(api.RankIssue).toHaveBeenCalledWith("p1", "PLAT-347", "PLAT-409", true, 1));
    expect(api.MoveIssueToColumn).not.toHaveBeenCalled();
  });

  it("refuses a drop that would change nothing, and journals nothing", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS, RETRO], [PROMO], []]));
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-409/ });

    // The top of the first card is the first card's own place.
    const dataTransfer = await dragTo(/PLAT-409/, 0, 5);
    expect(dataTransfer.dropEffect).toBe("none");
    expect(document.querySelector(".board-drop-line")).toBeNull();
    expect(document.querySelector(".board-cell-over")).toBeNull();
    expect(api.RankIssue).not.toHaveBeenCalled();
    expect(api.MoveIssueToColumn).not.toHaveBeenCalled();
  });

  it("draws the drop line where the card would land", async () => {
    // Three forty-pixel cards, so their midpoints are 20, 60 and 100 and a
    // clientY picks a gap rather than always falling past the last card.
    const WEBHOOK = issue({ key: "PLAT-333", summary: "Retire the legacy webhook" });
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS, RETRO, WEBHOOK], [PROMO], []]));
    renderView();
    const { card, cell, dataTransfer } = await startDrag(/PLAT-409/, 0);
    expect(card).toHaveClass("board-card-dragging");

    // Above the third card's midpoint: the line is drawn in that card's
    // own slot, which is the box it is positioned against.
    fireDrag(cell, "dragover", dataTransfer, 70);
    expect(cell).toHaveClass("board-cell-over");
    const slots = [...cell.querySelectorAll(".board-card-slot")];
    expect(cell.querySelectorAll(".board-drop-line")).toHaveLength(1);
    expect(slots[2].firstElementChild).toHaveClass("board-drop-line");

    // Below the last card there is no slot to measure against, so the line
    // takes its own place in the cell rather than being positioned against
    // whatever ancestor happens to be positioned.
    fireDrag(cell, "dragover", dataTransfer, 200);
    const end = cell.querySelector(".board-drop-line");
    expect(end?.parentElement).toBe(cell);
    expect(end).toHaveClass("board-drop-line-end");
  });

  it("outlines the whole cell for a cross-column drop, and draws no line in it", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS, RETRO], [PROMO], []]));
    renderView();
    const { cell, dataTransfer } = await startDrag(/PLAT-409/, 1);
    fireDrag(cell, "dragover", dataTransfer, 5);
    // A column move lands the card at the rank it already has, so a gap
    // line would promise a place this move never takes.
    expect(cell).toHaveClass("board-cell-target");
    expect(cell.querySelector(".board-drop-line")).toBeNull();
  });

  it("refuses a drop below the cards a capped column is not showing, on the dragover", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue({
      ...oneLane([[KEYS, RETRO], [PROMO], []], [41, 0, 0]),
      capped: true,
    });
    renderView();
    const { cell, dataTransfer } = await startDrag(/PLAT-409/, 0);
    fireDrag(cell, "dragover", dataTransfer, 200);
    expect(dataTransfer.dropEffect).toBe("none");
    expect(cell.querySelector(".board-drop-line")).toBeNull();
    expect(cell).not.toHaveClass("board-cell-over");
  });

  it("says so when a column collects no status a card could be moved into", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(
      board({
        columns: [COLUMNS[0], { name: "Waiting", statusIds: [], total: 0, points: 0 }],
        lanes: [{ id: "", label: "All issues", count: 1, cells: [[KEYS], []], overflow: [0, 0] }],
      }),
    );
    renderView();
    const { cell, dataTransfer } = await startDrag(/PLAT-409/, 1);
    fireDrag(cell, "dragover", dataTransfer, 5);
    expect(dataTransfer.dropEffect).toBe("none");
    fireDrag(cell, "drop", dataTransfer, 5);
    expect(
      await screen.findByText("Waiting collects no status, so a card cannot be moved into it"),
    ).toBeInTheDocument();
    expect(api.MoveIssueToColumn).not.toHaveBeenCalled();
  });

  it("moves a card a column with Ctrl and an arrow", async () => {
    const user = userEvent.setup();
    renderView();
    const card = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(card);
    await user.keyboard("{Control>}{ArrowRight}{/Control}");
    await waitFor(() => expect(api.MoveIssueToColumn).toHaveBeenCalledWith("p1", "PLAT-409", "3"));
  });

  it("ranks a card inside its cell with Ctrl and an arrow", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS, RETRO], [PROMO], []]));
    renderView();
    const card = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(card);
    await user.keyboard("{Control>}{ArrowDown}{/Control}");
    await waitFor(() => expect(api.RankIssue).toHaveBeenCalledWith("p1", "PLAT-409", "PLAT-347", false, 1));
  });

  it("moves a card to another sprint from its own menu", async () => {
    const user = userEvent.setup();
    renderView();
    const card = await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(within(card).getByRole("button", { name: "Actions on PLAT-412" }));
    await user.click(await within(card).findByRole("menuitem", { name: "Move to Sprint 13" }));
    await waitFor(() => expect(api.MoveIssueToSprint).toHaveBeenCalledWith("p1", "PLAT-412", "13"));
    // The menu is not the card: opening it must not open the detail panel.
    expect(screen.queryByRole("heading", { name: "PLAT-412" })).not.toBeInTheDocument();
  });

  it("marks a card carrying a pending move rather than giving it the plain pending dot", async () => {
    vi.mocked(api.ListPendingChanges).mockResolvedValue([TRANSITION_ROW]);
    renderView();
    const card = await screen.findByRole("gridcell", { name: /PLAT-412/ });
    expect(await within(card).findByRole("img", { name: "Pending move" })).toBeInTheDocument();
    expect(within(card).queryByRole("img", { name: "Pending changes" })).not.toBeInTheDocument();
  });

  it("warns when the drop target cannot be reached, and offers to put the card back", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListPendingChanges).mockResolvedValue([{ ...TRANSITION_ROW, entityKey: "PLAT-409" }]);
    vi.mocked(api.CanTransition).mockResolvedValue({ reachable: ["Review", "Done"], allowed: false });
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-409/ });

    await dragTo(/PLAT-409/, 1, 100);
    const banner = await screen.findByText(
      "PLAT-409 cannot reach In Progress from where it is now. Jira offers Review, Done.",
    );
    expect(banner.closest(".pending-banner-warn")).toBeInTheDocument();
    expect(screen.getByRole("gridcell", { name: /PLAT-409/ })).toHaveClass("board-card-warn");

    await user.click(screen.getByRole("button", { name: "Put it back" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 7));
    await waitFor(() => expect(screen.queryByText(/cannot reach In Progress/)).not.toBeInTheDocument());
  });

  it("marks a card whose move failed at the last Commit, with the reason in its label", async () => {
    vi.mocked(api.ListPendingChanges).mockResolvedValue([TRANSITION_ROW]);
    sync.lastCommit = {
      committed: [], created: [], linked: [], moved: [], conflicts: [], remaining: 1,
      failures: [{
        key: "PLAT-412", entityType: "issue_transition", rowId: 7, retryable: false,
        error: "PLAT-412 cannot reach In Progress; it can reach Done",
      }],
    };
    renderView();
    const card = await screen.findByRole(
      "gridcell",
      { name: "PLAT-412 Checkout: apply promo code In Progress. PLAT-412 cannot reach In Progress; it can reach Done" },
    );
    expect(card).toHaveClass("board-card-failed");
    // The move was pushed and refused, so the dot that promises it is
    // about to land would be the one thing this card must not wear.
    expect(within(card).queryByRole("img", { name: "Pending move" })).not.toBeInTheDocument();
  });

  it("keeps focus on the card it moved to another column", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("gridcell", { name: /PLAT-409/ }));
    vi.mocked(api.GetBoard).mockResolvedValue(
      oneLane([[], [PROMO, issue({ ...KEYS, status: "In Progress" })], [RETRO]]),
    );
    await user.keyboard("{Control>}{ArrowRight}{/Control}");
    // The move unmounts the card and mounts a new one in the other cell,
    // so this is only true when the board remembered that it held focus
    // before the move rather than asking afterwards.
    await waitFor(() => expect(screen.getByRole("gridcell", { name: /PLAT-409/ })).toHaveFocus());

    vi.mocked(api.GetBoard).mockResolvedValue(
      oneLane([[], [PROMO], [RETRO, issue({ ...KEYS, status: "Done" })]]),
    );
    await user.keyboard("{Control>}{ArrowRight}{/Control}");
    await waitFor(() => expect(api.MoveIssueToColumn).toHaveBeenCalledWith("p1", "PLAT-409", "5"));
  });

  it("refuses to reorder a draft, rather than announcing a move it never made", async () => {
    const user = userEvent.setup();
    const draft = issue({ key: "TAM-NEW-3", summary: "drafted", draft: true });
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS, draft, RETRO], [PROMO], []]));
    renderView();
    await user.click(await screen.findByRole("gridcell", { name: /TAM-NEW-3/ }));
    await user.keyboard("{Control>}{ArrowUp}{/Control}");
    expect(
      await screen.findByText("TAM-NEW-3 cannot be reordered until it is created"),
    ).toBeInTheDocument();
    expect(api.RankIssue).not.toHaveBeenCalled();
  });

  it("ranks a card past a draft rather than against it", async () => {
    const user = userEvent.setup();
    const draft = issue({ key: "TAM-NEW-3", summary: "drafted", draft: true });
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS, draft, RETRO], [PROMO], []]));
    renderView();
    await user.click(await screen.findByRole("gridcell", { name: /PLAT-347/ }));
    await user.keyboard("{Control>}{ArrowUp}{/Control}");
    await waitFor(() =>
      expect(api.RankIssue).toHaveBeenCalledWith("p1", "PLAT-347", "PLAT-409", true, 1),
    );
  });

  it("announces where a moved card landed", async () => {
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-409/ });
    vi.mocked(api.GetBoard).mockResolvedValue(
      oneLane([[], [PROMO, issue({ ...KEYS, status: "In Progress" })], [RETRO]]),
    );
    await dragTo(/PLAT-409/, 1, 100);
    expect(await screen.findByText("PLAT-409 moved to In Progress, 2 of 2")).toBeInTheDocument();
  });

  it("refuses a keyboard move while a commit is pushing, the way the drag and the menu do", async () => {
    sync.status = "committing";
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("gridcell", { name: /PLAT-409/ }));
    await user.keyboard("{Control>}{ArrowRight}{/Control}");
    expect(
      await screen.findByText("PLAT-409 cannot be moved while a commit is running"),
    ).toBeInTheDocument();
    expect(api.MoveIssueToColumn).not.toHaveBeenCalled();
  });

  it("opens a card's move menu from the keyboard and puts focus in it", async () => {
    const user = userEvent.setup();
    renderView();
    const card = await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(card);
    await user.keyboard("{Shift>}{F10}{/Shift}");
    // The panel is a state change React flushes when the key handler
    // returns, so the focus has to wait for the render rather than run in
    // the press that asked for it.
    await waitFor(() =>
      expect(within(card).getByRole("menuitem", { name: "Move to To Do" })).toHaveFocus(),
    );
  });

  it("drops the warning when the move it warned about is discarded elsewhere", async () => {
    const user = userEvent.setup();
    let rows: PendingChange[] = [{ ...TRANSITION_ROW, entityKey: "PLAT-409" }];
    vi.mocked(api.ListPendingChanges).mockImplementation(async () => rows);
    vi.mocked(api.CanTransition).mockResolvedValue({ reachable: ["Done"], allowed: false });
    vi.mocked(api.GetBoard).mockResolvedValue(oneLane([[KEYS, RETRO], [PROMO], []]));
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-409/ });

    await dragTo(/PLAT-409/, 1, 100);
    await screen.findByText(/PLAT-409 cannot reach In Progress/);

    // The Pending changes dialog, Discard all, and a Commit that pushed
    // the row all end the same way: the row is gone on the next read.
    rows = [];
    await user.click(screen.getByRole("gridcell", { name: /PLAT-347/ }));
    await user.keyboard("{Control>}{ArrowUp}{/Control}");
    await waitFor(() => expect(api.RankIssue).toHaveBeenCalled());
    await waitFor(() =>
      expect(screen.queryByText(/PLAT-409 cannot reach In Progress/)).not.toBeInTheDocument(),
    );
  });
});

describe("BoardsView selection", () => {
  // A control click is the gesture that starts a multi-selection, and it is
  // the modifier that has to reach the card: userEvent's click helper does
  // not carry one, so the event is fired with it set.
  function check(card: HTMLElement) {
    fireEvent.click(card, { ctrlKey: true });
  }

  it("counts the checked cards and marks each one", async () => {
    renderView();
    const keys = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    check(keys);
    check(screen.getByRole("gridcell", { name: /PLAT-412/ }));
    check(screen.getByRole("gridcell", { name: /PLAT-347/ }));
    expect(await screen.findByText("3 cards selected")).toBeInTheDocument();
    for (const key of [/PLAT-409/, /PLAT-412/, /PLAT-347/]) {
      expect(screen.getByRole("gridcell", { name: key })).toHaveClass("board-card-checked");
    }
    // The grid says it holds more than one selection now.
    expect(screen.getByRole("grid", { name: "Board" })).toHaveAttribute("aria-multiselectable");
  });

  it("moves every checked card in one call to the bulk binding", async () => {
    const user = userEvent.setup();
    renderView();
    check(await screen.findByRole("gridcell", { name: /PLAT-409/ }));
    check(screen.getByRole("gridcell", { name: /PLAT-412/ }));
    check(screen.getByRole("gridcell", { name: /PLAT-347/ }));
    await screen.findByText("3 cards selected");

    await user.selectOptions(screen.getByRole("combobox", { name: "Move the selected cards to" }), "13");
    await user.click(screen.getByRole("button", { name: "Move 3 cards" }));

    await waitFor(() =>
      expect(api.JournalSprintMoves).toHaveBeenCalledWith("p1", ["PLAT-409", "PLAT-412", "PLAT-347"], "13"),
    );
    expect(api.JournalSprintMoves).toHaveBeenCalledTimes(1);
    // One journal write for the whole selection, never one call per card.
    expect(api.MoveIssueToSprint).not.toHaveBeenCalled();
  });

  it("closes the detail panel while more than one card is checked", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("gridcell", { name: /PLAT-412/ }));
    expect(await screen.findByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
    check(screen.getByRole("gridcell", { name: /PLAT-412/ }));
    check(screen.getByRole("gridcell", { name: /PLAT-409/ }));
    await screen.findByText("2 cards selected");
    expect(screen.queryByRole("heading", { name: "PLAT-412" })).not.toBeInTheDocument();
  });

  it("clears the selection when the sprint changes under it", async () => {
    const user = userEvent.setup();
    renderView();
    check(await screen.findByRole("gridcell", { name: /PLAT-409/ }));
    check(screen.getByRole("gridcell", { name: /PLAT-412/ }));
    await screen.findByText("2 cards selected");
    await user.selectOptions(screen.getByRole("combobox", { name: "Sprint" }), "13");
    await waitFor(() => expect(screen.queryByText(/cards selected/)).not.toBeInTheDocument());
  });

  it("clears the selection on a plain click and on Clear", async () => {
    const user = userEvent.setup();
    renderView();
    check(await screen.findByRole("gridcell", { name: /PLAT-409/ }));
    check(screen.getByRole("gridcell", { name: /PLAT-412/ }));
    await screen.findByText("2 cards selected");
    await user.click(screen.getByRole("gridcell", { name: /PLAT-347/ }));
    await waitFor(() => expect(screen.queryByText(/cards selected/)).not.toBeInTheDocument());

    check(screen.getByRole("gridcell", { name: /PLAT-409/ }));
    await screen.findByText("1 card selected");
    await user.click(screen.getByRole("button", { name: "Clear" }));
    await waitFor(() => expect(screen.queryByText(/card selected/)).not.toBeInTheDocument());
  });

  // Enter opens the panel for one card, which is the same thing a plain
  // click is, so it empties the selection the same way a plain click does.
  it("clears the selection on Enter, which is the panel's own gesture", async () => {
    const user = userEvent.setup();
    renderView();
    const card = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    check(card);
    check(screen.getByRole("gridcell", { name: /PLAT-412/ }));
    await screen.findByText("2 cards selected");

    card.focus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(screen.queryByText(/cards selected/)).not.toBeInTheDocument());
    expect(await screen.findByRole("heading", { name: "PLAT-409" })).toBeInTheDocument();
  });

  // The board is the scroller, so a Space the board does not swallow scrolls
  // the cards out from under the focused one. Every Space is suppressed;
  // only the unmodified one checks.
  it("swallows a modified Space rather than letting it scroll the board", async () => {
    renderView();
    const card = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    card.focus();
    expect(fireEvent.keyDown(card, { key: " ", shiftKey: true })).toBe(false);
    expect(fireEvent.keyDown(card, { key: " ", ctrlKey: true })).toBe(false);
    expect(screen.queryByText(/card selected/)).not.toBeInTheDocument();
  });

  // Two profiles over the same project key draw the same issue keys, so a
  // selection carried across a switch would look entirely plausible and act
  // on the wrong profile's cards.
  it("clears the selection when the profile changes under it", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListProfiles).mockResolvedValue([
      { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
      { id: "p2", name: "Acme Platform (staging)", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
    ]);
    renderView();
    check(await screen.findByRole("gridcell", { name: /PLAT-409/ }));
    check(screen.getByRole("gridcell", { name: /PLAT-412/ }));
    await screen.findByText("2 cards selected");

    await user.click(screen.getByRole("button", { name: "Switch profile" }));
    // Waiting for the board the new profile drew, so this is the selection
    // after the redraw rather than the gap while one is in flight.
    await waitFor(() => expect(api.GetBoard).toHaveBeenCalledWith("p2", 1, "12", "none"));
    await screen.findByRole("gridcell", { name: /PLAT-409/ });
    expect(screen.queryByText(/cards selected/)).not.toBeInTheDocument();
  });

  // The backlog is always somewhere to put a card, and both the card's own
  // menu and the panel's sprint field offer it, so the bar cannot be the one
  // surface saying there is nowhere to move them.
  it("moves the checked cards to the backlog when that is the destination", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListBoardSprints).mockResolvedValue([SPRINTS[0]]);
    renderView();
    check(await screen.findByRole("gridcell", { name: /PLAT-409/ }));
    check(screen.getByRole("gridcell", { name: /PLAT-412/ }));
    await screen.findByText("2 cards selected");

    const picker = screen.getByRole("combobox", { name: "Move the selected cards to" });
    expect(within(picker).getByRole("option", { name: "The backlog" })).toBeInTheDocument();
    await user.selectOptions(picker, "backlog");
    await user.click(screen.getByRole("button", { name: "Move 2 cards" }));

    await waitFor(() =>
      expect(api.JournalSprintMoves).toHaveBeenCalledWith("p1", ["PLAT-409", "PLAT-412"], ""),
    );
  });
});

describe("BoardsView sprint ceremonies", () => {
  // The picker defaults to the active sprint, so starting one means asking
  // for the future sprint first.
  async function openStart(user: ReturnType<typeof userEvent.setup>) {
    await user.selectOptions(await screen.findByRole("combobox", { name: "Sprint" }), "13");
    await user.click(await screen.findByRole("button", { name: "Start sprint" }));
    return screen.findByRole("dialog", { name: "Start Sprint 13" });
  }

  it("offers a start for a future sprint and a completion for the active one", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    expect(screen.getByRole("button", { name: "Complete sprint" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Start sprint" })).not.toBeInTheDocument();

    await user.selectOptions(screen.getByRole("combobox", { name: "Sprint" }), "13");
    expect(await screen.findByRole("button", { name: "Start sprint" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Complete sprint" })).not.toBeInTheDocument();
  });

  it("fills the start dialog's dates from the suggestion and says where they came from", async () => {
    const user = userEvent.setup();
    renderView();
    const dialog = await openStart(user);
    await waitFor(() => expect(within(dialog).getByLabelText("Start")).toHaveValue("2026-09-14"));
    expect(within(dialog).getByLabelText("End")).toHaveValue("2026-09-28");
    expect(within(dialog).getByText(/Suggested 14 days from this board's last sprints/)).toBeInTheDocument();
    // The sprint already active on the board is named rather than guessed at.
    expect(within(dialog).getByText(/Sprint 12 is already active on this board/)).toBeInTheDocument();
  });

  it("says a fortnight is the default when the board has no history to measure", async () => {
    const user = userEvent.setup();
    vi.mocked(api.SuggestSprintDates).mockResolvedValue({
      name: "", start: "2026-09-14", end: "2026-09-28", length: 14, fromHistory: false,
    });
    renderView();
    const dialog = await openStart(user);
    expect(
      await within(dialog).findByText(/No closed sprint on this board to measure, so this is the 14 days default/),
    ).toBeInTheDocument();
  });

  it("starts the sprint with the dates the dialog collected", async () => {
    const user = userEvent.setup();
    renderView();
    const dialog = await openStart(user);
    await waitFor(() => expect(within(dialog).getByLabelText("Start")).toHaveValue("2026-09-14"));
    await user.click(within(dialog).getByRole("button", { name: "Start sprint" }));
    await waitFor(() =>
      expect(api.StartSprint).toHaveBeenCalledWith("p1", 1, 13, "Sprint 13", "", "2026-09-14", "2026-09-28"),
    );
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Start Sprint 13" })).not.toBeInTheDocument());
    // Scoped to the banner: the same sentence also goes to the shared live
    // region, so an unscoped query matches twice as soon as the region's
    // own timer fires.
    const banner = await screen.findByRole("status", { name: "Sprint ceremony" });
    expect(within(banner).getByText("Sprint 13 is running, 2026-09-14 to 2026-09-28.")).toBeInTheDocument();
  });

  it("refuses an end date before the start without asking Jira", async () => {
    const user = userEvent.setup();
    renderView();
    const dialog = await openStart(user);
    await waitFor(() => expect(within(dialog).getByLabelText("End")).toHaveValue("2026-09-28"));
    fireEvent.change(within(dialog).getByLabelText("End"), { target: { value: "2026-09-01" } });
    await user.click(within(dialog).getByRole("button", { name: "Start sprint" }));
    expect(await within(dialog).findByText("The sprint ends before it starts.")).toBeInTheDocument();
    expect(api.StartSprint).not.toHaveBeenCalled();
  });

  it("keeps the start dialog open with Jira's message when the start fails", async () => {
    const user = userEvent.setup();
    vi.mocked(api.StartSprint).mockRejectedValue(new Error("Sprint 12 is already active on this board"));
    renderView();
    const dialog = await openStart(user);
    await waitFor(() => expect(within(dialog).getByLabelText("Start")).toHaveValue("2026-09-14"));
    await user.click(within(dialog).getByRole("button", { name: "Start sprint" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Sprint 12 is already active on this board");
    // Still open, still holding what the user typed, and the button is the
    // retry: there is no offline state to disable it for.
    expect(screen.getByRole("dialog", { name: "Start Sprint 13" })).toBeInTheDocument();
    expect(within(dialog).getByLabelText("End")).toHaveValue("2026-09-28");
    expect(within(dialog).getByRole("button", { name: "Start sprint" })).toBeEnabled();
  });

  it("names every unfinished card in the completion, and where they can go", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(screen.getByRole("button", { name: "Complete sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "Complete Sprint 12" });

    // Two of the three cards are outside the board's last column, and the
    // dialog names them rather than counting them.
    expect(within(dialog).getByText("2 cards are not finished and will move out of the sprint:")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-409")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-412")).toBeInTheDocument();
    expect(within(dialog).queryByText("PLAT-347")).not.toBeInTheDocument();
    expect(within(dialog).getByText(/A card counts as finished when it sits in Done/)).toBeInTheDocument();
    expect(within(dialog).getByText(/cannot be reversed from TAM/)).toBeInTheDocument();

    const destination = within(dialog).getByRole("combobox", { name: "Move them to" });
    expect(within(destination).getByRole("option", { name: "The backlog" })).toBeInTheDocument();
    expect(within(destination).getByRole("option", { name: "Sprint 13" })).toBeInTheDocument();
  });

  it("completes into the chosen sprint and says what moved where", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CompleteSprint).mockResolvedValue({ moved: 2, movedTo: "Sprint 13", failed: [], message: "" });
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(screen.getByRole("button", { name: "Complete sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "Complete Sprint 12" });
    await user.selectOptions(within(dialog).getByRole("combobox", { name: "Move them to" }), "13");
    await user.click(within(dialog).getByRole("button", { name: "Complete sprint" }));

    await waitFor(() => expect(api.CompleteSprint).toHaveBeenCalledWith("p1", 1, 12, "13"));
    const banner = await screen.findByRole("status", { name: "Sprint ceremony" });
    expect(
      within(banner).getByText("Sprint 12 is closed. 2 unfinished cards moved to Sprint 13."),
    ).toBeInTheDocument();
    // The picker moved to where the cards went, rather than to whatever is
    // first in the list.
    await waitFor(() => expect(screen.getByRole("combobox", { name: "Sprint" })).toHaveValue("13"));
  });

  // A push that fell over partway comes back as a completion carrying its
  // own message, never as a rejection: Wails hands the frontend the value or
  // the error and never both, and the keys are the half that matters. The
  // dialog then stops promising a move and names the cards that did not make
  // it, which is the one thing the user needs when every unfinished card has
  // already left an open sprint.
  it("names the cards a half finished completion did not move", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CompleteSprint).mockResolvedValue({
      moved: 1,
      movedTo: "the backlog",
      failed: ["PLAT-412"],
      message: "1 of 2 unfinished issues moved to the backlog, so the sprint was left open: 403 Forbidden",
    });
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(screen.getByRole("button", { name: "Complete sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "Complete Sprint 12" });
    await user.click(within(dialog).getByRole("button", { name: "Complete sprint" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("the sprint was left open");
    expect(screen.getByRole("dialog", { name: "Complete Sprint 12" })).toBeInTheDocument();
    // The list is about the cards that did not move now, and says so rather
    // than promising a move over a list that has changed meaning underneath.
    expect(
      within(dialog).getByText("1 card did not move and is still in Sprint 12:"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-412")).toBeInTheDocument();
    expect(within(dialog).queryByText("PLAT-409")).not.toBeInTheDocument();
    // The foot reports what happened instead of promising what will.
    expect(within(dialog).getByText("1 card moved to the backlog.")).toBeInTheDocument();
    expect(within(dialog).queryByText(/cards move to/)).not.toBeInTheDocument();
  });

  it("keeps the completion dialog open with Jira's own sentence when a close fails", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CompleteSprint).mockRejectedValue(
      new Error("2 of 2 unfinished issues moved to the backlog, but the sprint could not be closed and is open with none of them in it: 403"),
    );
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(screen.getByRole("button", { name: "Complete sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "Complete Sprint 12" });
    await user.click(within(dialog).getByRole("button", { name: "Complete sprint" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("could not be closed");
    expect(screen.getByRole("dialog", { name: "Complete Sprint 12" })).toBeInTheDocument();
    // The cards it was moving are still named above the message.
    expect(within(dialog).getByText("PLAT-409")).toBeInTheDocument();
  });

  // The list is the drawn board, and a drawn board is not the whole sprint.
  // Cards past a cell's cap, cards the sync has not fetched, and cards whose
  // status no column collects are in no cell to be read out of, so the
  // dialog says how many it cannot show rather than presenting the list as
  // the sprint.
  it("says the list is the last sync and how many cards it cannot show", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetBoard).mockResolvedValue({
      ...oneLane([[KEYS], [PROMO], [RETRO]], [2, 0, 4]),
      unmapped: 3,
      unmappedStatuses: ["Blocked"],
      notSynced: 1,
      capped: true,
    });
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(screen.getByRole("button", { name: "Complete sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "Complete Sprint 12" });

    expect(within(dialog).getByText(/This is the board as TAM last synced it/)).toBeInTheDocument();
    // Two in the first column's overflow, three unmapped and one unsynced.
    // The four in the last column's overflow have finished, so they are not
    // cards this list is meant to be naming.
    expect(within(dialog).getByText(/^6 cards on this board are not drawn/)).toBeInTheDocument();
  });

  it("stops the completion before the dialog opens when the sprint has pending work", async () => {
    const user = userEvent.setup();
    vi.mocked(api.PendingInSprint).mockResolvedValue(3);
    renderView();
    await screen.findByRole("gridcell", { name: /PLAT-412/ });
    await user.click(screen.getByRole("button", { name: "Complete sprint" }));
    expect(await screen.findByText("Commit before completing this sprint?")).toBeInTheDocument();
    expect(screen.getByText(/3 pending changes belong to cards staying in this sprint/)).toBeInTheDocument();
    expect(screen.queryByRole("dialog", { name: "Complete Sprint 12" })).not.toBeInTheDocument();
    expect(api.CompleteSprint).not.toHaveBeenCalled();
  });
});
