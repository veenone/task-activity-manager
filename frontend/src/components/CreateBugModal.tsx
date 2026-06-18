import { useState } from "react";
import { CreateBugForTest, errMsg } from "../api";

interface Props {
  profileId: string;
  testKey: string;
  testSummary: string;
  execKey: string;
  onClose: () => void;
  onCreated: () => void;
}

const PRIORITIES = ["Highest", "High", "Medium", "Low", "Lowest"];

// CreateBugModal files a Bug-type Jira issue against a test marked FAILED in an
// execution. Local-first: the bug is queued and pushed on the next Commit.
export function CreateBugModal({
  profileId,
  testKey,
  testSummary,
  execKey,
  onClose,
  onCreated,
}: Props) {
  const [summary, setSummary] = useState("");
  const [priority, setPriority] = useState("Medium");
  const [labels, setLabels] = useState("");
  const [description, setDescription] = useState(
    `Found while executing ${execKey}.\nTest ${testKey} "${testSummary}" was marked FAILED.\n\nSteps / actual result:\n`,
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function create() {
    if (!summary.trim()) return;
    setBusy(true);
    setError("");
    try {
      await CreateBugForTest(
        profileId,
        testKey,
        execKey,
        summary.trim(),
        description,
        priority,
        labels.trim() ? labels.trim().split(/[\s,]+/) : [],
      );
      onCreated();
      onClose();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="pending-head">
          <h2>Create bug for {testKey}</h2>
          <button className="btn btn-ghost" onClick={onClose} title="Cancel" aria-label="Cancel">
            ✕
          </button>
        </div>
        <div className="bug-form">
          <label>
            Summary
            <input
              value={summary}
              autoFocus
              onChange={(e) => setSummary(e.target.value)}
              placeholder="Short defect title"
            />
          </label>
          <label>
            Priority
            <select value={priority} onChange={(e) => setPriority(e.target.value)}>
              {PRIORITIES.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </select>
          </label>
          <label>
            Labels (space or comma separated)
            <input
              value={labels}
              onChange={(e) => setLabels(e.target.value)}
              placeholder="regression login"
            />
          </label>
          <label>
            Description
            <textarea
              rows={6}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </label>
          {error && <div className="error-text">{error}</div>}
        </div>
        <div className="pending-actions">
          <button className="btn" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button
            className="btn btn-primary"
            onClick={create}
            disabled={busy || !summary.trim()}
          >
            {busy ? "Filing…" : "Create bug"}
          </button>
        </div>
      </div>
    </div>
  );
}
