import { useRef, useState } from "react";
import { CreatePreconditionDetailed, EditPreconditionField, GetPreconditionCreateFields, errMsg } from "../../api";
import { MarkdownField } from "../MarkdownField";
import { Modal } from "../Modal";
import { CreateFieldInputs, useCreateScreenFields } from "../CreateFieldInputs";
import { PRECOND_TYPES } from "./usePreconditionsState";

// CreatePreconditionModal collects a summary, type, description and condition
// for a new Precondition (FR-13.4 / 13.5), queued for creation in Jira on
// commit. Description and Condition both use MarkdownField for consistency with
// the detail pane. The condition is applied via EditPreconditionField after
// create so the CreatePreconditionDetailed signature stays unchanged.
export function CreatePreconditionModal({
  profileId,
  onCancel,
  onCreated,
}: {
  profileId: string;
  onCancel: () => void;
  onCreated: (key: string) => void;
}) {
  const focusRef = useRef<HTMLInputElement>(null);
  const [summary, setSummary] = useState("");
  const [type, setType] = useState("Manual");
  const [description, setDescription] = useState("");
  const [condition, setCondition] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const extra = useCreateScreenFields(
    () => GetPreconditionCreateFields(profileId),
    profileId,
  );

  async function create() {
    if (!summary.trim() || !extra.valid) return;
    setBusy(true);
    setError("");
    try {
      const key = await CreatePreconditionDetailed(
        profileId,
        summary.trim(),
        type,
        description,
        extra.payload(),
      );
      // The condition is journaled as an ordinary field edit, which the commit
      // pushes through Xray's resolved condition field.
      if (condition.trim()) {
        await EditPreconditionField(profileId, key, "condition", condition);
      }
      onCreated(key);
    } catch (e) {
      setError(errMsg(e));
      setBusy(false);
    }
  }

  return (
    <Modal initialFocus={focusRef} onClose={onCancel} className="modal" labelledBy="new-precond-title">
      <div className="pending-head">
        <h2 id="new-precond-title">New precondition</h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close" aria-label="Close">
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <label className="precond-field">
          <span>Summary</span>
          <input
            className="detail-input"
            ref={focusRef}
            placeholder="e.g. User is logged in"
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") create();
            }}
          />
        </label>
        <label className="precond-field">
          <span>Type</span>
          <select
            className="detail-input"
            value={type}
            onChange={(e) => setType(e.target.value)}
          >
            {PRECOND_TYPES.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </label>
        <div className="precond-field">
          <span>Condition</span>
          <MarkdownField
            className="detail-input precond-desc"
            value={condition}
            onChange={setCondition}
            onCommit={() => {}}
            rows={3}
            placeholder="e.g. Given the user is authenticated. Markdown supported."
          />
        </div>
        <div className="precond-field">
          <span>Description</span>
          <MarkdownField
            className="detail-input precond-desc"
            value={description}
            onChange={setDescription}
            onCommit={() => {}}
            rows={3}
            placeholder="Optional. Markdown supported."
          />
        </div>
        <CreateFieldInputs fields={extra.fields} values={extra.values} onChange={extra.setValue} />
        {error && <div className="error-text">{error}</div>}
      </div>
      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={busy}>
          Cancel
        </button>
        <button
          className="btn btn-primary"
          onClick={create}
          disabled={busy || !summary.trim() || !extra.valid}
        >
          {busy ? "Creating…" : "Create"}
        </button>
      </div>
    </Modal>
  );
}
