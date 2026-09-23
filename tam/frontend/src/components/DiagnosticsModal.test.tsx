import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../api";
import { DiagnosticsModal } from "./DiagnosticsModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    GetDiagnostics: vi.fn(),
    ReadLog: vi.fn(),
    ExportDiagnostics: vi.fn(),
  };
});

const diag: api.Diagnostics = {
  version: "0.1.0",
  dbPath: "C:/Users/dev/AppData/Roaming/tam/tam.db",
  sharedPath: "C:/Users/dev/AppData/Roaming/agile-suite/profiles.db",
  logPath: "C:/Users/dev/AppData/Roaming/tam/tam.log",
  os: "windows",
  arch: "amd64",
  goVersion: "go1.25.0",
  schemaVersion: 14,
  profileCount: 2,
  startupError: "",
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetDiagnostics).mockResolvedValue(diag);
  vi.mocked(api.ReadLog).mockResolvedValue("tam: sync PLAT failed: 502 Bad Gateway");
  vi.mocked(api.ExportDiagnostics).mockResolvedValue("C:/Users/dev/AppData/Roaming/tam/tam-diagnostics-1.txt");
});

describe("DiagnosticsModal", () => {
  it("names the three paths, the build and the platform", async () => {
    render(<DiagnosticsModal onClose={vi.fn()} />);
    const fields = await screen.findByRole("group", { name: "Build and environment" });
    expect(within(fields).getByText(diag.dbPath)).toBeInTheDocument();
    expect(within(fields).getByText(diag.sharedPath)).toBeInTheDocument();
    expect(within(fields).getByText(diag.logPath)).toBeInTheDocument();
    expect(within(fields).getByText("v0.1.0")).toBeInTheDocument();
    expect(within(fields).getByText("go1.25.0 · windows/amd64")).toBeInTheDocument();
  });

  it("shows the recent log, which is what a failed sync is read from", async () => {
    render(<DiagnosticsModal onClose={vi.fn()} />);
    expect(await screen.findByText(/sync PLAT failed: 502 Bad Gateway/)).toBeInTheDocument();
    expect(api.ReadLog).toHaveBeenCalledWith();
  });

  it("says plainly when the log cannot be read", async () => {
    vi.mocked(api.ReadLog).mockRejectedValue(new Error("no log file has been written yet at C:/tam.log"));
    render(<DiagnosticsModal onClose={vi.fn()} />);
    expect(
      await screen.findByText("The log could not be read: no log file has been written yet at C:/tam.log"),
    ).toBeInTheDocument();
  });

  it("says the log is empty rather than drawing an empty pane", async () => {
    vi.mocked(api.ReadLog).mockResolvedValue("");
    render(<DiagnosticsModal onClose={vi.fn()} />);
    expect(await screen.findByText("The log is empty.")).toBeInTheDocument();
  });

  it("exports and says where the file went", async () => {
    const user = userEvent.setup();
    render(<DiagnosticsModal onClose={vi.fn()} />);
    await user.click(await screen.findByRole("button", { name: "Export" }));
    expect(
      await screen.findByText(/C:\/Users\/dev\/AppData\/Roaming\/tam\/tam-diagnostics-1.txt/),
    ).toBeInTheDocument();
  });

  it("says why an export failed instead of looking like it worked", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ExportDiagnostics).mockRejectedValue(new Error("write the diagnostics file: disk full"));
    render(<DiagnosticsModal onClose={vi.fn()} />);
    await user.click(await screen.findByRole("button", { name: "Export" }));
    expect(await screen.findByText(/disk full/)).toBeInTheDocument();
  });

  it("reads the log again on Refresh, so a sync run with the dialog open shows up", async () => {
    const user = userEvent.setup();
    render(<DiagnosticsModal onClose={vi.fn()} />);
    await screen.findByText(/502 Bad Gateway/);
    vi.mocked(api.ReadLog).mockResolvedValue("tam: synced PLAT (PLAT): 60 fetched");
    await user.click(screen.getByRole("button", { name: "Refresh" }));
    expect(await screen.findByText(/60 fetched/)).toBeInTheDocument();
  });

  it("is a named dialog that closes on Escape", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<DiagnosticsModal onClose={onClose} />);
    expect(await screen.findByRole("dialog", { name: "Diagnostics" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });
});
