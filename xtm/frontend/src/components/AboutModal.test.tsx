import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AboutModal } from "./AboutModal";

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
  BrowserOpenURL: vi.fn(),
}));
vi.mock("../api", () => api);

describe("XTM AboutModal", () => {
  it("names the Xray version it targets and opens the Xray REST docs", async () => {
    const user = userEvent.setup();
    render(<AboutModal onClose={vi.fn()} />);
    const readout = screen.getByRole("group", { name: "Build and environment" });
    expect(await within(readout).findByText("v1.10.0")).toBeInTheDocument();
    expect(within(readout).getByText("Jira DC 8.14+ · Xray 8.4.0")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Documentation/ }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://docs.getxray.app/display/XRAY/REST+API");
  });

  it("offers the tour only when it can replay it, closing About first", async () => {
    const user = userEvent.setup();
    const { unmount } = render(<AboutModal onClose={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /Take the tour/ })).not.toBeInTheDocument();
    unmount();

    const calls: string[] = [];
    render(<AboutModal onClose={() => calls.push("close")} onTakeTour={() => calls.push("tour")} />);
    await user.click(screen.getByRole("button", { name: /Take the tour/ }));
    expect(calls).toEqual(["close", "tour"]);
  });
});
