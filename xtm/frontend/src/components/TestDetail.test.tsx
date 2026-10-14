import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TestDetail } from "./TestDetail";
import { DialogProvider } from "../contexts/DialogContext";
import {
  AddTestStep,
  EditTestField,
  ReorderTestSteps,
  SetTestRequirements,
  TransitionTest,
} from "../api";

// TestDetail is the slide-in panel for one test. These tests pin what its
// sections do with the loaded test: field saves, status moves, steps,
// requirement links, bugs, run history and custom fields. The API is mocked
// to one test, QA-1, with two steps.

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({
    activeId: "p1",
    activeProfile: { id: "p1", projectKey: "QA", jiraUrl: "https://jira.example.com" },
  }),
}));

vi.mock("../api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api")>()),
  GetTest: vi.fn(async () => ({
    key: "QA-1",
    summary: "Login works",
    description: "",
    priority: "High",
    status: "Draft",
    labels: ["smoke"],
    components: [],
    execType: "Manual",
    folderId: "",
    updated: "",
  })),
  GetTestMeta: vi.fn(async () => ({ created: "", creator: "ann", updated: "", updatedBy: "" })),
  GetTestReview: vi.fn(async () => null),
  GetTestContainers: vi.fn(async () => []),
  GetTestRequirements: vi.fn(async () => [
    { key: "REQ-7", projectKey: "QA", summary: "Users can sign in", status: "Open" },
  ]),
  GetTestPreconditions: vi.fn(async () => []),
  ListAllPreconditions: vi.fn(async () => []),
  ListRequirementsWithCoverage: vi.fn(async () => []),
  GetTestBugs: vi.fn(async () => [
    { key: "BUG-3", projectKey: "QA", summary: "Crash on submit", status: "Open" },
  ]),
  GetTestRunHistory: vi.fn(async () => [
    { execKey: "TE-1", execSummary: "Older", runStatus: "FAIL", updatedAt: "2026-01-01", createdAt: "2026-01-01" },
    { execKey: "TE-2", execSummary: "Newer", runStatus: "PASS", updatedAt: "2026-02-01", createdAt: "2026-02-01" },
  ]),
  GetTestSteps: vi.fn(async () => [
    { xrayId: "s1", action: "Open the page", data: "", expected: "" },
    { xrayId: "s2", action: "Submit", data: "", expected: "" },
  ]),
  CheckJiraTestSteps: vi.fn(async () => null),
  GetTestCustomFields: vi.fn(async () => [
    { fieldId: "cf1", name: "Team", type: "string", value: "Core" },
  ]),
  GetProfileCrossProjectSources: vi.fn(async () => ""),
  GetTestTransitions: vi.fn(async () => [{ id: "t1", name: "Approve", to: "Approved" }]),
  GetCapabilities: vi.fn(async () => {
    throw new Error("use defaults");
  }),
  EditTestField: vi.fn(async () => {}),
  TransitionTest: vi.fn(async () => {}),
  AddTestStep: vi.fn(async () => ({ xrayId: "s3", action: "", data: "", expected: "" })),
  ReorderTestSteps: vi.fn(async () => {}),
  SetTestRequirements: vi.fn(async () => {}),
}));

function renderDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const onEdited = vi.fn();
  render(
    <QueryClientProvider client={client}>
      <DialogProvider>
        <TestDetail
          testKey="QA-1"
          version={0}
          pendingForTest={[]}
          folders={[]}
          jiraUrl="https://jira.example.com"
          onClose={() => {}}
          onEdited={onEdited}
        />
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onEdited };
}

describe("TestDetail", () => {
  beforeEach(() => vi.clearAllMocks());

  it("saves an edited summary on blur", async () => {
    const user = userEvent.setup();
    const { onEdited } = renderDetail();
    const input = await screen.findByDisplayValue("Login works");
    await user.clear(input);
    await user.type(input, "Login still works");
    await user.tab();
    await waitFor(() =>
      expect(EditTestField).toHaveBeenCalledWith("p1", "QA-1", "summary", "Login still works"),
    );
    expect(onEdited).toHaveBeenCalled();
  });

  it("moves the status through a workflow transition", async () => {
    const user = userEvent.setup();
    renderDetail();
    await user.selectOptions(await screen.findByDisplayValue("Move to…"), "Approved");
    await waitFor(() => expect(TransitionTest).toHaveBeenCalledWith("p1", "QA-1", "Approved"));
  });

  it("adds a step and reorders steps", async () => {
    const user = userEvent.setup();
    renderDetail();
    await user.click(await screen.findByRole("button", { name: "+ Add step" }));
    await waitFor(() => expect(AddTestStep).toHaveBeenCalledWith("p1", "QA-1", "", "", ""));
    await user.click(screen.getAllByTitle("Move step down")[0]);
    await waitFor(() =>
      expect(ReorderTestSteps).toHaveBeenCalledWith("p1", "QA-1", ["s2", "s1", "s3"]),
    );
  });

  it("unlinks a requirement after confirming", async () => {
    const user = userEvent.setup();
    renderDetail();
    await user.click(await screen.findByRole("button", { name: "Unlink this requirement" }));
    await user.click(await screen.findByRole("button", { name: "Unlink" }));
    await waitFor(() => expect(SetTestRequirements).toHaveBeenCalledWith("p1", "QA-1", []));
  });

  it("lists linked bugs and sorts run history newest first", async () => {
    renderDetail();
    expect(await screen.findByText("Crash on submit")).toBeInTheDocument();
    const table = await screen.findByRole("table", { name: "Run history" });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows[0]).toHaveTextContent("TE-2");
    expect(rows[1]).toHaveTextContent("TE-1");
  });

  it("expands the custom fields on demand", async () => {
    const user = userEvent.setup();
    renderDetail();
    const toggle = await screen.findByRole("button", { name: /Custom Fields/ });
    expect(screen.queryByDisplayValue("Core")).not.toBeInTheDocument();
    await user.click(toggle);
    expect(screen.getByDisplayValue("Core")).toBeInTheDocument();
  });
});
