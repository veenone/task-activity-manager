import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider, ProfileProvider } from "@agile-suite/core";
import * as api from "../api";
import { ProfilesModal } from "./ProfilesModal";
import { profileBackend } from "../profileBackend";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    CreateProfile: vi.fn(),
    DeleteProfile: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
  };
});

beforeEach(() => {
  vi.mocked(api.ListProfiles).mockResolvedValue([]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "", theme: "light" });
  vi.mocked(api.CreateProfile).mockResolvedValue({
    id: "new", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "",
  });
});

function renderModal(onClose = vi.fn()) {
  render(
    <DialogProvider>
      <ProfileProvider backend={profileBackend}>
        <ProfilesModal onClose={onClose} />
      </ProfileProvider>
    </DialogProvider>,
  );
  return onClose;
}

describe("ProfilesModal", () => {
  it("creates a demo profile without a token and makes it the default", async () => {
    renderModal();
    await userEvent.type(screen.getByLabelText("Name"), "Demo team");
    await userEvent.type(screen.getByLabelText("Jira URL"), "demo");
    await userEvent.type(screen.getByLabelText("Project key"), "DEMO");
    await userEvent.click(screen.getByLabelText("Make this the default profile"));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(api.CreateProfile).toHaveBeenCalledWith("Demo team", "demo", "DEMO", "", true),
    );
    // The provider reloads the list once the save went through.
    expect(api.ListProfiles).toHaveBeenCalledTimes(1);
  });

  it("shows the backend's validation message", async () => {
    vi.mocked(api.CreateProfile).mockRejectedValue(
      new Error("a live Jira profile needs a personal access token"),
    );
    renderModal();
    await userEvent.type(screen.getByLabelText("Name"), "Acme");
    await userEvent.type(screen.getByLabelText("Jira URL"), "https://jira.acme.example");
    await userEvent.type(screen.getByLabelText("Project key"), "PLAT");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("personal access token"),
    );
  });
});
