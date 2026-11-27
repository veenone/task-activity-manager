import { useState } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import type { ComponentSummary } from "../api";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";
import { useDeleteComponent } from "../queries/components";

interface Props {
  profileId: string;
  component: ComponentSummary;
  // others are the project's other components, for "Move its issues to".
  others: ComponentSummary[];
  onClose: () => void;
  onDeleted: (line: string) => void;
}

// DeleteComponentModal deletes a component. As in XTM, it offers to move the
// component's issues to another component first. A draft goes at once and
// leaves every issue it was set on; a component Jira holds is deleted on
// Commit, which moves its issues as part of the delete.
export function DeleteComponentModal({ profileId, component, others, onClose, onDeleted }: Props) {
  const { runQuietLock } = useSync();
  const del = useDeleteComponent(profileId, runQuietLock);
  const [moveTo, setMoveTo] = useState("0");
  const [error, setError] = useState("");
  const name = component.name;

  function confirm() {
    if (del.isPending) return;
    del.mutate({ id: component.id, moveTo: component.draft ? 0 : Number(moveTo) }, {
      onSuccess: () => {
        const line = component.draft ? `The draft ${name} was deleted.` : `${name} will be deleted on Commit.`;
        announce(line);
        onDeleted(line);
        onClose();
      },
      onError: (err) => setError(errMsg(err)),
    });
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" role="alertdialog" labelledBy="delete-component-title" closeOnOverlayClick={false} closeOnEsc={!del.isPending}>
      <div className="pending-head">
        <h2 id="delete-component-title">Delete {name}</h2>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>
      <div className="bulk-body edit-form">
        {component.draft ? (
          <p>Nothing reaches Jira. Issues you gave this component lose it.</p>
        ) : (
          <>
            <p>
              {component.issues === 0
                ? `No issues TAM holds use ${name}.`
                : `${plural(component.issues, "issue", "issues")} TAM holds ${component.issues === 1 ? "uses" : "use"} ${name}.`}
            </p>
            {others.length > 0 && (
              <label className="edit-row" htmlFor="delete-component-move">
                <span className="muted small">Move its issues to</span>
                <select id="delete-component-move" className="detail-input" value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
                  <option value="0">Nowhere (just remove the component)</option>
                  {others.map((o) => <option key={o.id} value={String(o.id)}>{o.name}</option>)}
                </select>
              </label>
            )}
            <p className="muted small">Commit deletes it in Jira. Until then you can discard the delete in Pending changes.</p>
          </>
        )}
        {error && <p className="error-text" role="alert">{error}</p>}
      </div>
      <div className="pending-actions">
        <span />
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={del.isPending}>Cancel</button>
          <button type="button" className="btn btn-danger" disabled={del.isPending} onClick={confirm}>Delete component</button>
        </span>
      </div>
    </Modal>
  );
}
