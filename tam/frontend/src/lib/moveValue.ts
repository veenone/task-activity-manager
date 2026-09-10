import { ENTITY_RANK, ENTITY_SPRINT_MOVE, ENTITY_TRANSITION, MOVE_LABELS } from "../api";
import type { PendingChange } from "../api";

// moveValue reads the values a board move journals. issuerepo packs a
// transition and a sprint move as "id|Name" and a rank as
// "side|neighbour|board", and this is the frontend's half of that
// encoding: its own module because the Pending changes dialog and the
// Activity tab both read it, and neither of them knows anything about a
// board.

const SEP = "|";

// moveName is the name half of an "id|Name" value, falling back to the id
// when the cache never held a name for it, so a dialog prints something a
// person can look up rather than an empty cell.
export function moveName(value: string): string {
  const at = value.indexOf(SEP);
  if (at < 0) return value;
  const name = value.slice(at + 1);
  return name || value.slice(0, at);
}

// moveId is the id half of an "id|Name" value, the mirror of issuerepo's
// MoveID and here for the reason Go's exists: a name can differ between the
// cached row and the board configuration, so the id is the only half that
// says whether two values name the same place. moveName is the half to read
// when the value is being printed rather than compared.
export function moveId(value: string): string {
  const at = value.indexOf(SEP);
  return at < 0 ? value : value.slice(0, at);
}

// journalTouchesSprint says whether the journal holds a sprint move with this
// sprint at either end of it: a card on its way in, or a card on its way out.
// Either one puts the sprint's cached membership out of step with Jira's,
// because the pending move is replayed over the cache and Jira has not seen
// it yet, so a surface quoting a sprint's size has to say so.
export function journalTouchesSprint(rows: readonly PendingChange[], sprintId: number): boolean {
  const id = String(sprintId);
  return rows.some(
    (r) => r.entityType === ENTITY_SPRINT_MOVE && (moveId(r.beforeVal) === id || moveId(r.afterVal) === id),
  );
}

// Rank is a rank row's after value read back: the card it was dropped
// against and which side of it.
export interface Rank {
  neighbourKey: string;
  before: boolean;
}

export function parseRank(value: string): Rank {
  const [side = "", neighbourKey = ""] = value.split(SEP);
  return { neighbourKey, before: side === "before" };
}

// MoveWords is one journaled move in words: what kind of move it was, and
// where the card came from and went. from is empty for a rank, which has
// no cached value to have come from.
export interface MoveWords {
  label: string;
  from: string;
  to: string;
}

// BACKLOG is what an empty sprint id reads as. The backlog is a
// destination, not an absence, and "(none)" would say the opposite.
const BACKLOG = "Backlog";

// moveWords turns one journal row, or one audit entry, into the three
// pieces both dialogs print: "Status: To Do to In Progress",
// "Sprint: Sprint 12 to Sprint 13", "Rank: before PLAT-409".
export function moveWords(entityType: string, beforeVal: string, afterVal: string): MoveWords {
  switch (entityType) {
    case ENTITY_TRANSITION:
      return { label: MOVE_LABELS[entityType], from: moveName(beforeVal), to: moveName(afterVal) };
    case ENTITY_SPRINT_MOVE:
      return {
        label: MOVE_LABELS[entityType],
        from: beforeVal ? moveName(beforeVal) : BACKLOG,
        to: afterVal ? moveName(afterVal) : BACKLOG,
      };
    case ENTITY_RANK: {
      const rank = parseRank(afterVal);
      return {
        label: MOVE_LABELS[entityType],
        from: "",
        to: `${rank.before ? "before" : "after"} ${rank.neighbourKey}`,
      };
    }
    default:
      return { label: entityType, from: beforeVal, to: afterVal };
  }
}
