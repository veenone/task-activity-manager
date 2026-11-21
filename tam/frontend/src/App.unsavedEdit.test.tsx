import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import * as api from "./api";
import { TOUR_VERSION } from "./tour/steps";
import App from "./App";
import { profileBackend } from "./profileBackend";
import { ViewProvider } from "./nav";
import { ModalProvider } from "./modals";
import { SyncProvider } from "./contexts/SyncContext";

// An edit typed into the issue panel lives only in the panel until Save
// edit journals it, so anything that unmounts the panel has to ask first
// (#521). These run through the shell because two of the four ways out, the
// view tabs and the profile select, belong to it.

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    Health: vi.fn(), ListProfiles: vi.fn(), GetSettings: vi.fn(), SetDefaultProfile: vi.fn(),
    GetSyncState: vi.fn(), ListIssues: vi.fn(), GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn(),
    ListSprints: vi.fn(), ListOpenSprints: vi.fn(), ListFilterOptions: vi.fn(), GetSubtaskTypeName: vi.fn(),
    ListProjectTypes: vi.fn(), GetProfileSetting: vi.fn(), ListBoards: vi.fn(), ListBoardSprints: vi.fn(),
    GetEditableFields: vi.fn(), ListEpics: vi.fn(), ListPendingChanges: vi.fn(),
    EventsOn: vi.fn((event: string, cb: (...args: unknown[]) => void) => {
      menu.handlers[event] = cb;
      return () => {};
    }),
  };
});

// The native menu's handlers, kept so a test can play the part of the menu.
const menu = vi.hoisted(() => ({ handlers: {} as Record<string, (...args: unknown[]) => void> }));

const issue = (key: string, summary: string): api.Issue => ({
  key, id: key, project: "DEMO", type: "story", summary, status: "To Do",
  assignee: "", reporter: "", priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "",
  storyPoints: null, rank: "", created: "", updated: "",
});

function renderApp() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <SyncProvider>
            <ViewProvider>
              <ModalProvider>
                <App />
              </ModalProvider>
            </ViewProvider>
          </SyncProvider>
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const panel = () => screen.getByRole("complementary", { name: /DEMO-/ });
const summary = () => within(panel()).getByLabelText("Summary");
const tab = (name: string) => within(screen.getByRole("navigation", { name: "Views" })).getByRole("button", { name });

// Opens DEMO-1 and, unless told not to, types into its Summary.
async function openWithEdit(user: ReturnType<typeof userEvent.setup>, edit = true) {
  renderApp();
  await user.click(await screen.findByRole("row", { name: /DEMO-1/ }));
  await waitFor(() => expect(summary()).toBeEnabled());
  if (edit) await user.type(summary(), " now");
}

async function answer(user: ReturnType<typeof userEvent.setup>, choice: "Cancel" | "Discard") {
  const dialog = await screen.findByRole("alertdialog", { name: /Discard your edit/ });
  // The dialog has two Cancel buttons, the corner one and the one in the
  // footer; the footer is the one a reader means.
  const buttons = within(dialog).getAllByRole("button", { name: choice });
  await user.click(buttons[buttons.length - 1]);
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.Health).mockResolvedValue({ ok: true, error: "", dbPath: "", sharedPath: "", logPath: "" });
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Demo team", jiraUrl: "demo", projectKey: "DEMO", backend: "jira", createdAt: "" },
    { id: "p2", name: "Other team", jiraUrl: "demo", projectKey: "OTH", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light", showNavRail: false, tourSeenVersion: TOUR_VERSION });
  vi.mocked(api.SetDefaultProfile).mockResolvedValue();
  vi.mocked(api.GetSyncState).mockResolvedValue({ lastSynced: "", lastFull: "", lastError: "", issueCount: 0, projectTotal: 0 });
  vi.mocked(api.ListIssues).mockResolvedValue({ issues: [issue("DEMO-1", "Apply promo"), issue("DEMO-2", "Refund flow")], total: 2 });
  vi.mocked(api.GetIssueDetail).mockRejectedValue(new Error("offline"));
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
  vi.mocked(api.ListSprints).mockResolvedValue([]);
  vi.mocked(api.ListOpenSprints).mockResolvedValue([]);
  vi.mocked(api.ListFilterOptions).mockResolvedValue({ statuses: [], assignees: [] });
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
  vi.mocked(api.ListProjectTypes).mockResolvedValue([]);
  vi.mocked(api.GetProfileSetting).mockResolvedValue("");
  vi.mocked(api.ListBoards).mockResolvedValue([]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue([]);
  vi.mocked(api.GetEditableFields).mockResolvedValue([]);
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.ListPendingChanges).mockResolvedValue([]);
});

describe("the issue panel's unsaved edit", () => {
  it("asks before another issue replaces it, and Cancel keeps the text", async () => {
    const user = userEvent.setup();
    await openWithEdit(user);
    await user.click(screen.getByRole("row", { name: /DEMO-2/ }));
    await answer(user, "Cancel");
    expect(panel()).toHaveAccessibleName("DEMO-1");
    expect(summary()).toHaveValue("Apply promo now");

    await user.click(screen.getByRole("row", { name: /DEMO-2/ }));
    await answer(user, "Discard");
    await waitFor(() => expect(panel()).toHaveAccessibleName("DEMO-2"));
  });

  it("asks before the panel is closed", async () => {
    const user = userEvent.setup();
    await openWithEdit(user);
    await user.click(within(panel()).getByRole("button", { name: "Close" }));
    await answer(user, "Cancel");
    expect(summary()).toHaveValue("Apply promo now");

    await user.click(within(panel()).getByRole("button", { name: "Close" }));
    await answer(user, "Discard");
    await waitFor(() => expect(screen.queryByRole("complementary", { name: /DEMO-/ })).toBeNull());
  });

  it("asks before another view replaces it", async () => {
    const user = userEvent.setup();
    await openWithEdit(user);
    await user.click(tab("Boards"));
    await answer(user, "Cancel");
    expect(tab("Backlog")).toHaveAttribute("aria-current", "page");
    expect(summary()).toHaveValue("Apply promo now");

    await user.click(tab("Boards"));
    await answer(user, "Discard");
    await waitFor(() => expect(tab("Boards")).toHaveAttribute("aria-current", "page"));
  });

  // A View menu shortcut fires from the native menu even with a dialog up,
  // and switching under the dialog unmounts whatever it holds (#528).
  it("ignores the View menu while a dialog is open", async () => {
    const user = userEvent.setup();
    renderApp();
    const create = await screen.findByRole("button", { name: "+ New" });
    await waitFor(() => expect(create).toBeEnabled());
    await user.click(create);
    const dialog = await screen.findByRole("dialog");
    act(() => menu.handlers["menu:view"]("worklog"));
    expect(dialog).toBeInTheDocument();
    expect(tab("Backlog")).toHaveAttribute("aria-current", "page");
  });

  it("asks before the profile is switched", async () => {
    const user = userEvent.setup();
    await openWithEdit(user);
    const profile = screen.getByRole("combobox", { name: "Profile" });
    await waitFor(() => expect(profile).toBeEnabled());
    await user.selectOptions(profile, "p2");
    await answer(user, "Cancel");
    expect(profile).toHaveValue("p1");
    expect(summary()).toHaveValue("Apply promo now");

    await user.selectOptions(profile, "p2");
    await answer(user, "Discard");
    await waitFor(() => expect(profile).toHaveValue("p2"));
  });

  it("asks nothing when nothing is unsaved", async () => {
    const user = userEvent.setup();
    await openWithEdit(user, false);
    await user.click(screen.getByRole("row", { name: /DEMO-2/ }));
    await waitFor(() => expect(panel()).toHaveAccessibleName("DEMO-2"));
    await user.click(within(panel()).getByRole("button", { name: "Close" }));
    await user.click(screen.getByRole("row", { name: /DEMO-1/ }));
    await user.click(tab("Boards"));
    await waitFor(() => expect(tab("Boards")).toHaveAttribute("aria-current", "page"));
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });
});
