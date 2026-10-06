import { ProgressBar } from "@agile-suite/core";
import type { Issue } from "../api";
import { workDuration } from "../lib/format";
import { hasTime, issueTime, timeBar, timeFigures } from "../lib/issueTime";
import type { IssueTime } from "../lib/issueTime";

// The detail panel's time tracking block: the bar, the three figures
// under it, and who the figures belong to.
//
// It is always on screen, including for an issue nobody has estimated.
// The three figures used to be rows in the field list that were rendered
// only when the issue carried any, so an untracked issue showed nothing
// at all and that was read as the feature being absent (#144). A block
// that says "not tracked" is a smaller cost than one that cannot be told
// apart from a missing feature.

// FAMILY_NOTE says the figures beside it are the issue and its sub-tasks
// together. The Backlog marks the same thing with a sign, because a
// column has no room for a sentence; a panel has.
const FAMILY_NOTE = "including sub-tasks";

export function TimeTracking({ issue }: { issue: Issue }) {
  const time = issueTime(issue);
  const bar = timeBar(time);
  const figures = timeFigures(time);
  return (
    <section className="detail-time" aria-label="Time tracking">
      <h4 className="detail-time-head">
        Time tracking
        {time.family && <span className="muted small"> {FAMILY_NOTE}</span>}
      </h4>
      {!hasTime(time) ? (
        <p className="muted small detail-time-empty">Not tracked in Jira.</p>
      ) : (
        <>
          {bar && <ProgressBar value={bar.value} max={bar.max} label={bar.label} valueText={bar.valueText} tone={bar.tone} />}
          {/* Every figure is in text beside the bar: colour and length
              are never the only carriers of what it says. */}
          <dl className="detail-time-figures">
            {figures.map((f) => (
              <div key={f.label}>
                <dt>{f.label}</dt>
                <dd>{f.text}</dd>
              </div>
            ))}
          </dl>
          {ownAside(time) && <p className="muted small detail-time-own">{ownAside(time)}</p>}
        </>
      )}
    </section>
  );
}

// ownAside names what belongs to the issue itself, for a parent whose
// figures are its family's. "none on the issue itself" is the common
// case and the one worth saying: it is what tells a reader the hours
// were logged by the children.
function ownAside(t: IssueTime): string {
  if (!t.family) return "";
  if (t.ownSpentSeconds === null || t.ownSpentSeconds === 0) return "None of it logged on the issue itself.";
  if (t.ownSpentSeconds === t.spentSeconds) return "";
  return `${workDuration(t.ownSpentSeconds)} of it logged on the issue itself.`;
}
