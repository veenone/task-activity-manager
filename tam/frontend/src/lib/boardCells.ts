import type { BoardView, Issue } from "../api";

// A card's place on the board: which lane band, which column, and where in
// that cell's list. The board addresses cards by position rather than by
// issue key so a lane, a column, and a cell can all be walked with the same
// arithmetic, the way lib/epicTreeItems flattens the tree for EpicTree.
export interface Pos {
  lane: number;
  col: number;
  index: number;
}

// posId is the focus token the view keeps and the DOM carries, so the
// focused card can be found again after a re-render.
export function posId(p: Pos): string {
  return `${p.lane}-${p.col}-${p.index}`;
}

export function parsePos(id: string): Pos | null {
  const parts = id.split("-");
  if (parts.length !== 3) return null;
  const [lane, col, index] = parts.map(Number);
  if ([lane, col, index].some((n) => !Number.isInteger(n) || n < 0)) return null;
  return { lane, col, index };
}

// cardAtPos reads one card out of the view by its position, which the view
// and its keyboard both need and neither owns.
export function cardAtPos(view: BoardView, p: Pos | undefined): Issue | undefined {
  if (!p) return undefined;
  return view.lanes[p.lane]?.cells[p.col]?.[p.index];
}

function cardCount(view: BoardView, lane: number, col: number): number {
  return view.lanes[lane]?.cells[col]?.length ?? 0;
}

export function hasCard(view: BoardView, p: Pos): boolean {
  return p.index < cardCount(view, p.lane, p.col);
}

// firstCard is the fallback the focus rule leans on: the first card of the
// first cell that holds one, reading lanes top to bottom and columns left to
// right. It is undefined only when the board holds no cards at all.
export function firstCard(view: BoardView): Pos | undefined {
  for (let lane = 0; lane < view.lanes.length; lane++) {
    for (let col = 0; col < view.columns.length; col++) {
      if (cardCount(view, lane, col) > 0) return { lane, col, index: 0 };
    }
  }
  return undefined;
}

// clampFocus guarantees exactly one focusable card survives every state
// change: it keeps the current one when it still exists, otherwise takes the
// selected card's place, otherwise the first card on the board. It returns
// "" only for a board with nothing to focus.
export function clampFocus(view: BoardView, id: string, selected?: Pos): string {
  const cur = parsePos(id);
  if (cur && hasCard(view, cur)) return id;
  if (selected && hasCard(view, selected)) return posId(selected);
  const first = firstCard(view);
  return first ? posId(first) : "";
}

// cardKeys is the board in reading order: lane by lane, column by column,
// card by card. It is the order a shift gesture measures a run against and
// the order a bulk action sends its keys in, so both agree with what the
// reader sees.
export function cardKeys(view: BoardView): string[] {
  const out: string[] = [];
  for (const lane of view.lanes) {
    for (const cell of lane.cells ?? []) {
      for (const card of cell) out.push(card.key);
    }
  }
  return out;
}

// findCard locates one issue key on the board, so a click or a selection
// can be turned back into a position.
export function findCard(view: BoardView, key: string): Pos | undefined {
  if (!key) return undefined;
  for (let lane = 0; lane < view.lanes.length; lane++) {
    const cells = view.lanes[lane].cells ?? [];
    for (let col = 0; col < cells.length; col++) {
      const index = cells[col].findIndex((c) => c.key === key);
      if (index >= 0) return { lane, col, index };
    }
  }
  return undefined;
}

// nextColumn steps sideways within one lane, skipping columns whose cell is
// empty: an empty cell has nothing to focus, so stopping in one would strand
// the keyboard. The row index is kept, clamped to what the new cell holds.
function nextColumn(view: BoardView, p: Pos, step: number): Pos | undefined {
  for (let col = p.col + step; col >= 0 && col < view.columns.length; col += step) {
    const n = cardCount(view, p.lane, col);
    if (n > 0) return { lane: p.lane, col, index: Math.min(p.index, n - 1) };
  }
  return undefined;
}

// nextLane steps down (or up) into the same column of another lane, again
// skipping lanes whose cell in that column is empty.
function nextLane(view: BoardView, p: Pos, step: number): Pos | undefined {
  for (let lane = p.lane + step; lane >= 0 && lane < view.lanes.length; lane += step) {
    const n = cardCount(view, lane, p.col);
    if (n > 0) return { lane, col: p.col, index: step > 0 ? 0 : n - 1 };
  }
  return undefined;
}

// moveFocus answers one arrow, Home, or End with the position it lands on,
// or undefined when the key moves nowhere. Left and Right cross columns and
// keep the row; Up and Down walk a cell and then continue into the next
// lane; Home and End take the first and last card of the cell.
export function moveFocus(view: BoardView, p: Pos, key: string): Pos | undefined {
  const n = cardCount(view, p.lane, p.col);
  switch (key) {
    case "ArrowRight":
      return nextColumn(view, p, 1);
    case "ArrowLeft":
      return nextColumn(view, p, -1);
    case "ArrowDown":
      return p.index + 1 < n ? { ...p, index: p.index + 1 } : nextLane(view, p, 1);
    case "ArrowUp":
      return p.index > 0 ? { ...p, index: p.index - 1 } : nextLane(view, p, -1);
    case "Home":
      return n > 0 ? { ...p, index: 0 } : undefined;
    case "End":
      return n > 0 ? { ...p, index: n - 1 } : undefined;
    default:
      return undefined;
  }
}
