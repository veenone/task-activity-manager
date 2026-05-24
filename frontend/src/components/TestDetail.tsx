import { useEffect, useState } from "react";
import { GetTest, GetTestPreconditions, errMsg } from "../api";
import type { TestCase, Precondition } from "../api";

interface Props {
  profileId: string;
  testKey: string;
  onClose: () => void;
}

export function TestDetail({ profileId, testKey, onClose }: Props) {
  const [test, setTest] = useState<TestCase | null>(null);
  const [preconditions, setPreconditions] = useState<Precondition[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    setTest(null);
    setPreconditions([]);

    Promise.all([
      GetTest(profileId, testKey),
      GetTestPreconditions(profileId, testKey),
    ])
      .then(([t, pre]) => {
        if (cancelled) return;
        setTest(t);
        setPreconditions(pre);
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
  }, [profileId, testKey]);

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

      {test && (
        <div className="detail-body">
          <h3>{test.summary}</h3>
          <dl className="detail-fields">
            <dt>Status</dt>
            <dd>{test.status || "—"}</dd>
            <dt>Priority</dt>
            <dd>{test.priority || "—"}</dd>
            <dt>Labels</dt>
            <dd>
              {test.labels && test.labels.length > 0
                ? test.labels.join(", ")
                : "—"}
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

          <h4>Description</h4>
          <pre className="detail-desc">
            {test.description ? test.description : "(no description)"}
          </pre>

          <p className="muted detail-note">
            Test steps load in a later update — synced lazily from Xray.
          </p>
        </div>
      )}
    </aside>
  );
}
