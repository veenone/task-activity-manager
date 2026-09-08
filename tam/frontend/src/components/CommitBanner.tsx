import { isMoveEntity } from "../api";
import type { CommitFailure, CommitMove, CommitResult } from "../api";
import { plural } from "../lib/format";

// movedPhrase names one board write the way a created issue names its new
// key: a transition and a sprint move by where the card went, a rank by the
// card it was placed against and which side of it. Side is empty for
// everything but a rank, which is what tells the two apart.
export function movedPhrase(m: CommitMove): string {
  return m.side ? `${m.key} ${m.side} ${m.target}` : `${m.key} to ${m.target}`;
}

// bannerLine renders a commit result as one sentence.
export function bannerLine(r: CommitResult): string {
  const parts: string[] = [];
  if (r.committed.length) parts.push(plural(r.committed.length, "issue pushed", "issues pushed"));
  if (r.created.length) {
    const mapping = r.created.map((c) => `${c.tempKey} is now ${c.key}`).join(", ");
    parts.push(`${r.created.length} created (${mapping})`);
  }
  if (r.linked.length) parts.push(plural(r.linked.length, "link pushed", "links pushed"));
  const moved = r.moved ?? [];
  // A move Jira had already made is not a card this Commit moved, and
  // counting it as one credits the push with work it did not do.
  const pushed = moved.filter((m) => !m.satisfied);
  const already = moved.length - pushed.length;
  if (pushed.length) {
    parts.push(`${plural(pushed.length, "card moved", "cards moved")} (${pushed.map(movedPhrase).join(", ")})`);
  }
  if (already) parts.push(`${already} already in place`);
  if (r.conflicts.length) parts.push(`${r.conflicts.length} held back`);
  if (r.failures.length) parts.push(`${r.failures.length} failed`);
  if (parts.length === 0) return "Last commit: nothing to push.";
  if (!r.committed.length && !r.created.length && !r.linked.length && !pushed.length) {
    return `Last commit: nothing pushed, ${parts.join(", ")}.`;
  }
  return `Last commit: ${parts.join(", ")}.`;
}

// undoable is a board failure the user can take back: it names the journal
// row it was, so an Undo discards exactly that move and leaves the issue's
// other pending rows alone.
export function undoable(f: CommitFailure): boolean {
  return isMoveEntity(f.entityType ?? "") && !!f.rowId;
}

// stillPending drops a board failure whose journal row has gone. The last
// commit's result survives an Undo, so without this the banner kept the
// line and its button, and the button then failed with "pending change not
// found". A failure that names no row cannot be undone and is left alone.
export function stillPending(f: CommitFailure, rowIds: Set<number>): boolean {
  return !isMoveEntity(f.entityType ?? "") || !f.rowId || rowIds.has(f.rowId);
}

// retryWorthOffering is false when every failure is one that will fail
// identically forever, which a transition with no path is. "Commit again to
// retry" is the one thing that cannot help there, and offering it is a
// promise the next Commit cannot keep.
export function retryWorthOffering(failures: CommitFailure[]): boolean {
  return failures.some((f) => f.retryable !== false);
}

interface Props {
  result: CommitResult;
  // heldKeys are the conflicts this dialog is still showing a card for.
  heldKeys: Set<string>;
  // pendingRowIds are the journal rows that still exist, which is what
  // decides whether a failed move still has anything to say or to undo.
  pendingRowIds: Set<number>;
  busy: boolean;
  onUndo: (rowId: number, key: string) => void;
}

// CommitBanner is what the last Commit did, and what is left to do about
// it: the issues it held back, the pushes that failed, an Undo on each
// board move that will never land, and the retry line only where a retry
// could help.
export function CommitBanner({ result, heldKeys, pendingRowIds, busy, onUndo }: Props) {
  // The sentence above stays the record of what this Commit did, failures
  // included; the lines below it are the work still outstanding, so a move
  // the user has since taken back drops out of them.
  const failures = result.failures.filter((f) => stillPending(f, pendingRowIds));
  const warn = result.conflicts.length > 0 || failures.length > 0;
  return (
    <div className={`pending-banner${warn ? " pending-banner-warn" : ""}`} role="status">
      <p className="b">{bannerLine(result)}</p>
      {result.conflicts.filter((c) => heldKeys.has(c.key)).map((c) => (
        <p key={c.key} className="small">{c.key} changed in Jira since you edited it. Resolve it below, then commit again.</p>
      ))}
      {failures.map((f) => (
        <p key={`${f.key}-${f.entityType ?? ""}-${f.rowId ?? 0}`} className="small error-text">
          {f.key}: {f.error}{" "}
          {undoable(f) && (
            <button type="button" className="btn" disabled={busy} onClick={() => onUndo(f.rowId!, f.key)}>
              Undo this move
            </button>
          )}
        </p>
      ))}
      {retryWorthOffering(failures) && (
        <p className="muted small">Commit again to retry the failures.</p>
      )}
    </div>
  );
}
