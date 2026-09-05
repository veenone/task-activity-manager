import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import * as api from "./api";
import App from "./App";
import { profileBackend } from "./profileBackend";
import { ViewProvider } from "./nav";
import { ModalProvider } from "./modals";

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    Health: vi.fn(),
    GetDiagnostics: vi.fn(),
    ListProfiles: vi.fn(),
    CreateProfile: vi.fn(),
    DeleteProfile: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    EventsOn: vi.fn(() => () => {}),
    BrowserOpenURL: vi.fn(),
  };
});

function renderApp() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ViewProvider>
            <ModalProvider>
              <App />
            </ModalProvider>
          </ViewProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(api.Health).mockResolvedValue({
    ok: true, error: "", dbPath: "C:/tam.db", sharedPath: "C:/profiles.db", logPath: "C:/tam.log",
  });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
});

describe("App shell", () => {
  it("shows the title, the demo chip, and the active profile", async () => {
    renderApp();
    expect(screen.getByText("Task Activity Manager")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("DEMO")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: /profile/i })).toHaveValue("p1");
  });

  it("switches views from the nav rail and names the phase", async () => {
    renderApp();
    await userEvent.click(screen.getByRole("button", { name: "Epics" }));
    expect(screen.getByRole("heading", { name: "Epics" })).toBeInTheDocument();
    expect(screen.getByText(/arrives in Phase 2/)).toBeInTheDocument();
  });

  it("surfaces a startup failure instead of a blank page", async () => {
    vi.mocked(api.Health).mockResolvedValue({
      ok: false, error: "open local store: disk full", dbPath: "", sharedPath: "", logPath: "",
    });
    renderApp();
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("disk full"),
    );
  });
});
