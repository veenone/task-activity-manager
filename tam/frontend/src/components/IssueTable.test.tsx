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

  it("colours the status chip from the category, not from the status name", () => {
    const rowOf = renderTable([
      issue({ key: "PLAT-412", status: "Erledigt", statusCategory: "done" }),
      issue({ key: "PLAT-409", status: "Resolved" }),
    ]);
    expect(within(rowOf(/^PLAT-412 /)).getByText("Erledigt")).toHaveClass("chip-status-done");
    // A row synced before the category was stored keeps the name guess.
    expect(within(rowOf(/^PLAT-409 /)).getByText("Resolved")).toHaveClass("chip-status-done");
  });

  it("marks a subtask whose parent is not on this page", () => {
    const rowOf = renderTable([STORY, CHILD, STRAY]);
    const stray = rowOf(/^PLAT-501 /);
    expect(leadDepth(stray)).toBe(leadDepth(rowOf(/^PLAT-500 /)));
    expect(within(stray).getByText("Parent not shown")).toBeInTheDocument();
    expect(stray).toHaveAttribute("aria-level", "2");
  });

  it("reads the time logged against the estimate, and leaves an unestimated row blank", () => {
    const rowOf = renderTable([
      issue({
        key: "PLAT-412", timeSpentSeconds: 21600, originalEstimateSeconds: 28800,
        aggregateTimeSpentSeconds: 21600, aggregateEstimateSeconds: 28800,
      }),
      issue({ key: "PLAT-409" }),
    ]);
    expect(within(rowOf(/^PLAT-412 /)).getByText("6h/8h")).toBeInTheDocument();
    // Not "0h": an issue nobody estimated has not been estimated at nothing.
    expect(within(rowOf(/^PLAT-409 /)).queryByText(/h of /)).toBeNull();
  });

  // The issue #142 was raised for: everything is estimated on the
  // sub-tasks, so the row showed nothing at all until it read as its
  // family.
  it("reads a row estimated through its sub-tasks as the family, marked", () => {
    const rowOf = renderTable([
      issue({ key: "PLAT-412", aggregateEstimateSeconds: 144000, aggregateRemainingSeconds: 100800, aggregateTimeSpentSeconds: 43200 }),
    ]);
    const row = rowOf(/^PLAT-412 /);
    expect(within(row).getByText("Σ 12h/40h")).toBeInTheDocument();
    // The bar is what makes the column readable at a glance, and it says
    // its own value: colour and length are never the only carriers.
    const bar = within(row).getByRole("progressbar");
    expect(bar).toHaveAttribute("aria-valuenow", String(43200));
    expect(bar).toHaveAttribute("aria-valuemax", String(144000));
    expect(bar).toHaveAttribute("aria-valuetext", "12h of 40h, including sub-tasks");
  });

  // Past the estimate the bar turns over: the track is what was logged
  // and the fill is the estimate inside it, so the overrun is the part
  // the fill does not reach. A bar clamped at full would have said
  // "finished" about work that went over.
  it("turns the bar over for a row that overran, and marks one logged to plan", () => {
    const rowOf = renderTable([
      issue({ key: "PLAT-412", originalEstimateSeconds: 7200, timeSpentSeconds: 21600, aggregateEstimateSeconds: 7200, aggregateTimeSpentSeconds: 21600 }),
      issue({ key: "PLAT-409", originalEstimateSeconds: 7200, timeSpentSeconds: 7200, aggregateEstimateSeconds: 7200, aggregateTimeSpentSeconds: 7200 }),
    ]);
    const over = within(rowOf(/^PLAT-412 /)).getByRole("progressbar");
    expect(over).toHaveClass("progress-bar-over");
    expect(over).toHaveAttribute("aria-valuenow", String(7200));
    expect(over).toHaveAttribute("aria-valuemax", String(21600));
    expect(over).toHaveAttribute("aria-valuetext", "6h logged against 2h estimated, over by 4h");
    expect(within(rowOf(/^PLAT-409 /)).getByRole("progressbar")).toHaveClass("progress-bar-complete");
  });

  // Nothing to measure against, so the row is marked rather than
  // measured: a stripe in its own colour, with no progressbar role
  // because there is no value for one to carry.
  it("marks a row with hours logged and no estimate instead of measuring it", () => {
    const rowOf = renderTable([issue({ key: "PLAT-412", timeSpentSeconds: 3600, aggregateTimeSpentSeconds: 3600 })]);
    const row = rowOf(/^PLAT-412 /);
    expect(within(row).getByText("1h logged")).toBeInTheDocument();
    expect(within(row).queryByRole("progressbar")).toBeNull();
    expect(row.querySelector(".progress-bar-unestimated")).not.toBeNull();
  });

  // An issue nobody has touched gets neither: there is nothing to say.
  it("leaves an untracked row with no bar and no stripe", () => {
    const rowOf = renderTable([issue({ key: "PLAT-409" })]);
    const row = rowOf(/^PLAT-409 /);
    expect(within(row).queryByRole("progressbar")).toBeNull();
    expect(row.querySelector(".progress-bar-unestimated")).toBeNull();
  });
});
