import { describe, expect, it } from "vitest";
import type { Issue } from "../api";
import { drawnParents, familyPlace, orderFamilies } from "./issueFamilies";

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "",
    reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null,
    rank: "", created: "", updated: "",
    ...over,
  };
}

const STORY = issue({ key: "PLAT-412", type: "story", summary: "Apply promo code" });
const CHILD = issue({ key: "PLAT-500", type: "subtask", parentKey: "PLAT-412", summary: "Wire input" });
const STRAY = issue({ key: "PLAT-501", type: "subtask", parentKey: "PLAT-901", summary: "Retire the gateway" });

describe("familyPlace", () => {
  it("tells a child from a subtask whose parent was left out of the set", () => {
    // Every list here pages or truncates, so the second case is routine:
    // the parent fell outside the budget and the row is on its own.
    const drawn = drawnParents([STORY, CHILD, STRAY]);
    expect(familyPlace(STORY, drawn)).toBe("root");
    expect(familyPlace(CHILD, drawn)).toBe("child");
    expect(familyPlace(STRAY, drawn)).toBe("detached");
  });

  it("counts only the parents a row could actually sit under", () => {
    // An epic is not a parent at this level, and neither is another
    // subtask, so a row naming one of them has nothing to nest under.
    const epic = issue({ key: "PLAT-100", type: "epic" });
    const ofEpic = issue({ key: "PLAT-502", type: "subtask", parentKey: "PLAT-100" });
    const ofChild = issue({ key: "PLAT-503", type: "subtask", parentKey: CHILD.key });
    const drawn = drawnParents([epic, STORY, CHILD, ofEpic, ofChild]);
    expect(familyPlace(ofEpic, drawn)).toBe("detached");
    expect(familyPlace(ofChild, drawn)).toBe("detached");
    expect([...drawn].sort()).toEqual([STORY.key]);
  });

  it("leaves a detached subtask in the order it arrived in", () => {
    expect(orderFamilies([STORY, STRAY, CHILD]).map((i) => i.key)).toEqual([
      STORY.key, CHILD.key, STRAY.key,
    ]);
  });
});
