import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CoverageView } from "./CoverageView";
import { DialogProvider } from "../contexts/DialogContext";
import {
  CreateCanonicalRequirement,
  DeleteCanonicalRequirement,
  SeedDemoCoverageExample,
  SetMemberVersion,
  UpsertCoverageNode,
} from "../api";

// CoverageView is the parameter-coverage workspace: functional requirements on
// the left, and for the selected one its matrix, gaps, reuse and versions.
// These tests pin what the view does itself; the panels it hosts are stubbed.

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

const CANON = [{ id: "c1", name: "C_Sign", memberCount: 2 }];
const DETAIL = {
  model: {
    groups: [
      {
        id: "g1",
        parameters: [
          {
            id: "pa1",
            name: "mechanism",
            values: [
              { id: "v1", valueLabel: "RSA", valueKind: "value", isRequired: true },
              { id: "v2", valueLabel: "ECDSA", valueKind: "value", isRequired: true },
            ],
          },
        ],
      },
    ],
  },
  report: {
    percent: 50,
    testedValues: 1,
    totalValues: 2,
    groups: [{ groupId: "g1", name: "Mechanisms", tested: 1, total: 2, percent: 50 }],
    values: {
      v1: { tested: true, runStatus: "PASSED", testKeys: ["QA-1"] },
      v2: { tested: false, runStatus: "", testKeys: [] },
    },
  },
  gaps: [{ valueId: "v2", groupName: "Mechanisms", paramName: "mechanism", valueLabel: "ECDSA", valueKind: "value", errorCode: "" }],
  reuse: [
    { requirementKey: "REQ-1", projectKey: "A", summary: "Sign", status: "Open", acceptedVersionId: "" },
    { requirementKey: "REQ-2", projectKey: "B", summary: "Sign", status: "Open", acceptedVersionId: "" },
  ],
  stale: [],
};

vi.mock("../queries/coverage", () => ({
  useCanonicalRequirements: () => ({ data: CANON, error: null, refetch: async () => {} }),
  useCoverageDetail: (_p: string, selected: string) => ({
    data: selected ? DETAIL : undefined,
    isFetching: false,
    error: null,
    refetch: async () => {},
  }),
}));

vi.mock("./CoverageMap", () => ({ CoverageMap: () => <div>coverage map</div> }));
vi.mock("./CoverageGuide", () => ({ CoverageGuide: () => <div>coverage guide</div> }));
vi.mock("./VersionBar", () => ({ VersionBar: () => null }));
vi.mock("./ChangeRequestsPanel", () => ({ ChangeRequestsPanel: () => null }));
vi.mock("./VersionDashboard", () => ({ VersionDashboard: () => null }));
vi.mock("./CoveragePublishPanel", () => ({ CoveragePublishPanel: () => null }));

vi.mock("../api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api")>()),
  ListVersions: vi.fn(async () => [{ id: "ver1", name: "1.0", status: "stable" }]),
  CreateCanonicalRequirement: vi.fn(async () => "c2"),
  DeleteCanonicalRequirement: vi.fn(async () => {}),
  UpsertCoverageNode: vi.fn(async () => {}),
  SetMemberVersion: vi.fn(async () => {}),
  SeedDemoCoverageExample: vi.fn(async () => "c1"),
}));

function renderView(isDemo = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DialogProvider>
        <CoverageView isDemo={isDemo} demoVariant="" />
      </DialogProvider>
    </QueryClientProvider>,
  );
}

async function selectCanon() {
  await userEvent.click(screen.getByRole("button", { name: "C_Sign" }));
}

describe("CoverageView", () => {
  beforeEach(() => vi.clearAllMocks());

  it("adds a functional requirement", async () => {
    renderView();
    await userEvent.type(screen.getByPlaceholderText("New functional requirement…"), "C_Verify");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(CreateCanonicalRequirement).toHaveBeenCalledWith("p1", "C_Verify", "", "");
  });

  it("deletes a functional requirement once confirmed", async () => {
    renderView();
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(DeleteCanonicalRequirement).not.toHaveBeenCalled();
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(DeleteCanonicalRequirement).toHaveBeenCalledWith("p1", "c1"));
  });

  it("loads the Login demo example on a demo profile", async () => {
    renderView(true);
    await userEvent.click(screen.getByRole("button", { name: "Load demo example (Login)" }));
    await waitFor(() => expect(SeedDemoCoverageExample).toHaveBeenCalledWith("p1"));
    expect(await screen.findByText(/Loaded the Login demo example/)).toBeInTheDocument();
  });

  it("shows the headline and each value's status in the matrix", async () => {
    renderView();
    await selectCanon();
    expect(screen.getByText("50% · 1/2")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Coverage" }));
    const table = await screen.findByRole("table", { name: "Values in Mechanisms" });
    const rows = within(table).getAllByRole("row");
    expect(rows.find((r) => r.textContent?.includes("RSA"))).toHaveTextContent("Passed");
    expect(rows.find((r) => r.textContent?.includes("ECDSA"))).toHaveTextContent("Gap");
  });

  it("adds a group to the selected version", async () => {
    renderView();
    await selectCanon();
    await userEvent.click(screen.getByRole("button", { name: "Coverage" }));
    await userEvent.type(await screen.findByPlaceholderText("Add group…"), "Padding");
    await userEvent.click(screen.getByRole("button", { name: "Add group" }));
    await waitFor(() => expect(UpsertCoverageNode).toHaveBeenCalled());
    expect((UpsertCoverageNode as unknown as ReturnType<typeof vi.fn>).mock.calls[0][1]).toMatchObject({
      kind: "group",
      canonicalId: "c1",
      versionId: "ver1",
      name: "Padding",
      sortOrder: 1,
    });
  });

  it("lists the gaps", async () => {
    renderView();
    await selectCanon();
    await userEvent.click(screen.getByRole("button", { name: "Gaps (1)" }));
    const table = screen.getByRole("table", { name: "Coverage gaps" });
    expect(within(table).getByText("ECDSA")).toBeInTheDocument();
  });

  it("locks every member to the selected version", async () => {
    renderView();
    await selectCanon();
    await userEvent.click(screen.getByRole("button", { name: "Versions & CRs" }));
    await userEvent.click(await screen.findByRole("button", { name: "Apply 1.0 to all" }));
    await waitFor(() => expect(SetMemberVersion).toHaveBeenCalledTimes(2));
    expect(SetMemberVersion).toHaveBeenCalledWith("p1", "c1", "REQ-2", "ver1");
  });
});
