import { useEffect, useState } from "react";
import { ListContainers, AllocateTests, errMsg } from "../api";
import type { Container, AllocateResult } from "../api";

interface Props {
  profileId: string;
  testKeys: string[];
  onComplete: () => void;
  onCancel: () => void;
}

const KINDS: Array<{ value: string; label: string }> = [
  { value: "testset", label: "Test Set" },
  { value: "testplan", label: "Test Plan" },
  { value: "testexec", label: "Test Execution" },
];

// BulkAllocateModal adds the selected Tests to an existing Test Set, Test Plan
// or Test Execution (FR-3.4–3.6, add-only). Tests already in the chosen
// container are reported back and not re-queued.
export function BulkAllocateModal({
  profileId,
  testKeys,
  onComplete,
  onCancel,
}: Props) {
  const [kind, setKind] = useState("testset");
  const [containers, setContainers] = useState<Container[]>([]);
  const [target, setTarget] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [applying, setApplying] = useState(false);
  const [applyError, setApplyError] = useState("");
  const [result, setResult] = useState<AllocateResult | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setLoadError("");
    ListContainers(profileId, kind)
      .then((cs) => {
        if (cancelled) return;
        setContainers(cs ?? []);
        setTarget(cs && cs.length > 0 ? cs[0].key : "");
      })
      .catch((e) => {
        if (!cancelled) setLoadError(errMsg(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, kind]);

  async function apply() {
    if (!target) return;
    setApplying(true);
    setApplyError("");
    try {
      const r = await AllocateTests(profileId, target, testKeys);
      setResult(r);
    } catch (e) {
      setApplyError(errMsg(e));
    } finally {
      setApplying(false);
    }
  }

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal bulk-modal" onClick={(e) => e.stopPropagation()}>
        <div className="pending-head">
          <h2>
            Allocate ({testKeys.length}{" "}
            {testKeys.length === 1 ? "test" : "tests"})
          </h2>
          <button className="btn btn-ghost" onClick={onCancel} title="Close">
            ✕
          </button>
        </div>

        {!result && (
          <div className="bulk-body">
            <label className="bulk-row">
              <span>Type</span>
              <select value={kind} onChange={(e) => setKind(e.target.value)}>
                {KINDS.map((k) => (
                  <option key={k.value} value={k.value}>
                    {k.label}
                  </option>
                ))}
              </select>
            </label>

            <label className="bulk-row">
              <span>Container</span>
              {loading ? (
                <span className="muted">Loading…</span>
              ) : (
                <select
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  disabled={containers.length === 0}
                >
                  {containers.length === 0 && (
                    <option value="">None synced for this type</option>
                  )}
                  {containers.map((c) => (
                    <option key={c.key} value={c.key}>
                      {c.key} — {c.summary}
                    </option>
                  ))}
                </select>
              )}
            </label>

            {loadError && <div className="error-text">{loadError}</div>}

            <p className="muted bulk-preview">
              Selected tests are added to the container as local pending
              changes; commit them from the Pending list. Tests already in the
              container are skipped.
            </p>

            {applyError && <div className="error-text">{applyError}</div>}
          </div>
        )}

        {result && (
          <div className="bulk-body">
            {result.added.length > 0 && (
              <p className="ok-text">
                ✓ Queued {result.added.length}{" "}
                {result.added.length === 1 ? "test" : "tests"} for allocation to{" "}
                <span className="mono">{target}</span>.
              </p>
            )}
            {result.alreadyMembers.length > 0 && (
              <p className="muted">
                {result.alreadyMembers.length}{" "}
                {result.alreadyMembers.length === 1 ? "test was" : "tests were"}{" "}
                already in the container.
              </p>
            )}
            {result.added.length === 0 &&
              result.alreadyMembers.length === 0 && (
                <p className="muted">Nothing to allocate.</p>
              )}
          </div>
        )}

        <div className="pending-actions">
          {!result ? (
            <>
              <button className="btn" onClick={onCancel} disabled={applying}>
                Cancel
              </button>
              <button
                className="btn btn-primary"
                onClick={apply}
                disabled={applying || loading || !target}
              >
                {applying ? "Allocating…" : "Allocate"}
              </button>
            </>
          ) : (
            <button className="btn btn-primary" onClick={onComplete}>
              Done
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
