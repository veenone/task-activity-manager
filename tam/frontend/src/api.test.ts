import { describe, it, expect, vi, beforeEach } from "vitest";
import * as bindings from "../wailsjs/go/main/App";
import { GetSprintReport, isDemoUrl } from "./api";

// Only the report binding is stood in for. The rest of the generated module
// is the real thing, so this file still exercises api.ts as it ships.
vi.mock("../wailsjs/go/main/App", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../wailsjs/go/main/App")>()),
  GetSprintReport: vi.fn(),
}));

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

describe("GetSprintReport's sprint id guard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("refuses a NaN sprint id before the call is made", async () => {
    // Wails marshals with JSON.stringify, so NaN would have reached Go as 0
    // and been answered with the newest closed sprint's report under
    // whatever heading the caller happened to be showing.
    await expect(GetSprintReport("p1", 1, Number.NaN, false)).rejects.toThrow(/zero or more/);
    expect(bindings.GetSprintReport).not.toHaveBeenCalled();
  });

  it("refuses a negative sprint id before the call is made", async () => {
    await expect(GetSprintReport("p1", 1, -3, false)).rejects.toThrow(/zero or more/);
    expect(bindings.GetSprintReport).not.toHaveBeenCalled();
  });

  it("refuses a sprint id that is not a whole number", async () => {
    await expect(GetSprintReport("p1", 1, 1.5, false)).rejects.toThrow(/zero or more/);
    expect(bindings.GetSprintReport).not.toHaveBeenCalled();
  });

  it("passes zero through, since it asks for the board's most recent closed sprint", async () => {
    vi.mocked(bindings.GetSprintReport).mockResolvedValue({ unavailable: "" } as never);
    await GetSprintReport("p1", 1, 0, false);
    expect(bindings.GetSprintReport).toHaveBeenCalledWith("p1", 1, 0, false);
  });

  it("passes a real sprint id and the refresh flag through unchanged", async () => {
    vi.mocked(bindings.GetSprintReport).mockResolvedValue({ unavailable: "" } as never);
    await GetSprintReport("p1", 1, 11, true);
    expect(bindings.GetSprintReport).toHaveBeenCalledWith("p1", 1, 11, true);
  });
});
