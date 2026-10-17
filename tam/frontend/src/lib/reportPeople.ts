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

// TimeFigures is what a person row and a card row both carry.
export interface TimeFigures {
  estimateSeconds: number;
  loggedSeconds: number;
  spentSeconds: number;
  inProgressSeconds: number;
  remainingSeconds: number;
}

// TimeColumn is one time column of a people table: its heading and the
// figure it shows.
export interface TimeColumn {
  label: string;
  seconds: (t: TimeFigures) => number;
}

const ESTIMATE: TimeColumn = { label: "Estimate", seconds: (t) => t.estimateSeconds };
const LOGGED: TimeColumn = { label: "Logged in sprint", seconds: (t) => t.loggedSeconds };
const ACTUAL: TimeColumn = { label: "Actual", seconds: (t) => t.spentSeconds };
const IN_PROGRESS: TimeColumn = { label: "In progress", seconds: (t) => t.inProgressSeconds };
const REMAINING: TimeColumn = { label: "Remaining", seconds: (t) => t.remainingSeconds };

// The People section's time columns, in the order a reader follows them:
// what was planned, what the sprint spent, what was spent in all, how long
// the cards were worked, and what is left. The board allocation has no
// changelog behind it, so it shows only the three the cache can answer.
export const PEOPLE_TIME = [ESTIMATE, LOGGED, ACTUAL, IN_PROGRESS, REMAINING];
export const ALLOCATION_TIME = [ESTIMATE, ACTUAL, REMAINING];

// timeCells is a person's or a card's time columns, worded.
export function timeCells(t: TimeFigures, columns: TimeColumn[] = PEOPLE_TIME): string[] {
  return columns.map((c) => workDuration(c.seconds(t)));
}

// peopleTable is one row per person, in the order the backend sorted them.
// A blank username or email stays blank: Jira hides email on some instances,
// and the unassigned row has neither.
export function peopleTable(people: ReportPerson[], caption = "People", time: TimeColumn[] = PEOPLE_TIME): TableSpec {
  return {
    caption,
    columns: ["Person", "Username", "Email", "Cards", "Points", ...time.map((c) => c.label)],
    rows: people.map((p) => ({
      key: personKey(p),
      cells: [p.name || "Unassigned", p.username, p.email, String(p.cards), points(p.points), ...timeCells(p, time)],
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

// The board allocation's notes, short for the same reason the People notes
// are.
export function allocationScopeLine(): string {
  return "Covers every open card on the board, in any sprint or the backlog.";
}

export function allocationSyncLine(): string {
  return "Figures are from the last sync.";
}

export function allocationEmptyLine(): string {
  return "No card on this board is open.";
}

// allocationSection is the board's open work per assignee (#288). Unlike
// peopleSection it is always one section once the rows were read: a board
// with nothing open says so, since an absent section would read as one TAM
// never looked at.
export function allocationSection(people: ReportPerson[]): ReportSection[] {
  const heading = "Board allocation";
  if (people.length === 0) {
    return [{ heading, lines: [allocationEmptyLine()], table: { columns: [], rows: [] }, notes: [], images: [] }];
  }
  const table = peopleTable(people, heading, ALLOCATION_TIME);
  return [{
    heading,
    lines: [],
    table: {
      columns: table.columns,
      rows: table.rows.map((r) => r.cells),
      barHeading: BAR_HEADING,
      bars: people.map(outputBar),
    },
    notes: [allocationScopeLine(), allocationSyncLine()],
    images: [],
  }];
}
