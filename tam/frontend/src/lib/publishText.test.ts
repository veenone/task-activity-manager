import { describe, it, expect } from "vitest";
import { busyLine, isBusyRefusal, progressStage } from "./publishText";

// The publishing and exporting wording that reportText.test.ts covered
// before the two split. The sentences the outputs ribbon renders are
// asserted where they were already: publishedLine, savedLine and
// publisherStatusWord through the ribbon in ReportOutputs.test.tsx, and
// nothingToPublishLine against unavailableLine in reportDocument.test.ts.

describe("isBusyRefusal", () => {
  it("recognises the refusal the lock makes, whichever operation is holding it", () => {
    expect(isBusyRefusal("a sync is already running for this profile")).toBe(true);
    expect(isBusyRefusal("a report is already running for this profile")).toBe(true);
    expect(isBusyRefusal("a commit is already running for this profile")).toBe(true);
  });
  it("does not mistake an ordinary read failure for it", () => {
    expect(isBusyRefusal("Get \"https://jira.example/rest\": connection refused")).toBe(false);
  });
});

describe("busyLine", () => {
  it("quotes the refusal and says the lock refuses rather than queues", () => {
    const line = busyLine("a sync is already running for this profile");
    expect(line).toContain("sync is already running for this profile");
    expect(line).toContain("refuses rather than waits");
  });
  // The message is written to sit inside an error, and this is the first
  // thing the reader sees in the banner.
  it("opens the sentence with a capital rather than with Go's lowercase word", () => {
    expect(busyLine("a report is already running for this profile")).toMatch(/^A report is already running/);
  });
});

describe("progressStage", () => {
  const frame = { phase: "sprint", sprintId: 11, sprintName: "Sprint 11", fetched: 25, total: 200, done: false };
  it("names the sprint being read", () => {
    expect(progressStage(frame)).toBe("Reading Sprint 11");
  });
  it("says when the sprint being read is one the velocity table needs", () => {
    expect(progressStage({ ...frame, phase: "velocity" })).toBe("Reading Sprint 11 for the velocity table");
  });
  it("falls back to the sprint's id when the frame carries no name", () => {
    expect(progressStage({ ...frame, sprintName: "" })).toBe("Reading sprint 11");
  });
  it("says a report is being built for a phase it does not know", () => {
    expect(progressStage({ ...frame, phase: "later" })).toBe("Building the sprint report");
  });
});
