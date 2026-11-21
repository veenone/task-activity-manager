import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { clearViewState } from "@agile-suite/core";
import { TraceabilityTabs } from "./TraceabilityTabs";

const h = vi.hoisted(() => ({
  statsError: null as Error | null,
  statsRefetch: vi.fn(),
  sankeyRefetch: vi.fn(),
}));

vi.mock("../api", () => ({
  ExportTraceability: vi.fn(async () => ""),
  GetProfileProjectKey: vi.fn(async () => "QA"),
  BrowserOpenURL: vi.fn(),
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
  isDemoUrl: () => false,
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

vi.mock("./SankeyChart", () => ({ SankeyChart: () => null }));
vi.mock("./RequirementSankey", () => ({ RequirementSankey: () => null }));

// One object per hook for the whole file: the view prunes its selections in
// effects keyed on the query data, so a fresh array each render never settles.
const q = vi.hoisted(() => {
  const loaded = (data: unknown) => ({ data, error: null, refetch: () => {} });
  return {
    stats: loaded({ total: 3, byCoverage: [] }),
    list: loaded([]),
    none: loaded(null),
    sankeyError: new Error("plan filter failed"),
  };
});

vi.mock("../queries/traceability", () => ({
  useTraceabilityStats: () =>
    h.statsError ? { data: undefined, error: h.statsError, refetch: h.statsRefetch } : q.stats,
  useTraceabilityReqOptions: () => q.list,
  useRequirementSankey: () => q.none,
  usePlanExecSankey: () => ({ data: undefined, error: q.sankeyError, refetch: h.sankeyRefetch }),
  useSubTaskSankey: () => q.none,
  useTraceabilityPlanContainers: () => q.list,
  useTraceabilityExecContainers: () => q.list,
  useTraceabilityExecutions: () => q.list,
  useTraceabilityBugs: () => q.list,
}));

// XTM had no Retry anywhere (#532): each failed load now offers one, and the
// flow's error sits under the filters that may have caused it.
describe("TraceabilityTabs load errors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clearViewState("p1");
    h.statsError = null;
  });

  it("keeps the execution filters on screen and retries the flow", async () => {
    render(<TraceabilityTabs />);
    expect(screen.getByRole("alert")).toHaveTextContent("plan filter failed");
    expect(screen.getByRole("checkbox", { name: "Cross-project only" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(h.sankeyRefetch).toHaveBeenCalledTimes(1);
  });

  it("retries the statistics when they fail", async () => {
    h.statsError = new Error("database is locked");
    render(<TraceabilityTabs />);
    expect(screen.getByRole("alert")).toHaveTextContent("database is locked");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(h.statsRefetch).toHaveBeenCalledTimes(1);
  });
});
