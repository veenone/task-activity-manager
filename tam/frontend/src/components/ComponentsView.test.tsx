import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, LiveRegion, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { ComponentSummary, Issue } from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { ComponentsView } from "./ComponentsView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    ListComponentSummaries: vi.fn(),
    ComponentIssues: vi.fn(),
    CreateComponent: vi.fn(),
    EditComponent: vi.fn(),
    DeleteComponent: vi.fn(),
    SearchUsers: vi.fn(),
    ListPendingChanges: vi.fn(),
    GetPermissions: vi.fn(),
  };
});

vi.mock("../contexts/SyncContext", () => ({
  useSync: () => ({ status: "idle", runQuietLock: async <T,>(action: () => Promise<T>) => action() }),
}));

function component(over: Partial<ComponentSummary>): ComponentSummary {
  return { id: 10000, project: "PLAT", name: "Auth", description: "Login and sessions", leadName: "alee", leadDisplayName: "A. Lee", assigneeType: "COMPONENT_LEAD", draft: false, issues: 2, ...over };
}

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "SSO login", status: "To Do", statusId: "1",
    assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "",
    parentKey: "", storyPoints: null, rank: "", created: "", updated: "", components: [10000],
    ...over,
  };
}

const AUTH = component({});
const BOARD = component({ id: 10001, name: "Board", description: "", leadName: "", leadDisplayName: "", assigneeType: "PROJECT_DEFAULT", issues: 0 });

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
            <ComponentsView />
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
  vi.mocked(api.ListComponentSummaries).mockResolvedValue([AUTH, BOARD]);
  vi.mocked(api.ComponentIssues).mockResolvedValue([issue({}), issue({ key: "PLAT-2", summary: "Password reset" })]);
  vi.mocked(api.DeleteComponent).mockResolvedValue(undefined);
  vi.mocked(api.SearchUsers).mockResolvedValue([]);
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
  vi.mocked(api.GetPermissions).mockResolvedValue({ project: "PLAT", byProject: {}, names: {} });
});

describe("ComponentsView", () => {
  it("lists the components with their issue counts and the selected one's issues", async () => {
    renderView();
    const auth = await screen.findByRole("button", { name: /^Auth/ });
    expect(auth).toHaveTextContent("2 issues");
    expect(screen.getByRole("button", { name: /^Board/ })).toHaveTextContent("0 issues");
    const detail = screen.getByRole("heading", { name: "Auth" }).closest(".components-detail") as HTMLElement;
    expect(detail).toHaveTextContent("A. Lee");
    expect(detail).toHaveTextContent("Component lead");
    expect(await screen.findByText("Password reset")).toBeInTheDocument();
    expect(api.ComponentIssues).toHaveBeenCalledWith("p1", 10000);
  });

  it("queues a delete that moves the issues to another component", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Password reset");
    await user.click(screen.getByRole("button", { name: "Delete Auth" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Delete Auth" });
    expect(dialog).toHaveTextContent("2 issues TAM holds use Auth.");
    await user.selectOptions(within(dialog).getByLabelText("Move its issues to"), "10001");
    await user.click(within(dialog).getByRole("button", { name: "Delete component" }));
    await waitFor(() => expect(api.DeleteComponent).toHaveBeenCalledWith("p1", 10000, 10001));
    expect(screen.getByRole("status", { name: "Component outcome" })).toHaveTextContent("Auth will be deleted on Commit.");
  });

  it("turns the writes off without Administer Projects, saying why", async () => {
    vi.mocked(api.GetPermissions).mockResolvedValue({
      project: "PLAT", byProject: { PLAT: { ADMINISTER_PROJECTS: false } }, names: { ADMINISTER_PROJECTS: "Administer Projects" },
    });
    renderView();
    await screen.findByText("Password reset");
    await waitFor(() => expect(screen.getByRole("button", { name: "New component" })).toBeDisabled());
    expect(screen.getByRole("button", { name: "New component" })).toHaveAttribute("title", expect.stringContaining("Administer Projects"));
    expect(screen.getByRole("button", { name: "Edit Auth" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete Auth" })).toBeDisabled();
  });

  it("marks a component waiting for Commit", async () => {
    vi.mocked(api.ListPendingChanges).mockResolvedValue([
      { id: 1, entityType: "component_delete", entityKey: "10000", field: "delete", beforeVal: "Auth", afterVal: "{}", baseVersion: "", createdAt: "" },
    ]);
    renderView();
    const auth = await screen.findByRole("button", { name: /^Auth/ });
    await waitFor(() => expect(auth).toHaveTextContent("Deleting on Commit"));
    expect(screen.getByRole("button", { name: "Delete Auth" })).toBeDisabled();
  });
});
