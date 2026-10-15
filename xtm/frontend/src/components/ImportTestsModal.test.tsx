import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ImportTestsModal } from "./ImportTestsModal";

const api = vi.hoisted(() => ({
  PreviewImport: vi.fn(),
  ImportTests: vi.fn(),
  ExportImportTemplate: vi.fn(),
}));
vi.mock("../api", () => ({
  ...api,
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));
vi.mock("./useNotice", () => ({ useNotice: () => ({ notice: vi.fn() }) }));

const imported = { created: 2, skipped: 0, errors: [], unknownComponents: [] };

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.PreviewImport.mockResolvedValue({ headers: ["Summary"], rowCount: 2, sample: [] });
  api.ImportTests.mockResolvedValue(imported);
});

async function importTwo(onComplete = vi.fn(), onCancel = vi.fn()) {
  const { container } = render(<ImportTestsModal onComplete={onComplete} onCancel={onCancel} />);
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  await userEvent.upload(input, new File(["Summary\nA\nB\n"], "tests.csv"));
  await screen.findByText(/tests\.csv/);
  await userEvent.click(screen.getByRole("button", { name: "Import" }));
  await screen.findByText(/Imported 2 tests as/);
  return { onComplete, onCancel };
}

describe("ImportTestsModal", () => {
  // Done refreshes the pending list. Closing with the ✕ after an import has
  // to do the same, or the imported tests stay out of the list (#263).
  it("treats closing after an import as Done", async () => {
    const { onComplete, onCancel } = await importTwo();
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();
  });

  it("cancels when closed before importing", async () => {
    const onComplete = vi.fn();
    const onCancel = vi.fn();
    render(<ImportTestsModal onComplete={onComplete} onCancel={onCancel} />);
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onComplete).not.toHaveBeenCalled();
  });
});
