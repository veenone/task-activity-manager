import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PendingChangesModal } from "./PendingChangesModal";
import type { PendingChange } from "../api";

const confirm = vi.fn();
vi.mock("./useConfirm", () => ({ useConfirm: () => ({ confirm }) }));

const created = {
  id: 7, entityType: "test_create", entityKey: "NEW-1", field: "create", beforeVal: "",
  afterVal: JSON.stringify({ summary: "Login rejects an expired token" }), baseVersion: "", createdAt: "", transitionFields: "",
} as PendingChange;
const edited = {
  id: 8, entityType: "test_case", entityKey: "QA-4", field: "summary", beforeVal: "Old", afterVal: "New",
  baseVersion: "", createdAt: "", transitionFields: "",
} as PendingChange;

function show() {
  const onDiscard = vi.fn();
  const onCommitIds = vi.fn();
  render(
    <PendingChangesModal
      changes={[created, edited]} onDiscard={onDiscard} onDiscardAll={vi.fn()} onCommit={vi.fn()} onCommitIds={onCommitIds}
      onJumpTo={vi.fn()} onResolveOverride={vi.fn()} onResolveKeepRemote={vi.fn()} onResolveMerge={vi.fn()}
      onResolveRecreate={vi.fn()} onClose={vi.fn()} committing={false} lastResult={null}
    />,
  );
  return { onDiscard, onCommitIds };
}

describe("discarding a pending row (#535)", () => {
  beforeEach(() => confirm.mockReset());

  it("asks before discarding a new test, and keeps it when the user says no", async () => {
    const user = userEvent.setup();
    const { onDiscard } = show();
    confirm.mockResolvedValueOnce(false);
    await user.click(screen.getByRole("button", { name: "Discard new test on NEW-1" }));
    expect(confirm).toHaveBeenCalledWith(expect.objectContaining({ message: expect.stringContaining("Login rejects an expired token") }));
    expect(onDiscard).not.toHaveBeenCalled();

    confirm.mockResolvedValueOnce(true);
    await user.click(screen.getByRole("button", { name: "Discard new test on NEW-1" }));
    expect(onDiscard).toHaveBeenCalledWith(7);
  });

  it("discards a field edit at once and names the row on both buttons", async () => {
    const user = userEvent.setup();
    const { onDiscard, onCommitIds } = show();
    await user.click(screen.getByRole("button", { name: "Discard summary on QA-4" }));
    expect(confirm).not.toHaveBeenCalled();
    expect(onDiscard).toHaveBeenCalledWith(8);
    await user.click(screen.getByRole("button", { name: "Commit summary on QA-4" }));
    expect(onCommitIds).toHaveBeenCalledWith([8]);
  });
});
