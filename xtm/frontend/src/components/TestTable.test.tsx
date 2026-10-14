import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TestTable } from "./TestTable";
import { clearViewState } from "../lib/viewState";
import { CreateSavedView, ExportTests, ListMatchingKeys } from "../api";

// What the mocked queries and selection return; each test sets its own.
const h = vi.hoisted(() => ({
  page: { tests: [] as { key: string; summary: string; status: string }[], total: 0 },
  queries: [] as Record<string, unknown>[],
  selectedSet: new Set<string>(),
  selectAllMatching: (() => {}) as (keys: string[]) => void,
  promptAnswer: null as string | null,
}));

vi.mock("../api", () => ({
  ListMatchingKeys: vi.fn(async () => ["QA-1", "QA-2", "QA-3"]),
  ListStatuses: vi.fn(async () => []),
  ExportTests: vi.fn(async () => ""),
  CreateSavedView: vi.fn(async () => ({ id: "v1", name: "Mine", query: "" })),
  ListSavedViews: vi.fn(async () => []),
  DeleteSavedView: vi.fn(),
  errMsg: (e: unknown) => String(e),
}));

// jsdom has no layout, so the real virtualizer measures a zero-height viewport
// and renders nothing. This one lays out every row.
vi.mock("@tanstack/react-virtual", () => ({
  useVirtualizer: ({ count, estimateSize }: { count: number; estimateSize: () => number }) => ({
    getVirtualItems: () =>
      Array.from({ length: count }, (_, i) => ({ index: i, start: i * estimateSize() })),
    getTotalSize: () => count * estimateSize(),
    scrollToIndex: () => {},
  }),
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

vi.mock("../contexts/SelectionContext", () => ({
  useSelection: () => ({
    selectedKey: "",
    selectedSet: h.selectedSet,
    setSelectedKey: vi.fn(),
    toggle: vi.fn(),
    togglePage: vi.fn(),
    selectAllMatching: (keys: string[]) => h.selectAllMatching(keys),
  }),
}));

vi.mock("../features", () => ({
  REVIEW_ENABLED: false,
  useCapabilities: () => ({ supportsTestTypes: false }),
}));

vi.mock("../queries/tests", () => ({
  useTests: (_p: string, q: Record<string, unknown>) => {
    h.queries.push(q);
    return { data: h.page, isFetching: false, error: null };
  },
}));

vi.mock("../queries/testCalls", () => ({
  useTestCallerKeys: () => ({ data: undefined }),
}));

vi.mock("./usePrompt", () => ({
  usePrompt: () => ({ prompt: vi.fn(async () => h.promptAnswer) }),
}));
vi.mock("./useNotice", () => ({
  useNotice: () => ({ notice: vi.fn(async () => {}) }),
}));

// The Columns panel was a bare popover: no dialog role, no name, and no way
// to close it from the keyboard.
describe("TestTable Columns panel", () => {
  it("opens as a named dialog and closes on Escape", async () => {
    render(
      <TestTable folderId="" containerKey="" component="" pendingByTestKey={new Map()} />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Columns" }));
    expect(screen.getByRole("dialog", { name: "Columns" })).toBeInTheDocument();

    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("takes focus in, keeps Tab inside, and hands focus back to Columns", async () => {
    render(
      <TestTable folderId="" containerKey="" component="" pendingByTestKey={new Map()} />,
    );
    const trigger = screen.getByRole("button", { name: "Columns" });
    await userEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: "Columns" });
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    for (let i = 0; i < 40; i++) await userEvent.tab();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    await userEvent.keyboard("{Escape}");
    expect(document.activeElement).toBe(trigger);
  });
});

const tests = (n: number) =>
  Array.from({ length: n }, (_, i) => ({ key: `QA-${i + 1}`, summary: `Test ${i + 1}`, status: "Draft" }));
const lastQuery = () => h.queries[h.queries.length - 1];
const renderTable = () =>
  render(<TestTable folderId="F1" containerKey="" component="" pendingByTestKey={new Map()} />);

// The grid's own behaviour, pinned before it was split into a folder (#240).
describe("TestTable grid", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clearViewState("p1");
    localStorage.clear();
    h.page = { tests: [], total: 0 };
    h.queries = [];
    h.selectedSet.clear();
    h.promptAnswer = null;
  });

  it("renders the page's rows", () => {
    h.page = { tests: tests(2), total: 2 };
    renderTable();
    const grid = screen.getByRole("table", { name: "Tests" });
    expect(within(grid).getByText("Test 2")).toBeInTheDocument();
    expect(within(grid).getByRole("checkbox", { name: "Select QA-1" })).toBeInTheDocument();
  });

  it("moves row focus with the arrow keys", async () => {
    const user = userEvent.setup();
    h.page = { tests: tests(3), total: 3 };
    renderTable();
    const rows = screen.getAllByRole("row").slice(1);
    rows[0].focus();
    await user.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(rows[1]);
    await user.keyboard("{End}");
    expect(document.activeElement).toBe(rows[2]);
  });

  it("sorts by a column, then flips the direction", async () => {
    const user = userEvent.setup();
    renderTable();
    await user.click(screen.getByRole("button", { name: /Summary/ }));
    expect(lastQuery()).toMatchObject({ sortBy: "summary", desc: false, folderId: "F1" });
    await user.click(screen.getByRole("button", { name: /Summary/ }));
    expect(lastQuery()).toMatchObject({ sortBy: "summary", desc: true });
  });

  it("pages forward and shows the range", async () => {
    const user = userEvent.setup();
    h.page = { tests: tests(3), total: 250 };
    renderTable();
    expect(screen.getByText(/of 3$/)).toBeInTheDocument();
    await user.click(screen.getByTitle("Next page"));
    expect(lastQuery()).toMatchObject({ offset: 100, limit: 100 });
  });

  it("saves the current filter as a view", async () => {
    const user = userEvent.setup();
    h.promptAnswer = "Mine";
    renderTable();
    await user.type(screen.getByPlaceholderText("Status…"), "Draft");
    await user.click(screen.getByRole("button", { name: "Save view" }));
    await waitFor(() => expect(CreateSavedView).toHaveBeenCalled());
    const [, name, query] = (CreateSavedView as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(name).toBe("Mine");
    expect(JSON.parse(query)).toMatchObject({ status: "Draft", sortBy: "key", desc: true });
  });

  it("offers to select every matching test once the page is selected", async () => {
    const user = userEvent.setup();
    const picked: string[][] = [];
    h.selectAllMatching = (keys) => picked.push(keys);
    h.page = { tests: tests(2), total: 3 };
    h.selectedSet.add("QA-1").add("QA-2");
    renderTable();
    await user.click(screen.getByRole("button", { name: /Select all 3 matching/ }));
    await waitFor(() => expect(picked).toEqual([["QA-1", "QA-2", "QA-3"]]));
    expect(ListMatchingKeys).toHaveBeenCalledWith("p1", expect.objectContaining({ limit: 0, folderId: "F1" }));
  });

  it("exports every test matching the filter", async () => {
    const user = userEvent.setup();
    h.page = { tests: tests(1), total: 1 };
    renderTable();
    await user.click(screen.getByRole("button", { name: "Export" }));
    expect(ExportTests).toHaveBeenCalledWith("p1", expect.objectContaining({ limit: 0, offset: 0, folderId: "F1" }));
  });

  it("hides a column from the Columns dialog", async () => {
    const user = userEvent.setup();
    renderTable();
    expect(screen.getByRole("columnheader", { name: "Labels" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Columns" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("checkbox", { name: "Labels" }));
    expect(screen.queryByRole("columnheader", { name: "Labels" })).not.toBeInTheDocument();
  });
});
