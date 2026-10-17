import type { ReportBar, ReportPerson, ReportSection, SprintReport } from "../api";
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

// The section's notes: three short sentences, one per figure a reader might
// misread.
export function peopleLoggedLine(): string {
  return "Logged counts each person's own worklogs in the sprint; a card row counts everyone's.";
}

export function peopleInProgressLine(): string {
  return "In progress is time a card sat In Progress, credited to its current assignee.";
}

export function peopleEstimateLine(): string {
  return "Actual is all time logged on a card, before the sprint too.";
}

// BAR_HEADING names the actual-against-estimate column, on screen and in the
// outputs.
export const BAR_HEADING = "Actual against estimate";

// outputBar is a person's bar as the outputs take it, or null with no
// estimate to measure against.
function outputBar(p: ReportPerson): ReportBar | null {
  return p.estimateSeconds > 0 ? { done: p.spentSeconds, target: p.estimateSeconds } : null;
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
    table: {
      columns: table.columns,
      rows: table.rows.map((r) => r.cells),
      barHeading: BAR_HEADING,
      bars: people.map(outputBar),
    },
    notes: [peopleLoggedLine(), peopleInProgressLine(), peopleEstimateLine()],
    images: [],
  }];
}
