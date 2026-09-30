import type { Board } from "../api";
import { useBoard } from "../queries/boards";
import { kanbanDocument } from "../lib/reportDocument";
import { kanbanEmptyLine, kanbanUnsyncedLine } from "../lib/reportText";
import { CapacityPanel } from "./CapacityPanel";
import { ReportOutputs } from "./ReportOutputs";

// KanbanReport is the Reports view for a board with no sprint: its columns
// against their limits, and nothing else, because nothing else about a kanban
// board has been read. Issue #119.
//
// The columns come from GetBoard, which is the read the Boards view already
// makes and the same composition boardrepo.ColumnHeads keeps the heads of.
// So there is no sprint id in any of this, no changelog walk, no per-profile
// lock, no progress frame and nothing to cancel, and the numbers here are the
// numbers on the board's own column heads rather than a second count of them.
//
// It draws the body and the outputs bar but not the frame around them: the
// board picker above it belongs to the view, which offers both kinds of board
// and decides which report a pick means.
//
// The two empty boards are two states because they are two different jobs.
// Columns missing from the cache are the Boards view's refresh, so that one
// carries the button; columns that hold nothing are an issue sync, and a team
// sent to Boards from there would refresh the columns it already has.
export function KanbanReport({
  profileId,
  board,
  onOpenBoards,
}: {
  profileId: string;
  board: Board;
  onOpenBoards?: () => void;
}) {
  const view = useBoard(profileId, board.id, "", "none", true);
  const columns = view.data?.columns ?? [];
  // A column count of nothing across every column is a board whose columns
  // arrived and whose cards did not. total rather than the limit's own count,
  // because a board that excludes subtasks from its limits still holds them.
  const nothingSynced = columns.length > 0 && columns.every((c) => c.total === 0);
  // Null for a board with no columns cached, which is what leaves the three
  // publishers disabled with their own reason beside them. Cheap enough to
  // word on every render: it is a table of a handful of rows.
  const doc = kanbanDocument(board.name, columns);
  const publishable = !!doc && !nothingSynced;

  function body() {
    if (view.isError) {
      return (
        <p className="error-text" role="alert">
          Could not read this board's columns: {view.error.message}{" "}
          <button type="button" className="btn" onClick={() => void view.refetch()}>Retry</button>
        </p>
      );
    }
    if (!view.data) return <p className="muted" role="status">Reading the board's columns</p>;
    if (columns.length === 0) {
      return (
        <p className="muted" role="status">
          {kanbanUnsyncedLine()}
          {onOpenBoards && <>{" "}<button type="button" className="btn" onClick={onOpenBoards}>Open Boards</button></>}
        </p>
      );
    }
    if (nothingSynced) return <p className="muted" role="status">{kanbanEmptyLine()}</p>;
    // The section the three publishers lay out, drawn by the component that
    // draws a sprint report's. One set of rows, worded once.
    return <CapacityPanel section={doc?.sections[0]} />;
  }

  return (
    <>
      <div className="report-body">{body()}</div>
      {publishable && (
        <div className="report-outputs">
          <ReportOutputs profileId={profileId} boardId={board.id} doc={doc} />
        </div>
      )}
    </>
  );
}
