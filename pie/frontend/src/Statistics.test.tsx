import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { query } from "../wailsjs/go/models";
import { Statistics } from "./Statistics";

// One clock reading for every date, so two dates 100 days apart stay exactly
// that far apart however long the fixture takes to build.
const NOW = Date.now();
const ago = (days: number) => new Date(NOW - days * 86_400_000).toISOString().replace("Z", "+0000");

function issue(key: string, age: number, status: string, category: string, more: Partial<query.Issue> = {}) {
  return {
    key,
    project: key.split("-")[0],
    created: ago(age),
    resolved: "",
    status,
    statusCategory: category,
    bases: [key.startsWith("A") ? "One" : "Two"],
    matched: true,
    links: [],
    ...more,
  } as unknown as query.Issue;
}

const result = {
  total: 3,
  issues: [
    issue("A-1", 10, "Open", "new"),
    issue("A-2", 20, "Open", "new"),
    issue("B-1", 400, "Closed", "done", { resolved: ago(300) }),
  ],
  sets: [{ keys: ["A-1", "B-1"], projects: ["A", "B"], outOfStep: true }],
} as unknown as query.Result;

function headline(label: string) {
  return screen.getByText(label).closest("li");
}

describe("Statistics", () => {
  it("counts the run and narrows every number to the chosen project", async () => {
    render(<Statistics result={result} productField="Product base" />);
    expect(headline("not finished")).toHaveTextContent("2 not finished");
    expect(headline("median age, not finished")).toHaveTextContent("15 days");
    expect(headline("reach more than one product base")).toHaveTextContent("2 reach");
    expect(headline("linked sets out of step")).toHaveTextContent("1 linked");

    await userEvent.type(screen.getByLabelText("Project"), "B");
    expect(headline("not finished")).toHaveTextContent("0 not finished");
    const rows = within(screen.getByRole("table", { name: "Per project" })).getAllByRole("row");
    expect(rows.map((r) => r.textContent)).toEqual(["ProjectProduct baseIssuesNot finishedMedian age, not finished", "BTwo10-"]);
  });

  it("times each finished status", () => {
    render(<Statistics result={result} productField="" />);
    const rows = within(screen.getByRole("table", { name: "Finished, by status" })).getAllByRole("row");
    expect(rows[1]).toHaveTextContent("Closed11100 days");
  });

  it("leaves product bases out until the profile names the field", () => {
    render(<Statistics result={result} productField="" />);
    expect(screen.queryByText("reach more than one product base")).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Product base" })).not.toBeInTheDocument();
    expect(screen.getByText(/Name the product base field/)).toBeVisible();
  });

  // The field is read when a query runs, so a run from before it was set has
  // no bases to count, and a 0 would read as a measurement.
  it("asks for a new run when the field is set but the run has no bases", () => {
    const old = { ...result, issues: result.issues.map((i) => ({ ...i, bases: [] })) } as unknown as query.Result;
    render(<Statistics result={old} productField="Product base" />);
    expect(screen.queryByText("reach more than one product base")).not.toBeInTheDocument();
    expect(screen.getByText(/Run the query again to read the product base field/)).toBeVisible();
  });

  it("drops a project filter the next run does not contain", async () => {
    const { rerender } = render(<Statistics result={result} productField="" />);
    await userEvent.type(screen.getByLabelText("Project"), "B");
    const onlyA = { ...result, issues: result.issues.filter((i) => i.project === "A"), sets: [] } as unknown as query.Result;
    rerender(<Statistics result={onlyA} productField="" />);
    expect(screen.getByText("No project matches")).toBeVisible();
    expect(headline("not finished")).toHaveTextContent("2 not finished");
  });

  it("gives each chart a table of its numbers", () => {
    render(<Statistics result={result} productField="" />);
    const ages = within(screen.getByRole("table", { name: "Age of unfinished issues, days" })).getAllByRole("row");
    expect(ages[1]).toHaveTextContent("< 302");
    const reach = within(screen.getByRole("table", { name: "Projects each issue reaches" })).getAllByRole("row");
    expect(reach.slice(1).map((r) => r.textContent)).toEqual(["11", "22"]);
  });
});
