import type { DragEvent, KeyboardEvent } from "react";
import { announce } from "@agile-suite/core";
import type { Issue } from "../api";
import { TypeChip } from "./TypeChip";

interface Props {
  issue: Issue;
  selected: boolean;
  focused: boolean;
  // columnName is on the card, not only on the column head: the card carries
  // no status chip, so the column is the only thing holding the status, and
  // a screen reader that never hears it is reading a list of summaries.
  columnName: string;
  // colIndex is the card's one-based column, for aria-colindex.
  colIndex: number;
  posId: string;
  onSelect: () => void;
  onFocus: () => void;
  onKeyDown: (e: KeyboardEvent) => void;
}

// BoardCard is one card in one cell of the board's grid, so it takes the
// grid's own semantics rather than a button's. Phase 3a is read only: the
// card refuses a drag where the drag happens, because the caveat line above
// the board is prevention and this is the answer at the moment the user
// actually asks the question.
export function BoardCard({
  issue, selected, focused, columnName, colIndex, posId, onSelect, onFocus, onKeyDown,
}: Props) {
  const points = issue.storyPoints ?? null;
  const assignee = issue.assignee || "Unassigned";

  function onDragStart(e: DragEvent) {
    e.preventDefault();
    announce("Read only for now. Dragging arrives in the next release.");
  }

  return (
    <div
      role="gridcell"
      aria-selected={selected}
      aria-colindex={colIndex}
      aria-label={`${issue.key} ${issue.summary} ${columnName}`}
      tabIndex={focused ? 0 : -1}
      data-board-pos={posId}
      className={`board-card${selected ? " board-card-selected" : ""}`}
      draggable={false}
      onDragStart={onDragStart}
      onClick={onSelect}
      onFocus={onFocus}
      onKeyDown={onKeyDown}
    >
      <div className="board-card-head">
        <TypeChip type={issue.type} />
        <span className="accent-text">{issue.key}</span>
        {issue.draft && <span className="chip chip-draft">Draft</span>}
        {issue.pending && <span className="pending-dot" role="img" aria-label="Pending changes" />}
      </div>
      <div>{issue.summary}</div>
      <div className="board-card-foot">
        <span>{assignee}</span>
        {points !== null && <span>{`${points} pts`}</span>}
      </div>
    </div>
  );
}
