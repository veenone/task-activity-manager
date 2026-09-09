// The board's multi-selection: the cards a bulk action will touch, and the
// anchor a shift gesture measures from.
//
// It is a set of issue keys, never a set of positions. The board redraws on
// every refetch, and a sync that moves one card would leave a position-based
// selection pointing at whichever cards happen to sit in those slots
// afterwards, which is the same class of bug the focus model hit in 3b.
//
// This module has no React and no DOM: the three gestures are arithmetic
// over a set and an ordered key list, which is the part worth testing on its
// own. Which gesture a click or a key press is, and what the board does with
// the answer, belongs to BoardsView.

export interface Selection {
  // keys are the checked cards. A card the board no longer draws stays in
  // here until the next gesture; checkedIn is what filters it out, so a
  // refetch can never widen what an action touches.
  keys: ReadonlySet<string>;
  // anchor is the card a shift gesture extends from: the last card a plain
  // click, a control click, or a Space landed on. Empty when there is none,
  // in which case a shift gesture is an ordinary one.
  anchor: string;
}

export const NONE: Selection = { keys: new Set<string>(), anchor: "" };

// clear empties the selection, keeping the card given as the next anchor so
// a shift gesture straight after a plain click extends from where the user
// just clicked.
export function clear(anchor = ""): Selection {
  return { keys: new Set<string>(), anchor };
}

export function isChecked(sel: Selection, key: string): boolean {
  return sel.keys.has(key);
}

// replace is the plain gesture: the selection becomes exactly this card. It
// is what the other two fall back to when the anchor has left the board.
export function replace(key: string): Selection {
  return { keys: new Set([key]), anchor: key };
}

// toggle checks or unchecks one card and leaves the rest alone, which is
// what a control click and Space both do. The card becomes the anchor
// either way: the next shift gesture measures from the card the user last
// touched, whether they were adding it or taking it out.
export function toggle(sel: Selection, key: string): Selection {
  const keys = new Set(sel.keys);
  if (!keys.delete(key)) keys.add(key);
  return { keys, anchor: key };
}

// extend selects the run between the anchor and this card, in the board's
// own reading order. The anchor does not move, so a shift gesture that
// crosses it flips the run to the other side rather than growing forever,
// and the cards checked by hand outside the run are dropped, which is what
// every list with a shift gesture does.
export function extend(sel: Selection, order: readonly string[], key: string): Selection {
  const from = order.indexOf(sel.anchor);
  const to = order.indexOf(key);
  if (from < 0 || to < 0) return replace(key);
  const [lo, hi] = from <= to ? [from, to] : [to, from];
  return { keys: new Set(order.slice(lo, hi + 1)), anchor: sel.anchor };
}

// checkedIn is the selection as an action sees it: the checked cards the
// board is actually drawing, in the board's own order. A card that has been
// filtered out, moved off the board, or dropped by a sync is not one this
// selection may act on, and reading through here is what guarantees the
// count on screen and the keys sent to Go are the same list.
export function checkedIn(sel: Selection, order: readonly string[]): string[] {
  return order.filter((key) => sel.keys.has(key));
}
