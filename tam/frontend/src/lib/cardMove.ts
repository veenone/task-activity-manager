import type { BoardView } from "../api";
import type { Pos } from "./boardCells";

// cardMove is the arithmetic of moving a card: where a drop lands inside a
// cell, which column a key press steps to, and whether either of them
// changes anything. It is its own module because the drag handlers and the
// keyboard handlers both ask the same questions, and because arithmetic
// with no DOM and no React in it is the part worth testing directly.

// Rect is the part of a DOMRect this file reads. Taking the two numbers
// rather than the DOMRect keeps the arithmetic testable without a layout.
export interface Rect {
  top: number;
  bottom: number;
}

// Drop is where a cursor landed inside a cell: the gap it points at, and
// the card it is measured against. index is 0 for the top of the cell and
// cards.length for below the last one. neighbourKey is "" only when the
// cell holds no cards at all, which is a column move and not a rank.
export interface Drop {
  index: number;
  neighbourKey: string;
  before: boolean;
}

// columnDrop reads a drop position out of the cursor. A card's own
// midpoint is the line: above it the drop goes before that card, below it
// after. rects is parallel to keys; a key with no rect (a card the browser
// has not laid out) is skipped rather than measured against zero.
export function columnDrop(keys: string[], clientY: number, rects: Rect[]): Drop {
  let index = keys.length;
  for (let i = 0; i < keys.length; i++) {
    const r = rects[i];
    if (!r) continue;
    if (clientY < (r.top + r.bottom) / 2) {
      index = i;
      break;
    }
  }
  if (keys.length === 0) return { index: 0, neighbourKey: "", before: false };
  if (index === 0) return { index: 0, neighbourKey: keys[0], before: true };
  return { index, neighbourKey: keys[index - 1], before: false };
}

// isSameCell says whether a position is in the cell a drop is over. A move
// across lanes is not one of this phase's moves: a lane is an assignee or
// an epic, and neither is reassigned by dragging a card.
export function isSameCell(p: Pos | undefined, lane: number, col: number): boolean {
  return !!p && p.lane === lane && p.col === col;
}

// isNoMove is the drop that changes nothing: the card's own cell, at its
// own place or immediately after it. Both gaps beside a card leave it
// exactly where it is, and promising a move there and doing nothing reads
// as a bug rather than as a no-op.
export function isNoMove(p: Pos | undefined, lane: number, col: number, index: number): boolean {
  return isSameCell(p, lane, col) && !!p && (index === p.index || index === p.index + 1);
}

// KeyMove is what one keyboard move resolves to: a column to move into, a
// place in the cell to rank to, or a refusal with the sentence that says
// why. The refusal is a first-class answer because a silent key press on
// the edge of a board tells a screen reader nothing.
export type KeyMove =
  | { kind: "column"; col: number }
  | { kind: "rank"; index: number; neighbourKey: string; before: boolean }
  | { kind: "refused"; message: string };

// cardsIn is the cell's card list, or an empty one.
function cardsIn(view: BoardView, lane: number, col: number) {
  return view.lanes[lane]?.cells[col] ?? [];
}

// keyboardMove answers Ctrl and an arrow with the move it makes. Left and
// Right step exactly one column, empty or not: boardCells.moveFocus
// deliberately skips an empty column so focus never strands in one, and
// reusing it here would teleport a card past the empty column the user was
// aiming at, which is where a card usually goes. Up and Down step one
// place inside the cell.
export function keyboardMove(view: BoardView, p: Pos, issueKey: string, key: string): KeyMove | undefined {
  switch (key) {
    case "ArrowRight":
    case "ArrowLeft":
      return columnStep(view, p, issueKey, key === "ArrowRight" ? 1 : -1);
    case "ArrowUp":
    case "ArrowDown":
      return rankStep(view, p, issueKey, key === "ArrowDown" ? 1 : -1);
    default:
      return undefined;
  }
}

function columnStep(view: BoardView, p: Pos, issueKey: string, step: number): KeyMove {
  const col = p.col + step;
  if (col < 0) return { kind: "refused", message: `${issueKey} is already in the first column` };
  if (col >= view.columns.length) {
    return { kind: "refused", message: `${issueKey} is already in the last column` };
  }
  const column = view.columns[col];
  if ((column.statusIds ?? []).length === 0) {
    return { kind: "refused", message: `${column.name} collects no status, so a card cannot be moved into it` };
  }
  return { kind: "column", col };
}

function rankStep(view: BoardView, p: Pos, issueKey: string, step: number): KeyMove {
  const cards = cardsIn(view, p.lane, p.col);
  const index = p.index + step;
  if (index < 0) return { kind: "refused", message: `${issueKey} is already at the top of this column` };
  if (index >= cards.length) {
    const hidden = view.lanes[p.lane]?.overflow[p.col] ?? 0;
    if (hidden > 0) {
      return { kind: "refused", message: `${issueKey} cannot move past the cards this column is not showing` };
    }
    return { kind: "refused", message: `${issueKey} is already at the bottom of this column` };
  }
  return { kind: "rank", index, neighbourKey: cards[index].key, before: step < 0 };
}

// The move a view has just made, kept until the board query has settled
// and the card can be found where it landed. kind is what to say about it;
// target names a column or a sprint for the moves that have one, and at
// stamps the intent so a second identical move is still a new one.
export interface MoveIntent {
  key: string;
  kind: "column" | "sprint" | "up" | "down";
  target: string;
  at: number;
}

// landedMessage is what a move announces once the board has redrawn: where
// the card is now, and how far down the column it sits, because a column
// head that recounts silently tells a screen reader nothing. A card that
// left the board altogether, which a sprint move does, is named by its
// destination alone.
export function landedMessage(intent: MoveIntent, view: BoardView, p: Pos | undefined): string {
  if (!p) return `${intent.key} moved to ${intent.target || "the backlog"}`;
  const total = cardsIn(view, p.lane, p.col).length;
  const place = `${p.index + 1} of ${total}`;
  switch (intent.kind) {
    case "column":
      return `${intent.key} moved to ${view.columns[p.col]?.name ?? intent.target}, ${place}`;
    case "sprint":
      return `${intent.key} moved to ${intent.target || "the backlog"}, ${place}`;
    case "up":
      return `${intent.key} moved up, ${place}`;
    default:
      return `${intent.key} moved down, ${place}`;
  }
}
