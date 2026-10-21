// The sprint report: its series, the document built from it, and the exports.

import * as App from "../../wailsjs/go/main/App";
import { reportout } from "../../wailsjs/go/models";
import type { ColumnView } from "./boards";

// The sprint report, as internal/sprintreport hands it over. Every number
// is a float64 in Go, because a board counting story points routinely
// carries halves.

export interface ReportDay {
  // The local calendar date, bucketed in the zone the backend built the
  // series in, as 2006-01-02.
  date: string;
  scope: number;
  completed: number;
  remaining: number;
  ideal: number;
}

export interface ReportSeries {
  sprintId: number;
  sprintName: string;
  // "points" or "cards". unitReason says why when it is cards, and is one
  // of REPORT_UNIT_REASONS; it is empty for points.
  unit: string;
  unitReason: string;
  committed: number;
  added: number;
  removed: number;
  completed: number;
  carriedOver: number;
  // The day by day line behind the totals, which the burndown draws.
  days: ReportDay[];
  // The same walk counted in hours: the cards' estimates as the scope and
  // the work logged against them as the burn. Empty for a sprint where no
  // card carries an estimate in time, which is what tells the view to draw
  // no such chart rather than a flat line at zero. Optional so fixtures
  // written before it still type-check.
  timeDays?: ReportDay[];
  // The keys of the issues whose changelog came back cut short, so the
  // numbers above rest in part on a partial history.
  truncated: string[];
  // The keys of every card the walk read, which is every card the sprint
  // held when the search ran. Anything counted per card, the report's done
  // agreement section being the first, counts against these rather than
  // asking the cache who is in the sprint now, so a report published for a
  // review says the same thing a month later. A card dragged out of the
  // sprint and left out is not among them, the blind spot removed carries.
  issues: string[];
  // One row per person (#287), sorted by display name with the unassigned
  // row last. Absent on a series stored before people were counted, and
  // the report draws no People section for one.
  people?: ReportPerson[];
}

// ReportPerson is reports.Person: what was assigned to somebody in the
// sprint and the time they spent on it. name is "" on the unassigned row.
export interface ReportPerson {
  name: string;
  username: string;
  email: string;
  cards: number;
  points: number;
  estimateSeconds: number;
  remainingSeconds: number;
  loggedSeconds: number;
  inProgressSeconds: number;
  // Every hour logged on the person's cards so far, by anyone: the actual
  // an estimate is measured against.
  spentSeconds: number;
  // The cards behind cards, for the People tab's breakdown.
  issues: ReportPersonCard[];
}

// ReportPersonCard is reports.PersonCard: one card in a person's breakdown.
// loggedSeconds counts every author inside the sprint.
export interface ReportPersonCard {
  key: string;
  summary: string;
  status: string;
  statusCategory: string;
  points: number | null;
  estimateSeconds: number;
  remainingSeconds: number;
  spentSeconds: number;
  loggedSeconds: number;
  inProgressSeconds: number;
}

export interface VelocityRow {
  sprintId: number;
  sprintName: string;
  // The unit rides on the row and not on the table, because a board that
  // moved from points to cards mid-history holds two different quantities.
  unit: string;
  unitReason: string;
  committed: number;
  completed: number;
  truncated: boolean;
}

export interface SprintReport {
  series: ReportSeries;
  velocity: VelocityRow[];
  // When the series was reconstructed, in RFC 3339, and the chosen sprint's
  // stamp only. The velocity rows carry no age of their own and most of
  // them are usually served from the store, so nothing here describes how
  // old the table is.
  builtAt: string;
  // capacity is the board's column heads: the cards each column holds
  // against the WIP limit Jira sets on it, the same numbers the Boards view
  // draws. It describes the board now rather than the sprint's history, since
  // a limit is a fact about a column at this moment. Optional because a
  // report read back from an older build carries none.
  capacity?: ColumnView[];
  // One of REPORT_UNAVAILABLE_REASONS when there is no report, and empty
  // otherwise. When it is set, series and velocity are empty.
  unavailable: string;
}

// The four reasons a report has nothing to show, mirroring the constants in
// internal/sprintreport. Each is a property of the data rather than a
// failure of the call, which is why they travel in the answer.
export const REPORT_UNAVAILABLE_REASONS = [
  "boardNotSynced",
  "sprintNotFound",
  "sprintHasNoDates",
  "noClosedSprint",
] as const;

// The two reasons a series counts cards, mirroring internal/reports.
export const REPORT_UNIT_REASONS = ["nothingEstimated", "noPointsFieldSeen"] as const;

// REPORT_PROGRESS_EVENT carries sprintreport.Progress while a report runs.
// It is not the sync's event: the shell's banner reads that one, and a
// report announcing itself there would say a sync was running.
export const REPORT_PROGRESS_EVENT = "tam:report-progress";

export interface ReportProgress {
  // "sprint" for the sprint the user asked for, "velocity" for one of the
  // older sprints the table needs and the store did not hold.
  phase: string;
  sprintId: number;
  sprintName: string;
  // Issues of this one sprint, not of the whole report.
  fetched: number;
  total: number;
  // The last frame of one sprint's fetch, not of the report.
  done: boolean;
}

// GetSprintReport is one sprint's reconstructed series and its board's
// velocity table together, in the one call that takes the per-profile lock
// once. sprintId of 0 asks for the board's most recent closed sprint, and
// refresh rebuilds from Jira instead of from the stored series, for the
// whole table rather than only the sprint on screen. It takes Go's
// per-profile lock, so a caller reaches it through SyncContext the way the
// sprint writes do rather than calling it directly.
//
// The sprint id is checked here rather than in a component so every future
// caller inherits the check. Wails marshals arguments with JSON.stringify,
// which turns both undefined and NaN into null, and Go decodes that as 0:
// an uninitialised picker would otherwise be served the newest closed
// sprint's report under whatever heading the view happened to be showing.
export const GetSprintReport = async (
  profileId: string,
  boardId: number,
  sprintId: number,
  refresh: boolean,
): Promise<SprintReport> => {
  if (!Number.isInteger(sprintId) || sprintId < 0) {
    throw new Error(`a sprint report needs a sprint id that is zero or more, not ${String(sprintId)}`);
  }
  return App.GetSprintReport(profileId, boardId, sprintId, refresh) as Promise<SprintReport>;
};
// CancelSprintReport stops whatever report this profile has running, so a
// fetch of six sprints of changelog does not hold the lock against the next
// sync for a screen nobody is looking at. It does nothing when none is
// running, which is why the view can call it on every unmount.
export const CancelSprintReport: (profileId: string) => Promise<void> = App.CancelSprintReport;

// The report as something other than a screen. A ReportDocument is headings,
// sentences, tables and the caveats on them, built by lib/reportDocument out
// of lib/reportText and lib/reportTables, so the Confluence page, the
// spreadsheet and the deck all read the report's one vocabulary and Go words
// none of it. notes is the caveats, and no renderer may drop them.
export interface ReportTable {
  columns: string[];
  rows: string[][];
  // An optional last column of progress bars, one per row (null for a row
  // with none): the spreadsheet draws Excel data bars, the page and the deck
  // draw them as text. reportout.Bar on the Go side.
  barHeading?: string;
  bars?: (ReportBar | null)[];
}

export interface ReportBar {
  done: number;
  target: number;
}

// A section's chart as a picture: the file name it travels under inside the
// output, the description a reader who cannot see it is given, and the PNG's
// bytes as base64, which is how they cross the binding. A Go []byte would
// arrive here as an array of numbers.
//
// The frontend rasterises it, because the chart's colours are CSS custom
// properties that only a browser resolves. Plural on a section because the
// velocity chart splits into one panel per unit.
export interface ReportImage {
  name: string;
  alt: string;
  data: string;
}

export interface ReportSection {
  heading: string;
  lines: string[];
  table: ReportTable;
  notes: string[];
  images: ReportImage[];
}

export interface ReportDocument {
  title: string;
  sections: ReportSection[];
}

// PublishedPage is the Confluence page a publish wrote, so the view can say
// which one it was. warning is empty unless the page was written and something
// on it was not: a chart Confluence refused as an attachment. The page is
// still the outcome, so that is not an error.
export interface PublishedPage {
  title: string;
  pageId: string;
  warning: string;
}

// PublishSprintReport writes the report to its own page under the sprint's
// Confluence page, through the transport the rituals sync uses. It is a
// write, so it happens when the user asks for it and never on mount, and it
// takes Go's per-profile lock under "report". It reaches no Jira.
export const PublishSprintReport = (
  profileId: string,
  boardId: number,
  sprintId: number,
  doc: ReportDocument,
): Promise<PublishedPage> =>
  App.PublishSprintReport(profileId, boardId, sprintId, reportout.Document.createFrom(doc)) as Promise<PublishedPage>;

// The two file exports. Each opens a save dialog and answers with the path it
// wrote, or "" when the dialog was cancelled, the convention ExportProfile
// set. A cancelled export is not a failure and has nothing to say.
export const ExportSprintReportXLSX = (doc: ReportDocument): Promise<string> =>
  App.ExportSprintReportXLSX(reportout.Document.createFrom(doc));
export const ExportSprintReportPPTX = (doc: ReportDocument): Promise<string> =>
  App.ExportSprintReportPPTX(reportout.Document.createFrom(doc));
