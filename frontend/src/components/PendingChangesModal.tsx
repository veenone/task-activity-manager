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
    (lastResult.succeeded.length > 0 ||
      lastResult.conflicted.length > 0 ||
      lastResult.failed.length > 0);

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
            {lastResult!.conflicted.length > 0 && (
              <div className="conflict-text">
                <p>
                  <strong>
                    Conflict{lastResult!.conflicted.length === 1 ? "" : "s"} (
                    {lastResult!.conflicted.length})
                  </strong>{" "}
                  — remote moved since your edit. These stay in pending; sync
                  to pull the remote change, then re-commit or discard.
                </p>
                <ul className="commit-fail-list">
                  {lastResult!.conflicted.map((c, i) => (
                    <li key={i}>
                      <button
                        className="link-btn mono"
                        onClick={() => onJumpTo(c.testKey)}
                      >
                        {c.testKey}
                      </button>
                      : base{" "}
                      <code className="conflict-ts">{c.baseVersion}</code>{" "}
                      vs remote{" "}
                      <code className="conflict-ts">{c.remoteVersion}</code>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {lastResult!.failed.length > 0 && (
              <div className="error-text">
                <p>
                  Failed ({lastResult!.failed.length}) — these changes remain
                  in pending:
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
                {changes.map((c) => {
                  // Step entity_keys are "<testKey>:<xrayID>"; we route
                  // the jump-to action to the parent test and label the
                  // field as "step:<field>" so the modal makes it clear
                  // which kind of edit this is.
                  const isStep = c.entityType === "test_step";
                  const parentKey = isStep
                    ? c.entityKey.split(":")[0]
                    : c.entityKey;
                  const stepID = isStep
                    ? c.entityKey.substring(c.entityKey.indexOf(":") + 1)
                    : "";
                  return (
                    <tr key={c.id}>
                      <td>
                        <button
                          className="link-btn mono"
                          onClick={() => onJumpTo(parentKey)}
                          title="Open this test"
                        >
                          {parentKey}
                        </button>
                        {isStep && (
                          <span className="muted step-suffix">
                            {" · step "}
                            <span className="mono">{stepID}</span>
                          </span>
                        )}
                      </td>
                      <td>{isStep ? `step:${c.field}` : c.field}</td>
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
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        <div className="pending-actions">
          <p className="muted pending-footnote-inline">
            Successful commits leave this list; failures and conflicts stay
            and can be retried or discarded.
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
