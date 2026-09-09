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
export function useBoardSelection(order: string[]) {
  const [selection, setSelection] = useState<Selection>(NONE);
  const keys = useMemo(() => checkedIn(selection, order), [selection, order]);
  const checked = useMemo(() => new Set(keys), [keys]);

  return {
    // keys are the checked cards in the board's order, which is exactly what
    // a bulk action sends; checked is the same list as a set, for the grid.
    keys,
    checked,
    count: keys.length,
    // reset empties it and forgets the anchor: a change of board, sprint,
    // swimlane or profile is a different board, and a selection made on the
    // last one means nothing on this one.
    reset: () => setSelection(NONE),
    // clearTo is the plain gesture: nothing checked, and the card just
    // touched left as the anchor a shift gesture will measure from.
    clearTo: (key: string) => setSelection(clear(key)),
    check: (key: string) => setSelection((s) => toggle(s, key)),
    extendTo: (key: string) => setSelection((s) => extend(s, order, key)),
  };
}

export type BoardSelection = ReturnType<typeof useBoardSelection>;
