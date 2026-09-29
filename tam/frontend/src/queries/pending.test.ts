import { describe, it, expect } from "vitest";
import type { PendingChange, Worklog } from "../api";
import { groupPending, sprintWaiting, worklogTotal } from "./pending";

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

  // A worklog row sorted into edits would be drawn as a field change from
  // nothing to a blob of JSON, which is the only rendering the dialog has for
  // a row it does not recognise.
  it("reads a worklog row as an entry rather than a field edit", () => {
    const entry = { started: "2026-09-29T01:00:00.000+0700", timeSpent: "2h 30m", comment: "Pairing", seconds: 9000 };
    const groups = groupPending([
      row(2, "worklog", "PLAT-1", JSON.stringify(entry)),
      row(1, "issue", "PLAT-1"),
    ]);
    expect(groups).toHaveLength(1);
    expect(groups[0].edits.map((r) => r.id)).toEqual([1]);
    expect(groups[0].worklogs.map((w) => w.row.id)).toEqual([2]);
    expect(groups[0].worklogs[0].log.timeSpent).toBe("2h 30m");
    expect(groups[0].worklogs[0].log.comment).toBe("Pairing");
  });

  it("keeps a worklog row whose payload will not parse, as an edit to discard", () => {
    const groups = groupPending([row(7, "worklog", "PLAT-1", "not json")]);
    expect(groups[0].worklogs).toEqual([]);
    expect(groups[0].edits.map((r) => r.id)).toEqual([7]);
  });
});

describe("worklogTotal", () => {
  it("adds Jira's seconds and the pending ones together", () => {
    const logs = [
      { seconds: 3600 },
      { seconds: 9000, pending: true },
    ] as Worklog[];
    expect(worklogTotal(logs)).toBe(12600);
    expect(worklogTotal([])).toBe(0);
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
