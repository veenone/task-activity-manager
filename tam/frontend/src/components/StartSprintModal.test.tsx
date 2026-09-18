import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Sprint } from "../api";
import { StartSprintModal } from "./StartSprintModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, StartSprint: vi.fn(), SuggestSprintDates: vi.fn() };
});

// The start takes Go's per-profile lock through runQuietLock. The stub is
// what the real one is once the lock is free.
vi.mock("../contexts/SyncContext", () => ({
  useSync: () => ({ runQuietLock: async <T,>(action: () => Promise<T>) => action() }),
}));

const SPRINT: Sprint = { id: 13, boardId: 1, name: "Sprint 13", state: "future", startDate: "", endDate: "", goal: "" };

function renderModal(sprint: Partial<Sprint> = {}) {
  const onStarted = vi.fn();
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <StartSprintModal profileId="p1" boardId={1} sprint={{ ...SPRINT, ...sprint }} active={undefined} onClose={vi.fn()} onStarted={onStarted} />
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onStarted };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.StartSprint).mockResolvedValue(undefined);
  vi.mocked(api.SuggestSprintDates).mockResolvedValue({ name: "", start: "2026-09-14", end: "2026-09-28", length: 14, fromHistory: true });
});

describe("StartSprintModal", () => {
  it("says the start is saved locally and sent on Commit, not now", () => {
    renderModal();
    expect(screen.getByText("Saved locally. Commit starts the sprint in Jira.")).toBeInTheDocument();
    expect(screen.queryByText("Sends to Jira now")).not.toBeInTheDocument();
    expect(screen.queryByText(/does not wait for Commit/)).not.toBeInTheDocument();
  });

  it("says a draft sprint is created and then started on Commit", () => {
    renderModal({ id: -1, draft: true });
    expect(screen.getByText("A draft sprint. Commit creates it in Jira, then starts it.")).toBeInTheDocument();
  });

  it("journals the start and reports it as waiting for Commit", async () => {
    const user = userEvent.setup();
    const { onStarted } = renderModal();
    await waitFor(() => expect(screen.getByLabelText("Start")).toHaveValue("2026-09-14"));
    await user.click(screen.getByRole("button", { name: "Start sprint" }));
    await waitFor(() => expect(api.StartSprint).toHaveBeenCalledWith("p1", 1, 13, "Sprint 13", "", "2026-09-14", "2026-09-28"));
    await waitFor(() => expect(onStarted).toHaveBeenCalledWith("13", "Sprint 13 will start on Commit, 2026-09-14 to 2026-09-28."));
  });
});
