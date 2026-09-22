import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { SprintDetail } from "../api";
import { calendarDay } from "../lib/format";
import { SprintTimeline } from "./SprintTimeline";

// The clock is pinned rather than threaded through the component: the
// formatters take now as a defaulted parameter, the way formatWhen and
// dayOfSprint already do, so nothing has to drill a prop down the tree.
// toFake: ["Date"] is not decoration, faking every timer breaks userEvent.
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-09-03T10:00:00Z"));
});
afterEach(() => {
  vi.useRealTimers();
});

function detail(over: Partial<SprintDetail> = {}): SprintDetail {
  return {
    id: 12, boardId: 1, name: "Sprint 12", state: "active",
    startDate: "2026-08-29T09:00:00Z", endDate: "2026-09-12T09:00:00Z", goal: "Ship checkout",
    issues: [], total: 14, done: 8, points: 34, donePoints: 21,
    membershipCached: true, notSynced: 0, truncated: false,
    ...over,
  };
}

const timeBar = (name = "Time elapsed in Sprint 12") => screen.getByRole("progressbar", { name });
const pointsBar = () => screen.queryByRole("progressbar", { name: "Points done in Sprint 12" });

describe("SprintTimeline", () => {
  it("says where the sprint is, how much calendar is left, and how much has landed", () => {
    render(<SprintTimeline detail={detail()} />);
    expect(screen.getByText("Day 6 of 15")).toBeInTheDocument();
    expect(screen.getByText("9 days left")).toBeInTheDocument();
    expect(timeBar()).toHaveAttribute("aria-valuetext", "Day 6 of 15");
    expect(screen.getByText("8 of 14 done, 21 of 34 pts")).toBeInTheDocument();
    const pts = pointsBar();
    expect(pts).toHaveAttribute("aria-valuenow", "21");
    expect(pts).toHaveAttribute("aria-valuemax", "34");
  });

  it("marks today on a running sprint and leaves a finished one unmarked", () => {
    const { container: running } = render(<SprintTimeline detail={detail()} />);
    const { container: finished } = render(
      <SprintTimeline detail={detail({ state: "closed", completeDate: "2026-08-30T09:00:00Z" })} />,
    );
    expect(running.querySelector(".progress-bar-marker")).not.toBeNull();
    // A closed sprint has no today inside it, so a line saying "you are
    // here" would be pointing at nothing.
    expect(finished.querySelector(".progress-bar-marker")).toBeNull();
  });

  it("shows a future sprint an empty points bar rather than a full one", () => {
    render(<SprintTimeline detail={detail({
      state: "future", startDate: "2026-09-14T09:00:00Z", endDate: "2026-09-28T09:00:00Z",
      total: 0, done: 0, points: 21, donePoints: 0,
    })} />);
    expect(screen.getByText("Starts in 11 days")).toBeInTheDocument();
    const pts = pointsBar();
    expect(pts).toHaveAttribute("aria-valuenow", "0");
    expect(pts).toHaveAttribute("aria-valuetext", "0 of 21 pts planned");
  });

  it("says a closed sprint has not been read rather than drawing an empty bar", () => {
    // This is the state of every closed sprint on every board the first
    // time it is synced, so it is not an edge case: the backfill reads
    // twelve of them per pass and the rest wait their turn.
    render(<SprintTimeline detail={detail({
      state: "closed", membershipCached: false, total: 0, done: 0, points: 0, donePoints: 0,
    })} />);
    expect(screen.getByText(/not read yet/)).toBeInTheDocument();
    expect(pointsBar()).toBeNull();
    // The calendar is still known, so the time bar is still drawn.
    expect(timeBar()).toHaveAttribute("aria-valuetext", `Closed ${calendarDay("2026-09-12")}`);
  });

  it("draws nothing at all for a sprint whose dates it cannot read", () => {
    render(<SprintTimeline detail={detail({ startDate: "", endDate: "", total: 0, done: 0, points: 0, donePoints: 0 })} />);
    expect(screen.getByText("no timeline")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).toBeNull();
  });

  it("says how far past its end a sprint nobody closed has run", () => {
    vi.setSystemTime(new Date("2026-09-14T10:00:00Z"));
    render(<SprintTimeline detail={detail()} />);
    expect(screen.getByText("2 days over")).toBeInTheDocument();
    expect(timeBar()).toHaveAttribute("aria-valuenow", "100");
  });
});
