import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { DiagnosticsModal } from "./DiagnosticsModal";

const api = vi.hoisted(() => ({
  GetDiagnostics: vi.fn(async () => ({
    version: "1.10.0",
    dbPath: "C:/xtm/xtm.db",
    logPath: "C:/xtm/xtm.log",
    os: "windows",
    arch: "amd64",
    goVersion: "go1.25.0",
    schemaVersion: 52,
    profileCount: 3,
    startupError: "",
  })),
  ReadLog: vi.fn(async (_lines: number) => "xtm: synced QA"),
  ExportDiagnostics: vi.fn(async () => "C:/xtm/diag.txt"),
}));
vi.mock("../api", () => api);

describe("XTM DiagnosticsModal", () => {
  it("reads the last 500 log lines and shows the profile count", async () => {
    render(<DiagnosticsModal onClose={vi.fn()} />);
    expect(await screen.findByText("xtm: synced QA")).toBeInTheDocument();
    expect(api.ReadLog).toHaveBeenCalledWith(500);
    const fields = screen.getByRole("group", { name: "Build and environment" });
    const profiles = within(fields).getByText("Profiles").closest("div")!;
    expect(within(profiles).getByText("3")).toBeInTheDocument();
    expect(within(fields).getByText("C:/xtm/xtm.log")).toBeInTheDocument();
  });
});
