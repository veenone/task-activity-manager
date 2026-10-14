import { useEffect, useState } from "react";
import { EditTestStepField, DeleteTestStep, AddTestStep, errMsg } from "../../api";
import type { PendingChange, Step } from "../../api";
import { MarkdownField } from "../MarkdownField";
import { DirtyDot } from "./rows";

type StepField = "action" | "data" | "expected";

interface StepRowProps {
  profileId: string;
  testKey: string;
  step: Step;
  pendingForTest: PendingChange[];
  isFirst: boolean;
  isLast: boolean;
  readOnly?: boolean;
  confirm: (opts: {
    title: string;
    message?: string;
    confirmLabel?: string;
  }) => Promise<boolean>;
  onMove: (dir: "up" | "down") => void;
  onDuplicate: () => void;
  onLocalChange: (field: StepField, value: string) => void;
  onLocalDelete: (xrayId: string) => void;
  onEdited: () => void;
}

// StepRow renders one editable Test Step (FR-2.5). Each field saves on
// blur, mirroring the same pattern used by the Test-level field editors.
// Dirty markers come from the pendingForTest prop — we filter to rows
// belonging to this step's entity_key.
export function StepRow({
  profileId,
  testKey,
  step,
  pendingForTest,
  isFirst,
  isLast,
  readOnly,
  confirm,
  onMove,
  onDuplicate,
  onLocalChange,
  onLocalDelete,
  onEdited,
}: StepRowProps) {
  const [action, setAction] = useState(step.action);
  const [data, setData] = useState(step.data);
  const [expected, setExpected] = useState(step.expected);
  const [saveError, setSaveError] = useState("");

  const entityKey = `${testKey}:${step.xrayId}`;
  // A step that's only queued for creation (not yet in Jira) shows a "new"
  // badge and, on delete, just cancels the queued add rather than scheduling
  // a remote removal.
  const isNew = pendingForTest.some(
    (p) => p.entityType === "test_step_add" && p.entityKey === entityKey,
  );

  async function deleteStep() {
    if (readOnly) return;
    const ok = await confirm({
      title: isNew ? "Discard step" : "Delete step",
      message: isNew
        ? "Discard this new step? It hasn't been sent to Jira yet."
        : "Delete this step? It will be removed from Jira on commit.",
      confirmLabel: isNew ? "Discard" : "Delete",
    });
    if (!ok) return;
    setSaveError("");
    try {
      await DeleteTestStep(profileId, testKey, step.xrayId);
      onLocalDelete(step.xrayId);
      onEdited();
    } catch (e) {
      setSaveError(errMsg(e));
    }
  }

  const isDirty = (field: StepField) =>
    pendingForTest.some(
      (p) =>
        p.entityType === "test_step" &&
        p.entityKey === entityKey &&
        p.field === field,
    );

  async function save(field: StepField, value: string) {
    if (readOnly) return;
    let backendValue: string;
    switch (field) {
      case "action":
        backendValue = step.action;
        break;
      case "data":
        backendValue = step.data;
        break;
      case "expected":
        backendValue = step.expected;
        break;
    }
    if (value === backendValue) return;
    setSaveError("");
    try {
      await EditTestStepField(profileId, testKey, step.xrayId, field, value);
      onLocalChange(field, value);
      onEdited();
    } catch (e) {
      setSaveError(errMsg(e));
    }
  }

  // A "call test" step invokes another test instead of holding manual content;
  // it has no editable action/data/expected, just the called test + controls.
  if (step.calledTestKey) {
    return (
      <li>
        <div className="step-head step-call-head">
          <span className="step-call-label">
            ⮡ Calls <span className="mono">{step.calledTestKey}</span>
            {isNew && <span className="step-new-badge">new</span>}
          </span>
          {!readOnly && (
            <>
              <div className="step-move">
                <button
                  className="btn btn-ghost step-move-btn"
                  onClick={() => onMove("up")}
                  disabled={isFirst}
                  title="Move step up"
                >
                  ▲
                </button>
                <button
                  className="btn btn-ghost step-move-btn"
                  onClick={() => onMove("down")}
                  disabled={isLast}
                  title="Move step down"
                >
                  ▼
                </button>
              </div>
              <button
                className="btn btn-ghost step-duplicate"
                onClick={onDuplicate}
                title="Duplicate this call"
              >
                ⧉
              </button>
              <button
                className="btn btn-ghost step-delete"
                onClick={deleteStep}
                title={isNew ? "Discard this call" : "Delete this call"}
               aria-label={isNew ? "Discard this call" : "Delete this call"}>
                ✕
              </button>
            </>
          )}
        </div>
        {saveError && <div className="error-text step-save-error">{saveError}</div>}
      </li>
    );
  }

  return (
    <li>
      <div className="step-head">
        {readOnly ? (
          <p className="step-edit step-edit-action step-field-static">{action}</p>
        ) : (
          <MarkdownField
            className="step-edit step-edit-action"
            value={action}
            onChange={setAction}
            onCommit={() => save("action", action)}
            rows={2}
            placeholder="(action)"
          />
        )}
        {isNew && <span className="step-new-badge">new</span>}
        {!readOnly && (
          <>
            <div className="step-move">
              <button
                className="btn btn-ghost step-move-btn"
                onClick={() => onMove("up")}
                disabled={isFirst}
                title="Move step up"
              >
                ▲
              </button>
              <button
                className="btn btn-ghost step-move-btn"
                onClick={() => onMove("down")}
                disabled={isLast}
                title="Move step down"
              >
                ▼
              </button>
            </div>
            <button
              className="btn btn-ghost step-duplicate"
              onClick={onDuplicate}
              title="Duplicate this step"
            >
              ⧉
            </button>
            <button
              className="btn btn-ghost step-delete"
              onClick={deleteStep}
              title={isNew ? "Discard this new step" : "Delete this step"}
             aria-label={isNew ? "Discard this new step" : "Delete this step"}>
              ✕
            </button>
          </>
        )}
      </div>
      {isDirty("action") && <DirtyDot />}
      <div className="step-row">
        <span className="step-label">
          Data {isDirty("data") && <DirtyDot />}
        </span>
        {readOnly ? (
          <p className="step-edit step-field-static">{data}</p>
        ) : (
          <MarkdownField
            className="step-edit"
            value={data}
            onChange={setData}
            onCommit={() => save("data", data)}
            multiline={false}
            placeholder="(optional)"
          />
        )}
      </div>
      <div className="step-row">
        <span className="step-label">
          Expected {isDirty("expected") && <DirtyDot />}
        </span>
        {readOnly ? (
          <p className="step-edit step-field-static">{expected}</p>
        ) : (
          <MarkdownField
            className="step-edit"
            value={expected}
            onChange={setExpected}
            onCommit={() => save("expected", expected)}
            rows={2}
            placeholder="(expected result)"
          />
        )}
      </div>
      {saveError && <div className="error-text step-save-error">{saveError}</div>}
    </li>
  );
}

// InlineStepsEditor renders Kiwi's single inline-text step model
// (caps.stepModel "inline-text") in place of the Xray multi-step CRUD list.
// Kiwi has no step objects — flattenSteps (read path) collapses its one
// `text` field to at most one neutral Step — so there is nothing to
// add/delete/reorder, just one multi-line field. Saving calls
// EditTestStepField on that step's id when it already exists; when there is
// no step yet (empty text), it calls AddTestStep to create the first one.
// Either way the commit engine collapses the result back to Kiwi's single
// `text` field (see internal/syncer/commit.go).
export function InlineStepsEditor({
  profileId,
  testKey,
  step,
  readOnly,
  onLocalChange,
  onLocalCreate,
  onEdited,
}: {
  profileId: string;
  testKey: string;
  step: Step | undefined;
  readOnly?: boolean;
  onLocalChange: (xrayId: string, value: string) => void;
  onLocalCreate: (step: Step) => void;
  onEdited: () => void;
}) {
  const [text, setText] = useState(step?.action ?? "");
  const [saveError, setSaveError] = useState("");

  useEffect(() => {
    setText(step?.action ?? "");
  }, [step?.xrayId, step?.action]);

  async function save() {
    if (readOnly) return;
    setSaveError("");
    try {
      if (step) {
        if (text === step.action) return;
        await EditTestStepField(profileId, testKey, step.xrayId, "action", text);
        onLocalChange(step.xrayId, text);
      } else {
        if (!text.trim()) return;
        const s = await AddTestStep(profileId, testKey, text, "", "");
        onLocalCreate(s);
      }
      onEdited();
    } catch (e) {
      setSaveError(errMsg(e));
    }
  }

  return (
    <div className="steps-inline">
      {readOnly ? (
        <p className="detail-input detail-input-static detail-desc-static">
          {text || <span className="muted">No steps defined for this test.</span>}
        </p>
      ) : (
        <MarkdownField
          className="detail-desc-edit"
          value={text}
          onChange={setText}
          onCommit={save}
          rows={8}
          placeholder="Steps for this test."
        />
      )}
      {saveError && <div className="error-text step-save-error">{saveError}</div>}
    </div>
  );
}
