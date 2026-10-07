import type { Issue } from "../api";
import { workDuration } from "./format";

// Which of an issue's two sets of time figures a reader is shown.
//
// Jira keeps an issue's own estimate, remaining and spent apart from its
// family's, and puts the family's in the aggregate trio. An issue
// estimated through its sub-tasks answers null for its own three and
// carries everything in the other set, which is why reading only its own
// showed nothing at all for a parent (#142).
//
// So a row reads as its family when it has one, the way Jira's own time
// tracking panel does, and says which set it is showing. A leaf carries
// the same values in both, so it reads as itself and is marked as
// nothing.
//
// This is the same rule as backend.Issue.Time in Go, which the export
// uses. The two are pinned against the same cases in their own tests, so
// either drifting fails where it is written.

// FAMILY_MARK is what says a figure is the family's. It is a prefix
// rather than a word so the Time column stays a column; the cell's title
// spells it out, and the panel names it in full.
export const FAMILY_MARK = "Σ";

export interface IssueTime {
  estimateSeconds: number | null;
  remainingSeconds: number | null;
  spentSeconds: number | null;
  // family says the three above are the issue and its sub-tasks
  // together rather than the issue alone.
  family: boolean;
  // The issue's own, kept beside the family's so the panel can say what
  // belongs to the issue itself. null on a parent whose every hour was
  // logged on a child.
  ownSpentSeconds: number | null;
  ownEstimateSeconds: number | null;
}

const value = (v?: number | null): number | null => (v === undefined ? null : v);

// differs says the family carries a figure the issue's own does not
// match. A missing family value says nothing: that is an instance or a
// row cached before the aggregates were stored, not a family of no work.
function differs(own: number | null, family: number | null): boolean {
  if (family === null) return false;
  return own === null || own !== family;
}

export function issueTime(issue: Issue): IssueTime {
  const ownEstimate = value(issue.originalEstimateSeconds);
  const ownRemaining = value(issue.remainingEstimateSeconds);
  const ownSpent = value(issue.timeSpentSeconds);
  const famEstimate = value(issue.aggregateEstimateSeconds);
  const famRemaining = value(issue.aggregateRemainingSeconds);
  const famSpent = value(issue.aggregateTimeSpentSeconds);
  const family =
    differs(ownEstimate, famEstimate) ||
    differs(ownRemaining, famRemaining) ||
    differs(ownSpent, famSpent);
  return {
    estimateSeconds: family ? famEstimate ?? ownEstimate : ownEstimate,
    remainingSeconds: family ? famRemaining ?? ownRemaining : ownRemaining,
    spentSeconds: family ? famSpent ?? ownSpent : ownSpent,
    family,
    ownSpentSeconds: ownSpent,
    ownEstimateSeconds: ownEstimate,
  };
}

// hasTime says there is anything to show at all. An issue nobody has
// estimated or logged against shows nothing, not a zero.
export function hasTime(t: IssueTime): boolean {
  return t.estimateSeconds !== null || t.remainingSeconds !== null || t.spentSeconds !== null;
}

// timeCell is the Backlog's Time column: logged against estimated, with
// the mark in front when the figures are the family's.
export function timeCell(issue: Issue): string {
  const t = issueTime(issue);
  if (!hasTime(t)) return "";
  // Compact: the bar beside it carries the proportion, so the cell
  // needs the two numbers and not a sentence. The sentence, remaining
  // included, is in timeCellTitle.
  const body =
    t.spentSeconds !== null && t.estimateSeconds !== null
      ? `${workDuration(t.spentSeconds)}/${workDuration(t.estimateSeconds)}`
      : t.estimateSeconds !== null
        ? `${workDuration(t.estimateSeconds)} est`
        : `${workDuration(t.spentSeconds as number)} logged`;
  return t.family ? `${FAMILY_MARK} ${body}` : body;
}

// timeCellTitle spells out what the cell is showing, since the mark on
// its own is not an explanation.
export function timeCellTitle(issue: Issue): string {
  const t = issueTime(issue);
  if (!hasTime(t)) return "";
  const parts = [
    t.estimateSeconds !== null ? `${workDuration(t.estimateSeconds)} estimated` : "",
    t.remainingSeconds !== null ? `${workDuration(t.remainingSeconds)} remaining` : "",
    t.spentSeconds !== null ? `${workDuration(t.spentSeconds)} logged` : "",
  ].filter((p) => p !== "");
  return t.family ? `${parts.join(", ")}, including sub-tasks` : parts.join(", ");
}

// TimeBar is what the Backlog cell and the panel draw their bar from:
// what has been logged against what was estimated, plus the two strings
// a progressbar needs to say what it means where no stylesheet applies.
export interface TimeBar {
  value: number;
  max: number;
  label: string;
  valueText: string;
  // Which state the bar is in, which is also what colours it: under way,
  // logged exactly to the estimate, or past it.
  tone: "progress" | "complete" | "over";
}

// timeBar is the bar for one issue's figures, or null when there is
// nothing to draw one against.
//
// An estimate is what gives the track its scale, so a row with hours
// logged and nothing estimated gets no bar: a track with no end would be
// a shape saying something it does not know. An estimate with nothing
// logged does get one, empty, because "planned and untouched" is worth
// seeing. Work that overran fills the track and no further; the figures
// beside it are what say it went over, since a bar cannot.
export function timeBar(t: IssueTime): TimeBar | null {
  if (t.estimateSeconds === null || t.estimateSeconds <= 0) return null;
  const estimate = t.estimateSeconds;
  const logged = t.spentSeconds ?? 0;
  const family = t.family ? ", including sub-tasks" : "";
  const label = t.family
    ? "Time logged against the estimate, including sub-tasks"
    : "Time logged against the estimate";
  // Past the estimate the bar turns over. The track becomes what was
  // actually logged and the fill becomes the estimate inside it, so the
  // overrun is the part of the track the fill does not reach: a bar
  // clamped at full would have said "finished" about work that went
  // over, which is the opposite of what happened.
  if (logged > estimate) {
    return {
      value: estimate,
      max: logged,
      label,
      tone: "over",
      valueText: `${workDuration(logged)} logged against ${workDuration(estimate)} estimated, over by ${workDuration(logged - estimate)}${family}`,
    };
  }
  return {
    value: logged,
    max: estimate,
    label,
    tone: logged === estimate ? "complete" : "progress",
    valueText: `${workDuration(logged)} of ${workDuration(estimate)}${family}`,
  };
}

// timeUnestimated says the row has work logged against no estimate.
//
// There is no target, so there is no proportion and timeBar answers
// nothing: a track with no end would be a shape claiming something it
// does not know. The row is still worth marking rather than leaving
// blank, because somebody is spending time on work nobody sized, and
// that is a different fact from an untouched row. What marks it is a
// stripe rather than a bar, in its own colour, and the figure beside it
// is what says how long.
export function timeUnestimated(t: IssueTime): boolean {
  return t.estimateSeconds === null && (t.spentSeconds ?? 0) > 0;
}

// timeFigures is the three the panel lists under the bar, in the order
// a reader reads them, with the ones an issue does not carry left out.
export function timeFigures(t: IssueTime): { label: string; text: string }[] {
  const out: { label: string; text: string }[] = [];
  if (t.estimateSeconds !== null) out.push({ label: "Estimated", text: workDuration(t.estimateSeconds) });
  if (t.remainingSeconds !== null) out.push({ label: "Remaining", text: workDuration(t.remainingSeconds) });
  if (t.spentSeconds !== null) out.push({ label: "Logged", text: workDuration(t.spentSeconds) });
  return out;
}
