import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ImportRequirementsModal } from "./ImportRequirementsModal";

const api = vi.hoisted(() => ({
  AnalyzeRequirementImport: vi.fn(),
  ExportRequirementImportTemplate: vi.fn(),
  ImportRequirements: vi.fn(),
  ListRequirementSources: vi.fn(),
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

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.ListRequirementSources.mockResolvedValue([{ projectKey: "PRD", issueTypes: "Story" }]);
  api.AnalyzeRequirementImport.mockResolvedValue({ newCount: 1, existingCount: 0, rows: [] });
  api.ImportRequirements.mockResolvedValue({ created: 1, skippedExisting: 0 });
});

describe("ImportRequirementsModal", () => {
  // Done refreshes the pending list. Closing with the ✕ after an import has
  // to do the same, or the imported requirements stay out of it (#263).
  it("treats closing after an import as Done", async () => {
    const onComplete = vi.fn();
    const onCancel = vi.fn();
    const { container } = render(<ImportRequirementsModal onComplete={onComplete} onCancel={onCancel} />);
    await userEvent.upload(
      container.querySelector<HTMLInputElement>('input[type="file"]')!,
      new File(["x"], "reqs.csv"),
    );
    await userEvent.click(await screen.findByRole("button", { name: "Import 1 new" }));
    await screen.findByText(/Imported 1 requirement/);
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();
  });
});
