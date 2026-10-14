import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DiagnosticsDialog } from "./DiagnosticsDialog";
import type { DiagnosticsInfo } from "./diagnostics";

const diag: DiagnosticsInfo = {
  version: "0.1.0",
  dbPath: "C:/Users/dev/AppData/Roaming/app/app.db",
  logPath: "C:/Users/dev/AppData/Roaming/app/app.log",
  os: "windows",
  arch: "amd64",
  goVersion: "go1.25.0",
  schemaVersion: 14,
  profileCount: 2,
  startupError: "",
};

function setup(over: Partial<Parameters<typeof DiagnosticsDialog>[0]> = {}) {
  const props = {
    appName: "the app",
    getDiagnostics: vi.fn(async () => diag),
    readLog: vi.fn(async () => "sync PLAT failed: 502 Bad Gateway"),
    exportDiagnostics: vi.fn(async () => "C:/out/diagnostics-1.txt"),
    onClose: vi.fn(),
    ...over,
  };
  render(<DiagnosticsDialog {...props} />);
  return props;
}

describe("DiagnosticsDialog", () => {
  it("names the build, the platform, the paths and any extra row", async () => {
    setup({ extraRows: (d) => [["Profiles", String(d.profileCount)]] });
    const fields = await screen.findByRole("group", { name: "Build and environment" });
    expect(within(fields).getByText(diag.dbPath)).toBeInTheDocument();
    expect(within(fields).getByText(diag.logPath)).toBeInTheDocument();
    expect(within(fields).getByText("v0.1.0")).toBeInTheDocument();
    expect(within(fields).getByText("go1.25.0 · windows/amd64")).toBeInTheDocument();
    expect(within(fields).getByText("2")).toBeInTheDocument();
  });

  it("shows a startup error as a row of its own", async () => {
    setup({ getDiagnostics: vi.fn(async () => ({ ...diag, startupError: "store locked" })) });
    expect(await screen.findByText("store locked")).toBeInTheDocument();
  });

  it("shows the recent log", async () => {
    setup();
    expect(await screen.findByText(/502 Bad Gateway/)).toBeInTheDocument();
  });

  it("says plainly when the log cannot be read", async () => {
    setup({ readLog: vi.fn(async () => Promise.reject(new Error("no log file yet"))) });
    expect(await screen.findByText("The log could not be read: no log file yet")).toBeInTheDocument();
  });

  it("says the log is empty rather than drawing an empty pane", async () => {
    setup({ readLog: vi.fn(async () => "") });
    expect(await screen.findByText("The log is empty.")).toBeInTheDocument();
  });

  it("names what the export carries, in the app's own name", async () => {
    setup({ appName: "XTM" });
    expect(await screen.findByText(/Jira items and people XTM has worked on/)).toBeInTheDocument();
  });

  it("exports and says where the file went, or why it did not", async () => {
    const user = userEvent.setup();
    const exportDiagnostics = vi.fn(async () => "C:/out/diagnostics-1.txt");
    setup({ exportDiagnostics });
    await user.click(await screen.findByRole("button", { name: "Export" }));
    expect(await screen.findByText("C:/out/diagnostics-1.txt")).toBeInTheDocument();
    exportDiagnostics.mockRejectedValueOnce(new Error("disk full"));
    await user.click(screen.getByRole("button", { name: "Export" }));
    expect(await screen.findByText(/disk full/)).toBeInTheDocument();
  });

  it("reads the log again on Refresh", async () => {
    const user = userEvent.setup();
    const readLog = vi.fn(async () => "sync PLAT failed: 502 Bad Gateway");
    setup({ readLog });
    await screen.findByText(/502 Bad Gateway/);
    readLog.mockResolvedValueOnce("synced PLAT: 60 fetched");
    await user.click(screen.getByRole("button", { name: "Refresh" }));
    expect(await screen.findByText(/60 fetched/)).toBeInTheDocument();
  });

  it("is a named dialog that closes on Escape", async () => {
    const user = userEvent.setup();
    const props = setup();
    expect(await screen.findByRole("dialog", { name: "Diagnostics" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(props.onClose).toHaveBeenCalled();
  });
});
