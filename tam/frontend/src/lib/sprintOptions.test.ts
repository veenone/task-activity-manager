import { describe, it, expect } from "vitest";
import type { SprintOption } from "../api";
import { duplicateNameIds, sprintOptionLabel } from "./sprintOptions";

describe("duplicateNameIds", () => {
  it("marks both sprints of a shared name, not only the second", () => {
    const sprints: SprintOption[] = [
      { id: 12, name: "Sprint 1", boardName: "Platform board" },
      { id: 13, name: "Sprint 1", boardName: "Ops board" },
      { id: 14, name: "Sprint 2", boardName: "Ops board" },
    ];
    const dups = duplicateNameIds(sprints);
    expect(dups.has(12)).toBe(true);
    expect(dups.has(13)).toBe(true);
    expect(dups.has(14)).toBe(false);
  });

  it("folds a shared name case-insensitively", () => {
    const sprints: SprintOption[] = [
      { id: 12, name: "Sprint 1", boardName: "Platform board" },
      { id: 13, name: "sprint 1", boardName: "Ops board" },
    ];
    const dups = duplicateNameIds(sprints);
    expect(dups.has(12)).toBe(true);
    expect(dups.has(13)).toBe(true);
  });

  it("leaves a unique name alone", () => {
    const dups = duplicateNameIds([{ id: 12, name: "Sprint 1" }]);
    expect(dups.size).toBe(0);
  });
});

describe("sprintOptionLabel", () => {
  it("adds the board name only for a duplicate that carries one", () => {
    const dups = new Set([12]);
    expect(sprintOptionLabel({ id: 12, name: "Sprint 1", boardName: "Platform board" }, dups))
      .toBe("Sprint 1 (Platform board)");
  });

  it("shows a unique sprint's name plain", () => {
    expect(sprintOptionLabel({ id: 14, name: "Sprint 2", boardName: "Ops board" }, new Set())).toBe("Sprint 2");
  });

  it("shows a duplicate plain when it carries no board name to disambiguate with", () => {
    const dups = new Set([12]);
    expect(sprintOptionLabel({ id: 12, name: "Sprint 1" }, dups)).toBe("Sprint 1");
  });
});
