import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import type { SyncProgress } from "@agile-suite/core";
import * as api from "../api";
import type { ReportPerson, ReportSeries, Sprint, SprintReport } from "../api";
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
    // The kanban half of the view reads the board's column heads out of the
    // cache and publishes them, so it touches neither of the two above.
    GetBoard: vi.fn(),
    PublishSprintReport: vi.fn(),
    BoardAllocation: vi.fn(),
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
    timeDays: [],
    truncated: [],
    issues: [],
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

const rita: ReportPerson = {
  name: "Rita Anand", username: "ranand", email: "r.anand@example.com", cards: 3, points: 8,
  estimateSeconds: 12 * 3600, remainingSeconds: 2 * 3600, loggedSeconds: 9 * 3600, inProgressSeconds: 26 * 3600,
  spentSeconds: 10 * 3600,
  issues: [{
    key: "PLAT-412", summary: "Apply promo code at payment", status: "In Progress", statusCategory: "indeterminate",
    points: 5, estimateSeconds: 8 * 3600, remainingSeconds: 2 * 3600, spentSeconds: 7 * 3600,
    loggedSeconds: 6 * 3600, inProgressSeconds: 20 * 3600,
  }],
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetSprintReport).mockResolvedValue(report());
  sync.progress = null;
  sync.running = null;
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListBoards).mockResolvedValue([{ id: 1, name: "Acme Platform Scrum", type: "scrum" }]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue(SPRINTS);
  vi.mocked(api.CancelSprintReport).mockResolvedValue();
  vi.mocked(api.BoardAllocation).mockResolvedValue([]);
});

// The People section (#287) on screen, in its own tab: the sprint report
// tab keeps the one-window layout #113 built, and People is a different
// scope of question, who carried the sprint rather than how it went.
describe("ReportsView people tab", () => {
  it("keeps the sprint report as the first tab and shows People only when chosen", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ people: [rita] }) }));
    renderView();
    await screen.findByText(SENTENCE);
    const tabs = screen.getByRole("tablist", { name: "Report views" });
    expect(within(tabs).getByRole("tab", { name: "Sprint report" })).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("group", { name: "People" })).toBeNull();

    await user.click(within(tabs).getByRole("tab", { name: "People" }));
    expect(within(tabs).getByRole("tab", { name: "People" })).toHaveAttribute("aria-selected", "true");
    const panel = screen.getByRole("group", { name: "People" });
    const row = within(panel).getByRole("row", { name: /Rita Anand/ });
    expect(within(row).getByRole("cell", { name: "r.anand@example.com" })).toBeInTheDocument();
    expect(within(row).getByRole("cell", { name: "9h" })).toBeInTheDocument();
    expect(within(row).getByRole("cell", { name: "26h" })).toBeInTheDocument();
    // The notes sit in their own box, apart from the figures.
    expect(within(within(panel).getByRole("note")).getByText(/credited to its current assignee/)).toBeInTheDocument();
    // The sprint's charts belong to the other tab.
    expect(screen.queryByText(SENTENCE)).toBeNull();
  });

  it("draws each person's actual against their estimate as the backlog's bar", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ people: [rita] }) }));
    renderView();
    await screen.findByText(SENTENCE);
    await user.click(screen.getByRole("tab", { name: "People" }));
    const row = within(screen.getByRole("group", { name: "People" })).getByRole("row", { name: /Rita Anand/ });
    expect(within(row).getByRole("progressbar")).toHaveAttribute("aria-valuetext", "10h of 12h");
  });

  it("opens a person's cards under their row and closes them again", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ people: [rita] }) }));
    renderView();
    await screen.findByText(SENTENCE);
    await user.click(screen.getByRole("tab", { name: "People" }));
    const toggle = screen.getByRole("button", { name: "Show Rita Anand's cards" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("row", { name: /PLAT-412/ })).toBeNull();

    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const card = screen.getByRole("row", { name: /PLAT-412/ });
    expect(within(card).getByText("Apply promo code at payment")).toBeInTheDocument();
    expect(within(card).getByText("In Progress")).toBeInTheDocument();
    expect(within(card).getByRole("cell", { name: "20h" })).toBeInTheDocument();
    expect(within(card).getByRole("progressbar")).toHaveAttribute("aria-valuetext", "7h of 8h");

    await user.click(toggle);
    expect(screen.queryByRole("row", { name: /PLAT-412/ })).toBeNull();
  });

  it("moves between the tabs with the arrow keys", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText(SENTENCE);
    screen.getByRole("tab", { name: "Sprint report" }).focus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("tab", { name: "People" })).toHaveFocus();
    expect(screen.getByRole("tab", { name: "People" })).toHaveAttribute("aria-selected", "true");
  });

  it("says a report stored before people were counted needs a rebuild, rather than drawing nothing", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText(SENTENCE);
    await user.click(screen.getByRole("tab", { name: "People" }));
    expect(screen.queryByRole("group", { name: "People" })).toBeNull();
    expect(screen.getByText(/stored before TAM counted people/)).toBeInTheDocument();
  });
});

// The board allocation (#288) sits on the People tab under the sprint's own
// People section: the board's open work per assignee, from the cache.
describe("ReportsView board allocation", () => {
  it("shows each assignee's open work on the board with the bar and the cards behind it", async () => {
    const user = userEvent.setup();
    vi.mocked(api.BoardAllocation).mockResolvedValue([{ ...rita, loggedSeconds: 0, inProgressSeconds: 0 }]);
    renderView();
    await screen.findByText(SENTENCE);
    await user.click(screen.getByRole("tab", { name: "People" }));
    const panel = await screen.findByRole("group", { name: "Board allocation" });
    const headers = within(panel).getAllByRole("columnheader").map((h) => h.textContent);
    expect(headers).toEqual(["Person", "Username", "Email", "Cards", "Points", "Estimate", "Actual", "Remaining", "Actual against estimate"]);
    const row = within(panel).getByRole("row", { name: /Rita Anand/ });
    expect(within(row).getByRole("progressbar")).toHaveAttribute("aria-valuetext", "10h of 12h");
    await user.click(within(panel).getByRole("button", { name: "Show Rita Anand's cards" }));
    expect(within(panel).getByRole("row", { name: /PLAT-412/ })).toBeInTheDocument();
    expect(within(within(panel).getByRole("note")).getByText(/every open card on the board/)).toBeInTheDocument();
    expect(api.BoardAllocation).toHaveBeenCalledWith("p1", 1);
  });

  it("says a board with no open cards has none", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText(SENTENCE);
    await user.click(screen.getByRole("tab", { name: "People" }));
    expect(await screen.findByText("No card on this board is open.")).toBeInTheDocument();
  });
});

describe("ReportsView board allocation on a kanban board", () => {
  it("draws the allocation under the column capacity", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([{ id: 2, name: "Ops Kanban", type: "kanban" }]);
    vi.mocked(api.GetBoard).mockResolvedValue({
      boardId: 2, sprintId: "", swimlane: "none", lanes: [],
      columns: [{ name: "To Do", statusIds: ["1"], total: 2, points: 0, counted: 2, min: null, max: null, constraint: "issueCount" }],
      donePoints: 0, unmapped: 0, unmappedStatuses: [], notSynced: 0, capped: false, needsStatusSync: false,
    });
    vi.mocked(api.BoardAllocation).mockResolvedValue([{ ...rita, loggedSeconds: 0, inProgressSeconds: 0 }]);
    renderView();
    expect(await screen.findByRole("group", { name: /column capacity/i })).toBeInTheDocument();
    const panel = await screen.findByRole("group", { name: "Board allocation" });
    expect(within(panel).getByRole("row", { name: /Rita Anand/ })).toBeInTheDocument();
    expect(api.BoardAllocation).toHaveBeenCalledWith("p1", 2);
  });
});
