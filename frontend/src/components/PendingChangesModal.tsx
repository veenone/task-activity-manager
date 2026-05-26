import type { PendingChange } from "../api";

interface Props {
  changes: PendingChange[];
  onDiscard: (id: number) => Promise<void> | void;
  onJumpTo: (testKey: string) => void;
  onClose: () => void;
}

export function PendingChangesModal({
  changes,
  onDiscard,
  onJumpTo,
  onClose,
}: Props) {
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
                      <button className="btn" onClick={() => onDiscard(c.id)}>
                        Discard
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <p className="muted pending-footnote">
          Discarding reverts the field to its synced value and removes the
          entry from this list. Commit-to-Jira lands in a later slice.
        </p>
      </div>
    </div>
  );
}

function truncate(s: string, n: number): string {
  if (!s) return "";
  if (s.length <= n) return s;
  return s.slice(0, n) + "…";
}
