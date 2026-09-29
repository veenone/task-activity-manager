import type { UseQueryResult } from "@tanstack/react-query";
import { errMsg } from "@agile-suite/core";
import type { ColumnView } from "../api";
import {
  flowEmptyLine,
  flowHeadingLine,
  flowModeLine,
  flowNoColumnsLine,
  flowPendingLine,
  flowScopeLine,
} from "../lib/flowText";
import { counted } from "../lib/columnLimit";
import { CapacityTable } from "./CapacityTable";

// FlowReport is what the Reports view draws for a kanban board.
//
// A kanban board has no sprint, so none of SprintSummary, the charts or the
// velocity panel can answer for one. Column capacity is the figure that needs
// no sprint and no changelog: Go counts the board's own cards out of the synced
// cache against the limits the boards sync stored.
//
// It is deliberately short, and flowPendingLine is why it is allowed to be. A
// report showing one section reads as a report that failed to build the rest,
// so it says which figures are still coming. Throughput and cycle time need
// the changelog walk and arrive with it.
//
// The three states below are three different things for a reader to do, which
// is why they are told apart rather than flattened into one empty answer: a
// board whose columns nobody has synced needs the Boards view, a board whose
// cards are not in the store needs a project sync, and a failed read needs a
// retry.
export function FlowReport({
  capacity,
  onOpenBoards,
}: {
  capacity: UseQueryResult<ColumnView[], Error>;
  onOpenBoards?: () => void;
}) {
  if (capacity.isLoading) {
    return <p className="muted" role="status">Counting the board's columns</p>;
  }
  if (capacity.isError) {
    return (
      <p className="error-text" role="alert">
        Could not count this board's columns: {errMsg(capacity.error)}{" "}
        <button type="button" className="btn" onClick={() => void capacity.refetch()}>Retry</button>
      </p>
    );
  }

  const columns = capacity.data ?? [];
  // No columns is a board whose configuration nothing has read, so there is
  // nothing to place a card in. That is not an empty report.
  if (columns.length === 0) {
    return (
      <p className="muted" role="status">
        {flowNoColumnsLine()}
        {onOpenBoards && <>{" "}<button type="button" className="btn" onClick={onOpenBoards}>Open Boards</button></>}
      </p>
    );
  }
  // Columns that are known and hold nothing is a fact about the issue cache,
  // which is a different thing to fix, so it gets its own sentence.
  if (columns.every((c) => counted(c) === 0)) {
    return <p className="muted" role="status">{flowEmptyLine()}</p>;
  }

  return (
    <div className="report-summary">
      {/* The board is named in the head above, so the heading says what kind
          of figures these are rather than repeating it. */}
      <h3 className="report-heading" id="report-capacity-heading">{flowHeadingLine()}</h3>
      {/* The line that separates these figures from a sprint report's: they
          are true now, not for a period that has closed. A reader who takes
          them for a window's summary reads every one of them wrong, and
          nothing else on screen says which they are. */}
      <p className="report-mode report-mode-closed">{flowModeLine()}</p>
      <CapacityTable columns={columns} headingId="report-capacity-heading" />
      <p className="muted small">{flowScopeLine()}</p>
      <p className="muted small">{flowPendingLine()}</p>
    </div>
  );
}
