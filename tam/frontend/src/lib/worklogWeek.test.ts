import { describe, it, expect } from "vitest";
import type { LoggedWork } from "../api";
import { addDays, cellEntries, dayTotals, gaps, isWeekend, nextStart, seconds, weekOf } from "./worklogWeek";

function entry(key: string, started: string, secs: number): LoggedWork {
  return { key, summary: "", log: { id: "", author: "", authorName: "", started, timeSpent: "", seconds: secs, comment: "" } };
}

describe("worklogWeek", () => {
  it("builds the week from its Monday, whichever day it is given", () => {
    const week = ["2026-11-16", "2026-11-17", "2026-11-18", "2026-11-19", "2026-11-20", "2026-11-21", "2026-11-22"];
    expect(weekOf("2026-11-18")).toEqual(week);
    expect(weekOf("2026-11-16")).toEqual(week);
    expect(weekOf("2026-11-22")).toEqual(week);
  });

  it("steps across a month and a year end", () => {
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
    expect(addDays("2026-03-01", -1)).toBe("2026-02-28");
    expect(isWeekend("2026-11-21")).toBe(true);
    expect(isWeekend("2026-11-20")).toBe(false);
  });

  it("files an entry under the day its own stamp reads, whatever the offset", () => {
    const entries = [
      entry("PLAT-1", "2026-11-17T00:30:00.000+0700", 3600),
      entry("PLAT-1", "2026-11-17T23:30:00.000-0800", 1800),
      entry("PLAT-2", "2026-11-17T10:00:00.000+0700", 7200),
    ];
    expect(dayTotals(entries).get("2026-11-17")).toBe(12600);
    expect(seconds(cellEntries(entries, "PLAT-1", "2026-11-17"))).toBe(5400);
    expect(cellEntries(entries, "PLAT-2", "2026-11-18")).toEqual([]);
  });

  it("lists the short workdays behind today, newest first, without today or the weekend", () => {
    const entries = [
      entry("PLAT-1", "2026-11-16T09:00:00.000+0700", 8 * 3600),
      entry("PLAT-1", "2026-11-17T09:00:00.000+0700", 5.5 * 3600),
    ];
    // Today is Wed 18 Nov. Seven days back reaches Wed 11 Nov.
    expect(gaps(entries, "2026-11-18", 8 * 3600, 7)).toEqual([
      { day: "2026-11-17", logged: 5.5 * 3600 },
      { day: "2026-11-13", logged: 0 },
      { day: "2026-11-12", logged: 0 },
      { day: "2026-11-11", logged: 0 },
    ]);
  });

  it("offers a start where the day's last entry ended, and nine on an empty day", () => {
    expect(nextStart([], "2026-11-17")).toBe("2026-11-17T09:00");
    const entries = [
      entry("PLAT-1", "2026-11-17T09:00:00.000+0700", 3 * 3600),
      entry("PLAT-2", "2026-11-17T14:00:00.000+0700", 1800),
    ];
    expect(nextStart(entries, "2026-11-17")).toBe("2026-11-17T14:30");
    // A long entry late in the day would end tomorrow; the offer stays today.
    expect(nextStart([entry("PLAT-1", "2026-11-17T20:00:00.000+0700", 6 * 3600)], "2026-11-17")).toBe("2026-11-17T23:59");
  });
});
