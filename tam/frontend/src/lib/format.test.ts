import { describe, it, expect } from "vitest";
import { formatWhen } from "./format";

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
