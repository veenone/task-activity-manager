import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { VelocityRow } from "../../api";
import { VelocityChart } from "./VelocityChart";

function row(over: Partial<VelocityRow>): VelocityRow {
  return {
    sprintId: 1, sprintName: "Sprint 1", unit: "points", unitReason: "", committed: 30, completed: 28, truncated: false,
    ...over,
  };
}

const ROWS = [
  row({ sprintId: 10, sprintName: "Sprint 10", committed: 30, completed: 28 }),
  row({ sprintId: 11, sprintName: "Sprint 11", committed: 34, completed: 29 }),
  row({ sprintId: 12, sprintName: "Sprint 12", committed: 20, completed: 20 }),
];

describe("VelocityChart", () => {
  it("draws a committed and a completed bar per sprint with the amount beside each", () => {
    const { container } = render(<VelocityChart rows={ROWS} />);
    expect(container.querySelectorAll(".chart-bar")).toHaveLength(6);
    const figure = screen.getByRole("figure", { name: "Velocity" });
    expect(within(figure).getAllByText("34").length).toBeGreaterThan(0);
    expect(within(figure).getAllByText("29").length).toBeGreaterThan(0);
    const rows = within(screen.getByRole("table", { name: /Velocity/ })).getAllByRole("row");
    expect(rows).toHaveLength(4);
    expect(rows[2]).toHaveTextContent("Sprint 11");
    expect(rows[2]).toHaveTextContent("34 points");
    expect(rows[2]).toHaveTextContent("29 points");
  });

  it("names the chart and legend entries so colour is not the only carrier", () => {
    render(<VelocityChart rows={ROWS} />);
    expect(screen.getByRole("img", { name: "Velocity" })).toBeInTheDocument();
    const legend = screen.getByRole("list", { name: /legend/i });
    expect(legend).toHaveTextContent("Committed");
    expect(legend).toHaveTextContent("Completed");
  });

  it("splits sprints estimated in different units into one chart per unit and says why", () => {
    const mixed = [ROWS[0], row({ sprintId: 11, sprintName: "Sprint 11", unit: "cards", committed: 12, completed: 9 }), ROWS[2]];
    const { container } = render(<VelocityChart rows={mixed} />);
    expect(screen.getByRole("img", { name: "Velocity in points" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Velocity in cards" })).toBeInTheDocument();
    expect(container.querySelectorAll(".chart-bar")).toHaveLength(6);
    expect(screen.getByText(/different units/)).toBeInTheDocument();
    const cards = within(screen.getByRole("table", { name: /cards/ })).getAllByRole("row");
    expect(cards).toHaveLength(2);
    expect(cards[1]).toHaveTextContent("12 cards");
  });

  it("draws one closed sprint and says there is no trend yet", () => {
    const { container } = render(<VelocityChart rows={[ROWS[0]]} />);
    expect(container.querySelectorAll(".chart-bar")).toHaveLength(2);
    expect(screen.getByText(/no trend/)).toBeInTheDocument();
  });

  it("draws nothing for no rows, leaving the table's sentence to say so", () => {
    const { container } = render(<VelocityChart rows={[]} />);
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(container.textContent).toBe("");
  });

  it("reads a sprint's pair on focus and moves along the sprints with the arrows", async () => {
    const user = userEvent.setup();
    render(<VelocityChart rows={ROWS} />);
    await user.tab();
    expect(screen.getByRole("status")).toHaveTextContent("Sprint 10: committed 30 points, completed 28.");
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("status")).toHaveTextContent("Sprint 11: committed 34 points, completed 29.");
  });
});
