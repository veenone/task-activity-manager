import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { RichText, RichTextField, SyntaxToggle, detectFormat, errMsg, toPlainText } from "@agile-suite/core";
import type { RichFormat } from "@agile-suite/core";
import { EDITABLE_FIELDS } from "../api";
import type { EditableField, Issue } from "../api";
import { useEditIssue, useEditableFields } from "../queries/pending";
import { useEpics } from "../queries/tree";
import { AssigneePicker } from "./AssigneePicker";
import { PriorityPicker } from "./PriorityPicker";

// The syntax the user picked for one issue's description, keyed by profile,
// key and field so two issues never share a choice. It is module level and
// lives for the app run: the spec's "session", which is what makes a choice
// survive the panel closing and reopening on the same issue without being
// stored anywhere.
const pickedFormats = new Map<string, RichFormat>();

function memoryKey(profileId: string, issueKey: string): string {
  return `${profileId}:${issueKey}:description`;
}

// descriptionFormat is the syntax a description is read in: the user's pick
// if there is one, detection otherwise. The panel asks for it to decide
// which comments read in a syntax different enough to be worth a chip.
export function descriptionFormat(profileId: string, issueKey: string, text: string): RichFormat {
  return pickedFormats.get(memoryKey(profileId, issueKey)) ?? detectFormat(text).format;
}

// EPIC_SUMMARY_MAX is how much of an epic's summary shows in its option
// label before it is cut off with a single ellipsis character.
const EPIC_SUMMARY_MAX = 60;

function epicOptionLabel(key: string, summary: string): string {
  const plain = toPlainText(summary, "summary");
  const cut = plain.length > EPIC_SUMMARY_MAX ? `${plain.slice(0, EPIC_SUMMARY_MAX)}…` : plain;
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
  // projectKey, onOpenLink and onIssueKey are what the rendered description
  // needs to reach the instance: they are computed once by whoever knows the
  // profile's Jira URL rather than twice, here and beside the comments.
  projectKey: string;
  onOpenLink: (url: string) => void;
  onIssueKey?: (key: string) => void;
  // onSyntaxPicked lets the panel redraw when the toggle moves: the comment
  // chips are drawn against the description's syntax, and this component's
  // own state change would not reach them.
  onSyntaxPicked?: () => void;
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
export function EditableFields({ profileId, issue, description, descriptionReady, busy, projectKey, onOpenLink, onIssueKey, onSyntaxPicked }: Props) {
  const base = valuesOf(issue, description);
  const [values, setValues] = useState<Values>(base);
  const [dirty, setDirty] = useState<Set<EditableField>>(new Set());
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  // The editor is held open by the key it was opened on, so moving to
  // another issue leaves its read view showing rather than an editor over a
  // description the reader never asked to change.
  const [editingKey, setEditingKey] = useState("");
  // The picked syntax lives in a module map, not in state, so it survives
  // this component unmounting with the panel; the counter is only what turns
  // a pick into a re-render.
  const [, bumpFormat] = useState(0);
  const edit = useEditIssue(profileId);
  const epics = useEpics(profileId);
  const screen = useEditableFields(profileId, issue.type, issue.key);

  // A field Jira does not list is shown, and disabled, rather than hidden.
  // The user can see Story points on the issue in Jira and reads it here in
  // the grid, so a field that quietly vanished from this form would read as
  // a bug in TAM; disabled with the reason beside it says who has to change
  // what. An empty answer means nothing is known, not that nothing may be
  // edited, so a never-synced profile keeps the whole list.
  //
  // Resolved only once the read has landed and said so, the same shape the
  // create dialog uses for its own screen answer. The query cache is empty
  // at every app start and the backend asks Jira before it reads its own
  // store, so there is a real window with no answer in hand; drawing an
  // enabled control through it and disabling it a round trip later is how a
  // value gets typed into a field Jira will refuse.
  const listed = screen.isSuccess && screen.data.length > 0 ? new Set<EditableField>(screen.data) : null;
  const offScreen = (field: EditableField) => listed !== null && !listed.has(field);
  // locked covers both: a field Jira will not take, and every field while
  // nothing is known yet. Nothing locked can be typed into, and nothing
  // locked can be saved, because disabled is a control state and not a write
  // guard: a value can still reach the form from the issue shown before this
  // one, which the reset effect keeps on purpose.
  const locked = (field: EditableField) => screen.isLoading || offScreen(field);

  const editing = editingKey === issue.key;
  const picked = pickedFormats.get(memoryKey(profileId, issue.key));
  const detected = useMemo(() => detectFormat(values.description).format, [values.description]);

  function pickFormat(f: RichFormat) {
    pickedFormats.set(memoryKey(profileId, issue.key), f);
    bumpFormat((n) => n + 1);
    onSyntaxPicked?.();
  }

  // Cancel restores the text the detail was read with and clears the dirty
  // mark with it, so nothing is journaled by a Save of another field.
  function cancelEdit() {
    set("description", base.description);
    setEditingKey("");
  }

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

  const changed = EDITABLE_FIELDS.map((f) => f.id).filter((f) => values[f] !== base[f] && !locked(f));
  // Whether anything on this form is refused, which is what the one sentence
  // about administrators is worth saying for. Per field it would repeat, and
  // the project this was found on leaves two of the seven off every screen.
  const anyOffScreen = EDITABLE_FIELDS.some((f) => offScreen(f.id));

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
      // Saved text is what the read view now shows, so the editor has
      // nothing left to hold open.
      setEditingKey("");
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <form className="edit-form" onSubmit={(e) => void onSubmit(e)} aria-label="Edit fields">
      {EDITABLE_FIELDS.filter((f) => f.id !== "parentKey" || issue.type !== "epic").map((f) => {
        const off = offScreen(f.id);
        const shut = locked(f.id);
        return (
        <div key={f.id} className="edit-row">
          {f.id === "description" ? (
            // The label keeps pointing at the textarea's id, which is what
            // the editor renders it with; the read view beside it is markup,
            // not a control, and is reached by its own text.
            <div className="edit-row-head">
              <label className="muted small" htmlFor="edit-description">{f.label}</label>
              {!editing && <SyntaxToggle value={picked ?? detected} onChange={pickFormat} disabled={!descriptionReady} />}
              <button
                type="button"
                className="btn btn-ghost edit-description-action"
                disabled={!descriptionReady || shut}
                onClick={editing ? cancelEdit : () => setEditingKey(issue.key)}
              >
                {editing ? "Cancel" : "Edit"}
              </button>
            </div>
          ) : (
            <label className="muted small" htmlFor={`edit-${f.id}`}>{f.label}</label>
          )}
          {f.id === "description" ? (
            editing ? (
              <RichTextField
                value={values.description}
                onChange={(v) => set("description", v)}
                format={picked ?? "auto"}
                onFormatChange={pickFormat}
                projectKey={projectKey}
                onOpenLink={onOpenLink}
                onIssueKey={onIssueKey}
                textarea={{ id: "edit-description", className: "detail-input", disabled: !descriptionReady }}
              />
            ) : !descriptionReady ? (
              <p className="muted small">Loading the description</p>
            ) : values.description.trim() === "" ? (
              <p className="muted small">No description.</p>
            ) : (
              <RichText
                text={values.description}
                format={picked ?? "auto"}
                projectKey={projectKey}
                onOpenLink={onOpenLink}
                onIssueKey={onIssueKey}
              />
            )
          ) : f.id === "assignee" ? (
            // The grid holds the display name a sync wrote, but the write
            // path sends {"assignee": {"name": …}}, so what this control
            // stores has to be the username. It seeds the input with the
            // display name it already has, and replaces it the moment a
            // person is picked.
            <AssigneePicker
              profileId={profileId}
              id={`edit-${f.id}`}
              value={values.assignee}
              fallbackLabel={issue.assignee}
              onChange={(v) => set("assignee", v)}
              disabled={busy || shut}
            />
          ) : f.id === "priority" ? (
            <PriorityPicker
              profileId={profileId}
              id={`edit-${f.id}`}
              value={values.priority}
              onChange={(v) => set("priority", v)}
              disabled={busy || shut}
              emptyLabel="(none)"
            />
          ) : f.id === "parentKey" ? (
            // Gated on isLoading, not isFetching: the epic list is stable, so a
            // background refetch should leave the select showing its current
            // options rather than blanking the control mid-edit.
            epics.isLoading ? (
              <select id={`edit-${f.id}`} className="detail-input" disabled value="">
                <option value="">(loading)</option>
              </select>
            ) : (
              <select
                id={`edit-${f.id}`}
                className="detail-input"
                value={values.parentKey}
                disabled={shut}
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
              id={`edit-${f.id}`}
              className="detail-input"
              type="text"
              inputMode={f.id === "storyPoints" ? "decimal" : undefined}
              value={values[f.id]}
              disabled={shut}
              onChange={(e) => set(f.id, e.target.value)}
            />
          )}
          {off && (
            <p className="muted small">
              {`${f.label} is not on this issue's edit screen in Jira.`}
            </p>
          )}
        </div>
        );
      })}
      {anyOffScreen && (
        <p className="muted small">
          A Jira administrator has to add a field to the edit screen before TAM can change it here.
        </p>
      )}
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
