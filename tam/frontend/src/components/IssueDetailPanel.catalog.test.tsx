// The issue panel's Components and Fix versions fields (#605): picks are
// journaled through Save edit as catalog ids, a shipped release cannot be
// picked again, and the fields lock with the reason when Jira would refuse.
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import { profileBackend } from "../profileBackend";
import * as api from "../api";
import type { Issue } from "../api";
import { IssueDetailPanel } from "./IssueDetailPanel";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual, GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn(), EditIssue: vi.fn(), ListActivity: vi.fn(), GetLinkTypes: vi.fn(),
    ListEpics: vi.fn(), SearchUsers: vi.fn(), ListPriorities: vi.fn(), GetSubtaskTypeName: vi.fn(), GetEditableFields: vi.fn(),
    ListWorklogs: vi.fn(), ListRitualDocuments: vi.fn(), DoneAgreementTicks: vi.fn(), BrowserOpenURL: vi.fn(),
    GetVersions: vi.fn(), GetComponents: vi.fn(), GetPermissions: vi.fn(),
  };
});
vi.mock("../contexts/SyncContext", () => ({ useSync: vi.fn(() => ({ status: "idle" })) }));

const story: Issue = {
  key: "PLAT-412", id: "1", project: "PLAT", type: "story", summary: "Promo code at payment", status: "In Progress",
  assignee: "", reporter: "PO", priority: "High", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null,
  rank: "", created: "", updated: "", description: "", components: [10000], fixVersions: [],
};

const version = (id: number, name: string, released = false): api.Version => ({
  id, project: "PLAT", name, description: "", startDate: "", releaseDate: "", released, archived: false, draft: false,
});

function renderPanel(issue: Issue = story) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <IssueDetailPanel profileId="p1" issue={issue} onClose={vi.fn()} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: story.key, description: "", links: [], fields: {}, comments: [], commentTotal: 0, commentsTruncated: false, fetchedAt: "" });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
  vi.mocked(api.EditIssue).mockResolvedValue();
  vi.mocked(api.ListActivity).mockResolvedValue([]);
  vi.mocked(api.GetLinkTypes).mockResolvedValue([]);
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.SearchUsers).mockResolvedValue([]);
  vi.mocked(api.ListPriorities).mockResolvedValue(["High"]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
  vi.mocked(api.GetEditableFields).mockResolvedValue([]);
  vi.mocked(api.GetComponents).mockResolvedValue([
    { id: 10000, project: "PLAT", name: "Auth", description: "", leadName: "", leadDisplayName: "", assigneeType: "", draft: false },
    { id: 10001, project: "PLAT", name: "Board", description: "", leadName: "", leadDisplayName: "", assigneeType: "", draft: false },
  ]);
  vi.mocked(api.GetVersions).mockResolvedValue([version(10100, "v1.2"), version(10101, "v1.1", true)]);
  vi.mocked(api.GetPermissions).mockResolvedValue({ project: "PLAT", byProject: {}, names: {} });
});

describe("IssueDetailPanel catalog fields", () => {
  it("journals picked components as ids through Save edit", async () => {
    const user = userEvent.setup();
    renderPanel();
    const components = await screen.findByRole("button", { name: "Components: Auth" });
    await waitFor(() => expect(components).toBeEnabled());
    await user.click(components);
    await user.click(await screen.findByRole("checkbox", { name: "Board" }));
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    await waitFor(() => expect(api.EditIssue).toHaveBeenCalledWith("p1", "PLAT-412", "components", "10000, 10001"));
  });

  it("offers a released version but not to pick", async () => {
    const user = userEvent.setup();
    renderPanel();
    const fix = await screen.findByRole("button", { name: "Fix versions: None" });
    await waitFor(() => expect(fix).toBeEnabled());
    await user.click(fix);
    expect(await screen.findByRole("checkbox", { name: "v1.1 (released)" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "v1.2" })).toBeEnabled();
  });

  it("turns Fix versions off without Resolve Issues, and says why", async () => {
    vi.mocked(api.GetPermissions).mockResolvedValue({ project: "PLAT", byProject: { PLAT: { RESOLVE_ISSUES: false } }, names: { RESOLVE_ISSUES: "Resolve Issues" } });
    renderPanel();
    expect(await screen.findByText('Your Jira account does not have the "Resolve Issues" permission in PLAT.')).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Fix versions: None" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Components: Auth" })).toBeEnabled();
  });

  it("shows another project's fields read-only", async () => {
    renderPanel({ ...story, key: "OPS-7", project: "OPS" });
    expect(await screen.findByText("OPS is not this profile's project, so its components are set in Jira.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Components: / })).toBeDisabled();
  });
});
