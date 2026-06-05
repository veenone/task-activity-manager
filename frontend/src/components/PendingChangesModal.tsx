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
                  // Step entity_keys are "<testKey>:<xrayID>"; route the
                  // jump-to action to the parent test and label the row so
                  // the modal makes clear which kind of step change this is
                  // (edit / add / delete).
                  const isStep = c.entityType.startsWith("test_step");
                  const hasStepID = isStep && c.entityKey.includes(":");
                  const parentKey = hasStepID
                    ? c.entityKey.split(":")[0]
                    : c.entityKey;
                  const stepID = hasStepID
                    ? c.entityKey.substring(c.entityKey.indexOf(":") + 1)
                    : "";
                  const { field, before, after } = describeChange(c);
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
                        {hasStepID && (
                          <span className="muted step-suffix">
                            {" · step "}
                            <span className="mono">{stepID}</span>
                          </span>
                        )}
                      </td>
                      <td>{field}</td>
                      <td className="pending-before">
                        {truncate(before, 100)}
                      </td>
                      <td className="pending-after">
                        {truncate(after, 100)}
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

// describeChange renders a pending row's field label and before/after for the
// table. Structural step ops (add / delete) carry the step as JSON in one
// column; we surface the action text so the row reads sensibly instead of
// dumping raw JSON.
function describeChange(c: PendingChange): {
  field: string;
  before: string;
  after: string;
} {
  switch (c.entityType) {
    case "test_step":
      return { field: `step:${c.field}`, before: c.beforeVal, after: c.afterVal };
    case "test_step_add":
      return { field: "step: new", before: "", after: stepAction(c.afterVal) };
    case "test_step_delete":
      return { field: "step: delete", before: stepAction(c.beforeVal), after: "" };
    case "test_step_order":
      return {
        field: "steps: reorder",
        before: orderSummary(c.beforeVal),
        after: orderSummary(c.afterVal),
      };
    case "test_membership_add":
      return {
        field: "allocate",
        before: "",
        after: membershipSummary(c.afterVal),
      };
    default:
      return { field: c.field, before: c.beforeVal, after: c.afterVal };
  }
}

// membershipSummary renders an allocation payload ({kind, members}) as
// "N tests" so the pending row reads clearly instead of showing raw JSON.
function membershipSummary(json: string): string {
  try {
    const p = JSON.parse(json) as { members?: string[] };
    const n = p.members?.length ?? 0;
    return `${n} ${n === 1 ? "test" : "tests"}`;
  } catch {
    return json;
  }
}

// orderSummary renders a step-order JSON array as a compact "N steps: a → b →
// …" line so the reorder row reads at a glance.
function orderSummary(json: string): string {
  try {
    const ids = JSON.parse(json) as string[];
    return `${ids.length} steps: ${ids.join(" → ")}`;
  } catch {
    return json;
  }
}

// stepAction pulls the human-readable action out of a step JSON snapshot,
// falling back to the raw string if it isn't the expected shape.
function stepAction(json: string): string {
  try {
    const s = JSON.parse(json) as { action?: string };
    return s.action ?? "";
  } catch {
    return json;
  }
}

function truncate(s: string, n: number): string {
  if (!s) return "";
  if (s.length <= n) return s;
  return s.slice(0, n) + "…";
}
