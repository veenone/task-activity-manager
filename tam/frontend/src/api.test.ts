import { describe, it, expect } from "vitest";
import { isDemoUrl } from "./api";

describe("isDemoUrl", () => {
  it("matches demo, demo: and demo- forms, case-insensitively", () => {
    expect(isDemoUrl("demo")).toBe(true);
    expect(isDemoUrl(" DEMO ")).toBe(true);
    expect(isDemoUrl("demo:pkcs")).toBe(true);
    expect(isDemoUrl("demo-agile")).toBe(true);
  });
  it("rejects live URLs, blanks, and the Kiwi demo", () => {
    expect(isDemoUrl("https://jira.acme.example")).toBe(false);
    expect(isDemoUrl("")).toBe(false);
    expect(isDemoUrl(undefined)).toBe(false);
    expect(isDemoUrl("kiwi-demo")).toBe(false);
  });
});
