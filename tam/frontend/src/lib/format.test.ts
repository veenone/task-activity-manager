import { describe, it, expect } from "vitest";
import { calendarDay, day, dayInput, dayOfSprint, formatWhen, progressText, sprintDates, sprintRelative } from "./format";

describe("formatWhen", () => {
  const now = new Date("2026-09-05T14:00:00Z");
  it("is empty for an empty stamp", () => {
    expect(formatWhen("", now)).toBe("");
  });
  it("says today with the time for a same-day stamp", () => {
    const stamp = new Date("2026-09-05T10:42:00Z").toISOString();
    const expected = new Date(stamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    expect(formatWhen(stamp, now)).toBe(`today ${expected}`);
  });
  it("shows the date and time otherwise", () => {
    const stamp = "2026-09-01T08:00:00Z";
    const d = new Date(stamp);
    const expected = `${d.toLocaleDateString()} ${d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
    expect(formatWhen(stamp, now)).toBe(expected);
  });
  it("returns the raw text for something that is not a date", () => {
    expect(formatWhen("never", now)).toBe("never");
  });
});

describe("sprintDates", () => {
  it("says what it knows when a sprint is missing one of its dates", () => {
    // Jira leaves both empty on a future sprint nobody has scheduled, and
    // either one can be missing on a sprint made through the API.
    expect(sprintDates({ startDate: "", endDate: "2026-09-12T09:00:00Z" })).toMatch(/^ends /);
    expect(sprintDates({ startDate: "2026-08-29T09:00:00Z", endDate: "" })).toMatch(/^started /);
    expect(sprintDates({ startDate: "", endDate: "" })).toBe("");
  });

  it("joins the two days with 'to' when it has both", () => {
    expect(sprintDates({ startDate: "2026-08-29T09:00:00Z", endDate: "2026-09-12T09:00:00Z" })).toContain(" to ");
  });
});

describe("dayInput", () => {
  it("reads the day off the stamp rather than converting through a Date", () => {
    // A sprint starting at 09:00 UTC would come back as the day before for
    // a reader west of it, so an edit dialog nobody touched would move the
    // sprint by a day.
    expect(dayInput("2026-08-29T09:00:00.000+0000")).toBe("2026-08-29");
    expect(dayInput("2026-08-29")).toBe("2026-08-29");
  });

  it("is empty for a stamp that carries no day", () => {
    expect(dayInput("")).toBe("");
    expect(dayInput("never")).toBe("");
  });
});

describe("progressText", () => {
  it("reads points landed of points estimated, so one column says one thing", () => {
    expect(progressText(8, 14, 21, 34)).toBe("8 of 14 done, 21 of 34 pts");
  });

  it("drops the points half when nothing in the scope is estimated", () => {
    expect(progressText(1, 3, 0, 0)).toBe("1 of 3 done");
  });

  it("trims a float that is really a whole number", () => {
    expect(progressText(1, 2, 2.5, 5.0000001)).toBe("1 of 2 done, 2.5 of 5 pts");
  });
});

describe("day", () => {
  it("reads the civil day off the stamp, whichever side of midnight the clock is", () => {
    // day() used to put the stamp through new Date() and toLocaleDateString,
    // so a sprint starting at 09:00 UTC read as the day before for a reader
    // west of it and the row moved the sprint by a day. It reads the leading
    // date the way dayInput does now, and renders it the way calendarDay
    // does, so both ends of one civil day land on the same day name.
    const twelfth = calendarDay("2026-09-12");
    expect(day("2026-09-12T23:00:00Z")).toBe(twelfth);
    expect(day("2026-09-12T01:00:00Z")).toBe(twelfth);
    expect(day("2026-09-12")).toBe(twelfth);
  });

  it("says nothing about a stamp that carries no day", () => {
    expect(day("")).toBe("");
    expect(day("not a date")).toBe("");
  });
});

describe("dayOfSprint", () => {
  const start = "2026-08-29T09:00:00Z";
  const end = "2026-09-12T09:00:00Z";

  it("counts the first day as day one", () => {
    expect(dayOfSprint(start, end, new Date("2026-08-29T10:00:00Z"))).toBe("day 1 of 15");
  });

  it("counts both end days, so a fortnight is fifteen days and not thirteen", () => {
    // The length used to be the difference between the two dates, which
    // counted neither the first day nor the last: a 29 Aug to 12 Sep sprint
    // read "day 13 of 13" on the day it ended. The row and the view's own
    // summary line draw this on one screen, so an exclusive count here
    // would have printed two lengths for one sprint.
    expect(dayOfSprint(start, end, new Date("2026-09-03T10:00:00Z"))).toBe("day 6 of 15");
    expect(dayOfSprint(start, end, new Date("2026-09-12T10:00:00Z"))).toBe("day 15 of 15");
  });

  it("can say what day a one-day sprint is on", () => {
    expect(dayOfSprint(start, start, new Date("2026-08-29T15:00:00Z"))).toBe("day 1 of 1");
  });

  it("stops at the last day of a sprint running late", () => {
    // Day 19 of 14 is a fact about the sprint being late, and this line is
    // drawing a calendar.
    expect(dayOfSprint(start, end, new Date("2026-09-19T10:00:00Z"))).toBe("day 15 of 15");
  });

  it("says nothing about a sprint it cannot measure", () => {
    expect(dayOfSprint("", end)).toBe("");
    expect(dayOfSprint(start, "")).toBe("");
    expect(dayOfSprint("never", "either")).toBe("");
    expect(dayOfSprint(end, start)).toBe("");
  });
});

describe("sprintRelative", () => {
  const start = "2026-08-29T09:00:00Z";
  const end = "2026-09-12T09:00:00Z";
  const sprint = (over: Partial<Parameters<typeof sprintRelative>[0]> = {}) =>
    ({ startDate: start, endDate: end, state: "active", ...over });

  it("says where a running sprint is and how much calendar is left", () => {
    const t = sprintRelative(sprint(), new Date("2026-09-03T10:00:00Z"));
    expect(t.label).toBe("Day 6 of 15");
    expect(t.trailing).toBe("9 days left");
    expect(t.elapsed).toBeGreaterThan(0);
    expect(t.elapsed).toBeLessThan(1);
  });

  it("counts the first day as day one rather than day zero", () => {
    expect(sprintRelative(sprint(), new Date("2026-08-29T10:00:00Z")).label).toBe("Day 1 of 15");
  });

  it("reaches zero on the last day without going past it", () => {
    const t = sprintRelative(sprint(), new Date("2026-09-12T10:00:00Z"));
    expect(t.label).toBe("Day 15 of 15");
    expect(t.trailing).toBe("0 days left");
  });

  it("says how far past its end a sprint nobody closed has run", () => {
    const t = sprintRelative(sprint(), new Date("2026-09-14T10:00:00Z"));
    // dayOfSprint clamps to the last day for the same reason; this is the
    // line that carries the fact the clamp drops, rather than contradicting
    // it with a day 17 of 15.
    expect(t.label).toBe("2 days over");
    expect(t.elapsed).toBe(1);
  });

  it("counts a future sprint down to its start and says how long it will run", () => {
    const t = sprintRelative(sprint({ state: "future" }), new Date("2026-08-18T10:00:00Z"));
    expect(t.label).toBe("Starts in 11 days");
    expect(t.trailing).toBe("15 days");
    expect(t.elapsed).toBe(0);
  });

  it("dates a closed sprint by when it was completed, falling back to its end", () => {
    const completed = sprintRelative(
      sprint({ state: "closed", completeDate: "2026-09-11T16:00:00Z" }),
      new Date("2026-09-20T10:00:00Z"),
    );
    expect(completed.label).toBe(`Closed ${calendarDay("2026-09-11")}`);
    expect(completed.elapsed).toBe(1);
    const never = sprintRelative(sprint({ state: "closed" }), new Date("2026-09-20T10:00:00Z"));
    expect(never.label).toBe(`Closed ${calendarDay("2026-09-12")}`);
  });

  it("can draw a sprint that runs for one day", () => {
    const t = sprintRelative({ startDate: start, endDate: start, state: "active" }, new Date("2026-08-29T15:00:00Z"));
    expect(t.label).toBe("Day 1 of 1");
    expect(t.trailing).toBe("0 days left");
    expect(Number.isNaN(t.elapsed)).toBe(false);
  });

  it("tells the row to draw no timeline at all when it cannot read the dates", () => {
    // -1 rather than 0: a sprint with no readable range has not started, and
    // a bar drawn at zero says it has.
    for (const s of [
      { startDate: "", endDate: end, state: "active" },
      { startDate: start, endDate: "", state: "active" },
      { startDate: "never", endDate: "either", state: "active" },
      { startDate: end, endDate: start, state: "active" },
    ]) {
      expect(sprintRelative(s, new Date("2026-09-03T10:00:00Z"))).toEqual({ label: "", trailing: "", elapsed: -1 });
    }
  });

  it("reads both ends of one civil day as the same start day", () => {
    // Constructed as instants rather than by naming a zone, so the case
    // holds whatever TZ the runner is in.
    const late = sprintRelative({ startDate: "2026-09-12T23:00:00Z", endDate: "2026-09-26T23:00:00Z", state: "future" }, new Date("2026-09-01T10:00:00Z"));
    const early = sprintRelative({ startDate: "2026-09-12T01:00:00Z", endDate: "2026-09-26T01:00:00Z", state: "future" }, new Date("2026-09-01T10:00:00Z"));
    expect(late.label).toBe(early.label);
    expect(late.trailing).toBe(early.trailing);
  });
});

describe("calendarDay", () => {
  it("prints a bare report date as the day it names, whatever zone the reader is in", () => {
    // A bare date read through new Date() is UTC midnight, which is the day
    // before for a reader west of Greenwich. The report's days are local
    // calendar days and print as the day written.
    expect(calendarDay("2026-09-12")).toBe(new Date(2026, 8, 12).toLocaleDateString(undefined, { day: "numeric", month: "short" }));
  });

  it("prints nothing for a date it cannot read", () => {
    expect(calendarDay("")).toBe("");
    expect(calendarDay("soon")).toBe("");
  });
});
