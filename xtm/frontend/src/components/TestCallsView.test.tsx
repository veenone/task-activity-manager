import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { clearViewState } from "@agile-suite/core";
import { TestCallsView } from "./TestCallsView";

const h = vi.hoisted(() => ({
  refetch: vi.fn(),
}));

vi.mock("../api", () => ({
  SyncTestCalls: vi.fn(async () => undefined),
  EventsOn: vi.fn(() => () => {}),
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

vi.mock("./TestDetail", () => ({ TestDetail: () => null }));

vi.mock("../queries/testCalls", () => ({
  useTestCallLinks: () => ({
    data: undefined,
    isFetching: false,
    error: new Error("database is locked"),
    refetch: h.refetch,
  }),
}));

// The load error replaced the whole view, toolbar and all, so the Sync that
// might fix it was gone too (#532).
describe("TestCallsView load error", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clearViewState("p1");
  });

  it("keeps the toolbar and offers Retry, with no empty-state line", async () => {
    render(<TestCallsView />);
    expect(screen.getByRole("alert")).toHaveTextContent("database is locked");
    expect(screen.getByRole("button", { name: /Sync/ })).toBeEnabled();
    expect(screen.queryByText(/No call-test relationships yet/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(h.refetch).toHaveBeenCalledTimes(1);
  });
});
