import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { RitualDocument } from "../api";
import { DONE_AGREEMENT } from "../lib/ritualText";
import { DoneAgreementSection, agreementProgressLine } from "./DoneAgreementSection";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, ListRitualDocuments: vi.fn(), DoneAgreementTicks: vi.fn(), SetDoneAgreementTick: vi.fn() };
});

function taskList(...items: string[]): string {
  const tasks = items.map((i) => `<ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>${i}</ac:task-body></ac:task>`);
  return `<h2>Items</h2><ac:task-list>${tasks.join("")}</ac:task-list>`;
}

function docFor(sprintId: number, body: string): RitualDocument {
  return {
    profileId: "p1", boardId: 1, sprintId, ritualType: DONE_AGREEMENT,
    title: sprintId === 0 ? "PLAT board · Done agreement" : "PLAT Sprint 12 · Done agreement",
    body, baseBody: body, pageId: "", version: 0, conflictBody: "", conflictVersion: 0,
    status: "local", updatedAt: "", syncedAt: "",
  };
}

const boardDoc = docFor(0, taskList("Reviewed by someone else", "Unit tests pass"));
const sprintDoc = docFor(12, taskList("Load test run against staging"));

function renderSection(sprintId = 12) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <DoneAgreementSection profileId="p1" boardId={1} issueKey="PLAT-412" sprintId={sprintId} />
      </DialogProvider>
    </QueryClientProvider>,
  );
}

function box(name: string): HTMLInputElement {
  return screen.getByRole("checkbox", { name }) as HTMLInputElement;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListRitualDocuments).mockResolvedValue([boardDoc, sprintDoc]);
  vi.mocked(api.DoneAgreementTicks).mockResolvedValue({ "PLAT-412": ["Unit tests pass"] });
  vi.mocked(api.SetDoneAgreementTick).mockResolvedValue();
});

describe("DoneAgreementSection", () => {
  it("shows the board's items and its sprint's additions, ticked where the store says so", async () => {
    renderSection();
    expect(await screen.findByRole("checkbox", { name: "Reviewed by someone else" })).not.toBeChecked();
    expect(box("Unit tests pass")).toBeChecked();
    expect(box("Load test run against staging")).not.toBeChecked();
    expect(api.DoneAgreementTicks).toHaveBeenCalledWith("p1", 1, ["PLAT-412"]);
  });

  it("shows an issue in no sprint the board's items alone", async () => {
    renderSection(0);
    expect(await screen.findByRole("checkbox", { name: "Reviewed by someone else" })).toBeInTheDocument();
    expect(api.ListRitualDocuments).toHaveBeenCalledWith("p1", 1, 0);
    expect(screen.queryByRole("checkbox", { name: "Load test run against staging" })).not.toBeInTheDocument();
  });

  it("ticks an item for that issue and shows it ticked", async () => {
    const user = userEvent.setup();
    renderSection();
    const item = await screen.findByRole("checkbox", { name: "Reviewed by someone else" });
    // What the store answers once the tick is in it, set before the click so
    // the refresh the write triggers reads the new answer and not this one.
    vi.mocked(api.DoneAgreementTicks).mockResolvedValue({ "PLAT-412": ["Unit tests pass", "Reviewed by someone else"] });
    await user.click(item);
    await waitFor(() =>
      expect(api.SetDoneAgreementTick).toHaveBeenCalledWith("p1", 1, "PLAT-412", "Reviewed by someone else", true));
    await waitFor(() => expect(box("Reviewed by someone else")).toBeChecked());
  });

  it("unticks an item that was ticked", async () => {
    const user = userEvent.setup();
    renderSection();
    await user.click(await screen.findByRole("checkbox", { name: "Unit tests pass" }));
    await waitFor(() =>
      expect(api.SetDoneAgreementTick).toHaveBeenCalledWith("p1", 1, "PLAT-412", "Unit tests pass", false));
  });

  // The record, which is the point of the feature: the tick says what was
  // ticked, so rewording the item leaves it visible against the old words
  // rather than dropping it or moving it onto the new ones.
  it("keeps a tick made against words the agreement no longer states, and says so", async () => {
    vi.mocked(api.ListRitualDocuments).mockResolvedValue([docFor(0, taskList("Unit tests pass on CI")), sprintDoc]);
    renderSection();
    expect(await screen.findByRole("checkbox", { name: "Unit tests pass on CI" })).not.toBeChecked();
    expect(box("Unit tests pass")).toBeChecked();
    expect(screen.getByText("Made against wording the agreement has since changed.")).toBeInTheDocument();
    // It can still be cleared, which is the only thing left to do with it.
    const user = userEvent.setup();
    await user.click(box("Unit tests pass"));
    await waitFor(() =>
      expect(api.SetDoneAgreementTick).toHaveBeenCalledWith("p1", 1, "PLAT-412", "Unit tests pass", false));
  });

  it("says where the ticks stay, since they are the one write that never reaches Jira", async () => {
    renderSection();
    expect(await screen.findByText("Kept in TAM. Not sent to Jira and not pushed on Commit.")).toBeInTheDocument();
  });

  it("says the agreement has no items rather than drawing an empty list", async () => {
    vi.mocked(api.ListRitualDocuments).mockResolvedValue([]);
    vi.mocked(api.DoneAgreementTicks).mockResolvedValue({});
    renderSection();
    expect(await screen.findByText("This board's done agreement has no items yet. The Rituals view is where a team writes them.")).toBeInTheDocument();
  });

  it("offers the read again when it failed", async () => {
    vi.mocked(api.ListRitualDocuments).mockRejectedValue(new Error("the store is closed"));
    renderSection();
    expect(await screen.findByText(/the store is closed/)).toBeInTheDocument();
    vi.mocked(api.ListRitualDocuments).mockResolvedValue([boardDoc, sprintDoc]);
    await userEvent.setup().click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("checkbox", { name: "Unit tests pass" })).toBeChecked();
  });
});

describe("agreementProgressLine", () => {
  it("counts the ticked items against the ones the agreement states", () => {
    expect(agreementProgressLine([
      { text: "a", ticked: true, stated: true },
      { text: "b", ticked: false, stated: true },
      { text: "c", ticked: true, stated: false },
    ])).toBe("1 of 2");
  });

  it("is nothing at all while there is no agreement to count against", () => {
    expect(agreementProgressLine(undefined)).toBeUndefined();
    expect(agreementProgressLine([])).toBeUndefined();
  });
});
