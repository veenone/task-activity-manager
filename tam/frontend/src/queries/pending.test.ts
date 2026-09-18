import { describe, it, expect } from "vitest";
import type { PendingChange } from "../api";
import { groupPending, sprintWaiting } from "./pending";

function row(id: number, entityType: string, entityKey: string, afterVal = "{}"): PendingChange {
  return { id, entityType, entityKey, field: "x", beforeVal: "", afterVal, baseVersion: "", createdAt: "" };
}

describe("groupPending", () => {
  it("keeps a sprint, a board and an issue that share a key in their own groups", () => {
    const groups = groupPending([
      row(4, "issue", "-1"),
      row(3, "board_create", "-1", JSON.stringify({ name: "Checkout", type: "scrum", filterName: "f", jql: "project = PLAT" })),
      row(2, "sprint_create", "-1", JSON.stringify({ boardId: 1, name: "Sprint 15" })),
      row(1, "sprint_edit", "-1"),
    ]);
    expect(groups).toHaveLength(3);
    expect(new Set(groups.map((g) => g.id)).size).toBe(3);
    expect(groups.every((g) => g.key === "-1")).toBe(true);
    const sprint = groups.find((g) => g.sprintRow)!;
    expect(sprint.sprint?.name).toBe("Sprint 15");
    expect(sprint.sprintChanges.map((r) => r.id)).toEqual([1]);
    expect(sprint.edits).toEqual([]);
    const issue = groups.find((g) => g.edits.some((r) => r.id === 4))!;
    expect(issue.sprintRow).toBeNull();
    expect(issue.edits.map((r) => r.id)).toEqual([4]);
  });

  it("orders draft sprints, then sprint changes, then drafts, then the rest", () => {
    const groups = groupPending([
      row(5, "issue", "PLAT-1"),
      row(4, "issue_create", "TAM-NEW-1"),
      row(3, "sprint_delete", "13"),
      row(2, "sprint_create", "-1"),
    ]);
    expect(groups.map((g) => g.key)).toEqual(["-1", "13", "TAM-NEW-1", "PLAT-1"]);
    expect(groups[1].sprintChanges.map((r) => r.id)).toEqual([3]);
  });
});

describe("sprintWaiting", () => {
  it("names what Commit will do to each sprint and ignores every other row", () => {
    const waiting = sprintWaiting([
      row(1, "sprint_start", "13"),
      row(2, "sprint_complete", "12"),
      row(3, "sprint_delete", "14"),
      row(4, "sprint_edit", "15"),
      row(5, "issue", "16"),
    ]);
    expect([...waiting.entries()]).toEqual([
      [13, "Starting on Commit"],
      [12, "Completing on Commit"],
      [14, "Deleting on Commit"],
    ]);
  });
});
