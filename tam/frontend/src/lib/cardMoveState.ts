import { isMoveEntity } from "../api";
import type { CommitResult, PendingChange } from "../api";

// cardMoveState folds everything that is known about a card's move into
// the one thing the card can show. Until this existed a card that will
// never land looked exactly like one about to land: both wore the same
// amber pending dot, and the failure only existed in a dialog the user may
// never open.

// MoveState is what a card's position is doing, in the order a card
// reports them: a failure first, because it is the one that needs an
// answer, then a held issue, then a warning, then a check still in flight,
// then the ordinary pending move.
export type MoveState = "" | "pending" | "checking" | "warned" | "failed" | "conflicted";

// CardMove is one card's state and, for a failure, the reason to put in
// its label. The board is the surface the user made the move on, so it is
// the surface that has to say what became of it.
export interface CardMove {
  state: MoveState;
  reason: string;
}

// Warning is a CanTransition answer that came back no: where the card was
// going and what Jira offered instead.
export interface Warning {
  target: string;
  reachable: string[];
}

export interface MoveInputs {
  pending: PendingChange[];
  commit: CommitResult | null;
  warnings: Map<string, Warning>;
  checking: Set<string>;
}

// movedKeys is every issue key carrying a journaled board move.
export function movedKeys(pending: PendingChange[]): Set<string> {
  return new Set(pending.filter((r) => isMoveEntity(r.entityType)).map((r) => r.entityKey));
}

// cardMoves is one entry per key with a pending move, plus the failures a
// commit reported for a board row. A key with an ordinary pending edit and
// no move is not in the map: the plain pending dot already says that, and
// a moved card's claim is about its position.
export function cardMoves({ pending, commit, warnings, checking }: MoveInputs): Map<string, CardMove> {
  const moves = new Map<string, CardMove>();
  for (const key of movedKeys(pending)) moves.set(key, { state: "pending", reason: "" });
  for (const key of checking) {
    if (moves.has(key)) moves.set(key, { state: "checking", reason: "" });
  }
  for (const [key, warning] of warnings) {
    if (moves.has(key)) moves.set(key, { state: "warned", reason: warningLine(key, warning) });
  }
  for (const conflict of commit?.conflicts ?? []) {
    if (moves.has(conflict.key)) moves.set(conflict.key, { state: "conflicted", reason: "" });
  }
  for (const failure of commit?.failures ?? []) {
    // The journaled set is the guard every other branch here uses, and a
    // failure needs it most: the last commit's result outlives the journal
    // row it names, so an Undo that discards the row would otherwise leave
    // the card wearing the old reason and offering an Undo that has
    // nothing left to discard.
    if (!isMoveEntity(failure.entityType ?? "") || !moves.has(failure.key)) continue;
    moves.set(failure.key, { state: "failed", reason: failure.error });
  }
  return moves;
}

// warningLine is the sentence a refused check reads as, in the banner and
// in the card's own label. It names where the card cannot go and what Jira
// said it could, since "this move will fail" with no alternative leaves
// the user nowhere to go but the web board.
export function warningLine(key: string, warning: Warning): string {
  const head = `${key} cannot reach ${warning.target} from where it is now.`;
  if (warning.reachable.length === 0) {
    return `${head} Jira offers no other status from here.`;
  }
  return `${head} Jira offers ${warning.reachable.join(", ")}.`;
}
