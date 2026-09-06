import React from "react";
import { describe, it, expect, vi, afterAll, beforeAll, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import type { Issue } from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
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

// A selected row's IssueDetailPanel reads useSync to hold Save during a sync
// or commit; its provider needs more wiring than this view otherwise uses,
// so it is mocked here rather than wired up just for that.
vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ status: "idle" }) }));

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

// Switcher stands in for the topbar's profile picker, which lives above
// BacklogView in the shell.
function Switcher() {
  const { setActiveId } = useProfile<api.Profile, api.Settings>();
  return <button type="button" onClick={() => setActiveId("p2")}>Switch profile</button>;
}

function renderView() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <ModalProvider>
            <Loader />
            <Switcher />
            <BacklogView />
          </ModalProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

// jsdom does no layout, so every element reports offsetHeight 0. The row
// virtualiser reads that as an empty viewport and mounts no rows at all, so
// the scroll container is given the height a real window would give it.
const realOffsetHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "offsetHeight");

beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
    configurable: true,
    get(this: HTMLElement) {
      return this.classList.contains("issue-body") ? 600 : 0;
    },
  });
});

afterAll(() => {
  if (realOffsetHeight) {
    Object.defineProperty(HTMLElement.prototype, "offsetHeight", realOffsetHeight);
  } else {
    delete (HTMLElement.prototype as { offsetHeight?: number }).offsetHeight;
  }
});

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
    { id: "p2", name: "Ops", jiraUrl: "demo", projectKey: "OPS", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: rows, total: 1248 });
  vi.mocked(api.ListSprints).mockResolvedValue([{ id: "12", name: "Sprint 12" }, { id: "13", name: "Sprint 13" }]);
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: "PLAT-412", description: "", links: [], fields: {} });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
});

const lastQuery = () => vi.mocked(api.ListIssues).mock.calls.at(-1)?.[1];

// The pager formats its counts for the machine's locale, so the expectation
// has to group its thousands the same way rather than hard-coding a comma.
const showing = (first: number, last: number, total: number) =>
  `Showing ${first.toLocaleString()} to ${last.toLocaleString()} of ${total.toLocaleString()}`;

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
    expect(screen.getByText(showing(1, 25, 1248))).toBeInTheDocument();
    expect(lastQuery()).toEqual({ text: "", types: [], sprintId: "", offset: 0, limit: 25 });
    expect(screen.getByRole("button", { name: "+ New" })).toBeInTheDocument();
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
    expect(screen.getByText(showing(26, 50, 1248))).toBeInTheDocument();
    // Page one is still cached, so going back is served without another
    // backend call and the pager is what shows the move.
    await userEvent.click(screen.getByRole("button", { name: "Previous page" }));
    await waitFor(() => expect(screen.getByText(showing(1, 25, 1248))).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Previous page" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() => expect(screen.getByText(showing(26, 50, 1248))).toBeInTheDocument());
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

  it("moves between rows with the arrow keys and selects with Enter", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-409")).toBeInTheDocument());
    screen.getByRole("row", { name: /PLAT-412/ }).focus();
    await userEvent.keyboard("{ArrowDown}");
    expect(screen.getByRole("row", { name: /PLAT-409/ })).toHaveFocus();
    await userEvent.keyboard("{Enter}");
    expect(screen.getByRole("row", { name: /PLAT-409/ })).toHaveAttribute("aria-selected", "true");
  });

  it("switching profiles drops the search text before the first query", async () => {
    renderView();
    await waitFor(() => expect(screen.getByText("PLAT-412")).toBeInTheDocument());
    await userEvent.type(screen.getByRole("searchbox", { name: "Search issues" }), "promo");
    await waitFor(() => expect(lastQuery()?.text).toBe("promo"));

    await userEvent.click(screen.getByRole("button", { name: "Switch profile" }));
    await waitFor(() =>
      expect(vi.mocked(api.ListIssues).mock.calls.some((c) => c[0] === "p2")).toBe(true),
    );
    const firstForP2 = vi.mocked(api.ListIssues).mock.calls.find((c) => c[0] === "p2");
    expect(firstForP2?.[1]).toEqual({ text: "", types: [], sprintId: "", offset: 0, limit: 25 });
    // No query for the new profile may carry the old profile's filters.
    for (const c of vi.mocked(api.ListIssues).mock.calls) {
      if (c[0] === "p2") expect(c[1].text).toBe("");
    }
  });

  it("explains an empty cache", async () => {
    vi.mocked(api.ListIssues).mockResolvedValue({ issues: [], total: 0 });
    renderView();
    await waitFor(() => expect(screen.getByText(/No issues cached yet/)).toBeInTheDocument());
  });

  it("shows the pending dot and the Draft chip on rows", async () => {
    vi.mocked(api.ListIssues).mockResolvedValue({
      issues: [
        issue({ key: "TAM-NEW-1", summary: "Add a retry", status: "Draft", draft: true, pending: true }),
        issue({ key: "PLAT-412", summary: "Promo", status: "In Progress", pending: true }),
        issue({ key: "PLAT-409", summary: "Rotate keys" }),
      ],
      total: 3,
    });
    renderView();
    await waitFor(() => expect(screen.getByText("Add a retry")).toBeInTheDocument());
    const rows = await screen.findAllByRole("row");
    // rows[0] is the header row.
    expect(within(rows[1]).getByLabelText("Pending changes")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Draft")).toBeInTheDocument();
    expect(within(rows[2]).getByLabelText("Pending changes")).toBeInTheDocument();
    expect(within(rows[3]).queryByLabelText("Pending changes")).not.toBeInTheDocument();
  });
});
