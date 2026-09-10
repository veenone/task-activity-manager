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
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

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

// MS_PER_DAY is the whole-day divisor dayOfSprint measures with.
const MS_PER_DAY = 24 * 60 * 60 * 1000;

// dayOfSprint answers "day 6 of 14" for a sprint that is running, and ""
// for one that cannot be measured: either date missing or unreadable, or a
// range that ends before it starts. The day is clamped to the range, so a
// sprint running past its end date reads as its last day rather than as day
// 19 of 14, which is a fact about the sprint being late and not about the
// calendar this line is drawing.
export function dayOfSprint(startIso: string, endIso: string, now: Date = new Date()): string {
  if (!startIso || !endIso) return "";
  const start = new Date(startIso);
  const end = new Date(endIso);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return "";
  const length = Math.round((end.getTime() - start.getTime()) / MS_PER_DAY);
  if (length < 1) return "";
  const elapsed = Math.round((now.getTime() - start.getTime()) / MS_PER_DAY) + 1;
  return `day ${Math.min(Math.max(elapsed, 1), length)} of ${length}`;
}
