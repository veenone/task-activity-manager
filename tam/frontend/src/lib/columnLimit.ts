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
//
// A limit comes from one of two places, and which one is part of what these
// functions say. Jira's wins wherever the board sets one, because that is the
// rule the whole team already sees in Jira; the local one fills the gap on the
// boards, most of them, whose admin never set a limit at all. TAM writes
// nothing back to board configuration, so the two are never the same number,
// and a team reading "7 of 5" has to know whether that 5 is the board's rule
// or one they set themselves.

// A breach is what the limit says about the count: nothing, over the
// maximum, or under the minimum.
export type Breach = "" | "over" | "under";

// Where a column's limit came from, empty for a column with none.
export type LimitSource = "" | "jira" | "tam";

// jiraLimit is the pair the board sets, normalised. A board cached before TAM
// read the limits carries neither, and undefined and null are the same fact.
function jiraLimit(c: ColumnView): { min: number | null; max: number | null } {
  return { min: c.min ?? null, max: c.max ?? null };
}

// limitSource is which of the two places this column's limit came from. Jira's
// pair is checked first because it wins: a board that sets a limit has said
// what the limit is, and a local number cannot overrule it.
export function limitSource(c: ColumnView): LimitSource {
  const { min, max } = jiraLimit(c);
  if (min !== null || max !== null) return "jira";
  return (c.localMax ?? null) !== null ? "tam" : "";
}

// limitSourceWords names the source in the words every surface uses, so the
// board head and the published capacity table attribute a number the same way.
export function limitSourceWords(c: ColumnView): string {
  switch (limitSource(c)) {
    case "jira":
      return "from Jira";
    case "tam":
      return "set in TAM";
    default:
      return "";
  }
}

// effectiveLimit is the pair this column is actually measured against: the
// board's when it sets one, the local maximum otherwise. It is exported
// because lib/reportTables words the same pair into a cell of the capacity
// table, and the precedence has to be decided once.
export function effectiveLimit(c: ColumnView): { min: number | null; max: number | null } {
  if (limitSource(c) === "tam") return { min: null, max: c.localMax ?? null };
  return jiraLimit(c);
}

// counted is the number a limit is measured against. Go sends it; the total
// is the fallback for a board cached before the count was split out, which
// is what a board counting every card would have sent anyway.
export function counted(c: ColumnView): number {
  return c.counted ?? c.total;
}

// hasLimit says whether this column has a limit at all, from either place.
export function hasLimit(c: ColumnView): boolean {
  return limitSource(c) !== "";
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
  const { min, max } = effectiveLimit(c);
  const n = counted(c);
  if (max !== null && n > max) return "over";
  if (min !== null && n < min) return "under";
  return "";
}

// limitMeter is the same facts drawn: the count as a fill, and the limit as
// a mark on the track it fills. A column with no limit has nothing to draw
// and answers null.
//
// The scale is the larger of the count and the limit, which is what puts the
// mark at the end of the track while the column is within its limit and
// moves it inside the fill once the column is past it. That is what makes
// 4 of 3 read as a bar running on beyond its mark rather than as one that is
// merely full, and it is the only thing about the drawing that says a column
// is over: the colour changes too, but a reader who cannot see the colour
// reads the breach off the shape.
//
// The mark is the ceiling wherever the column has one and the floor
// otherwise, which is the figure limitLine leads with in each case.
export function limitMeter(c: ColumnView): { count: number; scale: number; mark: number } | null {
  const { min, max } = effectiveLimit(c);
  const target = max ?? min;
  if (target === null) return null;
  const count = counted(c);
  const scale = Math.max(count, target);
  // A column limited to nothing and holding nothing has no scale to divide
  // by. Its mark is the end of an empty track, which is where a limit the
  // count has not passed belongs.
  return { count, scale, mark: scale > 0 ? target / scale : 1 };
}

// limitLine is the clause the column head prints under the column's name,
// empty for a column with no limit. A breach is named in words rather than
// left to a colour, because a reader who cannot see the colour is the reader
// this number matters most to, and the clause says when subtasks are left
// out of the count, since the head's own card total counts them.
//
// The source rides next to the figure rather than at the end of the clause, so
// it attaches to the number it qualifies instead of to the breach.
export function limitLine(c: ColumnView): string {
  const { min, max } = effectiveLimit(c);
  if (min === null && max === null) return "";
  const n = counted(c);
  const parts = [max === null ? `${n}, minimum ${min}` : `${n} of ${max}`];
  if (max !== null && min !== null) parts.push(`minimum ${min}`);
  parts.push(limitSourceWords(c));
  if (excludesSubtasks(c)) parts.push("subtasks not counted");
  const breach = limitBreach(c);
  if (breach === "over") parts.push("over the limit");
  if (breach === "under") parts.push("below the minimum");
  return parts.join(", ");
}
