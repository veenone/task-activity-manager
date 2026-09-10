import { describe, it, expect } from "vitest";
import { ENTITY_RANK, ENTITY_SPRINT_MOVE, FIELD_SPRINT_ID } from "../api";
import type { PendingChange } from "../api";
import { journalTouchesSprint, moveId } from "./moveValue";

function row(over: Partial<PendingChange>): PendingChange {
  return {
    id: 1, entityType: ENTITY_SPRINT_MOVE, entityKey: "PLAT-409", field: FIELD_SPRINT_ID,
    beforeVal: "", afterVal: "", baseVersion: "", createdAt: "",
    ...over,
  };
}

describe("moveId", () => {
  it("reads the id half of a journaled place, and the whole value when there is no name", () => {
    expect(moveId("12|Sprint 12")).toBe("12");
    expect(moveId("12")).toBe("12");
    // The backlog is journaled as an empty value, which is a place and not
    // an id, so it matches no sprint.
    expect(moveId("")).toBe("");
  });
});

describe("journalTouchesSprint", () => {
  it("finds a card journaled out of the sprint, which is what shortens its count", () => {
    const rows = [row({ beforeVal: "12|Sprint 12", afterVal: "13|Sprint 13" })];
    expect(journalTouchesSprint(rows, 12)).toBe(true);
  });

  it("finds a card journaled into the sprint too, which lengthens it early", () => {
    const rows = [row({ beforeVal: "12|Sprint 12", afterVal: "13|Sprint 13" })];
    expect(journalTouchesSprint(rows, 13)).toBe(true);
  });

  it("reads the id and not the name, since a cached name can differ from the board's", () => {
    const rows = [row({ beforeVal: "12|Sprint twelve", afterVal: "" })];
    expect(journalTouchesSprint(rows, 12)).toBe(true);
    // A move to the backlog names no destination sprint at all.
    expect(journalTouchesSprint(rows, 0)).toBe(false);
  });

  it("ignores the sprint no pending move names, and the moves that are not sprint moves", () => {
    const rows = [
      row({ beforeVal: "12|Sprint 12", afterVal: "13|Sprint 13" }),
      row({ entityType: ENTITY_RANK, beforeVal: "", afterVal: "before|PLAT-412|1" }),
    ];
    expect(journalTouchesSprint(rows, 14)).toBe(false);
    expect(journalTouchesSprint([], 12)).toBe(false);
  });
});
