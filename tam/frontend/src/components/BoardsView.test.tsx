import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { BoardView, Issue } from "../api";
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
  };
});

// The shared sync state gates Refresh, and a selected card's
// IssueDetailPanel reads it to hold Save. Both come from one hoisted object
// so a test can put the shell into "a sync is running" before it renders.
const sync = vi.hoisted(() => ({ status: "idle", canSync: true }));
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
    unmapped: 0,
    unmappedStatuses: [],
    notSynced: 0,
    capped: false,
    needsStatusSync: false,
    ...over,
  };
}

const COLUMNS = [
  { name: "To Do", total: 4, points: 20 },
  { name: "In Progress", total: 2, points: 0 },
  { name: "Done", total: 3, points: 27 },
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

const SPRINTS: api.Sprint[] = [
  { id: 12, boardId: 1, name: "Sprint 12", state: "active", startDate: "2026-08-29T09:00:00Z", endDate: "2026-09-12T09:00:00Z" },
  { id: 13, boardId: 1, name: "Sprint 13", state: "future", startDate: "", endDate: "" },
];

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

  it("reads the sprint dates and the point split in the summary line", async () => {
    renderView();
    expect(
      await screen.findByText(/^Sprint 12, active, ends .+\. 27 of 47 points done\. Synced today/),
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

  it("refuses a drag where the drag happens", async () => {
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
    expect(
      screen.getByText(
        "Columns and cards follow your board's configuration. Quick filters and the board's own swimlanes are not applied.",
      ),
    ).toBeInTheDocument();
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

  it("warns when the board's configuration gave it no columns", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(board({ columns: [], lanes: [] }));
    renderView();
    const banner = await screen.findByText("This board's configuration could not be read, so it has no columns.");
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

  it("does not call an unsynced sprint empty", async () => {
    vi.mocked(api.GetBoard).mockResolvedValue(board({ columns: COLUMNS, lanes: [], notSynced: 14 }));
    renderView();
    expect(
      await screen.findByText("No cards in this sprint have been synced. 14 sit outside this project."),
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

  it("selects the focused card on Enter and on Space", async () => {
    const user = userEvent.setup();
    renderView();
    const first = await screen.findByRole("gridcell", { name: /PLAT-409/ });
    await user.click(first);
    await user.keyboard("{ArrowRight}{Enter}");
    expect(await screen.findByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
    await user.keyboard("{ArrowLeft} ");
    expect(await screen.findByRole("heading", { name: "PLAT-409" })).toBeInTheDocument();
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
