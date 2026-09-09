import type { KeyboardEvent, MouseEvent } from "react";
import type { UseQueryResult } from "@tanstack/react-query";
import type { Board, BoardView, Issue, Sprint, Swimlane } from "../api";
import { BoardGrid } from "./BoardGrid";
import { BoardNotes } from "./BoardNotes";
import type { BoardMoves } from "./useBoardMoves";

interface Props {
  boards: UseQueryResult<Board[], Error>;
  view: UseQueryResult<BoardView, Error>;
  // filtered is the view the grid draws: the query's own board with the
  // toolbar's filter applied. The query result is still what says whether
  // the board loaded, failed, or is empty, so the two travel together.
  filtered: BoardView | undefined;
  // unavailable is the profile setting the boards sync writes when this Jira
  // answered with no Agile API at all, which is a different empty than one a
  // sync would fill.
  unavailable: boolean;
  hasBoards: boolean;
  // hasSprint says a sprint is in scope. A kanban board has none, and
  // neither does a scrum board with no open sprint, so the empty state
  // must not call the board's own list a sprint.
  hasSprint: boolean;
  swimlane: Swimlane;
  selectedKey: string;
  // checked is the multi-selection, by key, which the grid paints and the
  // selection bar acts on.
  checked: ReadonlySet<string>;
  focusId: string;
  // The move half of the board: the writes, the drag state, and what each
  // card's move is doing, plus the key of the card that has just landed.
  moves: BoardMoves;
  flashKey: string;
  sprints: Sprint[];
  sprintId: string;
  committing: boolean;
  canSync: boolean;
  onSync: () => void;
  onSelect: (issue: Issue, e: MouseEvent<HTMLDivElement>) => void;
  onFocusCard: (id: string) => void;
  onKeyDown: (e: KeyboardEvent, id: string) => void;
}

// BoardBody is every state the board itself can be in, in the order they
// exclude each other: the boards list, then the composed view, then the
// board. Each state says what is missing and, where a sync would fix it,
// offers one.
export function BoardBody({
  boards, view, filtered, unavailable, hasBoards, hasSprint, swimlane, selectedKey, checked, focusId, moves, flashKey,
  sprints, sprintId, committing, canSync, onSync, onSelect, onFocusCard, onKeyDown,
}: Props) {
  if (boards.isError) {
    return (
      <p className="error-text">
        Could not load the boards: {boards.error.message}{" "}
        <button type="button" className="btn" onClick={() => void boards.refetch()}>Retry</button>
      </p>
    );
  }
  if (boards.isLoading) return <p className="muted">Loading the board</p>;
  if (!hasBoards) {
    return (
      <p className="muted">
        {unavailable
          ? "This Jira has no boards"
          : "This project has no boards in Jira, or the sync has not run"}
      </p>
    );
  }
  if (view.isError) {
    return (
      <p className="error-text">
        Could not load the board: {view.error.message}{" "}
        <button type="button" className="btn" onClick={() => void view.refetch()}>Retry</button>
      </p>
    );
  }
  const data = filtered ?? view.data;
  if (!data) return <p className="muted">Loading the board</p>;

  // Right after the version 4 upgrade no cached issue carries a status id
  // yet, so there is nothing to bucket into a column. Drawing an empty board
  // and blaming the user for it would be the worst first impression this
  // feature could make; the next step is the sync itself.
  if (data.needsStatusSync) {
    return (
      <p className="muted">
        Sync to draw this board{" "}
        <button type="button" className="btn" disabled={!canSync} onClick={onSync}>Sync</button>
      </p>
    );
  }
  // A board's row and its columns are written in one transaction, so no
  // columns is Jira's own answer rather than a half-written board.
  if (data.columns.length === 0) {
    return (
      <div className="pending-banner pending-banner-warn">
        <p>Jira reports no columns for this board, so there is nothing to draw.</p>
      </div>
    );
  }

  const cards = data.lanes.reduce((sum, lane) => sum + lane.count, 0);
  // The cache is all the empty state knows about: a key with no cached row
  // may be in another project, or simply not synced yet, and saying which
  // would be a guess.
  const nothing = hasSprint ? "No cards in this sprint" : "No cards on this board";
  const nothingSynced = hasSprint
    ? "No cards in this sprint have been synced."
    : "No cards on this board have been synced.";
  return (
    <>
      {cards === 0 ? (
        <p className="muted">
          {data.notSynced > 0
            ? `${nothingSynced} ${data.notSynced} ${data.notSynced === 1 ? "card is" : "cards are"} on it but not in this cache.`
            : nothing}
        </p>
      ) : (
        <>
          {/* Nothing else on screen says the moves exist, and the grid
              names this line in its aria-describedby. */}
          <p className="muted small board-move-keys" id="board-move-keys">
            Drag a card to move it, or focus one and hold Ctrl with an arrow key. Space checks a card and Shift with an
            arrow checks a run of them. Shift and F10 open a card's move menu.
          </p>
          <BoardGrid
            view={data}
            swimlane={swimlane}
            selectedKey={selectedKey}
            checked={checked}
            focusId={focusId}
            moves={moves}
            flashKey={flashKey}
            sprints={sprints}
            sprintId={sprintId}
            committing={committing}
            onSelect={onSelect}
            onFocusCard={onFocusCard}
            onKeyDown={onKeyDown}
          />
        </>
      )}
      <BoardNotes view={data} />
    </>
  );
}
