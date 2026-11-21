import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import type { PendingChange } from "../api";
import { groupPending } from "../queries/pending";
import { PendingGroupRows } from "./PendingGroupRows";

const draft: PendingChange = {
  id: 3, entityType: "issue_create", entityKey: "TAM-NEW-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "",
  afterVal: JSON.stringify({ type: "task", summary: "Add a retry to the webhook", description: "", priority: "Medium", labels: [], assignee: "", storyPoints: 0, extra: {} }),
};
const edit: PendingChange = { id: 2, entityType: "issue", entityKey: "PLAT-409", field: "priority", beforeVal: "Medium", afterVal: "High", baseVersion: "v1", createdAt: "" };

function show(row: PendingChange) {
  const onDiscard = vi.fn();
  render(
    <DialogProvider>
      <table>
        <PendingGroupRows group={groupPending([row])[0]} project="PLAT" heldReasons={[]} unpushableIds={new Set()} busy={false} onDiscard={onDiscard} />
      </table>
    </DialogProvider>,
  );
  return onDiscard;
}

describe("discarding a pending row (#535)", () => {
  it("asks before throwing away a draft issue, and keeps it on Cancel", async () => {
    const user = userEvent.setup();
    const onDiscard = show(draft);
    await user.click(screen.getByRole("button", { name: "Discard TAM-NEW-1" }));
    const ask = await screen.findByRole("alertdialog");
    expect(ask).toHaveTextContent("Add a retry to the webhook");
    await user.click(within(ask).getAllByRole("button", { name: "Cancel" }).at(-1)!);
    expect(onDiscard).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Discard TAM-NEW-1" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Discard" }));
    expect(onDiscard).toHaveBeenCalledWith(draft);
  });

  it("discards a field edit at once", async () => {
    const user = userEvent.setup();
    const onDiscard = show(edit);
    await user.click(screen.getByRole("button", { name: "Discard priority on PLAT-409" }));
    expect(onDiscard).toHaveBeenCalledWith(edit);
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });
});
