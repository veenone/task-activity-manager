import { describe, it, expect } from "vitest";
import { filterBoard, matchesCard } from "./boardFilter";
import type { BoardView, Issue } from "../api";

function card(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "story", summary: "A card",
    status: "To Do", assignee: "", reporter: "", priority: "", labels: [],
    sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "",
    created: "", updated: "", ...over,
  };
}

const anand = card({ key: "PLAT-412", assignee: "R. Anand", type: "story", storyPoints: 5 });
const ortiz = card({ key: "PLAT-401", assignee: "M. Ortiz", type: "bug", storyPoints: 3 });
const draft = card({ key: "TAM-NEW-1", assignee: "", type: "task", storyPoints: 2 });

function view(): BoardView {
  return {
    boardId: 1, sprintId: "", swimlane: "none",
    columns: [
      { name: "To Do", statusIds: ["1"], total: 2, points: 8 },
      { name: "Done", statusIds: ["5"], total: 1, points: 2 },
    ],
    lanes: [{ id: "", label: "All", count: 3, cells: [[anand, ortiz], [draft]], overflow: [4, 0] }],
    donePoints: 2, unmapped: 1, unmappedStatuses: ["Blocked"], notSynced: 0,
    capped: true, needsStatusSync: false,
  };
}

describe("matchesCard", () => {
  it("matches on key, assignee, or issue type, case-insensitively", () => {
    expect(matchesCard(anand, "412")).toBe(true);
    expect(matchesCard(anand, "anand")).toBe(true);
    expect(matchesCard(anand, "STORY")).toBe(true);
    expect(matchesCard(anand, "ortiz")).toBe(false);
  });

  it("matches everything on a blank filter", () => {
    expect(matchesCard(anand, "   ")).toBe(true);
  });
});

describe("filterBoard", () => {
  it("returns the view untouched when nothing is being filtered", () => {
    const v = view();
    expect(filterBoard(v, "")).toBe(v);
  });

  it("keeps the board's shape and recounts what survives", () => {
    const out = filterBoard(view(), "bug");
    // Every column stays, so a filter never looks like a lost column.
    expect(out.columns.map((c) => c.name)).toEqual(["To Do", "Done"]);
    expect(out.lanes[0].cells[0].map((c) => c.key)).toEqual(["PLAT-401"]);
    expect(out.lanes[0].cells[1]).toEqual([]);
    // The counts describe what is drawn, not what the board holds.
    expect(out.columns[0].total).toBe(1);
    expect(out.columns[0].points).toBe(3);
    expect(out.columns[1].total).toBe(0);
    expect(out.lanes[0].count).toBe(1);
  });

  it("drops overflow rather than guessing at it", () => {
    // The cards a cap left out were never filtered, so no number for them
    // would be honest.
    expect(filterBoard(view(), "bug").lanes[0].overflow).toEqual([0, 0]);
  });

  it("leaves the board's own totals alone", () => {
    // These describe the board, not this view of it.
    const out = filterBoard(view(), "bug");
    expect(out.unmapped).toBe(1);
    expect(out.donePoints).toBe(2);
    expect(out.capped).toBe(true);
  });
});
