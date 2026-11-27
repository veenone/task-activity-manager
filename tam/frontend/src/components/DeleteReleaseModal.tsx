import { useState } from "react";
import { Modal, announce, errMsg } from "@agile-suite/core";
import type { VersionSummary } from "../api";
import { useSync } from "../contexts/SyncContext";
import { plural } from "../lib/format";
import { useDeleteRelease } from "../queries/releases";

interface Props {
  profileId: string;
  release: VersionSummary;
  // releases is every cached release, for the "move its issues to" picker.
  releases: VersionSummary[];
  onClose: () => void;
  onDeleted: (line: string) => void;
}

// DeleteReleaseModal deletes a release. A draft goes at once and leaves
// every issue it was set on; a release Jira holds is deleted on Commit,
// with its issues moved to another release when one is picked.
export function DeleteReleaseModal({ profileId, release, releases, onClose, onDeleted }: Props) {
  const { runQuietLock } = useSync();
  const del = useDeleteRelease(profileId, runQuietLock);
  const [moveTo, setMoveTo] = useState("0");
  const [error, setError] = useState("");
  const targets = releases.filter((v) => v.id !== release.id && !v.archived);
  const issues = plural(release.total, "issue", "issues");

  function onSubmit() {
    if (del.isPending) return;
    del.mutate({ id: release.id, moveTo: release.draft ? 0 : Number(moveTo) }, {
      onSuccess: () => {
        const line = release.draft ? `The draft ${release.name} was deleted.` : `${release.name} will be deleted on Commit.`;
        announce(line);
        onDeleted(line);
        onClose();
      },
      onError: (err) => setError(errMsg(err)),
    });
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" role="alertdialog" labelledBy="delete-release-title" closeOnOverlayClick={false} closeOnEsc={!del.isPending}>
      <div className="pending-head">
        <h2 id="delete-release-title">{release.draft ? `Delete the draft ${release.name}?` : `Delete ${release.name}?`}</h2>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>
      <div className="bulk-body edit-form">
        {release.draft ? (
          <p>Nothing reaches Jira. Issues you gave this release lose it.</p>
        ) : (
          <>
            <p>{`Commit deletes ${release.name} in Jira. TAM holds ${issues} in it; the issues themselves are not deleted.`}</p>
            <label className="edit-row" htmlFor="delete-release-move">
              <span className="muted small">Move its issues to</span>
              <select id="delete-release-move" className="detail-input" value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
                <option value="0">No release</option>
                {targets.map((v) => <option key={v.id} value={String(v.id)}>{v.name}</option>)}
              </select>
            </label>
            <p className="muted small">Until then you can discard the delete in Pending changes. Once Commit sends it, it cannot be undone.</p>
          </>
        )}
        {error && <p className="error-text" role="alert">{error}</p>}
      </div>
      <div className="pending-actions">
        <span />
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={del.isPending}>Keep it</button>
          <button type="button" className="btn btn-danger" disabled={del.isPending} onClick={onSubmit}>
            {release.draft ? "Delete draft" : "Delete release"}
          </button>
        </span>
      </div>
    </Modal>
  );
}
