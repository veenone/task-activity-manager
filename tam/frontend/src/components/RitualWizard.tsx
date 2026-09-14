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

  // Issue order is the published order, so the wizard has to show it rather
  // than the sprint's own order: chosen issues render first, in the order
  // `chosen` itself carries, followed by the sprint's remaining unselected
  // issues. A key `chosen` carries that `sprintIssues` no longer does (the
  // card left the sprint since this draft was last saved) still renders,
  // marked so the user can tell it is gone, and can still be deselected;
  // dropping it silently would re-save a selection the user never saw.
  const sprintByKey = new Map(sprintIssues.map((issue) => [issue.key, issue] as const));
  const chosenKeys = new Set(chosen.map((i) => i.key));
  const rows = [
    ...chosen.map((c) => {
      const issue = sprintByKey.get(c.key);
      return issue
        ? { key: c.key, summary: issue.summary, status: issue.status, missing: false }
        : { key: c.key, summary: "", status: "", missing: true };
    }),
    ...sprintIssues
      .filter((issue) => !chosenKeys.has(issue.key))
      .map((issue) => ({ key: issue.key, summary: issue.summary, status: issue.status, missing: false })),
  ];

  return (
    <section className="ritual-wizard" aria-label={`Edit ${ritualType}`}>
      <h3>{draft.title || ritualType}</h3>

      <label className="field">
        <span>Remark</span>
        <textarea value={remark} onChange={(e) => setRemark(e.target.value)} rows={3} />
      </label>

      <h4>Issues</h4>
      <ul className="ritual-issues">
        {rows.map((row) => {
          const picked = chosen.find((i) => i.key === row.key);
          return (
            <li key={row.key} role="group" aria-label={`${row.key} ${row.summary}`}>
              <label>
                <input type="checkbox" checked={!!picked} onChange={() => toggle(row.key)} />
                <span>{row.key} {row.summary}</span>
                {row.missing ? <span className="warn-text small">No longer in the sprint</span> : <span className="muted small">{row.status}</span>}
              </label>
              {picked && (
                <label className="field">
                  <span>Issue remark</span>
                  <input value={picked.remark} onChange={(e) => setIssueRemark(row.key, e.target.value)} />
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
