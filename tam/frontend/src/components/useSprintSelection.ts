import { useMemo, useState } from "react";
import { NONE, checkedIn, clear, extend, toggle } from "../lib/boardSelection";
import type { Selection } from "../lib/boardSelection";

// useSprintSelection is the Sprints view's multi-selection: the same pure
// gestures lib/boardSelection gives the board, over one piece of state, with
// the one difference this screen forces.
//
// The board draws one sprint at a time, so its reading order is one list.
// This tree draws every sprint at once, so it is a list per sprint, and both
// of the lists' uses turn on that. checkedIn reads across all of them, since
// a fill can carry cards out of several sprints at once and the count on
// screen has to be the list that is sent. A shift gesture reads one of them,
// the scope it was made in, so a drag from the top of one sprint to the
// bottom of another checks that second sprint's run and not three sprints'
// work together: extend cannot find the anchor in another scope's list and
// falls back to the plain gesture, which is the clamp, not an accident.
//
// Every list holds issue keys and nothing else. A sprint's own row id would
// be checked and then sent to Go as an issue key if it ever got in, because
// extend slices the list it is given without looking at what is in it.
//
// madeFor is the active profile. It is reset here rather than by the caller
// for the same reason useBoardSelection resets there: the caller's own
// render-time reset runs before this hook exists, an effect would be one
// render too late, and two profiles over one project draw the same keys, so
// a selection carried across a switch would look entirely plausible.
export function useSprintSelection(orderByScope: Map<string, string[]>, madeFor: string) {
  const [selection, setSelection] = useState<Selection>(NONE);
  const [belongsTo, setBelongsTo] = useState(madeFor);
  if (belongsTo !== madeFor) {
    setBelongsTo(madeFor);
    setSelection(NONE);
  }
  const order = useMemo(() => [...orderByScope.values()].flat(), [orderByScope]);
  const keys = useMemo(() => checkedIn(selection, order), [selection, order]);
  const checked = useMemo(() => new Set(keys), [keys]);

  return {
    // keys are the checked cards in the tree's own order, which is what a
    // fill sends; checked is the same list as a set, for the rows.
    keys,
    checked,
    count: keys.length,
    // reset empties it and forgets the anchor: a change of board or profile
    // is a different list of sprints, and a selection made on the last one
    // means nothing on this one.
    reset: () => setSelection(NONE),
    clearTo: (key: string) => setSelection(clear(key)),
    check: (key: string) => setSelection((s) => toggle(s, key)),
    extendTo: (key: string, scope: string) =>
      setSelection((s) => extend(s, orderByScope.get(scope) ?? [], key)),
  };
}

export type SprintSelection = ReturnType<typeof useSprintSelection>;
