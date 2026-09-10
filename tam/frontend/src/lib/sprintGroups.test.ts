import { describe, it, expect } from "vitest";
import { UNASSIGNED_LABEL, groupByAssignee } from "./sprintGroups";
import type { Issue } from "../api";

function issue(key: string, assignee: string, storyPoints: number | null = null): Issue {
  return {
    key, id: key, project: "PLAT", type: "task", summary: key, status: "To Do", statusId: "1",
    assignee, reporter: "", priority: "", labels: [], sprintId: "12", sprintName: "Sprint 12",
    parentKey: "", storyPoints, rank: "", created: "", updated: "",
  };
}

describe("groupByAssignee", () => {
  it("orders the bands by points, whatever order the cards arrived in", () => {
    const groups = groupByAssignee([
      issue("PLAT-1", "M. Ortiz", 2),
      issue("PLAT-2", "R. Anand", 8),
      issue("PLAT-3", "R. Anand", 3),
    ]);
    expect(groups.map((g) => g.label)).toEqual(["R. Anand", "M. Ortiz"]);
    expect(groups[0].points).toBe(11);
    expect(groups[0].issues.map((i) => i.key)).toEqual(["PLAT-2", "PLAT-3"]);
  });

  it("breaks a tie on points by the count, and a tie on both by the name", () => {
    const groups = groupByAssignee([
      issue("PLAT-1", "S. Wu"),
      issue("PLAT-2", "A. Blum"),
      issue("PLAT-3", "K. Ito"),
      issue("PLAT-4", "K. Ito"),
    ]);
    // K. Ito carries two cards and no points, so the count decides; the
    // other two have neither, so their names do.
    expect(groups.map((g) => g.label)).toEqual(["K. Ito", "A. Blum", "S. Wu"]);
  });

  it("puts the cards nobody owns last, however much they carry", () => {
    const groups = groupByAssignee([
      issue("PLAT-1", "", 21),
      issue("PLAT-2", "R. Anand", 1),
    ]);
    expect(groups.map((g) => g.label)).toEqual(["R. Anand", UNASSIGNED_LABEL]);
    expect(groups[1].id).toBe("");
  });

  it("keeps the cards of one band in the order they came in", () => {
    const groups = groupByAssignee([issue("PLAT-9", "R. Anand"), issue("PLAT-2", "R. Anand")]);
    expect(groups[0].issues.map((i) => i.key)).toEqual(["PLAT-9", "PLAT-2"]);
  });

  it("has no bands for a sprint with no cards", () => {
    expect(groupByAssignee([])).toEqual([]);
  });
});
