import type { ColumnView, Issue } from "../api";

// "Not finished" has one definition in TAM, and this is it: a card whose
// status the board's last column does not collect. The board draws by that
// same mapping, internal/sprints completes by it, and the completion dialog
// says so in words, so a second rule anywhere would put two answers about
// the same card on one screen.
//
// It takes cards and columns rather than a drawn board, because the two
// callers hold different shapes. The Boards view has cells to flatten, and
// a cell already knows its column; the Sprints view has one sprint's issue
// list and no cell anywhere, which is why the rule is written against the
// status id every card carries rather than against where it was drawn.
//
// A board with fewer than two columns has no last column to judge against
// and answers with nothing rather than with everything: on a board whose
// only column is also its last, every card would otherwise be finished, and
// on a board with none, every card unfinished.
export function unfinished(issues: Issue[], columns: ColumnView[]): Issue[] {
  const last = columns.length - 1;
  if (last < 1) return [];
  const finished = new Set(columns[last].statusIds ?? []);
  // A draft carries no status id, since Jira has never given it one, so it
  // is unfinished here the same way the board draws it in the first column.
  return issues.filter((i) => !finished.has(i.statusId ?? ""));
}
