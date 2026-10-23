import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import { profileBackend } from "../profileBackend";
import * as api from "../api";
import { DashboardsView } from "./DashboardsView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    ListDashboards: vi.fn(),
    ListJiraFilters: vi.fn(),
    CreateDashboard: vi.fn(),
    RefreshDashboard: vi.fn(),
    DeleteDashboard: vi.fn(),
  };
});

const snapshot = (over: Partial<api.DashboardSnapshot> = {}): api.DashboardSnapshot => ({
  total: 12,
  points: 34,
  estimateSeconds: 28800,
  spentSeconds: 7200,
  byStatus: [{ name: "In Progress", count: 7 }, { name: "Done", count: 5 }],
  byType: [{ name: "story", count: 8 }, { name: "bug", count: 4 }],
  byAssignee: [{ name: "R. Anand", count: 9 }, { name: "Unassigned", count: 3 }],
  capped: false,
  ...over,
});

const dashboard = (over: Partial<api.Dashboard> = {}): api.Dashboard => ({
  id: "d1",
  name: "My open bugs",
  filterId: "10100",
  jql: "type = Bug AND resolution = Unresolved",
  refreshedAt: "2026-10-02T09:00:00Z",
  snapshot: snapshot(),
  ...over,
});

// The view reads the active profile, which the provider only has once it
// has loaded one. The app does that at startup; here one component does.
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
          <Loader />
          <DashboardsView />
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
  vi.mocked(api.ListDashboards).mockResolvedValue([dashboard()]);
  vi.mocked(api.ListJiraFilters).mockResolvedValue([
    { id: "10100", name: "My open bugs", jql: "type = Bug AND resolution = Unresolved", scope: "owned" },
    { id: "10101", name: "Platform this quarter", jql: "project in (PLAT, OPS)", scope: "shared" },
  ]);
  vi.mocked(api.CreateDashboard).mockResolvedValue(dashboard({ id: "d2", name: "Platform this quarter" }));
  vi.mocked(api.RefreshDashboard).mockResolvedValue(dashboard({ snapshot: snapshot({ total: 15 }) }));
  vi.mocked(api.DeleteDashboard).mockResolvedValue();
});

describe("DashboardsView", () => {
  it("draws a dashboard's figures and the rankings behind them", async () => {
    renderView();
    const panel = await screen.findByRole("region", { name: "My open bugs" });
    expect(within(panel).getByText("12")).toBeInTheDocument();
    expect(within(panel).getByText("34")).toBeInTheDocument();
    // The status ranking reads biggest first.
    const statuses = within(panel).getByRole("table", { name: /status/i });
    expect(within(statuses).getAllByRole("row")[1]).toHaveTextContent("In Progress");
    expect(within(statuses).getAllByRole("row")[1]).toHaveTextContent("7");
  });

  // A number with no date on it reads as today's, which is the one thing a
  // snapshot must not be mistaken for.
  it("says when the figures were taken", async () => {
    renderView();
    const panel = await screen.findByRole("region", { name: "My open bugs" });
    expect(within(panel).getByText(/Counted/)).toBeInTheDocument();
  });

  it("pins a saved filter as a new dashboard", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("region", { name: "My open bugs" });
    await user.selectOptions(screen.getByLabelText("Saved filter"), "10101");
    await user.click(screen.getByRole("button", { name: /Add dashboard/ }));
    await waitFor(() =>
      expect(api.CreateDashboard).toHaveBeenCalledWith("p1", "Platform this quarter", "10101", "project in (PLAT, OPS)"),
    );
  });

  it("refreshes one dashboard on its own", async () => {
    const user = userEvent.setup();
    renderView();
    const panel = await screen.findByRole("region", { name: "My open bugs" });
    await user.click(within(panel).getByRole("button", { name: /Refresh/ }));
    await waitFor(() => expect(api.RefreshDashboard).toHaveBeenCalledWith("p1", "d1"));
  });

  // The figures are the last ones read, so a failed refresh has to leave
  // them on screen and say why it could not replace them.
  it("keeps the figures and says so when a refresh cannot reach Jira", async () => {
    const user = userEvent.setup();
    vi.mocked(api.RefreshDashboard).mockRejectedValue(new Error("no route to host"));
    renderView();
    const panel = await screen.findByRole("region", { name: "My open bugs" });
    await user.click(within(panel).getByRole("button", { name: /Refresh/ }));
    expect(await screen.findByText(/no route to host/)).toBeInTheDocument();
    expect(within(panel).getByText("12")).toBeInTheDocument();
  });

  it("says what to do when the profile has no dashboards yet", async () => {
    vi.mocked(api.ListDashboards).mockResolvedValue([]);
    renderView();
    expect(await screen.findByText(/No dashboards yet/)).toBeInTheDocument();
  });

  // A connection that cannot read filters is not a person who has starred
  // none, and the view must not word it as one.
  it("reports a connection that cannot read saved filters", async () => {
    vi.mocked(api.ListJiraFilters).mockRejectedValue(new Error("this connection cannot read saved filters"));
    renderView();
    expect(await screen.findByText(/cannot read saved filters/)).toBeInTheDocument();
  });

  // #346: a filter you wrote and never starred used to be missing here
  // altogether. Now that both lists are read, the picker has to say which
  // is which, or a name you do not recognise is indistinguishable from one
  // you wrote.
  it("groups the picker by the filters you own and the ones shared with you", async () => {
    renderView();
    // The select is on screen from the first render, so waiting for it
    // proves nothing. Waiting for an option proves the filters arrived.
    await screen.findByRole("option", { name: "My open bugs" });
    const picker = screen.getByRole("combobox", { name: "Saved filter" });
    const yours = within(picker).getByRole("group", { name: "Your filters" });
    expect(within(yours).getByRole("option", { name: "My open bugs" })).toBeInTheDocument();
    const theirs = within(picker).getByRole("group", { name: "Shared with you" });
    expect(within(theirs).getByRole("option", { name: "Platform this quarter" })).toBeInTheDocument();
    expect(screen.getByText(/Filters you own come first/)).toBeInTheDocument();
  });

  // A Jira with no owned-filter route leaves the scope empty, and a group
  // labelled from a scope nobody read would be a guess on screen. The list
  // stays flat, and the guidance that explains the groups goes with them.
  it("lists the filters flat when the instance cannot say which are yours", async () => {
    vi.mocked(api.ListJiraFilters).mockResolvedValue([
      { id: "10100", name: "My open bugs", jql: "type = Bug", scope: "" },
      { id: "10101", name: "Platform this quarter", jql: "project = PLAT", scope: "" },
    ]);
    renderView();
    await screen.findByRole("option", { name: "My open bugs" });
    const picker = screen.getByRole("combobox", { name: "Saved filter" });
    expect(within(picker).queryAllByRole("group")).toHaveLength(0);
    expect(screen.queryByText(/Filters you own come first/)).not.toBeInTheDocument();
  });

  // One group with nothing in it would be a heading over empty space.
  it("draws only the group that has filters in it", async () => {
    vi.mocked(api.ListJiraFilters).mockResolvedValue([
      { id: "10100", name: "My open bugs", jql: "type = Bug", scope: "owned" },
    ]);
    renderView();
    await screen.findByRole("option", { name: "My open bugs" });
    const picker = screen.getByRole("combobox", { name: "Saved filter" });
    expect(within(picker).getByRole("group", { name: "Your filters" })).toBeInTheDocument();
    expect(within(picker).queryByRole("group", { name: "Shared with you" })).not.toBeInTheDocument();
    // A line explaining the order of two groups, over one group, promises a
    // heading that is not there. This is the live Data Center case: every
    // filter a reader has starred is usually one they wrote (#348).
    expect(screen.queryByText(/Filters you own come first/)).not.toBeInTheDocument();
  });

});
