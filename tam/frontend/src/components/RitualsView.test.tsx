import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { CLOSED_EMPTY_SENTENCE, GONE_SENTENCE, NO_SCRUM_BOARD_SENTENCE, UNCONFIGURED_SENTENCE } from "../lib/ritualText";
import { RitualsView } from "./RitualsView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(), GetSettings: vi.fn(), GetConfluenceConfig: vi.fn(), ListBoards: vi.fn(), ListBoardSprints: vi.fn(),
    EnsureSprintRituals: vi.fn(), LastRitualSync: vi.fn(), ResolveRitualConflict: vi.fn(), ForgetRitualPage: vi.fn(),
    DeleteRitualDocument: vi.fn(), BrowserOpenURL: vi.fn(),
  };
});

const sync = vi.hoisted(() => ({ running: null as string | null, runRitualsSync: vi.fn() }));
vi.mock("../contexts/SyncContext", () => ({ useSync: () => sync }));

// The editor has its own tests; here it only has to show which body and mode
// it was handed. latestEditorProps records the most recent props the mock
// was rendered with so a test can reach into onSaved directly: RitualEditor
// saves on unmount, so a save for the sprint the user just navigated away
// from can resolve after the switch, and the controller-level fix in
// RitualsView for that race (matching boardId, sprintId AND ritualType, not
// ritualType alone) needs a way to fire that late onSaved by hand.
const editorMock = vi.hoisted(() => ({ latest: null as null | { doc: api.RitualDocument; body?: string; readOnly?: boolean; onSaved?: (d: api.RitualDocument) => void } }));
vi.mock("./ritual-editor/RitualEditor", () => ({
  RitualEditor: (p: { doc: api.RitualDocument; body?: string; readOnly?: boolean; onSaved?: (d: api.RitualDocument) => void }) => {
    editorMock.latest = p;
    return <div data-testid="editor" data-type={p.doc.ritualType} data-readonly={String(!!p.readOnly)}>{p.body ?? p.doc.body}</div>;
  },
}));

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
          <RitualsView />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

function docFor(ritualType: string, over: Partial<api.RitualDocument> = {}): api.RitualDocument {
  return {
    profileId: "p1", boardId: 1, sprintId: 14, ritualType, title: `Sprint 14 · ${ritualType}`, body: `<p>${ritualType} body</p>`,
    baseBody: "", pageId: "", version: 0, conflictBody: "", conflictVersion: 0, status: "local",
    updatedAt: "2026-09-14T09:00:00Z", syncedAt: "", ...over,
  };
}
const five = (over: Record<string, Partial<api.RitualDocument>> = {}) =>
  ["_sprint", "planning", "standup", "review", "retro"].map((t) => docFor(t, over[t]));

beforeEach(() => {
  vi.clearAllMocks();
  editorMock.latest = null;
  sync.running = null;
  vi.mocked(api.ListProfiles).mockResolvedValue([{ id: "p1", name: "Acme", jiraUrl: "https://jira.example.com", projectKey: "PLAT", backend: "jira", createdAt: "" }]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetConfluenceConfig).mockResolvedValue({ baseURL: "https://confluence.example.com", spaceKey: "PLAT", rootPageID: "100" });
  vi.mocked(api.ListBoards).mockResolvedValue([{ id: 1, name: "PLAT board", type: "scrum" }] as api.Board[]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue([
    { id: 13, boardId: 1, name: "Sprint 13", state: "closed", startDate: "", endDate: "", goal: "", completeDate: "" },
    { id: 14, boardId: 1, name: "Sprint 14", state: "active", startDate: "", endDate: "", goal: "", completeDate: "" },
  ] as api.Sprint[]);
  vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five());
  vi.mocked(api.LastRitualSync).mockResolvedValue("");
});

describe("RitualsView", () => {
  it("lists the active sprint's five pages with their statuses and opens Planning", async () => {
    vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five({ review: { status: "conflict" }, retro: { status: "synced" } }));
    renderView();
    const nav = await screen.findByRole("navigation", { name: "Ritual documents" });
    for (const label of ["Overview", "Planning", "Standup", "Review", "Retrospective"]) {
      expect(within(nav).getByRole("button", { name: new RegExp(label) })).toBeInTheDocument();
    }
    expect(within(nav).getByRole("button", { name: /Review/ })).toHaveTextContent("Conflict");
    expect(api.EnsureSprintRituals).toHaveBeenCalledWith("p1", 1, 14);
    expect(screen.getByTestId("editor")).toHaveAttribute("data-type", "planning");
    expect(screen.getByText("Not synced yet · 3 unsynced · 1 conflict")).toBeInTheDocument();
  });

  it("keeps editing available without Confluence and says how to sync", async () => {
    vi.mocked(api.GetConfluenceConfig).mockResolvedValue({ baseURL: "", spaceKey: "", rootPageID: "" });
    renderView();
    expect(await screen.findByTestId("editor")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sync rituals" })).toBeDisabled();
    expect(screen.getByText(UNCONFIGURED_SENTENCE)).toBeInTheDocument();
  });

  it("syncs through the lock, reports what happened, and reloads", async () => {
    sync.runRitualsSync.mockResolvedValue({ created: 5, pulled: 0, pushed: 0, conflicts: 0, gone: 0,
      failed: [{ sprintName: "Sprint 14", title: "Sprint 14 · Review", reason: "403 Forbidden" }], syncedAt: "2026-09-14T10:00:00Z" });
    renderView();
    await screen.findByTestId("editor");
    await userEvent.click(screen.getByRole("button", { name: "Sync rituals" }));
    expect(sync.runRitualsSync).toHaveBeenCalledWith(1);
    expect(await screen.findByText("Sync finished: 5 created, 1 failed.")).toBeInTheDocument();
    expect(screen.getByText("403 Forbidden", { exact: false })).toBeInTheDocument();
    await waitFor(() => expect(api.EnsureSprintRituals).toHaveBeenCalledTimes(2));
  });

  it("shows theirs read only, keeps mine, and confirms before taking theirs", async () => {
    vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five({
      planning: { status: "conflict", pageId: "42", version: 1, conflictVersion: 3, conflictBody: "<p>their planning</p>" },
    }));
    renderView();
    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent("Confluence has a newer version (v3). Your local edits are kept until you choose.");

    await userEvent.click(within(banner).getByRole("button", { name: "View theirs" }));
    expect(screen.getByTestId("editor")).toHaveTextContent("their planning");
    expect(screen.getByTestId("editor")).toHaveAttribute("data-readonly", "true");

    await userEvent.click(within(banner).getByRole("button", { name: "Keep mine" }));
    await waitFor(() => expect(api.ResolveRitualConflict).toHaveBeenCalledWith("p1", 1, 14, "planning", "mine"));

    await userEvent.click(within(screen.getByRole("alert")).getByRole("button", { name: "Take theirs" }));
    const dialog = await screen.findByRole("alertdialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Take theirs" }));
    await waitFor(() => expect(api.ResolveRitualConflict).toHaveBeenCalledWith("p1", 1, 14, "planning", "theirs"));
  });

  it("offers to recreate a page gone from Confluence or remove the local copy", async () => {
    vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five({ planning: { status: "gone", pageId: "42" } }));
    renderView();
    const banner = await screen.findByRole("alert");
    expect(banner).toHaveTextContent(GONE_SENTENCE);
    await userEvent.click(within(banner).getByRole("button", { name: "Recreate on next Sync" }));
    await waitFor(() => expect(api.ForgetRitualPage).toHaveBeenCalledWith("p1", 1, 14, "planning"));
    await userEvent.click(within(screen.getByRole("alert")).getByRole("button", { name: "Remove local copy" }));
    await userEvent.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(api.DeleteRitualDocument).toHaveBeenCalledWith("p1", 1, 14, "planning"));
  });

  it("links a synced page to Confluence", async () => {
    vi.mocked(api.EnsureSprintRituals).mockResolvedValue(five({ planning: { status: "synced", pageId: "42" } }));
    renderView();
    await userEvent.click(await screen.findByRole("button", { name: "Open in Confluence" }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://confluence.example.com/pages/viewpage.action?pageId=42");
  });

  it("says a closed sprint without pages has none", async () => {
    vi.mocked(api.EnsureSprintRituals).mockImplementation(async (_p, _b, sprintId) => (sprintId === 13 ? [] : five()));
    renderView();
    await screen.findByTestId("editor");
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Ritual sprint" }), "13");
    expect(await screen.findByText(CLOSED_EMPTY_SENTENCE)).toBeInTheDocument();
  });

  it("says when there is no scrum board", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([{ id: 2, name: "Kanban", type: "kanban" }] as api.Board[]);
    renderView();
    expect(await screen.findByText(NO_SCRUM_BOARD_SENTENCE)).toBeInTheDocument();
  });

  // Controller ruling: onSaved must match boardId, sprintId AND ritualType,
  // not ritualType alone. RitualEditor saves on unmount, so a save begun for
  // sprint 14's Planning page can still be in flight when the user switches
  // to sprint 13; if the resolved save only matched on ritualType it would
  // overwrite sprint 13's Planning document with sprint 14's stale body.
  it("ignores a stale onSaved from a page the sprint switch already left", async () => {
    vi.mocked(api.EnsureSprintRituals).mockImplementation(async (_p, boardId, sprintId) =>
      sprintId === 13
        ? ["_sprint", "planning", "standup", "review", "retro"].map((t) => docFor(t, { sprintId: 13, title: `Sprint 13 · ${t}` }))
        : five(),
    );
    renderView();
    await screen.findByTestId("editor");
    const savedOnSprint14 = editorMock.latest?.onSaved;
    expect(savedOnSprint14).toBeInstanceOf(Function);

    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Ritual sprint" }), "13");
    await waitFor(() => expect(screen.getByTestId("editor")).toHaveAttribute("data-type", "planning"));
    await waitFor(() => expect(screen.getByTestId("editor")).toHaveTextContent(docFor("planning", { sprintId: 13, title: "Sprint 13 · planning" }).body.replace(/<[^>]+>/g, "")));

    // Fire the sprint-14 save's onSaved now, after the switch to sprint 13.
    savedOnSprint14!(docFor("planning", { sprintId: 14, body: "stale" }));

    expect(screen.getByTestId("editor")).toHaveTextContent(
      docFor("planning", { sprintId: 13, title: "Sprint 13 · planning" }).body.replace(/<[^>]+>/g, ""),
    );
    expect(screen.getByTestId("editor")).not.toHaveTextContent("stale");
  });

  // Critical finding: sync() had no staleness guard. Setting sync.running
  // before render (rather than mid-flight, whose reflection in the DOM
  // depends on a React re-render this plain mock object cannot trigger on
  // its own) is the direct way to prove the pickers are disabled whenever
  // the shared lock actually says a rituals sync is running.
  it("disables the board and sprint pickers while a rituals sync is running", async () => {
    vi.mocked(api.ListBoards).mockResolvedValue([
      { id: 1, name: "PLAT board", type: "scrum" }, { id: 2, name: "Second board", type: "scrum" },
    ] as api.Board[]);
    sync.running = "rituals";
    renderView();
    await screen.findByTestId("editor");
    expect(screen.getByRole("combobox", { name: "Ritual board" })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "Ritual sprint" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Syncing rituals" })).toBeInTheDocument();
  });

  // Critical finding: sync() closed over boardId/sprintId from the render at
  // click time, so a switch made while the request was still in flight had
  // the resolved answer overwrite whatever sprint was now on screen: the
  // result banner, the last-synced line, and the reloaded documents all
  // landed on the sprint the user had already left. This does not set
  // sync.running (a separate, simpler test above covers the pickers'
  // disabled state), so the switch below goes through the picker exactly as
  // a user would drive it once the lock that disables it has released.
  it("answers only for the sprint a Sync was started on, not one switched to while it was in flight", async () => {
    vi.mocked(api.EnsureSprintRituals).mockImplementation(async (_p, _b, sprintId) =>
      sprintId === 13
        ? ["_sprint", "planning", "standup", "review", "retro"].map((t) => docFor(t, { sprintId: 13, title: `Sprint 13 · ${t}` }))
        : five(),
    );
    let resolveSync!: (v: api.RitualSyncResult) => void;
    sync.runRitualsSync.mockImplementation(() => new Promise<api.RitualSyncResult>((resolve) => { resolveSync = resolve; }));

    renderView();
    await screen.findByTestId("editor");

    await userEvent.click(screen.getByRole("button", { name: "Sync rituals" }));
    expect(sync.runRitualsSync).toHaveBeenCalledWith(1);

    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Ritual sprint" }), "13");
    await waitFor(() => expect(screen.getByTestId("editor")).toHaveAttribute("data-type", "planning"));
    await waitFor(() => expect(screen.getByTestId("editor")).toHaveTextContent(
      docFor("planning", { sprintId: 13, title: "Sprint 13 · planning" }).body.replace(/<[^>]+>/g, ""),
    ));

    resolveSync({ created: 5, pulled: 0, pushed: 0, conflicts: 0, gone: 0, failed: [], syncedAt: "2026-09-14T10:00:00Z" });

    // The stale answer must not paint a summary banner over sprint 13, and
    // sprint 13's documents (already shown above) must not be clobbered by
    // a reload keyed to sprint 14.
    await waitFor(() => expect(sync.runRitualsSync).toHaveResolved());
    expect(screen.queryByText(/Sync finished/)).not.toBeInTheDocument();
    expect(screen.getByTestId("editor")).toHaveTextContent(
      docFor("planning", { sprintId: 13, title: "Sprint 13 · planning" }).body.replace(/<[^>]+>/g, ""),
    );
  });

  // Important finding: selected never reset and had no fallback when the
  // current sprint's list lacked that type, which a closed sprint's Ensure
  // (no backfill) makes routine: nav showed no aria-current item and the
  // article pane rendered nothing, with no explanation.
  it("opens the first available document when the selected type is missing from a closed sprint", async () => {
    vi.mocked(api.EnsureSprintRituals).mockImplementation(async (_p, _b, sprintId) =>
      sprintId === 13 ? [docFor("_sprint", { sprintId: 13 }), docFor("standup", { sprintId: 13 })] : five(),
    );
    renderView();
    await screen.findByTestId("editor");
    expect(screen.getByTestId("editor")).toHaveAttribute("data-type", "planning");

    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Ritual sprint" }), "13");
    await waitFor(() => expect(screen.getByTestId("editor")).toHaveAttribute("data-type", "_sprint"));
    const nav = screen.getByRole("navigation", { name: "Ritual documents" });
    expect(within(nav).getByRole("button", { name: /Overview/ })).toHaveAttribute("aria-current", "page");
  });
});
