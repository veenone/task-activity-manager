import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReportSeries } from "../../api";
import { OutcomeChart } from "./OutcomeChart";

function series(over: Partial<ReportSeries> = {}): ReportSeries {
  return {
    sprintId: 11, sprintName: "Sprint 11", unit: "points", unitReason: "",
    committed: 34, added: 5, removed: 2, completed: 29, carriedOver: 10, days: [], truncated: [],
    ...over,
  };
}

describe("OutcomeChart", () => {
  it("draws five bars with each amount stated at the bar's end", () => {
    const { container } = render(<OutcomeChart series={series()} />);
    expect(container.querySelectorAll(".chart-bar")).toHaveLength(5);
    const figure = screen.getByRole("figure", { name: "Sprint outcome" });
    for (const value of ["34 points", "5 points", "2 points", "29 points", "10 points"]) {
      expect(within(figure).getAllByText(value).length).toBeGreaterThan(0);
    }
    const rows = within(screen.getByRole("table", { name: /outcome/i })).getAllByRole("row");
    expect(rows).toHaveLength(6);
    expect(rows[1]).toHaveTextContent("Committed");
    expect(rows[1]).toHaveTextContent("34 points");
    expect(rows[5]).toHaveTextContent("Carried over");
  });

  it("calls the last bar Remaining while the sprint is still running", () => {
    render(<OutcomeChart series={series()} live />);
    expect(screen.getByRole("table")).toHaveTextContent("Remaining");
    expect(screen.getByRole("table")).not.toHaveTextContent("Carried over");
  });

  it("draws zero-sized bars for a sprint with nothing in it rather than failing to scale", () => {
    const empty = series({ committed: 0, added: 0, removed: 0, completed: 0, carriedOver: 0 });
    const { container } = render(<OutcomeChart series={empty} />);
    expect(container.querySelectorAll(".chart-bar")).toHaveLength(5);
    expect(screen.getByRole("img", { name: "Sprint outcome" })).toBeInTheDocument();
  });

  it("reads a bar on focus and on hover", async () => {
    const user = userEvent.setup();
    const { container } = render(<OutcomeChart series={series({ unit: "cards" })} />);
    await user.tab();
    expect(screen.getByRole("status")).toHaveTextContent("Committed: 34 cards.");
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("status")).toHaveTextContent("Added: 5 cards.");
    await user.hover(container.querySelectorAll(".chart-hit")[3]);
    expect(screen.getByRole("status")).toHaveTextContent("Completed: 29 cards.");
  });
});
