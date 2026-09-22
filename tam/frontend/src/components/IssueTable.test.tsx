import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import type { Issue } from "../api";
import { IssueTable } from "./IssueTable";
import { leadDepth, leadSlotText } from "../test/rowLead";

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "",
    reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null,
    rank: "", created: "", updated: "",
    ...over,
  };
}

const STORY = issue({ key: "PLAT-412", type: "story", summary: "Apply promo code at payment" });
const CHILD = issue({ key: "PLAT-500", type: "subtask", parentKey: "PLAT-412", summary: "Wire the promo input" });
const LONER = issue({ key: "PLAT-409", summary: "Rotate the gateway keys" });
// The backlog is paged, so a family can be split across two pages and the
// second page holds a subtask with nothing above it to hang off.
const STRAY = issue({ key: "PLAT-501", type: "subtask", parentKey: "PLAT-901", summary: "Retire the old gateway" });

function renderTable(issues: Issue[]) {
  render(
    <IssueTable
      issues={issues}
      selectedKey=""
      onSelect={vi.fn()}
      sort=""
      desc={false}
      onSort={vi.fn()}
    />,
  );
  return (name: RegExp) => screen.getByRole("row", { name });
}

describe("IssueTable", () => {
  it("starts every row of a family from one lead", () => {
    const rowOf = renderTable([STORY, CHILD, LONER]);
    const parent = rowOf(/^PLAT-412 /);
    const loner = rowOf(/^PLAT-409 /);
    const child = rowOf(/^PLAT-500 /);
    // A row that expands and a row that cannot both reserve the toggle's
    // place, so their summaries start level with each other.
    expect(leadSlotText(parent)).toBe("▾ 1");
    expect(leadSlotText(loner)).toBe("");
    expect(leadDepth(loner)).toBe(leadDepth(parent));
    // And the child starts to the right of both. It used to start to the
    // left of its own parent, because the toggle was wider than the indent.
    expect(leadDepth(child)).toBeGreaterThan(leadDepth(parent));
  });

  it("states a row's depth in the hierarchy the arrow keys already walk", () => {
    const rowOf = renderTable([STORY, CHILD, LONER]);
    expect(rowOf(/^PLAT-412 /)).toHaveAttribute("aria-level", "1");
    expect(rowOf(/^PLAT-412 /)).toHaveAttribute("aria-expanded", "true");
    expect(rowOf(/^PLAT-500 /)).toHaveAttribute("aria-level", "2");
    // A row with nothing under it is not collapsed, it is a leaf.
    expect(rowOf(/^PLAT-409 /)).not.toHaveAttribute("aria-expanded");
  });

  it("marks a subtask whose parent is not on this page", () => {
    const rowOf = renderTable([STORY, CHILD, STRAY]);
    const stray = rowOf(/^PLAT-501 /);
    expect(leadDepth(stray)).toBe(leadDepth(rowOf(/^PLAT-500 /)));
    expect(within(stray).getByText("Parent not shown")).toBeInTheDocument();
    expect(stray).toHaveAttribute("aria-level", "2");
  });
});
