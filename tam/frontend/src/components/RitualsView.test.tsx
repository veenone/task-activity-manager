import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { RitualsView } from "./RitualsView";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    GetConfluenceConfig: vi.fn(),
    ListConfluenceChildPages: vi.fn(),
    ListRitualAssociations: vi.fn(),
    GetRitualPage: vi.fn(),
    GetSprintReport: vi.fn(),
    ListBoards: vi.fn(),
    ListBoardSprints: vi.fn(),
    ListRitualDrafts: vi.fn(),
    ListSprintIssues: vi.fn(),
    ScaffoldSprintRituals: vi.fn(),
  };
});

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

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme Platform", jiraUrl: "https://jira.example.com", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.GetConfluenceConfig).mockResolvedValue({
    baseURL: "https://confluence.example.com", spaceKey: "PLAT", rootPageID: "100",
  });
  vi.mocked(api.ListConfluenceChildPages).mockResolvedValue({ results: [], start: 0, limit: 100, size: 0 } as never);
  vi.mocked(api.ListRitualAssociations).mockResolvedValue([]);
  vi.mocked(api.ListBoards).mockResolvedValue([{ id: 1, name: "Acme Platform Scrum", type: "scrum" }]);
  vi.mocked(api.ListBoardSprints).mockResolvedValue([
    { id: 12, boardId: 1, name: "Sprint 12", state: "active", startDate: "", endDate: "", goal: "" },
  ]);
  vi.mocked(api.ListSprintIssues).mockResolvedValue([]);
});

describe("RitualsView", () => {
  it("offers to draw up the sprint's rituals when none exist", async () => {
    vi.mocked(api.ListRitualDrafts).mockResolvedValue([]);
    renderView();
    expect(await screen.findByRole("button", { name: /Set up .* rituals/ })).toBeInTheDocument();
  });

  it("shows a slot per stored ritual with its status", async () => {
    vi.mocked(api.ListRitualDrafts).mockResolvedValue([
      { ritualType: "planning", title: "Sprint 12 Planning", status: "draft" },
      { ritualType: "review", title: "Sprint 12 Review", status: "draft" },
    ] as never);
    renderView();
    expect(await screen.findByText("Sprint 12 Planning")).toBeInTheDocument();
    expect(screen.getAllByText("draft")).toHaveLength(2);
  });
});
