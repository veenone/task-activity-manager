import { describe, expect, it } from "vitest";
import { statusClass } from "./statusClass";

describe("statusClass", () => {
  it("colours from Jira's category, whatever the instance calls the status", () => {
    expect(statusClass("Zu erledigen", "new")).toBe("todo");
    expect(statusClass("En cours", "indeterminate")).toBe("active");
    expect(statusClass("Erledigt", "done")).toBe("done");
    // Which is the point: these three names guess as grey.
    expect(statusClass("Erledigt")).toBe("todo");
  });

  it("falls back to the name when no category is stored", () => {
    // A row synced before schema 19, or one whose category Jira answered
    // with a key this app does not model.
    expect(statusClass("In Progress")).toBe("active");
    expect(statusClass("In Review", "")).toBe("active");
    expect(statusClass("Resolved")).toBe("done");
    expect(statusClass("To Do")).toBe("todo");
  });
});
