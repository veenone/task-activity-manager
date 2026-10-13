import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BulkCreateComponentsModal } from "./BulkCreateComponentsModal";

const api = vi.hoisted(() => ({
  CheckComponents: vi.fn(),
  CreateComponents: vi.fn(),
  ParseComponentWorkbook: vi.fn(),
  DownloadComponentTemplate: vi.fn(),
}));
vi.mock("../../api", () => ({
  ...api,
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));
vi.mock("../../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));
vi.mock("../../files", () => ({
  fileToBase64: vi.fn(async () => ({ b64: "QUJD", xlsx: true })),
}));

const input = (name: string, extra = {}) => ({
  name,
  description: "",
  leadUserName: "",
  assigneeType: "",
  ...extra,
});
const ready = (name: string, extra = {}) => ({
  input: { ...input(name, extra), assigneeType: "PROJECT_DEFAULT" },
  problem: "",
});

function renderModal() {
  const onCreated = vi.fn();
  const onClose = vi.fn();
  render(<BulkCreateComponentsModal onCreated={onCreated} onClose={onClose} />);
  return { onCreated, onClose };
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
});

describe("BulkCreateComponentsModal", () => {
  it("previews pasted names and creates only the ready ones", async () => {
    api.CheckComponents.mockResolvedValue([
      ready("API"),
      { input: input("core"), problem: 'The project already has a component named "Core".' },
      ready("UI"),
    ]);
    api.CreateComponents.mockResolvedValue({
      created: [{ id: "9", name: "API" }],
      failed: [{ row: 2, name: "UI", error: "Jira said no" }],
    });
    const { onCreated } = renderModal();

    await userEvent.type(
      screen.getByLabelText("Names, one per line"),
      "API\n\ncore\n UI ",
    );
    await userEvent.click(screen.getByRole("button", { name: "Preview" }));

    expect(api.CheckComponents).toHaveBeenCalledWith("p1", [
      input("API"),
      input("core"),
      input("UI"),
    ]);
    const table = await screen.findByRole("table");
    expect(
      within(table).getByText('The project already has a component named "Core".'),
    ).toBeTruthy();

    await userEvent.click(
      screen.getByRole("button", { name: "Create 2 components" }),
    );
    expect(api.CreateComponents).toHaveBeenCalledWith("p1", [
      ready("API").input,
      ready("UI").input,
    ]);
    expect(await screen.findByText("Created 1 component.")).toBeTruthy();
    expect(screen.getByText("UI: Jira said no")).toBeTruthy();
    expect(onCreated).toHaveBeenCalledTimes(1);
  });

  it("previews the rows of a loaded workbook", async () => {
    const rows = [input("API", { description: "Public API", leadUserName: "jdoe", assigneeType: "COMPONENT_LEAD" })];
    api.ParseComponentWorkbook.mockResolvedValue(rows);
    api.CheckComponents.mockResolvedValue([{ input: rows[0], problem: "" }]);
    renderModal();

    const file = new File(["x"], "components.xlsx");
    await userEvent.upload(screen.getByLabelText("XLSX workbook"), file);

    expect(api.ParseComponentWorkbook).toHaveBeenCalledWith("QUJD");
    expect(api.CheckComponents).toHaveBeenCalledWith("p1", rows);
    const table = await screen.findByRole("table");
    expect(within(table).getByText("Public API")).toBeTruthy();
    expect(within(table).getByText("Component lead")).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Create 1 component" }),
    ).toBeTruthy();
  });

  it("says why a workbook could not be read", async () => {
    api.ParseComponentWorkbook.mockRejectedValue(
      new Error("The workbook has no Name column. Start from the template."),
    );
    renderModal();
    await userEvent.upload(
      screen.getByLabelText("XLSX workbook"),
      new File(["x"], "c.xlsx"),
    );
    expect(
      await screen.findByText("The workbook has no Name column. Start from the template."),
    ).toBeTruthy();
    expect(api.CheckComponents).not.toHaveBeenCalled();
  });

  it("downloads the template", async () => {
    api.DownloadComponentTemplate.mockResolvedValue("C:/t.xlsx");
    renderModal();
    await userEvent.click(screen.getByRole("button", { name: "Download template" }));
    expect(api.DownloadComponentTemplate).toHaveBeenCalled();
    expect(await screen.findByText("Template saved to C:/t.xlsx")).toBeTruthy();
  });
});
