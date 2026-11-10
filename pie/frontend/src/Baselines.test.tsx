import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import { BaselinesDialog } from "./BaselinesDialog";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  JiraProjects: vi.fn(),
  AddBaseline: vi.fn(),
  AddDescendants: vi.fn(),
  RemoveBaseline: vi.fn(),
  RemoveDescendant: vi.fn(),
}));

const base = { key: "BASE", descendants: [] as string[] };

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.JiraProjects).mockResolvedValue([
    { key: "BASE", name: "Card OS" },
    { key: "DELTA", name: "Reader" },
  ] as never);
});

function show(baselines: { key: string; descendants: string[] }[] = []) {
  const onChange = vi.fn();
  render(
    <DialogProvider>
      <BaselinesDialog profileId="p1" baselines={baselines} onChange={onChange} onClose={vi.fn()} />
    </DialogProvider>,
  );
  return { user: userEvent.setup(), onChange };
}

it("marks a baseline and puts a project under it, naming both", async () => {
  vi.mocked(api.AddBaseline).mockResolvedValue([base]);
  vi.mocked(api.AddDescendants).mockResolvedValue([{ key: "BASE", descendants: ["DELTA"] }]);
  const { user, onChange } = show();

  await user.type(screen.getByLabelText("New baseline"), "base");
  await user.click(screen.getByRole("button", { name: "Add baseline" }));
  expect(api.AddBaseline).toHaveBeenCalledWith("p1", "BASE");
  const group = await screen.findByRole("group", { name: "Card OS (BASE)" });

  await user.type(within(group).getByLabelText("Add a descendant of Card OS (BASE)"), "DELTA");
  await user.click(within(group).getByRole("button", { name: "Add" }));
  expect(api.AddDescendants).toHaveBeenCalledWith("p1", "BASE", ["DELTA"]);
  expect(await within(group).findByText("Reader (DELTA)")).toBeVisible();
  expect(onChange).toHaveBeenLastCalledWith([{ key: "BASE", descendants: ["DELTA"] }]);
});

it("says why a project cannot go under a baseline", async () => {
  vi.mocked(api.AddDescendants).mockRejectedValue("DELTA is already a descendant of AURA");
  const { user } = show([base]);
  const group = screen.getByRole("group", { name: /BASE/ });
  await user.type(within(group).getByRole("combobox"), "DELTA");
  await user.click(within(group).getByRole("button", { name: "Add" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("DELTA is already a descendant of AURA");
});

it("asks before removing a baseline, then takes a descendant out on its own", async () => {
  vi.mocked(api.RemoveDescendant).mockResolvedValue([base]);
  vi.mocked(api.RemoveBaseline).mockResolvedValue([]);
  const { user, onChange } = show([{ key: "BASE", descendants: ["DELTA"] }]);
  await screen.findByRole("group", { name: "Card OS (BASE)" });

  await user.click(screen.getByRole("button", { name: "Remove Reader (DELTA)" }));
  expect(api.RemoveDescendant).toHaveBeenCalledWith("p1", "DELTA");

  await user.click(screen.getByRole("button", { name: "Remove Card OS (BASE)" }));
  const ask = await screen.findByRole("alertdialog");
  expect(ask).toHaveTextContent("Remove Card OS (BASE)?");
  await user.click(within(ask).getByRole("button", { name: "Remove" }));
  expect(api.RemoveBaseline).toHaveBeenCalledWith("p1", "BASE");
  expect(onChange).toHaveBeenLastCalledWith([]);
});

it("closes only the suggestion list on Escape, not the dialog", async () => {
  const onClose = vi.fn();
  render(
    <DialogProvider>
      <BaselinesDialog profileId="p1" baselines={[]} onChange={vi.fn()} onClose={onClose} />
    </DialogProvider>,
  );
  const user = userEvent.setup();
  const box = screen.getByRole("combobox", { name: "New baseline" });
  await user.click(box);
  await screen.findByRole("option", { name: "Card OS (BASE)" });
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  expect(onClose).not.toHaveBeenCalled();
});
