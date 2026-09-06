import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, call, errMsg, useProfile } from "@agile-suite/core";
import { CreateIssue, ISSUE_TYPES } from "../api";
import type { FieldSpec, IssueDraft, IssueType, Profile, Settings } from "../api";
import { useCreateFields } from "../queries/pending";
import { invalidateWrites } from "../queries/invalidate";

interface Props {
  onClose: () => void;
  onCreated: (key: string) => void;
}

// CREATABLE are the types plan 1b can draft. Requirements and epics arrive
// with plan 1c.
const CREATABLE: IssueType[] = ["task", "story", "bug"];

// MetaField renders one create-meta field by its schema type: a select for
// option and array fields with values, a date or number input, or text.
function MetaField({ spec, value, onChange }: { spec: FieldSpec; value: string; onChange: (v: string) => void }) {
  const id = `meta-${spec.id}`;
  if ((spec.type === "option" || spec.type === "array") && spec.allowedValues.length > 0) {
    return (
      <label className="edit-row" htmlFor={id}>
        <span className="muted small">{spec.name}</span>
        <select id={id} className="detail-input" value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">Choose</option>
          {spec.allowedValues.map((o) => (
            <option key={o.id} value={o.id}>{o.value}</option>
          ))}
        </select>
      </label>
    );
  }
  return (
    <label className="edit-row" htmlFor={id}>
      <span className="muted small">{spec.name}</span>
      <input
        id={id}
        className="detail-input"
        type={spec.type === "date" ? "date" : "text"}
        inputMode={spec.type === "number" ? "decimal" : undefined}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  );
}

export function NewIssueModal({ onClose, onCreated }: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const qc = useQueryClient();
  const [type, setType] = useState<IssueType>("task");
  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("");
  const [labels, setLabels] = useState("");
  const [assignee, setAssignee] = useState("");
  const [points, setPoints] = useState("");
  const [extra, setExtra] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const meta = useCreateFields(activeId, type);
  const specs = meta.data ?? [];
  const summaryRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    summaryRef.current?.focus();
  }, []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (summary.trim() === "") {
      setError("Summary cannot be empty.");
      return;
    }
    if (points.trim() !== "" && Number.isNaN(Number(points.trim()))) {
      setError("Story points must be a number.");
      return;
    }
    for (const s of specs) {
      if (s.required && !(extra[s.id] ?? "").trim()) {
        setError(`${s.name} is required.`);
        return;
      }
    }
    const draft: IssueDraft = {
      type,
      summary: summary.trim(),
      description,
      priority: priority.trim(),
      labels: labels.split(",").map((l) => l.trim()).filter(Boolean),
      assignee: assignee.trim(),
      storyPoints: points.trim() === "" ? null : Number(points.trim()),
      extra: Object.fromEntries(Object.entries(extra).filter(([, v]) => v.trim() !== "")),
    };
    setError("");
    setSaving(true);
    try {
      const key = await call(() => CreateIssue(activeId, draft));
      invalidateWrites(qc, activeId);
      onCreated(key);
      onClose();
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal onClose={onClose} className="modal new-issue-modal" labelledBy="new-issue-title">
      <div className="pending-head">
        <h2 id="new-issue-title">New issue</h2>
        <span className="muted">{activeProfile ? `in ${activeProfile.projectKey}` : ""}</span>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>
      <form className="edit-form" onSubmit={(e) => void onSubmit(e)}>
        <label className="edit-row" htmlFor="new-type">
          <span className="muted small">Type</span>
          <select id="new-type" className="detail-input" value={type} onChange={(e) => { setType(e.target.value as IssueType); setExtra({}); setError(""); }}>
            {ISSUE_TYPES.filter((t) => CREATABLE.includes(t.id)).map((t) => (
              <option key={t.id} value={t.id}>{t.label}</option>
            ))}
          </select>
        </label>
        <label className="edit-row" htmlFor="new-summary">
          <span className="muted small">Summary</span>
          <input id="new-summary" ref={summaryRef} className="detail-input" type="text" value={summary} onChange={(e) => setSummary(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-description">
          <span className="muted small">Description</span>
          <textarea id="new-description" className="detail-input" rows={4} value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-priority">
          <span className="muted small">Priority</span>
          <input id="new-priority" className="detail-input" type="text" placeholder="Jira's default when empty" value={priority} onChange={(e) => setPriority(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-labels">
          <span className="muted small">Labels</span>
          <input id="new-labels" className="detail-input" type="text" placeholder="comma separated" value={labels} onChange={(e) => setLabels(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-assignee">
          <span className="muted small">Assignee</span>
          <input id="new-assignee" className="detail-input" type="text" placeholder="Jira username" value={assignee} onChange={(e) => setAssignee(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="new-points">
          <span className="muted small">Story points</span>
          <input id="new-points" className="detail-input" type="text" inputMode="decimal" value={points} onChange={(e) => setPoints(e.target.value)} />
        </label>

        <div className="meta-fields">
          {meta.isError ? (
            <p className="muted small">Jira's required fields could not be read ({meta.error.message}). The form stays minimal; Jira validates the rest on Commit.</p>
          ) : meta.isPending ? (
            <p className="muted small">Checking which fields Jira requires</p>
          ) : specs.length > 0 ? (
            <>
              <p className="muted small">Jira requires these for a {ISSUE_TYPES.find((t) => t.id === type)?.label ?? type}:</p>
              {specs.map((s) => (
                <MetaField key={s.id} spec={s} value={extra[s.id] ?? ""} onChange={(v) => setExtra((cur) => ({ ...cur, [s.id]: v }))} />
              ))}
            </>
          ) : null}
        </div>

        <div className="edit-actions">
          <button type="submit" className="btn btn-primary" disabled={saving}>{saving ? "Creating" : "Create draft"}</button>
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          {error ? (
            <span className="error-text small" role="alert">{error}</span>
          ) : (
            <span className="muted small">The draft joins the Backlog now; Commit creates it in Jira.</span>
          )}
        </div>
      </form>
    </Modal>
  );
}
