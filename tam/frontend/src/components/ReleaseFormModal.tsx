import { useState } from "react";
import type { FormEvent } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import type { VersionSummary } from "../api";
import { useSync } from "../contexts/SyncContext";
import { useCreateRelease, useEditRelease } from "../queries/releases";

interface Props {
  profileId: string;
  // release is the one being edited; without it the dialog drafts a new one.
  release?: VersionSummary;
  onClose: () => void;
  // onSaved hands back the release's id and the sentence for the banner.
  onSaved: (id: number, line: string) => void;
}

// ReleaseFormModal drafts a new release or edits one. Either way nothing
// reaches Jira until Commit: a new release is a draft under a negative id,
// and an edit of a release Jira holds is a pending change.
export function ReleaseFormModal({ profileId, release, onClose, onSaved }: Props) {
  const { runQuietLock } = useSync();
  const create = useCreateRelease(profileId, runQuietLock);
  const edit = useEditRelease(profileId, runQuietLock);
  const [name, setName] = useState(release?.name ?? "");
  const [description, setDescription] = useState(release?.description ?? "");
  const [startDate, setStartDate] = useState(release?.startDate ?? "");
  const [releaseDate, setReleaseDate] = useState(release?.releaseDate ?? "");
  const [error, setError] = useState("");
  const busy = create.isPending || edit.isPending;
  const title = release ? `Edit ${release.name}` : "New release";

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    const fields = { name: name.trim(), description, startDate, releaseDate };
    if (!fields.name) {
      setError("A release needs a name.");
      return;
    }
    if (startDate && releaseDate && releaseDate < startDate) {
      setError("A release cannot end before it starts.");
      return;
    }
    const done = (id: number, line: string) => {
      announce(line);
      onSaved(id, line);
      onClose();
    };
    if (!release) {
      create.mutate(fields, {
        onSuccess: (made) => done(made.id, `${made.name || fields.name} was drafted. Commit creates it in Jira.`),
        onError: (err) => setError(errMsg(err)),
      });
      return;
    }
    edit.mutate({ ...fields, id: release.id }, {
      onSuccess: () => done(release.id, release.draft
        ? `${fields.name} was saved.`
        : `${fields.name} was saved locally. Commit sends the change to Jira.`),
      onError: (err) => setError(errMsg(err)),
    });
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="release-form-title" closeOnOverlayClick={false} closeOnEsc={!busy}>
      <div className="pending-head">
        <h2 id="release-form-title">{title}</h2>
        <p className="muted small">{release && !release.draft ? "Saved locally. Commit sends the change to Jira." : "Drafted locally. Commit creates it in Jira."}</p>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="release-form" className="bulk-body edit-form" onSubmit={onSubmit}>
        <label className="edit-row" htmlFor="release-name">
          <span className="muted small">Name</span>
          <input id="release-name" className="detail-input" value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="edit-row" htmlFor="release-description">
          <span className="muted small">Description</span>
          <textarea id="release-description" className="detail-input" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <div className="edit-row">
          <span className="muted small">Dates</span>
          <span className="date-field">
            <label className="muted small" htmlFor="release-start">Start date</label>
            <input id="release-start" className="detail-input detail-input-inline" type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
            <label className="muted small" htmlFor="release-date">Release date</label>
            <input id="release-date" className="detail-input detail-input-inline" type="date" value={releaseDate} onChange={(e) => setReleaseDate(e.target.value)} />
          </span>
        </div>
      </form>

      <div className="pending-actions">
        <span className="new-issue-status">
          {error && <span className="error-text small" role="alert">{error}</span>}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={busy}>Cancel</button>
          <button type="submit" form="release-form" className="btn btn-primary" disabled={busy}>
            {busy ? "Saving" : release ? "Save" : "Create release"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
