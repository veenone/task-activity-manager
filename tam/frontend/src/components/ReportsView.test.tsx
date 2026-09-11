import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import type { SyncProgress } from "@agile-suite/core";
import * as api from "../api";
import type { ReportSeries, Sprint, SprintReport } from "../api";
import { profileBackend } from "../profileBackend";
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

// The view takes Go's per-profile lock through SyncContext.runReport and
// reads the shell's progress back off it. Both are stood in for here, with
// the lock as the passthrough it is when nothing else holds it;
// SyncContext.test.tsx is where the lock and the stage wording are tested.
const sync = vi.hoisted(() => ({
  progress: null as SyncProgress | null,
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

function renderView() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <Loader />
          <ReportsView />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const SENTENCE = "Sprint 11 committed 34 points, added 5, removed 2, completed 29 and carried over 10.";

beforeEach(() => {
  vi.clearAllMocks();
  sync.progress = null;
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
    expect(screen.getByText(/Committed is a floor rather than a total/)).toBeInTheDocument();
    expect(screen.getByText(/left and came back/)).toBeInTheDocument();
  });

  it("prints the method line with the numbers", async () => {
    renderView();
    await screen.findByText(SENTENCE);
    expect(screen.getByText(/done means a status the board's last column collects/)).toBeInTheDocument();
    expect(screen.getByText(/public changelog rather than from Jira's own stored sprint records/)).toBeInTheDocument();
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
    expect(screen.getByText(/Estimating the cards would give this board a points report/)).toBeInTheDocument();
  });

  it("counts cards and does not claim the instance has no story points field", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(
      report({ series: series({ unit: "cards", unitReason: "noPointsFieldSeen", committed: 12 }) }),
    );
    renderView();
    expect(await screen.findByText(/Sprint 11 committed 12 cards/)).toBeInTheDocument();
    expect(screen.getByText(/cannot tell those two apart/)).toBeInTheDocument();
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
    const rows = screen.getAllByRole("row");
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

  it("offers only the board's closed sprints, newest first", async () => {
    renderView();
    const picker = await screen.findByRole("combobox", { name: "Sprint" });
    const names = Array.from(picker.querySelectorAll("option")).map((o) => o.textContent);
    expect(names).toEqual(["Sprint 11", "Sprint 10"]);
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
    expect(await screen.findByText(/never closed a sprint/)).toBeInTheDocument();
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
    expect(screen.getByText(/Committed is a floor rather than a total/)).toBeInTheDocument();
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
    expect(banner).toHaveTextContent("a sync is already running for this profile");
    expect(banner).toHaveTextContent(/refuses rather than waits/);
    expect(banner).not.toHaveTextContent(/could not be read/);
  });

  it("state 8: a report being built says which sprint is being read and how far along it is", async () => {
    sync.progress = { phase: "velocity", fetched: 25, total: 200, done: false, stage: "Reading Sprint 10 for the velocity table" };
    // A promise that never settles is what a minutes-long read looks like
    // from here, and a silent wait is the thing this state exists to avoid.
    vi.mocked(api.GetSprintReport).mockReturnValue(new Promise<SprintReport>(() => {}));
    renderView();
    expect(await screen.findByText("Reading Sprint 10 for the velocity table, 25 of 200 issues")).toBeInTheDocument();
  });
});
