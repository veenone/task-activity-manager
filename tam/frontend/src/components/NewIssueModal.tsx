import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, announce, call, errMsg, useConfirm, useProfile } from "@agile-suite/core";
import { CreateIssue, ISSUE_TYPES } from "../api";
import type { FieldSpec, IssueDraft, IssueType, Profile, Settings } from "../api";
import { useCreateFields } from "../queries/pending";
import { useEpics } from "../queries/tree";
import { useOpenSprints } from "../queries/boards";
import { useSubtaskType } from "../queries/people";
import { AssigneePicker } from "./AssigneePicker";
import { PriorityPicker } from "./PriorityPicker";
import { invalidateWrites } from "../queries/invalidate";

interface Props {
  onClose: () => void;
  onCreated: (key: string) => void;
  // initialType seeds the type select; EpicsView's "+ New epic" opens the
  // dialog with it set to "epic" so drafting an epic is one click away.
  initialType?: IssueType;
  // lockType fixes the draft to initialType and drops the type select. The
  // Epics view's "+ New epic" is a statement, not an opening question: the
  // user already chose, so re-offering the choice was the dialog disagreeing
  // with the button that opened it. Separate from initialType because a
  // seeded type is not always a fixed one.
  lockType?: boolean;
  // parentKey fixes the draft's parent, for a sub-task drafted from the
  // issue it belongs to. Unlike the epic picker this is not a choice: the
  // issue the user was looking at is the parent.
  parentKey?: string;
  // parentSummary names that issue beside its key, so the link the draft is
  // about to make is legible without looking it up.
  parentSummary?: string;
  // initialEpic seeds the Epic picker for a type that has one. Unlike
  // parentKey this is a default the user may change: a draft started while
  // an epic is on screen almost always belongs to it, and starting at
  // "(none)" made every such draft an orphan that had to be reparented
  // afterwards.
  initialEpic?: string;
}

// CREATABLE are the types this app can draft on its own. A sub-task is not
// among them: it cannot exist without a parent, so it is only drafted from an
// issue, through lockType with that issue as the parent.
const CREATABLE: IssueType[] = ["task", "epic", "story", "bug", "requirement"];

// EPIC_SUMMARY_MAX keeps a long epic summary from stretching the select, the
// same cut EditableFields makes.
const EPIC_SUMMARY_MAX = 48;

function epicOptionLabel(key: string, summary: string): string {
  const cut = summary.length > EPIC_SUMMARY_MAX ? `${summary.slice(0, EPIC_SUMMARY_MAX)}…` : summary;
  return `${key} ${cut}`;
}

function typeLabel(type: IssueType): string {
  return ISSUE_TYPES.find((t) => t.id === type)?.label ?? type;
}

// hasParentPicker says whether the type chooses its own parent. A sub-task
// is always drafted from the issue it belongs to, so it never picks one; an
// epic has no parent at all.
function hasParentPicker(type: IssueType): boolean {
  return type !== "epic" && type !== "subtask";
}

// hasSprintPicker says whether the type can belong to a sprint at all. An
// epic cannot: it is not a card a board carries, and Jira has no sprint
// field on it.
function hasSprintPicker(type: IssueType): boolean {
  return type !== "epic";
}

// hasPoints says whether a type carries story points. An epic is measured by
// the sum of its children and a requirement is not estimated at all, so
// neither shows the field, and neither sends a value.
function hasPoints(type: IssueType): boolean {
  return type !== "requirement" && type !== "epic";
}

// MetaField renders one create-meta field by its schema type. A field whose
// create-meta listed allowed values gets a select, and an array field gets a
// multi-select, because a Jira array (components, fix versions, a
// multi-select) takes more than one value; the chosen ids are joined with a
// comma, which is what the Go side splits. Anything else is a text input,
// with a hint saying so where the value is really a list.
function MetaField({
  spec,
  value,
  invalid,
  onChange,
}: {
  spec: FieldSpec;
  value: string;
  invalid: boolean;
  onChange: (v: string) => void;
}) {
  const id = `meta-${spec.id}`;
  const isList = spec.type === "array";
  const shared = {
    id,
    className: "detail-input",
    "aria-required": spec.required || undefined,
    "aria-invalid": invalid || undefined,
  };
  const label = (
    <label className="muted small" htmlFor={id}>
      {spec.name}
      {spec.required && <span className="field-required" aria-hidden="true"> *</span>}
    </label>
  );

  if (spec.allowedValues.length > 0) {
    const selected = value === "" ? [] : value.split(",");
    return (
      <div className="edit-row">
        {label}
        <span className="edit-cell">
          <select
            {...shared}
            multiple={isList}
            size={isList ? Math.min(spec.allowedValues.length, 4) : undefined}
            value={isList ? selected : value}
            onChange={(e) =>
              onChange(
                isList
                  ? Array.from(e.target.selectedOptions, (o) => o.value).join(",")
                  : e.target.value,
              )
            }
          >
            {!isList && <option value="">Select a {spec.name.toLowerCase()}</option>}
            {spec.allowedValues.map((o) => (
              <option key={o.id} value={o.id}>{o.value}</option>
            ))}
          </select>
          {isList && <span className="muted small">Pick one or more.</span>}
        </span>
      </div>
    );
  }

  return (
    <div className="edit-row">
      {label}
      <span className="edit-cell">
        <input
          {...shared}
          type={spec.type === "date" ? "date" : "text"}
          inputMode={spec.type === "number" ? "decimal" : undefined}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
        {isList && <span className="muted small">A comma list.</span>}
      </span>
    </div>
  );
}

// NewIssueModal drafts one issue. Nothing it creates exists in Jira: it
// writes a TAM-NEW-n row into the journal, and Commit is what pushes it. The
// dialog says so in its subtitle rather than only in a footnote, because that
// is the one thing about this app a new user has to understand, and it used to
// be the line the error message replaced.
export function NewIssueModal({
  onClose,
  onCreated,
  initialType = "task",
  lockType = false,
  parentKey: fixedParent = "",
  parentSummary = "",
  initialEpic = "",
}: Props) {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const { confirm } = useConfirm();
  const qc = useQueryClient();
  const [type, setType] = useState<IssueType>(initialType);
  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("");
  const [labels, setLabels] = useState("");
  const [assignee, setAssignee] = useState("");
  const [points, setPoints] = useState("");
  const [parentKey, setParentKey] = useState(fixedParent || initialEpic);
  // "" is the backlog, a destination and not an absence, the same default
  // MoveIssueToSprint's own picker opens on.
  const [sprintId, setSprintId] = useState("");
  const [extra, setExtra] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  // The control the last validation failure belongs to, so the message is
  // anchored to a field instead of only sitting in the footer.
  const [invalidField, setInvalidField] = useState("");
  const [saving, setSaving] = useState(false);
  const meta = useCreateFields(activeId, type);
  const epics = useEpics(activeId);
  const openSprints = useOpenSprints(activeId);
  const subtaskType = useSubtaskType(activeId);
  const specs = meta.data ?? [];
  const summaryRef = useRef<HTMLInputElement>(null);
  const formRef = useRef<HTMLFormElement>(null);

  // Modal focuses the first focusable node, which is its close button; this
  // runs after it (a parent's effect runs after its child's) and moves focus
  // to the field the user actually came here to fill.
  useEffect(() => {
    summaryRef.current?.focus();
  }, []);

  const dirty =
    summary !== "" ||
    description !== "" ||
    priority !== "" ||
    labels !== "" ||
    assignee !== "" ||
    points !== "" ||
    Object.values(extra).some((v) => v !== "");

  // Escape, Cancel, and the overlay all land here, so typed work is never
  // thrown away without being asked about.
  async function requestClose() {
    if (!dirty) {
      onClose();
      return;
    }
    const ok = await confirm({
      title: "Discard this draft?",
      message: "What you have typed here has not been saved. Nothing reaches Jira either way.",
      confirmLabel: "Discard",
      danger: true,
    });
    if (ok) onClose();
  }

  // Changing the type changes which fields Jira requires and which of the
  // form's own apply, so everything type-specific is cleared rather than
  // silently carried into a shape it no longer fits.
  function changeType(next: IssueType) {
    setType(next);
    setExtra({});
    setError("");
    setInvalidField("");
    if (!hasPoints(next)) setPoints("");
    if (next === "epic") {
      setParentKey("");
      setSprintId("");
    }
  }

  function fail(message: string, field: string) {
    setError(message);
    setInvalidField(field);
    formRef.current?.querySelector<HTMLElement>(`#${field}`)?.focus();
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    // disabled={} stops a click and an implicit Enter, but not a programmatic
    // submit, so the in-flight guard is here too.
    if (saving || meta.isPending) return;
    if (summary.trim() === "") {
      fail("Summary cannot be empty.", "new-summary");
      return;
    }
    if (points.trim() !== "" && Number.isNaN(Number(points.trim()))) {
      fail("Story points must be a number.", "new-points");
      return;
    }
    for (const s of specs) {
      const v = (extra[s.id] ?? "").trim();
      if (s.required && !v) {
        fail(`${s.name} is required.`, `meta-${s.id}`);
        return;
      }
      // Jira's own number fields get the same check the built-in story points
      // field gets; inputMode is a keyboard hint, not validation.
      if (v !== "" && s.type === "number" && Number.isNaN(Number(v))) {
        fail(`${s.name} must be a number.`, `meta-${s.id}`);
        return;
      }
    }
    // The name travels with the id: Task 1's create path reads the draft's
    // JSON, and the sprint's name is what the Backlog and the detail panel
    // show before Commit, before there is anything cached to look it up in.
    const chosenSprintName = openSprints.data?.find((s) => String(s.id) === sprintId)?.name ?? "";
    const draft: IssueDraft = {
      type,
      summary: summary.trim(),
      description,
      priority,
      labels: labels.split(",").map((l) => l.trim()).filter(Boolean),
      assignee,
      storyPoints: !hasPoints(type) || points.trim() === "" ? null : Number(points.trim()),
      parentKey: type === "epic" ? "" : parentKey,
      sprintId: type === "epic" ? "" : sprintId,
      sprintName: type === "epic" ? "" : chosenSprintName,
      extra: Object.fromEntries(Object.entries(extra).filter(([, v]) => v.trim() !== "")),
    };
    setError("");
    setInvalidField("");
    setSaving(true);
    try {
      const key = await call(() => CreateIssue(activeId, draft));
      invalidateWrites(qc, activeId);
      // Creating used to close in silence, so the only evidence was a row the
      // active filters might hide. Naming the placeholder key also teaches the
      // TAM-NEW-n model at the one moment it means something.
      announce(`${typeLabel(type)} drafted as ${key}. Commit creates it in Jira.`);
      onCreated(key);
      onClose();
    } catch (err) {
      setError(errMsg(err));
      setInvalidField("");
    } finally {
      setSaving(false);
    }
  }

  const checking = meta.isPending;
  const parentEpics = epics.data ?? [];

  return (
    <Modal
      onClose={() => void requestClose()}
      className="modal pending-modal new-issue-modal"
      labelledBy="new-issue-title"
      closeOnOverlayClick={false}
    >
      <div className="pending-head">
        <div className="new-issue-title">
          <h2 id="new-issue-title">
            New {type === "subtask" ? (subtaskType.data || "sub-task").toLowerCase() : typeLabel(type).toLowerCase()}
          </h2>
          {/* The one thing a new user has to understand, at full strength and
              in a slot no error message can take. */}
          <p className="muted small">
            Drafted locally{activeProfile ? ` in ${activeProfile.projectKey}` : ""}. Commit creates it in Jira.
          </p>
        </div>
        <button type="button" className="btn btn-ghost" onClick={() => void requestClose()} title="Close" aria-label="Close">
          ✕
        </button>
      </div>

      <form id="new-issue-form" ref={formRef} className="bulk-body edit-form" onSubmit={(e) => void onSubmit(e)}>
        {!lockType && (
          <label className="edit-row" htmlFor="new-type">
            <span className="muted small">Type</span>
            <select id="new-type" className="detail-input" value={type} onChange={(e) => changeType(e.target.value as IssueType)}>
              {ISSUE_TYPES.filter((t) => CREATABLE.includes(t.id)).map((t) => (
                <option key={t.id} value={t.id}>{t.label}</option>
              ))}
            </select>
          </label>
        )}
        {type === "subtask" && (
          <div className="edit-row">
            <span className="muted small">Parent</span>
            {/* Stated, not chosen: the issue this was opened from is the
                parent, and it is what the draft carries to Jira's own
                parent field. Naming it here is what makes the link the
                user is about to create visible before they create it. */}
            <span className="new-issue-parent">
              {fixedParent || "none"}
              {parentSummary && <span className="muted small"> {parentSummary}</span>}
            </span>
          </div>
        )}
        <label className="edit-row" htmlFor="new-summary">
          <span className="muted small">
            Summary<span className="field-required" aria-hidden="true"> *</span>
          </span>
          <input
            id="new-summary"
            ref={summaryRef}
            className="detail-input"
            type="text"
            aria-required
            aria-invalid={invalidField === "new-summary" || undefined}
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
          />
        </label>
        {hasParentPicker(type) && (
          <label className="edit-row" htmlFor="new-parent">
            <span className="muted small">Epic</span>
            {/* Gated on isLoading, not isFetching, the same way the detail
                panel's picker is: a background refetch should not blank the
                control mid-edit. */}
            <select
              id="new-parent"
              className="detail-input"
              disabled={epics.isLoading}
              value={parentKey}
              onChange={(e) => setParentKey(e.target.value)}
            >
              <option value="">{epics.isLoading ? "(loading)" : "(none)"}</option>
              {parentEpics.map((epic) => (
                <option key={epic.key} value={epic.key}>{epicOptionLabel(epic.key, epic.summary)}</option>
              ))}
            </select>
          </label>
        )}
        {hasSprintPicker(type) && (
          <label className="edit-row" htmlFor="new-sprint">
            <span className="muted small">Sprint</span>
            <select
              id="new-sprint"
              className="detail-input"
              value={sprintId}
              onChange={(e) => setSprintId(e.target.value)}
            >
              <option value="">The backlog</option>
              {(openSprints.data ?? []).map((s) => (
                <option key={s.id} value={String(s.id)}>{s.name}</option>
              ))}
            </select>
          </label>
        )}
        <label className="edit-row" htmlFor="new-description">
          <span className="muted small">Description</span>
          <textarea id="new-description" className="detail-input" rows={4} value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <div className="edit-row">
          <label className="muted small" htmlFor="new-priority">Priority</label>
          <PriorityPicker
            profileId={activeId}
            id="new-priority"
            value={priority}
            onChange={setPriority}
            emptyLabel="Jira's default"
          />
        </div>
        <div className="edit-row">
          <label className="muted small" htmlFor="new-labels">Labels</label>
          <span className="edit-cell">
            <input id="new-labels" className="detail-input" type="text" value={labels} onChange={(e) => setLabels(e.target.value)} />
            <span className="muted small">A comma list.</span>
          </span>
        </div>
        <div className="edit-row">
          <label className="muted small" htmlFor="new-assignee">Assignee</label>
          <AssigneePicker
            profileId={activeId}
            id="new-assignee"
            value={assignee}
            onChange={setAssignee}
          />
        </div>
        {hasPoints(type) && (
          <label className="edit-row" htmlFor="new-points">
            <span className="muted small">Story points</span>
            <input
              id="new-points"
              className="detail-input"
              type="text"
              inputMode="decimal"
              aria-invalid={invalidField === "new-points" || undefined}
              value={points}
              onChange={(e) => setPoints(e.target.value)}
            />
          </label>
        )}

        {/* Rendered only when it has something in it: an unconditional
            wrapper drew its border-top above the buttons, attached to
            nothing, on every type that needs no extra fields. */}
        {(meta.isError || checking || specs.length > 0) && (
          <div className="meta-fields">
            {meta.isError ? (
              <p className="muted small">
                Jira&apos;s required fields could not be read ({meta.error.message}). The form stays minimal; Jira
                validates the rest on Commit.
              </p>
            ) : checking ? (
              <p className="muted small" role="status">Checking which fields Jira requires.</p>
            ) : (
              <>
                <p className="muted small">Jira requires these for a {typeLabel(type).toLowerCase()}:</p>
                {specs.map((s) => (
                  <MetaField
                    key={s.id}
                    spec={s}
                    value={extra[s.id] ?? ""}
                    invalid={invalidField === `meta-${s.id}`}
                    onChange={(v) => setExtra((cur) => ({ ...cur, [s.id]: v }))}
                  />
                ))}
              </>
            )}
          </div>
        )}
      </form>

      <div className="pending-actions">
        {/* The error never replaces the standing hint now; both have a slot. */}
        <span className="new-issue-status">
          {error && (
            <span className="error-text small" role="alert">{error}</span>
          )}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={() => void requestClose()} disabled={saving}>
            Cancel
          </button>
          {/* Submitting before the required fields are known would skip them
              entirely and surface the failure as a Jira 400 at Commit, so the
              button waits. A failed read is different: drafting anyway is the
              intended degrade. */}
          <button type="submit" form="new-issue-form" className="btn btn-primary" disabled={saving || checking}>
            {saving ? "Creating" : checking ? "Checking Jira" : "Create draft"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
