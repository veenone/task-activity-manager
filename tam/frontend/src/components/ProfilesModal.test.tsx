import React, { useEffect } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider, ProfileProvider, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { Profile, Settings } from "../api";
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
    GetProfileSetting: vi.fn(),
    SetProfileSetting: vi.fn(),
  };
});

beforeEach(() => {
  vi.mocked(api.ListProfiles).mockResolvedValue([]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "", theme: "light" });
  vi.mocked(api.CreateProfile).mockResolvedValue({
    id: "new", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "",
  });
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.SetProfileSetting).mockResolvedValue();
});

// The provider loads nothing by itself; App does the first load on mount.
// The modal is rendered on its own here, so the harness stands in for App.
function Loaded({ children }: { children: React.ReactNode }) {
  const { reload } = useProfile<Profile, Settings>();
  useEffect(() => {
    void reload();
  }, [reload]);
  return <>{children}</>;
}

function renderModal(onClose = vi.fn()) {
  render(
    <DialogProvider>
      <ProfileProvider backend={profileBackend}>
        <Loaded>
          <ProfilesModal onClose={onClose} />
        </Loaded>
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
    // Once for the harness's first load, once more after the save went through.
    expect(api.ListProfiles).toHaveBeenCalledTimes(2);
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

  it("edits the requirement issue type per profile", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([
      { id: "p1", name: "Acme Platform", jiraUrl: "https://jira.acme.example", projectKey: "PLAT", backend: "xray", createdAt: "" },
    ]);
    vi.mocked(api.GetProfileSetting).mockResolvedValue("Business Requirement");
    renderModal();
    const field = await screen.findByRole("textbox", { name: "Requirement issue type for Acme Platform" });
    await waitFor(() => expect(field).toHaveValue("Business Requirement"));
    await userEvent.clear(field);
    await userEvent.type(field, "Req");
    await userEvent.tab();
    await waitFor(() =>
      expect(api.SetProfileSetting).toHaveBeenCalledWith("p1", "requirement_issue_type", "Req"),
    );
  });
});
