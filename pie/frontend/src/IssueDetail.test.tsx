import { expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { query } from "../wailsjs/go/models";
import { ResultsTable } from "./ResultsTable";
import { IssueDetail } from "./IssueDetail";

function issue(key: string, visibility: string, impact = "") {
  return {
    key,
    project: "A",
    projectName: "Alpha",
    type: "Bug",
    summary: `Summary of ${key}`,
    status: "Open",
    priority: "High",
    updated: "2026-10-02T00:00:00.000+0000",
    clientVisibility: visibility,
    clientImpact: impact,
    matched: true,
  } as unknown as query.Issue;
}

const issues = [issue("A-1", "Visible", "Seen at two sites."), issue("A-2", ""), issue("A-3", "Possible")];

it("shows client visibility as a column and a filter once the run holds it", async () => {
  const user = userEvent.setup();
  render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={async () => true} />);
  expect(screen.getByRole("columnheader", { name: "Client visibility" })).toBeVisible();
  await user.selectOptions(screen.getByLabelText("Client visibility"), "Possible");
  const rows = within(screen.getByRole("table", { name: "Query results" })).getAllByRole("row");
  expect(rows).toHaveLength(2);
  expect(rows[1]).toHaveTextContent("A-3");
});

it("leaves the column out when no issue has a value", () => {
  render(<ResultsTable issues={[issue("A-2", "")]} caseKeys={new Set()} onOpenCases={async () => true} />);
  expect(screen.queryByRole("columnheader", { name: "Client visibility" })).not.toBeInTheDocument();
  expect(screen.queryByLabelText("Client visibility")).not.toBeInTheDocument();
});

it("opens an issue's details with its client impact, and closes on Escape", async () => {
  const user = userEvent.setup();
  render(<ResultsTable issues={issues} caseKeys={new Set()} onOpenCases={async () => true} />);
  await user.click(screen.getByRole("button", { name: "A-1" }));
  const panel = screen.getByRole("complementary", { name: "A-1 details" });
  expect(panel).toHaveTextContent("Summary of A-1");
  expect(panel).toHaveTextContent("Alpha (A)");
  expect(within(panel).getByText("Seen at two sites.")).toBeVisible();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("complementary")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "A-2" }));
  expect(screen.getByRole("complementary", { name: "A-2 details" })).toHaveTextContent("No client impact recorded");
  await user.click(screen.getByRole("button", { name: "Close details" }));
  expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
});

it("leaves the details open when Escape is pressed inside a dialog", async () => {
  const user = userEvent.setup();
  const onClose = vi.fn();
  render(
    <>
      <IssueDetail issue={issues[0]} onClose={onClose} />
      <div role="dialog" aria-label="Another dialog">
        <input aria-label="Inside" />
      </div>
    </>,
  );
  await user.click(screen.getByLabelText("Inside"));
  await user.keyboard("{Escape}");
  expect(onClose).not.toHaveBeenCalled();
  await user.click(screen.getByRole("heading", { name: "A-1" }));
  await user.keyboard("{Escape}");
  expect(onClose).toHaveBeenCalledTimes(1);
});
