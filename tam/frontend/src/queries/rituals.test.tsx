import { describe, expect, it } from "vitest";
import { encodeRitualIssues, parseRitualIssues } from "../api";

describe("ritual issues", () => {
  it("round-trips keys and remarks in order", () => {
    const issues = [
      { key: "PLAT-14", remark: "demoed" },
      { key: "PLAT-22", remark: "blocked on infra" },
    ];
    expect(parseRitualIssues(encodeRitualIssues(issues))).toEqual(issues);
  });

  it("reads an empty column as no issues", () => {
    expect(parseRitualIssues("")).toEqual([]);
    expect(parseRitualIssues("[]")).toEqual([]);
  });

  it("returns no issues rather than throwing on malformed stored data", () => {
    expect(parseRitualIssues("{not json")).toEqual([]);
  });
});
