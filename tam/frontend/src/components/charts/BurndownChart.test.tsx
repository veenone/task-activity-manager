import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReportDay } from "../../api";
import { calendarDay } from "../../lib/format";
import { BurndownChart } from "./BurndownChart";

// n days of a sprint that took five points on in its third day: scope steps
// up, remaining follows it, and the guide runs straight down.
function days(n: number): ReportDay[] {
  return Array.from({ length: n }, (_, i) => ({
    date: `2026-09-${String(i + 1).padStart(2, "0")}`,
    scope: 34 + (i >= 2 ? 5 : 0),
    completed: i * 4,
    remaining: 34 + (i >= 2 ? 5 : 0) - i * 4,
    ideal: Math.max(0, 34 - i * 3.4),
  }));
}

describe("BurndownChart", () => {
  it("draws one point per day and reads the same days back as hidden table rows", () => {
    const { container } = render(<BurndownChart days={days(5)} unit="points" />);
    expect(container.querySelectorAll(".chart-point")).toHaveLength(5);
    expect(container.querySelectorAll(".chart-line")).toHaveLength(3);
    const table = screen.getByRole("table", { name: /Burndown/ });
    const rows = within(table).getAllByRole("row");
    expect(rows).toHaveLength(6);
    expect(rows[1]).toHaveTextContent(calendarDay("2026-09-01"));
    expect(rows[1]).toHaveTextContent("34");
    expect(rows[5]).toHaveTextContent("23");
  });

  it("states the last remaining and scope values in text beside the lines", () => {
    render(<BurndownChart days={days(5)} unit="points" />);
    const figure = screen.getByRole("figure");
    expect(within(figure).getAllByText("23 points").length).toBeGreaterThan(0);
    expect(within(figure).getAllByText("39 points").length).toBeGreaterThan(0);
  });

  it("names the chart for a screen reader from its caption", () => {
    render(<BurndownChart days={days(3)} unit="points" />);
    expect(screen.getByRole("img", { name: "Burndown" })).toBeInTheDocument();
    expect(screen.getByRole("figure", { name: "Burndown" })).toBeInTheDocument();
  });

  it("draws a single-day sprint as points, not lines", () => {
    const { container } = render(<BurndownChart days={days(1)} unit="cards" />);
    expect(container.querySelectorAll(".chart-point")).toHaveLength(1);
    expect(container.querySelectorAll(".chart-line")).toHaveLength(0);
    expect(within(screen.getByRole("table")).getAllByRole("row")).toHaveLength(2);
  });

  it("says when there is no day to draw and draws nothing", () => {
    render(<BurndownChart days={[]} unit="points" />);
    expect(screen.getByText("No days to draw yet.")).toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("shows the day's values on hover", async () => {
    const user = userEvent.setup();
    const { container } = render(<BurndownChart days={days(3)} unit="points" />);
    await user.hover(container.querySelectorAll(".chart-hit")[1]);
    expect(screen.getByRole("status")).toHaveTextContent(
      `${calendarDay("2026-09-02")}: 34 points in scope, 4 completed, 30 remaining`,
    );
    await user.unhover(container.querySelectorAll(".chart-hit")[1]);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("moves the tooltip along the days with Left and Right", async () => {
    const user = userEvent.setup();
    render(<BurndownChart days={days(3)} unit="points" />);
    await user.tab();
    expect(screen.getByRole("status")).toHaveTextContent(`${calendarDay("2026-09-01")}:`);
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("status")).toHaveTextContent(`${calendarDay("2026-09-02")}:`);
    await user.keyboard("{ArrowRight}");
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("status")).toHaveTextContent(`${calendarDay("2026-09-03")}:`);
    await user.keyboard("{ArrowLeft}");
    expect(screen.getByRole("status")).toHaveTextContent(`${calendarDay("2026-09-02")}:`);
  });

  it("thins the x labels of a sprint over thirty days and keeps the first one", () => {
    const long = Array.from({ length: 31 }, (_, i) => ({
      ...days(1)[0],
      date: i < 30 ? `2026-09-${String(i + 1).padStart(2, "0")}` : "2026-10-01",
    }));
    const { container } = render(<BurndownChart days={long} unit="points" />);
    const labels = container.querySelectorAll(".chart-x-label");
    expect(labels.length).toBeLessThan(31);
    expect(labels.length).toBeGreaterThan(2);
    expect(labels[0]).toHaveTextContent(calendarDay("2026-09-01"));
  });
});
