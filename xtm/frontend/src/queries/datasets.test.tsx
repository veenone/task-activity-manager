import React from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "../api";
import { keys } from "./keys";
import { useDatasetCatalog, useDatasetCatalogDetail, useTestDataset } from "./datasets";

vi.mock("../api", () => ({
  ListDatasetCatalog: vi.fn(),
  GetDatasetCatalogDetail: vi.fn(),
  GetTestDataset: vi.fn(),
  errMsg: (error: unknown) =>
    error instanceof Error ? error.message : String(error),
}));

function makeHarness() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
  }
  return { client, Wrapper };
}

describe("dataset catalog queries", () => {
  beforeEach(() => vi.clearAllMocks());

  it("does not read either query without the inputs that enable it", () => {
    const { Wrapper } = makeHarness();
    const list = renderHook(
      () => useDatasetCatalog("", { search: "", scope: "", ownership: "", limit: 50, offset: 0 }),
      { wrapper: Wrapper },
    );
    const detail = renderHook(
      () => useDatasetCatalogDetail("p1", "QA-1", false),
      { wrapper: Wrapper },
    );
    const dataset = renderHook(() => useTestDataset("p1", "QA-1", false), {
      wrapper: Wrapper,
    });

    expect(list.result.current.fetchStatus).toBe("idle");
    expect(detail.result.current.fetchStatus).toBe("idle");
    expect(dataset.result.current.fetchStatus).toBe("idle");
    expect(api.ListDatasetCatalog).not.toHaveBeenCalled();
    expect(api.GetDatasetCatalogDetail).not.toHaveBeenCalled();
    expect(api.GetTestDataset).not.toHaveBeenCalled();
  });

  it("keys a Test dataset by profile and Test identity", async () => {
    (api.GetTestDataset as ReturnType<typeof vi.fn>).mockResolvedValue({
      parameters: [{ name: "USER", combinatorial: false }],
      rows: [["alice"]],
    });
    const { client, Wrapper } = makeHarness();
    const { result } = renderHook(() => useTestDataset("p1", "QA-1", true), {
      wrapper: Wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(client.getQueryData(["test-dataset", "p1", "QA-1"])).toEqual(result.current.data);
    expect(api.GetTestDataset).toHaveBeenCalledWith("p1", "QA-1", false);
  });

  it("does not let a late response from the previous Test seed the current Test", async () => {
    let resolveQA1!: (value: unknown) => void;
    const qa1 = new Promise((resolve) => { resolveQA1 = resolve; });
    (api.GetTestDataset as ReturnType<typeof vi.fn>).mockImplementation(
      (_profile: string, key: string) => key === "QA-1"
        ? qa1
        : Promise.resolve({ parameters: [{ name: "QA2" }], rows: [["two"]] }),
    );
    const { client, Wrapper } = makeHarness();
    const { result, rerender } = renderHook(
      ({ testKey }) => useTestDataset("p1", testKey, true),
      { wrapper: Wrapper, initialProps: { testKey: "QA-1" } },
    );

    rerender({ testKey: "QA-2" });
    await waitFor(() => expect(result.current.data?.parameters[0]?.name).toBe("QA2"));
    resolveQA1({ parameters: [{ name: "QA1" }], rows: [["one"]] });
    await act(async () => {});

    expect(result.current.data?.parameters[0]?.name).toBe("QA2");
    expect(client.getQueryData(["test-dataset", "p1", "QA-1"])).toEqual({
      parameters: [{ name: "QA1" }], rows: [["one"]],
    });
    expect(client.getQueryData(["test-dataset", "p1", "QA-2"])).toEqual({
      parameters: [{ name: "QA2" }], rows: [["two"]],
    });
  });

  it("puts the complete query object in the catalog cache key", async () => {
    (api.ListDatasetCatalog as ReturnType<typeof vi.fn>).mockResolvedValue({
      state: null,
      tests: [],
      total: 0,
    });
    const { client, Wrapper } = makeHarness();
    const query = {
      search: "login",
      scope: "plan",
      ownership: "external",
      limit: 25,
      offset: 50,
    };

    const { result } = renderHook(() => useDatasetCatalog("p1", query), {
      wrapper: Wrapper,
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(client.getQueryData(keys.datasetCatalog("p1", query))).toEqual({
      state: null,
      tests: [],
      total: 0,
    });
    expect(api.ListDatasetCatalog).toHaveBeenCalledWith("p1", query);
  });

  it("waits for an expanded Test before reading detail", async () => {
    (api.GetDatasetCatalogDetail as ReturnType<typeof vi.fn>).mockResolvedValue({
      test: { testKey: "QA-1" },
      definitions: [],
      usages: [],
      execPlans: [],
    });
    const { Wrapper } = makeHarness();
    const { result, rerender } = renderHook(
      ({ enabled }) => useDatasetCatalogDetail("p1", "QA-1", enabled),
      { wrapper: Wrapper, initialProps: { enabled: false } },
    );
    expect(result.current.fetchStatus).toBe("idle");

    rerender({ enabled: true });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(api.GetDatasetCatalogDetail).toHaveBeenCalledWith("p1", "QA-1");
  });

  it("refreshes active list and detail reads through the dataset family", async () => {
    (api.ListDatasetCatalog as ReturnType<typeof vi.fn>).mockResolvedValue({
      state: null,
      tests: [],
      total: 0,
    });
    (api.GetDatasetCatalogDetail as ReturnType<typeof vi.fn>).mockResolvedValue({
      test: { testKey: "QA-1" },
      definitions: [],
      usages: [],
      execPlans: [],
    });
    const { client, Wrapper } = makeHarness();
    const query = { search: "", scope: "", ownership: "", limit: 50, offset: 0 };
    const list = renderHook(() => useDatasetCatalog("p1", query), {
      wrapper: Wrapper,
    });
    const detail = renderHook(
      () => useDatasetCatalogDetail("p1", "QA-1", true),
      { wrapper: Wrapper },
    );
    await waitFor(() => {
      expect(list.result.current.isSuccess).toBe(true);
      expect(detail.result.current.isSuccess).toBe(true);
    });

    await act(async () => {
      await client.invalidateQueries({ queryKey: keys.datasets("p1") });
    });
    await waitFor(() => {
      expect(api.ListDatasetCatalog).toHaveBeenCalledTimes(2);
      expect(api.GetDatasetCatalogDetail).toHaveBeenCalledTimes(2);
    });
  });
});
