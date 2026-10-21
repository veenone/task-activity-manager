import { describe, it, expect } from "vitest";
import { filterBoard, groupBoard, matchesCard } from "./boardFilter";
import { ASSIGNEE_NONE } from "../api";
import type { BoardView, Issue } from "../api";
import { NO_FILTER } from "./issueFilter";

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
  it("groups siblings under a parent without moving children out of their status columns", () => {
    const v = view();
    const child = card({ key: "PLAT-2", type: "subtask", parentKey: anand.key });
    const doneChild = card({ key: "PLAT-3", type: "subtask", parentKey: anand.key, status: "Done" });
    v.lanes[0].cells = [[child, ortiz, anand], [doneChild]];
    const grouped = groupBoard(v);
    expect(grouped.lanes[0].cells[0].map((i) => i.key)).toEqual([ortiz.key, anand.key, child.key]);
    const filtered = groupBoard(filterBoard(v, child.key));
    expect(filtered.lanes[0].cells[0].map((i) => i.key)).toEqual([anand.key, child.key]);
    expect(filtered.lanes[0].cells[1]).toEqual([doneChild]);
    expect(v.lanes[0].cells[0][0]).toBe(child);
  });
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

describe("filterBoard's type, status and assignee bar", () => {
  // The board filters in the browser because it composes itself from one read:
  // narrowing it in SQL would mean refetching the whole board to answer a
  // question the cards on screen already carry.
  it("narrows the cards the same way the store narrows a query", () => {
    const out = filterBoard(view(), "", { types: ["bug"], status: "", assignee: "" });
    expect(out.lanes[0].cells.flat().map((c) => c.key)).toEqual(["PLAT-401"]);
    expect(out.columns.map((c) => c.total)).toEqual([1, 0]);
  });

  it("asks for unassigned cards as its own choice", () => {
    const out = filterBoard(view(), "", { types: [], status: "", assignee: ASSIGNEE_NONE });
    expect(out.lanes[0].cells.flat().map((c) => c.key)).toEqual(["TAM-NEW-1"]);
  });

  // The text box and the bar narrow together, so a standup can type a key and
  // still have the bar's choice applied rather than silently replaced.
  it("ANDs the text box with the bar", () => {
    expect(filterBoard(view(), "PLAT-401", { types: ["bug"], status: "", assignee: "" }).lanes[0].count).toBe(1);
    expect(filterBoard(view(), "PLAT-401", { types: ["story"], status: "", assignee: "" }).lanes[0].count).toBe(0);
  });

  // The untouched board is returned by identity, which is what keeps a render
  // with no filter from rebuilding every lane.
  it("returns the board untouched when neither is set", () => {
    const v = view();
    expect(filterBoard(v, "", NO_FILTER)).toBe(v);
  });
});
