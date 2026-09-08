import type { BoardView, Issue } from "../api";

// matchesCard is the board filter's one rule: a card matches when the text
// appears in its key, its assignee, or its issue type. Those three are what
// a standup asks for out loud ("where's mine", "where are the bugs",
// "what's -124 doing"), and all three are on the card already, so a filter
// over them needs nothing the view has not got.
export function matchesCard(issue: Issue, needle: string): boolean {
  const q = needle.trim().toLowerCase();
  if (q === "") return true;
  return (
    issue.key.toLowerCase().includes(q) ||
    issue.assignee.toLowerCase().includes(q) ||
    issue.type.toLowerCase().includes(q)
  );
}

// filterBoard hides the cards that do not match, keeping the board's shape:
// every column and every lane stays, so a filter never makes a column look
// as though the board lost it, and an empty lane still says which lane it is.
//
// The counts are recomputed from what survives, because a column head
// reading "12 cards" over one card is worse than no count at all. Overflow
// is dropped for the same reason: the cards a cap left out were never
// filtered, so claiming a number for them would be a guess. The board-level
// totals (unmapped, notSynced, donePoints) are left alone: they describe the
// board, not this view of it.
export function filterBoard(view: BoardView, needle: string): BoardView {
  if (needle.trim() === "") return view;
  const totals = view.columns.map(() => ({ total: 0, points: 0 }));
  const lanes = view.lanes.map((lane) => {
    const cells = lane.cells.map((cell, col) => {
      const kept = cell.filter((iss) => matchesCard(iss, needle));
      totals[col].total += kept.length;
      totals[col].points += kept.reduce((sum, i) => sum + (i.storyPoints ?? 0), 0);
      return kept;
    });
    return {
      ...lane,
      cells,
      overflow: lane.overflow.map(() => 0),
      count: cells.reduce((n, c) => n + c.length, 0),
    };
  });
  return {
    ...view,
    columns: view.columns.map((c, i) => ({ ...c, total: totals[i].total, points: totals[i].points })),
    lanes,
  };
}
