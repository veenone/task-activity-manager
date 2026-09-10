import { useState } from "react";
import type { Sprint } from "../api";
import { plural } from "../lib/format";

// BACKLOG is the picker's value for the board's own backlog, which is a
// destination and not an absence. It cannot collide with a sprint id, which
// is a number.
const BACKLOG = "backlog";

interface Props {
  // count is how many cards the tree is drawing that are checked, which is
  // exactly the list the fill sends.
  count: number;
  // sprints are this board's open sprints. Every one of them is offered,
  // including the sprint a checked card is already in: a selection here can
  // span several sprints, so there is no one sprint to leave out the way
  // the board's own bar leaves out the sprint on screen.
  sprints: Sprint[];
  busy: boolean;
  // onFill takes the destination's sprint id, "" for the backlog, which is
  // the same shape JournalSprintMoves takes.
  onFill: (sprintId: string) => void;
  onClear: () => void;
}

// SprintFillBar is how a sprint gets filled: check the work, choose where it
// goes, and send it. Like every other bulk move on a board it is journaled,
// through the same MoveManyToSprint one drag journals through, so nothing
// reaches Jira until Commit and a filled sprint can be emptied again by
// discarding the pending rows.
//
// It is the board's own selection bar, class for class, because it is the
// same action: the two screens differ in what a selection can be made over,
// not in what happens to it.
export function SprintFillBar({ count, sprints, busy, onFill, onClear }: Props) {
  const [target, setTarget] = useState("");
  const chosen = sprints.find((s) => String(s.id) === target);
  // The backlog is always a destination, so the bar always has one to offer:
  // a board with no open sprint is not a board with nowhere to put these
  // cards.
  const ready = target === BACKLOG || !!chosen;

  return (
    <div className="board-selection-bar" role="group" aria-label="Checked cards">
      <span className="bulk-count">{plural(count, "card", "cards")} selected</span>
      <label className="board-picker">
        <span>Move to</span>
        <select
          aria-label="Move the checked cards to"
          className="board-select-narrow"
          value={target}
          onChange={(e) => setTarget(e.target.value)}
        >
          <option value="">Choose a destination</option>
          <option value={BACKLOG}>The backlog</option>
          {sprints.map((s) => (
            <option key={s.id} value={String(s.id)}>{s.name}</option>
          ))}
        </select>
      </label>
      <button
        type="button"
        className="btn btn-primary"
        disabled={busy || !ready}
        // This is not what stops a second press: a disabled button
        // dispatches no click at all, so the line never runs while a fill
        // is in flight. It guards the handler rather than the button, and
        // the view's own onFill guards the mutation again before it sends,
        // which is the check a second press actually meets.
        onClick={() => !busy && ready && onFill(chosen ? String(chosen.id) : "")}
      >
        {`Move ${plural(count, "card", "cards")}`}
      </button>
      <button type="button" className="btn board-selection-clear" onClick={onClear}>Clear</button>
    </div>
  );
}
