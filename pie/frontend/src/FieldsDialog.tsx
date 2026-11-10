import { useState } from "react";
import { Modal } from "@agile-suite/core";
import { SetFields } from "../wailsjs/go/main/App";
import type { saved } from "../wailsjs/go/models";
import { errorText } from "./ProfileForm";

const NAMES: { key: keyof saved.Fields; label: string }[] = [
  { key: "productBase", label: "Product base" },
  { key: "clientVisibility", label: "Client visibility" },
  { key: "clientImpact", label: "Client impact" },
];

// FieldsDialog names the profile's Jira custom fields. Go checks every name
// against Jira and stores none if one is refused, so the dialog stays open
// with the analyst's text until all of them pass.
export function FieldsDialog({
  profileId,
  fields,
  onClose,
  onSaved,
}: {
  profileId: string;
  fields: saved.Fields;
  onClose: () => void;
  onSaved: (fields: saved.Fields) => void;
}) {
  const [draft, setDraft] = useState<saved.Fields>(fields);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  async function save() {
    setSaving(true);
    setError("");
    try {
      await SetFields(profileId, draft);
      onSaved(draft);
    } catch (e) {
      setError(errorText(e));
      setSaving(false);
    }
  }

  return (
    <Modal onClose={onClose} labelledBy="pie-fields">
      <form
        className="profile-form"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <h2 id="pie-fields">Jira fields</h2>
        <p className="muted">Custom field names as Jira shows them. Leave a name empty if the profile does not use it. Run the query again after a change.</p>
        {NAMES.map(({ key, label }) => (
          <label key={key}>
            {label}
            <input value={draft[key]} onChange={(e) => setDraft({ ...draft, [key]: e.target.value })} />
          </label>
        ))}
        {error && <p className="error-text">{error}</p>}
        <div className="form-actions form-actions-end">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            Save
          </button>
        </div>
      </form>
    </Modal>
  );
}
