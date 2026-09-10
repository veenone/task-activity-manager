import { describe, it, expect } from "vitest";
import { unfinished } from "./unfinished";
import type { ColumnView, Issue } from "../api";

function issue(key: string, statusId: string): Issue {
  return {
    key, id: key, project: "PLAT", type: "task", summary: key, status: "To Do", statusId,
    assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "",
    parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
  };
}

const COLUMNS: ColumnView[] = [
  { name: "To Do", statusIds: ["1"], total: 0, points: 0 },
  { name: "In Progress", statusIds: ["3"], total: 0, points: 0 },
  { name: "Done", statusIds: ["5", "6"], total: 0, points: 0 },
];

describe("unfinished", () => {
  it("keeps every card the last column does not collect", () => {
    const cards = [issue("PLAT-1", "1"), issue("PLAT-2", "3"), issue("PLAT-3", "5")];
    expect(unfinished(cards, COLUMNS).map((i) => i.key)).toEqual(["PLAT-1", "PLAT-2"]);
  });

  it("counts every status the last column collects as finished", () => {
    // A last column mapping two statuses finishes a card in either of them.
    expect(unfinished([issue("PLAT-3", "6")], COLUMNS)).toEqual([]);
  });

  it("treats a card with no status id as unfinished", () => {
    // A draft is the case: Jira has never given it a status, and the board
    // draws it in the first column for the same reason.
    expect(unfinished([issue("PLAT-9", "")], COLUMNS).map((i) => i.key)).toEqual(["PLAT-9"]);
  });

  it("answers with nothing on a board with no last column to judge against", () => {
    const cards = [issue("PLAT-1", "1")];
    expect(unfinished(cards, [COLUMNS[0]])).toEqual([]);
    expect(unfinished(cards, [])).toEqual([]);
  });
});
