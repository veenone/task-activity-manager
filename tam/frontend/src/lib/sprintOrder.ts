import type { Sprint } from "../api";

// closedNewestFirst is the sprint picker's order, and it mirrors
// reports.VelocitySprints in Go: closed sprints by actual completion date
// (planned end when completion is absent), newest first,
// with the higher id breaking a tie. That is the rule a sprint id of 0
// resolves through, so the first row of this list is the sprint the view
// opens on, and the control never points at one sprint while the numbers
// below it describe another.
//
// A sprint whose start date will not parse is dropped, because Go drops it
// too: it parses both dates and skips a sprint that fails either. Sorting
// on the end date alone let a sprint with a readable end and an unreadable
// start sit at the top of this list while a sprint id of 0 resolved past
// it, so the picker and the numbers disagreed about which sprint was the
// newest one. Nothing was ever mis-titled, since the heading follows the
// response, but the two should agree without that backstop.
//
// A sprint whose end date will not parse sinks to the bottom instead of
// being dropped. It cannot be the first row from there, so it costs the
// agreement above nothing, and picking it is answered with the
// sprintHasNoDates state, which says more than leaving it out would.
export function closedNewestFirst(sprints: Sprint[]): Sprint[] {
  const readable = (d: string) => !Number.isNaN(new Date(d).getTime());
  const ended = (s: Sprint) => {
    const t = new Date(s.completeDate?.trim() ? s.completeDate : s.endDate).getTime();
    return Number.isNaN(t) ? -Infinity : t;
  };
  return sprints
    .filter((s) => s.state === "closed" && readable(s.startDate))
    .sort((a, b) => (ended(b) === ended(a) ? b.id - a.id : ended(b) - ended(a)));
}
