import type { ReportPerson, ReportSection, SprintReport } from "../api";
import { points, workDuration } from "./format";
import { timeBar } from "./issueTime";
import type { TimeBar } from "./issueTime";
import type { TableSpec } from "./reportTables";

// reportPeople is the sprint report's People section (#287): who carried the
// sprint, what they logged, and how long their cards sat in progress.
//
// Its table and its sentences live together here rather than in
// lib/reportTables and lib/reportText, which are both near the size limit
// (C2). The rule those two exist for still holds: the People tab and the
// three outputs all read the section from here, so it is worded once.

// The time columns, in the order a reader follows them: what was planned,
// what the sprint spent, what was spent in all, how long the cards were
// being worked, and what is left.
export const PEOPLE_COLUMNS = ["Estimate", "Logged in sprint", "Actual", "In progress", "Remaining"];

// timeCells is a person's or a card's time columns, in PEOPLE_COLUMNS order.
export function timeCells(t: {
  estimateSeconds: number;
  loggedSeconds: number;
  spentSeconds: number;
  inProgressSeconds: number;
  remainingSeconds: number;
}): string[] {
  return [t.estimateSeconds, t.loggedSeconds, t.spentSeconds, t.inProgressSeconds, t.remainingSeconds].map(workDuration);
}

// peopleTable is one row per person, in the order the backend sorted them.
// A blank username or email stays blank: Jira hides email on some instances,
// and the unassigned row has neither.
export function peopleTable(people: ReportPerson[]): TableSpec {
  return {
    caption: "People",
    columns: ["Person", "Username", "Email", "Cards", "Points", ...PEOPLE_COLUMNS],
    rows: people.map((p) => ({
      key: personKey(p),
      cells: [p.name || "Unassigned", p.username, p.email, String(p.cards), points(p.points), ...timeCells(p)],
    })),
  };
}

export function personKey(p: ReportPerson): string {
  return p.username || p.name || "unassigned";
}

// personBar is the backlog's time bar for a person or a card: the actual
// against the estimate, turning over when the actual runs past it. It is
// null with no estimate, since a track with no end says nothing.
export function personBar(estimateSeconds: number, spentSeconds: number): TimeBar | null {
  return timeBar({
    estimateSeconds: estimateSeconds > 0 ? estimateSeconds : null,
    spentSeconds,
    remainingSeconds: null,
    family: false,
    ownSpentSeconds: spentSeconds,
    ownEstimateSeconds: estimateSeconds > 0 ? estimateSeconds : null,
  });
}

// The section's notes. Each answers a question a reader of the table asks.
export function peopleLoggedLine(): string {
  return "Logged in sprint is the time each person logged between the sprint's start and end, sub-tasks included. If you logged time on someone else's card, you get a row of your own.";
}

export function peopleInProgressLine(): string {
  return "In progress is how long each card sat in a status Jira counts as In Progress during the sprint. It's charged to whoever holds the card now, because TAM doesn't track earlier assignees.";
}

export function peopleEstimateLine(): string {
  return "Estimate, Actual and Remaining are each card's own figures as they stand today. Actual counts all time logged on the card, including before the sprint.";
}

// peopleCardLine is the breakdown's own note: a card's row answers for the
// card, so its logged time counts everyone who logged on it.
export function peopleCardLine(): string {
  return "In each person's card list, Logged in sprint counts everyone who logged time on that card.";
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
