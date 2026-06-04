import { useEffect, useState } from "react";
import {
  GetTest,
  GetTestPreconditions,
  GetTestTransitions,
  GetTestSteps,
  TransitionTest,
  EditTestField,
  EditTestStepField,
  DeleteTestStep,
  errMsg,
} from "../api";
import type {
  TestCase,
  Precondition,
  PendingChange,
  Transition,
  Step,
} from "../api";

interface Props {
  profileId: string;
  testKey: string;
  version: number;
  pendingForTest: PendingChange[];
  onClose: () => void;
  onEdited: () => void;
}

type EditableField = "summary" | "description" | "priority" | "labels";

export function TestDetail({
  profileId,
  testKey,
  version,
  pendingForTest,
  onClose,
  onEdited,
}: Props) {
  const [test, setTest] = useState<TestCase | null>(null);
  const [preconditions, setPreconditions] = useState<Precondition[]>([]);
  const [transitions, setTransitions] = useState<Transition[]>([]);
  const [steps, setSteps] = useState<Step[]>([]);
  const [stepsLoading, setStepsLoading] = useState(false);
  const [stepsError, setStepsError] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [saveError, setSaveError] = useState("");

  // Local editable state — initialised from the loaded Test, then driven by
  // the user. Each blur compares against `test` (the last persisted value)
  // and saves only on a real change.
  const [summary, setSummary] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("");
  const [labels, setLabels] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    setSaveError("");
    Promise.all([
      GetTest(profileId, testKey),
      GetTestPreconditions(profileId, testKey),
    ])
      .then(([t, pre]) => {
        if (cancelled) return;
        setTest(t);
        setSummary(t.summary);
        setDescription(t.description);
        setPriority(t.priority);
        setLabels((t.labels ?? []).join(" "));
        setPreconditions(pre);
        // Transitions load alongside but can fail without blocking the
        // rest of the detail panel — workflow may not be set up yet, or
        // the user may not have edit permission.
        GetTestTransitions(profileId, testKey)
          .then((ts) => {
            if (!cancelled) setTransitions(ts ?? []);
          })
          .catch((e) => {
            if (!cancelled) console.error("transitions:", errMsg(e));
          });
        // Steps load lazily: cache hit is instant, cache miss makes one
        // Xray call. Failure renders inline next to the Steps heading
        // rather than blocking the whole panel.
        setStepsLoading(true);
        setStepsError("");
        GetTestSteps(profileId, testKey, false)
          .then((s) => {
            if (!cancelled) setSteps(s ?? []);
          })
          .catch((e) => {
            if (!cancelled) setStepsError(errMsg(e));
          })
          .finally(() => {
            if (!cancelled) setStepsLoading(false);
          });
      })
      .catch((e) => {
        if (!cancelled) setError(errMsg(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, testKey, version]);

  async function saveField(field: EditableField, value: string) {
    if (!test) return;

    let backendValue: string;
    switch (field) {
      case "summary":
        backendValue = test.summary;
        break;
      case "description":
        backendValue = test.description;
        break;
      case "priority":
        backendValue = test.priority;
        break;
      case "labels":
        backendValue = (test.labels ?? []).join(" ");
        break;
    }
    if (value === backendValue) return;

    setSaveError("");
    try {
      await EditTestField(profileId, testKey, field, value);
      // Reflect the new persisted value locally so subsequent diffs work.
      const updated: TestCase = { ...test };
      switch (field) {
        case "summary":
          updated.summary = value;
          break;
        case "description":
          updated.description = value;
          break;
        case "priority":
          updated.priority = value;
          break;
        case "labels":
          updated.labels = value.split(/\s+/).filter(Boolean);
          break;
      }
      setTest(updated);
      onEdited();
    } catch (e) {
      setSaveError(`Save failed: ${errMsg(e)}`);
    }
  }

  // refreshSteps re-fetches Steps from Xray, bypassing the cache. Useful
  // after someone else edits steps directly in Jira.
  async function refreshSteps() {
    setStepsLoading(true);
    setStepsError("");
    try {
      const s = await GetTestSteps(profileId, testKey, true);
      setSteps(s ?? []);
    } catch (e) {
      setStepsError(errMsg(e));
    } finally {
      setStepsLoading(false);
    }
  }

  // applyTransition records the workflow transition locally (FR-4.2). After
  // the local write, we re-query for the transitions available from the new
  // status so the next pick reflects the post-transition workflow position.
  async function applyTransition(targetStatus: string) {
    if (!test || !targetStatus) return;
    setSaveError("");
    try {
      await TransitionTest(profileId, testKey, targetStatus);
      setTest({ ...test, status: targetStatus });
      try {
        const ts = await GetTestTransitions(profileId, testKey);
        setTransitions(ts ?? []);
      } catch (e) {
        console.error("re-fetch transitions:", errMsg(e));
      }
      onEdited();
    } catch (e) {
      setSaveError(`Transition failed: ${errMsg(e)}`);
    }
  }

  const isDirty = (field: string) =>
    pendingForTest.some((p) => p.field === field);

  return (
    <aside className="detail">
      <div className="detail-head">
        <span className="mono detail-key">{testKey}</span>
        <button className="btn btn-ghost" onClick={onClose} title="Close">
          ✕
        </button>
      </div>

      {loading && <div className="muted detail-body">Loading…</div>}
      {error && <div className="error-text detail-body">{error}</div>}

      {test && !loading && (
        <div className="detail-body">
          {saveError && (
            <div className="error-text detail-save-error">{saveError}</div>
          )}

          <div className="field-label">
            Summary {isDirty("summary") && <DirtyDot />}
          </div>
          <input
            className="detail-input"
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            onBlur={() => saveField("summary", summary)}
          />

          <dl className="detail-fields">
            <dt>
              Status {isDirty("status") && <DirtyDot />}
            </dt>
            <dd>
              <div className="status-row">
                <span className="status-pill">{test.status || "—"}</span>
                {transitions.length > 0 && (
                  <select
                    className="transition-select"
                    value=""
                    onChange={(e) => {
                      if (e.target.value) applyTransition(e.target.value);
                    }}
                  >
                    <option value="">Move to…</option>
                    {transitions.map((t) => (
                      <option key={t.id} value={t.to}>
                        {t.name} → {t.to}
                      </option>
                    ))}
                  </select>
                )}
              </div>
            </dd>

            <dt>
              Priority {isDirty("priority") && <DirtyDot />}
            </dt>
            <dd>
              <input
                className="detail-input detail-input-inline"
                value={priority}
                onChange={(e) => setPriority(e.target.value)}
                onBlur={() => saveField("priority", priority)}
              />
            </dd>

            <dt>
              Labels {isDirty("labels") && <DirtyDot />}
            </dt>
            <dd>
              <input
                className="detail-input detail-input-inline"
                value={labels}
                onChange={(e) => setLabels(e.target.value)}
                onBlur={() => saveField("labels", labels)}
                placeholder="space-separated"
              />
            </dd>

            <dt>Updated</dt>
            <dd>{test.updated || "—"}</dd>
          </dl>

          <h4>Preconditions</h4>
          {preconditions.length === 0 ? (
            <p className="muted">None linked</p>
          ) : (
            <ul className="pre-list">
              {preconditions.map((p) => (
                <li key={p.key}>
                  <span className="mono">{p.key}</span> — {p.summary}
                </li>
              ))}
            </ul>
          )}

          <h4>
            Description {isDirty("description") && <DirtyDot />}
          </h4>
          <textarea
            className="detail-desc-edit"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            onBlur={() => saveField("description", description)}
            rows={8}
          />

          <h4 className="steps-head">
            Steps
            <button
              className="link-btn steps-refresh"
              onClick={refreshSteps}
              disabled={stepsLoading}
              title="Re-fetch steps from Jira"
            >
              {stepsLoading ? "Loading…" : "Refresh"}
            </button>
          </h4>
          {stepsError && <div className="error-text">{stepsError}</div>}
          {!stepsError && !stepsLoading && steps.length === 0 && (
            <p className="muted">No steps defined for this test.</p>
          )}
          {steps.length > 0 && (
            <ol className="steps-list">
              {steps.map((s) => (
                <StepRow
                  key={s.xrayId}
                  profileId={profileId}
                  testKey={testKey}
                  step={s}
                  pendingForTest={pendingForTest}
                  onLocalChange={(field, value) => {
                    setSteps((prev) =>
                      prev.map((p) =>
                        p.xrayId === s.xrayId ? { ...p, [field]: value } : p,
                      ),
                    );
                  }}
                  onLocalDelete={(xrayId) => {
                    setSteps((prev) => prev.filter((p) => p.xrayId !== xrayId));
                  }}
                  onEdited={onEdited}
                />
              ))}
            </ol>
          )}

          <p className="muted detail-note">
            Edits are saved locally and queued in <b>Pending</b> until you
            commit them to Jira. Add / remove / reorder steps lands in a
            later update.
          </p>
        </div>
      )}
    </aside>
  );
}

type StepField = "action" | "data" | "expected";

interface StepRowProps {
  profileId: string;
  testKey: string;
  step: Step;
  pendingForTest: PendingChange[];
  onLocalChange: (field: StepField, value: string) => void;
  onLocalDelete: (xrayId: string) => void;
  onEdited: () => void;
}

// StepRow renders one editable Test Step (FR-2.5). Each field saves on
// blur, mirroring the same pattern used by the Test-level field editors.
// Dirty markers come from the pendingForTest prop — we filter to rows
// belonging to this step's entity_key.
function StepRow({
  profileId,
  testKey,
  step,
  pendingForTest,
  onLocalChange,
  onLocalDelete,
  onEdited,
}: StepRowProps) {
  const [action, setAction] = useState(step.action);
  const [data, setData] = useState(step.data);
  const [expected, setExpected] = useState(step.expected);
  const [saveError, setSaveError] = useState("");

  async function deleteStep() {
    if (!window.confirm(`Delete this step? It will be removed from Jira on commit.`)) {
      return;
    }
    setSaveError("");
    try {
      await DeleteTestStep(profileId, testKey, step.xrayId);
      onLocalDelete(step.xrayId);
      onEdited();
    } catch (e) {
      setSaveError(errMsg(e));
    }
  }

  const entityKey = `${testKey}:${step.xrayId}`;
  const isDirty = (field: StepField) =>
    pendingForTest.some(
      (p) =>
        p.entityType === "test_step" &&
        p.entityKey === entityKey &&
        p.field === field,
    );

  async function save(field: StepField, value: string) {
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

  return (
    <li>
      <div className="step-head">
        <textarea
          className="step-edit step-edit-action"
          value={action}
          onChange={(e) => setAction(e.target.value)}
          onBlur={() => save("action", action)}
          rows={2}
          placeholder="(action)"
        />
        <button
          className="btn btn-ghost step-delete"
          onClick={deleteStep}
          title="Delete this step"
        >
          ✕
        </button>
      </div>
      {isDirty("action") && <DirtyDot />}
      <div className="step-row">
        <span className="step-label">
          Data {isDirty("data") && <DirtyDot />}
        </span>
        <input
          className="step-edit"
          value={data}
          onChange={(e) => setData(e.target.value)}
          onBlur={() => save("data", data)}
          placeholder="(optional)"
        />
      </div>
      <div className="step-row">
        <span className="step-label">
          Expected {isDirty("expected") && <DirtyDot />}
        </span>
        <textarea
          className="step-edit"
          value={expected}
          onChange={(e) => setExpected(e.target.value)}
          onBlur={() => save("expected", expected)}
          rows={2}
          placeholder="(expected result)"
        />
      </div>
      {saveError && <div className="error-text step-save-error">{saveError}</div>}
    </li>
  );
}

function DirtyDot() {
  return (
    <span className="dirty-dot" title="Pending edit">
      ●
    </span>
  );
}
