import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { EmptyState, LoadError } from "./EmptyState";

describe("EmptyState", () => {
  it("says the data is empty when no filter is set", () => {
    render(<EmptyState filtered={false} noun="tests">No tests yet. Run a sync.</EmptyState>);
    expect(screen.getByText("No tests yet. Run a sync.")).toBeInTheDocument();
  });

  // The empty line of a filtered list sent people to sync data they already
  // had (#532): a filter that hides everything is the filter's doing.
  it("blames the filter, not the data, when a filter is set", () => {
    render(<EmptyState filtered noun="cards">No cards on this board</EmptyState>);
    expect(screen.getByText("No cards match the current filter.")).toBeInTheDocument();
    expect(screen.queryByText("No cards on this board")).not.toBeInTheDocument();
  });
});

describe("LoadError", () => {
  it("shows the failure and retries on request", async () => {
    const retry = vi.fn();
    render(<LoadError message="Could not load the tests: disk full" onRetry={retry} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load the tests: disk full");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });
});
