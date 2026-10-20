import { useMemo, useState } from "react";
import { useProfile } from "../contexts/ProfileContext";
import { Modal } from "@agile-suite/core";
import { ScanTestStepText, ReplaceTestStepText, errMsg } from "../api";
import type {
  BulkEditResult,
  StepTextMatch,
  StepTextQuery,
  StepTextScan,
} from "../api";
import { call } from "../lib/apiCall";

interface Props {
  testKeys: string[];
  onComplete: (result: BulkEditResult) => void;
  onCancel: () => void;
}

// The step fields a search can read, with the label each one carries in the
// match list.
const STEP_FIELDS: { id: string; label: string }[] = [
  { id: "action", label: "Action" },
  { id: "data", label: "Data" },
  { id: "expected", label: "Expected" },
];

// matchId identifies one match across a re-render and in the exclude set.
export function matchId(m: StepTextMatch): string {
  return `${m.testKey}:${m.xrayId}:${m.field}`;
}

// fieldLabel is how a match row names the field it rewrites.
export function fieldLabel(field: string): string {
  return STEP_FIELDS.find((f) => f.id === field)?.label ?? field;
}

// BulkStepReplaceModal finds a string in the steps of the selected tests and
// replaces it across them (#311). The scan writes nothing; applying queues one
// pending step edit per kept match, so the result lands in Pending changes
// like any hand-edited step.
export function BulkStepReplaceModal({ testKeys, onComplete, onCancel }: Props) {
  const { activeId: profileId } = useProfile();
  const [find, setFind] = useState("");
  const [replace, setReplace] = useState("");
  const [fields, setFields] = useState<string[]>(STEP_FIELDS.map((f) => f.id));
  const [matchCase, setMatchCase] = useState(false);
  const [wholeWord, setWholeWord] = useState(false);
  const [scan, setScan] = useState<StepTextScan | null>(null);
  const [excluded, setExcluded] = useState<Set<string>>(new Set());
  const [scanning, setScanning] = useState(false);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<BulkEditResult | null>(null);

  const query: StepTextQuery = { find, replace, fields, matchCase, wholeWord };

  // Any change to the query invalidates the preview: the backend recomputes
  // each replacement from the query it is given, so applying against a stale
  // match list would write text the user never saw.
  function edited<T>(set: (v: T) => void): (v: T) => void {
    return (v: T) => {
      set(v);
      setScan(null);
      setExcluded(new Set());
      setResult(null);
    };
  }

  const kept = useMemo(
    () => (scan?.matches ?? []).filter((m) => !excluded.has(matchId(m))),
    [scan, excluded],
  );

  function toggleField(id: string) {
    setScan(null);
    setExcluded(new Set());
    setFields((current) =>
      current.includes(id)
        ? current.filter((f) => f !== id)
        : STEP_FIELDS.map((f) => f.id).filter((f) => current.includes(f) || f === id),
    );
  }

  function toggleMatch(id: string) {
    setExcluded((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function runScan() {
    if (find.trim() === "") {
      setError("Type the text to find.");
      return;
    }
    if (fields.length === 0) {
      setError("Pick at least one step field to search.");
      return;
    }
    setScanning(true);
    setError("");
    try {
      const found = await call(() => ScanTestStepText(profileId, testKeys, query));
      setScan(found);
      setExcluded(new Set());
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setScanning(false);
    }
  }

  async function apply() {
    if (kept.length === 0) {
      setError("No matches are selected.");
      return;
    }
    setApplying(true);
    setError("");
    try {
      const r = await call(() =>
        ReplaceTestStepText(
          profileId,
          kept.map((m) => ({ testKey: m.testKey, xrayId: m.xrayId, field: m.field })),
          query,
        ),
      );
      setResult(r);
      if (r.failed.length === 0) onComplete(r);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setApplying(false);
    }
  }

  const n = testKeys.length;
  const matchedTests = new Set(kept.map((m) => m.testKey)).size;
  return (
    <Modal
      onClose={onCancel}
      className="modal bulk-modal"
      labelledBy="bulk-step-replace-title"
    >
      <div className="pending-head">
        <h2 id="bulk-step-replace-title">
          Replace step text ({n} {n === 1 ? "test" : "tests"})
        </h2>
        <button
          className="btn btn-ghost"
          onClick={onCancel}
          title="Close"
          aria-label="Close"
        >
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <div className="bulk-row">
          <span>Find</span>
          <input
            aria-label="Find"
            value={find}
            onChange={(e) => edited(setFind)(e.target.value)}
            placeholder="Text to find in the steps"
          />
        </div>
        <div className="bulk-row">
          <span>Replace with</span>
          <input
            aria-label="Replace with"
            value={replace}
            onChange={(e) => edited(setReplace)(e.target.value)}
            placeholder="Leave empty to delete the text"
          />
        </div>
        <div className="bulk-row">
          <span>Search in</span>
          <div className="step-replace-opts">
            {STEP_FIELDS.map((f) => (
              <label key={f.id}>
                <input
                  type="checkbox"
                  checked={fields.includes(f.id)}
                  onChange={() => toggleField(f.id)}
                />
                <span>{f.label}</span>
              </label>
            ))}
          </div>
        </div>
        <div className="bulk-row">
          <span>Options</span>
          <div className="step-replace-opts">
            <label>
              <input
                type="checkbox"
                checked={matchCase}
                onChange={() => edited(setMatchCase)(!matchCase)}
              />
              <span>Match case</span>
            </label>
            <label>
              <input
                type="checkbox"
                checked={wholeWord}
                onChange={() => edited(setWholeWord)(!wholeWord)}
              />
              <span>Whole word</span>
            </label>
          </div>
        </div>

        {scan === null ? (
          <p className="muted bulk-preview">
            Find the matches first. Nothing is changed until you apply.
          </p>
        ) : scan.matches.length === 0 ? (
          <p className="muted bulk-preview">
            No step text in the {n} selected {n === 1 ? "test" : "tests"} matches.
          </p>
        ) : (
          <>
            <p className="muted bulk-preview">
              {kept.length} of {scan.matches.length}{" "}
              {scan.matches.length === 1 ? "match" : "matches"} selected, across{" "}
              {matchedTests} {matchedTests === 1 ? "test" : "tests"}.
            </p>
            <ul className="step-replace-list">
              {scan.matches.map((m) => {
                const id = matchId(m);
                return (
                  <li key={id} className="step-replace-row">
                    <label>
                      <input
                        type="checkbox"
                        checked={!excluded.has(id)}
                        onChange={() => toggleMatch(id)}
                      />
                      <span className="step-replace-where mono">
                        {m.testKey} step {m.index} {fieldLabel(m.field)}
                      </span>
                    </label>
                    <span className="step-replace-before">{m.before}</span>
                    <span className="step-replace-arrow">{"→"}</span>
                    <span className="step-replace-after">{m.after}</span>
                  </li>
                );
              })}
            </ul>
            {scan.skipped.length > 0 && (
              <p className="muted bulk-preview">
                {scan.skipped.length}{" "}
                {scan.skipped.length === 1 ? "test has" : "tests have"} no match
                and will not change.
              </p>
            )}
            {scan.failed.length > 0 && (
              <div className="error-text">
                <ul className="commit-fail-list">
                  {scan.failed.map((f) => (
                    <li key={f.testKey}>
                      <span className="mono">{f.testKey}</span>: {f.error}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}

        {error && <div className="error-text">{error}</div>}
        {result && result.failed.length > 0 && (
          <div className="error-text">
            <ul className="commit-fail-list">
              {result.failed.map((f) => (
                <li key={f.testKey}>
                  <span className="mono">{f.testKey}</span>: {f.error}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
      <div className="pending-actions">
        <button className="btn" onClick={onCancel} disabled={scanning || applying}>
          Cancel
        </button>
        <button className="btn" onClick={runScan} disabled={scanning || applying}>
          {scanning ? "Finding…" : "Find matches"}
        </button>
        <button
          className="btn btn-primary"
          onClick={apply}
          disabled={scanning || applying || kept.length === 0}
        >
          {applying ? "Replacing…" : `Replace ${kept.length}`}
        </button>
      </div>
    </Modal>
  );
}
