import React from "react";
import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { Issue } from "../api";
import { profileBackend } from "../profileBackend";
import { BacklogView } from "./BacklogView";

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
  };
});

function issue(over: Partial<Issue>): Issue {
  return {
    key: "PLAT-1", id: "1", project: "PLAT", type: "task", summary: "x", status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
    ...over,
  };
}

const rows: Issue[] = [
  issue({ key: "PLAT-412", type: "story", summary: "Checkout: apply promo code at payment step", status: "In Progress", assignee: "R. Anand", sprintId: "12", sprintName: "Sprint 12", storyPoints: 5 }),
  issue({ key: "PLAT-409", type: "task", summary: "Rotate payment gateway API keys", sprintId: "12", sprintName: "Sprint 12", storyPoints: 2 }),
  issue({ key: "PLAT-388", type: "requirement", summary: "Promo codes must be single-use per customer", status: "Approved", assignee: "PO" }),
];

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
          <BacklogView />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

// jsdom does no layout, so every element reports offsetHeight 0. The row
// virtualiser reads that as an empty viewport and mounts no rows at all, so
// the scroll container is given the height a real window would give it.
beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
    configurable: true,
    get(this: HTMLElement) {
      return this.classList.contains("issue-body") ? 600 : 0;
    },
  });
});

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: rows, total: 1248 });
  vi.mocked(api.ListSprints).mockResolvedValue([{ id: "12", name: "Sprint 12" }, { id: "13", name: "Sprint 13" }]);
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: "PLAT-412", description: "", links: [], fields: {} });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
});

const lastQuery = () => vi.mocked(api.ListIssues).mock.calls.at(-1)?.[1];

describe("BacklogView", () => {
  it("renders the page with the seven columns and the count", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    const header = screen.getByRole("row", { name: /key type summary status assignee sprint pts/i });
    expect(header).toBeInTheDocument();
    const row = screen.getByRole("row", { name: /PLAT-412/ });
    expect(within(row).getByText("Story")).toBeInTheDocument();
    expect(within(row).getByText("In Progress")).toBeInTheDocument();
    expect(within(row).getByText("R. Anand")).toBeInTheDocument();
    expect(within(row).getByText("12")).toBeInTheDocument();
    expect(within(row).getByText("5")).toBeInTheDocument();
    expect(screen.getByText("Showing 1 to 25 of 1,248")).toBeInTheDocument();
    expect(lastQuery()).toEqual({ text: "", types: [], sprintId: "", offset: 0, limit: 25 });
  });

  it("filters by text, type chips, and sprint", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    await userEvent.type(screen.getByRole("searchbox", { name: "Search issues" }), "promo");
    await waitFor(() => expect(lastQuery()?.text).toBe("promo"));
    expect(lastQuery()?.offset).toBe(0);
    await userEvent.click(screen.getByRole("button", { name: "Story", pressed: false }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["story"]));
    await userEvent.click(screen.getByRole("button", { name: "Bug", pressed: false }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["story", "bug"]));
    await userEvent.click(screen.getByRole("button", { name: "Story", pressed: true }));
    await waitFor(() => expect(lastQuery()?.types).toEqual(["bug"]));
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Sprint" }), "13");
    await waitFor(() => expect(lastQuery()?.sprintId).toBe("13"));
  });

  it("pages forward and back and resets the page when the filter changes", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() => expect(lastQuery()?.offset).toBe(25));
    expect(screen.getByText("Showing 26 to 50 of 1,248")).toBeInTheDocument();
    // Page one is still cached, so going back is served without another
    // backend call and the pager is what shows the move.
    await userEvent.click(screen.getByRole("button", { name: "Previous page" }));
    await waitFor(() => expect(screen.getByText("Showing 1 to 25 of 1,248")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() => expect(screen.getByText("Showing 26 to 50 of 1,248")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: "Bug", pressed: false }));
    await waitFor(() => expect(lastQuery()).toMatchObject({ types: ["bug"], offset: 0 }));
  });

  it("selects a row on click", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-409")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("row", { name: /PLAT-409/ }));
    expect(screen.getByRole("row", { name: /PLAT-409/ })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("row", { name: /PLAT-412/ })).toHaveAttribute("aria-selected", "false");
  });

  it("explains an empty cache", async () => {
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
    renderView();
    await waitFor(() => expect(screen.getByText(/No issues cached yet/)).toBeInTheDocument());
  });
});
