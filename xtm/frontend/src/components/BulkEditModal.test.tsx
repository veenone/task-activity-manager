import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BulkEditModal } from "./BulkEditModal";

const bulkEdit = vi.fn();

vi.mock("../api", () => ({
  BulkEditTests: (...a: unknown[]) => bulkEdit(...a),
  ListLabels: vi.fn(async () => [
    { label: "smoke", count: 2 },
    { label: "login", count: 1 },
  ]),
  errMsg: (e: unknown) => String(e),
}));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

function renderModal() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <BulkEditModal testKeys={["QA-1", "QA-2"]} onComplete={() => {}} onCancel={() => {}} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  bulkEdit.mockReset();
  bulkEdit.mockResolvedValue({ succeeded: ["QA-1", "QA-2"], failed: [] });
});

describe("BulkEditModal labels", () => {
  it("adds several labels picked from suggestions in one call", async () => {
    renderModal();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Field" }), "labels");
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Operation" }), "add_label");
    const picker = screen.getByRole("combobox", { name: "Labels" });
    await userEvent.type(picker, "smo");
    await userEvent.click(await screen.findByRole("option", { name: "smoke" }));
    await userEvent.type(picker, "fresh{Enter}");
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(bulkEdit).toHaveBeenCalledWith("p1", ["QA-1", "QA-2"], {
      operation: "add_label",
      field: "labels",
      value: "smoke fresh",
    });
  });

  it("Replace all keeps a typed label that was not confirmed with Enter", async () => {
    renderModal();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Field" }), "labels");
    await userEvent.type(screen.getByRole("combobox", { name: "Labels" }), "regression");
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(bulkEdit).toHaveBeenCalledWith("p1", ["QA-1", "QA-2"], {
      operation: "set",
      field: "labels",
      value: "regression",
    });
  });
});
