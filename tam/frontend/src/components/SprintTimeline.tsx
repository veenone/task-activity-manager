import { ProgressBar } from "@agile-suite/core";
import type { SprintDetail } from "../api";
import { points, progressText, sprintRelative } from "../lib/format";

// UNREAD is what a closed sprint says before the backfill has reached it.
// Every closed sprint on every board is in this state the first time the
// board is synced, and the twelve a pass reads is a cap, not a promise, so
// this is a row people see rather than a corner case. A bar at zero would
// say the sprint held nothing, which is a different claim.
const UNREAD = "Cards not read yet";

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
  // calendar to draw, and a bar at zero would invent one.
  if (timing.elapsed < 0) return <span className="sprint-timeline muted small">no timeline</span>;

  const closed = detail.state === "closed";
  const unread = closed && !detail.membershipCached;
  const planned = detail.state === "future";
  return (
    <span className="sprint-timeline">
      <span className="sprint-timeline-head">
        <span className="sprint-timeline-label">{timing.label}</span>
        <span className="sprint-timeline-trailing muted small">{timing.trailing}</span>
      </span>
      <ProgressBar
        value={Math.round(timing.elapsed * 100)}
        max={100}
        label={`Time elapsed in ${detail.name}`}
        valueText={timing.label}
        // Today's line, and only where there is a today inside the range:
        // a finished sprint has none, and a sprint that has not started has
        // nothing to mark on an empty bar.
        marker={detail.state === "active" ? timing.elapsed : undefined}
        tone={closed ? "muted" : "time"}
      />
      <span className="sprint-timeline-scope muted small">
        {unread ? UNREAD : detail.total > 0 ? progressText(detail.done, detail.total, detail.donePoints, detail.points) : ""}
      </span>
      {!unread && detail.points > 0 && (
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
