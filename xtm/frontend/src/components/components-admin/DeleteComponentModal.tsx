import { useId, useState } from "react";
import { Modal } from "@agile-suite/core";
import type { ProjectComponent } from "../../api";
import { errMsg } from "../../api";

interface Props {
  component: ProjectComponent;
  others: ProjectComponent[];
  issueCount: number;
  onConfirm: (moveIssuesTo: string) => Promise<void>;
  onCancel: () => void;
}

// DeleteComponentModal confirms a delete. When issues use the component it
// offers to move them to another one first, which Jira does as part of the
// delete.
export function DeleteComponentModal({ component, others, issueCount, onConfirm, onCancel }: Props) {
  const [moveTo, setMoveTo] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const titleId = useId();

  async function confirm() {
    setBusy(true);
    setError("");
    try {
      await onConfirm(moveTo);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal onClose={onCancel} className="modal bulk-modal" role="alertdialog" labelledBy={titleId}>
      <div className="pending-head">
        <h2 id={titleId}>Delete {component.name}</h2>
      </div>
      <div className="bulk-body">
        <p>
          {issueCount === 0
            ? `No issues use ${component.name}.`
            : `${issueCount} ${issueCount === 1 ? "issue uses" : "issues use"} ${component.name} in Jira.`}
        </p>
        {issueCount > 0 && others.length > 0 && (
          <label className="bulk-row">
            <span>Move its issues to</span>
            <select value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
              <option value="">Nowhere (just remove the component)</option>
              {others.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </label>
        )}
        {error && <div className="error-text">{error}</div>}
      </div>
      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={busy}>
          Cancel
        </button>
        <button className="btn btn-danger" onClick={confirm} disabled={busy}>
          Delete component
        </button>
      </div>
    </Modal>
  );
}
