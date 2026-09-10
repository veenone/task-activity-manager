import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Sprint } from "../api";
import { EditSprintModal } from "./EditSprintModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, EditSprint: vi.fn() };
});

// The edit takes Go's per-profile lock through runQuietLock. The stub is what
// the real one is once the lock is free; SyncContext.test.tsx tests the lock.
vi.mock("../contexts/SyncContext", () => ({
  useSync: () => ({ runQuietLock: async <T,>(action: () => Promise<T>) => action() }),
}));

const SPRINT: Sprint = {
  id: 12, boardId: 1, name: "Sprint 12", state: "future",
  startDate: "2026-08-29T09:00:00.000+0000", endDate: "2026-09-12T09:00:00.000+0000",
  goal: "Ship checkout",
};

function renderModal(sprint: Partial<Sprint> = {}, otherNames: string[] = []) {
  const onEdited = vi.fn();
  const onClose = vi.fn();
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <EditSprintModal
          profileId="p1"
          boardId={1}
          sprint={{ ...SPRINT, ...sprint }}
          otherNames={otherNames}
          onClose={onClose}
          onEdited={onEdited}
        />
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onEdited, onClose };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.EditSprint).mockResolvedValue("");
});

describe("EditSprintModal", () => {
  it("opens on the sprint's own name, goal and dates", () => {
    renderModal();
    expect(screen.getByLabelText("Name")).toHaveValue("Sprint 12");
    expect(screen.getByLabelText("Goal")).toHaveValue("Ship checkout");
    // The day is read off the stamp rather than converted through a Date, so
    // a sprint starting at 09:00 UTC does not open a day earlier for a
    // reader west of it.
    expect(screen.getByLabelText("Start")).toHaveValue("2026-08-29");
    expect(screen.getByLabelText("End")).toHaveValue("2026-09-12");
    // No suggestion is read here, so nothing captions the dates with a
    // sprint length nobody asked for.
    expect(screen.queryByText("Reading this board's sprint length.")).not.toBeInTheDocument();
  });

  it("says the write does not wait for Commit", () => {
    renderModal();
    expect(screen.getByText("Sends to Jira now")).toBeInTheDocument();
  });

  it("sends clearGoal only when a goal that was there has been emptied", async () => {
    const user = userEvent.setup();
    renderModal();
    await user.clear(screen.getByLabelText("Goal"));
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(api.EditSprint).toHaveBeenCalledWith(
      "p1", 1, 12, "Sprint 12", "", "2026-08-29", "2026-09-12", true,
    ));
  });

  it("leaves clearGoal off for a sprint that never had a goal", async () => {
    const user = userEvent.setup();
    renderModal({ goal: "" });
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(api.EditSprint).toHaveBeenCalledWith(
      "p1", 1, 12, "Sprint 12", "", "2026-08-29", "2026-09-12", false,
    ));
  });

  it("nudges about a name another sprint already has without refusing it", async () => {
    const user = userEvent.setup();
    renderModal({}, ["Sprint 13"]);
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Sprint 13");
    expect(screen.getByText("Another sprint on this board is already called that.")).toBeInTheDocument();
    // A duplicate name is legal in Jira and often deliberate, so it is said
    // and not enforced.
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(api.EditSprint).toHaveBeenCalled());
  });

  it("asks before moving a running sprint's end date, and sends nothing if the answer is no", async () => {
    const user = userEvent.setup();
    renderModal({ state: "active" });
    await user.clear(screen.getByLabelText("End"));
    await user.type(screen.getByLabelText("End"), "2026-09-19");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    const ask = await screen.findByRole("alertdialog", { name: "Move a running sprint's end date?" });
    expect(within(ask).getByText(/everyone reading this board sees the sprint end on 2026-09-19/)).toBeInTheDocument();
    await user.click(within(ask).getByRole("button", { name: "Leave it" }));
    expect(api.EditSprint).not.toHaveBeenCalled();
  });

  it("does not ask about a date that has not moved", async () => {
    const user = userEvent.setup();
    renderModal({ state: "active" });
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Sprint twelve");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(api.EditSprint).toHaveBeenCalled());
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("keeps the dialog open with Jira's own sentence when the edit is refused", async () => {
    const user = userEvent.setup();
    vi.mocked(api.EditSprint).mockRejectedValue(new Error("a sprint operation is already running for this profile"));
    const { onClose } = renderModal();
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    // The refusal comes from Go's own lock, which refuses whether or not
    // anything on screen is showing a banner, so the dialog is the only
    // place it can be read.
    expect(await screen.findByRole("alert")).toHaveTextContent("a sprint operation is already running");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refuses a dateless sprint before Jira is asked about it", async () => {
    const user = userEvent.setup();
    renderModal();
    await user.clear(screen.getByLabelText("End"));
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText("The end date cannot be empty.")).toBeInTheDocument();
    expect(api.EditSprint).not.toHaveBeenCalled();
  });

  it("carries a note back beside its success", async () => {
    const user = userEvent.setup();
    vi.mocked(api.EditSprint).mockResolvedValue("The board's sprints could not be re-read. Press Refresh.");
    const { onEdited } = renderModal();
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onEdited).toHaveBeenCalledWith(
      "Sprint 12 was updated. The board's sprints could not be re-read. Press Refresh.",
    ));
  });
});
