import React, { useEffect } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
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
    CreateProfileReusingToken: vi.fn(),
    UpdateProfile: vi.fn(),
    DeleteProfile: vi.fn(),
    ExportProfile: vi.fn(),
    ImportProfile: vi.fn(),
    TestConnection: vi.fn(),
    TestProfileConnection: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    GetProfileSetting: vi.fn(),
    SetProfileSetting: vi.fn(),
  };
});

const acme: Profile = {
  id: "p1",
  name: "Acme Platform",
  jiraUrl: "https://jira.acme.example",
  projectKey: "PLAT",
  backend: "xray",
  createdAt: "",
  scopeJql: "",
  caCert: "",
  allowUntrustedTls: false,
};

beforeEach(() => {
  // Call history only; the resolved values below survive it. Without this the
  // reload counts leak from one test into the next.
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "", theme: "light" });
  vi.mocked(api.CreateProfile).mockResolvedValue({
    id: "new", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "",
  });
  vi.mocked(api.UpdateProfile).mockResolvedValue(acme);
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

// The start state offers Create in two places (the list's CTA and the empty
// pane); the CTA is the first in the document, and either opens the same form.
async function startCreate() {
  const buttons = await screen.findAllByRole("button", { name: "Create new profile" });
  await userEvent.click(buttons[0]);
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
  it("opens on the start state, editing nothing", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([acme]);
    renderModal();
    expect(await screen.findByText("Nothing open yet")).toBeInTheDocument();
    expect(screen.queryByLabelText("Profile name")).not.toBeInTheDocument();
  });

  it("creates a demo profile without a token", async () => {
    renderModal();
    await startCreate();
    await userEvent.type(screen.getByLabelText("Profile name"), "Demo team");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "demo");
    await userEvent.type(screen.getByLabelText("Project key"), "DEMO");
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    await waitFor(() =>
      expect(api.CreateProfile).toHaveBeenCalledWith("Demo team", "demo", "DEMO", "", "", "", false),
    );
    // Once for the harness's first load, once more after the save went through.
    await waitFor(() => expect(api.ListProfiles).toHaveBeenCalledTimes(2));
  });

  it("shows the backend's validation message", async () => {
    vi.mocked(api.CreateProfile).mockRejectedValue(
      new Error("a live Jira profile needs a personal access token"),
    );
    renderModal();
    await startCreate();
    await userEvent.type(screen.getByLabelText("Profile name"), "Acme");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "https://jira.acme.example");
    await userEvent.type(screen.getByLabelText("Project key"), "PLAT");
    await userEvent.type(screen.getByLabelText("Personal Access Token"), "tok");
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("personal access token"),
    );
  });

  it("rejects a project key with a trailing slash before it reaches the backend", async () => {
    renderModal();
    await startCreate();
    await userEvent.type(screen.getByLabelText("Project key"), "PLAT/");
    expect(await screen.findByText(/must start with a letter/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create profile" })).toBeDisabled();
  });

  it("edits a profile it was opened on, keeping the stored token when the field is blank", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([acme]);
    vi.mocked(api.GetProfileSetting).mockResolvedValue("Business Requirement");
    renderModal();
    await userEvent.click(await screen.findByText("Acme Platform"));
    const reqType = screen.getByLabelText(/Requirement issue type/);
    await waitFor(() => expect(reqType).toHaveValue("Business Requirement"));
    await userEvent.clear(screen.getByLabelText("Scope JQL (optional)"));
    await userEvent.type(screen.getByLabelText("Scope JQL (optional)"), "labels = team-a");
    await userEvent.clear(reqType);
    await userEvent.type(reqType, "Req");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() =>
      expect(api.UpdateProfile).toHaveBeenCalledWith(
        "p1", "Acme Platform", "https://jira.acme.example", "PLAT",
        "labels = team-a", "", "", false,
      ),
    );
    expect(api.SetProfileSetting).toHaveBeenCalledWith("p1", "requirement_issue_type", "Req");
  });

  it("warns that changing the project key clears the cache", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([acme]);
    renderModal();
    await userEvent.click(await screen.findByText("Acme Platform"));
    await userEvent.clear(screen.getByLabelText("Project key"));
    await userEvent.type(screen.getByLabelText("Project key"), "OTHER");
    expect(screen.getByText(/clears this profile's cached issues/)).toBeInTheDocument();
  });

  it("tests a saved profile with its stored token", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([acme]);
    vi.mocked(api.TestProfileConnection).mockResolvedValue("Ada Lovelace");
    renderModal();
    await userEvent.click(await screen.findByText("Acme Platform"));
    await userEvent.click(screen.getByRole("button", { name: "Test connection" }));
    await waitFor(() =>
      expect(api.TestProfileConnection).toHaveBeenCalledWith(
        "p1", "https://jira.acme.example", "", false,
      ),
    );
    expect(await screen.findByText("Connected as Ada Lovelace")).toBeInTheDocument();
  });

  it("offers to reuse a stored token when creating beside an existing profile", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([acme]);
    vi.mocked(api.CreateProfileReusingToken).mockResolvedValue({ ...acme, id: "p2" });
    renderModal();
    await startCreate();
    await userEvent.selectOptions(
      screen.getByLabelText("Personal Access Token"),
      "p1",
    );
    await userEvent.type(screen.getByLabelText("Profile name"), "Acme Ops");
    await userEvent.type(screen.getByLabelText("Jira base URL"), "https://jira.acme.example");
    await userEvent.type(screen.getByLabelText("Project key"), "OPS");
    await userEvent.click(screen.getByRole("button", { name: "Create profile" }));
    await waitFor(() =>
      expect(api.CreateProfileReusingToken).toHaveBeenCalledWith(
        "Acme Ops", "https://jira.acme.example", "OPS", "", "p1",
      ),
    );
  });

  it("deletes a profile after confirming, saying it goes from XTM too", async () => {
    vi.mocked(api.ListProfiles).mockResolvedValue([acme]);
    vi.mocked(api.DeleteProfile).mockResolvedValue();
    renderModal();
    await userEvent.click(await screen.findByText("Acme Platform"));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog", { name: /Delete profile/ });
    expect(dialog).toHaveTextContent("Xray Test Manager");
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete profile" }));
    await waitFor(() => expect(api.DeleteProfile).toHaveBeenCalledWith("p1"));
  });

  it("opens an imported profile so a token can be entered", async () => {
    vi.mocked(api.ImportProfile).mockResolvedValue({ ...acme, id: "p9", name: "Imported" });
    vi.mocked(api.ListProfiles)
      .mockResolvedValueOnce([])
      .mockResolvedValue([{ ...acme, id: "p9", name: "Imported" }]);
    renderModal();
    await userEvent.click(await screen.findByRole("button", { name: "Import from file…" }));
    await waitFor(() => expect(screen.getByLabelText("Profile name")).toHaveValue("Imported"));
  });

  it("stays on the start state when the import dialog is cancelled", async () => {
    vi.mocked(api.ImportProfile).mockResolvedValue({
      id: "", name: "", jiraUrl: "", projectKey: "", backend: "", createdAt: "",
    });
    renderModal();
    await userEvent.click(await screen.findByRole("button", { name: "Import from file…" }));
    expect(await screen.findByText("Nothing open yet")).toBeInTheDocument();
  });
});
