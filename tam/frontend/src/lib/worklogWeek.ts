import type { LoggedWork } from "../api";

// The Worklog view counts in calendar days, written "2026-11-17". A day here
// is never an instant: an entry belongs to the day its own started stamp
// reads, which is the first ten characters of it, and the grid's columns are
// the reader's own local days.

const two = (n: number) => String(n).padStart(2, "0");

// dayOf is a Date's local calendar day.
export function dayOf(d: Date): string {
  return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}`;
}

// dateOf is the local noon of a day. Noon rather than midnight, so adding
// days across a clock change cannot land on the day before.
function dateOf(day: string): Date {
  const [y, m, d] = day.split("-").map(Number);
  return new Date(y, m - 1, d, 12);
}

export function addDays(day: string, n: number): string {
  const d = dateOf(day);
  d.setDate(d.getDate() + n);
  return dayOf(d);
}

export function isWeekend(day: string): boolean {
  const w = dateOf(day).getDay();
  return w === 0 || w === 6;
}

// weekOf is the seven days, Monday first, of the week a day falls in.
export function weekOf(day: string): string[] {
  const back = (dateOf(day).getDay() + 6) % 7;
  const monday = addDays(day, -back);
  return Array.from({ length: 7 }, (_, i) => addDays(monday, i));
}

// dayLabel is a day as the grid and the panel name it: "Tue 17 Nov".
export function dayLabel(day: string): string {
  const d = dateOf(day);
  const weekday = d.toLocaleDateString(undefined, { weekday: "short" });
  return `${weekday} ${d.toLocaleDateString(undefined, { day: "numeric", month: "short" })}`;
}

// periodLabel names a run of days by its ends: "16 Nov to 22 Nov 2026".
export function periodLabel(days: string[]): string {
  const first = dateOf(days[0]);
  const last = dateOf(days[days.length - 1]);
  const short = (d: Date) => d.toLocaleDateString(undefined, { day: "numeric", month: "short" });
  return `${short(first)} to ${short(last)} ${last.getFullYear()}`;
}

// entryDay is the day an entry was logged on.
export function entryDay(e: LoggedWork): string {
  return e.log.started.slice(0, 10);
}

// dayTotals adds the entries up by day, in seconds.
export function dayTotals(entries: LoggedWork[]): Map<string, number> {
  const out = new Map<string, number>();
  for (const e of entries) out.set(entryDay(e), (out.get(entryDay(e)) ?? 0) + (e.log.seconds || 0));
  return out;
}

// cellEntries is what was logged on one issue on one day.
export function cellEntries(entries: LoggedWork[], key: string, day: string): LoggedWork[] {
  return entries.filter((e) => e.key === key && entryDay(e) === day);
}

export function seconds(entries: LoggedWork[]): number {
  return entries.reduce((sum, e) => sum + (e.log.seconds || 0), 0);
}

export interface Gap {
  day: string;
  logged: number;
}

// gaps is the workdays before today, at most lookback days back, that came
// in under the target, newest first. Today is left out because it is not
// over, and a weekend because nobody owes it hours.
export function gaps(entries: LoggedWork[], today: string, target: number, lookback: number): Gap[] {
  const totals = dayTotals(entries);
  const out: Gap[] = [];
  for (let i = 1; i <= lookback; i++) {
    const day = addDays(today, -i);
    const logged = totals.get(day) ?? 0;
    if (!isWeekend(day) && logged < target) out.push({ day, logged });
  }
  return out;
}

// DayState is how a day stands against the target, in words and in the tone
// its bar takes. The words carry it on their own: the bar is never the only
// thing saying a day is short.
export interface DayState {
  text: string;
  tone: "complete" | "short" | "open";
}

// dayState words a workday. A day that reached the target is complete
// whenever it was. One that did not is short only once it is over: today and
// the days ahead read as a running total, because nobody is late yet.
export function dayState(logged: number, target: number, day: string, today: string, duration: (s: number) => string): DayState {
  if (logged >= target) return { text: `${duration(logged)}, complete`, tone: "complete" };
  if (day >= today) return { text: `${duration(logged)} of ${duration(target)}`, tone: "open" };
  return { text: logged > 0 ? `${duration(target - logged)} short` : "nothing logged", tone: "short" };
}

// DAY_STARTS is the hour a day's first entry is offered at.
const DAY_STARTS = 9;

// nextStart is the wall clock the form offers for a new entry on a day, as a
// date-and-time field holds one: where the day's last entry ended, or nine in
// the morning on a day with none. It stays inside the day, because an entry
// that started after midnight belongs to the next one.
export function nextStart(dayEntries: LoggedWork[], day: string): string {
  let minute = DAY_STARTS * 60;
  for (const e of dayEntries) {
    const clock = /T(\d{2}):(\d{2})/.exec(e.log.started);
    if (!clock) continue;
    const end = Number(clock[1]) * 60 + Number(clock[2]) + Math.round((e.log.seconds || 0) / 60);
    if (end > minute) minute = end;
  }
  minute = Math.min(minute, 23 * 60 + 59);
  return `${day}T${two(Math.floor(minute / 60))}:${two(minute % 60)}`;
}
