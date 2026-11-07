import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import { CyclesPage } from "./CyclesPage";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  Cycles: vi.fn(),
  TakeCycle: vi.fn(),
  GetCycle: vi.fn(),
  RetakeCycle: vi.fn(),
  DeleteCycle: vi.fn(),
}));

const draft = { id: 2, name: "Week 45", takenAt: "2026-11-07T10:00:00Z", publishedAt: "" };
const published = { id: 1, name: "Week 44", takenAt: "2026-10-31T10:00:00Z", publishedAt: "2026-10-31T12:00:00Z" };
const project = (key: string, verdict: string) => ({ key, verdict, reason: "", assessedAt: "" });
const detail = {
  ...draft,
  cases: [{ id: 1, key: "ACME-1", state: "open", notes: "", openedAt: "", assessed: 1, affected: 1, projects: [project("ALPHA", "affected")] }],
  changes: {
    since: "Week 44",
    new: ["ACME-3"],
    gone: ["ACME-2"],
    verdicts: [{ case: "ACME-1", project: "ALPHA", from: "investigating", to: "affected" }],
  },
};

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.Cycles).mockResolvedValue([draft, published]);
  vi.mocked(api.GetCycle).mockResolvedValue(detail as never);
});

function show() {
  render(
    <DialogProvider>
      <CyclesPage profileId="p1" result={null} />
    </DialogProvider>,
  );
  return userEvent.setup();
}

it("lists the cycles newest first and opens the newest with what changed and its grid", async () => {
  show();
  const list = await screen.findByRole("list", { name: "Cycles" });
  expect(
    within(list)
      .getAllByRole("button")
      .map((b) => b.textContent),
  ).toEqual(["Week 45Taken 2026-11-07, not published", "Week 44Taken 2026-10-31, published 2026-10-31"]);
  expect(await screen.findByText("Compared with Week 44, the last cycle published before this one.")).toBeVisible();
  expect(api.GetCycle).toHaveBeenCalledWith("p1", 2);
  const counts = screen.getByRole("list", { name: "Changes" });
  expect(
    within(counts)
      .getAllByRole("listitem")
      .map((l) => l.textContent),
  ).toEqual(["1 new cases", "1 cases gone", "1 verdicts changed"]);
  expect(screen.getByText("New: ACME-3")).toBeVisible();
  expect(screen.getByText("Gone: ACME-2")).toBeVisible();
  const moved = screen.getByRole("table", { name: "Verdicts that changed" });
  expect(
    within(moved)
      .getAllByRole("cell")
      .map((c) => c.textContent),
  ).toEqual(["ACME-1", "ALPHA", "investigating", "affected"]);
  const grid = screen.getByRole("table", { name: "Cases by project" });
  expect(within(grid).getByRole("cell", { name: "affected" })).toBeVisible();
});

it("says every case is new when no cycle was published before", async () => {
  vi.mocked(api.GetCycle).mockResolvedValue({ ...detail, changes: { since: "", new: ["ACME-1"], gone: [], verdicts: [] } } as never);
  show();
  expect(await screen.findByText("No cycle was published before this one, so every case counts as new.")).toBeVisible();
  expect(screen.queryByRole("table", { name: "Verdicts that changed" })).not.toBeInTheDocument();
});

it("takes a new cycle under the name given and opens it", async () => {
  const user = show();
  vi.mocked(api.TakeCycle).mockResolvedValue({ id: 3, name: "Week 46", takenAt: "2026-11-14T10:00:00Z", publishedAt: "" });
  vi.mocked(api.Cycles).mockResolvedValue([{ id: 3, name: "Week 46", takenAt: "2026-11-14T10:00:00Z", publishedAt: "" }, draft, published]);
  await user.click(await screen.findByRole("button", { name: "New cycle" }));
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByRole("textbox"), "Week 46");
  await user.click(within(dialog).getByRole("button", { name: "Take" }));

  expect(api.TakeCycle).toHaveBeenCalledWith("p1", "Week 46");
  expect(await screen.findByRole("button", { name: /^Week 46/, pressed: true })).toBeVisible();
  expect(api.GetCycle).toHaveBeenLastCalledWith("p1", 3);
});

it("shows why a cycle could not be taken", async () => {
  const user = show();
  vi.mocked(api.TakeCycle).mockRejectedValue("a cycle with that name already exists");
  await user.click(await screen.findByRole("button", { name: "New cycle" }));
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByRole("textbox"), "Week 45");
  await user.click(within(dialog).getByRole("button", { name: "Take" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("a cycle with that name already exists");
});

it("takes an unpublished cycle again, and deletes it only after confirming", async () => {
  const user = show();
  vi.mocked(api.RetakeCycle).mockResolvedValue({ ...detail, takenAt: "2026-11-07T11:00:00Z" } as never);
  await user.click(await screen.findByRole("button", { name: "Take again" }));
  expect(api.RetakeCycle).toHaveBeenCalledWith("p1", 2);

  await user.click(screen.getByRole("button", { name: "Delete" }));
  vi.mocked(api.Cycles).mockResolvedValue([published]);
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
  expect(api.DeleteCycle).toHaveBeenCalledWith("p1", 2);
  expect(await screen.findByRole("button", { name: /^Week 44/, pressed: true })).toBeVisible();
});

it("offers neither Take again nor Delete on a published cycle", async () => {
  vi.mocked(api.Cycles).mockResolvedValue([published]);
  vi.mocked(api.GetCycle).mockResolvedValue({ ...detail, ...published } as never);
  show();
  expect(await screen.findByText(/Compared with/)).toBeVisible();
  expect(screen.queryByRole("button", { name: "Take again" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
});

it("keeps the cycle on screen when Take again fails", async () => {
  const user = show();
  vi.mocked(api.RetakeCycle).mockRejectedValue("that cycle is published, so it stays as it was");
  await user.click(await screen.findByRole("button", { name: "Take again" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("that cycle is published");
  expect(screen.getByRole("table", { name: "Cases by project" })).toBeVisible();
});
