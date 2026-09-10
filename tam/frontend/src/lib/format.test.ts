import { describe, it, expect } from "vitest";
import { dayInput, dayOfSprint, formatWhen, progressText, sprintDates } from "./format";

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

describe("dayOfSprint", () => {
  const start = "2026-08-29T09:00:00Z";
  const end = "2026-09-12T09:00:00Z";

  it("counts the first day as day one", () => {
    expect(dayOfSprint(start, end, new Date("2026-08-29T10:00:00Z"))).toBe("day 1 of 14");
  });

  it("counts the days elapsed since the start", () => {
    expect(dayOfSprint(start, end, new Date("2026-09-03T10:00:00Z"))).toBe("day 6 of 14");
  });

  it("stops at the last day of a sprint running late", () => {
    // Day 19 of 14 is a fact about the sprint being late, and this line is
    // drawing a calendar.
    expect(dayOfSprint(start, end, new Date("2026-09-19T10:00:00Z"))).toBe("day 14 of 14");
  });

  it("says nothing about a sprint it cannot measure", () => {
    expect(dayOfSprint("", end)).toBe("");
    expect(dayOfSprint(start, "")).toBe("");
    expect(dayOfSprint("never", "either")).toBe("");
    expect(dayOfSprint(end, start)).toBe("");
  });
});
