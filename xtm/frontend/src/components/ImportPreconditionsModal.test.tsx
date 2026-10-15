import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ImportPreconditionsModal } from "./ImportPreconditionsModal";

const api = vi.hoisted(() => ({
  PreviewImport: vi.fn(),
  ImportPreconditions: vi.fn(),
  ExportPreconditionImportTemplate: vi.fn(),
  GetPreconditionCreateFields: vi.fn(),
}));
vi.mock("../api", () => ({
  ...api,
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));
vi.mock("../files", () => ({
  fileToBase64: vi.fn(async () => ({ b64: "QUJD", xlsx: false })),
}));
vi.mock("./useNotice", () => ({ useNotice: () => ({ notice: vi.fn() }) }));

const mapping = {
  summary: "Summary",
  type: "Type",
  condition: "Condition",
  description: "Description",
  labels: "Labels",
};

const result = (over = {}) => ({
  created: 0,
  newCount: 1,
  existingCount: 1,
  rows: [
    { row: 2, summary: "Card inserted", type: "Manual", status: "new" },
    { row: 3, summary: "Network is up", type: "Manual", status: "existing" },
  ],
  errors: [],
  unknownTypes: [],
  ...over,
});

async function loadFile(onComplete = vi.fn(), onCancel = vi.fn()) {
  render(<ImportPreconditionsModal onComplete={onComplete} onCancel={onCancel} />);
  await userEvent.upload(
    screen.getByLabelText("CSV or XLSX file"),
    new File(["x"], "pre.csv"),
  );
  await screen.findByText("pre.csv (2 rows)");
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.PreviewImport.mockResolvedValue({
    headers: ["Summary", "Type", "Condition", "Description", "Labels"],
    rowCount: 2,
    sample: [],
  });
  api.GetPreconditionCreateFields.mockResolvedValue([
    { id: "customfield_20000", name: "Team", required: true, type: "text", allowedValues: [] },
  ]);
});

describe("ImportPreconditionsModal", () => {
  it("validates, then imports the new rows with the required fields", async () => {
    api.ImportPreconditions.mockResolvedValueOnce(result());
    api.ImportPreconditions.mockResolvedValueOnce(result({ created: 1 }));
    await loadFile();

    const validate = screen.getByRole("button", { name: "Validate" });
    expect(validate).toBeDisabled();
    await userEvent.type(await screen.findByLabelText("Team *"), "Payments");

    await userEvent.click(validate);
    expect(api.ImportPreconditions).toHaveBeenLastCalledWith(
      "p1", "QUJD", false, mapping, true, { customfield_20000: "Payments" },
    );
    expect(await screen.findByText("1 new, 1 already exists.")).toBeTruthy();

    await userEvent.click(screen.getByRole("button", { name: "Import" }));
    expect(api.ImportPreconditions).toHaveBeenLastCalledWith(
      "p1", "QUJD", false, mapping, false, { customfield_20000: "Payments" },
    );
    expect(
      await screen.findByText(/Imported 1 precondition as a pending create/),
    ).toBeTruthy();
  });

  it("stays on the errors when Import finds an unknown type", async () => {
    api.GetPreconditionCreateFields.mockResolvedValue([]);
    api.ImportPreconditions.mockResolvedValue(
      result({
        errors: [{ row: 3, message: 'Type "Automated" is not Manual, Generic or Cucumber.' }],
        unknownTypes: ["Automated"],
      }),
    );
    await loadFile();

    await userEvent.click(screen.getByRole("button", { name: "Import" }));
    expect(
      await screen.findByText('row 3: Type "Automated" is not Manual, Generic or Cucumber.'),
    ).toBeTruthy();
    expect(screen.queryByText(/Imported/)).toBeNull();
    expect(screen.getByText(/Nothing was imported/)).toBeTruthy();
  });

  // Done refreshes the pending list. Closing with the ✕ after an import has
  // to do the same, or the imported rows stay out of the list (#263).
  it("treats closing after an import as Done", async () => {
    api.GetPreconditionCreateFields.mockResolvedValue([]);
    api.ImportPreconditions.mockResolvedValue(result({ created: 1 }));
    const onComplete = vi.fn();
    const onCancel = vi.fn();
    await loadFile(onComplete, onCancel);
    await userEvent.click(screen.getByRole("button", { name: "Import" }));
    await screen.findByText(/Imported 1 precondition/);
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();
  });
});
