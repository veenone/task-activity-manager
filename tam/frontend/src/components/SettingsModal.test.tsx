import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SettingsModal } from "./SettingsModal";

const bindings = vi.hoisted(() => ({
  GetSettings: vi.fn(),
  SetReportExportDirectory: vi.fn(),
  ChooseReportExportDirectory: vi.fn(),
}));

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, ...bindings };
});

beforeEach(() => {
  bindings.GetSettings.mockReset().mockResolvedValue({ reportExportDir: "" });
  bindings.SetReportExportDirectory.mockReset().mockResolvedValue(undefined);
  bindings.ChooseReportExportDirectory.mockReset().mockResolvedValue("");
});

describe("SettingsModal", () => {
  it("saves the export folder the user typed and closes", async () => {
    bindings.GetSettings.mockResolvedValue({ reportExportDir: "C:\\old" });
    const onClose = vi.fn();
    render(<SettingsModal onClose={onClose} />);
    const field = await screen.findByLabelText(/Report export folder/);
    expect(field).toHaveValue("C:\\old");

    await userEvent.clear(field);
    await userEvent.type(field, "C:\\reports");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(bindings.SetReportExportDirectory).toHaveBeenCalledWith("C:\\reports"));
    expect(onClose).toHaveBeenCalled();
  });

  it("keeps the dialog open and says why when the folder is refused", async () => {
    bindings.SetReportExportDirectory.mockRejectedValue(new Error("C:\\nope is not a folder"));
    const onClose = vi.fn();
    render(<SettingsModal onClose={onClose} />);
    await screen.findByLabelText(/Report export folder/);
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("C:\\nope is not a folder");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("fills the field from the folder picker, and leaves it alone when the picker is cancelled", async () => {
    bindings.GetSettings.mockResolvedValue({ reportExportDir: "C:\\old" });
    render(<SettingsModal onClose={vi.fn()} />);
    const field = await screen.findByLabelText(/Report export folder/);

    await userEvent.click(screen.getByRole("button", { name: "Browse..." }));
    await waitFor(() => expect(bindings.ChooseReportExportDirectory).toHaveBeenCalled());
    expect(field).toHaveValue("C:\\old");

    bindings.ChooseReportExportDirectory.mockResolvedValue("D:\\picked");
    await userEvent.click(screen.getByRole("button", { name: "Browse..." }));
    await waitFor(() => expect(field).toHaveValue("D:\\picked"));
  });
});
