import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
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
};

beforeEach(() => {
  vi.clearAllMocks();
  sync.progress = null;
  sync.running = null;
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListBoards).mockResolvedValue([{ id: 1, name: "Acme Platform Scrum", type: "scrum" }]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue(SPRINTS);
  vi.mocked(api.CancelSprintReport).mockResolvedValue();
});

// The People section (#287) on screen: the same section the outputs carry,
// drawn from lib/reportPeople rather than composed again here.
describe("ReportsView people", () => {
  it("shows a row per person with the time they logged and spent in progress", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(report({ series: series({ people: [rita] }) }));
    renderView();
    await screen.findByText(SENTENCE);
    const panel = await screen.findByRole("group", { name: "People" });
    const row = within(panel).getByRole("row", { name: /Rita Anand/ });
    expect(within(row).getByRole("cell", { name: "r.anand@example.com" })).toBeInTheDocument();
    expect(within(row).getByRole("cell", { name: "9h" })).toBeInTheDocument();
    expect(within(row).getByRole("cell", { name: "26h" })).toBeInTheDocument();
    expect(within(panel).getByText(/charged to whoever holds the card now/)).toBeInTheDocument();
  });

  it("draws no People section for a report stored before people were counted", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(report());
    renderView();
    await screen.findByText(SENTENCE);
    expect(screen.queryByRole("group", { name: "People" })).toBeNull();
  });
});
