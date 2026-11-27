import { useState } from "react";
import type { FormEvent } from "react";
import { ASSIGNEE_TYPES, Modal, announce, errMsg } from "@agile-suite/core";
import type { ComponentSummary } from "../api";
import { useSync } from "../contexts/SyncContext";
import { useCreateComponent, useEditComponent } from "../queries/components";
import { AssigneePicker } from "./AssigneePicker";

interface Props {
  profileId: string;
  // component is the one being edited; without it the dialog drafts one.
  component?: ComponentSummary;
  // takenNames are the other components' names, checked before sending.
  takenNames: string[];
  onClose: () => void;
  // onSaved hands back the component's id and the sentence for the banner.
  onSaved: (id: number, line: string) => void;
}

// ComponentFormModal drafts a new component or edits one, with XTM's fields
// and wording. Nothing reaches Jira until Commit: a new component is a draft
// under a negative id, and an edit of one Jira holds is a pending change.
export function ComponentFormModal({ profileId, component, takenNames, onClose, onSaved }: Props) {
  const { runQuietLock } = useSync();
  const create = useCreateComponent(profileId, runQuietLock);
  const edit = useEditComponent(profileId, runQuietLock);
  const [name, setName] = useState(component?.name ?? "");
  const [description, setDescription] = useState(component?.description ?? "");
  const [lead, setLead] = useState({ name: component?.leadName ?? "", display: component?.leadDisplayName ?? "" });
  const [assigneeType, setAssigneeType] = useState(component?.assigneeType || "PROJECT_DEFAULT");
  const [error, setError] = useState("");
  const busy = create.isPending || edit.isPending;

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    const trimmed = name.trim();
    if (!trimmed) {
      setError("A component needs a name.");
      return;
    }
    if (takenNames.some((n) => n.toLowerCase() === trimmed.toLowerCase())) {
      setError(`A component named "${trimmed}" already exists.`);
      return;
    }
    const fields = { name: trimmed, description: description.trim(), leadName: lead.name, leadDisplayName: lead.name ? lead.display : "", assigneeType };
    const done = (id: number, line: string) => {
      announce(line);
      onSaved(id, line);
      onClose();
    };
    if (!component) {
      create.mutate(fields, {
        onSuccess: (made) => done(made.id, `${made.name || trimmed} was drafted. Commit creates it in Jira.`),
        onError: (err) => setError(errMsg(err)),
      });
      return;
    }
    edit.mutate({ ...fields, id: component.id }, {
      onSuccess: () => done(component.id, component.draft
        ? `${trimmed} was saved.`
        : `${trimmed} was saved locally. Commit sends the change to Jira.`),
      onError: (err) => setError(errMsg(err)),
    });
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="component-form-title" closeOnOverlayClick={false} closeOnEsc={!busy}>
      <div className="pending-head">
        <h2 id="component-form-title">{component ? `Edit ${component.name}` : "New component"}</h2>
        <p className="muted small">{component && !component.draft ? "Saved locally. Commit sends the change to Jira." : "Drafted locally. Commit creates it in Jira."}</p>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="component-form" className="bulk-body edit-form" onSubmit={onSubmit}>
        <label className="edit-row" htmlFor="component-name">
          <span className="muted small">Name</span>
          <input id="component-name" className="detail-input" value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="component-description">
          <span className="muted small">Description</span>
          <textarea id="component-description" className="detail-input" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <div className="edit-row">
          <label className="muted small" htmlFor="component-lead">Lead</label>
          <AssigneePicker
            profileId={profileId}
            id="component-lead"
            value={lead.name}
            fallbackLabel={lead.display}
            onChange={(username, display) => setLead({ name: username, display: display ?? "" })}
          />
        </div>
        <label className="edit-row" htmlFor="component-assignee">
          <span className="muted small">Default assignee</span>
          <select id="component-assignee" className="detail-input" value={assigneeType} onChange={(e) => setAssigneeType(e.target.value)}>
            {ASSIGNEE_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
          </select>
        </label>
      </form>

      <div className="pending-actions">
        <span className="new-issue-status">
          {error && <span className="error-text small" role="alert">{error}</span>}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" form="component-form" className="btn btn-primary" disabled={busy}>
            {busy ? (component ? "Saving" : "Creating") : component ? "Save" : "Create"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
