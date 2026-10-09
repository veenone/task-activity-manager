import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BulkLabelsModal } from "./BulkLabelsModal";

const bulkLabels = vi.fn();

vi.mock("../api", () => ({
  BulkEditLabels: (...a: unknown[]) => bulkLabels(...a),
  ListLabels: vi.fn(async () => [
    { label: "smoke", count: 2 },
    { label: "login", count: 1 },
  ]),
  ListTestLabels: vi.fn(async () => ({
    "QA-1": ["smoke", "login"],
    "QA-2": ["smoke"],
    "QA-3": [],
  })),
  errMsg: (e: unknown) => String(e),
}));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

function renderModal(onComplete = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <BulkLabelsModal
        testKeys={["QA-1", "QA-2", "QA-3"]}
        onComplete={onComplete}
        onCancel={() => {}}
      />
    </QueryClientProvider>,
  );
  return onComplete;
}

async function pick(name: string, text: string) {
  const box = screen.getByRole("combobox", { name });
  await userEvent.type(box, text);
  await screen.findByRole("option", { name: new RegExp(text) });
  await userEvent.keyboard("{Enter}");
}

beforeEach(() => {
  bulkLabels.mockReset();
  bulkLabels.mockResolvedValue({ succeeded: ["QA-1", "QA-2", "QA-3"], failed: [] });
});

describe("BulkLabelsModal", () => {
  it("previews how many selected tests change", async () => {
    renderModal();
    await pick("Remove labels", "login");
    expect(await screen.findByText("1 of 3 selected tests will change.")).toBeTruthy();
    await pick("Add labels", "smoke");
    expect(await screen.findByText("2 of 3 selected tests will change.")).toBeTruthy();
  });

  it("sends add and remove lists and completes", async () => {
    const onComplete = renderModal();
    await pick("Add labels", "regression");
    await pick("Remove labels", "smoke");
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(bulkLabels).toHaveBeenCalledWith(
      "p1",
      ["QA-1", "QA-2", "QA-3"],
      ["regression"],
      ["smoke"],
    );
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it("refuses a label in both lists without calling the backend", async () => {
    renderModal();
    await pick("Add labels", "smoke");
    await pick("Remove labels", "smoke");
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(screen.getByText('"smoke" is in both Add and Remove.')).toBeTruthy();
    expect(bulkLabels).not.toHaveBeenCalled();
  });

  it("lists failed tests and stays open", async () => {
    bulkLabels.mockResolvedValue({
      succeeded: ["QA-1"],
      failed: [{ testKey: "QA-2", error: "not found" }],
    });
    const onComplete = renderModal();
    await pick("Add labels", "x");
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect((await screen.findByText("QA-2")).closest("li")?.textContent).toBe(
      "QA-2: not found",
    );
    expect(onComplete).not.toHaveBeenCalled();
  });
});
