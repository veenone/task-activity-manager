import type { DragEvent, KeyboardEvent, MouseEvent, ReactNode } from "react";
import type { Issue } from "../api";
import type { CardMove } from "../lib/cardMoveState";
import { TypeChip } from "./TypeChip";

interface Props {
  issue: Issue;
  // selected is the one card the detail panel is about; checked is one card
  // of the multi-selection a bulk action will touch. They are two models on
  // one surface, so they paint differently, the way XTM's row-selected and
  // row-checked do.
  selected: boolean;
  checked: boolean;
  focused: boolean;
  // columnName is on the card, not only on the column head: the card carries
  // no status chip, so the column is the only thing holding the status, and
  // a screen reader that never hears it is reading a list of summaries.
  columnName: string;
  // colIndex is the card's one-based column, for aria-colindex.
  colIndex: number;
  posId: string;
  // move is what this card's position is doing: pending, being checked,
  // warned, failed, or held back. A card that will never land must not
  // look like one that is about to.
  move: CardMove;
  // flashed marks the card this view has just moved, for the two seconds
  // after it lands.
  flashed: boolean;
  dragging: boolean;
  // draggable is false while a commit is pushing. The move bindings take
  // no busy guard, matching every other local write, so this is the honest
  // way to say "not now" rather than a guard invented for one surface.
  draggable: boolean;
  // menu is the card's own move menu, rendered in the head beside the key.
  menu: ReactNode;
  // onSelect is given the click itself, because which gesture it was is the
  // whole question: plain selects, control toggles the check, shift extends
  // the run.
  onSelect: (e: MouseEvent<HTMLDivElement>) => void;
  onFocus: () => void;
  onKeyDown: (e: KeyboardEvent) => void;
  onDragStart: (e: DragEvent) => void;
  onDragEnd: () => void;
}

// The class each move state paints the card with. The pending move gets a
// dot of its own rather than the plain pending dot: a moved card's
// position is what is provisional, and a card carrying a pending summary
// edit is not making that claim.
const MOVE_CLASS: Record<string, string> = {
  checking: "board-card-checking",
  warned: "board-card-warn",
  failed: "board-card-failed",
  conflicted: "board-card-conflict",
};

// BoardCard is one card in one cell of the board's grid, so it takes the
// grid's own semantics rather than a button's. It is the surface the user
// made a move on, so it is the surface that reports what became of it.
export function BoardCard({
  issue, selected, checked, focused, columnName, colIndex, posId, move, flashed, dragging, draggable, menu,
  onSelect, onFocus, onKeyDown, onDragStart, onDragEnd,
}: Props) {
  const points = issue.storyPoints ?? null;
  const assignee = issue.assignee || "Unassigned";
  // A failed move is not pending in the sense the dot means: it was pushed
  // and refused, so a dot promising it will land is the one thing this card
  // must not say. Its border and the reason in its label carry it instead.
  const pendingMove = move.state !== "" && move.state !== "failed";
  const label = [`${issue.key} ${issue.summary} ${columnName}`, move.reason].filter(Boolean).join(". ");
  const className = [
    "board-card",
    selected ? "board-card-selected" : "",
    checked ? "board-card-checked" : "",
    MOVE_CLASS[move.state] ?? "",
    dragging ? "board-card-dragging" : "",
    flashed ? "board-card-moved" : "",
  ].filter(Boolean).join(" ");

  return (
    <div
      role="gridcell"
      // The grid is multi-selectable now, so a checked card is a selected
      // one to a screen reader whether or not the panel is about it.
      aria-selected={selected || checked}
      aria-colindex={colIndex}
      aria-label={label}
      // A failed move's reason otherwise lived only in the label above, so
      // a sighted user had to open the Pending changes dialog to read why
      // the card is marked. The hover costs nothing and says it in place.
      title={move.reason || undefined}
      tabIndex={focused ? 0 : -1}
      data-board-pos={posId}
      className={className}
      draggable={draggable}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onClick={onSelect}
      onFocus={onFocus}
      onKeyDown={onKeyDown}
    >
      <div className="board-card-head">
        <TypeChip type={issue.type} />
        <span className="accent-text">{issue.key}</span>
        {issue.draft && <span className="chip chip-draft">Draft</span>}
        {pendingMove && <span className="pending-dot pending-dot-move" role="img" aria-label="Pending move" />}
        {move.state === "" && issue.pending && (
          <span className="pending-dot" role="img" aria-label="Pending changes" />
        )}
        {menu}
      </div>
      <div>{issue.summary}</div>
      <div className="board-card-foot">
        <span>{assignee}</span>
        {points !== null && <span>{`${points} pts`}</span>}
      </div>
    </div>
  );
}
