import { useEffect, useState } from "react";
import { AddEvidenceIssue, AddEvidenceNote, Evidence, FindEvidence, RemoveEvidence, SetEvidenceState } from "../wailsjs/go/main/App";
import type { evidence } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";

const KIND: Record<string, string> = {
  link: "Link",
  component: "Component",
  version: "Fix version",
  text: "Text match",
  note: "Note",
  issue: "Issue",
};

const byHand = (kind: string) => kind === "note" || kind === "issue";

// An issue key, as cases.IsIssueKey checks it again in Go.
const ISSUE_KEY = /^[A-Za-z][A-Za-z0-9_]*-[0-9]+$/;

// EvidencePanel is what an analyst weighs before a verdict: suggestions from
// Jira, accepted or dismissed one by one, and notes and issues added by hand.
// Nothing here sets a verdict. projects are the case's own; evidence for any
// other project offers to add it.
export function EvidencePanel({
  profileId,
  caseId,
  projects,
  onAddProject,
}: {
  profileId: string;
  caseId: number;
  projects: string[];
  onAddProject: (key: string) => void;
}) {
  const [items, setItems] = useState<evidence.Item[] | null>(null);
  const [phrases, setPhrases] = useState("");
  const [finding, setFinding] = useState(false);
  const [project, setProject] = useState("");
  const [entry, setEntry] = useState("");
  const [error, setError] = useState("");
  const forProject = projects.includes(project) ? project : (projects[0] ?? "");

  useEffect(() => {
    let current = true;
    Evidence(profileId, caseId).then(
      (l) => current && setItems(l),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId, caseId]);

  async function change(action: () => Promise<evidence.Item[]>) {
    setError("");
    try {
      setItems(await action());
      return true;
    } catch (e) {
      setError(errorText(e));
      return false;
    }
  }

  async function find() {
    setFinding(true);
    const list = phrases.split("\n").map((p) => p.trim()).filter(Boolean);
    await change(() => FindEvidence(profileId, caseId, list));
    setFinding(false);
  }

  const groups = [...new Set((items ?? []).map((i) => i.project))];

  return (
    <section className="pie-evidence" aria-labelledby="pie-evidence-heading">
      <h3 id="pie-evidence-heading">Evidence</h3>
      <label className="field-label" htmlFor="pie-phrases">
        Phrases to search, one per line
      </label>
      <textarea id="pie-phrases" className="detail-input" rows={2} value={phrases} onChange={(e) => setPhrases(e.target.value)} />
      <div className="form-actions">
        <button type="button" className="btn" disabled={finding} onClick={() => void find()}>
          Find evidence
        </button>
        {finding && <span className="muted">Asking Jira...</span>}
        <span className="muted">Links are strong. Shared names and phrase matches are weak: check them before you rely on them.</span>
      </div>
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}

      {items?.length === 0 && <p className="muted">No evidence yet.</p>}
      {groups.map((p) => (
        <div key={p} role="group" aria-label={`Evidence for ${p}`} className="pie-trace-project">
          <div className="pie-detail-head">
            <h3>{p}</h3>
            {!projects.includes(p) && (
              <button type="button" className="btn btn-ghost" onClick={() => onAddProject(p)}>
                {`Add ${p} to the case`}
              </button>
            )}
          </div>
          <ul className="pie-evidence-list">
            {(items ?? [])
              .filter((i) => i.project === p)
              .map((i) => (
                <li key={i.id} className={i.state === "dismissed" ? "pie-evidence-dismissed" : undefined}>
                  <span className="pie-evidence-kind">{i.weak ? `${KIND[i.kind]}, weak` : KIND[i.kind]}</span>
                  <span>{i.detail || i.ref}</span>
                  <span className="muted">{byHand(i.kind) ? "added by hand" : i.state}</span>
                  {!byHand(i.kind) && i.state !== "accepted" && (
                    <button type="button" className="btn btn-ghost" aria-label={`Accept ${i.ref}`} onClick={() => void change(() => SetEvidenceState(profileId, caseId, i.id, "accepted"))}>
                      Accept
                    </button>
                  )}
                  {!byHand(i.kind) && i.state !== "dismissed" && (
                    <button type="button" className="btn btn-ghost" aria-label={`Dismiss ${i.ref}`} onClick={() => void change(() => SetEvidenceState(profileId, caseId, i.id, "dismissed"))}>
                      Dismiss
                    </button>
                  )}
                  {byHand(i.kind) && (
                    <button type="button" className="btn btn-ghost" aria-label={`Remove ${i.ref}`} onClick={() => void change(() => RemoveEvidence(profileId, caseId, i.id))}>
                      Remove
                    </button>
                  )}
                </li>
              ))}
          </ul>
        </div>
      ))}

      {projects.length > 0 && (
        <div className="pie-filters">
          <label>
            Project
            <select className="detail-input pie-filter-select" value={forProject} onChange={(e) => setProject(e.target.value)}>
              {projects.map((k) => (
                <option key={k} value={k}>
                  {k}
                </option>
              ))}
            </select>
          </label>
          <label>
            Note or issue key
            <input className="detail-input" value={entry} onChange={(e) => setEntry(e.target.value)} />
          </label>
          <button
            type="button"
            className="btn"
            disabled={entry.trim() === ""}
            onClick={() => void change(() => AddEvidenceNote(profileId, caseId, forProject, entry)).then((ok) => ok && setEntry(""))}
          >
            Add note
          </button>
          <button
            type="button"
            className="btn"
            disabled={!ISSUE_KEY.test(entry.trim())}
            onClick={() => void change(() => AddEvidenceIssue(profileId, caseId, forProject, entry)).then((ok) => ok && setEntry(""))}
          >
            Add issue
          </button>
        </div>
      )}
    </section>
  );
}
