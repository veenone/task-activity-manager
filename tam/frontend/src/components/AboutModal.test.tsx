import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../api";
import { AboutModal } from "./AboutModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    GetDiagnostics: vi.fn(async () => ({
      version: "0.1.0",
      dbPath: "C:/tam/tam.db",
      sharedPath: "C:/agile-suite/profiles.db",
      logPath: "C:/tam/tam.log",
      os: "windows",
      arch: "amd64",
      goVersion: "go1.25.0",
      schemaVersion: 14,
      profileCount: 2,
      startupError: "",
    })),
    BrowserOpenURL: vi.fn(),
  };
});

describe("TAM AboutModal", () => {
  it("adds the shared profile store and the log to the readout", async () => {
    render(<AboutModal onClose={vi.fn()} />);
    expect(screen.getByRole("dialog", { name: "About Task Activity Manager" })).toBeInTheDocument();
    const readout = screen.getByRole("group", { name: "Build and environment" });
    const profiles = (await within(readout).findByText("C:/agile-suite/profiles.db")).closest("div")!;
    expect(within(profiles).getByText("Profiles")).toBeInTheDocument();
    const log = within(readout).getByText("C:/tam/tam.log").closest("div")!;
    expect(within(log).getByText("Log")).toBeInTheDocument();
  });

  it("opens the Jira REST documentation", async () => {
    const user = userEvent.setup();
    render(<AboutModal onClose={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /Documentation/ }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://docs.atlassian.com/software/jira/docs/api/REST/latest/");
  });
});
