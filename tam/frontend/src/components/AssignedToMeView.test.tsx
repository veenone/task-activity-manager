import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { Issue } from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { AssignedToMeView } from "./AssignedToMeView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    ListIssues: vi.fn(),
    ListSprints: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
    ListOpenSprints: vi.fn(),
    GetSubtaskTypeName: vi.fn(),
    GetProfileSetting: vi.fn(),
  };
});

// AssignedToMeView's own Sync button reads canSync/runSync; nothing else it
// renders touches the reducer, so the mock stays minimal like BacklogView's.
const runSync = vi.fn();
vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ canSync: true, runSync }) }));

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "",
    assigneeName: "rahmad", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "",
    storyPoints: null, rank: "", created: "", updated: "",
    ...over,
  };
}

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderView() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ModalProvider>
            <Loader />
            <AssignedToMeView />
          </ModalProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: "PLAT-412", description: "", links: [], fields: {}, comments: [], commentTotal: 0, commentsTruncated: false, fetchedAt: "" });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Technical task");
});

describe("AssignedToMeView", () => {
  it("names the connected user in the header and lists only their issues", async () => {
    vi.mocked(api.GetProfileSetting).mockImplementation((_id, key) =>
      Promise.resolve(key === "jira_username" ? "rahmad" : key === "jira_display_name" ? "R. Anand" : ""),
    );
    const rows = [
      issue({ key: "PLAT-412", summary: "Checkout: apply promo code" }),
      issue({ key: "PLAT-409", summary: "Rotate payment gateway API keys" }),
    ];
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: rows, total: 2 });

    renderView();

    expect(await screen.findByText("Assigned to rahmad (R. Anand) · 2 issues")).toBeInTheDocument();
    const grid = screen.getByRole("treegrid", { name: "Issues" });
    expect(within(grid).getByRole("row", { name: /PLAT-412/ })).toBeInTheDocument();
    expect(within(grid).getByRole("row", { name: /PLAT-409/ })).toBeInTheDocument();
    // The filter narrows by username: prove it reached the binding.
    expect(vi.mocked(api.ListIssues).mock.calls.at(-1)?.[1]).toMatchObject({
      assigneeName: "rahmad",
      assigneeDisplayName: "R. Anand",
    });
  });

  it("explains itself when there is no profile at all, rather than rendering nothing", async () => {
    // No profile means both settings queries are disabled, and a disabled
    // query stays pending forever, so a bare isPending guard would leave the
    // whole pane blank with nothing to explain it. A fresh install reaches
    // exactly this state.
    vi.mocked(api.ListProfiles).mockResolvedValue([]);
    vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "", theme: "light" });
    vi.mocked(api.GetProfileSetting).mockResolvedValue("");
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });

    renderView();

    expect(
      await screen.findByText("TAM does not know your Jira user yet. Sync once or test the connection."),
    ).toBeInTheDocument();
  });

  it("shows the no-username empty state with both actions, and no grid", async () => {
    vi.mocked(api.GetProfileSetting).mockResolvedValue("");
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });

    renderView();

    expect(
      await screen.findByText("TAM does not know your Jira user yet. Sync once or test the connection."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sync" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Test connection" })).toBeEnabled();
    expect(screen.queryByRole("treegrid", { name: "Issues" })).not.toBeInTheDocument();
    expect(api.ListIssues).not.toHaveBeenCalled();
  });

  it("shows the stale-rows note when a listed row still lacks assigneeName", async () => {
    vi.mocked(api.GetProfileSetting).mockImplementation((_id, key) =>
      Promise.resolve(key === "jira_username" ? "rahmad" : key === "jira_display_name" ? "R. Anand" : ""),
    );
    const stale = [
      issue({ key: "PLAT-412", assigneeName: "" }),
      issue({ key: "PLAT-409", assigneeName: "rahmad" }),
    ];
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: stale, total: 2 });

    renderView();

    expect(
      await screen.findByText("Showing matches by display name until the next sync"),
    ).toBeInTheDocument();
  });

  it("hides the stale-rows note once every listed row has an assigneeName", async () => {
    vi.mocked(api.GetProfileSetting).mockImplementation((_id, key) =>
      Promise.resolve(key === "jira_username" ? "rahmad" : key === "jira_display_name" ? "R. Anand" : ""),
    );
    const fresh = [issue({ key: "PLAT-412", assigneeName: "rahmad" }), issue({ key: "PLAT-409", assigneeName: "rahmad" })];
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: fresh, total: 2 });

    renderView();

    await screen.findByText("Assigned to rahmad (R. Anand) · 2 issues");
    expect(screen.queryByText("Showing matches by display name until the next sync")).not.toBeInTheDocument();
  });
});
