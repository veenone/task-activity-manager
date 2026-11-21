import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PendingChangesModal } from "./PendingChangesModal";
import { DirtyDot } from "./test-detail/rows";
import { renderCell } from "./test-table/cells";
import type { CommitResult, TestCase } from "../api";

vi.mock("./useConfirm", () => ({ useConfirm: () => ({ confirm: vi.fn(async () => true) }) }));

function show(lastResult: CommitResult) {
  render(
    <PendingChangesModal
      changes={[]} onDiscard={vi.fn()} onDiscardAll={vi.fn()} onCommit={vi.fn()} onCommitIds={vi.fn()}
      onJumpTo={vi.fn()} onResolveOverride={vi.fn()} onResolveKeepRemote={vi.fn()} onResolveMerge={vi.fn()}
      onResolveRecreate={vi.fn()} onClose={vi.fn()} committing={false} lastResult={lastResult}
    />,
  );
}

describe("commit results reach a screen reader (#537)", () => {
  it("announces what landed as status and what failed as an alert", () => {
    show({ succeeded: ["QA-1"], conflicted: [], failed: [{ testKey: "QA-2", error: "Jira said no" }] } as CommitResult);
    expect(screen.getByRole("status")).toHaveTextContent("Committed 1 test to Jira.");
    expect(screen.getByRole("alert")).toHaveTextContent("QA-2: Jira said no");
  });
});

describe("pending markers carry text (#537)", () => {
  it("names the dot on a detail row", () => {
    render(<DirtyDot />);
    expect(screen.getByRole("img", { name: "Pending edit" })).toHaveTextContent("●");
  });

  it("names the dot in the test table", () => {
    render(
      <table><tbody><tr>{renderCell("key", { key: "QA-7" } as TestCase, true, false, {})}</tr></tbody></table>,
    );
    expect(screen.getByRole("img", { name: "Pending edits" })).toHaveTextContent("●");
  });
});
