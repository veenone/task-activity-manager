import { describe, it, expect } from "vitest";
import type { ReportPerson, SprintReport } from "../api";
import { reportDocument } from "./reportDocument";
import { peopleSection } from "./reportPeople";

const rita: ReportPerson = {
  name: "Rita Anand",
  username: "ranand",
  email: "r.anand@example.com",
  cards: 3,
  points: 8,
  estimateSeconds: 12 * 3600,
  remainingSeconds: 2 * 3600 + 30 * 60,
  loggedSeconds: 9 * 3600,
  inProgressSeconds: 26 * 3600,
};
const unassigned: ReportPerson = {
  name: "",
  username: "",
  email: "",
  cards: 1,
  points: 2,
  estimateSeconds: 0,
  remainingSeconds: 0,
  loggedSeconds: 0,
  inProgressSeconds: 0,
};

function report(people?: ReportPerson[], state: Partial<SprintReport> = {}): SprintReport {
  return {
    series: {
      sprintId: 11,
      sprintName: "Sprint 11",
      unit: "points",
      unitReason: "",
      committed: 10,
      added: 0,
      removed: 0,
      completed: 8,
      carriedOver: 2,
      days: [],
      truncated: [],
      issues: ["PLAT-1"],
      people,
    },
    velocity: [],
    builtAt: "2026-03-16T09:00:00Z",
    unavailable: "",
    ...state,
  };
}

describe("the People section", () => {
  it("is a row per person in the order the backend sorted them, Unassigned named as such", () => {
    const [section] = peopleSection(report([rita, unassigned]));
    expect(section.heading).toBe("People");
    expect(section.table.columns).toEqual([
      "Person",
      "Username",
      "Email",
      "Cards",
      "Points",
      "Estimate",
      "Remaining",
      "Logged in sprint",
      "In progress",
    ]);
    expect(section.table.rows).toEqual([
      ["Rita Anand", "ranand", "r.anand@example.com", "3", "8", "12h", "2h 30m", "9h", "26h"],
      ["Unassigned", "", "", "1", "2", "0m", "0m", "0m", "0m"],
    ]);
  });

  it("says how logged and in-progress time are counted, and who in-progress time is charged to", () => {
    const [section] = peopleSection(report([rita]));
    expect(section.notes.join(" ")).toMatch(/worklogs? .*author/i);
    expect(section.notes.join(" ")).toMatch(/In Progress category/);
    expect(section.notes.join(" ")).toMatch(/whoever holds the card now/);
  });

  it("is left out for a report stored before people were counted", () => {
    expect(peopleSection(report(undefined))).toEqual([]);
    expect(peopleSection(report([]))).toEqual([]);
  });

  it("rides in the published document, so the page, the spreadsheet and the deck carry it", () => {
    const doc = reportDocument(report([rita]));
    expect(doc?.sections.map((s) => s.heading)).toContain("People");
  });
});
