import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  DatasetCatalogDetail,
  DatasetCatalogPage,
  DatasetCatalogQuery,
} from "../../api";
import { BrowserOpenURL, SyncDatasetCatalog } from "../../api";
import {
  useDatasetCatalog,
  useDatasetCatalogDetail,
} from "../../queries/datasets";
import { DatasetsView } from "./DatasetsView";

const beginSync = vi.fn();
const endSync = vi.fn();
const failSync = vi.fn();

vi.mock("../../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));
vi.mock("../../contexts/SyncContext", () => ({
  useSync: () => ({
    beginSync,
    endSync,
    failSync,
    canSync: true,
    progress: null,
  }),
}));
vi.mock("../../lib/viewState", async () => {
  const ReactModule = await import("react");
  return {
    useViewState: <T,>(_profile: string, _view: string, _field: string, initial: T) =>
      ReactModule.useState(initial),
  };
});
vi.mock("../../queries/datasets", () => ({
  useDatasetCatalog: vi.fn(),
  useDatasetCatalogDetail: vi.fn(),
}));
vi.mock("../../api", () => ({
  SyncDatasetCatalog: vi.fn(),
  BrowserOpenURL: vi.fn(),
  errMsg: (error: unknown) =>
    error instanceof Error ? error.message : String(error),
  isDemoUrl: (url: string) => url === "demo",
}));

const state = {
  activeGeneration: 1,
  lastSyncedAt: "2026-10-18T12:00:00Z",
  ownedTestCount: 1,
  externalTestCount: 0,
  relationshipCount: 2,
  testGroupCount: 1,
  definitionCount: 2,
};

const testRow = {
  testKey: "QA-1",
  summary: "Signs in with a valid account",
  projectKey: "QA",
  external: false,
  internalOpenable: true,
  defaultParameterCount: 1,
  defaultRowCount: 2,
  planCount: 1,
  executionCount: 1,
  overrideCount: 1,
};

function page(overrides: Partial<DatasetCatalogPage> = {}): DatasetCatalogPage {
  return { state, tests: [testRow], total: 1, ...overrides };
}

const detail: DatasetCatalogDetail = {
  test: testRow,
  definitions: [
    {
      testKey: "QA-1",
      scopeKind: "test",
      contextKey: "",
      contextSummary: "",
      parameters: [{ name: "USER", combinatorial: false }],
      rows: [["alice"]],
      metadataComplete: true,
    },
    {
      testKey: "QA-1",
      scopeKind: "plan",
      contextKey: "PLAN-1",
      contextSummary: "Release plan",
      parameters: [{ name: "USER", combinatorial: false }],
      rows: [["plan-user"]],
      metadataComplete: false,
    },
  ],
  usages: [
    {
      testKey: "QA-1",
      containerKind: "testplan",
      containerKey: "PLAN-1",
      containerSummary: "Release plan",
    },
    {
      testKey: "QA-1",
      containerKind: "testexec",
      containerKey: "EXEC-1",
      containerSummary: "October run",
    },
  ],
  execPlans: [{ execKey: "EXEC-1", planKey: "PLAN-1" }],
};

function queryResult(data: DatasetCatalogPage | undefined, error: Error | null = null) {
  return {
    data,
    error,
    isLoading: false,
    isFetching: false,
  } as ReturnType<typeof useDatasetCatalog>;
}

function renderView(
  props: Partial<React.ComponentProps<typeof DatasetsView>> = {},
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const onOpenTest = vi.fn();
  const onOpenContainer = vi.fn();
  render(
    <QueryClientProvider client={client}>
      <DatasetsView
        jiraUrl="https://jira.example.com"
        onOpenTest={onOpenTest}
        onOpenContainer={onOpenContainer}
        {...props}
      />
    </QueryClientProvider>,
  );
  return { client, onOpenTest, onOpenContainer };
}

describe("DatasetsView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useDatasetCatalog).mockReturnValue(
      queryResult(page({ state: null, tests: [], total: 0 })),
    );
    vi.mocked(useDatasetCatalogDetail).mockReturnValue({
      data: detail,
      error: null,
      isLoading: false,
    } as ReturnType<typeof useDatasetCatalogDetail>);
    vi.mocked(SyncDatasetCatalog).mockResolvedValue(undefined);
  });

  it("explains the never-synced state and offers an explicit scan", () => {
    renderView();
    expect(screen.getByRole("heading", { name: "Datasets" })).toBeVisible();
    expect(screen.getByText(/has not been scanned for datasets/i)).toBeVisible();
    expect(screen.getByRole("button", { name: "Scan datasets" })).toBeEnabled();
  });

  it("keeps the previous page visible while a scan is running", async () => {
    vi.mocked(useDatasetCatalog).mockReturnValue(queryResult(page()));
    let finish: (() => void) | undefined;
    vi.mocked(SyncDatasetCatalog).mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole("button", { name: "Scan datasets" }));
    expect(screen.getByText(testRow.summary)).toBeVisible();
    expect(screen.getByRole("button", { name: "Scanning datasets" })).toBeDisabled();
    finish?.();
    await waitFor(() => expect(endSync).toHaveBeenCalled());
  });

  it("distinguishes a successful empty result from a catalog not yet built", () => {
    vi.mocked(useDatasetCatalog).mockReturnValue(
      queryResult(page({ tests: [], total: 0 })),
    );
    renderView();
    expect(screen.getByText(/no tests with datasets match/i)).toBeVisible();
    expect(screen.queryByText(/has not been scanned/i)).not.toBeInTheDocument();
  });

  it("keeps snapshot rows visible when loading or scanning fails", async () => {
    vi.mocked(useDatasetCatalog).mockReturnValue(
      queryResult(page(), new Error("catalog refresh failed")),
    );
    vi.mocked(SyncDatasetCatalog).mockRejectedValue(new Error("scan failed"));
    const user = userEvent.setup();
    renderView();

    expect(screen.getByText("catalog refresh failed")).toBeVisible();
    expect(screen.getByText(testRow.summary)).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Scan datasets" }));
    expect(await screen.findByText("scan failed")).toBeVisible();
    expect(screen.getByText(testRow.summary)).toBeVisible();
  });

  it("shows grouped defaults, overrides, usages, and unknown Plan metadata", async () => {
    vi.mocked(useDatasetCatalog).mockReturnValue(
      queryResult(
        page({
          tests: [
            {
              ...testRow,
              external: true,
              projectKey: "EXT",
              defaultParameterCount: 0,
              defaultRowCount: 0,
            },
          ],
        }),
      ),
    );
    const user = userEvent.setup();
    renderView();

    expect(screen.getByText("External")).toBeVisible();
    expect(screen.getByText("Override only")).toBeVisible();
    expect(screen.getByText("1 override")).toBeVisible();
    await user.click(screen.getByRole("button", { name: /expand QA-1/i }));
    expect(await screen.findByRole("heading", { name: "Default dataset" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "Plan overrides" })).toBeVisible();
    expect(screen.getByText(/do not say what type each parameter is/)).toBeVisible();
    expect(screen.getByRole("heading", { name: "Used by" })).toBeVisible();
    expect(screen.getByText("Inherited default")).toBeVisible();
  });

  it("renders_scope_cards_and_a_compact_usage_table", async () => {
    vi.mocked(useDatasetCatalog).mockReturnValue(queryResult(page()));
    const user = userEvent.setup();
    renderView();

    const expand = screen.getByRole("button", { name: /expand QA-1/i });
    expect(expand).toHaveAttribute("aria-expanded", "false");
    await user.click(expand);
    expect(expand).toHaveAttribute("aria-expanded", "true");

    expect(screen.getByRole("region", { name: "Default dataset" })).toBeVisible();
    expect(screen.getByRole("region", { name: "Plan PLAN-1 dataset" })).toBeVisible();
    const usageTable = screen.getByRole("table", {
      name: "Containers using this dataset",
    });
    for (const heading of ["Type", "Container", "Summary", "Dataset"]) {
      expect(
        within(usageTable).getByRole("columnheader", { name: heading }),
      ).toBeVisible();
    }
    expect(within(usageTable).getByRole("button", { name: "Open Plan PLAN-1" })).toBeVisible();
    expect(
      within(usageTable).getByRole("button", { name: "Open Execution EXEC-1" }),
    ).toBeVisible();
    expect(within(usageTable).getByText("Override")).toBeVisible();
    expect(within(usageTable).getByText("Inherited default")).toBeVisible();
  });

  it("keeps_long_usage_content_reachable_at_narrow_width", async () => {
    const longSummary = "October release execution with an intentionally long container summary";
    vi.mocked(useDatasetCatalog).mockReturnValue(queryResult(page()));
    vi.mocked(useDatasetCatalogDetail).mockReturnValue({
      data: {
        ...detail,
        usages: detail.usages.map((usage) =>
          usage.containerKey === "EXEC-1"
            ? { ...usage, containerSummary: longSummary }
            : usage,
        ),
      },
      error: null,
      isLoading: false,
    } as ReturnType<typeof useDatasetCatalogDetail>);
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole("button", { name: /expand QA-1/i }));
    const usageTable = screen.getByRole("table", {
      name: "Containers using this dataset",
    });
    expect(screen.getByText(longSummary)).toBeVisible();
    expect(usageTable.parentElement).toHaveClass("dataset-usages-scroll");
  });

  it("persists complete filters, resets paging, and pages by Test group", async () => {
    vi.mocked(useDatasetCatalog).mockImplementation(
      (_profile: string, _query: DatasetCatalogQuery) =>
        queryResult(page({ total: 75 })) as ReturnType<typeof useDatasetCatalog>,
    );
    const user = userEvent.setup();
    renderView();

    await user.type(screen.getByRole("searchbox", { name: "Search datasets" }), "login");
    await user.selectOptions(screen.getByLabelText("Dataset scope"), "plan");
    await user.selectOptions(screen.getByLabelText("Test ownership"), "external");
    await user.selectOptions(screen.getByLabelText("Rows per page"), "25");
    await user.click(screen.getByRole("button", { name: "Next page" }));

    expect(useDatasetCatalog).toHaveBeenLastCalledWith("p1", {
      search: "login",
      scope: "plan",
      ownership: "external",
      limit: 25,
      offset: 25,
    });
  });

  it("expands from the keyboard and sends exact navigation targets", async () => {
    vi.mocked(useDatasetCatalog).mockReturnValue(queryResult(page()));
    const user = userEvent.setup();
    const { onOpenTest, onOpenContainer } = renderView();
    const expand = screen.getByRole("button", { name: /expand QA-1/i });
    expand.focus();
    await user.keyboard("{Enter}");

    await user.click(await screen.findByRole("button", { name: "Open Test QA-1" }));
    await user.click(screen.getByRole("button", { name: "Open Plan PLAN-1" }));
    await user.click(screen.getByRole("button", { name: "Open Execution EXEC-1" }));
    await user.click(screen.getByRole("button", { name: "Open QA-1 in Jira" }));

    expect(onOpenTest).toHaveBeenCalledWith("QA-1");
    expect(onOpenContainer).toHaveBeenNthCalledWith(1, {
      kind: "testplan",
      key: "PLAN-1",
    });
    expect(onOpenContainer).toHaveBeenNthCalledWith(2, {
      kind: "testexec",
      key: "EXEC-1",
    });
    expect(BrowserOpenURL).toHaveBeenCalledWith(
      "https://jira.example.com/browse/QA-1",
    );
  });

  it("offers Jira, but not an unsafe local action, for an external Test", async () => {
    vi.mocked(useDatasetCatalog).mockReturnValue(
      queryResult(
        page({
          tests: [{ ...testRow, external: true, internalOpenable: false }],
        }),
      ),
    );
    vi.mocked(useDatasetCatalogDetail).mockReturnValue({
      data: {
        ...detail,
        test: { ...testRow, external: true, internalOpenable: false },
      },
      error: null,
      isLoading: false,
    } as ReturnType<typeof useDatasetCatalogDetail>);
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole("button", { name: /expand QA-1/i }));

    expect(
      screen.queryByRole("button", { name: "Open Test QA-1" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Open QA-1 in Jira" }),
    ).toBeVisible();
  });
});
