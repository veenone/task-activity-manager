import type { ColumnView } from "../api";

// columnLimit is how a board column's WIP limit reads, in one place because
// two surfaces read it: the column head on the board and the capacity
// section of the report, which lib/reportTables builds. A limit worded one
// way on screen and another on a published page is a figure the team cannot
// reconcile, which is the same rule lib/reportText exists for.
//
// A column with no limit is the ordinary case. Every function here answers
// for one, and none of them treats an absent limit as zero: zero is a limit
// a board can really set, and a column limited to nothing is over the moment
// it holds a card.

// A breach is what the limit says about the count: nothing, over the
// maximum, or under the minimum.
export type Breach = "" | "over" | "under";

// limitOf is the pair, normalised. A board cached before TAM read the limits
// carries neither, and undefined and null are the same fact here.
function limitOf(c: ColumnView): { min: number | null; max: number | null } {
  return { min: c.min ?? null, max: c.max ?? null };
}

// counted is the number a limit is measured against. Go sends it; the total
// is the fallback for a board cached before the count was split out, which
// is what a board counting every card would have sent anyway.
export function counted(c: ColumnView): number {
  return c.counted ?? c.total;
}

// hasLimit says whether Jira sets a limit on this column at all.
export function hasLimit(c: ColumnView): boolean {
  const { min, max } = limitOf(c);
  return min !== null || max !== null;
}

// excludesSubtasks is the board counting a column without its subtasks,
// which is one of the two things Jira's own constraint can mean.
function excludesSubtasks(c: ColumnView): boolean {
  return c.constraint === "issueCountExclSubs";
}

// limitBreach is the column against its limits. A column with no limit is
// never a breach, and a maximum takes precedence over a minimum on a board
// whose pair cannot both be satisfied.
export function limitBreach(c: ColumnView): Breach {
  const { min, max } = limitOf(c);
  const n = counted(c);
  if (max !== null && n > max) return "over";
  if (min !== null && n < min) return "under";
  return "";
}

// limitLine is the clause the column head prints under the column's name,
// empty for a column with no limit. A breach is named in words rather than
// left to a colour, because a reader who cannot see the colour is the reader
// this number matters most to, and the clause says when subtasks are left
// out of the count, since the head's own card total counts them.
export function limitLine(c: ColumnView): string {
  const { min, max } = limitOf(c);
  if (min === null && max === null) return "";
  const n = counted(c);
  const parts = [max === null ? `${n}, minimum ${min}` : `${n} of ${max}`];
  if (max !== null && min !== null) parts.push(`minimum ${min}`);
  if (excludesSubtasks(c)) parts.push("subtasks not counted");
  const breach = limitBreach(c);
  if (breach === "over") parts.push("over the limit");
  if (breach === "under") parts.push("below the minimum");
  return parts.join(", ");
}
