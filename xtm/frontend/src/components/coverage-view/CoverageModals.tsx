import { useCallback, useEffect, useState } from "react";
import { SetCanonicalMembers, ListCoverageCandidateTests, ListValueTests, SetValueTests, ListRequirementsWithCoverage, errMsg } from "../../api";
import type { CandidateTest } from "../../api";
import { BrowseTestsPicker } from "../CoverageTestPicker";
import { Modal } from "../Modal";

// MapTestsModal lets the user pick which Tests exercise a parameter value, from
// the pool of tests linked to the canonical's member requirements.
export function MapTestsModal({
  profileId,
  canonicalId,
  valueId,
  valueLabel,
  onClose,
  onSaved,
}: {
  profileId: string;
  canonicalId: string;
  valueId: string;
  valueLabel: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [cands, setCands] = useState<CandidateTest[]>([]);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [manual, setManual] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [showPicker, setShowPicker] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const [c, current] = await Promise.all([
          ListCoverageCandidateTests(profileId, canonicalId),
          ListValueTests(profileId, valueId),
        ]);
        setCands(c);
        setPicked(new Set(current));
      } catch (e) {
        setError(errMsg(e));
      }
    })();
  }, [profileId, canonicalId, valueId]);

  function toggle(key: string) {
    setPicked((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  async function save() {
    setBusy(true);
    try {
      const extra = manual
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      const keys = Array.from(new Set([...picked, ...extra]));
      await SetValueTests(profileId, valueId, keys);
      onSaved();
    } catch (e) {
      setError(errMsg(e));
      setBusy(false);
    }
  }

  // Merge keys chosen in the BrowseTestsPicker into the manual field, skipping
  // any already present in the candidate checkboxes (picked) or manual text.
  function handlePickerAdd(keys: string[]) {
    const existingManual = new Set(
      manual
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
    );
    const toAdd = keys.filter((k) => !picked.has(k) && !existingManual.has(k));
    if (toAdd.length === 0) return;
    const base = manual.trim();
    setManual(base ? `${base}, ${toAdd.join(", ")}` : toAdd.join(", "));
  }

  const handleClosePicker = useCallback(() => setShowPicker(false), []);

  // Compute the keys already selected so the picker can show them as disabled.
  const pickerExcludeKeys = [
    ...picked,
    ...manual.split(",").map((s) => s.trim()).filter(Boolean),
  ];

  return (
    <>
      <Modal onClose={onClose} className="modal map-tests-modal" labelledBy="cov-map-tests-title">
          <div className="pending-head">
            <h2 id="cov-map-tests-title">Map tests to "{valueLabel}"</h2>
            <button className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">✕</button>
          </div>
          <div className="cov-body">
            {error && <div className="cov-error">{error}</div>}
            <div className="cov-cands">
              {cands.length === 0 && <p className="cov-empty">No candidate tests (link member requirements that have covering tests).</p>}
              {cands.map((c) => (
                <label key={c.testKey} className="cov-cand">
                  <input type="checkbox" checked={picked.has(c.testKey)} onChange={() => toggle(c.testKey)} />
                  <span className="cov-cand-key">{c.testKey}</span>
                  <span className="cov-cand-sum">{c.summary}</span>
                </label>
              ))}
            </div>
            <div className="cov-manual">
              Other test keys (comma-separated):
              <div className="cov-manual-row">
                <input
                  className="cov-input"
                  value={manual}
                  onChange={(e) => setManual(e.target.value)}
                  placeholder="TEST-1234, TEST-1235"
                />
                <button className="btn" type="button" onClick={() => setShowPicker(true)}>
                  Browse tests…
                </button>
              </div>
            </div>
          </div>
          <div className="pending-actions">
            <button className="btn" onClick={onClose}>Cancel</button>
            <button className="btn btn-primary" disabled={busy} onClick={() => void save()}>Save</button>
          </div>
      </Modal>
      {showPicker && (
        <BrowseTestsPicker
          excludeKeys={pickerExcludeKeys}
          onClose={handleClosePicker}
          onAdd={handlePickerAdd}
        />
      )}
    </>
  );
}

// MembersModal links the customer/project requirements that reuse this canonical
// functional requirement, picked from the profile's synced requirements.
export function MembersModal({
  profileId,
  canonicalId,
  current,
  onClose,
  onSaved,
}: {
  profileId: string;
  canonicalId: string;
  current: string[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [reqs, setReqs] = useState<{ key: string; summary: string; project: string }[]>([]);
  const [picked, setPicked] = useState<Set<string>>(new Set(current));
  const [filter, setFilter] = useState("");
  const [manual, setManual] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const list = await ListRequirementsWithCoverage(profileId);
        setReqs(list.map((r) => ({ key: r.key, summary: r.summary, project: r.projectKey ?? "" })));
      } catch (e) {
        setError(errMsg(e));
      }
    })();
  }, [profileId]);

  const shown = reqs.filter(
    (r) => !filter || r.key.toLowerCase().includes(filter.toLowerCase()) || r.summary.toLowerCase().includes(filter.toLowerCase()),
  );

  async function save() {
    setBusy(true);
    try {
      const extra = manual.split(",").map((s) => s.trim()).filter(Boolean);
      const keys = Array.from(new Set([...picked, ...extra]));
      await SetCanonicalMembers(profileId, canonicalId, keys);
      onSaved();
    } catch (e) {
      setError(errMsg(e));
      setBusy(false);
    }
  }

  function toggle(key: string) {
    setPicked((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  return (
    <Modal onClose={onClose} className="modal members-modal" labelledBy="cov-members-title">
        <div className="pending-head">
          <h2 id="cov-members-title">Member requirements</h2>
          <button className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">✕</button>
        </div>
        <div className="cov-body">
          {error && <div className="cov-error">{error}</div>}
          <input className="cov-input" placeholder="Filter…" value={filter} onChange={(e) => setFilter(e.target.value)} />
          <div className="cov-cands">
            {shown.map((r) => (
              <label key={r.key} className="cov-cand">
                <input type="checkbox" checked={picked.has(r.key)} onChange={() => toggle(r.key)} />
                <span className="cov-cand-key">{r.key}</span>
                <span className="cov-cand-sum">{r.summary}</span>
              </label>
            ))}
          </div>
          <label className="cov-manual">
            Other requirement keys (comma-separated):
            <input className="cov-input" value={manual} onChange={(e) => setManual(e.target.value)} placeholder="CUST-HSM-BANK-010" />
          </label>
        </div>
        <div className="pending-actions">
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={busy} onClick={() => void save()}>Save</button>
        </div>
    </Modal>
  );
}
