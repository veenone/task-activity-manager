import { describe, it, expect } from "vitest";
import type { CommitResult, PendingChange } from "../api";
import { cardMoves } from "./cardMoveState";

function row(over: Partial<PendingChange>): PendingChange {
  return {
    id: 12, entityType: "issue_transition", entityKey: "PLAT-412", field: "statusId",
    beforeVal: "1|To Do", afterVal: "3|In Progress", baseVersion: "v1", createdAt: "",
    ...over,
  };
}

function commit(over: Partial<CommitResult>): CommitResult {
  return {
    committed: [], created: [], linked: [], moved: [], conflicts: [], failures: [], remaining: 0,
    ...over,
  };
}

const REFUSED = {
  key: "PLAT-412", entityType: "issue_transition", rowId: 12, retryable: false,
  error: "PLAT-412 cannot move to In Progress: no transition reaches it",
};

const NONE = { warnings: new Map(), checking: new Set<string>() };

describe("cardMoves", () => {
  it("marks a card whose move a Commit refused", () => {
    const moves = cardMoves({ pending: [row({})], commit: commit({ failures: [REFUSED] }), ...NONE });
    expect(moves.get("PLAT-412")).toEqual({ state: "failed", reason: REFUSED.error });
  });

  it("forgets the failure once its journal row is gone", () => {
    // The Undo on the banner discards the row, but lastCommit stands until
    // the next Commit. A card still wearing the old reason would offer an
    // Undo with nothing left to discard.
    const moves = cardMoves({ pending: [], commit: commit({ failures: [REFUSED] }), ...NONE });
    expect(moves.has("PLAT-412")).toBe(false);
  });

  it("leaves a card whose other move is still pending in the failed state", () => {
    const moves = cardMoves({
      pending: [row({ id: 13, entityType: "issue_rank", field: "rank", beforeVal: "", afterVal: "before|PLAT-409|1" })],
      commit: commit({ failures: [REFUSED] }),
      ...NONE,
    });
    expect(moves.get("PLAT-412")?.state).toBe("failed");
  });
});
