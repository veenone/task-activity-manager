import { useMemo, useState } from "react";
import { NONE, checkedIn, clear, extend, toggle } from "../lib/boardSelection";
import type { Selection } from "../lib/boardSelection";

// useBoardSelection is the board's multi-selection as React holds it: the
// pure gestures in lib/boardSelection over one piece of state, plus the two
// readings the board needs of them.
//
// order is the board in reading order, which the caller memoises off the
// drawn view. Everything the selection is read through goes past it, so a
// card the board is no longer drawing can never be counted, painted, or
// moved: a refetch narrows the selection instead of silently widening what
// the next action touches.
//
// madeFor is who the selection belongs to, the active profile. It is reset
// here rather than by the caller because the caller's own render-time reset
// runs before this hook exists, and an effect would be one render too late:
// a bulk move fired in between would carry the last profile's keys. Two
// profiles over the same project key draw the same issue keys, so a
// selection carried across a switch would look entirely plausible.
export function useBoardSelection(order: string[], madeFor: string) {
  const [selection, setSelection] = useState<Selection>(NONE);
  const [belongsTo, setBelongsTo] = useState(madeFor);
  if (belongsTo !== madeFor) {
    setBelongsTo(madeFor);
    setSelection(NONE);
  }
  const keys = useMemo(() => checkedIn(selection, order), [selection, order]);
  const checked = useMemo(() => new Set(keys), [keys]);

  return {
    // keys are the checked cards in the board's order, which is exactly what
    // a bulk action sends; checked is the same list as a set, for the grid.
    keys,
    checked,
    count: keys.length,
    // reset empties it and forgets the anchor: a change of board, sprint or
    // swimlane is a different board, and a selection made on the last one
    // means nothing on this one. A change of profile does the same, from the
    // check above rather than from a caller.
    reset: () => setSelection(NONE),
    // clearTo is the plain gesture: nothing checked, and the card just
    // touched left as the anchor a shift gesture will measure from.
    clearTo: (key: string) => setSelection(clear(key)),
    check: (key: string) => setSelection((s) => toggle(s, key)),
    extendTo: (key: string) => setSelection((s) => extend(s, order, key)),
  };
}

export type BoardSelection = ReturnType<typeof useBoardSelection>;
