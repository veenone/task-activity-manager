import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { clearViewState } from "@agile-suite/core";
import { Dashboard } from "./Dashboard";

const h = vi.hoisted(() => ({
  error: null as Error | null,
  refetch: vi.fn(),
}));

vi.mock("../api", () => ({
  ListFolders: vi.fn(async () => []),
  ListComponents: vi.fn(async () => []),
  ListStatuses: vi.fn(async () => ["Draft"]),
  ExportDashboard: vi.fn(async () => ""),
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

vi.mock("./DuplicatesCard", () => ({ DuplicatesCard: () => null }));

vi.mock("../queries/statistics", () => ({
  useStatistics: () => ({ data: undefined, isFetching: false, error: h.error, refetch: h.refetch }),
}));

// A filter can be what makes the statistics fail, so the error used to take
// the filter bar with it and leave nothing to change (#532).
describe("Dashboard load error", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clearViewState("p1");
    h.error = new Error("no such column: folder");
  });

  it("keeps the filters on screen and offers Retry", async () => {
    render(<Dashboard />);
    expect(screen.getByRole("alert")).toHaveTextContent("no such column: folder");
    expect(screen.getByTitle("Limit the dashboard to a status")).toHaveValue("");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(h.refetch).toHaveBeenCalledTimes(1);
  });
});
