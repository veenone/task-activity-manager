import { useId, useState } from "react";
import { Modal } from "@agile-suite/core";
import { useProfile } from "../../contexts/ProfileContext";
import { useUserSearch } from "../../queries/components";
import type { ComponentInput, ProjectComponent } from "../../api";
import { errMsg } from "../../api";

const ASSIGNEE_TYPES = [
  { value: "PROJECT_DEFAULT", label: "Project default" },
  { value: "COMPONENT_LEAD", label: "Component lead" },
  { value: "PROJECT_LEAD", label: "Project lead" },
  { value: "UNASSIGNED", label: "Unassigned" },
];

interface Props {
  initial?: ProjectComponent;
  takenNames: string[];
  onSubmit: (input: ComponentInput) => Promise<void>;
  onCancel: () => void;
}

// ComponentFormModal creates or edits one component. Name checks run here so
// a blank or duplicate name never reaches Jira; Jira checks again.
export function ComponentFormModal({
  initial,
  takenNames,
  onSubmit,
  onCancel,
}: Props) {
  const { activeId: profileId } = useProfile();
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [lead, setLead] = useState({
    name: initial?.leadName ?? "",
    display: initial?.leadDisplayName ?? "",
  });
  const [leadQuery, setLeadQuery] = useState("");
  const [assigneeType, setAssigneeType] = useState(
    initial?.assigneeType || "PROJECT_DEFAULT",
  );
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const users = useUserSearch(profileId, leadQuery);
  const titleId = useId();

  async function submit() {
    const trimmed = name.trim();
    if (!trimmed) {
      setError("A component needs a name.");
      return;
    }
    if (takenNames.some((n) => n.toLowerCase() === trimmed.toLowerCase())) {
      setError(`A component named "${trimmed}" already exists.`);
      return;
    }
    setSaving(true);
    setError("");
    try {
      await onSubmit({
        name: trimmed,
        description: description.trim(),
        leadUserName: lead.name,
        assigneeType,
      });
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal onClose={onCancel} className="modal bulk-modal" labelledBy={titleId}>
      <div className="pending-head">
        <h2 id={titleId}>
          {initial ? `Edit ${initial.name}` : "New component"}
        </h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close">
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <label className="bulk-row">
          <span>Name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="bulk-row">
          <span>Description</span>
          <textarea
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <div className="bulk-row">
          <span>Lead</span>
          <div className="component-lead">
            {lead.name && (
              <p className="component-lead-current">
                {lead.display || lead.name}
                <button
                  className="btn btn-ghost"
                  onClick={() => setLead({ name: "", display: "" })}
                >
                  Clear lead
                </button>
              </p>
            )}
            <input
              type="search"
              aria-label="Lead"
              placeholder="Search users"
              value={leadQuery}
              onChange={(e) => setLeadQuery(e.target.value)}
            />
            {(users.data ?? []).length > 0 && (
              <ul className="component-lead-results">
                {(users.data ?? []).map((u) => (
                  <li key={u.name}>
                    <button
                      className="btn btn-ghost"
                      onClick={() => {
                        setLead({ name: u.name, display: u.displayName });
                        setLeadQuery("");
                      }}
                    >
                      {`${u.displayName} (${u.name})`}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
        <label className="bulk-row">
          <span>Default assignee</span>
          <select
            value={assigneeType}
            onChange={(e) => setAssigneeType(e.target.value)}
          >
            {ASSIGNEE_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>
        </label>
        {error && <div className="error-text">{error}</div>}
      </div>
      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={saving}>
          Cancel
        </button>
        <button className="btn btn-primary" onClick={submit} disabled={saving}>
          {saving
            ? initial
              ? "Saving…"
              : "Creating…"
            : initial
              ? "Save"
              : "Create"}
        </button>
      </div>
    </Modal>
  );
}
