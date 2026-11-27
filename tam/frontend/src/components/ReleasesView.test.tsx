import React from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, LiveRegion, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { Issue, VersionSummary } from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { ReleasesView } from "./ReleasesView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    ListVersionSummaries: vi.fn(),
    VersionIssues: vi.fn(),
    CreateVersion: vi.fn(),
    EditVersion: vi.fn(),
    ReleaseVersion: vi.fn(),
    ArchiveVersion: vi.fn(),
    DeleteVersion: vi.fn(),
    ListPendingChanges: vi.fn(),
    GetPermissions: vi.fn(),
  };
});

vi.mock("../contexts/SyncContext", () => ({
  useSync: () => ({ status: "idle", runQuietLock: async <T,>(action: () => Promise<T>) => action() }),
}));

function version(over: Partial<VersionSummary>): VersionSummary {
  return {
    id: 10, project: "PLAT", name: "v2.4", description: "", startDate: "2026-09-01", releaseDate: "2026-09-15",
    released: false, archived: false, draft: false, done: 1, total: 2, ...over,
  };
}

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "Export to CSV", status: "To Do", statusId: "1",
    assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "",
    parentKey: "", storyPoints: null, rank: "", created: "", updated: "", fixVersions: [10],
    ...over,
  };
}

const OPEN = version({});
const NEXT = version({ id: 11, name: "v2.5", startDate: "", releaseDate: "2026-10-15", done: 0, total: 0 });
const SHIPPED = version({ id: 9, name: "v2.3", released: true, startDate: "", releaseDate: "2026-08-15", done: 4, total: 4 });
const ISSUES = [issue({}), issue({ key: "PLAT-2", summary: "SSO login", status: "Done", statusCategory: "done" })];

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
            <LiveRegion />
            <ReleasesView />
          </ModalProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const banner = () => screen.getByRole("status", { name: "Release outcome" });

async function enabledButton(name: string) {
  const button = await screen.findByRole("button", { name });
  await waitFor(() => expect(button).toBeEnabled());
  return button;
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 8, 3, 12, 0, 0));
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListVersionSummaries).mockResolvedValue([SHIPPED, OPEN, NEXT]);
  vi.mocked(api.VersionIssues).mockResolvedValue(ISSUES);
  vi.mocked(api.CreateVersion).mockResolvedValue({ id: -1, project: "PLAT", name: "v2.6", description: "", startDate: "", releaseDate: "2026-11-01", released: false, archived: false, draft: true });
  vi.mocked(api.EditVersion).mockResolvedValue(undefined);
  vi.mocked(api.ReleaseVersion).mockResolvedValue(undefined);
  vi.mocked(api.ArchiveVersion).mockResolvedValue(undefined);
  vi.mocked(api.DeleteVersion).mockResolvedValue(undefined);
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
  vi.mocked(api.GetPermissions).mockResolvedValue({ project: "PLAT", byProject: {}, names: {} });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("ReleasesView", () => {
  it("lists the unreleased releases with dates and progress, and the selected one's issues", async () => {
    renderView();
    const list = await screen.findByRole("table", { name: "Releases" });
    await within(list).findByRole("button", { name: /v2\.4/ });
    expect(within(list).queryByRole("button", { name: /v2\.3/ })).toBeNull();
    const row = within(list).getByRole("button", { name: /v2\.4/ }).closest("tr")!;
    expect(row).toHaveTextContent("Unreleased");
    expect(row).toHaveTextContent("2026-09-01 to 2026-09-15");
    expect(row).toHaveTextContent("1 of 2 done");
    await screen.findByText("SSO login");
    expect(api.VersionIssues).toHaveBeenCalledWith("p1", 10);
  });

  it("shows released releases under Released", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("button", { name: "v2.4" });
    await user.selectOptions(screen.getByRole("combobox", { name: "Show" }), "released");
    expect(await screen.findByRole("button", { name: "v2.3" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "v2.4" })).toBeNull();
  });

  it("drafts a new release", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await enabledButton("New release"));
    const dialog = await screen.findByRole("dialog", { name: "New release" });
    await user.type(within(dialog).getByLabelText("Name"), "v2.6");
    await user.type(within(dialog).getByLabelText("Release date"), "2026-11-01");
    await user.click(within(dialog).getByRole("button", { name: "Create release" }));
    await waitFor(() => expect(api.CreateVersion).toHaveBeenCalledWith("p1", "v2.6", "", "", "2026-11-01"));
    expect(banner()).toHaveTextContent("v2.6 was drafted. Commit creates it in Jira.");
  });

  it("turns release actions off without Administer Projects, saying why", async () => {
    vi.mocked(api.GetPermissions).mockResolvedValue({
      project: "PLAT", byProject: { PLAT: { ADMINISTER_PROJECTS: false } }, names: { ADMINISTER_PROJECTS: "Administer Projects" },
    });
    renderView();
    await screen.findByText("SSO login");
    await waitFor(() => expect(screen.getByRole("button", { name: "New release" })).toBeDisabled());
    expect(screen.getByRole("button", { name: "New release" })).toHaveAttribute("title", expect.stringContaining("Administer Projects"));
    expect(screen.getByRole("button", { name: "Release…" })).toBeDisabled();
  });

  it("releases with a choice for each unfinished issue", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("SSO login");
    await user.click(await enabledButton("Release…"));
    const dialog = await screen.findByRole("dialog", { name: "Release v2.4" });
    // Only the unfinished issue is listed, carried over to the next release.
    const choice = within(dialog).getByRole("combobox", { name: "PLAT-1" });
    expect(within(dialog).queryByRole("combobox", { name: "PLAT-2" })).toBeNull();
    expect(choice).toHaveValue("11");
    await user.selectOptions(choice, "0");
    expect(within(dialog).getByLabelText("Release date")).toHaveValue("2026-09-03");
    await user.click(within(dialog).getByRole("button", { name: "Release" }));
    await waitFor(() => expect(api.ReleaseVersion).toHaveBeenCalledWith("p1", 10, "2026-09-03", 11, [{ key: "PLAT-1", to: 0 }]));
    expect(banner()).toHaveTextContent("v2.4 will be released on Commit.");
  });

  it("offers descope only and a new release when no other release is open", async () => {
    vi.mocked(api.ListVersionSummaries).mockResolvedValue([SHIPPED, OPEN]);
    const user = userEvent.setup();
    renderView();
    await screen.findByText("SSO login");
    await user.click(await enabledButton("Release…"));
    const dialog = await screen.findByRole("dialog", { name: "Release v2.4" });
    expect(within(dialog).getByRole("combobox", { name: "PLAT-1" })).toHaveValue("0");
    expect(within(dialog).getByText(/No other release is open/)).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "New release" })).toBeEnabled();
  });

  it("marks a release waiting for Commit and holds its release back", async () => {
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 1, entityType: "version_release", entityKey: "10", field: "release", beforeVal: "", afterVal: "{}", baseVersion: "", createdAt: "" },
    ]);
    renderView();
    const list = await screen.findByRole("table", { name: "Releases" });
    await within(list).findByText("Releasing on Commit");
    await screen.findByText("SSO login");
    expect(screen.getByRole("button", { name: "Release…" })).toBeDisabled();
  });
});
