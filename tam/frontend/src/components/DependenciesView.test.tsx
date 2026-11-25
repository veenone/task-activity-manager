import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import type { CanvasEdge, CanvasNode } from "@agile-suite/core";
import * as api from "../api";
import type { DependencyGraph, DependencyLink, Issue } from "../api";
import { profileBackend } from "../profileBackend";
import { ModalProvider } from "../modals";
import { DependenciesView } from "./DependenciesView";
import { graphImage } from "@agile-suite/core/src/g6/G6Canvas";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    GetDependencyGraph: vi.fn(),
    ListSprints: vi.fn(),
    ListFilterOptions: vi.fn(),
    ListOpenSprints: vi.fn(),
    GetLinkTypes: vi.fn(),
    AddLink: vi.fn(),
    RemoveLink: vi.fn(),
    RetypeLink: vi.fn(),
    DiscardPendingChange: vi.fn(),
    GetIssueDetail: vi.fn(),
    ListLinkedTests: vi.fn(),
    ListActivity: vi.fn(),
    GetSubtaskTypeName: vi.fn(),
    ListProjectTypes: vi.fn(),
    ListPriorities: vi.fn(),
    SearchUsers: vi.fn(),
    ExportDependencies: vi.fn(),
  };
});
vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ status: "idle" }) }));

// jsdom has no canvas: the fake lists what it was handed and offers the
// three gestures the view listens for, a node click, an edge click and a
// drag between two nodes.
vi.mock("@agile-suite/core/src/g6/G6Canvas", () => ({
  default: ({ nodes, edges, onSelect, onSelectEdge, onConnect }: {
    nodes: CanvasNode[]; edges: CanvasEdge[]; onSelect: (id: string) => void; onSelectEdge: (id: string) => void; onConnect: (s: string, t: string) => void;
  }) => (
    <div data-testid="canvas">
      {nodes.map((n) => (
        <button key={n.id} type="button" data-lit={n.lit ? "yes" : "no"} onClick={() => onSelect(n.id)}>{`node ${n.id}`}</button>
      ))}
      {edges.map((e) => (
        <button key={e.id} type="button" onClick={() => onSelectEdge(e.id)}>{`edge ${e.source} to ${e.target}${e.label ? `, ${e.label}` : ""}`}</button>
      ))}
      <button type="button" onClick={() => onConnect("PLAT-3", "PLAT-1")}>drag PLAT-3 to PLAT-1</button>
      <button type="button" onClick={() => onConnect("PLAT-1", "PLAT-4")}>drag PLAT-1 to PLAT-4</button>
    </div>
  ),
  graphImage: vi.fn(),
}));
const PICTURE = "data:image/png;base64,iVBORw0KGgo=";

function issue(key: string, over: Partial<Issue> = {}): Issue {
  return {
    key, id: key, project: "PLAT", type: "story", summary: `Summary of ${key}`, status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
    ...over,
  };
}
const link = (outward: string, inward: string, over: Partial<DependencyLink> = {}): DependencyLink => ({
  id: `${outward}-${inward}`, type: "Blocks", outward, inward, owner: outward, direction: "outward", pending: false, pendingId: 0, removal: false, ...over,
});

// PLAT-1 blocks PLAT-2 blocks PLAT-3; PLAT-4 only relates to PLAT-3.
const graph: DependencyGraph = {
  issues: [issue("PLAT-1"), issue("PLAT-2", { status: "In Progress" }), issue("PLAT-3", { status: "Done" }), issue("PLAT-4")],
  external: [],
  links: [link("PLAT-1", "PLAT-2"), link("PLAT-2", "PLAT-3"), link("PLAT-4", "PLAT-3", { type: "Relates" })],
};

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
            <DependenciesView />
          </ModalProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" }]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetDependencyGraph).mockResolvedValue(graph);
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.ListFilterOptions).mockResolvedValue({ statuses: ["Done", "In Progress", "To Do"], assignees: [] });
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.GetLinkTypes).mockResolvedValue([
    { name: "Blocks", inward: "is blocked by", outward: "blocks" },
    { name: "Relates", inward: "relates to", outward: "relates to" },
  ]);
  vi.mocked(api.AddLink).mockResolvedValue();
  vi.mocked(api.RemoveLink).mockResolvedValue();
  vi.mocked(api.RetypeLink).mockResolvedValue();
  vi.mocked(api.GetIssueDetail).mockResolvedValue({ key: "PLAT-2", description: "", links: [], fields: {}, comments: [], commentTotal: 0, commentsTruncated: false, fetchedAt: "" });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
  vi.mocked(api.ListActivity).mockResolvedValue([]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
  vi.mocked(api.ListProjectTypes).mockResolvedValue([]);
  vi.mocked(api.ListPriorities).mockResolvedValue([]);
  vi.mocked(api.SearchUsers).mockResolvedValue([]);
  vi.mocked(api.ExportDependencies).mockResolvedValue("C:\\out\\deps.xlsx");
  vi.mocked(graphImage).mockResolvedValue(PICTURE);
});

describe("DependenciesView (#612)", () => {
  it("draws the Blocks links and their issues, leaving out a Relates-only issue", async () => {
    renderView();
    expect(await screen.findByRole("button", { name: "edge PLAT-1 to PLAT-2" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "node PLAT-3" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "node PLAT-4" })).toBeNull();
    expect(screen.getByText("3 issues, 2 links")).toBeInTheDocument();
  });

  it("lights a picked issue's chain and opens its issue panel", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "node PLAT-2" }));
    expect(await screen.findByRole("heading", { name: "PLAT-2" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "node PLAT-1" })).toHaveAttribute("data-lit", "yes");
    expect(screen.getByText(/PLAT-2 waits on 1, 1 wait on it/)).toBeInTheDocument();
  });

  it("narrows the graph by status", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("button", { name: "edge PLAT-1 to PLAT-2" });
    // A link stays while one of its ends matches: only PLAT-2 blocks PLAT-3 has a Done end.
    await user.selectOptions(screen.getByRole("combobox", { name: "Status" }), await screen.findByRole("option", { name: "Done" }));
    expect(screen.getByText("2 issues, 1 link")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "edge PLAT-1 to PLAT-2" })).toBeNull();
    expect(screen.getByRole("button", { name: "edge PLAT-2 to PLAT-3" })).toBeInTheDocument();
  });

  it("journals a link dragged between two issues, with the type chosen", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "drag PLAT-1 to PLAT-4" }));
    const dialog = await screen.findByRole("dialog", { name: "Link two issues" });
    expect(within(dialog).getByText("PLAT-1 blocks PLAT-4. Journaled now, pushed on Commit.")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Add link" }));
    await waitFor(() =>
      expect(api.AddLink).toHaveBeenCalledWith("p1", "PLAT-1", { type: "Blocks", direction: "outward", toKey: "PLAT-4", toSummary: "Summary of PLAT-4", toType: "Story" }),
    );
  });

  it("asks before adding a Blocks link that closes a loop, and adds nothing on Cancel", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "drag PLAT-3 to PLAT-1" }));
    await user.click(within(await screen.findByRole("dialog", { name: "Link two issues" })).getByRole("button", { name: "Add link" }));
    const ask = await screen.findByRole("alertdialog");
    expect(ask).toHaveTextContent("PLAT-1 already blocks PLAT-3");
    await user.click(within(ask).getAllByRole("button", { name: "Cancel" }).at(-1)!);
    expect(api.AddLink).not.toHaveBeenCalled();
  });

  it("removes a picked link, and changes its type, through the journal", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "edge PLAT-1 to PLAT-2" }));
    expect(await screen.findByText("PLAT-1 blocks PLAT-2")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Remove link" }));
    await waitFor(() =>
      expect(api.RemoveLink).toHaveBeenCalledWith("p1", "PLAT-1", expect.objectContaining({ id: "PLAT-1-PLAT-2", direction: "outward", key: "PLAT-2", type: "Blocks" })),
    );
    await user.click(screen.getByRole("button", { name: "Change type" }));
    await waitFor(() => expect(api.RetypeLink).toHaveBeenCalledWith("p1", "PLAT-1", expect.objectContaining({ id: "PLAT-1-PLAT-2" }), "Relates"));
  });

  it("marks a pending link as pending", async () => {
    vi.mocked(api.GetDependencyGraph).mockResolvedValue({ ...graph, links: [link("PLAT-1", "PLAT-2", { pending: true, pendingId: 9, id: "" })] });
    renderView();
    expect(await screen.findByRole("button", { name: "edge PLAT-1 to PLAT-2, pending" })).toBeInTheDocument();
  });
});

describe("exporting the dependencies (#612)", () => {
  it("exports the picked issue and its chain, with a picture of the chain alone", async () => {
    const user = userEvent.setup();
    renderView();
    const chainButton = await screen.findByRole("button", { name: "Export chain" });
    expect(chainButton).toBeDisabled();
    await user.click(await screen.findByRole("button", { name: "node PLAT-2" }));
    await user.click(chainButton);
    await waitFor(() => expect(api.ExportDependencies).toHaveBeenCalled());
    const [profile, scope, image] = vi.mocked(api.ExportDependencies).mock.calls[0];
    expect(profile).toBe("p1");
    expect(scope.focus).toBe("PLAT-2");
    expect([...scope.keys].sort()).toEqual(["PLAT-1", "PLAT-2", "PLAT-3"]);
    expect(scope.types).toEqual(["Blocks"]);
    expect(image).toBe(PICTURE);
    const [nodes] = vi.mocked(graphImage).mock.calls[0];
    expect(nodes.map((n) => n.id)).toEqual(["PLAT-1", "PLAT-2", "PLAT-3"]);
    expect(await screen.findByText("Saved to C:\\out\\deps.xlsx")).toBeInTheDocument();
  });

  it("exports every issue and every link type whatever the filters", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByRole("option", { name: "Done" });
    await user.selectOptions(screen.getByRole("combobox", { name: "Status" }), "Done");
    await user.click(screen.getByRole("button", { name: "Export all" }));
    await waitFor(() => expect(api.ExportDependencies).toHaveBeenCalled());
    expect(vi.mocked(api.ExportDependencies).mock.calls[0][1]).toEqual({ focus: "", keys: [], types: [] });
    const [nodes, edges] = vi.mocked(graphImage).mock.calls[0];
    expect(nodes.map((n) => n.id)).toEqual(["PLAT-1", "PLAT-2", "PLAT-3", "PLAT-4"]);
    expect(edges.map((e) => e.type)).toContain("Relates");
  });

  it("still exports the workbook when the picture cannot be drawn, and says so", async () => {
    vi.mocked(graphImage).mockRejectedValueOnce(new Error("no canvas"));
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "Export all" }));
    await waitFor(() => expect(api.ExportDependencies).toHaveBeenCalled());
    expect(vi.mocked(api.ExportDependencies).mock.calls[0][2]).toBe("");
    expect(await screen.findByText(/without the graph picture: no canvas/)).toBeInTheDocument();
  });

  it("says when the export failed, and says nothing when the dialog was cancelled", async () => {
    vi.mocked(api.ExportDependencies).mockRejectedValueOnce(new Error("disk full")).mockResolvedValueOnce("");
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByRole("button", { name: "Export all" }));
    expect(await screen.findByText("The dependencies could not be exported: disk full")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Export all" }));
    await waitFor(() => expect(api.ExportDependencies).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByText(/could not be exported/)).toBeNull());
    expect(screen.queryByText(/Saved to/)).toBeNull();
  });
});
