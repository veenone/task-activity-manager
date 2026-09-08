import { describe, it, expect } from "vitest";
import type { BoardView, Issue } from "../api";
import { columnDrop, isNoMove, isSameCell, keyboardMove, landedMessage } from "./cardMove";

// Cards forty pixels tall, stacked from the top of the cell, so a clientY
// can be read as "above the first card" or "in the lower half of the
// second" without a layout engine.
function rects(n: number) {
  return Array.from({ length: n }, (_, i) => ({ top: i * 40, bottom: i * 40 + 40 }));
}

function issue(key: string, draft = false): Issue {
  return {
    key, id: key, project: "PLAT", type: "task", summary: key, status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "",
    created: "", updated: "", draft,
  };
}

// cards is the shape a cell hands columnDrop: a key and whether it is a
// draft.
function cards(...keys: string[]) {
  return keys.map((key) => ({ key, draft: key.startsWith("TAM-NEW-") }));
}

function view(cells: Issue[][], overflow = [0, 0, 0]): BoardView {
  return {
    boardId: 1,
    sprintId: "",
    swimlane: "none",
    columns: [
      { name: "To Do", statusIds: ["1"], total: 0, points: 0 },
      { name: "In Progress", statusIds: ["3"], total: 0, points: 0 },
      { name: "Done", statusIds: ["5"], total: 0, points: 0 },
    ],
    lanes: [{ id: "", label: "All issues", count: cells.flat().length, cells, overflow }],
    donePoints: 0,
    unmapped: 0,
    unmappedStatuses: [],
    notSynced: 0,
    capped: false,
    needsStatusSync: false,
  };
}

const KEYS = cards("PLAT-409", "PLAT-412", "PLAT-347");

describe("columnDrop", () => {
  it("puts a drop above the first card's middle before that card", () => {
    expect(columnDrop(KEYS, 5, rects(3))).toEqual({ index: 0, neighbourKey: "PLAT-409", before: true });
    // The midpoint itself belongs to the lower half, so the line does not
    // flicker between two answers as the cursor crosses it.
    expect(columnDrop(KEYS, 20, rects(3))).toEqual({ index: 1, neighbourKey: "PLAT-409", before: false });
  });

  it("puts a drop below the last card after it", () => {
    expect(columnDrop(KEYS, 400, rects(3))).toEqual({ index: 3, neighbourKey: "PLAT-347", before: false });
  });

  it("measures each gap against the card above it", () => {
    expect(columnDrop(KEYS, 50, rects(3))).toEqual({ index: 1, neighbourKey: "PLAT-409", before: false });
    expect(columnDrop(KEYS, 90, rects(3))).toEqual({ index: 2, neighbourKey: "PLAT-412", before: false });
  });

  it("has no neighbour in an empty cell, which is a column move and not a rank", () => {
    expect(columnDrop([], 40, [])).toEqual({ index: 0, neighbourKey: "", before: false });
  });

  it("skips a card the browser has not laid out rather than measuring it against zero", () => {
    expect(columnDrop(KEYS, 50, [undefined as never, ...rects(3).slice(1)])).toEqual({
      index: 1, neighbourKey: "PLAT-409", before: false,
    });
  });

  it("ranks against the nearest card Jira has, never against a draft", () => {
    // The board draws the draft where it was dropped, but the committed
    // order can never name it, so a rank anchored on it would be pushed
    // from the card's stale position instead.
    const withDraft = cards("PLAT-350", "TAM-NEW-1", "PLAT-409");
    expect(columnDrop(withDraft, 90, rects(3))).toEqual({ index: 2, neighbourKey: "PLAT-350", before: false });
    expect(columnDrop(withDraft, 50, rects(3))).toEqual({ index: 1, neighbourKey: "PLAT-350", before: false });
  });

  it("looks below the gap when every card above it is a draft", () => {
    expect(columnDrop(cards("TAM-NEW-1", "PLAT-409"), 50, rects(2))).toEqual({
      index: 1, neighbourKey: "PLAT-409", before: true,
    });
  });

  it("has no neighbour in a cell of nothing but drafts", () => {
    expect(columnDrop(cards("TAM-NEW-1", "TAM-NEW-2"), 50, rects(2))).toEqual({
      index: 1, neighbourKey: "", before: false,
    });
  });
});

describe("isSameCell and isNoMove", () => {
  const p = { lane: 0, col: 1, index: 2 };

  it("is the same cell only in the same lane and column", () => {
    expect(isSameCell(p, 0, 1)).toBe(true);
    expect(isSameCell(p, 1, 1)).toBe(false);
    expect(isSameCell(p, 0, 2)).toBe(false);
    expect(isSameCell(undefined, 0, 1)).toBe(false);
  });

  it("calls both gaps beside the card its own place", () => {
    expect(isNoMove(p, 0, 1, 2)).toBe(true);
    expect(isNoMove(p, 0, 1, 3)).toBe(true);
    expect(isNoMove(p, 0, 1, 1)).toBe(false);
    expect(isNoMove(p, 0, 2, 2)).toBe(false);
  });
});

describe("keyboardMove", () => {
  const board = view([[issue("PLAT-409"), issue("PLAT-347")], [], [issue("PLAT-412")]]);

  it("steps to the adjacent column even when it is empty", () => {
    expect(keyboardMove(board, { lane: 0, col: 0, index: 0 }, "PLAT-409", "ArrowRight")).toEqual({ kind: "column", col: 1 });
  });

  it("refuses to step past either end, and says so", () => {
    expect(keyboardMove(board, { lane: 0, col: 0, index: 0 }, "PLAT-409", "ArrowLeft")).toEqual({
      kind: "refused", message: "PLAT-409 is already in the first column",
    });
    expect(keyboardMove(board, { lane: 0, col: 2, index: 0 }, "PLAT-412", "ArrowRight")).toEqual({
      kind: "refused", message: "PLAT-412 is already in the last column",
    });
  });

  it("refuses a column that collects no status", () => {
    const noStatus = view([[issue("PLAT-409")], [], []]);
    noStatus.columns[1] = { name: "Backlog", statusIds: [], total: 0, points: 0 };
    expect(keyboardMove(noStatus, { lane: 0, col: 0, index: 0 }, "PLAT-409", "ArrowRight")).toEqual({
      kind: "refused", message: "Backlog collects no status, so a card cannot be moved into it",
    });
  });

  it("ranks a card against the one it swaps with", () => {
    expect(keyboardMove(board, { lane: 0, col: 0, index: 0 }, "PLAT-409", "ArrowDown")).toEqual({
      kind: "rank", index: 1, neighbourKey: "PLAT-347", before: false,
    });
    expect(keyboardMove(board, { lane: 0, col: 0, index: 1 }, "PLAT-347", "ArrowUp")).toEqual({
      kind: "rank", index: 0, neighbourKey: "PLAT-409", before: true,
    });
  });

  it("refuses the top and the bottom of a cell, and names the cards it cannot see past", () => {
    expect(keyboardMove(board, { lane: 0, col: 0, index: 0 }, "PLAT-409", "ArrowUp")).toEqual({
      kind: "refused", message: "PLAT-409 is already at the top of this column",
    });
    expect(keyboardMove(board, { lane: 0, col: 0, index: 1 }, "PLAT-347", "ArrowDown")).toEqual({
      kind: "refused", message: "PLAT-347 is already at the bottom of this column",
    });
    const capped = view([[issue("PLAT-409"), issue("PLAT-347")], [], []], [41, 0, 0]);
    expect(keyboardMove(capped, { lane: 0, col: 0, index: 1 }, "PLAT-347", "ArrowDown")).toEqual({
      kind: "refused", message: "PLAT-347 cannot move past the cards this column is not showing",
    });
  });

  it("steps over a draft rather than offering it as a neighbour", () => {
    const withDraft = view([[issue("PLAT-409"), issue("TAM-NEW-1", true), issue("PLAT-347")], [], []]);
    expect(keyboardMove(withDraft, { lane: 0, col: 0, index: 0 }, "PLAT-409", "ArrowDown")).toEqual({
      kind: "rank", index: 2, neighbourKey: "PLAT-347", before: false,
    });
    expect(keyboardMove(withDraft, { lane: 0, col: 0, index: 2 }, "PLAT-347", "ArrowUp")).toEqual({
      kind: "rank", index: 0, neighbourKey: "PLAT-409", before: true,
    });
  });

  it("refuses the step when a draft is all there is in that direction", () => {
    const withDraft = view([[issue("TAM-NEW-1", true), issue("PLAT-409")], [], []]);
    expect(keyboardMove(withDraft, { lane: 0, col: 0, index: 1 }, "PLAT-409", "ArrowUp")).toEqual({
      kind: "refused", message: "PLAT-409 is already at the top of this column",
    });
  });

  it("leaves every other key to the focus model", () => {
    expect(keyboardMove(board, { lane: 0, col: 0, index: 0 }, "PLAT-409", "Home")).toBeUndefined();
  });
});

describe("landedMessage", () => {
  const board = view([[issue("PLAT-409"), issue("PLAT-347")], [issue("PLAT-412")], []]);

  it("names the column and the place in it", () => {
    expect(landedMessage({ key: "PLAT-412", kind: "column", target: "In Progress", at: 1 }, board, { lane: 0, col: 1, index: 0 }))
      .toBe("PLAT-412 moved to In Progress, 1 of 1");
  });

  it("says which way a rank went and where it ended up", () => {
    expect(landedMessage({ key: "PLAT-347", kind: "up", target: "", at: 1 }, board, { lane: 0, col: 0, index: 1 }))
      .toBe("PLAT-347 moved up, 2 of 2");
    expect(landedMessage({ key: "PLAT-409", kind: "down", target: "", at: 1 }, board, { lane: 0, col: 0, index: 0 }))
      .toBe("PLAT-409 moved down, 1 of 2");
  });

  it("names the destination alone for a card that has left this board", () => {
    expect(landedMessage({ key: "PLAT-412", kind: "sprint", target: "Sprint 13", at: 1 }, board, undefined))
      .toBe("PLAT-412 moved to Sprint 13");
    expect(landedMessage({ key: "PLAT-412", kind: "sprint", target: "", at: 1 }, board, undefined))
      .toBe("PLAT-412 moved to the backlog");
  });
});
