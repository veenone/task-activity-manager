import { ProgressBar } from "@agile-suite/core";
import type { SprintDetail } from "../api";
import { plural, points, progressText, sprintRelative } from "../lib/format";

// UNREAD is what a closed sprint says before the backfill has reached it.
// A bar at zero would say the sprint held nothing, which is a different
// claim, and one the row would have no way to take back.
const UNREAD = "cards not read yet";

// NO_TIMELINE is the row with no calendar to draw: the board's own
// unassigned work, and a sprint Jira has not scheduled.
const NO_TIMELINE = "no timeline";

// scopeLine is the one line under the bars saying what the sprint holds.
// A sprint with cards in it says how many are done and how many points have
// landed, which is also the points bar's caption; a sprint with none, and a
// row with no calendar at all, says the bare count, because there is no "of
// 14" in the sentence above to carry it.
function scopeLine(detail: SprintDetail): string {
  if (detail.state === "closed" && !detail.membershipCached) return UNREAD;
  if (detail.total > 0) return progressText(detail.done, detail.total, detail.donePoints, detail.points);
  return plural(detail.total, "card", "cards");
}

// sprintTimelineText is everything this cell says, as one sentence, for the
// row's accessible name. A treeitem with an explicit label is announced by
// that label alone, so a reader arrowing the tree never reaches the bars or
// the timing unless they are in it.
export function sprintTimelineText(detail: SprintDetail): string {
  const timing = sprintRelative(detail);
  return [timing.label || NO_TIMELINE, timing.trailing, scopeLine(detail)].filter(Boolean).join(", ");
}

// SprintTimeline owns the row's whole timeline and progress cell: where the
// sprint is in its calendar, how much calendar is left, and how much of its
// work has landed, as two lines of text and up to two bars.
//
// It takes no now. sprintRelative's own defaulted parameter covers it, the
// way dayOfSprint and formatWhen are already called throughout this
// frontend, and a test pins the clock with fake timers instead of a prop
// being drilled from the view down through the list and the row.
export function SprintTimeline({ detail }: { detail: SprintDetail }) {
  const timing = sprintRelative(detail);
  // -1 is a sprint with no readable range, which is the board's own
  // unassigned node and any sprint Jira has not scheduled. There is no
  // calendar to draw, and a bar at zero would invent one; the count is what
  // the row has left to say.
  const drawable = timing.elapsed >= 0;
  const closed = detail.state === "closed";
  const unread = closed && !detail.membershipCached;
  const planned = detail.state === "future";
  return (
    <span className="sprint-timeline">
      <span className="sprint-timeline-head">
        <span className="sprint-timeline-label">{drawable ? timing.label : NO_TIMELINE}</span>
        {timing.trailing && <span className="sprint-timeline-trailing muted small">{timing.trailing}</span>}
      </span>
      {drawable && (
        <ProgressBar
          value={Math.round(timing.elapsed * 100)}
          max={100}
          label={`Time elapsed in ${detail.name}`}
          valueText={timing.label}
          // Today's line, and only where there is a today inside the range:
          // a finished sprint has none, and a sprint that has not started
          // has nothing to mark on an empty bar.
          marker={detail.state === "active" ? timing.elapsed : undefined}
          tone={closed ? "muted" : "time"}
        />
      )}
      <span className="sprint-timeline-scope muted small">{scopeLine(detail)}</span>
      {drawable && !unread && detail.points > 0 && (
        <ProgressBar
          value={detail.donePoints}
          max={detail.points}
          label={`Points done in ${detail.name}`}
          valueText={planned
            ? `0 of ${points(detail.points)} pts planned`
            : `${points(detail.donePoints)} of ${points(detail.points)} pts`}
          tone="points"
        />
      )}
    </span>
  );
}
