// formatWhen renders a sync timestamp for the status bar: "today HH:MM" for
// a same-day stamp, the local date and time otherwise, "" for none, and the
// raw text when it is not a date at all.
export function formatWhen(iso: string, now: Date = new Date()): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  const sameDay =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate();
  return sameDay ? `today ${time}` : `${d.toLocaleDateString()} ${time}`;
}

// plural picks the singular or plural word for a count and prefixes it with
// the count itself, e.g. plural(1, "row", "rows") -> "1 row".
export function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

// day renders a sprint's own start or end as a calendar day. It is not a
// second wording for the sync age, which formatWhen owns above: a sprint's
// dates are days a team plans around, and "today 09:14" is neither.
export function day(iso: string): string {
  if (!iso) return "";
  // The civil day off the stamp, the way dayInput reads it, rendered the
  // way calendarDay renders one. Putting the stamp through new Date() and
  // toLocaleDateString is the bug dayInput's own comment warns about: a
  // sprint starting at 09:00 UTC read as the day before for a reader west
  // of it, and the row moved the sprint by a day. A stamp that carries no
  // leading date falls back to the old path, so day("not a date") is still
  // "".
  const bare = dayInput(iso);
  if (bare) return calendarDay(bare);
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

// calendarDay renders one of the report's bare 2006-01-02 days. It is read
// as a local day rather than through new Date(iso), which takes a bare date
// as UTC midnight and so prints the day before for a reader west of
// Greenwich; the backend bucketed these days in the local zone already.
export function calendarDay(date: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return "";
  const d = civilDay(date);
  return d ? d.toLocaleDateString(undefined, { day: "numeric", month: "short" }) : "";
}

// civilDay is the local midnight of the day a stamp names, and the one place
// in this file that turns a date into a Date. Everything about a sprint that
// is counted in days is counted from these rather than from the instants
// Jira sent: a sprint starting at 09:00 UTC and one starting at 23:00 UTC on
// the same day are the same day to the team planning around it, and reading
// them as instants made them a day apart for half the world.
function civilDay(iso: string): Date | null {
  const bare = dayInput(iso);
  if (!bare) return null;
  return new Date(Number(bare.slice(0, 4)), Number(bare.slice(5, 7)) - 1, Number(bare.slice(8, 10)));
}

// startOfDay is civilDay for a clock reading rather than a stamp.
const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate());

// sprintDates is a sprint's range in one phrase. A sprint missing one of its
// dates still says what it knows rather than nothing: Jira leaves both empty
// on a future sprint nobody has scheduled yet, and either one can be missing
// on a sprint made through the API.
export function sprintDates(s: { startDate: string; endDate: string }): string {
  const from = day(s.startDate);
  const to = day(s.endDate);
  if (from && to) return `${from} to ${to}`;
  return to ? `ends ${to}` : from ? `started ${from}` : "";
}

// dayInput renders a stored sprint date as the bare day a date input takes
// and writes. It reads the leading date off the stamp rather than converting
// through a Date: Jira sends a sprint's day with a time on it, and a sprint
// starting at 09:00 UTC would come back as the day before for a reader west
// of it, so an edit dialog nobody touched would move the sprint by a day.
export function dayInput(iso: string): string {
  const match = /^(\d{4}-\d{2}-\d{2})/.exec(iso);
  return match ? match[1] : "";
}

// points trims a float that is really a whole number, so 27 does not print
// as 27.0 and 2.5 still prints as 2.5. Go sums these as float64, so a column
// of halves can arrive a millionth of a point out.
export function points(n: number): string {
  return String(Math.round(n * 10) / 10);
}

// progressText is the one sentence both trees print in their progress
// column: "8 of 14 done, 21 of 34 pts". The Epics tree used to end it with
// the epic's whole estimate ("34 pts"), which said how big the epic was and
// nothing about how much of it had landed; with the Sprints tree beside it
// reading the other way, one column would have carried two sentences.
//
// The points half is dropped entirely when nothing in the scope is
// estimated, since "0 of 0 pts" is a fact about the team's habits rather
// than about this sprint's progress.
export function progressText(done: number, total: number, donePoints: number, allPoints: number): string {
  return `${done} of ${total} done` + (allPoints > 0 ? `, ${points(donePoints)} of ${points(allPoints)} pts` : "");
}

// MS_PER_DAY is the whole-day divisor every day count below measures with.
const MS_PER_DAY = 24 * 60 * 60 * 1000;

// days is the whole number of days from one instant to another, rounded, so
// a stamp's time of day cannot turn a fortnight into 13.6 days.
const days = (from: Date, to: Date) => Math.round((to.getTime() - from.getTime()) / MS_PER_DAY);

// sprintSpan is where a sprint is in its own calendar: which day of it today
// is, clamped to the range, and how many days it runs for, counting both end
// days. It is null for a sprint that cannot be measured, which is either date
// missing or unreadable, or a range that ends before it starts.
//
// Both dayOfSprint and sprintRelative read it, so the row and the Sprints
// view's own summary line cannot print two lengths for one sprint, which is
// what happened while the length was the bare difference between the dates.
function sprintSpan(startIso: string, endIso: string, now: Date): { day: number; length: number; over: number } | null {
  const start = civilDay(startIso);
  const end = civilDay(endIso);
  if (!start || !end) return null;
  const between = days(start, end);
  if (between < 0) return null;
  const length = between + 1;
  const today = startOfDay(now);
  const elapsed = days(start, today) + 1;
  return { day: Math.min(Math.max(elapsed, 1), length), length, over: Math.max(days(end, today), 0) };
}

// dayOfSprint answers "day 6 of 15" for a sprint that is running, and ""
// for one that cannot be measured. The day is clamped to the range, so a
// sprint running past its end date reads as its last day rather than as day
// 19 of 15, which is a fact about the sprint being late and not about the
// calendar this line is drawing; sprintRelative is what says the late part.
export function dayOfSprint(startIso: string, endIso: string, now: Date = new Date()): string {
  const span = sprintSpan(startIso, endIso, now);
  return span ? `day ${span.day} of ${span.length}` : "";
}

// SprintTiming is the whole of what a row says about where a sprint is in
// its calendar, in one call, so the wording lives in one place rather than
// once per surface that draws a sprint.
export interface SprintTiming {
  // "Day 6 of 15", "Starts in 11 days", "Closed 11 Sep", "2 days over".
  label: string;
  // "8 days left", "15 days", or "" for a row with no length to state.
  trailing: string;
  // 0 to 1, how far through its calendar the sprint is. -1 when it has no
  // readable range, which is what tells the row to draw no time bar at all:
  // a bar at zero would say the sprint has not started, and this one is
  // saying nothing is known.
  elapsed: number;
}

const NO_TIMING: SprintTiming = { label: "", trailing: "", elapsed: -1 };

// sprintRelative words one sprint's place in its calendar. It takes now as a
// defaulted parameter, the way formatWhen and dayOfSprint do, so nothing has
// to thread a clock down through the tree; a test pins the clock instead.
export function sprintRelative(
  s: { startDate: string; endDate: string; state: string; completeDate?: string },
  now: Date = new Date(),
): SprintTiming {
  const span = sprintSpan(s.startDate, s.endDate, now);
  if (!span) return NO_TIMING;
  const length = `${plural(span.length, "day", "days")}`;
  if (s.state === "closed") {
    // The day it was actually completed when Jira recorded one, which is
    // not always the day it was scheduled to end.
    return { label: `Closed ${day(s.completeDate || s.endDate)}`, trailing: length, elapsed: 1 };
  }
  if (s.state === "future") {
    const until = days(startOfDay(now), civilDay(s.startDate) as Date);
    return {
      label: until > 0 ? `Starts in ${plural(until, "day", "days")}` : "Starts today",
      trailing: length,
      elapsed: 0,
    };
  }
  if (span.over > 0) {
    return { label: `${plural(span.over, "day", "days")} over`, trailing: length, elapsed: 1 };
  }
  return {
    label: `Day ${span.day} of ${span.length}`,
    trailing: `${plural(span.length - span.day, "day", "days")} left`,
    elapsed: span.day / span.length,
  };
}
