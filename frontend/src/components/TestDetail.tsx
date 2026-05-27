import { useEffect, useState } from "react";
import {
  GetTest,
  GetTestPreconditions,
  GetTestTransitions,
  TransitionTest,
  EditTestField,
  errMsg,
} from "../api";
import type {
  TestCase,
  Precondition,
  PendingChange,
  Transition,
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

          <p className="muted detail-note">
            Edits are saved locally and queued in <b>Pending</b> until you
            commit them to Jira. Test steps load in a later update.
          </p>
        </div>
      )}
    </aside>
  );
}

function DirtyDot() {
  return (
    <span className="dirty-dot" title="Pending edit">
      ●
    </span>
  );
}
