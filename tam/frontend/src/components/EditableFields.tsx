import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { errMsg } from "@agile-suite/core";
import { EDITABLE_FIELDS } from "../api";
import type { EditableField, Issue } from "../api";
import { useEditIssue } from "../queries/pending";
import { useEpics } from "../queries/tree";

// EPIC_SUMMARY_MAX is how much of an epic's summary shows in its option
// label before it is cut off with a single ellipsis character.
const EPIC_SUMMARY_MAX = 60;

function epicOptionLabel(key: string, summary: string): string {
  const cut = summary.length > EPIC_SUMMARY_MAX ? `${summary.slice(0, EPIC_SUMMARY_MAX)}…` : summary;
  return `${key} ${cut}`;
}

interface Props {
  profileId: string;
  issue: Issue;
  // description is the cached detail's text; descriptionReady says the
  // detail has loaded, so the textarea is enabled and its edit is genuine.
  description: string;
  descriptionReady: boolean;
  // busy says a sync or commit is running; Save stays disabled so an edit
  // made mid-commit is never lost to the row refresh that follows it.
  busy: boolean;
}

type Values = Record<EditableField, string>;

function valuesOf(issue: Issue, description: string): Values {
  return {
    summary: issue.summary,
    description,
    priority: issue.priority,
    labels: issue.labels.join(", "),
    storyPoints: issue.storyPoints === null || issue.storyPoints === undefined ? "" : String(issue.storyPoints),
    assignee: issue.assignee,
    parentKey: issue.parentKey,
  };
}

// EditableFields is the write half of the Details tab. Each field the user
// changes becomes one journal row when Save edit is pressed; unchanged
// fields are not sent. Validation mirrors the store's so the common
// mistakes never round-trip.
export function EditableFields({ profileId, issue, description, descriptionReady, busy }: Props) {
  const base = valuesOf(issue, description);
  const [values, setValues] = useState<Values>(base);
  const [dirty, setDirty] = useState<Set<EditableField>>(new Set());
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const edit = useEditIssue(profileId);
  const epics = useEpics(profileId);

  // A fresh row from the backend (after save, sync, or commit) resets the
  // fields the user has not touched; dirty ones keep their text.
  useEffect(() => {
    setValues((cur) => {
      const next = { ...base };
      for (const f of dirty) next[f] = cur[f];
      return next;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [issue, description]);

  function set(field: EditableField, v: string) {
    setValues((cur) => ({ ...cur, [field]: v }));
    setDirty((cur) => {
      const next = new Set(cur);
      if (v === base[field]) next.delete(field);
      else next.add(field);
      return next;
    });
    setSaved(false);
    setError("");
  }

  const changed = EDITABLE_FIELDS.map((f) => f.id).filter((f) => values[f] !== base[f]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (values.summary.trim() === "") {
      setError("Summary cannot be empty.");
      return;
    }
    if (values.storyPoints.trim() !== "" && Number.isNaN(Number(values.storyPoints.trim()))) {
      setError("Story points must be a number.");
      return;
    }
    setError("");
    try {
      for (const field of changed) {
        await edit.mutateAsync({ key: issue.key, field, value: values[field] });
      }
      setDirty(new Set());
      setSaved(true);
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <form className="edit-form" onSubmit={(e) => void onSubmit(e)} aria-label="Edit fields">
      {EDITABLE_FIELDS.filter((f) => f.id !== "parentKey" || issue.type !== "epic").map((f) => (
        <label key={f.id} className="edit-row">
          <span className="muted small">{f.label}</span>
          {f.id === "description" ? (
            <textarea
              className="detail-input"
              rows={5}
              value={values.description}
              disabled={!descriptionReady}
              placeholder={descriptionReady ? "" : "Loading the description"}
              onChange={(e) => set("description", e.target.value)}
            />
          ) : f.id === "parentKey" ? (
            epics.isLoading ? (
              <select className="detail-input" disabled value="">
                <option value="">(loading)</option>
              </select>
            ) : (
              <select
                className="detail-input"
                value={values.parentKey}
                onChange={(e) => set("parentKey", e.target.value)}
              >
                <option value="">(none)</option>
                {(epics.data ?? []).map((epic) => (
                  <option key={epic.key} value={epic.key}>{epicOptionLabel(epic.key, epic.summary)}</option>
                ))}
              </select>
            )
          ) : (
            <input
              className="detail-input"
              type="text"
              inputMode={f.id === "storyPoints" ? "decimal" : undefined}
              value={values[f.id]}
              onChange={(e) => set(f.id, e.target.value)}
            />
          )}
        </label>
      ))}
      <div className="edit-actions">
        <button type="submit" className="btn btn-primary" disabled={changed.length === 0 || edit.isPending || busy}>
          {edit.isPending ? "Saving" : "Save edit"}
        </button>
        {error ? (
          <span className="error-text small" role="alert">{error}</span>
        ) : saved ? (
          <span className="muted small" role="status">Saved. Commit pushes it to Jira.</span>
        ) : (
          <span className="muted small">Labels are a comma list. Saving journals the change; nothing reaches Jira until Commit.</span>
        )}
      </div>
    </form>
  );
}
