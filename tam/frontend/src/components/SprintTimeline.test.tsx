import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { SprintDetail } from "../api";
import { calendarDay } from "../lib/format";
import { SprintTimeline } from "./SprintTimeline";

// The clock is pinned rather than threaded through the component: the
// formatters take now as a defaulted parameter, the way formatWhen and
// dayOfSprint already do, so nothing has to drill a prop down the tree.
// toFake: ["Date"] is not decoration, faking every timer breaks userEvent.
// PINNED is the clock every case here runs against, built with the
// local-time constructor rather than from a UTC instant. That is what makes
// these fixtures hold in any zone: a sprint's dates are read as civil days,
// so "now" has to be a civil day too. An instant like
// "2026-09-03T10:00:00Z" is a different calendar day either side of about
// eleven hours from UTC, and the row would read a day out for anyone there.
const PINNED = new Date(2026, 8, 3, 12, 0, 0);

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(PINNED);
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
    // And its bar is greyed rather than drawn in the running sprint's
    // accent. The class is the assertion because vite.config.ts sets
    // css: false, so no test here can read a computed colour; what the
    // class resolves to is theme.test.ts's business.
    expect(finished.querySelector(".progress-bar")).toHaveClass("progress-bar-muted");
    expect(running.querySelector(".progress-bar")).toHaveClass("progress-bar-time");
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
    expect(screen.getByText("cards not read yet")).toBeInTheDocument();
    expect(pointsBar()).toBeNull();
    // The calendar is still known, so the time bar is still drawn.
    expect(timeBar()).toHaveAttribute("aria-valuetext", `Closed ${calendarDay("2026-09-12")}`);
  });

  it("draws no bars for a sprint whose dates it cannot read, and counts it instead", () => {
    render(<SprintTimeline detail={detail({ startDate: "", endDate: "" })} />);
    expect(screen.getByText("no timeline")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).toBeNull();
    // It still says what it holds. A bar needs a calendar; a count does
    // not, and the row would otherwise say nothing at all about the board's
    // own unassigned work.
    expect(screen.getByText("8 of 14 done, 21 of 34 pts")).toBeInTheDocument();
  });

  it("counts a sprint that holds nothing rather than claiming progress in it", () => {
    render(<SprintTimeline detail={detail({ total: 0, done: 0, points: 0, donePoints: 0 })} />);
    expect(screen.getByText("0 cards")).toBeInTheDocument();
    expect(screen.queryByText(/done/)).toBeNull();
  });

  it("reads the same at either end of the same local day", () => {
    // The whole of the zone question, asserted rather than assumed: a zone
    // only changes which wall-clock moment "now" is, so a row that reads
    // the same one second after local midnight and one second before the
    // next reads the same everywhere. TZ is not settable on every platform
    // this suite runs on, so this is the guard that actually holds.
    vi.setSystemTime(new Date(2026, 8, 3, 0, 0, 1));
    const { container: justAfterMidnight } = render(<SprintTimeline detail={detail()} />);
    vi.setSystemTime(new Date(2026, 8, 3, 23, 59, 59));
    const { container: justBefore } = render(<SprintTimeline detail={detail()} />);
    expect(justAfterMidnight.textContent).toBe("Day 6 of 159 days left8 of 14 done, 21 of 34 pts");
    expect(justBefore.textContent).toBe(justAfterMidnight.textContent);
  });

  it("says how far past its end a sprint nobody closed has run", () => {
    vi.setSystemTime(new Date(2026, 8, 14, 12, 0, 0));
    render(<SprintTimeline detail={detail()} />);
    expect(screen.getByText("2 days over")).toBeInTheDocument();
    expect(timeBar()).toHaveAttribute("aria-valuenow", "100");
  });
});
