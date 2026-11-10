import { beforeEach, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import App from "./App";
import * as api from "../wailsjs/go/main/App";

const handlers = new Map<string, (...data: unknown[]) => void>();
vi.mock("../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, fn: (...data: unknown[]) => void) => {
    handlers.set(name, fn);
    return () => handlers.delete(name);
  },
  BrowserOpenURL: vi.fn(),
}));

vi.mock("../wailsjs/go/main/App", () => ({
  StartupError: vi.fn(),
  ListProfiles: vi.fn(),
  DefaultProfile: () => Promise.resolve(""),
  ListQueries: vi.fn(),
  Fields: vi.fn(),
  ListCases: vi.fn(),
  LastRun: vi.fn(),
  SetTheme: vi.fn(),
  GetDiagnostics: vi.fn(),
}));

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.StartupError).mockResolvedValue("");
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Q", jiraUrl: "demo" }] as never);
  vi.mocked(api.ListQueries).mockResolvedValue([]);
  vi.mocked(api.Fields).mockResolvedValue({ productBase: "", clientVisibility: "", clientImpact: "" } as never);
  vi.mocked(api.ListCases).mockResolvedValue([]);
  document.documentElement.dataset.theme = "light";
});

function show() {
  render(
    <DialogProvider>
      <App />
    </DialogProvider>,
  );
  return userEvent.setup();
}

const emit = (name: string, ...data: unknown[]) => act(() => handlers.get(name)?.(...data));

it("switches tabs from the View menu", async () => {
  show();
  await screen.findByRole("navigation", { name: "Views" });
  emit("menu:view", "links");
  expect(screen.getByRole("button", { name: "Link map" })).toHaveAttribute("aria-current", "page");
});

it("opens Manage profiles and Jira fields from the File menu", async () => {
  show();
  await screen.findByRole("navigation", { name: "Views" });
  // The menu item does nothing until the profile's fields have loaded.
  await waitFor(() => expect(screen.getByRole("button", { name: "Jira fields" })).toBeEnabled());
  emit("menu:fields");
  expect(await screen.findByRole("heading", { name: "Jira fields" })).toBeVisible();
  await userEvent.setup().click(screen.getByRole("button", { name: "Cancel" }));
  emit("menu:profiles");
  expect(await screen.findByRole("heading", { name: "Manage Profiles" })).toBeVisible();
});

it("toggles the theme from the top bar and stores the choice", async () => {
  vi.mocked(api.SetTheme).mockResolvedValue();
  const user = show();
  await user.click(await screen.findByRole("button", { name: "Switch to dark theme" }));
  expect(api.SetTheme).toHaveBeenCalledWith("dark");
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(screen.getByRole("button", { name: "Switch to light theme" })).toBeVisible();
});

it("applies a theme chosen in the View menu", async () => {
  show();
  await screen.findByRole("navigation", { name: "Views" });
  emit("menu:theme", "dark");
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(screen.getByRole("button", { name: "Switch to light theme" })).toBeVisible();
});

it("opens the shared About box with PIE's version and runtime", async () => {
  vi.mocked(api.GetDiagnostics).mockResolvedValue({
    version: "0.1.0",
    dbPath: "/home/analyst/pie.db",
    logPath: "",
    os: "windows",
    arch: "amd64",
    goVersion: "go1.25.0",
    schemaVersion: 4,
    profileCount: 1,
    startupError: "",
  } as never);
  show();
  await screen.findByRole("navigation", { name: "Views" });
  emit("menu:about");
  const about = await screen.findByRole("dialog", { name: "About Project Issue Explorer" });
  expect(await within(about).findByText("v0.1.0")).toBeVisible();
  expect(within(about).getByText("/home/analyst/pie.db")).toBeInTheDocument();
});
