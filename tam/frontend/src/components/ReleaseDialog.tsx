import { useState } from "react";
import { Modal, announce, errMsg, toPlainText, usePrompt } from "@agile-suite/core";
import type { Issue, VersionSummary } from "../api";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";
import { defaultCarryOver, isOpen } from "../lib/releases";
import { useCreateRelease, useReleaseRelease } from "../queries/releases";

interface Props {
  profileId: string;
  release: VersionSummary;
  // releases is every cached release, for the carry-over picker.
  releases: VersionSummary[];
  // unfinished is the release's cached issues that are not done.
  unfinished: Issue[];
  // scoped says the profile's scope JQL may hide some of the release's
  // issues from the cache.
  scoped: boolean;
  onClose: () => void;
  onReleased: (line: string) => void;
}

// DESCOPE is the choice value for "take the release off, add nothing".
const DESCOPE = "0";

// today is the local day as a date input holds it.
function today(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// ReleaseDialog queues a release. Each unfinished issue is carried over to
// another open release or descoped, which removes this release from the
// issue's fix versions and changes nothing else about it. One control sets
// every row. Confirming journals those edits and the release together; the
// set-all release also catches, at Commit, unfinished issues Jira holds in
// the release that TAM never synced.
export function ReleaseDialog({ profileId, release, releases, unfinished, scoped, onClose, onReleased }: Props) {
  const { runQuietLock } = useSync();
  const { prompt } = usePrompt();
  const send = useReleaseRelease(profileId, runQuietLock);
  const create = useCreateRelease(profileId, runQuietLock);
  const [made, setMade] = useState<VersionSummary[]>([]);
  const targets = [...releases, ...made].filter((v) => isOpen(v) && v.id !== release.id);
  const first = String(defaultCarryOver([...releases, ...made], release.id) || DESCOPE);
  const [all, setAll] = useState(first);
  const [choices, setChoices] = useState<Record<string, string>>(() => Object.fromEntries(unfinished.map((i) => [i.key, first])));
  const [date, setDate] = useState(today());
  const [error, setError] = useState("");
  const busy = send.isPending || create.isPending;

  function setEvery(value: string) {
    setAll(value);
    setChoices(Object.fromEntries(unfinished.map((i) => [i.key, value])));
  }

  async function newRelease() {
    const name = await prompt({ title: "New release", placeholder: "Name", submitLabel: "Create release" });
    if (!name?.trim()) return;
    create.mutate({ name: name.trim(), description: "", startDate: "", releaseDate: "" }, {
      onSuccess: (v) => {
        setMade((m) => [...m, { ...v, done: 0, total: 0 }]);
        setEvery(String(v.id));
      },
      onError: (err) => setError(errMsg(err)),
    });
  }

  function onSubmit() {
    if (busy) return;
    const moves = unfinished.map((i) => ({ key: i.key, to: Number(choices[i.key] ?? DESCOPE) }));
    send.mutate({ id: release.id, date, moveTo: Number(all), moves }, {
      onSuccess: () => {
        const line = `${release.name} will be released on Commit.`;
        announce(line);
        onReleased(line);
        onClose();
      },
      onError: (err) => setError(errMsg(err)),
    });
  }

  const options = (
    <>
      {targets.map((v) => <option key={v.id} value={String(v.id)}>{`Carry over to ${v.name}`}</option>)}
      <option value={DESCOPE}>Descope</option>
    </>
  );

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="release-dialog-title" closeOnOverlayClick={false} closeOnEsc={!busy}>
      <div className="pending-head">
        <div className="edit-sprint-title">
          <h2 id="release-dialog-title">{`Release ${release.name}`}</h2>
          <p>Saved locally. Commit releases it in Jira after the issue changes below.</p>
        </div>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <div className="bulk-body edit-form">
        <label className="edit-row" htmlFor="release-dialog-date">
          <span className="muted small">Release date</span>
          <input id="release-dialog-date" className="detail-input detail-input-inline" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </label>
        <p>
          {unfinished.length === 0
            ? "Every issue TAM holds in this release is done."
            : `${plural(unfinished.length, "issue is", "issues are")} not done. Carry each over to another release, or descope it. Descoping takes this release off the issue and leaves its sprint and status alone.`}
        </p>
        {targets.length === 0 && (
          <p className="release-none">
            <span className="muted small">No other release is open to carry issues over to. </span>
            <button type="button" className="btn" disabled={busy} onClick={() => void newRelease()}>New release</button>
          </p>
        )}
        <label className="edit-row" htmlFor="release-dialog-all">
          <span className="muted small">Set all</span>
          <select id="release-dialog-all" className="detail-input" value={all} onChange={(e) => setEvery(e.target.value)}>{options}</select>
        </label>
        {unfinished.length > 0 && (
          <div className="pending-list release-choices">
            {unfinished.map((i) => (
              <div key={i.key} className="pending-card release-choice">
                <span className="b accent-text">{i.key}</span>
                <span className="pending-card-summary">{toPlainText(i.summary, "summary")}</span>
                <select aria-label={i.key} className="detail-input" value={choices[i.key] ?? DESCOPE} onChange={(e) => setChoices((c) => ({ ...c, [i.key]: e.target.value }))}>
                  {options}
                </select>
              </div>
            ))}
          </div>
        )}
        <p className="muted small">
          {scoped
            ? "The profile's scope JQL may hide some of this release's issues from TAM. Commit asks Jira for them before releasing and moves any that are not done to the Set all release; with Descope there, the release waits until they are handled."
            : "Commit asks Jira for this release's unfinished issues before releasing, so none are left behind in it."}
        </p>
        {error && <p className="error-text" role="alert">{error}</p>}
      </div>

      <div className="pending-actions">
        <span className="muted small">Until Commit you can discard the release in Pending changes.</span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="button" className="btn btn-primary" disabled={busy} onClick={onSubmit}>{send.isPending ? "Saving" : "Release"}</button>
        </span>
      </div>
    </Modal>
  );
}
