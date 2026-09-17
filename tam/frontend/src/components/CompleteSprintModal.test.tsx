import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Issue, Sprint } from "../api";
import { CompleteSprintModal } from "./CompleteSprintModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, CompleteSprint: vi.fn() };
});

vi.mock("../contexts/SyncContext", () => ({
  useSync: () => ({ runQuietLock: async <T,>(action: () => Promise<T>) => action() }),
}));

const SPRINT: Sprint = { id: 12, boardId: 1, name: "Sprint 12", state: "active", startDate: "", endDate: "", goal: "" };
const NEXT: Sprint = { ...SPRINT, id: 13, name: "Sprint 13", state: "future" };

function card(key: string): Issue {
  return {
    key, id: key, project: "PLAT", type: "task", summary: `${key} work`, status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "12", sprintName: "Sprint 12", parentKey: "", rank: "", created: "", updated: "",
  };
}

function renderModal(incomplete: Issue[]) {
  const onCompleted = vi.fn();
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <CompleteSprintModal profileId="p1" boardId={1} sprint={SPRINT} futures={[NEXT]} incomplete={incomplete}
          hidden={0} lastColumn="Done" onClose={vi.fn()} onCompleted={onCompleted} />
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onCompleted };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.CompleteSprint).mockResolvedValue(undefined);
});

describe("CompleteSprintModal", () => {
  it("counts the cached unfinished cards as an estimate that Commit works out again", () => {
    renderModal([card("PLAT-1"), card("PLAT-2"), card("PLAT-3")]);
    expect(screen.getByText(
      "About 3 cards are not finished. The exact set is worked out again on Commit, and the Commit result reports how many moved.",
    )).toBeInTheDocument();
    expect(screen.getByText("PLAT-2")).toBeInTheDocument();
    expect(screen.queryByText(/will move out of the sprint/)).not.toBeInTheDocument();
    expect(screen.queryByText("Sends to Jira now")).not.toBeInTheDocument();
  });

  it("still offers a destination when the cache sees nothing unfinished", () => {
    renderModal([]);
    expect(screen.getByText(/^No card in this sprint looks unfinished right now\. The exact set is worked out again on Commit/)).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Move them to" })).toBeInTheDocument();
  });

  it("journals the completion with the preview count and reports it as waiting for Commit", async () => {
    const user = userEvent.setup();
    const { onCompleted } = renderModal([card("PLAT-1"), card("PLAT-2")]);
    await user.selectOptions(screen.getByRole("combobox", { name: "Move them to" }), "13");
    expect(screen.getByText("On Commit, unfinished cards move to Sprint 13.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Complete sprint" }));
    await waitFor(() => expect(api.CompleteSprint).toHaveBeenCalledWith("p1", 1, 12, "13", 2));
    await waitFor(() => expect(onCompleted).toHaveBeenCalledWith("12", "Sprint 12 will be completed on Commit. Unfinished cards move to Sprint 13."));
  });

  it("keeps a refusal in the dialog", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CompleteSprint).mockRejectedValue(new Error("2 pending change(s) belong to cards in this sprint; commit them before completing it"));
    const { onCompleted } = renderModal([card("PLAT-1")]);
    await user.click(screen.getByRole("button", { name: "Complete sprint" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("commit them before completing it");
    expect(onCompleted).not.toHaveBeenCalled();
  });
});
