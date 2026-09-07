import type { KeyboardEvent } from "react";
import type { UseQueryResult } from "@tanstack/react-query";
import type { Board, BoardView, Issue, Swimlane } from "../api";
import { BoardGrid } from "./BoardGrid";
import { BoardNotes } from "./BoardNotes";

interface Props {
  boards: UseQueryResult<Board[], Error>;
  view: UseQueryResult<BoardView, Error>;
  // unavailable is the profile setting the boards sync writes when this Jira
  // answered with no Agile API at all, which is a different empty than one a
  // sync would fill.
  unavailable: boolean;
  hasBoards: boolean;
  swimlane: Swimlane;
  selectedKey: string;
  focusId: string;
  canSync: boolean;
  onSync: () => void;
  onSelect: (issue: Issue) => void;
  onFocusCard: (id: string) => void;
  onKeyDown: (e: KeyboardEvent, id: string) => void;
}

// BoardBody is every state the board itself can be in, in the order they
// exclude each other: the boards list, then the composed view, then the
// board. Each state says what is missing and, where a sync would fix it,
// offers one.
export function BoardBody({
  boards, view, unavailable, hasBoards, swimlane, selectedKey, focusId, canSync,
  onSync, onSelect, onFocusCard, onKeyDown,
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
  const data = view.data;
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
  if (data.columns.length === 0) {
    return (
      <div className="pending-banner pending-banner-warn">
        <p>This board's configuration could not be read, so it has no columns.</p>
      </div>
    );
  }

  const cards = data.lanes.reduce((sum, lane) => sum + lane.count, 0);
  return (
    <>
      {cards === 0 ? (
        <p className="muted">
          {data.notSynced > 0
            ? `No cards in this sprint have been synced. ${data.notSynced} sit outside this project.`
            : "No cards in this sprint"}
        </p>
      ) : (
        <BoardGrid
          view={data}
          swimlane={swimlane}
          selectedKey={selectedKey}
          focusId={focusId}
          onSelect={onSelect}
          onFocusCard={onFocusCard}
          onKeyDown={onKeyDown}
        />
      )}
      <BoardNotes view={data} />
    </>
  );
}
