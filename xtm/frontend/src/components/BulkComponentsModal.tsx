import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Modal } from "@agile-suite/core";
import { useProfile } from "../contexts/ProfileContext";
import { TokenPicker } from "./TokenPicker";
import { countChanged } from "./BulkLabelsModal";
import { useComponentOptions } from "../queries/components";
import { validateComponentName } from "../lib/components";
import { useCreateComponentFromPicker } from "./components-admin/useCreateComponentFromPicker";
import { BulkEditComponents, ListTestComponents, errMsg } from "../api";
import type { BulkEditResult } from "../api";
import { call } from "../lib/apiCall";

interface Props {
  testKeys: string[];
  onComplete: (result: BulkEditResult) => void;
  onCancel: () => void;
}

// BulkComponentsModal adds and removes components across the selection, or
// replaces them outright. Each changed test gets one pending edit.
export function BulkComponentsModal({ testKeys, onComplete, onCancel }: Props) {
  const { activeId: profileId, activeProfile } = useProfile();
  const projectKey = activeProfile?.projectKey ?? "";
  const [mode, setMode] = useState<"edit" | "replace">("edit");
  const [add, setAdd] = useState<string[]>([]);
  const [remove, setRemove] = useState<string[]>([]);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<BulkEditResult | null>(null);

  const options = useComponentOptions(profileId, projectKey);
  const create = useCreateComponentFromPicker(profileId, projectKey);
  const { data: current } = useQuery({
    queryKey: [profileId, "tests", "components", testKeys],
    queryFn: () => call(() => ListTestComponents(profileId, testKeys)),
    enabled: !!profileId,
  });

  const replace = mode === "replace";
  const changed = useMemo(() => {
    if (!current) return null;
    if (!replace) return countChanged(current, add, remove);
    return Object.values(current).filter((c) => c.join("\n") !== add.join("\n")).length;
  }, [current, add, remove, replace]);

  async function apply() {
    const overlap = add.find((n) => remove.includes(n));
    if (!replace && overlap) {
      setError(`"${overlap}" is in both Add and Remove.`);
      return;
    }
    if (!replace && add.length === 0 && remove.length === 0) {
      setError("Pick at least one component to add or remove.");
      return;
    }
    setApplying(true);
    setError("");
    try {
      const r = await BulkEditComponents(profileId, testKeys, add, replace ? [] : remove, replace);
      setResult(r);
      if (r.failed.length === 0) onComplete(r);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setApplying(false);
    }
  }

  const n = testKeys.length;
  const suggestions = options.data ?? [];
  return (
    <Modal onClose={onCancel} className="modal bulk-modal" labelledBy="bulk-components-title">
      <div className="pending-head">
        <h2 id="bulk-components-title">
          Components ({n} {n === 1 ? "test" : "tests"})
        </h2>
        <button className="btn btn-ghost" onClick={onCancel} title="Close">
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <fieldset className="bulk-row bulk-row-radios">
          <legend className="sr-only">Mode</legend>
          <label>
            <input type="radio" checked={!replace} onChange={() => setMode("edit")} /> Add and remove
          </label>
          <label>
            <input type="radio" checked={replace} onChange={() => setMode("replace")} /> Replace all
          </label>
        </fieldset>
        <div className="bulk-row">
          <span>{replace ? "Set to" : "Add"}</span>
          <TokenPicker
            label="Add components"
            value={add}
            onChange={setAdd}
            suggestions={suggestions}
            allowCreate={!!create.onCreate}
            onCreate={create.onCreate}
            validate={validateComponentName}
            separator={null}
            placeholder="Type to search"
          />
        </div>
        {!replace && (
          <div className="bulk-row">
            <span>Remove</span>
            <TokenPicker
              label="Remove components"
              value={remove}
              onChange={setRemove}
              suggestions={suggestions}
              allowCreate={false}
              validate={validateComponentName}
              separator={null}
              placeholder="Type to search"
            />
          </div>
        )}
        {changed !== null && (
          <p className="muted bulk-preview">
            {changed} of {n} selected tests will change.
          </p>
        )}
        {create.error && <div className="error-text">{create.error}</div>}
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
