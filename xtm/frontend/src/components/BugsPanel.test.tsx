import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BugsPanel } from "./BugsPanel";
import { DialogProvider } from "../contexts/DialogContext";
import { clearViewState } from "../lib/viewState";
import { CreateContainerAndAllocate, GetTestRunHistory, SyncBugs } from "../api";

// BugsPanel is the master-detail view of the defects linked to the profile's
// tests. These tests pin the list (filter, checks, the bulk Create Test
// Execution), the detail pane, and the run history a test row expands to.

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

vi.mock("./TestDetail", () => ({ TestDetail: () => <div>test detail</div> }));

vi.mock("../api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api")>()),
  ListBugsWithTests: vi.fn(async () => [
    { key: "BUG-1", projectKey: "QA", summary: "Crash on submit", status: "Open", priority: "High", testKeys: ["QA-1", "QA-2"], issueType: "Bug", updated: "" },
    { key: "BUG-2", projectKey: "QA", summary: "Typo in footer", status: "Done", priority: "Low", testKeys: ["QA-2"], issueType: "Bug", updated: "" },
  ]),
  ListTestsForBug: vi.fn(async () => [
    { key: "QA-1", project: "QA", summary: "Submit the form", status: "Approved", runStatus: "FAIL" },
  ]),
  GetBugDetail: vi.fn(async () => ({ reporter: "ann", severity: "Major", description: "" })),
  GetTestRunHistory: vi.fn(async () => [
    { execKey: "TE-9", execSummary: "Nightly", runStatus: "FAIL", planKeys: ["TP-1"], finishedAt: "2026-01-01" },
  ]),
  SyncBugs: vi.fn(async () => {}),
  CreateContainerAndAllocate: vi.fn(async () => ({ tempKey: "NEW-5", added: 2 })),
}));

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DialogProvider>
        <BugsPanel jiraUrl="https://jira.example.com" onOpenTest={() => {}} />
      </DialogProvider>
    </QueryClientProvider>,
  );
}

describe("BugsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clearViewState("p1");
  });

  it("filters the bug list by text", async () => {
    const user = userEvent.setup();
    renderPanel();
    expect(await screen.findByRole("button", { name: "BUG-2 Typo in footer" })).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText(/Filter bugs/), "crash");
    expect(screen.queryByRole("button", { name: "BUG-2 Typo in footer" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "BUG-1 Crash on submit" })).toBeInTheDocument();
  });

  it("shows the selected bug's detail and affected tests", async () => {
    renderPanel();
    expect(await screen.findByRole("heading", { name: "Typo in footer" })).toBeInTheDocument();
    expect(await screen.findByText("ann")).toBeInTheDocument();
    const table = await screen.findByRole("table", { name: "Tests linked to this bug" });
    expect(within(table).getByText("Submit the form")).toBeInTheDocument();
  });

  it("creates a Test Execution from the checked bugs' linked tests", async () => {
    const user = userEvent.setup();
    renderPanel();
    await screen.findByRole("button", { name: "BUG-1 Crash on submit" });
    await user.click(screen.getByRole("checkbox", { name: /Select all \(2\)/ }));
    await user.click(screen.getByRole("button", { name: "Create Test Execution (2)" }));
    await user.click(await screen.findByRole("button", { name: "Create" }));
    await waitFor(() =>
      expect(CreateContainerAndAllocate).toHaveBeenCalledWith(
        "p1",
        "testexec",
        "Verify bugs: BUG-1, BUG-2",
        ["QA-1", "QA-2"],
      ),
    );
    expect(await screen.findByText(/Created Test Execution NEW-5 with 2 tests/)).toBeInTheDocument();
  });

  it("syncs just the bugs", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(await screen.findByRole("button", { name: "Sync" }));
    await waitFor(() => expect(SyncBugs).toHaveBeenCalledWith("p1"));
  });

  it("expands a test row into its run history grouped by plan", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(await screen.findByTitle("Expand run history"));
    await waitFor(() => expect(GetTestRunHistory).toHaveBeenCalledWith("p1", "QA-1"));
    const runs = await screen.findByRole("table", { name: "Test runs of QA-1" });
    expect(within(runs).getByRole("button", { name: "Plan: TP-1" })).toBeInTheDocument();
    expect(within(runs).getByRole("button", { name: "TE-9" })).toBeInTheDocument();
  });
});
