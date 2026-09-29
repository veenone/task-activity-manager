import { describe, expect, it } from "vitest";
import type { RitualDocument } from "../api";
import { isBoardAgreement, navItems, navKey, selectedKey } from "./ritualNav";

function doc(ritualType: string, sprintId = 14): RitualDocument {
  return {
    profileId: "p1", boardId: 1, sprintId, ritualType, title: `${ritualType} ${sprintId}`,
    body: "", baseBody: "", pageId: "", version: 0, conflictBody: "", conflictVersion: 0,
    status: "local", updatedAt: "", syncedAt: "",
  };
}

const five = ["_sprint", "planning", "standup", "review", "retro"].map((t) => doc(t));

describe("ritualNav", () => {
  it("lists the five pages, then the board's agreement, then the sprint's additions", () => {
    const items = navItems([...five, doc("doneagreement", 0), doc("doneagreement")]);
    expect(items.map((i) => i.label)).toEqual([
      "Overview", "Planning", "Standup", "Review", "Retrospective", "Done agreement", "This sprint's additions",
    ]);
    expect(items.map((i) => i.key)).toEqual([
      "14:_sprint", "14:planning", "14:standup", "14:review", "14:retro", "0:doneagreement", "14:doneagreement",
    ]);
  });

  // Two documents of the same type in one list is what the board's agreement
  // and a sprint's additions are, so a key built from the type alone would
  // collide and the nav would open one of them for both.
  it("keys the two agreements apart", () => {
    expect(navKey(doc("doneagreement", 0))).not.toEqual(navKey(doc("doneagreement")));
    expect(isBoardAgreement(doc("doneagreement", 0))).toBe(true);
    expect(isBoardAgreement(doc("doneagreement"))).toBe(false);
    expect(isBoardAgreement(doc("planning", 0))).toBe(false);
  });

  it("leaves out an agreement the board has not asked for", () => {
    expect(navItems(five).map((i) => i.key)).toEqual([
      "14:_sprint", "14:planning", "14:standup", "14:review", "14:retro",
    ]);
  });

  it("opens Planning by default and the first document when there is none", () => {
    expect(selectedKey(navItems(five), "")).toBe("14:planning");
    expect(selectedKey(navItems(five), "14:retro")).toBe("14:retro");
    const closed = navItems([doc("_sprint", 13), doc("standup", 13)]);
    expect(selectedKey(closed, "14:planning")).toBe("13:_sprint");
    expect(selectedKey([], "14:planning")).toBe("14:planning");
  });
});
