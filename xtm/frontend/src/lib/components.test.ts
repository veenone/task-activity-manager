import { describe, it, expect } from "vitest";
import { formatComponentsValue, validateComponentName } from "./components";

describe("component helpers", () => {
  it("formats the stored form as a readable list", () => {
    expect(formatComponentsValue("\nUser Management\nAPI\n")).toBe("User Management, API");
    expect(formatComponentsValue("")).toBe("");
  });
  it("validates names", () => {
    expect(validateComponentName("User Management")).toBeNull();
    expect(validateComponentName("   ")).toBe("A component needs a name.");
    expect(validateComponentName("a".repeat(256))).toBe("A component name can be at most 255 characters.");
  });
});
