import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Dashboard } from "./Dashboard";

const api = vi.hoisted(() => ({
  ListFolders: vi.fn(),
  ListComponents: vi.fn(),
  ListStatuses: vi.fn(),
  ExportDashboard: vi.fn(),
}));

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, ...api };
});

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "profile-1" }),
}));

vi.mock("@agile-suite/core", async () => {
  const actual = await vi.importActual<typeof import("@agile-suite/core")>("@agile-suite/core");
  return { ...actual, useViewState: (_p: string, _v: string, _k: string, init: string) => [init, vi.fn()] };
});

vi.mock("../queries/statistics", () => ({
  useStatistics: () => ({ data: { total: 0 }, isFetching: false, error: null, refetch: vi.fn() }),
}));

describe("Dashboard filter lists", () => {
  beforeEach(() => {
    api.ListFolders.mockResolvedValue([]);
    api.ListComponents.mockResolvedValue([]);
    api.ListStatuses.mockResolvedValue([]);
  });

  // A filter list that failed to load used to go to the console only, so the
  // picker sat empty with nothing on screen saying why (#542).
  it("says which filter list could not be loaded, and why", async () => {
    api.ListFolders.mockRejectedValue(new Error("database is locked"));
    api.ListStatuses.mockRejectedValue(new Error("no such table: test_case"));
    render(<Dashboard />);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load the folder list: database is locked");
    expect(alert).toHaveTextContent("Could not load the status list: no such table: test_case");
    expect(alert).not.toHaveTextContent("component");
  });
});
