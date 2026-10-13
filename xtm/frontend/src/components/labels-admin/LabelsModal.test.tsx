import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider } from "@agile-suite/core";
import { LabelsModal } from "./LabelsModal";

const api = vi.hoisted(() => ({
  ListLabels: vi.fn(),
  RenameLabel: vi.fn(),
  DeleteLabel: vi.fn(),
}));
vi.mock("../../api", () => ({
  ...api,
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));
vi.mock("../../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

const done = { succeeded: ["QA-1"], failed: [] };

function renderModal() {
  const onChanged = vi.fn();
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <DialogProvider>
        <LabelsModal onChanged={onChanged} onClose={vi.fn()} />
      </DialogProvider>
    </QueryClientProvider>,
  );
  return onChanged;
}

async function rows() {
  const table = await screen.findByRole("table");
  return within(table)
    .getAllByRole("row")
    .slice(1)
    .map((r) => within(r).getAllByRole("cell").slice(0, 2).map((c) => c.textContent));
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.ListLabels.mockResolvedValue([
    { label: "login", count: 1 },
    { label: "smok", count: 1 },
    { label: "smoke", count: 12 },
  ]);
});

describe("LabelsModal", () => {
  it("lists every label with its test count, filtered and sorted", async () => {
    renderModal();
    expect(await rows()).toEqual([
      ["login", "1"],
      ["smok", "1"],
      ["smoke", "12"],
    ]);

    await userEvent.type(screen.getByLabelText("Filter labels"), "smo");
    expect(await rows()).toEqual([
      ["smok", "1"],
      ["smoke", "12"],
    ]);

    await userEvent.selectOptions(screen.getByTitle("Sort field"), "tests");
    await userEvent.click(screen.getByRole("button", { name: "Toggle sort direction" }));
    expect((await rows())[0]).toEqual(["smoke", "12"]);
  });

  it("merges into an existing label after saying how many changes it queues", async () => {
    api.RenameLabel.mockResolvedValue(done);
    const onChanged = renderModal();

    await userEvent.click(await screen.findByRole("button", { name: "Rename smok" }));
    const prompt = screen.getByRole("dialog", { name: "Rename smok" });
    const input = within(prompt).getByRole("textbox");
    await userEvent.clear(input);
    await userEvent.type(input, "smoke");
    await userEvent.click(within(prompt).getByRole("button", { name: "Next" }));

    const confirm = await screen.findByRole("alertdialog");
    expect(within(confirm).getByText('Merge "smok" into "smoke"?')).toBeTruthy();
    expect(within(confirm).getByText(/queues 1 pending change,/)).toBeTruthy();
    await userEvent.click(within(confirm).getByRole("button", { name: "Merge" }));

    expect(api.RenameLabel).toHaveBeenCalledWith("p1", "smok", "smoke");
    expect(onChanged).toHaveBeenCalled();
  });

  it("deletes only once confirmed", async () => {
    api.DeleteLabel.mockResolvedValue(done);
    renderModal();

    await userEvent.click(await screen.findByRole("button", { name: "Delete smoke" }));
    let confirm = await screen.findByRole("alertdialog");
    expect(within(confirm).getByText(/queues 12 pending changes,/)).toBeTruthy();
    await userEvent.click(within(confirm).getByText("Cancel", { selector: "button" }));
    expect(api.DeleteLabel).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "Delete smoke" }));
    confirm = await screen.findByRole("alertdialog");
    await userEvent.click(within(confirm).getByRole("button", { name: "Delete" }));
    expect(api.DeleteLabel).toHaveBeenCalledWith("p1", "smoke");
  });

  it("shows why a change was refused", async () => {
    api.DeleteLabel.mockRejectedValue(
      new Error('Commit or discard the 2 pending label edits on tests that use "login" first.'),
    );
    renderModal();
    await userEvent.click(await screen.findByRole("button", { name: "Delete login" }));
    await userEvent.click(
      within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }),
    );
    expect(
      await screen.findByText('Commit or discard the 2 pending label edits on tests that use "login" first.'),
    ).toBeTruthy();
  });
});
