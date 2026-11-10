// The mocks and helpers ContainersView's tests share: the view pulls in a
// lot, so each test file importing this gets the same stubbed world.
import { vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ContainersView } from "./ContainersView";
import type { Capabilities } from "../api";
import { clearViewState } from "../lib/viewState";
import { BulkEditContainers } from "../api";
import { NavProvider, useNav } from "../contexts/NavContext";
import * as confirmModule from "./useConfirm";

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
// The confirm stub is made in the factory and read back from the mocked
// module, because a hoisted variable cannot be exported.
vi.mock("./useConfirm", () => ({
  useConfirm: () => ({ confirm: confirmFn }),
  confirmFn,
}));
vi.mock("./useNotice", () => ({
  useNotice: () => ({ notice: vi.fn(async () => {}) }),
}));

const mockUseCapabilities = vi.fn();
const confirmFn = vi.hoisted(() => vi.fn(async (_opts: unknown) => false));
export const mockConfirm = (confirmModule as unknown as { confirmFn: typeof confirmFn }).confirmFn;
vi.mock("../features", () => ({
  useCapabilities: (...args: unknown[]) => mockUseCapabilities(...args),
}));

// A permissive baseline (mirrors features.ts's defaultCapabilities) so tests
// only need to spell out the two fields under test.
const fullCaps: Capabilities = {
  name: "xray",
  supportsDatasets: true,
  supportsDatasetWrites: true,
  supportsParameterizedRunDetail: false, supportsParameterizedRunWrites: false, runStatuses: [], supportsRunStepResults: false,
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

export function renderWithCaps(overrides: Partial<Capabilities>) {
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
export function renderNavigationHarness() {
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
export async function selectTestExecutions() {
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
