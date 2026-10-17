import { describe, it, expect } from "vitest";
import type { ReportPerson, SprintReport } from "../api";
import { kanbanDocument, reportDocument } from "./reportDocument";
import { allocationSection, peopleSection, personBar } from "./reportPeople";

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
  spentSeconds: 10 * 3600,
  issues: [],
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
  spentSeconds: 0,
  issues: [],
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
      "Logged in sprint",
      "Actual",
      "In progress",
      "Remaining",
    ]);
    expect(section.table.rows).toEqual([
      ["Rita Anand", "ranand", "r.anand@example.com", "3", "8", "12h", "9h", "10h", "26h", "2h 30m"],
      ["Unassigned", "", "", "1", "2", "0m", "0m", "0m", "0m", "0m"],
    ]);
  });

  it("says how logged and in-progress time are counted, and who in-progress time is charged to", () => {
    const [section] = peopleSection(report([rita]));
    const notes = section.notes.join(" ");
    expect(section.notes).toHaveLength(3);
    expect(notes).toMatch(/each person's own worklogs/);
    expect(notes).toMatch(/credited to its current assignee/);
    expect(notes).toMatch(/Actual is all time logged on a card/);
  });

  it("carries each person's actual against their estimate as a bar column for the outputs", () => {
    const [section] = peopleSection(report([rita, unassigned]));
    expect(section.table.barHeading).toBe("Actual against estimate");
    expect(section.table.bars).toEqual([{ done: 10 * 3600, target: 12 * 3600 }, null]);
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

describe("personBar", () => {
  it("is the backlog's bar: actual time against the estimate", () => {
    expect(personBar(12 * 3600, 9 * 3600)).toMatchObject({ value: 9 * 3600, max: 12 * 3600, tone: "progress" });
  });

  it("turns over when the actual passes the estimate", () => {
    expect(personBar(4 * 3600, 6 * 3600)).toMatchObject({ value: 4 * 3600, max: 6 * 3600, tone: "over" });
  });

  it("draws nothing with no estimate to measure against", () => {
    expect(personBar(0, 3600)).toBeNull();
  });
});

describe("the Board allocation section", () => {
  it("is a row per assignee with estimate, actual and remaining, and the bar", () => {
    const [section] = allocationSection([rita, unassigned]);
    expect(section.heading).toBe("Board allocation");
    expect(section.table.columns).toEqual(["Person", "Username", "Email", "Cards", "Points", "Estimate", "Actual", "Remaining"]);
    expect(section.table.rows[0]).toEqual(["Rita Anand", "ranand", "r.anand@example.com", "3", "8", "12h", "10h", "2h 30m"]);
    expect(section.table.bars).toEqual([{ done: 10 * 3600, target: 12 * 3600 }, null]);
    expect(section.notes.join(" ")).toMatch(/every open card on the board/);
    expect(section.notes.join(" ")).toMatch(/last sync/);
    expect(section.notes.length).toBeLessThanOrEqual(3);
  });

  it("says a board with no open cards has none rather than printing an empty table", () => {
    const [section] = allocationSection([]);
    expect(section.table.columns).toEqual([]);
    expect(section.lines).toEqual(["No card on this board is open."]);
  });

  it("rides in the sprint report and the kanban report when given", () => {
    expect(reportDocument(report([rita]), false, {}, undefined, [rita])?.sections.map((s) => s.heading)).toContain("Board allocation");
    expect(reportDocument(report([rita]))?.sections.map((s) => s.heading)).not.toContain("Board allocation");
    const columns = [{ name: "To Do", statusIds: ["1"], total: 1, points: 0, counted: 1, min: null, max: null, constraint: "issueCount" }];
    expect(kanbanDocument("Ops", columns, [rita])?.sections.map((s) => s.heading)).toEqual(["Column capacity", "Board allocation"]);
  });
});
