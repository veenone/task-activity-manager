import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SyncProgress } from "../api";

const listeners = new Map<string, (p: SyncProgress) => void>();
const off = vi.fn();
vi.mock("../api", () => ({
  EventsOn: (name: string, fn: (p: SyncProgress) => void) => {
    listeners.set(name, fn);
    return () => {
      off(name);
      listeners.delete(name);
    };
  },
}));

import { CommitProgress } from "./CommitProgress";

function send(p: Partial<SyncProgress>) {
  act(() => listeners.get("commit:progress")?.({ phase: "commit", fetched: 0, total: 0, done: false, ...p }));
}

describe("commit progress in the pending changes dialog (#602)", () => {
  beforeEach(() => {
    listeners.clear();
    off.mockClear();
  });

  it("shows done of total and the key being pushed as events arrive", () => {
    render(<CommitProgress committing />);
    send({ fetched: 12, total: 100, stage: "QA-12" });

    const bar = screen.getByRole("progressbar", { name: "Commit progress" });
    expect(bar).toHaveAttribute("aria-valuenow", "12");
    expect(bar).toHaveAttribute("aria-valuemax", "100");
    expect(screen.getByText("Pushing QA-12: 12 of 100 done")).toBeInTheDocument();
  });

  it("fills the bar when the commit reports it is done", () => {
    render(<CommitProgress committing />);
    send({ fetched: 97, total: 100, stage: "QA-97" });
    send({ done: true });

    expect(screen.getByRole("progressbar", { name: "Commit progress" })).toHaveAttribute("aria-valuenow", "100");
    expect(screen.getByText("100 of 100 done")).toBeInTheDocument();
  });

  it("hides the bar and stops listening once the commit ends", () => {
    const { rerender } = render(<CommitProgress committing />);
    send({ fetched: 5, total: 10, stage: "QA-5" });
    rerender(<CommitProgress committing={false} />);

    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(off).toHaveBeenCalledWith("commit:progress");
  });
});
