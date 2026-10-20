import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../api";
import { useParameterizedRun } from "./parameterizedRuns";

vi.mock("../api", () => ({
  GetParameterizedRun: vi.fn(),
  errMsg: (error: unknown) => error instanceof Error ? error.message : String(error),
}));

function harness() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }
  return { client, Wrapper };
}

describe("parameterized run query", () => {
  beforeEach(() => vi.clearAllMocks());

  it("loads only after the member row is expanded", async () => {
    vi.mocked(api.GetParameterizedRun).mockResolvedValue({ iterations: [] } as never);
    const { Wrapper } = harness();
    const { result, rerender } = renderHook(
      ({ expanded }) => useParameterizedRun("p1", "EXEC-1", "QA-1", expanded),
      { wrapper: Wrapper, initialProps: { expanded: false } },
    );
    expect(result.current.fetchStatus).toBe("idle");
    expect(api.GetParameterizedRun).not.toHaveBeenCalled();

    rerender({ expanded: true });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.GetParameterizedRun).toHaveBeenCalledWith("p1", "EXEC-1", "QA-1", false);
  });

  it("isolates late responses by profile, execution and test", async () => {
    let resolveFirst!: (value: unknown) => void;
    const first = new Promise((resolve) => { resolveFirst = resolve; });
    vi.mocked(api.GetParameterizedRun).mockImplementation(
      (_profile, _exec, test) => test === "QA-1"
        ? first as never
        : Promise.resolve({ id: "run-2", testKey: "QA-2", iterations: [] }) as never,
    );
    const { client, Wrapper } = harness();
    const { result, rerender } = renderHook(
      ({ test }) => useParameterizedRun("p1", "EXEC-1", test, true),
      { wrapper: Wrapper, initialProps: { test: "QA-1" } },
    );
    rerender({ test: "QA-2" });
    await waitFor(() => expect(result.current.data?.testKey).toBe("QA-2"));
    resolveFirst({ id: "run-1", testKey: "QA-1", iterations: [] });
    await act(async () => {});

    expect(result.current.data?.testKey).toBe("QA-2");
    expect(client.getQueryData(["parameterized-run", "p1", "EXEC-1", "QA-1"])).toMatchObject({ testKey: "QA-1" });
    expect(client.getQueryData(["parameterized-run", "p1", "EXEC-1", "QA-2"])).toMatchObject({ testKey: "QA-2" });
  });
});
