import { ENTITY_RANK, ENTITY_SPRINT_MOVE, ENTITY_TRANSITION } from "../api";

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
      return { label: "Status", from: moveName(beforeVal), to: moveName(afterVal) };
    case ENTITY_SPRINT_MOVE:
      return {
        label: "Sprint",
        from: beforeVal ? moveName(beforeVal) : BACKLOG,
        to: afterVal ? moveName(afterVal) : BACKLOG,
      };
    case ENTITY_RANK: {
      const rank = parseRank(afterVal);
      return { label: "Rank", from: "", to: `${rank.before ? "before" : "after"} ${rank.neighbourKey}` };
    }
    default:
      return { label: entityType, from: beforeVal, to: afterVal };
  }
}
