import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { SprintReport } from "../api";
import { useSyncBoards } from "./boards";
import { invalidateProfileData } from "./invalidate";
import { useSprintReport } from "./reports";
import { invalidateSprintWrites } from "./sprints";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetSprintReport: vi.fn(), SyncBoards: vi.fn() };
});

// run stands in for SyncContext's runReport once the lock is free: it runs
// the action and hands the answer back. The lock itself is tested in
// contexts/SyncContext.test.tsx.
const run = async <T,>(action: () => Promise<T>) => action();

function wrapper(qc = createQueryClient()) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

// Longer than RETRY_AFTER_CANCEL_MS, so a retry that was going to happen has
// happened by the time the assertion after it runs.
const PAST_THE_RETRY = 400;
const settle = () => new Promise((r) => setTimeout(r, PAST_THE_RETRY));

function report(): SprintReport {
  return {
    series: {
      sprintId: 11, sprintName: "Sprint 11", unit: "points", unitReason: "",
      committed: 34, added: 5, removed: 2, completed: 29, carriedOver: 10, days: [], truncated: [],
    },
    velocity: [],
    builtAt: "2026-09-10T08:00:00Z",
    unavailable: "",
  };
}

const BUSY = "a sync is already running for this profile";

beforeEach(() => {
  vi.clearAllMocks();
});

// This is the one query in TAM that retries at all: the shared client
// defaults every other read to retry 0. What it buys is a board or sprint
// switch, which cancels the running report and starts the next one in the
// same commit, landing on a lock the cancelled read has not let go of yet.
describe("useSprintReport's one retry", () => {
  it("asks a second time when the profile was busy, and only a second time", async () => {
    vi.mocked(api.GetSprintReport).mockRejectedValue(new Error(BUSY));
    const { result } = renderHook(() => useSprintReport("p1", 1, 0, run), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.report.isError).toBe(true));
    expect(api.GetSprintReport).toHaveBeenCalledTimes(2);
    await settle();
    expect(api.GetSprintReport).toHaveBeenCalledTimes(2);
  });

  it("serves the report when the second ask lands, so a mid-read switch needs no Retry", async () => {
    vi.mocked(api.GetSprintReport)
      .mockRejectedValueOnce(new Error(BUSY))
      .mockResolvedValue(report());
    const { result } = renderHook(() => useSprintReport("p1", 1, 0, run), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.report.isSuccess).toBe(true));
    expect(result.current.report.data?.series.sprintName).toBe("Sprint 11");
    expect(api.GetSprintReport).toHaveBeenCalledTimes(2);
  });

  it("does not ask again after an ordinary read failure, which asking again would only repeat", async () => {
    vi.mocked(api.GetSprintReport).mockRejectedValue(new Error("read tcp: connection reset by peer"));
    const { result } = renderHook(() => useSprintReport("p1", 1, 0, run), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.report.isError).toBe(true));
    expect(api.GetSprintReport).toHaveBeenCalledTimes(1);
    await settle();
    expect(api.GetSprintReport).toHaveBeenCalledTimes(1);
  });

  it("does not reach Jira again once the view that asked has gone", async () => {
    let refuse: (e: Error) => void = () => {};
    vi.mocked(api.GetSprintReport).mockImplementation(
      () => new Promise<SprintReport>((_, reject) => { refuse = reject; }),
    );
    const view = renderHook(() => useSprintReport("p1", 1, 0, run), { wrapper: wrapper() });
    await waitFor(() => expect(api.GetSprintReport).toHaveBeenCalledTimes(1));
    await act(async () => { refuse(new Error(BUSY)); });
    view.unmount();
    await settle();
    expect(api.GetSprintReport).toHaveBeenCalledTimes(1);
  });

  it("keeps a rebuild fresh through a busy retry, then clears it for an ordinary refetch", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(report());
    const { result } = renderHook(() => useSprintReport("p1", 1, 0, run), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.report.isSuccess).toBe(true));

    vi.mocked(api.GetSprintReport).mockReset()
      .mockRejectedValueOnce(new Error(BUSY))
      .mockResolvedValueOnce(report())
      .mockResolvedValueOnce(report());
    await act(async () => { await result.current.rebuild(); });

    expect(api.GetSprintReport).toHaveBeenNthCalledWith(1, "p1", 1, 0, true);
    expect(api.GetSprintReport).toHaveBeenNthCalledWith(2, "p1", 1, 0, true);

    await act(async () => { await result.current.report.refetch(); });
    expect(api.GetSprintReport).toHaveBeenNthCalledWith(3, "p1", 1, 0, false);
  });

  it("clears a rebuild after its final busy refusal", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(report());
    const { result } = renderHook(() => useSprintReport("p1", 1, 0, run), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.report.isSuccess).toBe(true));

    vi.mocked(api.GetSprintReport).mockReset().mockRejectedValue(new Error(BUSY));
    await act(async () => { await result.current.rebuild(); });
    await waitFor(() => expect(result.current.report.isError).toBe(true));

    vi.mocked(api.GetSprintReport).mockReset().mockResolvedValue(report());
    await act(async () => { await result.current.report.refetch(); });
    expect(api.GetSprintReport).toHaveBeenCalledWith("p1", 1, 0, false);
  });

  it("does not carry a running rebuild intent to another report key", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(report());
    const { result, rerender } = renderHook(
      ({ profileId, boardId, sprintId }) => useSprintReport(profileId, boardId, sprintId, run),
      { wrapper: wrapper(), initialProps: { profileId: "p1", boardId: 1, sprintId: 11 } },
    );
    await waitFor(() => expect(result.current.report.isSuccess).toBe(true));

    let finishOld: (value: SprintReport) => void = () => {};
    vi.mocked(api.GetSprintReport).mockReset()
      .mockImplementationOnce(() => new Promise<SprintReport>((resolve) => { finishOld = resolve; }))
      .mockResolvedValue(report());
    void result.current.rebuild();
    await waitFor(() => expect(api.GetSprintReport).toHaveBeenCalledWith("p1", 1, 11, true));

    rerender({ profileId: "p2", boardId: 2, sprintId: 22 });
    await waitFor(() => expect(api.GetSprintReport).toHaveBeenCalledWith("p2", 2, 22, false));
    finishOld(report());
  });
});

describe("live report freshness", () => {
  it("reads an active sprint again when the view is reopened", async () => {
    vi.mocked(api.GetSprintReport).mockResolvedValue(report());
    const qc = createQueryClient();
    const first = renderHook(() => useSprintReport("p1", 1, 12, run, true), { wrapper: wrapper(qc) });
    await waitFor(() => expect(first.result.current.report.isSuccess).toBe(true));
    first.unmount();

    const second = renderHook(() => useSprintReport("p1", 1, 12, run, true), { wrapper: wrapper(qc) });
    await waitFor(() => expect(api.GetSprintReport).toHaveBeenCalledTimes(2));
    second.unmount();
  });
});

// The report is the one read in TAM with an infinite staleTime, so nothing
// but an invalidation ever replaces it inside a session.
describe("what drops a stored report", () => {
  const reportKey = JSON.stringify(["p1", "sprintReport"]);
  // The keys one call to an invalidating helper passed, in the order it
  // passed them, as strings a set membership test can be written against.
  const invalidated = (spy: { mock: { calls: unknown[][] } }) =>
    spy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey?: unknown } | undefined)?.queryKey));

  it("a sprint write does, because completing one changes which sprint is the newest closed one", () => {
    const qc = createQueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");
    invalidateSprintWrites(qc, "p1");
    expect(invalidated(spy)).toContain(reportKey);
  });

  it("a sync does, because its boards pass can move a status out of the board's last column", () => {
    const qc = createQueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");
    invalidateProfileData(qc, "p1");
    expect(invalidated(spy)).toContain(reportKey);
  });

  it("a boards refresh does, for the same reason: the last column is what done means", async () => {
    const qc = createQueryClient();
    const summary = { boards: 1, columns: 3, sprints: 2, cards: 12, dropped: [], unavailable: false, elapsed: "2s" };
    vi.mocked(api.SyncBoards).mockResolvedValue(summary);
    const { result } = renderHook(
      () => useSyncBoards("p1", () => Promise.resolve(summary)),
      { wrapper: wrapper(qc) },
    );
    const spy = vi.spyOn(qc, "invalidateQueries");
    result.current.mutate();
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidated(spy)).toContain(reportKey);
  });
});
