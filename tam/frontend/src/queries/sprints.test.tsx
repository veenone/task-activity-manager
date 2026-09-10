import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import { keys } from "./keys";
import { invalidateSprintWrites, useBoardSprintDetails, useDeleteSprint, useEditSprint } from "./sprints";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, ListBoardSprintDetails: vi.fn(), EditSprint: vi.fn(), DeleteSprint: vi.fn() };
});

const qc = createQueryClient();

function wrapper({ children }: { children: React.ReactNode }) {
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

// run stands in for SyncContext's runQuietLock once the lock is free: it
// runs the action and hands the answer back.
const run = async <T,>(action: () => Promise<T>) => action();

beforeEach(() => {
  vi.clearAllMocks();
  qc.clear();
});

describe("useBoardSprintDetails", () => {
  it("reads one board's sprints with the cards in each", async () => {
    vi.mocked(api.ListBoardSprintDetails).mockResolvedValue([]);
    const { result } = renderHook(() => useBoardSprintDetails("p1", 1), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.ListBoardSprintDetails).toHaveBeenCalledWith("p1", 1);
  });

  it("asks nothing of a kanban board, which has no sprints to ask about", () => {
    renderHook(() => useBoardSprintDetails("p1", 0), { wrapper });
    expect(api.ListBoardSprintDetails).not.toHaveBeenCalled();
  });
});

describe("the sprint writes", () => {
  it("sends clearGoal beside the draft, since a blank goal box cannot say which blank it is", async () => {
    vi.mocked(api.EditSprint).mockResolvedValue("");
    const { result } = renderHook(() => useEditSprint("p1", run), { wrapper });
    result.current.mutate({
      boardId: 1, sprintId: 12, name: "Sprint 12", goal: "", start: "2026-09-01", end: "2026-09-14", clearGoal: true,
    });
    await waitFor(() => expect(api.EditSprint).toHaveBeenCalledWith(
      "p1", 1, 12, "Sprint 12", "", "2026-09-01", "2026-09-14", true,
    ));
  });

  it("refreshes the list even when the write failed, because a delete can fail after Jira has acted", async () => {
    vi.mocked(api.DeleteSprint).mockRejectedValue(new Error("403 Forbidden"));
    const spy = vi.spyOn(qc, "invalidateQueries");
    const { result } = renderHook(() => useDeleteSprint("p1", run), { wrapper });
    result.current.mutate({ boardId: 1, sprintId: 12 });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(spy).toHaveBeenCalledWith({ queryKey: ["p1", "boardSprintDetails"] });
  });
});

describe("invalidateSprintWrites", () => {
  it("refreshes every list a sprint write can change", () => {
    const spy = vi.spyOn(qc, "invalidateQueries");
    invalidateSprintWrites(qc, "p1");
    const invalidated = spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey));
    // The board's sprint rows and this view's composed list are the obvious
    // two; the rest are the lists that go stale with them, and the
    // profile-wide open list is how the Backlog's Sprint field hears about
    // a sprint being made, renamed or closed.
    for (const key of [
      keys.boards("p1"),
      ["p1", "boardSprints"],
      ["p1", "boardSprintDetails"],
      ["p1", "board"],
      ["p1", "sprintSuggestion"],
      keys.openSprints("p1"),
    ]) {
      expect(invalidated).toContain(JSON.stringify(key));
    }
  });

  it("refreshes nothing without a profile to refresh it for", () => {
    const spy = vi.spyOn(qc, "invalidateQueries");
    invalidateSprintWrites(qc, "");
    expect(spy).not.toHaveBeenCalled();
  });
});
