import type { PendingChange } from "../api";
import { moveWords } from "../lib/moveValue";

interface Props {
  row: PendingChange;
  disabled: boolean;
  onDiscard: () => void;
}

// PendingMoveRow is one journaled board move in the Pending changes
// dialog. Without it a move rendered through the field label and read
// "statusId: 3 to 5", because the dialog knows fields and this row is a
// place. A rank has no value it came from, so its row is its label and
// where it went.
//
// The spans are separated the way the link and edit rows are: without the
// spaces a screen reader runs them together as one word, and the row reads
// "StatusTo DotoIn Progress".
export function PendingMoveRow({ row, disabled, onDiscard }: Props) {
  const { label, from, to } = moveWords(row.entityType, row.beforeVal, row.afterVal);
  return (
    <li className="pending-row pending-row-move">
      <span className="muted">{label}</span>{" "}
      <span>{from}</span>{" "}
      <span className="muted">{from ? "to" : ""}</span>{" "}
      <span className="b">{to}</span>{" "}
      <button
        type="button"
        className="btn btn-ghost"
        disabled={disabled}
        aria-label={`Discard the ${label.toLowerCase()} move on ${row.entityKey}`}
        onClick={onDiscard}
      >
        Discard
      </button>
    </li>
  );
}
