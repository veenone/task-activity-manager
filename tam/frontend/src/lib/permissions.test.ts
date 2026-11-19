import { describe, expect, it } from "vitest";
import type { PermissionView } from "../api";
import { denial, projectOf } from "./permissions";

const view: PermissionView = {
  project: "PLAT",
  byProject: { PLAT: { MANAGE_SPRINTS_PERMISSION: false, EDIT_ISSUES: true }, OPS: { EDIT_ISSUES: false } },
  names: { MANAGE_SPRINTS_PERMISSION: "Manage Sprints", EDIT_ISSUES: "Edit Issues" },
};

describe("projectOf", () => {
  it("reads the project off an issue key", () => {
    expect(projectOf("PLAT-431")).toBe("PLAT");
    expect(projectOf("A-B-7")).toBe("A-B");
    expect(projectOf("PLAT")).toBe("");
  });
});

describe("denial", () => {
  it("names the permission and the project when Jira denied it", () => {
    expect(denial(view, "MANAGE_SPRINTS_PERMISSION")).toBe(
      'Your Jira account does not have the "Manage Sprints" permission in PLAT.',
    );
  });

  it("uses the issue's own project when a key is given", () => {
    expect(denial(view, "EDIT_ISSUES", "PLAT-1")).toBe("");
    expect(denial(view, "EDIT_ISSUES", "OPS-9")).toContain("in OPS");
  });

  it("offers the action when the answer is unknown", () => {
    expect(denial(undefined, "EDIT_ISSUES")).toBe("");
    expect(denial(view, "LINK_ISSUES")).toBe("");
    expect(denial(view, "EDIT_ISSUES", "NEW-3")).toBe("");
    expect(denial(view, "EDIT_ISSUES", "TAM-NEW-1")).toBe("");
  });

  it("falls back to the key when Jira's name for it is missing", () => {
    expect(denial({ ...view, names: {} }, "MANAGE_SPRINTS_PERMISSION")).toContain('"MANAGE_SPRINTS_PERMISSION"');
  });
});
