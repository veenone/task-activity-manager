import { useState } from "react";
import type { FormEvent } from "react";
import { call, errMsg } from "@agile-suite/core";
import { LookupIssue } from "../api";
import type { Issue, LinkDraft } from "../api";
import { useAddLink, useLinkTypes } from "../queries/pending";

interface Props {
  profileId: string;
  issueKey: string;
  onAdded: () => void;
}

// AddLinkForm journals a link from the panel's issue to any key. The
// phrasing select lists each type's outward and inward wording (once when
// they read the same); Check confirms the target through Jira and shows
// its summary; Add journals it for the next Commit.
export function AddLinkForm({ profileId, issueKey, onAdded }: Props) {
  const types = useLinkTypes(profileId);
  const add = useAddLink(profileId);
  const [choice, setChoice] = useState("");
  const [key, setKey] = useState("");
  const [target, setTarget] = useState<Issue | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");

  const options: { value: string; label: string }[] = [];
  for (const t of types.data ?? []) {
    options.push({ value: `${t.name}|outward`, label: t.outward });
    if (t.inward !== t.outward) options.push({ value: `${t.name}|inward`, label: t.inward });
  }
  const selected = choice || options[0]?.value || "";

  async function check() {
    const k = key.trim().toUpperCase();
    setError("");
    setTarget(null);
    if (!k) {
      setError("Enter an issue key.");
      return;
    }
    setChecking(true);
    try {
      setTarget(await call(() => LookupIssue(profileId, k)));
      setKey(k);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setChecking(false);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!target || !selected) return;
    const [type, direction] = selected.split("|") as [string, "outward" | "inward"];
    const link: LinkDraft = { type, direction, toKey: target.key, toSummary: target.summary, toType: target.type };
    setError("");
    try {
      await add.mutateAsync({ key: issueKey, link });
      setKey("");
      setTarget(null);
      onAdded();
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <form className="add-link" onSubmit={(e) => void onSubmit(e)} aria-labelledby="add-link-title">
      <h3 id="add-link-title">Add link</h3>
      <label className="edit-row" htmlFor="link-type">
        <span className="muted small">Link</span>
        <select id="link-type" className="detail-input" value={selected} onChange={(e) => setChoice(e.target.value)} disabled={types.isPending || options.length === 0}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      </label>
      <label className="edit-row" htmlFor="link-key">
        <span className="muted small">Issue key</span>
        <span className="add-link-key">
          <input id="link-key" className="detail-input" type="text" value={key} placeholder="Any project, for example PAY-77" onChange={(e) => { setKey(e.target.value); setTarget(null); }} />
          <button type="button" className="btn" onClick={() => void check()} disabled={checking || !key.trim()}>{checking ? "Checking" : "Check"}</button>
        </span>
      </label>
      {target && (
        <p className="small">{`${target.key}, ${target.type ? target.type[0].toUpperCase() + target.type.slice(1) : "Issue"}, ${target.summary}`}</p>
      )}
      {types.isError && <p className="error-text small">Link types could not be read: {types.error.message}</p>}
      {error && <p className="error-text small" role="alert">{error}</p>}
      <div className="edit-actions">
        <button type="submit" className="btn btn-primary" disabled={!target || !selected || add.isPending}>Add</button>
        <span className="muted small">Journaled now, pushed on Commit.</span>
      </div>
    </form>
  );
}
