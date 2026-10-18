import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ContainersView } from "./ContainersView";
import type { Capabilities } from "../api";
import { clearViewState } from "../lib/viewState";
import { BulkEditContainers, BulkSetTestRunStatus, DeallocateTests, EditContainer } from "../api";
import { NavProvider, useNav } from "../contexts/NavContext";

// ContainersView pulls a lot in (containers/board/members queries, several
// modal children, three dialog hooks). This test only cares about the "Create
// bug" gate, so every dependency below is stubbed to the minimum shape that
// gets a Test Execution with one FAILed member row on screen.

vi.mock("../api", () => ({
  ListContainers: vi.fn(async (_profileId: string, kind: string) =>
    kind === "testexec"
      ? [
          {
            key: "TE-1",
            kind: "testexec",
            summary: "Nightly run",
            status: "OPEN",
            parentKey: "",
            parentSummary: "",
            issueType: "Test Execution",
            labels: [],
            environments: [],
            fixVersions: [],
            description: "",
          },
        ]
      : [
          {
            key: "PLAN-1",
            kind: "testplan",
            summary: "Release plan",
            status: "OPEN",
            parentKey: "",
            parentSummary: "",
            issueType: "Test Plan",
            labels: [],
            environments: [],
            fixVersions: [],
            description: "",
          },
        ],
  ),
  GetContainerBoard: vi.fn(async () => ({
    key: "TE-1",
    summary: "Nightly run",
    description: "",
    rows: [
      {
        testKey: "QA-1",
        summary: "Login works",
        status: "Approved",
        runStatus: "FAIL",
        isExternal: false,
      },
    ],
    runCounts: [{ label: "FAIL", count: 1 }],
  })),
  GetExecutionMembersWithRuns: vi.fn(async () => []),
  GetRunRollup: vi.fn(async () => null),
  ListBugsForContainer: vi.fn(async () => []),
  SeedSampleContainers: vi.fn(),
  CleanSampleData: vi.fn(),
  CreateContainerAndAllocate: vi.fn(),
  EditContainer: vi.fn(),
  DeleteContainer: vi.fn(),
  SetContainerEnvironments: vi.fn(),
  DeallocateTests: vi.fn(),
  SetTestRunStatus: vi.fn(),
  BulkSetTestRunStatus: vi.fn(),
  UnlinkBugFromRun: vi.fn(),
  SetTestRunComment: vi.fn(),
  BulkEditContainers: vi.fn(),
  ExportPytest: vi.fn(),
  SyncContainers: vi.fn(),
  BrowserOpenURL: vi.fn(),
  GetRunRollupBreakdown: vi.fn(async () => []),
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

// The Create/Link bug modals aren't exercised by this test (it only checks
// button presence), and mounting them for real would pull in their own
// query/dialog dependencies for no benefit here.
vi.mock("./CreateBugModal", () => ({
  CreateBugModal: () => null,
}));
vi.mock("./LinkBugPicker", () => ({
  LinkBugPicker: () => null,
}));
vi.mock("./AddTestsModal", () => ({
  AddTestsModal: () => null,
}));
vi.mock("./BugsPanel", () => ({
  BugsPanel: () => null,
}));
vi.mock("./JUnitImportModal", () => ({
  JUnitImportModal: () => null,
}));
vi.mock("./JUnitNewExecModal", () => ({
  JUnitNewExecModal: () => null,
}));
vi.mock("./TestDetail", () => ({
  TestDetail: () => null,
}));

// usePrompt / useConfirm / useNotice wrap DialogContext, which isn't mounted
// in this component-only render (mirrors ProfileForm.test.tsx's useConfirm stub).
vi.mock("./usePrompt", () => ({
  usePrompt: () => ({ prompt: vi.fn(async () => null) }),
}));
vi.mock("./useConfirm", () => ({
  useConfirm: () => ({ confirm: mockConfirm }),
}));
vi.mock("./useNotice", () => ({
  useNotice: () => ({ notice: vi.fn(async () => {}) }),
}));

const mockUseCapabilities = vi.fn();
const mockConfirm = vi.hoisted(() => vi.fn(async (_opts: unknown) => false));
vi.mock("../features", () => ({
  useCapabilities: (...args: unknown[]) => mockUseCapabilities(...args),
}));

// A permissive baseline (mirrors features.ts's defaultCapabilities) so tests
// only need to spell out the two fields under test.
const fullCaps: Capabilities = {
  name: "xray",
  supportsDatasets: true,
  supportsDatasetWrites: true,
  idStyle: "opaque",
  supportsJqlScope: true,
  stepModel: "objects",
  supportsTestTypes: true,
  supportsFolders: true,
  supportsFolderWrites: true,
  supportsComponentAdmin: true,
  supportsPreconditionObjects: true,
  supportsRequirementObjects: true,
  supportsIssueLinkTypes: true,
  supportsEnvironments: true,
  supportsContainers: true,
  containerKinds: ["testset", "testplan", "testexec"],
  supportsTestRuns: true,
  statusModel: "workflow",
  supportsWorkflowTransitions: true,
  supportsBugCreation: true,
  supportsBugLinks: true,
  supportsBugRouting: false,
  supportsTags: false,
};

function renderWithCaps(overrides: Partial<Capabilities>) {
  mockUseCapabilities.mockReturnValue({ ...fullCaps, ...overrides });
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NavProvider>
        <ContainersView onChanged={() => {}} isDemo={false} />
      </NavProvider>
    </QueryClientProvider>,
  );
}

function NavigationHarness() {
  const { openContainer } = useNav();
  return (
    <>
      <button onClick={() => openContainer({ kind: "testexec", key: "TE-1" })}>
        Open target execution
      </button>
      <ContainersView onChanged={() => {}} isDemo={false} />
    </>
  );
}
function renderNavigationHarness() {
  mockUseCapabilities.mockReturnValue(fullCaps);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NavProvider>
        <NavigationHarness />
      </NavProvider>
    </QueryClientProvider>,
  );
}
// The board defaults to the Test Plan kind; switch to Test Execution so the
// FAILed member row (and its "Create bug" control) is on screen.
async function selectTestExecutions() {
  const typeSelect = await screen.findByRole("combobox", { name: /type/i });
  await userEvent.selectOptions(typeSelect, "testexec");
}

beforeEach(() => {
  mockUseCapabilities.mockReset();
  mockConfirm.mockReset();
  mockConfirm.mockResolvedValue(false);
  vi.mocked(BulkEditContainers).mockClear();
  vi.restoreAllMocks();
  // useViewState persists selections in a module-level store keyed by
  // profileId, which otherwise leaks the "testexec" kind chosen by one test
  // into the next (all tests share profileId "p1").
  clearViewState("p1");
});

describe("ContainersView dataset navigation", () => {
  it("leaves Bugs mode and selects the exact target after changing kind", async () => {
    renderNavigationHarness();
    await userEvent.click(screen.getByRole("button", { name: "Bugs" }));
    expect(screen.getByRole("button", { name: "Bugs" })).toHaveClass("seg-btn-active");

    await userEvent.click(screen.getByRole("button", { name: "Open target execution" }));

    expect(await screen.findByRole("combobox", { name: "Type" })).toHaveValue(
      "testexec",
    );
    expect(
      await screen.findByTitle("Pick a Test Execution (type to filter)"),
    ).toHaveTextContent("TE-1 · Nightly run");
    expect(screen.getByRole("button", { name: "Containers" })).toHaveClass(
      "seg-btn-active",
    );
  });
});

describe("ContainersView create button", () => {
  it("names the kind it creates", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    expect(
      screen.getByRole("button", { name: "New Test Execution" }),
    ).toBeInTheDocument();
  });
});

describe("ContainersView Create bug gate", () => {
  it("offers Create bug when the backend can create one directly (Xray)", async () => {
    renderWithCaps({ supportsBugCreation: true, supportsBugRouting: false });
    await selectTestExecutions();

    expect(
      await screen.findByRole("button", { name: /create bug for this failed test/i }),
    ).toBeInTheDocument();
  });

  it("offers Create bug when the profile routes bugs elsewhere (configured Kiwi)", async () => {
    renderWithCaps({ supportsBugCreation: false, supportsBugRouting: true });
    await selectTestExecutions();

    expect(
      await screen.findByRole("button", { name: /create bug for this failed test/i }),
    ).toBeInTheDocument();
  });

  it("hides Create bug when the backend cannot file one and nothing is routed (unconfigured Kiwi)", async () => {
    renderWithCaps({ supportsBugCreation: false, supportsBugRouting: false });
    await selectTestExecutions();

    // Wait for the FAIL row to render so the query isn't just still loading.
    await screen.findByText("QA-1");
    expect(
      screen.queryByRole("button", { name: /create bug for this failed test/i }),
    ).toBeNull();
  });
});

describe("ContainersView keyboard", () => {
  it("starts renaming the container from the keyboard, as a click does", async () => {
    renderWithCaps({});
    await selectTestExecutions();

    const heading = await screen.findByRole("heading", { name: "Nightly run" });
    within(heading).getByRole("button").focus();
    await userEvent.keyboard("{Enter}");

    expect(screen.getByDisplayValue("Nightly run")).toBeInTheDocument();
  });

  it("filters the members by run status with Enter on a run badge", async () => {
    renderWithCaps({});
    await selectTestExecutions();

    const badge = await screen.findByRole("button", { name: "FAIL 1" });
    badge.focus();
    await userEvent.keyboard("{Enter}");

    expect(badge).toHaveAttribute("aria-pressed", "true");
  });

  it("names the Set env panel and closes it on Escape", async () => {
    renderWithCaps({});
    await selectTestExecutions();

    await userEvent.click(await screen.findByRole("button", { name: /Set env on shown/ }));
    expect(screen.getByRole("dialog", { name: /Set environment on the 1 shown/ })).toBeInTheDocument();

    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("takes focus into the Set env dialog and hands it back on close", async () => {
    renderWithCaps({});
    await selectTestExecutions();

    const trigger = await screen.findByRole("button", { name: /Set env on shown/ });
    await userEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: /Set environment on the 1 shown/ });
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    for (let i = 0; i < 12; i++) await userEvent.tab();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);

    await userEvent.keyboard("{Escape}");
    expect(document.activeElement).toBe(trigger);
  });
});

describe("ContainersView Set env on shown", () => {
  async function applySetEnv() {
    renderWithCaps({});
    await selectTestExecutions();
    await screen.findByText("TE-1");
    await userEvent.click(screen.getByRole("button", { name: /set env on shown/i }));
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: /batch environment operation/i }),
      "set_env",
    );
    await userEvent.type(screen.getByPlaceholderText("Environment…"), "staging");
    await userEvent.click(screen.getByRole("button", { name: /^apply to \d+$/i }));
  }

  it("leaves the executions alone when the confirm is cancelled", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    await applySetEnv();
    expect(mockConfirm).toHaveBeenCalledTimes(1);
    expect(BulkEditContainers).not.toHaveBeenCalled();
  });

  it("sets the environment on every shown execution once confirmed", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    mockConfirm.mockResolvedValue(true);
    await applySetEnv();
    await vi.waitFor(() =>
      expect(BulkEditContainers).toHaveBeenCalledWith("p1", ["TE-1"], {
        operation: "set_env",
        field: "",
        value: "staging",
      }),
    );
  });
});

// Pins for the T3.2 split (#231): what the view does with its members has to
// survive the code moving into smaller files.
describe("ContainersView members", () => {
  it("sets the result of the selected members in one go", async () => {
    vi.mocked(BulkSetTestRunStatus).mockResolvedValue({ succeeded: ["QA-1"], failed: [] } as never);
    renderWithCaps({});
    await selectTestExecutions();
    await screen.findByText("Login works");
    const boxes = screen.getAllByRole("checkbox");
    await userEvent.click(boxes[boxes.length - 1]);
    expect(screen.getByText("1 selected")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getAllByRole("combobox").find((c) => c.className === "run-select" && c.closest(".board-bulk"))!, "PASS");
    await userEvent.click(screen.getByRole("button", { name: "Apply to selected" }));
    expect(BulkSetTestRunStatus).toHaveBeenCalledWith("p1", "TE-1", ["QA-1"], "PASS");
  });

  it("removes a member from the container", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    await screen.findByText("Login works");
    await userEvent.click(screen.getByRole("button", { name: "Remove from this Test Execution" }));
    expect(DeallocateTests).toHaveBeenCalledWith("p1", "TE-1", ["QA-1"]);
  });

  it("renames the container with Enter", async () => {
    renderWithCaps({});
    await selectTestExecutions();
    const heading = await screen.findByRole("heading", { name: "Nightly run" });
    await userEvent.click(within(heading).getByRole("button"));
    const input = screen.getByDisplayValue("Nightly run");
    await userEvent.clear(input);
    await userEvent.type(input, "Weekly run{Enter}");
    expect(EditContainer).toHaveBeenCalledWith("p1", "TE-1", "Weekly run");
  });
});
