import type { ReportProgress, ReportSeries } from "../api";
import { formatWhen, points as trimPoints } from "./format";

// reportText is every sentence the sprint report puts on screen.
//
// It is a module of its own rather than string literals inside the three
// components for two reasons. The wording carries rules: committed is a
// floor and removed sees only the cards that came back, the two reasons for
// counting cards are different problems and only one of them is the user's
// to fix, and the method line is the answer to an argument in front of a
// team. Rules are worth testing without rendering anything. And Phase 5's
// Rituals publishes these same figures to Confluence, so a second surface
// is coming that must not word them a second way.
//
// The backend deliberately sends constants and not sentences, which is why
// this file exists at all: internal/sprintreport and internal/reports both
// say so in their own comments.

// unitWord is the noun a series' unit counts in, singular for one.
export function unitWord(unit: string, n: number): string {
  if (unit === "points") return n === 1 ? "point" : "points";
  if (unit === "cards") return n === 1 ? "card" : "cards";
  // A unit this version does not know is printed rather than guessed at.
  return unit;
}

// amount is a figure with its unit, the whole number trimmed of its decimal
// the way every other points figure in this frontend is.
export function amount(n: number, unit: string): string {
  return `${trimPoints(n)} ${unitWord(unit, n)}`;
}

// summarySentence is the sentence a sprint review starts with. The unit is
// named once, on the first figure, because every figure in it counts the
// same thing.
export function summarySentence(s: ReportSeries): string {
  const name = s.sprintName || "This sprint";
  return (
    `${name} committed ${amount(s.committed, s.unit)}, ` +
    `added ${trimPoints(s.added)}, removed ${trimPoints(s.removed)}, ` +
    `completed ${trimPoints(s.completed)} and carried over ${trimPoints(s.carriedOver)}.`
  );
}

// floorLine is the qualification that rides with every one of those
// figures. It is on the surface and not in a footnote because two of them
// can be low and nothing else on screen would say so.
export function floorLine(): string {
  return (
    "Committed is a floor rather than a total, and removed counts only the cards that left and came back. " +
    "A sprint's issues are read with a search for the sprint's current members, " +
    "so a card taken out while the sprint ran and left out was never fetched and leaves no trace here."
  );
}

// unitLine says why a report counts cards, and is empty for one counting
// points, where there is nothing to explain. The two reasons are different
// problems: one of them the team can fix by estimating, and the other is
// evidence about the instance that the backend is explicit about not being
// able to prove.
export function unitLine(unit: string, unitReason: string): string {
  if (unit !== "cards") return "";
  if (unitReason === "nothingEstimated") {
    return (
      "These figures count cards rather than points: story points are in evidence somewhere in this sprint's issues or their history, " +
      "and nothing inside the sprint's own window ever carried a value. " +
      "Estimating the cards would give this board a points report."
    );
  }
  if (unitReason === "noPointsFieldSeen") {
    return (
      "These figures count cards rather than points: nothing in this sprint's issues or their history mentions story points at all. " +
      "That is what an instance without a story points field looks like from here, " +
      "and it is also what a board that has one and has never used it looks like, so TAM cannot tell those two apart."
    );
  }
  return "These figures count cards rather than points.";
}

// velocityFloorLine is floorLine's rule worded for a table of sprints. The
// Committed column is a floor in every row for the same reason one sprint's
// figure is, and the table is read on its own: Phase 5's Rituals publishes
// these rows to Confluence out of this module, where there is no paragraph
// beside them to borrow the qualification from.
export function velocityFloorLine(): string {
  return (
    "Committed is a floor in every row rather than a total. A sprint's issues are read with a search for its " +
    "current members, so a card taken out while the sprint ran and left out was never fetched and is counted " +
    "in no row here."
  );
}

// methodLine is printed with the numbers rather than kept in a document,
// because TAM's figures and Jira's are computed from different sources and
// the disagreement surfaces in the middle of a review.
export function methodLine(): string {
  return (
    "How this is counted: done means a status the board's last column collects; " +
    "the history is reconstructed from Jira's public changelog rather than from Jira's own stored sprint records, " +
    "so these figures can differ from Jira's; and a removal is only visible for a card that came back."
  );
}

// MAX_NAMED_KEYS caps the keys truncationLine prints, so a sprint where
// every card came back cut short does not answer with a paragraph of keys.
const MAX_NAMED_KEYS = 8;

// nameList is a readable list of issue keys, capped at MAX_NAMED_KEYS with
// the rest counted.
export function nameList(keys: string[]): string {
  const named = keys.slice(0, MAX_NAMED_KEYS);
  const rest = keys.length - named.length;
  // The and belongs to whichever item is last in the sentence. When the
  // list is cut short that is the count, so the named keys are all
  // separated by commas and "PLAT-7 and PLAT-8 and 3 more" never happens.
  if (rest > 0) return `${named.join(", ")} and ${rest} more`;
  if (named.length < 2) return named.join("");
  return `${named.slice(0, -1).join(", ")} and ${named[named.length - 1]}`;
}

// truncationLine names the cards whose history Jira would not give in full
// and refuses to present the figures beside it as exact. It is empty when
// every changelog came back whole.
export function truncationLine(keys: string[]): string {
  if (keys.length === 0) return "";
  const cards = keys.length === 1 ? "card" : "cards";
  return (
    `Jira returned only part of the changelog for ${keys.length} ${cards} (${nameList(keys)}), ` +
    "so the figures above are not exact. Rebuilding the report reads those histories again."
  );
}

// builtAtLine stamps the sprint on screen and nothing else. The velocity
// rows carry no age of their own and most of them are read back from the
// store, so this line is careful to claim only the one sprint.
export function builtAtLine(builtAt: string): string {
  const when = formatWhen(builtAt);
  if (!when) return "";
  return `This sprint's figures were built ${when}. The velocity rows carry no stamp of their own.`;
}

// unavailableLine words the four reasons a report has nothing to show. Each
// one is a different thing to do next, which is why the backend tells them
// apart instead of answering with one empty report.
export function unavailableLine(reason: string): string {
  switch (reason) {
    case "boardNotSynced":
      return (
        "TAM cannot tell which statuses count as finished on this board, so it has nothing to build a report " +
        "from: either the board's columns are not in the cache, or its last column collects no status at all. " +
        "The Boards view's Refresh fetches the columns; a last column that collects nothing is fixed in Jira's " +
        "own board configuration."
      );
    case "sprintNotFound":
      return (
        "This board's cached sprint list does not hold that sprint. Either it was deleted in Jira, " +
        "or this view is holding an older copy of the list, which the Boards view's Refresh replaces."
      );
    case "sprintHasNoDates":
      return (
        "This sprint has no start or end date TAM can read. A report is a walk between two dates, " +
        "so this sprint cannot be reported on at all."
      );
    case "noClosedSprint":
      return (
        "This board has no closed sprint a report can be built from, so there is no report to open on and no " +
        "velocity table yet. Either it has never closed one, or every sprint it has closed carries a start or " +
        "an end date TAM cannot read."
      );
    default:
      // A reason a later backend adds is printed rather than swallowed: an
      // empty pane saying nothing is worse than an unfamiliar word.
      return `There is no report for this sprint, for a reason this version has no wording for: ${reason}.`;
  }
}

// isBusyRefusal recognises the refusal App.acquire makes in Go and the one
// SyncContext makes in front of it. Both end "is already running for this
// profile" and both name the operation holding the lock in front of it, Go
// from its own busy map and SyncContext from the operation name it carries
// beside the lock, so the phrase they share is what tells a refusal from a
// failed read. The one refusal that names nothing says "another operation",
// which this matches too.
export function isBusyRefusal(message: string): boolean {
  return / is already running for this profile/.test(message);
}

// busyLine is what a refused read says. The lock refuses rather than
// queues, so the only thing to do is wait for the other operation and ask
// again, and quoting the refusal is what says which operation that is.
export function busyLine(message: string): string {
  // The message is Go's, or SyncContext's in front of it, and both are
  // worded to sit inside an error rather than to open a paragraph.
  const opening = message.charAt(0).toUpperCase() + message.slice(1);
  return (
    `${opening}. A report takes the same per-profile lock a sync and a commit do, ` +
    "and that lock refuses rather than waits, so ask again once the other one has finished."
  );
}

// progressStage is the label for one frame of a report's progress. It
// carries no count: the status bar and the view each put the frame's own
// fetched and total beside it.
export function progressStage(p: ReportProgress): string {
  const name = p.sprintName || `sprint ${p.sprintId}`;
  if (p.phase === "sprint") return `Reading ${name}`;
  if (p.phase === "velocity") return `Reading ${name} for the velocity table`;
  return "Building the sprint report";
}
