import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useComponentCounts, useComponentOptions, useProjectComponents } from "./components";
import * as api from "../api";

vi.mock("../api", () => ({
  ListProjectComponentDetails: vi.fn(),
  ListComponents: vi.fn(),
  ListProjectComponents: vi.fn(),
  SearchUsers: vi.fn(),
  errMsg: (e: unknown) => String(e),
}));

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

beforeEach(() => vi.clearAllMocks());

describe("component queries", () => {
  it("useProjectComponents returns Jira's components", async () => {
    (api.ListProjectComponentDetails as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: "1", name: "Core", description: "", leadName: "", leadDisplayName: "", assigneeType: "PROJECT_DEFAULT" },
    ]);
    const { result } = renderHook(() => useProjectComponents("p1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.ListProjectComponentDetails).toHaveBeenCalledWith("p1");
    expect(result.current.data?.map((c) => c.name)).toEqual(["Core"]);
  });

  it("useComponentCounts maps names to test counts", async () => {
    (api.ListComponents as ReturnType<typeof vi.fn>).mockResolvedValue([
      { label: "Core", count: 4 },
    ]);
    const { result } = renderHook(() => useComponentCounts("p1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.get("Core")).toBe(4);
  });

  it("useComponentOptions returns the cached project components", async () => {
    (api.ListProjectComponents as ReturnType<typeof vi.fn>).mockResolvedValue(["API", "Core"]);
    const { result } = renderHook(() => useComponentOptions("p1", "QA"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.ListProjectComponents).toHaveBeenCalledWith("p1", "QA");
    expect(result.current.data).toEqual(["API", "Core"]);
  });
});
