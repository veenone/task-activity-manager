import { describe, expect, it } from "vitest";
import type { query } from "../wailsjs/go/models";
import { median, statistics } from "./stats";

const now = Date.UTC(2026, 10, 6);

function issue(key: string, created: string, status: string, category: string, bases: string[], more: Partial<query.Issue> = {}) {
  return {
    key,
    project: key.split("-")[0],
    created: `${created}T00:00:00.000+0000`,
    resolved: "",
    status,
    statusCategory: category,
    bases,
    matched: true,
    links: [],
    ...more,
  } as unknown as query.Issue;
}

// Counted by hand: A-1 and B-1 form one set, C-1 and the unmatched D-9
// another, and both are out of step. B-1 shares A-1's base, so only C-1's set
// reaches two bases.
const result = {
  total: 5,
  issues: [
    issue("A-1", "2026-11-01", "Open", "new", ["X"]),
    issue("B-1", "2025-11-06", "Closed", "done", ["X"], { resolved: "2026-01-05T00:00:00.000+0000" }),
    issue("A-2", "2026-08-03", "Postponed", "indeterminate", ["X"]),
    issue("A-3", "2024-11-06", "Corrected", "done", ["X"]),
    issue("C-1", "2026-10-07", "Rejected", "done", ["Z"], { resolved: "2026-10-17T00:00:00.000+0000" }),
    issue("D-9", "2020-01-01", "Open", "new", ["W"], { matched: false }),
  ],
  sets: [
    { keys: ["A-1", "B-1"], projects: ["A", "B"], outOfStep: true },
    { keys: ["C-1", "D-9"], projects: ["C", "D"], outOfStep: true },
  ],
} as unknown as query.Result;

describe("statistics", () => {
  it("counts the matched issues and their reach", () => {
    const s = statistics(result, { project: "", withinDays: 0 }, now);
    expect([s.issues, s.open, s.medianAge, s.multiProject, s.multiBase, s.outOfStep]).toEqual([5, 2, 50, 3, 1, 2]);
    expect(s.ages.map((b) => b.value)).toEqual([1, 0, 1, 0, 0]);
    expect(s.reach).toEqual([
      { label: "1", value: 2 },
      { label: "2", value: 3 },
    ]);
  });

  it("gives every month from the oldest issue to the newest, empty ones too", () => {
    const { created } = statistics(result, { project: "", withinDays: 0 }, now);
    expect(created).toHaveLength(25);
    expect(created[0]).toEqual({ label: "2024-11", value: 1 });
    expect(created[24]).toEqual({ label: "2026-11", value: 1 });
    expect(created.reduce((n, b) => n + b.value, 0)).toBe(5);
  });

  it("lists each project, leaving out issues reached only through a link", () => {
    expect(statistics(result, { project: "", withinDays: 0 }, now).projects).toEqual([
      { project: "A", bases: ["X"], issues: 3, open: 2, medianAge: 50 },
      { project: "B", bases: ["X"], issues: 1, open: 0, medianAge: null },
      { project: "C", bases: ["Z"], issues: 1, open: 0, medianAge: null },
    ]);
  });

  it("times each finished status, and never times one with no resolution date", () => {
    expect(statistics(result, { project: "", withinDays: 0 }, now).finished).toEqual([
      { status: "Closed", issues: 1, dated: 1, medianDays: 60 },
      { status: "Corrected", issues: 1, dated: 0, medianDays: null },
      { status: "Rejected", issues: 1, dated: 1, medianDays: 10 },
    ]);
  });

  it.each([
    ["one project", { project: "A", withinDays: 0 }, [3, 2, 1, 1]],
    ["the last 100 days", { project: "", withinDays: 100 }, [3, 2, 2, 2]],
  ])("filters every number to %s", (_, filter, want) => {
    const s = statistics(result, filter, now);
    expect([s.issues, s.open, s.multiProject, s.outOfStep]).toEqual(want);
  });
});

it("keeps an issue a few hours past the range out of it", () => {
  const late = { ...result, issues: [issue("A-1", "2026-08-07", "Open", "new", [])] } as unknown as query.Result;
  // 2026-08-07 is 91 days before now; an hour later it is still not within 91.
  expect(statistics(late, { project: "", withinDays: 91 }, now).issues).toBe(1);
  expect(statistics(late, { project: "", withinDays: 91 }, now + 3_600_000).issues).toBe(0);
});

describe("median", () => {
  it.each([
    [[], null],
    [[7], 7],
    [[9, 1, 5], 5],
    [[1, 4], 3],
  ])("of %j is %s", (xs, want) => {
    expect(median(xs)).toBe(want);
  });
});
