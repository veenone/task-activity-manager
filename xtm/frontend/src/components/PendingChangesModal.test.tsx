import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PendingChangesModal } from "./PendingChangesModal";
import type { PendingChange } from "../api";

// useConfirm wraps DialogContext, which is not mounted here.
vi.mock("./useConfirm", () => ({
  useConfirm: () => ({ confirm: vi.fn(async () => true) }),
}));

function datasetChange(over: Partial<PendingChange> = {}): PendingChange {
  return {
    id: 1,
    entityType: "test_dataset",
    entityKey: "QA-1",
    field: "dataset",
    beforeVal: '{"parameters":[],"rows":[]}',
    afterVal:
      '{"parameters":[{"name":"USERNAME","combinatorial":false},' +
      '{"name":"REGION","combinatorial":false}],' +
      '"rows":[["alice","eu-west"],["bob","us-east"]]}',
    baseVersion: "",
    createdAt: "2026-10-16T00:00:00Z",
    transitionFields: "",
    ...over,
  } as PendingChange;
}

function show(changes: PendingChange[]) {
  render(
    <PendingChangesModal
      changes={changes}
      onDiscard={vi.fn()}
      onDiscardAll={vi.fn()}
      onCommit={vi.fn()}
      onCommitIds={vi.fn()}
      onJumpTo={vi.fn()}
      onResolveOverride={vi.fn()}
      onResolveKeepRemote={vi.fn()}
      onResolveMerge={vi.fn()}
      onResolveRecreate={vi.fn()}
      onClose={vi.fn()}
      committing={false}
      lastResult={null}
    />,
  );
}

describe("PendingChangesModal", () => {
  // The journal carries the whole table as JSON. Printing it in a table cell
  // tells the reader nothing and pushes everything else off screen (#277 W1).
  it("sizes a queued dataset instead of printing it", () => {
    show([datasetChange()]);

    expect(screen.getByText("dataset")).toBeInTheDocument();
    expect(screen.getByText("none")).toBeInTheDocument();
    expect(screen.getByText("2 parameters, 2 rows")).toBeInTheDocument();
    expect(screen.queryByText(/eu-west/)).not.toBeInTheDocument();
  });

  it("counts one parameter and one row in the singular", () => {
    show([
      datasetChange({
        afterVal: '{"parameters":[{"name":"USERNAME","combinatorial":false}],"rows":[["alice"]]}',
      }),
    ]);

    expect(screen.getByText("1 parameter, 1 row")).toBeInTheDocument();
  });
});
