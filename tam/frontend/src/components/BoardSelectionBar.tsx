import { useState } from "react";
import type { Sprint } from "../api";
import { plural } from "../lib/format";

// BACKLOG is the picker's value for the backlog, which is a destination and
// not an absence. It cannot collide with a sprint id, which is a number.
const BACKLOG = "backlog";

interface Props {
  // count is how many cards the board is drawing that are checked, which is
  // the same list the move sends.
  count: number;
  // sprints are the board's open sprints. The one on screen is not a
  // destination, so it is filtered out here rather than by the caller.
  sprints: Sprint[];
  sprintId: string;
  busy: boolean;
  // onMove takes null for the backlog, the same shape the card's own menu
  // and the detail panel's sprint field send.
  onMove: (sprint: Sprint | null) => void;
  onClear: () => void;
}

// BoardSelectionBar is what several checked cards can be done with: the
// count, one action, and the way out. It is XTM's bulk toolbar, class for
// class, down to `.bulk-count` on the number.
//
// One action, and it is worded as exactly what it does. A bulk transition
// needs every card's own workflow checked and a bulk edit needs a form, so
// neither is here; offering "Move N cards" beside nothing else is honest,
// where a menu of one greyed-out option would read as unfinished. Where they
// go is the picker's job rather than the button's, because the backlog is
// one of the answers.
export function BoardSelectionBar({ count, sprints, sprintId, busy, onMove, onClear }: Props) {
  const [target, setTarget] = useState("");
  const others = sprints.filter((s) => String(s.id) !== sprintId);
  const chosen = others.find((s) => String(s.id) === target);
  // The backlog is always a destination, so the bar always has one to offer:
  // a board with no other open sprint is not a board with nowhere to put
  // these cards, and both the card menu and the panel's sprint field say so.
  const ready = target === BACKLOG || !!chosen;

  return (
    <div className="board-selection-bar" role="group" aria-label="Selected cards">
      <span className="bulk-count">{plural(count, "card", "cards")} selected</span>
      <label className="board-picker">
        <span>Move to</span>
        <select
          aria-label="Move the selected cards to"
          className="board-select-narrow"
          value={target}
          onChange={(e) => setTarget(e.target.value)}
        >
          <option value="">Choose a destination</option>
          <option value={BACKLOG}>The backlog</option>
          {others.map((s) => (
            <option key={s.id} value={String(s.id)}>{s.name}</option>
          ))}
        </select>
      </label>
      <button
        type="button"
        className="btn btn-primary"
        disabled={busy || !ready}
        onClick={() => ready && onMove(chosen ?? null)}
      >
        {`Move ${plural(count, "card", "cards")}`}
      </button>
      <button type="button" className="btn board-selection-clear" onClick={onClear}>Clear</button>
    </div>
  );
}
