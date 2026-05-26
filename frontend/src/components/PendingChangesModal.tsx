import type { PendingChange, CommitResult } from "../api";

interface Props {
  changes: PendingChange[];
  onDiscard: (id: number) => Promise<void> | void;
  onCommit: () => Promise<void> | void;
  onJumpTo: (testKey: string) => void;
  onClose: () => void;
  committing: boolean;
  lastResult: CommitResult | null;
}

export function PendingChangesModal({
  changes,
  onDiscard,
  onCommit,
  onJumpTo,
  onClose,
  committing,
  lastResult,
}: Props) {
  const hasResult =
    lastResult &&
    (lastResult.succeeded.length > 0 || lastResult.failed.length > 0);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal pending-modal"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="pending-head">
          <h2>Pending changes ({changes.length})</h2>
          <button className="btn btn-ghost" onClick={onClose} title="Close">
            ✕
          </button>
        </div>

        {hasResult && (
          <div className="commit-result">
            {lastResult!.succeeded.length > 0 && (
              <p className="ok-text">
                ✓ Committed {lastResult!.succeeded.length}{" "}
                {lastResult!.succeeded.length === 1 ? "test" : "tests"} to Jira.
              </p>
            )}
            {lastResult!.failed.length > 0 && (
              <div className="error-text">
                <p>
                  Failed ({lastResult!.failed.length}) — these changes remain
                  in the pending list:
                </p>
                <ul className="commit-fail-list">
                  {lastResult!.failed.map((f, i) => (
                    <li key={i}>
                      {f.testKey && (
                        <span className="mono">{f.testKey}: </span>
                      )}
                      {f.error}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        {changes.length === 0 ? (
          <p className="muted pending-empty">No pending changes.</p>
        ) : (
          <div className="pending-table-wrap">
            <table className="pending-table">
              <thead>
                <tr>
                  <th>Test</th>
                  <th>Field</th>
                  <th>Before</th>
                  <th>After</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {changes.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <button
                        className="link-btn mono"
                        onClick={() => onJumpTo(c.entityKey)}
                        title="Open this test"
                      >
                        {c.entityKey}
                      </button>
                    </td>
                    <td>{c.field}</td>
                    <td className="pending-before">
                      {truncate(c.beforeVal, 100)}
                    </td>
                    <td className="pending-after">
                      {truncate(c.afterVal, 100)}
                    </td>
                    <td>
                      <button
                        className="btn"
                        onClick={() => onDiscard(c.id)}
                        disabled={committing}
                      >
                        Discard
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <div className="pending-actions">
          <p className="muted pending-footnote-inline">
            Successful commits leave this list; failures stay and can be
            retried or discarded.
          </p>
          <button
            className="btn btn-primary"
            onClick={onCommit}
            disabled={committing || changes.length === 0}
          >
            {committing
              ? "Committing…"
              : changes.length === 1
                ? "Commit 1 change"
                : `Commit ${changes.length} changes`}
          </button>
        </div>
      </div>
    </div>
  );
}

function truncate(s: string, n: number): string {
  if (!s) return "";
  if (s.length <= n) return s;
  return s.slice(0, n) + "…";
}
