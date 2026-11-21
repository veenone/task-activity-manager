import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient } from "@agile-suite/core";
import { profileBackend } from "../profileBackend";
import * as api from "../api";
import type { Issue } from "../api";
import { IssueDetailPanel } from "./IssueDetailPanel";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetIssueDetail: vi.fn(), GetCachedIssue: vi.fn(), ListLinkedTests: vi.fn(), ListEpics: vi.fn(), GetSubtaskTypeName: vi.fn(), GetEditableFields: vi.fn(), ListWorklogs: vi.fn(), BrowserOpenURL: vi.fn() };
});

vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ status: "idle" }) }));

const story: Issue = {
  key: "PLAT-412", id: "1", project: "PLAT", type: "story", summary: "Apply promo code",
  status: "In Progress", assignee: "", reporter: "", priority: "", labels: [],
  sprintId: "", sprintName: "", parentKey: "PLAT-350", storyPoints: null, rank: "",
  created: "", updated: "", description: "",
};
const cached: Record<string, Issue> = {
  "PLAT-350": { ...story, key: "PLAT-350", id: "2", type: "epic", summary: "Checkout revamp", parentKey: "" },
  "PLAT-388": { ...story, key: "PLAT-388", id: "3", summary: "Promo codes are single-use", parentKey: "" },
};

function renderPanel(jiraUrl?: string) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <IssueDetailPanel profileId="p1" issue={story} jiraUrl={jiraUrl} onClose={vi.fn()} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
}

const panel = () => screen.getByRole("complementary");

async function openLinks(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: /^Links/ }));
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetIssueDetail).mockImplementation(async (_p, key) => ({
    key, description: "", fields: {}, comments: [], commentTotal: 0, commentsTruncated: false, fetchedAt: "",
    links: key === "PLAT-412"
      ? [
          { direction: "outward", type: "Relates", key: "PLAT-388", summary: "Promo codes are single-use", issueType: "Story" },
          { direction: "inward", type: "Tested By", key: "XT-1018", summary: "Promo test", issueType: "Test" },
        ]
      : [],
  }));
  vi.mocked(api.GetCachedIssue).mockImplementation(async (_p, key) => cached[key] ?? null);
  vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
  vi.mocked(api.ListEpics).mockResolvedValue([]);
  vi.mocked(api.GetSubtaskTypeName).mockResolvedValue("Sub-task");
  vi.mocked(api.GetEditableFields).mockResolvedValue([]);
  vi.mocked(api.ListWorklogs).mockResolvedValue([]);
});

// Linked keys were plain text and the parent was not shown at all, so
// following one meant leaving for Jira (#530).
describe("IssueDetailPanel's linked and parent issues", () => {
  it("opens a linked issue in the panel", async () => {
    const user = userEvent.setup();
    renderPanel();
    await openLinks(user);
    await user.click(await screen.findByRole("button", { name: "Open PLAT-388" }));
    await waitFor(() => expect(panel()).toHaveAccessibleName("PLAT-388"));
    expect(within(panel()).getByText("Promo codes are single-use")).toBeInTheDocument();
    expect(api.BrowserOpenURL).not.toHaveBeenCalled();
  });

  it("opens the parent issue in the panel", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("button", { name: "Open PLAT-350" }));
    await waitFor(() => expect(panel()).toHaveAccessibleName("PLAT-350"));
    expect(within(panel()).getByText("Checkout revamp")).toBeInTheDocument();
  });

  it("says a key TAM has not cached is not here and offers Jira", async () => {
    const user = userEvent.setup();
    renderPanel("https://jira.example.com");
    await openLinks(user);
    await user.click(await screen.findByRole("button", { name: "Open XT-1018" }));
    const dialog = await screen.findByRole("alertdialog", { name: /XT-1018 is not in TAM/ });
    await user.click(within(dialog).getByRole("button", { name: "Open in Jira" }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://jira.example.com/browse/XT-1018");
    expect(panel()).toHaveAccessibleName("PLAT-412");
  });

  it("says so without a Jira button on a profile with no Jira", async () => {
    const user = userEvent.setup();
    renderPanel();
    await openLinks(user);
    await user.click(await screen.findByRole("button", { name: "Open XT-1018" }));
    const dialog = await screen.findByRole("alertdialog", { name: /XT-1018 is not in TAM/ });
    expect(within(dialog).getByText(/no Jira to open it on/)).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Open in Jira" })).not.toBeInTheDocument();
  });
});
