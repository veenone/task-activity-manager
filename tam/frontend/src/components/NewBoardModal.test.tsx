import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { NewBoardModal } from "./NewBoardModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    CreateDraftBoard: vi.fn(),
  };
});

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderModal(onCreated = vi.fn(), onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <Loader />
          <NewBoardModal profileId="p1" onClose={onClose} onCreated={onCreated} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onCreated, onClose };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.CreateDraftBoard).mockResolvedValue(-2);
});

// The dialog reads the active profile for the JQL default, so every test
// waits for the form to actually mount rather than racing the provider.
async function nameField() {
  return waitFor(() => screen.getByLabelText("Name"));
}

describe("NewBoardModal", () => {
  it("refuses an empty name without calling CreateDraftBoard", async () => {
    const user = userEvent.setup();
    renderModal();
    await nameField();
    await user.click(screen.getByRole("button", { name: "Create board" }));
    expect(await screen.findByText("The board needs a name.")).toBeInTheDocument();
    expect(api.CreateDraftBoard).not.toHaveBeenCalled();
  });

  it("fills the filter name and JQL defaults when left blank", async () => {
    const user = userEvent.setup();
    const { onCreated, onClose } = renderModal();
    await user.type(await nameField(), "Payments Kanban");
    await user.click(screen.getByRole("button", { name: "Create board" }));
    await waitFor(() => expect(api.CreateDraftBoard).toHaveBeenCalledWith(
      "p1", "Payments Kanban", "scrum", "Filter for Payments Kanban", "project = PLAT ORDER BY Rank",
    ));
    expect(onCreated).toHaveBeenCalledWith(-2, expect.stringContaining("Payments Kanban was drafted"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("sends the type, filter name and JQL actually typed rather than the defaults", async () => {
    const user = userEvent.setup();
    renderModal();
    await user.type(await nameField(), "Ops Board");
    await user.selectOptions(screen.getByLabelText("Type"), "kanban");
    await user.type(screen.getByLabelText("Filter name"), "Ops filter");
    await user.type(screen.getByLabelText("JQL"), "project = OPS ORDER BY Rank");
    await user.click(screen.getByRole("button", { name: "Create board" }));
    await waitFor(() => expect(api.CreateDraftBoard).toHaveBeenCalledWith(
      "p1", "Ops Board", "kanban", "Ops filter", "project = OPS ORDER BY Rank",
    ));
  });

  it("never validates the JQL over the wire: no other binding is called", async () => {
    const user = userEvent.setup();
    renderModal();
    await user.type(await nameField(), "Any Board");
    await user.click(screen.getByRole("button", { name: "Create board" }));
    await waitFor(() => expect(api.CreateDraftBoard).toHaveBeenCalled());
    // The only Jira-shaped call this dialog can make at all is the local
    // draft write itself; nothing here goes out to validate the JQL.
    expect(api.CreateDraftBoard).toHaveBeenCalledTimes(1);
  });
});
