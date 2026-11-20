import { describe, it, expect, vi, beforeEach } from "vitest";
import * as api from "../api";
import type { PendingChange } from "../api";
import type { AppState } from "./useAppState";
import { useCommitActions } from "./useCommitActions";

vi.mock("../api", () => ({
  ResolveConflictOverride: vi.fn(async () => {}),
  ResolveConflictKeepRemote: vi.fn(async () => {}),
  ResolveConflictMerge: vi.fn(async () => {}),
  RecreateDeletedTest: vi.fn(async () => "NEW-1"),
  DiscardPendingChange: vi.fn(async () => {}),
  DiscardAllPendingChanges: vi.fn(async () => 0),
  CommitPendingChanges: vi.fn(async () => ({ succeeded: [], conflicted: [], failed: [] })),
  CommitPendingChangesByIDs: vi.fn(async () => ({ succeeded: [], conflicted: [], failed: [] })),
  errMsg: (e: unknown) => String(e),
}));

const change = (id: number, entityType: string, entityKey: string): PendingChange =>
  ({ id, entityType, entityKey, field: "summary" }) as PendingChange;

// Pending edits on two tests: PROJ-1 is the one in conflict, PROJ-2 has
// half-finished edits that must not be pushed by resolving PROJ-1.
const pending = [
  change(1, "test_case", "PROJ-1"),
  change(2, "test_step", "PROJ-1:7"),
  change(3, "test_case", "PROJ-2"),
  change(4, "custom_field", "PROJ-2:customfield_1"),
];

function state(): AppState {
  return {
    activeId: "p1",
    notice: vi.fn(),
    closeModal: vi.fn(),
    syncStatus: "idle",
    canCommit: true,
    beginCommit: vi.fn(),
    endCommit: vi.fn(),
    setSelectedKey: vi.fn(),
    setDetailVersion: vi.fn(),
    setLastCommitResult: vi.fn(),
    reloadPending: vi.fn(),
    refreshProfileData: vi.fn(),
    pendingChanges: pending,
  } as unknown as AppState;
}

beforeEach(() => vi.clearAllMocks());

describe("resolving one conflict", () => {
  it("keep mine commits only that test's pending changes", async () => {
    await useCommitActions(state()).resolveConflictOverride("PROJ-1", "v2");
    expect(api.CommitPendingChanges).not.toHaveBeenCalled();
    expect(api.CommitPendingChangesByIDs).toHaveBeenCalledWith("p1", [1, 2]);
  });

  it("resolve and re-commit commits only that test's pending changes", async () => {
    await useCommitActions(state()).resolveConflictMerge("PROJ-1", "v2", []);
    expect(api.CommitPendingChanges).not.toHaveBeenCalled();
    expect(api.CommitPendingChangesByIDs).toHaveBeenCalledWith("p1", [1, 2]);
  });
});
