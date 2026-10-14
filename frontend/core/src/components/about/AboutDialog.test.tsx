import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AboutDialog } from "./AboutDialog";

const diag = {
  version: "1.2.0",
  dbPath: "C:/data/app.db",
  logPath: "C:/data/app.log",
  os: "windows",
  arch: "amd64",
  goVersion: "go1.25.0",
  schemaVersion: 52,
  profileCount: 1,
  startupError: "",
};

function setup(over: Partial<Parameters<typeof AboutDialog>[0]> = {}) {
  const props = {
    appName: "Xray Test Manager",
    kicker: "Test Repository · Data Center",
    tagline: "Managing Xray tests offline.",
    logoSrc: "logo.png",
    targets: "Jira DC 8.14+",
    docsUrl: "https://docs.example/api",
    diagnostics: diag,
    onOpenUrl: vi.fn(),
    onClose: vi.fn(),
    ...over,
  };
  render(<AboutDialog {...props} />);
  return props;
}

describe("AboutDialog", () => {
  it("is named after the app and reads out its build", () => {
    setup({ extraRows: [["Log", diag.logPath, true]] });
    expect(screen.getByRole("dialog", { name: "About Xray Test Manager" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Xray Test Manager" })).toBeInTheDocument();
    const readout = screen.getByRole("group", { name: "Build and environment" });
    expect(within(readout).getByText("v1.2.0")).toBeInTheDocument();
    expect(within(readout).getByText("v52")).toBeInTheDocument();
    expect(within(readout).getByText("go1.25.0 · windows/amd64")).toBeInTheDocument();
    expect(within(readout).getByText("Jira DC 8.14+")).toBeInTheDocument();
    expect(within(readout).getByText(diag.dbPath)).toBeInTheDocument();
    expect(within(readout).getByText(diag.logPath)).toBeInTheDocument();
  });

  it("shows an ellipsis until the diagnostics arrive", () => {
    setup({ diagnostics: null });
    const readout = screen.getByRole("group", { name: "Build and environment" });
    expect(within(readout).getAllByText("…").length).toBeGreaterThan(0);
  });

  it("opens the documentation and closes", async () => {
    const user = userEvent.setup();
    const props = setup();
    await user.click(screen.getByRole("button", { name: /Documentation/ }));
    expect(props.onOpenUrl).toHaveBeenCalledWith("https://docs.example/api");
    await user.click(screen.getByText("Close", { selector: "button" }));
    expect(props.onClose).toHaveBeenCalled();
  });

  it("carries an app's own action beside the documentation", () => {
    setup({ actions: <button>Take the tour</button> });
    expect(screen.getByRole("button", { name: "Take the tour" })).toBeInTheDocument();
  });
});
