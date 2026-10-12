import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TestTable } from "./TestTable";

vi.mock("../api", () => ({
  ListMatchingKeys: vi.fn(async () => []),
  ListStatuses: vi.fn(async () => []),
  ExportTests: vi.fn(),
  CreateSavedView: vi.fn(),
  ListSavedViews: vi.fn(async () => []),
  DeleteSavedView: vi.fn(),
  errMsg: (e: unknown) => String(e),
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

vi.mock("../contexts/SelectionContext", () => ({
  useSelection: () => ({
    selectedKey: "",
    selectedSet: new Set<string>(),
    setSelectedKey: vi.fn(),
    toggle: vi.fn(),
    togglePage: vi.fn(),
    selectAllMatching: vi.fn(),
  }),
}));

vi.mock("../features", () => ({
  REVIEW_ENABLED: false,
  useCapabilities: () => ({ supportsTestTypes: false }),
}));

vi.mock("../queries/tests", () => ({
  useTests: () => ({ data: { tests: [], total: 0 }, isFetching: false, error: null }),
}));

vi.mock("../queries/testCalls", () => ({
  useTestCallerKeys: () => ({ data: undefined }),
}));

vi.mock("./usePrompt", () => ({
  usePrompt: () => ({ prompt: vi.fn(async () => null) }),
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
