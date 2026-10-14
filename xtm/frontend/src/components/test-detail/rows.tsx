import { useState } from "react";
import { EditPreconditionField, EditTestCustomField, errMsg } from "../../api";
import type { Precondition, ContainerMembership, PendingChange, CustomFieldValue } from "../../api";

// CustomFieldRow renders one Jira custom field as a label + editable value
// (FR-2.6). All types edit as text — typed editors (select / date) need live
// editmeta. Saves on blur; a dirty marker comes from the pending journal.
export function CustomFieldRow({
  profileId,
  testKey,
  field,
  pendingForTest,
  readOnly,
  onLocalChange,
  onEdited,
}: {
  profileId: string;
  testKey: string;
  field: CustomFieldValue;
  pendingForTest: PendingChange[];
  readOnly?: boolean;
  onLocalChange: (value: string) => void;
  onEdited: () => void;
}) {
  const [value, setValue] = useState(field.value);
  const [saveError, setSaveError] = useState("");

  const entityKey = `${testKey}:${field.fieldId}`;
  const isDirty = pendingForTest.some(
    (p) => p.entityType === "custom_field" && p.entityKey === entityKey,
  );

  async function save() {
    if (readOnly) return;
    if (value === field.value) return;
    setSaveError("");
    try {
      await EditTestCustomField(profileId, testKey, field.fieldId, value);
      onLocalChange(value);
      onEdited();
    } catch (e) {
      setValue(field.value);
      setSaveError(errMsg(e));
    }
  }

  return (
    <>
      <dt>
        {field.name} {isDirty && <DirtyDot />}
      </dt>
      <dd>
        {readOnly ? (
          <span>{value || "—"}</span>
        ) : (
          <input
            className="detail-input detail-input-inline"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onBlur={save}
          />
        )}
        {saveError && <div className="error-text">{saveError}</div>}
      </dd>
    </>
  );
}

// PreconditionRow renders one linked Precondition with an inline-editable
// summary (FR-13.5 update) and a remove button (FR-13.6). Editing the summary
// updates the Precondition issue itself, so it changes everywhere that
// precondition is linked.
export function PreconditionRow({
  profileId,
  precondition,
  readOnly,
  onRemove,
  onEdited,
}: {
  profileId: string;
  precondition: Precondition;
  readOnly?: boolean;
  onRemove: (key: string) => void;
  onEdited: () => void;
}) {
  const [summary, setSummary] = useState(precondition.summary);
  const [saveError, setSaveError] = useState("");

  async function save() {
    if (readOnly) return;
    if (summary === precondition.summary) return;
    setSaveError("");
    try {
      await EditPreconditionField(profileId, precondition.key, "summary", summary);
      onEdited();
    } catch (e) {
      setSummary(precondition.summary);
      setSaveError(errMsg(e));
    }
  }

  return (
    <li>
      <span className="mono">{precondition.key}</span>
      {readOnly ? (
        <span className="pre-summary-static">{summary}</span>
      ) : (
        <input
          className="pre-summary-edit"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          onBlur={save}
        />
      )}
      {!readOnly && (
        <button
          className="btn btn-ghost pre-remove"
          onClick={() => onRemove(precondition.key)}
          title="Remove this precondition"
          aria-label="Remove this precondition"
        >
          ✕
        </button>
      )}
      {saveError && <span className="error-text">{saveError}</span>}
    </li>
  );
}

// ContainerSection lists the Test Sets / Plans / Executions a Test belongs to
// (FR-1.3). Execution memberships also show the Test Run status badge.
export function ContainerSection({
  title,
  items,
  showRunStatus,
  readOnly,
  onRemove,
}: {
  title: string;
  items: ContainerMembership[];
  showRunStatus?: boolean;
  readOnly?: boolean;
  onRemove: (containerKey: string) => void;
}) {
  return (
    <>
      <h4>{title}</h4>
      {items.length === 0 ? (
        <p className="muted">None linked</p>
      ) : (
        <ul className="pre-list">
          {items.map((c) => (
            <li key={c.key}>
              <span className="mono">{c.key}</span> — {c.summary}
              {showRunStatus && c.runStatus && (
                <RunStatusBadge status={c.runStatus} />
              )}
              {!readOnly && (
                <button
                  className="btn btn-ghost pre-remove"
                  onClick={() => onRemove(c.key)}
                  title="Remove this test from the container"
                  aria-label="Remove this test from the container"
                >
                  ✕
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

// RunStatusBadge renders a Test Run result with a status-coloured pill. The
// class is derived from the lowercased status so the CSS can theme PASS / FAIL
// / TODO distinctly while unknown statuses fall back to a neutral style.
export function RunStatusBadge({ status }: { status: string }) {
  return (
    <span className={`run-badge run-${status.toLowerCase()}`}>{status}</span>
  );
}

export function verdictLabel(verdict?: string): string {
  switch (verdict) {
    case "approved":
      return "Approved";
    case "rejected":
      return "Rejected";
    case "pending":
      return "Pending review";
    default:
      return "Not reviewed";
  }
}

export function DirtyDot() {
  return (
    <span className="dirty-dot" title="Pending edit">
      ●
    </span>
  );
}
