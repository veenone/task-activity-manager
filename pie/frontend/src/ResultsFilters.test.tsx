import { expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { query } from "../wailsjs/go/models";
import { ResultsTable } from "./ResultsTable";

function issue(key: string, status: string, priority: string) {
  const project = key.split("-")[0];
  return {
    key,
    project,
    projectName: `${project} name`,
    type: "Bug",
    summary: `Summary of ${key}`,
    status,
    priority,
    updated: "2026-10-02T00:00:00.000+0000",
    matched: true,
  } as unknown as query.Issue;
}

const issues = [
  issue("A-1", "Open", "High"),
  issue("A-2", "Closed", "High"),
  issue("B-1", "Open", "Low"),
  issue("B-2", "Open", "High"),
];

const keys = () =>
  within(screen.getByRole("table", { name: "Query results" }))
    .getAllByRole("row")
    .slice(1)
    .map((r) => (r as HTMLTableRowElement).cells[1].textContent);

it("narrows the rows by project, status and priority together", async () => {
  const user = userEvent.setup();
  render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={async () => {}} />);
  expect(within(screen.getByLabelText("Project")).getAllByRole("option").map((o) => o.textContent)).toEqual([
    "All",
    "A name (A)",
    "B name (B)",
  ]);
  await user.selectOptions(screen.getByLabelText("Status"), "Open");
  expect(keys()).toEqual(["A-1", "B-1", "B-2"]);
  await user.selectOptions(screen.getByLabelText("Priority"), "High");
  expect(keys()).toEqual(["A-1", "B-2"]);
  await user.selectOptions(screen.getByLabelText("Project"), "B");
  expect(keys()).toEqual(["B-2"]);
  expect(screen.getByText("Showing 1 of 4 issues")).toBeVisible();
});

it("starts again at the first page when a filter changes", async () => {
  const user = userEvent.setup();
  const many = Array.from({ length: 60 }, (_, i) => issue(`A-${i + 1}`, i % 3 ? "Open" : "Closed", "High"));
  render(<ResultsTable issues={many} caseKeys={new Set()} onOpenCases={async () => {}} />);
  await user.click(screen.getByRole("button", { name: "Next page" }));
  await user.selectOptions(screen.getByLabelText("Status"), "Open");
  expect(screen.getByText("1-25 of 40")).toBeVisible();
  expect(screen.getByRole("status")).toHaveTextContent("Showing 40 of 60 issues");
});

it("drops a filter value the next run does not hold", async () => {
  const user = userEvent.setup();
  const { rerender } = render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={async () => {}} />);
  await user.selectOptions(screen.getByLabelText("Status"), "Closed");
  expect(keys()).toEqual(["A-2"]);
  rerender(<ResultsTable issues={issues.filter((i) => i.status === "Open")} caseKeys={new Set()} onOpenCases={async () => {}} />);
  expect(screen.getByLabelText("Status")).toHaveValue("");
  expect(keys()).toEqual(["A-1", "B-1", "B-2"]);
  expect(screen.queryByText(/^Showing/)).not.toBeInTheDocument();
});
