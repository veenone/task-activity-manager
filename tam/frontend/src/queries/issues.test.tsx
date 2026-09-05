import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import { useIssues, useSyncState } from "./issues";
import { invalidateProfileData } from "./invalidate";
import { keys } from "./keys";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListIssues: vi.fn(),
    GetSyncState: vi.fn(),
  };
});

const query = { text: "", types: [], sprintId: "", offset: 0, limit: 25 };

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0 });
});

function wrapper(qc = createQueryClient()) {
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

describe("issue queries", () => {
  it("does not fetch without a profile", async () => {
    const { result } = renderHook(() => useIssues("", query), { wrapper: wrapper() });
    await new Promise((r) => setTimeout(r, 10));
    expect(result.current.fetchStatus).toBe("idle");
    expect(api.ListIssues).not.toHaveBeenCalled();
  });

  it("fetches the page for the profile and query", async () => {
    vi.mocked(api.ListIssues).mockResolvedValue({
      issues: [{ key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", rank: "", created: "", updated: "" }],
      total: 1,
    });
    const { result } = renderHook(() => useIssues("p1", query), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.data?.total).toBe(1));
    expect(api.ListIssues).toHaveBeenCalledWith("p1", query);
  });

  it("invalidateProfileData refetches the issue and sync-state families", async () => {
    const qc = createQueryClient();
    const { result } = renderHook(() => ({ issues: useIssues("p1", query), state: useSyncState("p1") }), { wrapper: wrapper(qc) });
    await waitFor(() => expect(result.current.state.data).toBeDefined());
    expect(api.ListIssues).toHaveBeenCalledTimes(1);
    expect(api.GetSyncState).toHaveBeenCalledTimes(1);
    invalidateProfileData(qc, "p1");
    await waitFor(() => expect(api.ListIssues).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(api.GetSyncState).toHaveBeenCalledTimes(2));
  });

  it("keys are profile-scoped", () => {
    expect(keys.issues("p1", query)[0]).toBe("p1");
    expect(keys.issue("p1", "PLAT-1")).toEqual(["p1", "issue", "PLAT-1"]);
    expect(keys.linkedTests("p1", "PLAT-1")).toEqual(["p1", "issue", "PLAT-1", "tests"]);
  });
});
