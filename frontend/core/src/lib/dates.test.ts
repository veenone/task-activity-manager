import { describe, expect, it } from "vitest";
import { formatDate, formatDateTime, formatWhen, parseJiraDate } from "./dates";

// Local-time constructors throughout, so the expectations hold in any zone.
const local = (y: number, m: number, d: number, h = 0, min = 0) => new Date(y, m - 1, d, h, min).toISOString();

describe("parseJiraDate", () => {
  it("reads a Jira Data Center offset with no colon", () => {
    expect(parseJiraDate("2026-06-09T15:18:59.928+0100").toISOString()).toBe("2026-06-09T14:18:59.928Z");
  });
});

describe("formatDateTime", () => {
  it("is day first, whatever the machine locale says", () => {
    expect(formatDateTime(local(2026, 11, 20, 14, 5))).toBe("20/11/2026 14:05");
  });

  it("marks an empty value and keeps text that is not a date", () => {
    expect(formatDateTime("")).toBe("(none)");
    expect(formatDateTime("soon")).toBe("soon");
  });
});

describe("formatDate", () => {
  it("is day first too, so a field and the status bar agree", () => {
    expect(formatDate(local(2026, 11, 20, 14, 5))).toBe("20/11/2026");
  });
});

describe("formatWhen", () => {
  const now = new Date(2026, 10, 20, 16, 0);

  it("says today with the time for a same-day stamp", () => {
    expect(formatWhen(local(2026, 11, 20, 14, 5), now)).toBe("today 14:05");
  });

  it("uses the same day-first date and time as the fields otherwise", () => {
    expect(formatWhen(local(2026, 11, 11, 9, 30), now)).toBe("11/11/2026 09:30");
  });

  it("is empty for no stamp and raw for text that is not a date", () => {
    expect(formatWhen("", now)).toBe("");
    expect(formatWhen("never", now)).toBe("never");
  });
});
