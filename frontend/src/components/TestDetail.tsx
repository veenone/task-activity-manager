import { useEffect, useState } from "react";
import {
  GetTest,
  GetTestPreconditions,
  ListAllPreconditions,
  SetTestPreconditions,
  GetTestContainers,
  GetTestTransitions,
  GetTestSteps,
  TransitionTest,
  EditTestField,
  EditTestStepField,
  DeleteTestStep,
  AddTestStep,
  ReorderTestSteps,
  MoveTestToFolder,
  errMsg,
} from "../api";
import type {
  TestCase,
  Precondition,
  ContainerMembership,
  PendingChange,
  Transition,
  Step,
  Folder,
} from "../api";

interface Props {
  profileId: string;
  testKey: string;
  version: number;
  pendingForTest: PendingChange[];
  folders: Folder[];
  onClose: () => void;
  onEdited: () => void;
}

type EditableField = "summary" | "description" | "priority" | "labels";

export function TestDetail({
  profileId,
  testKey,
  version,
  pendingForTest,
  folders,
  onClose,
  onEdited,
}: Props) {
  const [test, setTest] = useState<TestCase | null>(null);
  const [preconditions, setPreconditions] = useState<Precondition[]>([]);
  const [allPreconditions, setAllPreconditions] = useState<Precondition[]>([]);
  const [containers, setContainers] = useState<ContainerMembership[]>([]);
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
      GetTestContainers(profileId, testKey),
      ListAllPreconditions(profileId),
    ])
      .then(([t, pre, cons, allPre]) => {
        if (cancelled) return;
        setTest(t);
        setSummary(t.summary);
        setDescription(t.description);
        setPriority(t.priority);
        setLabels((t.labels ?? []).join(" "));
        setPreconditions(pre);
        setContainers(cons ?? []);
        setAllPreconditions(allPre ?? []);
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

  // moveToFolder relocates the test in the Test Repository (FR-13.3). The new
  // folder is reflected locally so the next diff works, then queued for commit.
  async function moveToFolder(folderId: string) {
    if (!test || folderId === test.folderId) return;
    setSaveError("");
    try {
      await MoveTestToFolder(profileId, testKey, folderId);
      setTest({ ...test, folderId });
      onEdited();
    } catch (e) {
      setSaveError(`Move failed: ${errMsg(e)}`);
    }
  }

  // applyPreconditions replaces the test's precondition set, then refreshes the
  // displayed list from the store (FR-13.5). Add/remove both route here.
  async function applyPreconditions(nextKeys: string[]) {
    setSaveError("");
    try {
      await SetTestPreconditions(profileId, testKey, nextKeys);
      const refreshed = await GetTestPreconditions(profileId, testKey);
      setPreconditions(refreshed ?? []);
      onEdited();
    } catch (e) {
      setSaveError(`Precondition update failed: ${errMsg(e)}`);
    }
  }

  function removePrecondition(key: string) {
    applyPreconditions(
      preconditions.map((p) => p.key).filter((k) => k !== key),
    );
  }

  function addPrecondition(key: string) {
    if (!key || preconditions.some((p) => p.key === key)) return;
    applyPreconditions([...preconditions.map((p) => p.key), key]);
  }

  // addStep appends a new, empty step locally (FR-2.5). The backend returns
  // it with a temporary id; the user fills the fields in place (each blur
  // folds into the queued create) and it lands in Jira on commit.
  async function addStep() {
    setSaveError("");
    try {
      const s = await AddTestStep(profileId, testKey, "", "", "");
      setSteps((prev) => [...prev, s]);
      onEdited();
    } catch (e) {
      setSaveError(`Add step failed: ${errMsg(e)}`);
    }
  }

  // moveStep swaps a step with its neighbour and persists the whole new order
  // (FR-2.5). The reorder is a single test-level pending change; on failure we
  // roll the local list back so the UI matches what was actually saved.
  async function moveStep(index: number, dir: "up" | "down") {
    const target = dir === "up" ? index - 1 : index + 1;
    if (target < 0 || target >= steps.length) return;
    const previous = steps;
    const next = [...steps];
    [next[index], next[target]] = [next[target], next[index]];
    setSteps(next);
    setSaveError("");
    try {
      await ReorderTestSteps(
        profileId,
        testKey,
        next.map((s) => s.xrayId),
      );
      onEdited();
    } catch (e) {
      setSteps(previous);
      setSaveError(`Reorder failed: ${errMsg(e)}`);
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

            {folders.length > 0 && (
              <>
                <dt>
                  Folder {isDirty("folder") && <DirtyDot />}
                </dt>
                <dd>
                  <select
                    className="detail-input detail-input-inline"
                    value={test.folderId}
                    onChange={(e) => moveToFolder(e.target.value)}
                  >
                    <option value="">(repository root)</option>
                    {folders.map((f) => (
                      <option key={f.id} value={f.id}>
                        {f.id}
                      </option>
                    ))}
                  </select>
                </dd>
              </>
            )}

            <dt>Updated</dt>
            <dd>{test.updated || "—"}</dd>
          </dl>

          <h4>
            Preconditions {isDirty("preconditions") && <DirtyDot />}
          </h4>
          {preconditions.length === 0 ? (
            <p className="muted">None linked</p>
          ) : (
            <ul className="pre-list">
              {preconditions.map((p) => (
                <li key={p.key}>
                  <span className="mono">{p.key}</span> — {p.summary}
                  <button
                    className="btn btn-ghost pre-remove"
                    onClick={() => removePrecondition(p.key)}
                    title="Remove this precondition"
                  >
                    ✕
                  </button>
                </li>
              ))}
            </ul>
          )}
          {allPreconditions.some(
            (p) => !preconditions.some((lp) => lp.key === p.key),
          ) && (
            <select
              className="detail-input detail-input-inline pre-add"
              value=""
              onChange={(e) => {
                if (e.target.value) addPrecondition(e.target.value);
              }}
            >
              <option value="">+ Add precondition…</option>
              {allPreconditions
                .filter((p) => !preconditions.some((lp) => lp.key === p.key))
                .map((p) => (
                  <option key={p.key} value={p.key}>
                    {p.key} — {p.summary}
                  </option>
                ))}
            </select>
          )}

          <ContainerSection
            title="Test Sets"
            items={containers.filter((c) => c.kind === "testset")}
          />
          <ContainerSection
            title="Test Plans"
            items={containers.filter((c) => c.kind === "testplan")}
          />
          <ContainerSection
            title="Test Executions"
            items={containers.filter((c) => c.kind === "testexec")}
            showRunStatus
          />

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
            {pendingForTest.some((p) => p.entityType === "test_step_order") && (
              <span className="steps-reordered" title="Step order changed">
                reordered
              </span>
            )}
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
              {steps.map((s, i) => (
                <StepRow
                  key={s.xrayId}
                  profileId={profileId}
                  testKey={testKey}
                  step={s}
                  pendingForTest={pendingForTest}
                  isFirst={i === 0}
                  isLast={i === steps.length - 1}
                  onMove={(dir) => moveStep(i, dir)}
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
          {!stepsError && !stepsLoading && (
            <button className="link-btn steps-add" onClick={addStep}>
              + Add step
            </button>
          )}

          <p className="muted detail-note">
            Edits are saved locally and queued in <b>Pending</b> until you
            commit them to Jira. Reordering steps lands in a later update.
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
  isFirst: boolean;
  isLast: boolean;
  onMove: (dir: "up" | "down") => void;
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
  isFirst,
  isLast,
  onMove,
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
    const prompt = isNew
      ? "Discard this new step? It hasn't been sent to Jira yet."
      : "Delete this step? It will be removed from Jira on commit.";
    if (!window.confirm(prompt)) {
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
        {isNew && <span className="step-new-badge">new</span>}
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
          className="btn btn-ghost step-delete"
          onClick={deleteStep}
          title={isNew ? "Discard this new step" : "Delete this step"}
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

// ContainerSection lists the Test Sets / Plans / Executions a Test belongs to
// (FR-1.3). Execution memberships also show the Test Run status badge.
function ContainerSection({
  title,
  items,
  showRunStatus,
}: {
  title: string;
  items: ContainerMembership[];
  showRunStatus?: boolean;
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
function RunStatusBadge({ status }: { status: string }) {
  return (
    <span className={`run-badge run-${status.toLowerCase()}`}>{status}</span>
  );
}

function DirtyDot() {
  return (
    <span className="dirty-dot" title="Pending edit">
      ●
    </span>
  );
}
