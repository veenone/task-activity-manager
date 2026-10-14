import { useState } from "react";
import { Modal } from "../Modal";

interface SwapItem {
  key: string;
  label: string;
}

// SwapModal lists the test's current items as checkboxes (ticked = remove) and
// a multi-pick add list, then computes next = (current minus removed) + added
// and hands it back in one apply (RND_P_4TFINT_05-231).
export function SwapModal({
  title,
  current,
  candidates,
  onCancel,
  onConfirm,
}: {
  title: string;
  current: SwapItem[];
  candidates: SwapItem[];
  onCancel: () => void;
  onConfirm: (next: string[]) => void | Promise<void>;
}) {
  const [toRemove, setToRemove] = useState<Set<string>>(new Set());
  const [toAdd, setToAdd] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);

  function toggleRemove(key: string) {
    setToRemove((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function toggleAdd(key: string) {
    setToAdd((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  async function confirm() {
    const next = [
      ...current.map((c) => c.key).filter((k) => !toRemove.has(k)),
      ...candidates.map((c) => c.key).filter((k) => toAdd.has(k)),
    ];
    setBusy(true);
    try {
      await onConfirm(next);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal onClose={onCancel} className="modal bulk-modal" labelledBy="bulk-swap-title">
        <div className="pending-head">
          <h2 id="bulk-swap-title">{title}</h2>
          <button className="btn btn-ghost" onClick={onCancel} title="Close" aria-label="Close">
            ✕
          </button>
        </div>
        <div className="bulk-body">
          <div className="bulk-row bulk-swap-row">
            <span>Remove</span>
            <ul className="bulk-swap-list">
              {current.length === 0 && <li className="muted">None linked</li>}
              {current.map((c) => (
                <li key={c.key}>
                  <label>
                    <input
                      type="checkbox"
                      checked={toRemove.has(c.key)}
                      onChange={() => toggleRemove(c.key)}
                    />
                    <span>{c.label}</span>
                  </label>
                </li>
              ))}
            </ul>
          </div>
          <div className="bulk-row bulk-swap-row">
            <span>Add</span>
            <ul className="bulk-swap-list">
              {candidates.length === 0 && (
                <li className="muted">Nothing else to add</li>
              )}
              {candidates.map((c) => (
                <li key={c.key}>
                  <label>
                    <input
                      type="checkbox"
                      checked={toAdd.has(c.key)}
                      onChange={() => toggleAdd(c.key)}
                    />
                    <span>{c.label}</span>
                  </label>
                </li>
              ))}
            </ul>
          </div>
          <p className="muted bulk-preview">
            Ticked Remove items are dropped, and ticked Add items are linked,
            in one apply. The change is queued locally, commit it from
            Pending.
          </p>
        </div>
        <div className="pending-actions">
          <button className="btn" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
          <button
            className="btn btn-primary"
            onClick={confirm}
            disabled={busy || (toRemove.size === 0 && toAdd.size === 0)}
          >
            {busy ? "Applying…" : "Apply swap"}
          </button>
        </div>
    </Modal>
  );
}
