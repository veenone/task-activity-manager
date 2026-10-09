import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useProfile } from "../contexts/ProfileContext";
import { Modal } from "@agile-suite/core";
import { TokenPicker, validateLabel } from "./TokenPicker";
import { useLabels } from "../queries/app";
import { BulkEditLabels, ListTestLabels, errMsg } from "../api";
import type { BulkEditResult } from "../api";
import { call } from "../lib/apiCall";

interface Props {
  testKeys: string[];
  onComplete: (result: BulkEditResult) => void;
  onCancel: () => void;
}

// countChanged reports how many tests a given add/remove would change, using
// the same rule the backend applies: remove first, then add what is missing.
export function countChanged(
  current: Record<string, string[]>,
  add: string[],
  remove: string[],
): number {
  const drop = new Set(remove);
  let n = 0;
  for (const labels of Object.values(current)) {
    const kept = labels.filter((l) => !drop.has(l));
    const next = [...kept, ...add.filter((l) => !kept.includes(l))];
    if (next.join(" ") !== labels.join(" ")) n++;
  }
  return n;
}

// BulkLabelsModal adds and removes several labels across the selected tests in
// one pass. Each changed test gets one pending labels edit.
export function BulkLabelsModal({ testKeys, onComplete, onCancel }: Props) {
  const { activeId: profileId } = useProfile();
  const [add, setAdd] = useState<string[]>([]);
  const [remove, setRemove] = useState<string[]>([]);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<BulkEditResult | null>(null);

  const { data: buckets = [] } = useLabels(profileId);
  const suggestions = buckets.map((b) => b.label);
  const { data: current } = useQuery({
    queryKey: [profileId, "tests", "labels", testKeys],
    queryFn: () => call(() => ListTestLabels(profileId, testKeys)),
    enabled: !!profileId,
  });

  const changed = useMemo(
    () => (current ? countChanged(current, add, remove) : null),
    [current, add, remove],
  );
  const overlap = add.find((l) => remove.includes(l));

  async function apply() {
    if (overlap) {
      setError(`"${overlap}" is in both Add and Remove.`);
      return;
    }
    if (add.length === 0 && remove.length === 0) {
      setError("Pick at least one label to add or remove.");
      return;
    }
    setApplying(true);
    setError("");
    try {
      const r = await BulkEditLabels(profileId, testKeys, add, remove);
      setResult(r);
      if (r.failed.length === 0) onComplete(r);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setApplying(false);
    }
  }

  const n = testKeys.length;
  return (
    <Modal onClose={onCancel} className="modal bulk-modal" labelledBy="bulk-labels-title">
      <div className="pending-head">
        <h2 id="bulk-labels-title">
          Labels ({n} {n === 1 ? "test" : "tests"})
        </h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close">
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <div className="bulk-row">
          <span>Add</span>
          <TokenPicker
            label="Add labels"
            value={add}
            onChange={setAdd}
            suggestions={suggestions}
            allowCreate
            validate={validateLabel}
            placeholder="Type to search or create"
          />
        </div>
        <div className="bulk-row">
          <span>Remove</span>
          <TokenPicker
            label="Remove labels"
            value={remove}
            onChange={setRemove}
            suggestions={suggestions}
            allowCreate={false}
            validate={validateLabel}
            placeholder="Type to search"
          />
        </div>
        {changed !== null && (
          <p className="muted bulk-preview">
            {changed} of {n} selected tests will change.
          </p>
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
        <button className="btn" onClick={onCancel} disabled={applying}>
          Cancel
        </button>
        <button className="btn btn-primary" onClick={apply} disabled={applying}>
          {applying ? "Applying…" : "Apply"}
        </button>
      </div>
    </Modal>
  );
}
