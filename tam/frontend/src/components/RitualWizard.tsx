import { useEffect, useState } from "react";
import type { Issue } from "../api";
import { encodeRitualIssues, GetRitualDraft, parseRitualIssues, RitualIssue, SaveRitualDraft } from "../api";
import type { RitualDraft } from "../api";

// RitualWizard edits one ritual document. It saves a draft and never publishes:
// reaching Confluence is a separate, deliberate press elsewhere, so nobody
// pushes a page by finishing a form.
export function RitualWizard({
  profileId, boardId, sprintId, ritualType, sprintIssues, onSaved, onCancel,
}: {
  profileId: string; boardId: number; sprintId: number; ritualType: string;
  sprintIssues: Issue[]; onSaved: () => void; onCancel: () => void;
}) {
  const [draft, setDraft] = useState<RitualDraft | null>(null);
  const [remark, setRemark] = useState("");
  const [chosen, setChosen] = useState<RitualIssue[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let live = true;
    void GetRitualDraft(profileId, boardId, sprintId, ritualType)
      .then((d) => {
        if (!live) return;
        setDraft(d);
        setRemark(d.remark ?? "");
        setChosen(parseRitualIssues(d.issuesJson));
      })
      .catch((e) => live && setError(String(e)));
    return () => { live = false; };
  }, [profileId, boardId, sprintId, ritualType]);

  function toggle(key: string) {
    setChosen((current) =>
      current.some((i) => i.key === key)
        ? current.filter((i) => i.key !== key)
        : [...current, { key, remark: "" }]);
  }

  function setIssueRemark(key: string, value: string) {
    setChosen((current) => current.map((i) => (i.key === key ? { ...i, remark: value } : i)));
  }

  function save() {
    if (!draft) return;
    setSaving(true); setError("");
    void SaveRitualDraft(profileId, { ...draft, remark, issuesJson: encodeRitualIssues(chosen) })
      .then(onSaved)
      .catch((e) => setError(String(e)))
      .finally(() => setSaving(false));
  }

  if (!draft) return <p className="muted">{error || "Loading the ritual..."}</p>;

  return (
    <section className="ritual-wizard" aria-label={`Edit ${ritualType}`}>
      <h3>{draft.title || ritualType}</h3>

      <label className="field">
        <span>Remark</span>
        <textarea value={remark} onChange={(e) => setRemark(e.target.value)} rows={3} />
      </label>

      <h4>Issues</h4>
      <ul className="ritual-issues">
        {sprintIssues.map((issue) => {
          const picked = chosen.find((i) => i.key === issue.key);
          return (
            <li key={issue.key} role="group" aria-label={`${issue.key} ${issue.summary}`}>
              <label>
                <input type="checkbox" checked={!!picked} onChange={() => toggle(issue.key)} />
                <span>{issue.key} {issue.summary}</span>
                <span className="muted small">{issue.status}</span>
              </label>
              {picked && (
                <label className="field">
                  <span>Issue remark</span>
                  <input value={picked.remark} onChange={(e) => setIssueRemark(issue.key, e.target.value)} />
                </label>
              )}
            </li>
          );
        })}
      </ul>

      {error && <p className="warn-text small">{error}</p>}
      <div className="row">
        <button onClick={save} disabled={saving}>Save draft</button>
        <button className="ghost" onClick={onCancel}>Cancel</button>
      </div>
    </section>
  );
}
