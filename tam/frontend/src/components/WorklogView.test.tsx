import React from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { Issue, LoggedWork, WorklogRow } from "../api";
import { profileBackend } from "../profileBackend";
import { dayLabel } from "../lib/worklogWeek";
import { WorklogView } from "./WorklogView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    GetProfileSetting: vi.fn(),
    SetProfileSetting: vi.fn(),
    WorklogWeek: vi.fn(),
    LogWork: vi.fn(),
    CheckWorkDuration: vi.fn(),
    DiscardPendingChange: vi.fn(),
  };
});

function row(key: string, summary: string, over: Partial<WorklogRow> = {}): WorklogRow {
  const issue: Issue = {
    key, id: key, project: "PLAT", type: "task", summary, status: "In Progress", statusCategory: "indeterminate",
    assignee: "Demo User", assigneeName: "demo", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "",
    parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
  };
  return { issue, assigned: true, cached: true, ...over };
}

function logged(key: string, started: string, timeSpent: string, secs: number, pendingId = 0): LoggedWork {
  return {
    key, summary: "",
    log: { id: pendingId ? "" : `${key}-${started}`, author: "demo", authorName: "Demo User", started: `${started}:00.000+0700`, timeSpent, seconds: secs, comment: "", pending: !!pendingId, pendingId },
  };
}

// Today is Wednesday 18 November. Monday reached eight hours, Tuesday came in
// at five and a half, and today has two hours the journal still holds.
const rows = [
  row("PLAT-412", "Retry failed card payments"),
  row("PLAT-398", "Refund webhook drops duplicate events"),
  row("PLAT-431", "Cart totals wrong with mixed currencies"),
  row("OPS-88", "On-call handover notes", { assigned: false, cached: false }),
];
const all = [
  logged("PLAT-412", "2026-11-16T09:00", "4h", 14400),
  logged("PLAT-398", "2026-11-16T13:00", "4h", 14400),
  logged("PLAT-412", "2026-11-17T09:00", "3h", 10800),
  logged("PLAT-398", "2026-11-17T12:00", "2h", 7200),
  logged("OPS-88", "2026-11-17T14:00", "30m", 1800),
  logged("PLAT-412", "2026-11-18T09:00", "2h", 7200, 5),
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
          <WorklogView />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const cellName = (key: string, day: string, what: string) => `${key}, ${dayLabel(day)}, ${what}`;

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 18, 10, 0));
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.SetProfileSetting).mockResolvedValue();
  vi.mocked(api.LogWork).mockResolvedValue();
  vi.mocked(api.CheckWorkDuration).mockResolvedValue();
  vi.mocked(api.WorklogWeek).mockImplementation(async (_p, from, to) => ({
    rows,
    entries: all.filter((e) => e.log.started.slice(0, 10) >= from && e.log.started.slice(0, 10) <= to),
  }));
});

afterEach(() => {
  vi.useRealTimers();
});

describe("WorklogView", () => {
  it("reads this week and the fortnight behind today, and totals each day against the target", async () => {
    renderView();
    const monday = await screen.findByRole("columnheader", { name: new RegExp(dayLabel("2026-11-16")) });
    expect(api.WorklogWeek).toHaveBeenCalledWith("p1", "2026-11-16", "2026-11-22");
    expect(api.WorklogWeek).toHaveBeenCalledWith("p1", "2026-11-04", "2026-11-17");
    expect(within(monday).getByText("8h, complete")).toBeInTheDocument();
    const tuesday = screen.getByRole("columnheader", { name: new RegExp(dayLabel("2026-11-17")) });
    expect(within(tuesday).getByText("2h 30m short")).toBeInTheDocument();
    // Today is not over, so it reads as a running total rather than as short.
    const wednesday = screen.getByRole("columnheader", { name: new RegExp(dayLabel("2026-11-18")) });
    expect(within(wednesday).getByText("2h of 8h")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: cellName("PLAT-412", "2026-11-16", "4h logged") })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: cellName("PLAT-431", "2026-11-17", "nothing logged") })).toBeInTheDocument();
    expect(screen.getByText("15h 30m")).toBeInTheDocument();
  });

  it("draws an issue TAM does not hold without anything to press", async () => {
    renderView();
    const ops = (await screen.findByText("OPS-88")).closest("tr") as HTMLElement;
    // Once in Tuesday's cell and once as the row's week total.
    expect(within(ops).getAllByText("30m")).toHaveLength(2);
    expect(within(ops).queryByRole("button")).toBeNull();
  });

  it("logs on the issue and day of the cell, starting where the day's last entry ended", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: cellName("PLAT-431", "2026-11-17", "nothing logged") }));
    const panel = screen.getByRole("complementary", { name: `Log work on ${dayLabel("2026-11-17")}` });
    expect(within(panel).getByLabelText("Started")).toHaveValue("2026-11-17T14:30");
    expect(within(panel).getByLabelText("Issue")).toHaveValue("PLAT-431");

    await user.click(within(panel).getByRole("button", { name: "Fill the day, 2h 30m" }));
    expect(within(panel).getByLabelText("Time spent")).toHaveValue("2h 30m");
    await user.type(within(panel).getByLabelText("Comment"), "Repro");
    await user.click(within(panel).getByRole("button", { name: "Log work" }));
    await waitFor(() =>
      expect(api.LogWork).toHaveBeenCalledWith("p1", "PLAT-431", "2h 30m", "Repro", "2026-11-17T14:30"),
    );
  });

  it("lists the short workdays above the grid and opens the one that is picked", async () => {
    const user = userEvent.setup();
    renderView();
    const strip = await screen.findByRole("group", { name: "Days to fill" });
    expect(within(strip).getByText("9 workdays to fill")).toBeInTheDocument();
    expect(within(strip).getByRole("button", { name: `${dayLabel("2026-11-17")}2h 30m short` })).toBeInTheDocument();

    await user.click(within(strip).getByRole("button", { name: `${dayLabel("2026-11-13")}nothing logged` }));
    await waitFor(() => expect(api.WorklogWeek).toHaveBeenCalledWith("p1", "2026-11-09", "2026-11-15"));
    expect(await screen.findByRole("complementary", { name: `Log work on ${dayLabel("2026-11-13")}` })).toBeInTheDocument();
  });

  it("moves between cells with the arrow keys", async () => {
    const user = userEvent.setup();
    renderView();
    const start = await screen.findByRole("button", { name: cellName("PLAT-412", "2026-11-16", "4h logged") });
    start.focus();
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("button", { name: cellName("PLAT-398", "2026-11-16", "4h logged") })).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("button", { name: cellName("PLAT-398", "2026-11-17", "2h logged") })).toHaveFocus();
  });

  it("says why when the week cannot be read", async () => {
    vi.mocked(api.WorklogWeek).mockRejectedValue(new Error("TAM does not know your Jira user yet. Sync once or test the connection"));
    renderView();
    expect(await screen.findByRole("alert")).toHaveTextContent("TAM does not know your Jira user yet");
  });
});
