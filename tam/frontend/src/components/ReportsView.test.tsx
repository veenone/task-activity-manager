import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import type { SyncProgress } from "@agile-suite/core";
import * as api from "../api";
import type { ReportSeries, Sprint, SprintReport } from "../api";
import { profileBackend } from "../profileBackend";
import type { LockedOperation } from "../contexts/SyncContext";
import { ReportsView } from "./ReportsView";

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
    GetSprintReport: vi.fn(),
    CancelSprintReport: vi.fn(),
  };
});

// The view takes Go's per-profile lock through SyncContext.runReport, and
// reads two things back off the context: the progress frame the shell is
// drawing, and the name of the operation that frame belongs to. All three
// are stood in for here, with the lock as the passthrough it is when
// nothing else holds it; SyncContext.test.tsx is where the lock itself,
// the stage wording and the names are tested.
const sync = vi.hoisted(() => ({
  progress: null as SyncProgress | null,
  running: null as LockedOperation | null,
  runReport: <T,>(action: () => Promise<T>) => action(),
}));

vi.mock("../contexts/SyncContext", () => ({ useSync: () => sync }));

function series(over: Partial<ReportSeries> = {}): ReportSeries {
  return {
    sprintId: 11,
    sprintName: "Sprint 11",
    unit: "points",
    unitReason: "",
    committed: 34,
    added: 5,
    removed: 2,
    completed: 29,
    carriedOver: 10,
    days: [],
    truncated: [],
    ...over,
  };
}

function report(over: Partial<SprintReport> = {}): SprintReport {
  return {
    series: series(),
    velocity: [
      { sprintId: 10, sprintName: "Sprint 10", unit: "points", unitReason: "", committed: 30, completed: 28, truncated: false },
      { sprintId: 11, sprintName: "Sprint 11", unit: "points", unitReason: "", committed: 34, completed: 29, truncated: false },
    ],
    builtAt: "2026-09-10T08:00:00Z",
    unavailable: "",
    ...over,
  };
}

// An unavailable report is its reason and nothing else, the way the backend
// builds one: every slice empty rather than absent.
function unavailable(reason: string): SprintReport {
  return {
    series: series({
      sprintId: 0, sprintName: "", unit: "", committed: 0, added: 0, removed: 0, completed: 0, carriedOver: 0,
    }),
    velocity: [],
    builtAt: "",
    unavailable: reason,
  };
}

function sprint(over: Partial<Sprint>): Sprint {
  return {
    id: 11, boardId: 1, name: "Sprint 11", state: "closed",
    startDate: "2026-08-22T09:00:00.000+0000", endDate: "2026-09-05T09:00:00.000+0000", goal: "",
    ...over,
  };
}

const SPRINTS = [
  sprint({ id: 10, name: "Sprint 10", endDate: "2026-08-22T09:00:00.000+0000" }),
  sprint({ id: 11, name: "Sprint 11" }),
  sprint({ id: 12, name: "Sprint 12", state: "active", endDate: "2026-09-19T09:00:00.000+0000" }),
];

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderView(props: Partial<React.ComponentProps<typeof ReportsView>> = {}) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <Loader />
          <ReportsView {...props} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const SENTENCE = "Sprint 11 committed 34 points, added 5, removed 2, completed 29 and carried over 10.";

beforeEach(() => {
  vi.clearAllMocks();
  sync.progress = null;
  sync.running = null;
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListBoards).mockResolvedValue([
    { id: 1, name: "Acme Platform Scrum", type: "scrum" },
    { id: 2, name: "Ops Kanban", type: "kanban" },
  ]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue(SPRINTS);
  vi.mocked(api.GetSprintReport).mockResolvedValue(report());
  vi.mocked(api.CancelSprintReport).mockResolvedValue();
});

describe("ReportsView", () => {
  it("opens on the board's most recent closed sprint without naming one itself", async () => {
    renderView();
    expect(await screen.findByText(SENTENCE)).toBeInTheDocument();
    // Zero is what asks the backend for the newest closed sprint, and the
    // whole reason the noClosedSprint state is reachable at all.
    expect(api.GetSprintReport).toHaveBeenCalledWith("p1", 1, 0, false);
  });

  it("titles the report with the sprint the backend answered about, not the one the picker holds", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ sprintId: 10, sprintName: "Sprint 10" }) }));
    renderView();
    expect(await screen.findByRole("heading", { name: "Sprint 10" })).toBeInTheDocument();
  });

  it("says under the sentence that committed is a floor and removed sees only the cards that came back", async () => {
    renderView();
    await screen.findByText(SENTENCE);
    expect(screen.getByText(/Committed is a minimum estimate\. Removed counts cards that left and later returned/)).toBeInTheDocument();
    expect(screen.getByText(/left and later returned/)).toBeInTheDocument();
  });

  it("shows five labeled figures for quick sprint-review scanning", async () => {
    renderView();
    await screen.findByText(SENTENCE);
    const metrics = screen.getByLabelText("Sprint report figures");
    expect(metrics).toHaveTextContent("Committed34 points");
    expect(metrics).toHaveTextContent("Added5 points");
    expect(metrics).toHaveTextContent("Removed2 points");
    expect(metrics).toHaveTextContent("Completed29 points");
    expect(metrics).toHaveTextContent("Carried over10 points");
  });

  it("offers cancel while a report is loading", async () => {
    const user = userEvent.setup();
    let release: () => void = () => {};
    vi.mocked(api.GetSprintReport).mockImplementation(() => new Promise((resolve) => { release = () => resolve(report()); }));
    renderView();
    expect(await screen.findByRole("button", { name: "Cancel report" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Cancel report" }));
    await waitFor(() => expect(api.CancelSprintReport).toHaveBeenCalledWith("p1"));
    release();
  });

  it("prints the method line with the numbers", async () => {
    renderView();
    await screen.findByText(SENTENCE);
    expect(screen.getByText(/Done means the board's last column/)).toBeInTheDocument();
    expect(screen.getByText(/TAM uses Jira history, so its totals can differ from Jira's own/)).toBeInTheDocument();
  });

  it("counts points and explains nothing when that is what the sprint was estimated in", async () => {
    renderView();
    await screen.findByText(SENTENCE);
    expect(screen.queryByText(/count cards rather than points/)).not.toBeInTheDocument();
  });

  it("counts cards and says the team can fix it by estimating", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(
      report({ series: series({ unit: "cards", unitReason: "nothingEstimated", committed: 12 }) }),
    );
    renderView();
    expect(await screen.findByText(/Sprint 11 committed 12 cards/)).toBeInTheDocument();
    expect(screen.getByText(/Estimate the cards to report points/)).toBeInTheDocument();
  });

  it("counts cards and does not claim the instance has no story points field", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(
      report({ series: series({ unit: "cards", unitReason: "noPointsFieldSeen", committed: 12 }) }),
    );
    renderView();
    expect(await screen.findByText(/Sprint 11 committed 12 cards/)).toBeInTheDocument();
    expect(screen.getByText(/field is missing or unused/)).toBeInTheDocument();
  });

  it("names the cards whose changelog came back cut short and refuses to call the figures exact", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(
      report({ series: series({ truncated: ["PLAT-9", "PLAT-12"] }) }),
    );
    renderView();
    await screen.findByText(SENTENCE);
    expect(screen.getByText(/PLAT-9 and PLAT-12/)).toBeInTheDocument();
    expect(screen.getByText(/are not exact/)).toBeInTheDocument();
  });

  it("draws the velocity rows in the order the backend sent them, each in its own unit", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(
      report({
        velocity: [
          { sprintId: 10, sprintName: "Sprint 10", unit: "points", unitReason: "", committed: 30, completed: 28, truncated: false },
          { sprintId: 11, sprintName: "Sprint 11", unit: "cards", unitReason: "nothingEstimated", committed: 12, completed: 9, truncated: true },
        ],
      }),
    );
    renderView();
    await screen.findByText(SENTENCE);
    // Scoped to the velocity table: the outcome chart ships a data table of
    // its own, and it comes first in the page's rows.
    const rows = within(screen.getByRole("table", { name: /velocity/i })).getAllByRole("row");
    // The header row first, then the two sprints oldest first.
    expect(rows[1]).toHaveTextContent("Sprint 10");
    expect(rows[1]).toHaveTextContent("30 points");
    expect(rows[2]).toHaveTextContent("Sprint 11");
    expect(rows[2]).toHaveTextContent("12 cards");
    expect(rows[2]).toHaveTextContent("Built on a partial changelog");
  });

  it("hides the board picker when the profile has one scrum board and names the board instead", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([{ id: 1, name: "Acme Platform Scrum", type: "scrum" }]);
    renderView();
    expect(await screen.findByRole("heading", { name: "Acme Platform Scrum" })).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Board" })).not.toBeInTheDocument();
  });

  it("offers a board picker when the profile has two scrum boards, and no kanban board", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([
      { id: 1, name: "Acme Platform Scrum", type: "scrum" },
      { id: 2, name: "Ops Kanban", type: "kanban" },
      { id: 3, name: "Payments Scrum", type: "scrum" },
    ]);
    renderView();
    const picker = await screen.findByRole("combobox", { name: "Board" });
    expect(picker).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Ops Kanban" })).not.toBeInTheDocument();
  });

  it("announces board failures and offers both retry and Boards recovery", async () => {
    const openBoards = vi.fn();
    vi.mocked(api.ListBoards).mockRejectedValue(new Error("network down"));
    renderView({ onOpenBoards: openBoards });
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load the boards");
    expect(alert).not.toHaveTextContent("network down");
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open Boards" })).toBeInTheDocument();
  });

  it("keeps a stable Reports heading when the board picker is shown", async () => {
    renderView();
    expect(await screen.findByRole("heading", { name: "Reports", level: 2 })).toBeInTheDocument();
  });

  it("orders closed sprints by actual completion, with planned end as fallback", async () => {
    vi.mocked(api.ListBoardSprints).mockResolvedValue([
      sprint({ id: 10, name: "Closed late", endDate: "2026-08-22T09:00:00Z", completeDate: "2026-09-08T09:00:00Z" }),
      sprint({ id: 11, name: "Older cache", endDate: "2026-09-05T09:00:00Z" }),
      sprint({ id: 12, name: "No planned end", endDate: "", completeDate: "2026-09-06T09:00:00Z" }),
    ]);
    renderView();
    const picker = await screen.findByRole("combobox", { name: "Sprint" });
    expect(Array.from(picker.querySelectorAll("option")).map((o) => o.textContent))
      .toEqual(["Closed late", "No planned end", "Older cache"]);
  });

  it("offers closed sprints newest first and labeled active sprints, excluding future sprints", async () => {
    vi.mocked(api.ListBoardSprints).mockResolvedValue([...SPRINTS, sprint({ id: 13, name: "Future", state: "future" })]);
    renderView();
    const picker = await screen.findByRole("combobox", { name: "Sprint" });
    const names = Array.from(picker.querySelectorAll("option")).map((o) => o.textContent);
    expect(names).toEqual(["Sprint 11", "Sprint 10", "Sprint 12 (in progress)"]);
  });

  it("opens an active sprint with provisional wording and refreshes it on demand", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetSprintReport).mockImplementation(async (_profile, _board, id) =>
      id === 12 ? report({ series: series({ sprintId: 12, sprintName: "Sprint 12" }) }) : report());
    renderView();
    await screen.findByText(SENTENCE);
    await user.selectOptions(screen.getByRole("combobox", { name: "Sprint" }), "12");
    expect(await screen.findByText(/In progress: these figures are provisional/)).toBeInTheDocument();
    expect(screen.getByText(/completed 29 so far and has 10 remaining/)).toBeInTheDocument();
    expect(api.GetSprintReport).toHaveBeenCalledWith("p1", 1, 12, false);
    await user.click(screen.getByRole("button", { name: "Refresh report" }));
    await waitFor(() => expect(api.GetSprintReport).toHaveBeenCalledWith("p1", 1, 12, true));
    expect(screen.getByRole("table", { name: /Velocity/ })).toBeInTheDocument();
  });

  it("defaults to an active sprint when the board has no closed sprints", async () => {
    vi.mocked(api.ListBoardSprints).mockResolvedValue([SPRINTS[2]]);
    vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ sprintId: 12, sprintName: "Sprint 12" }), velocity: [] }));
    renderView();
    expect(await screen.findByText(/In progress: these figures are provisional/)).toBeInTheDocument();
    expect(api.GetSprintReport).toHaveBeenCalledWith("p1", 1, 12, false);
    expect(api.GetSprintReport).not.toHaveBeenCalledWith("p1", 1, 0, false);
    expect(screen.getByRole("combobox", { name: "Sprint" })).toHaveValue("12");
    expect(api.CancelSprintReport).not.toHaveBeenCalled();
  });

  // Go builds its velocity table from VelocitySprints, which parses both
  // dates and drops a sprint that fails either. A picker ordered on the end
  // date alone put a sprint Go had dropped at the top of the list, where a
  // sprint id of 0 resolves past it.
  it("leaves out a closed sprint whose start date cannot be read, the way the backend's own table does", async () => {
    vi.mocked(api.ListBoardSprints).mockResolvedValue([
      sprint({ id: 13, name: "Sprint 13", startDate: "", endDate: "2026-09-12T09:00:00.000+0000" }),
      ...SPRINTS,
    ]);
    renderView();
    const picker = await screen.findByRole("combobox", { name: "Sprint" });
    const names = Array.from(picker.querySelectorAll("option")).map((o) => o.textContent);
    expect(names).toEqual(["Sprint 11", "Sprint 10", "Sprint 12 (in progress)"]);
  });

  // Phase 5 publishes this table on its own, and the summary's own
  // qualification is a paragraph away in a pane that scrolls.
  it("qualifies the velocity table's Committed column beside the table itself", async () => {
    renderView();
    await screen.findByText(SENTENCE);
    expect(screen.getByText(/Committed is a minimum estimate in every row/)).toBeInTheDocument();
  });

  it("reads the sprint the user picks", async () => {
    const user = userEvent.setup();
    renderView();
    await user.selectOptions(await screen.findByRole("combobox", { name: "Sprint" }), "10");
    await waitFor(() => expect(api.GetSprintReport).toHaveBeenCalledWith("p1", 1, 10, false));
  });

  it("rebuilds from Jira when asked, ignoring the stored series", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText(SENTENCE);
    await user.click(screen.getByRole("button", { name: "Rebuild from Jira" }));
    await waitFor(() => expect(api.GetSprintReport).toHaveBeenCalledWith("p1", 1, 0, true));
  });

  it("cancels the report it left running when it unmounts", async () => {
    const view = renderView();
    await screen.findByText(SENTENCE);
    view.unmount();
    await waitFor(() => expect(api.CancelSprintReport).toHaveBeenCalledWith("p1"));
  });

  it("says which board has no scrum board to report on", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([{ id: 2, name: "Ops Kanban", type: "kanban" }]);
    renderView();
    expect(await screen.findByText(/No scrum board has been synced for this project/)).toBeInTheDocument();
    expect(api.GetSprintReport).not.toHaveBeenCalled();
  });
});

describe("ReportsView's eight states", () => {
  it("state 1: a board whose columns were never synced points at the Boards view", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(unavailable("boardNotSynced"));
    renderView();
    expect(await screen.findByText(/columns are not in the cache/)).toBeInTheDocument();
  });

  it("state 2: a sprint the cache does not hold reads as a stale sprint list", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(unavailable("sprintNotFound"));
    renderView();
    expect(await screen.findByText(/cached sprint list does not hold that sprint/)).toBeInTheDocument();
  });

  it("state 3: a sprint with no readable dates cannot be reported on at all", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(unavailable("sprintHasNoDates"));
    renderView();
    expect(await screen.findByText(/no start or end date TAM can read/)).toBeInTheDocument();
  });

  it("state 4: a board that has never closed a sprint has no table either", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(unavailable("noClosedSprint"));
    renderView();
    expect(await screen.findByText(/no closed sprint a report can be built from/)).toBeInTheDocument();
  });

  it("state 5: a failed read keeps the previous report on screen, correct, and offers a retry", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText(SENTENCE);

    vi.mocked(api.GetSprintReport).mockRejectedValue(new Error("read tcp: connection reset by peer"));
    await user.click(screen.getByRole("button", { name: "Rebuild from Jira" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(/could not be read: read tcp/);
    // Not merely still present: still the report it was, figure for figure,
    // with its velocity rows and its qualification intact.
    expect(screen.getByText(SENTENCE)).toBeInTheDocument();
    expect(screen.getAllByText(/Committed is a minimum estimate/).length).toBeGreaterThan(0);
    expect(screen.getByRole("row", { name: /Sprint 10/ })).toHaveTextContent("30 points");
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("state 5, again: a first read that fails shows the failure and no half a report", async () => {
    vi.mocked(api.GetSprintReport).mockRejectedValue(new Error("read tcp: connection reset by peer"));
    renderView();
    expect(await screen.findByRole("alert")).toHaveTextContent(/could not be read/);
    expect(screen.queryByText(/committed/)).not.toBeInTheDocument();
    expect(screen.queryByText(/last report that was read in full/)).not.toBeInTheDocument();
  });

  it("state 6: a report holding a truncated history says so where its figures are", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ truncated: ["PLAT-9"] }) }));
    renderView();
    await screen.findByText(SENTENCE);
    expect(screen.getByText(/Jira returned only part of the changelog for 1 card/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Rebuild from Jira" })).toBeInTheDocument();
  });

  it("state 7: a read refused because the profile is busy says so rather than reading as a failure", async () => {
    vi.mocked(api.GetSprintReport).mockRejectedValue(new Error("a sync is already running for this profile"));
    renderView();
    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent("A sync is already running for this profile");
    expect(banner).toHaveTextContent(/refuses rather than waits/);
    expect(banner).not.toHaveTextContent(/could not be read/);
  });

  it("state 8: a report being built says which sprint is being read and how far along it is", async () => {
    sync.running = "report";
    sync.progress = { phase: "velocity", fetched: 25, total: 200, done: false, stage: "Reading Sprint 10 for the velocity table" };
    // A promise that never settles is what a minutes-long read looks like
    // from here, and a silent wait is the thing this state exists to avoid.
    vi.mocked(api.GetSprintReport).mockReturnValue(new Promise<SprintReport>(() => {}));
    renderView();
    expect(await screen.findByText("Reading Sprint 10 for the velocity table, 25 of 200 issues")).toBeInTheDocument();
  });

  // Most reports are served out of the store in a second, and this copy is
  // printed on every one of them.
  it("state 8 does not promise minutes for a report the store can answer at once", async () => {
    sync.running = "report";
    vi.mocked(api.GetSprintReport).mockReturnValue(new Promise<SprintReport>(() => {}));
    renderView();
    expect(await screen.findByText(/comes back at once/)).toBeInTheDocument();
  });

  // The frames on the shell belong to whoever holds the lock. Opening this
  // view during a sync puts the read in its retry window with the sync
  // still reporting, and drawing that frame here presented the sync's
  // issue count as the report's own progress.
  it("state 8, refused: a report waiting on another operation shows none of that operation's progress", async () => {
    sync.running = "sync";
    sync.progress = { phase: "issues", fetched: 120, total: 4000, done: false, stage: "Fetching issues" };
    vi.mocked(api.GetSprintReport).mockReturnValue(new Promise<SprintReport>(() => {}));
    renderView();
    expect(await screen.findByText("Building the sprint report")).toBeInTheDocument();
    expect(screen.queryByText(/Fetching issues/)).not.toBeInTheDocument();
    expect(screen.queryByText(/120 of 4000 issues/)).not.toBeInTheDocument();
  });
  // The charts draw ReportSeries.days and the velocity rows the view already
  // receives. Before this they reached the frontend and nothing read them.
  describe("charts", () => {
    const DAYS = [
      { date: "2026-08-22", scope: 34, completed: 0, remaining: 34, ideal: 34 },
      { date: "2026-08-23", scope: 34, completed: 12, remaining: 22, ideal: 22 },
      { date: "2026-08-24", scope: 39, completed: 29, remaining: 10, ideal: 0 },
    ];

    it("draws the burndown, the outcome and the velocity", async () => {
      vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ days: DAYS }) }));
      renderView();
      expect(await screen.findByRole("figure", { name: "Burndown" })).toBeInTheDocument();
      expect(screen.getByRole("figure", { name: "Sprint outcome" })).toBeInTheDocument();
      expect(screen.getByRole("figure", { name: "Velocity" })).toBeInTheDocument();
    });

    // Colour cannot be the only carrier, so each chart ships the same numbers
    // as a table a screen reader reads.
    it("gives the burndown a data table carrying every day's figures", async () => {
      vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ days: DAYS }) }));
      renderView();
      const table = await screen.findByRole("table", { name: /burndown/i });
      const cells = within(table).getAllByRole("cell").map((c) => c.textContent);
      expect(cells).toContain("22");
      expect(cells).toContain("39");
    });
  });

  describe("the summary above the charts", () => {
    // The caveat governs two of the five figures printed above it. It used to
    // sit inside the collapsed details, so the numbers were read without it.
    it("shows the floor caveat without the reader opening anything", async () => {
      vi.mocked(api.GetSprintReport).mockResolvedValue(report());
      renderView();
      const caveat = await screen.findByText(/Committed is a minimum estimate\. Removed counts/);
      expect(caveat.closest("details")).toBeNull();
    });

    // The tiles say the same thing and are scannable, so the sentence is the
    // copy that moves rather than the one that stays.
    it("keeps the restated sentence, inside the details", async () => {
      vi.mocked(api.GetSprintReport).mockResolvedValue(report());
      renderView();
      const sentence = await screen.findByText(SENTENCE);
      expect(sentence.closest("details")).not.toBeNull();
    });

    // Five equal columns said the five figures were peers. They are a
    // baseline, two changes to it, and what came of it.
    it("groups the five figures as a baseline, its changes and the outcome", async () => {
      vi.mocked(api.GetSprintReport).mockResolvedValue(report());
      renderView();
      const baseline = await screen.findByRole("group", { name: "Baseline" });
      expect(within(baseline).getByRole("definition")).toHaveTextContent("34");
      expect(within(screen.getByRole("group", { name: "Changes" })).getAllByRole("definition")).toHaveLength(2);
      expect(within(screen.getByRole("group", { name: "Outcome" })).getAllByRole("definition")).toHaveLength(2);
    });
  });
});

// Issue #85, from the layout critique.
describe("ReportsView, fixed height", () => {
  it("says a report counts cards where the figures are, not inside the details", async () => {
    // unitLine says the five figures count cards rather than points, which
    // changes what every one of them means. floorLine was pulled out of the
    // details for exactly this reason and unitLine was left behind, so the
    // numbers were read without it.
    vi.mocked(api.GetSprintReport).mockResolvedValue(
      report({ series: series({ unit: "cards", unitReason: "nothingEstimated" }) }),
    );
    renderView();
    const line = await screen.findByText(/counts cards because points appear/);
    expect(line.closest("details")).toBeNull();
  });

  it("leaves the details for the workings, not for a qualification", async () => {
    renderView();
    const method = await screen.findByText(/Done means the board's last column/);
    expect(method.closest("details")).not.toBeNull();
  });

  it("gives the one scrolling panel a name and a place in the tab order", async () => {
    // It becomes the only scroller on the page, and a bare div with
    // overflow: auto is not reachable from the keyboard in WebView2.
    renderView();
    await screen.findByText(SENTENCE);
    const panel = await screen.findByRole("group", { name: /velocity/i });
    expect(panel).toHaveAttribute("tabindex", "0");
  });

  it("keeps the velocity chart out of the scrolling panel", async () => {
    // Its tooltip is absolutely positioned and an overflow: auto ancestor
    // clips it, and scrolling to a row would scroll its chart away.
    renderView();
    await screen.findByText(SENTENCE);
    const panel = await screen.findByRole("group", { name: /velocity/i });
    expect(within(panel).queryByRole("img", { name: /velocity/i })).toBeNull();
  });
});
