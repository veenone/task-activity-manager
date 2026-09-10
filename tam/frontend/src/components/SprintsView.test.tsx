import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, LiveRegion, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { BoardView, Issue, SprintDetail } from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { SprintsView } from "./SprintsView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    ListBoards: vi.fn(),
    ListBoardSprintDetails: vi.fn(),
    GetBoard: vi.fn(),
    JournalSprintMoves: vi.fn(),
    CreateSprint: vi.fn(),
    EditSprint: vi.fn(),
    DeleteSprint: vi.fn(),
    StartSprint: vi.fn(),
    CompleteSprint: vi.fn(),
    SuggestSprintDates: vi.fn(),
    PendingInSprint: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
    ListActivity: vi.fn(),
    ListEpics: vi.fn(),
    GetLinkTypes: vi.fn(),
    ListPendingChanges: vi.fn(),
    SearchUsers: vi.fn(),
    ListPriorities: vi.fn(),
    GetSubtaskTypeName: vi.fn(),
  };
});

// The five sprint writes take Go's per-profile lock; the stubs are what the
// real ones are once it is free. SyncContext.test.tsx tests the lock itself,
// and the detail panel reads status to hold Save during a sync.
vi.mock("../contexts/SyncContext", () => ({
  useSync: () => ({
    status: "idle",
    runQuietLock: async <T,>(action: () => Promise<T>) => action(),
    runSprintCeremony: async <T,>(action: () => Promise<T>) => action(),
  }),
}));

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", statusId: "1",
    assignee: "", reporter: "", priority: "", labels: [], sprintId: "12", sprintName: "Sprint 12",
    parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
    ...over,
  };
}

const PROMO = issue({ key: "PLAT-412", summary: "Apply promo code", assignee: "R. Anand", storyPoints: 8, statusId: "3" });
const KEYS = issue({ key: "PLAT-409", summary: "Rotate the gateway keys", assignee: "M. Ortiz", storyPoints: 2 });
const RETRO = issue({ key: "PLAT-347", summary: "Write retro notes", status: "Done", statusId: "5", storyPoints: 3 });

function detail(over: Partial<SprintDetail>): SprintDetail {
  const base: SprintDetail = {
    id: 12, boardId: 1, name: "Sprint 12", state: "active",
    startDate: "2026-08-29T09:00:00.000+0000", endDate: "2026-09-12T09:00:00.000+0000", goal: "Ship checkout",
    issues: [], total: 0, done: 0, points: 0, donePoints: 0,
    membershipCached: true, notSynced: 0, truncated: false,
    ...over,
  };
  if (over.total !== undefined) return base;
  return {
    ...base,
    total: base.issues.length,
    done: base.issues.filter((i) => i.status === "Done").length,
    points: base.issues.reduce((n, i) => n + (i.storyPoints ?? 0), 0),
    donePoints: base.issues.filter((i) => i.status === "Done").reduce((n, i) => n + (i.storyPoints ?? 0), 0),
  };
}

// Sprint 13's own card, for the gesture that tries to run into it from
// Sprint 12.
const DRAFT = issue({ key: "PLAT-500", summary: "Draft the migration", sprintId: "13", sprintName: "Sprint 13" });

const ACTIVE = detail({ issues: [PROMO, KEYS, RETRO] });
const FUTURE = detail({ id: 13, name: "Sprint 13", state: "future", goal: "", startDate: "", endDate: "", issues: [] });
const CLOSED = detail({ id: 11, name: "Sprint 11", state: "closed", goal: "", issues: [], membershipCached: false });
const BACKLOG = detail({
  id: 0, name: "Board backlog", state: "unassigned", startDate: "", endDate: "", goal: "",
  issues: [issue({ key: "PLAT-900", summary: "Unscheduled work", sprintId: "", sprintName: "" })],
});

const COLUMNS = [
  { name: "To Do", statusIds: ["1"], total: 1, points: 2 },
  { name: "In Progress", statusIds: ["3"], total: 1, points: 8 },
  { name: "Done", statusIds: ["5"], total: 1, points: 3 },
];

function boardView(): BoardView {
  return {
    boardId: 1, sprintId: "", swimlane: "none", columns: COLUMNS, lanes: [],
    donePoints: 3, unmapped: 0, unmappedStatuses: [], notSynced: 0, capped: false, needsStatusSync: false,
  };
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
            {/* Every sprint write announces itself as well as banners
                itself, so anything asserting on one of them says which. */}
            <LiveRegion />
            <SprintsView />
          </ModalProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const banner = () => screen.getByRole("status", { name: "Sprint outcome" });

async function openMenu(user: ReturnType<typeof userEvent.setup>, sprint: string) {
  await user.click(await screen.findByRole("button", { name: `Actions on ${sprint}` }));
  return screen.getByRole("menu");
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListBoards).mockResolvedValue([
    { id: 1, name: "Acme Platform Scrum", type: "scrum" },
    { id: 2, name: "Ops Kanban", type: "kanban" },
  ]);
  vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([ACTIVE, FUTURE, CLOSED, BACKLOG]);
  vi.mocked(api.GetBoard).mockResolvedValue(boardView());
  vi.mocked(api.JournalSprintMoves).mockResolvedValue(2);
  vi.mocked(api.CreateSprint).mockResolvedValue({
    sprint: { id: 14, boardId: 1, name: "Sprint 14", state: "future", startDate: "", endDate: "", goal: "" },
    note: "",
  });
  vi.mocked(api.EditSprint).mockResolvedValue("");
  vi.mocked(api.DeleteSprint).mockResolvedValue("");
  vi.mocked(api.StartSprint).mockResolvedValue("");
  vi.mocked(api.CompleteSprint).mockResolvedValue({ moved: 2, movedTo: "the backlog", failed: [], note: "", message: "" });
  vi.mocked(api.SuggestSprintDates).mockResolvedValue({
    name: "Sprint 14", start: "2026-09-14", end: "2026-09-28", length: 14, fromHistory: true,
  });
  vi.mocked(api.PendingInSprint).mockResolvedValue(0);
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: "PLAT-412", description: "", links: [], fields: {} });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
  vi.mocked(api.ListActivity).mockResolvedValue([]);
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.GetLinkTypes).mockResolvedValue([]);
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
  vi.mocked(api.SearchUsers).mockResolvedValue([]);
  vi.mocked(api.ListPriorities).mockResolvedValue(["High"]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
});

describe("SprintsView", () => {
  it("draws every sprint on the board with its state, dates and progress", async () => {
    renderView();
    expect(await screen.findByRole("treeitem", { name: "Sprint 12, Active" })).toBeInTheDocument();
    expect(screen.getByRole("treeitem", { name: "Sprint 13, Future" })).toBeInTheDocument();
    expect(screen.getByText("1 of 3 done, 3 of 13 pts")).toBeInTheDocument();
    // Closed sprints are behind the toggle, and the board's own work is not
    // a sprint and is never behind it.
    expect(screen.queryByRole("treeitem", { name: /Sprint 11/ })).not.toBeInTheDocument();
    expect(screen.getByRole("treeitem", { name: "Board backlog" })).toBeInTheDocument();
  });

  it("orients the reader with one line about the sprint that is running", async () => {
    renderView();
    expect(await screen.findByText(/^Sprint 12, day \d+ of 14, 1 of 3 done, 3 of 13 pts$/)).toBeInTheDocument();
  });

  it("says in the summary line what the running sprint's progress does not count", async () => {
    vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([
      detail({ issues: [PROMO, KEYS, RETRO], notSynced: 2 }), BACKLOG,
    ]);
    renderView();
    // Neither half of the progress counts a key the cache does not hold, and
    // the line is always on screen, so it says so rather than reading as a
    // complete count of the sprint.
    expect(await screen.findByText(/plus 2 cards this cache does not hold$/)).toBeInTheDocument();
  });

  it("names the board instead of offering a picker when there is one scrum board", async () => {
    renderView();
    expect(await screen.findByRole("heading", { name: "Acme Platform Scrum" })).toBeInTheDocument();
    // A kanban board has no sprints at all, so it is not offered here rather
    // than offered and then explained.
    expect(screen.queryByRole("combobox", { name: "Board" })).not.toBeInTheDocument();
  });

  it("offers the picker once a second scrum board exists, and reads the one that is chosen", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListBoards).mockResolvedValue([
      { id: 1, name: "Acme Platform Scrum", type: "scrum" },
      { id: 3, name: "Platform Delivery", type: "scrum" },
    ]);
    renderView();
    await user.selectOptions(await screen.findByRole("combobox", { name: "Board" }), "3");
    await waitFor(() => expect(api.ListBoardSprintDetails).toHaveBeenCalledWith("p1", 3));
  });

  it("shows the closed sprints only when asked", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByLabelText("Show closed sprints"));
    expect(await screen.findByRole("treeitem", { name: "Sprint 11, Closed" })).toBeInTheDocument();
  });

  it("says which empty it is when the profile has no scrum board", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([{ id: 2, name: "Ops Kanban", type: "kanban" }]);
    renderView();
    expect(
      await screen.findByText("No scrum board has been synced for this project, so there are no sprints to show."),
    ).toBeInTheDocument();
  });

  it("says which empty it is when the board has no sprints at all", async () => {
    vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([BACKLOG]);
    renderView();
    expect(await screen.findByText("This board has no sprints yet. New sprint makes the first one.")).toBeInTheDocument();
  });

  it("says which empty it is when every sprint is closed and the toggle is off", async () => {
    vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([CLOSED, BACKLOG]);
    renderView();
    expect(
      await screen.findByText("Every sprint on this board is closed. Show closed sprints brings them back."),
    ).toBeInTheDocument();
  });

  it("reports a read that failed rather than drawing an empty board", async () => {
    vi.mocked(api.ListBoardSprintDetails).mockRejectedValue(new Error("database is locked"));
    renderView();
    expect(await screen.findByText(/Could not load this board's sprints: database is locked/)).toBeInTheDocument();
  });

  it("opens the detail panel on one card and gives the pane back to the checked run on a second", async () => {
    const user = userEvent.setup();
    const { container } = renderView();
    await user.click(await screen.findByRole("treeitem", { name: "PLAT-412 Apply promo code" }));
    expect(await screen.findByRole("complementary")).toBeInTheDocument();
    await user.click(screen.getByRole("checkbox", { name: "Check PLAT-409" }));
    await user.click(screen.getByRole("checkbox", { name: "Check PLAT-412" }));
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    // The pane keeps the panel's width while the panel is gone, so checking
    // a second row does not slide every row sideways mid gesture.
    expect(container.querySelector(".detail-panel-reserve")).not.toBeNull();
    // Back down to the one card the panel was about, and the panel is what
    // the pane holds again.
    await user.click(screen.getByRole("checkbox", { name: "Check PLAT-409" }));
    expect(await screen.findByRole("complementary")).toBeInTheDocument();
  });

  it("reserves the pane for a row checked from the keyboard, which selects nothing", async () => {
    const user = userEvent.setup();
    const { container } = renderView();
    const row = await screen.findByRole("treeitem", { name: "PLAT-412 Apply promo code" });
    row.focus();
    // Space checks and Enter selects here, where the Epics tree binds both
    // to select. A run started with Space opens no panel, so without the
    // reserve the pane would be full width for the first card and narrow
    // for the second, sliding every row sideways mid gesture.
    await user.keyboard(" ");
    expect(screen.getByText("1 card selected")).toBeInTheDocument();
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    expect(container.querySelector(".detail-panel-reserve")).not.toBeNull();
  });

  it("stops a shift gesture at the sprint it started in", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([
      ACTIVE,
      detail({ id: 13, name: "Sprint 13", state: "future", goal: "", startDate: "", endDate: "", issues: [DRAFT] }),
      BACKLOG,
    ]);
    renderView();
    // Only the active sprint opens by itself, so Sprint 13 is opened by
    // hand to put its card on screen under Sprint 12's three.
    await user.click(await screen.findByRole("treeitem", { name: "Sprint 13, Future" }));
    await user.click(await screen.findByRole("treeitem", { name: "PLAT-412 Apply promo code" }));
    await user.keyboard("{Shift>}");
    await user.click(screen.getByRole("treeitem", { name: "PLAT-500 Draft the migration" }));
    await user.keyboard("{/Shift}");
    // The anchor is in another sprint's list, so the run cannot be measured
    // and the gesture falls back to checking the one card it landed on. A
    // run measured over the whole tree instead would have taken all three
    // of Sprint 12's cards with it.
    expect(screen.getByText("1 card selected")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Check PLAT-500" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Check PLAT-412" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Check PLAT-409" })).not.toBeChecked();
  });

  it("fills a sprint from the checked cards, and sends no row that is not a card", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("checkbox", { name: "Check PLAT-412" }));
    await user.click(screen.getByRole("checkbox", { name: "Check PLAT-409" }));
    expect(screen.getByText("2 cards selected")).toBeInTheDocument();
    await user.selectOptions(screen.getByRole("combobox", { name: "Move the checked cards to" }), "13");
    await user.click(screen.getByRole("button", { name: "Move 2 cards" }));
    await waitFor(() => expect(api.JournalSprintMoves).toHaveBeenCalledWith("p1", ["PLAT-412", "PLAT-409"], "13"));
    expect(banner()).toHaveTextContent("2 cards moved to Sprint 13. Commit sends this to Jira.");
  });

  it("reports the cards a fill could not move rather than counting them as moved", async () => {
    const user = userEvent.setup();
    vi.mocked(api.JournalSprintMoves).mockResolvedValue(1);
    renderView();
    await user.click(await screen.findByRole("checkbox", { name: "Check PLAT-412" }));
    await user.click(screen.getByRole("checkbox", { name: "Check PLAT-409" }));
    await user.selectOptions(screen.getByRole("combobox", { name: "Move the checked cards to" }), "backlog");
    await user.click(screen.getByRole("button", { name: "Move 2 cards" }));
    await waitFor(() => expect(banner()).toHaveTextContent("1 of 2 cards moved to the backlog."));
  });

  it("does nothing on a second press while the fill is still in flight", async () => {
    const user = userEvent.setup();
    let release: (n: number) => void = () => {};
    vi.mocked(api.JournalSprintMoves).mockReturnValue(new Promise<number>((r) => { release = r; }));
    renderView();
    await user.click(await screen.findByRole("checkbox", { name: "Check PLAT-412" }));
    await user.selectOptions(screen.getByRole("combobox", { name: "Move the checked cards to" }), "13");
    const move = screen.getByRole("button", { name: "Move 1 card" });
    await user.click(move);
    await user.click(move);
    expect(api.JournalSprintMoves).toHaveBeenCalledTimes(1);
    release(1);
  });

  it("says by name what a delete destroys, where its issues go, and that it cannot be undone", async () => {
    const user = userEvent.setup();
    renderView();
    const menu = await openMenu(user, "Sprint 12");
    await user.click(within(menu).getByRole("menuitem", { name: "Delete sprint…" }));
    const ask = await screen.findByRole("alertdialog", { name: "Delete Sprint 12?" });
    expect(within(ask).getByText("Jira deletes Sprint 12.")).toBeInTheDocument();
    expect(within(ask).getByText("Jira moves its 3 issues back to the backlog. The issues themselves are not deleted.")).toBeInTheDocument();
    expect(within(ask).getByText("This cannot be undone, from TAM or from Jira.")).toBeInTheDocument();
    expect(within(ask).getByText("Sends to Jira now")).toBeInTheDocument();
    expect(within(ask).getByRole("button", { name: "Delete sprint" })).toBeInTheDocument();
    expect(within(ask).getByRole("button", { name: "Keep it" })).toBeInTheDocument();
    await user.click(within(ask).getByRole("button", { name: "Keep it" }));
    expect(api.DeleteSprint).not.toHaveBeenCalled();
  });

  it("counts a delete's issues as a floor when the sprint holds keys the cache does not", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([
      detail({ issues: [PROMO], total: 1, notSynced: 4 }), BACKLOG,
    ]);
    renderView();
    const menu = await openMenu(user, "Sprint 12");
    await user.click(within(menu).getByRole("menuitem", { name: "Delete sprint…" }));
    const ask = await screen.findByRole("alertdialog", { name: "Delete Sprint 12?" });
    // A wrong number in the one confirmation nobody can undo is worse than
    // an admitted floor.
    expect(within(ask).getByText(/Jira moves at least 1 issue back to the backlog/)).toBeInTheDocument();
    expect(within(ask).getByText(/Some of this sprint's issues are not in this cache/)).toBeInTheDocument();
  });

  it("counts a delete's issues as a floor when a card has been journaled out of the sprint", async () => {
    const user = userEvent.setup();
    // What the fill bar on this very screen leaves behind: the card is out
    // of Sprint 12 in the cache and still in it in Jira, so the total the
    // read counts is already short of what a delete would move.
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      {
        id: 7, entityType: api.ENTITY_SPRINT_MOVE, entityKey: "PLAT-409", field: api.FIELD_SPRINT_ID,
        beforeVal: "12|Sprint 12", afterVal: "13|Sprint 13", baseVersion: "", createdAt: "",
      },
    ]);
    renderView();
    const menu = await openMenu(user, "Sprint 12");
    await user.click(within(menu).getByRole("menuitem", { name: "Delete sprint…" }));
    const ask = await screen.findByRole("alertdialog", { name: "Delete Sprint 12?" });
    expect(within(ask).getByText(/Jira moves at least 3 issues back to the backlog/)).toBeInTheDocument();
    expect(within(ask).getByText(/Cards are waiting for Commit to move in or out of this sprint/)).toBeInTheDocument();
  });

  it("reports a refused delete, which arrives after the confirmation has closed", async () => {
    const user = userEvent.setup();
    vi.mocked(api.DeleteSprint).mockRejectedValue(new Error("403 Forbidden: Manage Sprints"));
    renderView();
    const menu = await openMenu(user, "Sprint 12");
    await user.click(within(menu).getByRole("menuitem", { name: "Delete sprint…" }));
    const ask = await screen.findByRole("alertdialog", { name: "Delete Sprint 12?" });
    await user.click(within(ask).getByRole("button", { name: "Delete sprint" }));
    const failed = await screen.findByRole("alertdialog", { name: "The sprint was not deleted" });
    expect(within(failed).getByText(/403 Forbidden: Manage Sprints/)).toBeInTheDocument();
  });

  it("deletes the sprint and says so once the confirmation is answered", async () => {
    const user = userEvent.setup();
    renderView();
    const menu = await openMenu(user, "Sprint 12");
    await user.click(within(menu).getByRole("menuitem", { name: "Delete sprint…" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete sprint" }));
    await waitFor(() => expect(api.DeleteSprint).toHaveBeenCalledWith("p1", 1, 12));
    await waitFor(() => expect(banner()).toHaveTextContent("Sprint 12 was deleted."));
  });

  it("edits a sprint from its own row, opening on what that sprint holds", async () => {
    const user = userEvent.setup();
    renderView();
    const menu = await openMenu(user, "Sprint 12");
    await user.click(within(menu).getByRole("menuitem", { name: "Edit sprint…" }));
    const dialog = await screen.findByRole("dialog", { name: "Edit Sprint 12" });
    expect(within(dialog).getByLabelText("Goal")).toHaveValue("Ship checkout");
    await user.clear(within(dialog).getByLabelText("Name"));
    await user.type(within(dialog).getByLabelText("Name"), "Sprint 12a");
    await user.click(within(dialog).getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(api.EditSprint).toHaveBeenCalledWith(
      "p1", 1, 12, "Sprint 12a", "Ship checkout", "2026-08-29", "2026-09-12", false,
    ));
    await waitFor(() => expect(banner()).toHaveTextContent("Sprint 12a was updated."));
  });

  it("opens the start dialog on the sprint's own goal rather than on an empty box", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([
      detail({ id: 13, name: "Sprint 13", state: "future", goal: "Land the importer", issues: [] }), BACKLOG,
    ]);
    renderView();
    const menu = await openMenu(user, "Sprint 13");
    await user.click(within(menu).getByRole("menuitem", { name: "Start sprint…" }));
    const dialog = await screen.findByRole("dialog", { name: "Start Sprint 13" });
    // Starting a sprint sends the goal box back to Jira whatever is in it,
    // so an empty box over a real goal was a field the user typed into
    // without seeing what they were replacing.
    expect(within(dialog).getByLabelText("Goal")).toHaveValue("Land the importer");
  });

  it("completes a sprint from here, naming the unfinished cards by the board's own last column", async () => {
    const user = userEvent.setup();
    renderView();
    const menu = await openMenu(user, "Sprint 12");
    await user.click(within(menu).getByRole("menuitem", { name: "Complete sprint…" }));
    const dialog = await screen.findByRole("dialog", { name: "Complete Sprint 12" });
    expect(within(dialog).getByText("2 cards are not finished and will move out of the sprint:")).toBeInTheDocument();
    expect(within(dialog).getByText("PLAT-412")).toBeInTheDocument();
    // PLAT-347 sits in a status the board's Done column collects, so it is
    // finished and does not move.
    expect(within(dialog).queryByText("PLAT-347")).not.toBeInTheDocument();
    expect(within(dialog).getByText(/A card counts as finished when it sits in Done/)).toBeInTheDocument();
  });

  it("creates a sprint, announces it, and points at the row it landed on", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "New sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "New sprint" });
    await waitFor(() => expect(within(dialog).getByLabelText("Start")).toHaveValue("2026-09-14"));
    await user.click(within(dialog).getByRole("button", { name: "Create sprint" }));
    await waitFor(() => expect(api.CreateSprint).toHaveBeenCalled());
    await waitFor(() => expect(banner()).toHaveTextContent("Sprint 14 was created, 2026-09-14 to 2026-09-28."));
    // And announced, which is the report a reader who is not watching the
    // list gets. The shared live region is the unnamed status region; the
    // banner above is the named one.
    const live = screen.getAllByRole("status").find((el) => el.classList.contains("sr-only"));
    await waitFor(() => expect(live).toHaveTextContent("Sprint 14 was created"));
  });

  it("marks the create dialog as sending to Jira now, and still names Commit under it", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "New sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "New sprint" });
    expect(within(dialog).getByText("Sends to Jira now")).toBeInTheDocument();
    // Commit is the concept the whole app is built on and the word the user
    // has been trained on, so the chip carries the sentence naming it.
    expect(within(dialog).getByText("This does not wait for Commit.")).toBeInTheDocument();
  });

  it("carries a partial success's note into the banner beside the sentence", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CreateSprint).mockResolvedValue({
      sprint: { id: 14, boardId: 1, name: "Sprint 14", state: "future", startDate: "", endDate: "", goal: "" },
      note: "The board's sprints could not be re-read. Press Refresh.",
    });
    renderView();
    await user.click(await screen.findByRole("button", { name: "New sprint" }));
    const dialog = await screen.findByRole("dialog", { name: "New sprint" });
    await waitFor(() => expect(within(dialog).getByLabelText("Start")).toHaveValue("2026-09-14"));
    await user.click(within(dialog).getByRole("button", { name: "Create sprint" }));
    await waitFor(() => expect(banner()).toHaveTextContent("Press Refresh."));
  });
});
