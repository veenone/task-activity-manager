import { useState } from "react";
import type { Sprint } from "../api";
import { plural } from "../lib/format";

interface Props {
  // count is how many cards the board is drawing that are checked, which is
  // the same list the move sends.
  count: number;
  // sprints are the board's open sprints. The one on screen is not a
  // destination, so it is filtered out here rather than by the caller.
  sprints: Sprint[];
  sprintId: string;
  busy: boolean;
  onMove: (sprint: Sprint) => void;
  onClear: () => void;
}

// BoardSelectionBar is what several checked cards can be done with: the
// count, one action, and the way out. It is XTM's bulk toolbar, class for
// class, down to `.bulk-count` on the number.
//
// One action, and it is worded as exactly what it does. A bulk transition
// needs every card's own workflow checked and a bulk edit needs a form, so
// neither is here; offering "Move N cards to sprint" beside nothing else is
// honest, where a menu of one greyed-out option would read as unfinished.
export function BoardSelectionBar({ count, sprints, sprintId, busy, onMove, onClear }: Props) {
  const [target, setTarget] = useState("");
  const others = sprints.filter((s) => String(s.id) !== sprintId);
  const chosen = others.find((s) => String(s.id) === target);

  return (
    <div className="board-selection-bar" role="group" aria-label="Selected cards">
      <span className="bulk-count">{plural(count, "card", "cards")} selected</span>
      {others.length === 0 ? (
        <span className="muted small">This board has no other open sprint to move them to.</span>
      ) : (
        <>
          <label className="board-picker">
            <span>Move to</span>
            <select
              aria-label="Move the selected cards to"
              className="board-select-narrow"
              value={target}
              onChange={(e) => setTarget(e.target.value)}
            >
              <option value="">Choose a sprint</option>
              {others.map((s) => (
                <option key={s.id} value={String(s.id)}>{s.name}</option>
              ))}
            </select>
          </label>
          <button
            type="button"
            className="btn btn-primary"
            disabled={busy || !chosen}
            onClick={() => chosen && onMove(chosen)}
          >
            {`Move ${plural(count, "card", "cards")} to sprint`}
          </button>
        </>
      )}
      <button type="button" className="btn board-selection-clear" onClick={onClear}>Clear</button>
    </div>
  );
}
