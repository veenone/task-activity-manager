import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "./App";
import { DialogProvider } from "./contexts/DialogContext";
import { ProfileProvider } from "./contexts/ProfileContext";
import { SyncProvider } from "./contexts/SyncContext";
import { SelectionProvider } from "./contexts/SelectionContext";
import { NavProvider } from "./contexts/NavContext";
import { ModalProvider } from "./contexts/ModalContext";
import { Health, ListProfiles, SyncProfile, SyncProfileFull } from "./api";

// The native menu talks to the frontend through EventsOn; the tests keep the
// handlers so they can play the part of the menu.
const menu = vi.hoisted(() => ({ handlers: {} as Record<string, (...args: unknown[]) => void> }));

// App is the shell: it waits for the backend, loads profiles, and lays out the
// top bar, the view host, the dialogs and the status bar. These tests pin what
// the shell itself decides; the views and dialogs it hosts are stubbed.

vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  Health: vi.fn(async () => ({ ok: true, error: "", dbPath: "", logPath: "" })),
  ListProfiles: vi.fn(async () => [
    { id: "p1", name: "Alpha", projectKey: "AL", jiraUrl: "https://jira.example.com" },
  ]),
  GetSettings: vi.fn(async () => ({
    defaultProfileId: "p1",
    theme: "light",
    showCoverage: false,
    showNavRail: false,
    tourSeenVersion: 99,
  })),
  ListPendingChanges: vi.fn(async () => [
    { id: 1, entityType: "test_case", entityKey: "QA-1", field: "summary" },
    { id: 2, entityType: "test_step", entityKey: "QA-1:3", field: "action" },
  ]),
  GetSyncState: vi.fn(async () => ({ testCount: 1234, lastSyncedAt: "" })),
  ListFolders: vi.fn(async () => []),
  ListContainers: vi.fn(async () => []),
  ListComponents: vi.fn(async () => []),
  GetCapabilities: vi.fn(async () => {
    throw new Error("use defaults");
  }),
  EventsOn: vi.fn((event: string, cb: (...args: unknown[]) => void) => {
    menu.handlers[event] = cb;
    return () => {};
  }),
  SyncProfile: vi.fn(async () => {}),
  SyncProfileFull: vi.fn(async () => {}),
  SetTourSeenVersion: vi.fn(async () => {}),
}));

vi.mock("./components/TestTable", () => ({ TestTable: () => <div>test table</div> }));
vi.mock("./components/Dashboard", () => ({ Dashboard: () => <div>dashboard view</div> }));
vi.mock("./components/PendingChangesModal", () => ({
  PendingChangesModal: ({ changes }: { changes: unknown[] }) => (
    <div role="dialog">{changes.length} changes to review</div>
  ),
}));

const mock = (fn: unknown) => fn as ReturnType<typeof vi.fn>;

function renderApp() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DialogProvider>
        <ProfileProvider>
          <SyncProvider>
            <NavProvider>
              <SelectionProvider>
                <ModalProvider>
                  <App />
                </ModalProvider>
              </SelectionProvider>
            </NavProvider>
          </SyncProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

describe("App shell", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  it("shows the backend error when startup fails", async () => {
    mock(Health).mockResolvedValueOnce({
      ok: false,
      error: "database is locked",
      dbPath: "C:/xtm.db",
      logPath: "",
    });
    renderApp();
    expect(await screen.findByText("Backend failed to start")).toBeInTheDocument();
    expect(screen.getByText("database is locked")).toBeInTheDocument();
    expect(screen.getByText("C:/xtm.db")).toBeInTheDocument();
  });

  it("asks for a first profile when none exist", async () => {
    mock(ListProfiles).mockResolvedValueOnce([]);
    renderApp();
    await waitFor(() => expect(ListProfiles).toHaveBeenCalled());
    expect(
      await screen.findByText("Connect to your Jira Data Center project to get started."),
    ).toBeInTheDocument();
  });

  it("lists profiles and opens the pending dialog from the pending count", async () => {
    const user = userEvent.setup();
    renderApp();
    expect(await screen.findByRole("option", { name: "Alpha (AL)" })).toBeInTheDocument();
    await user.click(await screen.findByRole("button", { name: /2 pending/ }));
    expect(screen.getByRole("dialog")).toHaveTextContent("2 changes to review");
  });

  it("syncs the active profile from the Sync button", async () => {
    const user = userEvent.setup();
    renderApp();
    await user.click(await screen.findByRole("button", { name: "Sync" }));
    await waitFor(() => expect(SyncProfile).toHaveBeenCalledWith("p1"));
  });

  it("runs a full resync from the More menu after confirming", async () => {
    const user = userEvent.setup();
    renderApp();
    await user.click(await screen.findByRole("button", { name: /More/ }));
    await user.click(screen.getByRole("menuitem", { name: /Full resync/ }));
    await user.click(await screen.findByRole("button", { name: "Continue" }));
    await waitFor(() => expect(SyncProfileFull).toHaveBeenCalledWith("p1"));
  });

  it("switches views from the tab bar and marks the current tab", async () => {
    const user = userEvent.setup();
    renderApp();
    expect(await screen.findByText("test table")).toBeInTheDocument();
    const tabs = within(screen.getByRole("navigation", { name: "Views" }));
    await user.click(tabs.getByRole("button", { name: "Dashboard" }));
    expect(screen.getByText("dashboard view")).toBeInTheDocument();
    expect(screen.queryByText("test table")).not.toBeInTheDocument();
    expect(tabs.getByRole("button", { name: "Dashboard" })).toHaveAttribute("aria-current", "page");
    expect(tabs.getByRole("button", { name: "Browse" })).not.toHaveAttribute("aria-current");
  });

  it("switches views from the View menu", async () => {
    renderApp();
    expect(await screen.findByText("test table")).toBeInTheDocument();
    act(() => menu.handlers["menu:view"]("dashboard"));
    expect(await screen.findByText("dashboard view")).toBeInTheDocument();
  });

  // The menu is built once in Go and lists every view; which ones this
  // profile shows is the frontend's call, so it ignores the rest.
  it("ignores a View menu id that is unknown or hidden for this profile", async () => {
    renderApp();
    expect(await screen.findByText("test table")).toBeInTheDocument();
    act(() => menu.handlers["menu:view"]("nonsense"));
    act(() => menu.handlers["menu:view"]("coverage"));
    expect(screen.getByText("test table")).toBeInTheDocument();
    const tabs = within(screen.getByRole("navigation", { name: "Views" }));
    expect(tabs.getByRole("button", { name: "Browse" })).toHaveAttribute("aria-current", "page");
  });

  // The dot is a shape the stylesheet draws; a glyph inside it showed as a
  // square under the button's background (#255).
  it("marks the pending button with an empty dot, not a glyph", async () => {
    renderApp();
    const button = await screen.findByRole("button", { name: "2 pending" });
    expect(button.querySelector(".pending-dot")?.textContent).toBe("");
  });

  it("shows the synced test count in the status bar", async () => {
    const { container } = renderApp();
    const count = (1234).toLocaleString();
    await waitFor(() =>
      expect(container.querySelector("footer")).toHaveTextContent(
        `${count} tests · never synced`,
      ),
    );
  });
});
