import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SettingsModal } from "./SettingsModal";

const bindings = vi.hoisted(() => ({
  GetSettings: vi.fn(),
  SetReportExportDirectory: vi.fn(),
  ChooseReportExportDirectory: vi.fn(),
  SetProjectKeyPatternCheck: vi.fn(),
}));

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, ...bindings };
});

beforeEach(() => {
  bindings.GetSettings.mockReset().mockResolvedValue({ reportExportDir: "" });
  bindings.SetReportExportDirectory.mockReset().mockResolvedValue(undefined);
  bindings.ChooseReportExportDirectory.mockReset().mockResolvedValue("");
  bindings.SetProjectKeyPatternCheck.mockReset().mockResolvedValue(undefined);
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

  it("shows the project key check on when nothing is stored, and saves it off", async () => {
    const onClose = vi.fn();
    render(<SettingsModal onClose={onClose} />);
    const check = await screen.findByLabelText(/project key/i);
    // Enabled is the modal saying GetSettings has landed: every control is
    // disabled until then. Without the wait this assertion passes on the
    // default the component starts from, which is also on, so it could not
    // fail if the stored value never arrived (C6).
    await waitFor(() => expect(check).toBeEnabled());
    expect(check).toBeChecked();

    await userEvent.click(check);
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(bindings.SetProjectKeyPatternCheck).toHaveBeenCalledWith(false));
    expect(onClose).toHaveBeenCalled();
  });

  it("shows the project key check off when it is stored off", async () => {
    bindings.GetSettings.mockResolvedValue({ reportExportDir: "", checkProjectKeyPattern: false });
    render(<SettingsModal onClose={vi.fn()} />);
    const check = await screen.findByLabelText(/project key/i);
    // findBy resolves on the first render carrying the label, which is the one
    // before GetSettings resolves, and checkKey starts on. So this asserted
    // against the default rather than the stored value and lost the race
    // whenever the first poll beat the promise, which is what a loaded CI
    // runner does. Enabled is the modal saying the read has landed.
    await waitFor(() => expect(check).toBeEnabled());
    expect(check).not.toBeChecked();
  });
});
