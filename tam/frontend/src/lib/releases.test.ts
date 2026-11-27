import { describe, expect, it } from "vitest";
import type { Issue, PendingChange, VersionSummary } from "../api";
import { defaultCarryOver, filterReleases, releaseDates, releaseStatus, releaseWaiting, sortIssues, unfinishedIssues } from "./releases";

function version(over: Partial<VersionSummary>): VersionSummary {
  return {
    id: 1, project: "PLAT", name: "v", description: "", startDate: "", releaseDate: "",
    released: false, archived: false, draft: false, done: 0, total: 0, ...over,
  };
}

const OPEN = version({ id: 10, name: "v2.4", releaseDate: "2026-12-15" });
const LATER = version({ id: 11, name: "v2.5", releaseDate: "2027-01-15" });
const UNDATED = version({ id: 12, name: "v3.0" });
const SHIPPED = version({ id: 9, name: "v2.3", released: true, releaseDate: "2026-11-15" });
const OLD = version({ id: 8, name: "v1.0", released: true, archived: true });
const ALL = [SHIPPED, OPEN, LATER, UNDATED, OLD];

function row(over: Partial<PendingChange>): PendingChange {
  return { id: 1, entityType: "", entityKey: "", field: "", beforeVal: "", afterVal: "", baseVersion: "", createdAt: "", ...over };
}

describe("filterReleases", () => {
  it("keeps each status to its own filter", () => {
    expect(filterReleases(ALL, "unreleased").map((v) => v.name)).toEqual(["v2.4", "v2.5", "v3.0"]);
    expect(filterReleases(ALL, "released").map((v) => v.name)).toEqual(["v2.3"]);
    expect(filterReleases(ALL, "archived").map((v) => v.name)).toEqual(["v1.0"]);
    expect(filterReleases(ALL, "all")).toHaveLength(5);
  });
});

describe("releaseStatus", () => {
  it("names a draft, an archived, a released and an open release", () => {
    expect(releaseStatus(version({ draft: true }))).toEqual({ label: "Draft", tone: "draft" });
    expect(releaseStatus(OLD).label).toBe("Archived");
    expect(releaseStatus(SHIPPED).label).toBe("Released");
    expect(releaseStatus(OPEN)).toEqual({ label: "Unreleased", tone: "future" });
  });
});

describe("releaseDates", () => {
  it("prints what dates there are", () => {
    expect(releaseDates(version({ startDate: "2026-12-01", releaseDate: "2026-12-15" }))).toBe("2026-12-01 to 2026-12-15");
    expect(releaseDates(OPEN)).toBe("Release 2026-12-15");
    expect(releaseDates(version({ startDate: "2026-12-01" }))).toBe("Starts 2026-12-01");
    expect(releaseDates(UNDATED)).toBe("No dates");
  });
});

describe("unfinishedIssues", () => {
  it("reads the status category first, then the name", () => {
    const issues = [
      { key: "A-1", status: "Shipped", statusCategory: "done" },
      { key: "A-2", status: "Done", statusCategory: "indeterminate" },
      { key: "A-3", status: "Closed" },
      { key: "A-4", status: "To Do" },
    ] as Issue[];
    expect(unfinishedIssues(issues).map((i) => i.key)).toEqual(["A-2", "A-4"]);
  });
});

describe("defaultCarryOver", () => {
  it("picks the next open release by date, not the one being released", () => {
    expect(defaultCarryOver(ALL, OPEN.id)).toBe(LATER.id);
    expect(defaultCarryOver(ALL, LATER.id)).toBe(UNDATED.id);
  });
  it("falls back to an earlier open release, and to none", () => {
    expect(defaultCarryOver([OPEN, LATER], LATER.id)).toBe(OPEN.id);
    expect(defaultCarryOver([OPEN, SHIPPED], OPEN.id)).toBe(0);
  });
});

describe("releaseWaiting", () => {
  it("names what Commit will do to each release", () => {
    const got = releaseWaiting([
      row({ entityType: "version_release", entityKey: "10" }),
      row({ entityType: "version_archive", entityKey: "9", afterVal: JSON.stringify({ name: "v2.3", archived: true }) }),
      row({ entityType: "version_archive", entityKey: "8", afterVal: JSON.stringify({ name: "v1.0", archived: false }) }),
      row({ entityType: "version_delete", entityKey: "12" }),
      row({ entityType: "version_edit", entityKey: "11" }),
      row({ entityType: "sprint_delete", entityKey: "13" }),
    ]);
    expect(Object.fromEntries(got)).toEqual({
      10: "Releasing on Commit",
      9: "Archiving on Commit",
      8: "Unarchiving on Commit",
      12: "Deleting on Commit",
      11: "Edited, not committed",
    });
  });
});

describe("sortIssues", () => {
  const rows = [
    { key: "PLAT-10", summary: "b", status: "To Do", assignee: "", storyPoints: 3 },
    { key: "PLAT-9", summary: "a", status: "Done", assignee: "Lee", storyPoints: null },
  ] as unknown as Issue[];
  it("keeps the store's order unsorted and sorts keys by number", () => {
    expect(sortIssues(rows, "", false).map((i) => i.key)).toEqual(["PLAT-10", "PLAT-9"]);
    expect(sortIssues(rows, "key", false).map((i) => i.key)).toEqual(["PLAT-9", "PLAT-10"]);
    expect(sortIssues(rows, "summary", true).map((i) => i.key)).toEqual(["PLAT-10", "PLAT-9"]);
    expect(sortIssues(rows, "storyPoints", true).map((i) => i.key)).toEqual(["PLAT-10", "PLAT-9"]);
  });
});
