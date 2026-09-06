import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { NewIssueModal } from "./NewIssueModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    GetCreateFields: vi.fn(),
    CreateIssue: vi.fn(),
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
          <NewIssueModal onClose={onClose} onCreated={onCreated} />
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
  vi.mocked(api.GetCreateFields).mockImplementation(async (_p, type) =>
    type === "bug"
      ? [{ id: "customfield_10050", name: "Severity", type: "option", required: true, allowedValues: [{ id: "1", value: "Minor" }, { id: "3", value: "Critical" }] }]
      : [],
  );
  vi.mocked(api.CreateIssue).mockResolvedValue("TAM-NEW-1");
});

describe("NewIssueModal", () => {
  it("creates a task from the minimal form", async () => {
    const user = userEvent.setup();
    const { onCreated, onClose } = renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    await user.type(within(dialog).getByLabelText("Summary"), "Add a retry to the payment webhook consumer");
    await user.type(within(dialog).getByLabelText("Labels"), "payments, webhooks");
    await user.type(within(dialog).getByLabelText("Story points"), "3");
    await user.type(within(dialog).getByLabelText("Assignee"), "mortiz");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    await waitFor(() => expect(api.CreateIssue).toHaveBeenCalledWith("p1", {
      type: "task", summary: "Add a retry to the payment webhook consumer", description: "", priority: "",
      labels: ["payments", "webhooks"], assignee: "mortiz", storyPoints: 3, extra: {},
    }));
    expect(onCreated).toHaveBeenCalledWith("TAM-NEW-1");
    expect(onClose).toHaveBeenCalled();
  });

  it("asks for the type's required create-meta fields and sends them as extra", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    await user.selectOptions(within(dialog).getByLabelText("Type"), "bug");
    const severity = await within(dialog).findByLabelText("Severity");
    await user.type(within(dialog).getByLabelText("Summary"), "Promo field accepts spaces");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    expect(await within(dialog).findByText("Severity is required.")).toBeInTheDocument();
    expect(api.CreateIssue).not.toHaveBeenCalled();
    await user.selectOptions(severity, "3");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    await waitFor(() => expect(api.CreateIssue).toHaveBeenCalled());
    expect(vi.mocked(api.CreateIssue).mock.calls[0][1].extra).toEqual({ customfield_10050: "3" });
    expect(vi.mocked(api.CreateIssue).mock.calls[0][1].type).toBe("bug");
  });

  it("degrades to the minimal form when create-meta cannot be read", async () => {
    const user = userEvent.setup();
    vi.mocked(api.GetCreateFields).mockRejectedValue(new Error("GET failed: 403"));
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    expect(await within(dialog).findByText(/Jira's required fields could not be read/)).toBeInTheDocument();
    await user.type(within(dialog).getByLabelText("Summary"), "Still works");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    await waitFor(() => expect(api.CreateIssue).toHaveBeenCalled());
  });

  it("refuses a blank summary and shows the backend's error", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", { name: "New issue" });
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    expect(await within(dialog).findByText("Summary cannot be empty.")).toBeInTheDocument();
    vi.mocked(api.CreateIssue).mockRejectedValueOnce(new Error("summary cannot be empty"));
    await user.type(within(dialog).getByLabelText("Summary"), "x");
    await user.click(within(dialog).getByRole("button", { name: "Create draft" }));
    expect(await within(dialog).findByText(/summary cannot be empty/)).toBeInTheDocument();
  });
});
