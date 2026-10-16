import type { ReportPerson, ReportSection, SprintReport } from "../api";
import { points, workDuration } from "./format";
import type { TableSpec } from "./reportTables";

// reportPeople is the sprint report's People section (#287): who carried the
// sprint, what they logged, and how long their cards sat in progress.
//
// Its table and its sentences live together here rather than in
// lib/reportTables and lib/reportText, which are both near the size limit
// (C2). The rule those two exist for still holds: every surface that draws
// this section, the Reports view and the three outputs, reads it from
// peopleSection, so it is worded in one place.

// peopleTable is one row per person, in the order the backend sorted them.
// A blank username or email stays blank: Jira hides email on some instances,
// and the unassigned row has neither.
export function peopleTable(people: ReportPerson[]): TableSpec {
  return {
    caption: "People",
    columns: ["Person", "Username", "Email", "Cards", "Points", "Estimate", "Remaining", "Logged in sprint", "In progress"],
    rows: people.map((p) => ({
      key: p.username || p.name || "unassigned",
      cells: [
        p.name || "Unassigned",
        p.username,
        p.email,
        String(p.cards),
        points(p.points),
        workDuration(p.estimateSeconds),
        workDuration(p.remainingSeconds),
        workDuration(p.loggedSeconds),
        workDuration(p.inProgressSeconds),
      ],
    })),
  };
}

// peopleLoggedLine says what "Logged in sprint" counts: who logged the time,
// not who holds the card, which is why somebody with no card can have a row.
export function peopleLoggedLine(): string {
  return "Logged in sprint counts worklogs by their author between the sprint's start and its end, sub-tasks included. Someone who logged time on another person's card has a row of their own.";
}

// peopleInProgressLine says what "In progress" counts and whom it charges.
// TAM's changelog read keeps no assignee changes, so a card that changed
// hands is charged to its current holder, and the reader is told so.
export function peopleInProgressLine(): string {
  return "In progress is the time each card spent in a status Jira files under its In Progress category, inside the sprint, charged to whoever holds the card now.";
}

// peopleEstimateLine says the estimates are each card's own, so a parent and
// its sub-tasks in the same sprint are not counted twice.
export function peopleEstimateLine(): string {
  return "Estimate and Remaining add up each card's own figures as they stand now.";
}

// peopleSection is a list of nothing or one, the way capacitySection is: a
// series stored before people were counted carries none, and a section with
// an empty table would claim the sprint had nobody in it.
export function peopleSection(report: SprintReport): ReportSection[] {
  const people = report.series.people ?? [];
  if (people.length === 0) return [];
  const table = peopleTable(people);
  return [{
    heading: table.caption,
    lines: [],
    table: { columns: table.columns, rows: table.rows.map((r) => r.cells) },
    notes: [peopleLoggedLine(), peopleInProgressLine(), peopleEstimateLine()],
    images: [],
  }];
}
