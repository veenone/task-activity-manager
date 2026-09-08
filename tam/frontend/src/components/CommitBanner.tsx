import { isMoveEntity } from "../api";
import type { CommitFailure, CommitResult } from "../api";
import { plural } from "../lib/format";

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
  if (pushed.length) parts.push(plural(pushed.length, "card moved", "cards moved"));
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
  busy: boolean;
  onUndo: (rowId: number, key: string) => void;
}

// CommitBanner is what the last Commit did, and what is left to do about
// it: the issues it held back, the pushes that failed, an Undo on each
// board move that will never land, and the retry line only where a retry
// could help.
export function CommitBanner({ result, heldKeys, busy, onUndo }: Props) {
  const warn = result.conflicts.length > 0 || result.failures.length > 0;
  return (
    <div className={`pending-banner${warn ? " pending-banner-warn" : ""}`} role="status">
      <p className="b">{bannerLine(result)}</p>
      {result.conflicts.filter((c) => heldKeys.has(c.key)).map((c) => (
        <p key={c.key} className="small">{c.key} changed in Jira since you edited it. Resolve it below, then commit again.</p>
      ))}
      {result.failures.map((f) => (
        <p key={`${f.key}-${f.entityType ?? ""}-${f.rowId ?? 0}`} className="small error-text">
          {f.key}: {f.error}{" "}
          {undoable(f) && (
            <button type="button" className="btn" disabled={busy} onClick={() => onUndo(f.rowId!, f.key)}>
              Undo this move
            </button>
          )}
        </p>
      ))}
      {retryWorthOffering(result.failures) && (
        <p className="muted small">Commit again to retry the failures.</p>
      )}
    </div>
  );
}
