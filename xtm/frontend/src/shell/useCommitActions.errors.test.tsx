import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import * as api from "../api";
import type { CommitResult } from "../api";
import type { AppState } from "./useAppState";
import { useCommitActions } from "./useCommitActions";
import { PendingChangesModal } from "../components/PendingChangesModal";

vi.mock("../api", () => ({
  ResolveConflictOverride: vi.fn(async () => {}),
  ResolveConflictKeepRemote: vi.fn(async () => {}),
  ResolveConflictMerge: vi.fn(async () => {}),
  RecreateDeletedTest: vi.fn(async () => "NEW-1"),
  DiscardPendingChange: vi.fn(async () => {}),
  DiscardAllPendingChanges: vi.fn(async () => 0),
  CommitPendingChanges: vi.fn(),
  CommitPendingChangesByIDs: vi.fn(),
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

// useConfirm wraps DialogContext, which is not mounted here.
vi.mock("../components/useConfirm", () => ({
  useConfirm: () => ({ confirm: vi.fn(async () => true) }),
}));

// state records whatever the handlers put into lastCommitResult, which is
// what the pending modal renders as its error list.
function state() {
  let result: CommitResult | null = null;
  const s = {
    activeId: "p1",
    notice: vi.fn(),
    closeModal: vi.fn(),
    syncStatus: "idle",
    canCommit: true,
    beginCommit: vi.fn(),
    endCommit: vi.fn(),
    setSelectedKey: vi.fn(),
    setDetailVersion: vi.fn(),
    setLastCommitResult: vi.fn((v: CommitResult | null | ((p: CommitResult | null) => CommitResult | null)) => {
      result = typeof v === "function" ? v(result) : v;
    }),
    reloadPending: vi.fn(),
    refreshProfileData: vi.fn(),
    pendingChanges: [],
  } as unknown as AppState;
  return { s, result: () => result };
}

function renderModal(lastResult: CommitResult | null) {
  render(
    <PendingChangesModal
      changes={[]}
      lastResult={lastResult}
      committing={false}
      onClose={() => {}}
      onDiscard={() => {}}
      onDiscardAll={() => {}}
      onCommit={() => {}}
      onCommitIds={() => {}}
      onJumpTo={() => {}}
      onResolveOverride={() => {}}
      onResolveKeepRemote={() => {}}
      onResolveMerge={() => {}}
      onResolveRecreate={() => {}}
    />,
  );
}

beforeEach(() => vi.clearAllMocks());

describe("failed discard or resolution", () => {
  it("shows the error when Discard all is rejected", async () => {
    vi.mocked(api.DiscardAllPendingChanges).mockRejectedValueOnce(new Error("database is locked"));
    const { s, result } = state();
    await useCommitActions(s).handleDiscardAll();
    renderModal(result());
    expect(screen.getByText(/database is locked/)).toBeTruthy();
  });

  it("reports a rejected single discard", async () => {
    vi.mocked(api.DiscardPendingChange).mockRejectedValueOnce(new Error("row gone"));
    const { s, result } = state();
    await useCommitActions(s).handleDiscard(4);
    expect(result()?.failed.map((f) => f.error)).toEqual(["row gone"]);
  });

  it("reports a rejected keep remote against its test", async () => {
    vi.mocked(api.ResolveConflictKeepRemote).mockRejectedValueOnce(new Error("no store"));
    const { s, result } = state();
    await useCommitActions(s).resolveConflictKeepRemote("PROJ-1");
    expect(result()?.failed).toEqual([{ testKey: "PROJ-1", error: "no store" }]);
  });

  it("reports a rejected recreate against its test", async () => {
    vi.mocked(api.RecreateDeletedTest).mockRejectedValueOnce(new Error("create failed"));
    const { s, result } = state();
    await useCommitActions(s).resolveConflictRecreate("PROJ-1");
    expect(result()?.failed).toEqual([{ testKey: "PROJ-1", error: "create failed" }]);
  });
});
