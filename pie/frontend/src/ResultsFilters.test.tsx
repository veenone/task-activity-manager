import { expect, it } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
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
  render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={async () => true} />);
  fireEvent.focus(screen.getByLabelText("Project"));
  expect(within(screen.getByRole("listbox")).getAllByRole("option").map((o) => o.textContent)).toEqual(["A name (A)", "B name (B)"]);
  await user.selectOptions(screen.getByLabelText("Status"), "Open");
  expect(keys()).toEqual(["A-1", "B-1", "B-2"]);
  await user.selectOptions(screen.getByLabelText("Priority"), "High");
  expect(keys()).toEqual(["A-1", "B-2"]);
  await user.type(screen.getByLabelText("Project"), "B");
  expect(keys()).toEqual(["B-2"]);
  expect(screen.getByText("Showing 1 of 4 issues")).toBeVisible();
});

it("starts again at the first page when a filter changes", async () => {
  const user = userEvent.setup();
  const many = Array.from({ length: 60 }, (_, i) => issue(`A-${i + 1}`, i % 3 ? "Open" : "Closed", "High"));
  render(<ResultsTable issues={many} caseKeys={new Set()} onOpenCases={async () => true} />);
  await user.click(screen.getByRole("button", { name: "Next page" }));
  await user.selectOptions(screen.getByLabelText("Status"), "Open");
  expect(screen.getByText("1-25 of 40")).toBeVisible();
  expect(screen.getByText("Showing 40 of 60 issues")).toBeVisible();
});

it("drops a filter value the next run does not hold", async () => {
  const user = userEvent.setup();
  const { rerender } = render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={async () => true} />);
  await user.selectOptions(screen.getByLabelText("Status"), "Closed");
  expect(keys()).toEqual(["A-2"]);
  rerender(<ResultsTable issues={issues.filter((i) => i.status === "Open")} caseKeys={new Set()} onOpenCases={async () => true} />);
  expect(screen.getByLabelText("Status")).toHaveValue("");
  expect(keys()).toEqual(["A-1", "B-1", "B-2"]);
  expect(screen.queryByText(/^Showing/)).not.toBeInTheDocument();
});

it("sorts the rows by a clicked header, across pages, and reverses on a second click", async () => {
  render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={async () => true} />);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Sort by Status" }));
  expect(keys()).toEqual(["A-2", "A-1", "B-1", "B-2"]);
  await user.click(screen.getByRole("button", { name: "Sort by Key" }));
  await user.click(screen.getByRole("button", { name: "Sort by Key" }));
  expect(keys()).toEqual(["B-2", "B-1", "A-2", "A-1"]);
  expect(screen.getByRole("columnheader", { name: /Key/ })).toHaveAttribute("aria-sort", "descending");
});

it("searches the key, summary and project name as you type", async () => {
  render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={async () => true} />);
  const user = userEvent.setup();
  const search = screen.getByRole("searchbox", { name: "Search" });
  await user.type(search, "b name");
  expect(keys()).toEqual(["B-1", "B-2"]);
  await user.clear(search);
  await user.type(search, "a-2");
  expect(keys()).toEqual(["A-2"]);
  expect(screen.getByText("Showing 1 of 4 issues")).toBeVisible();
});
