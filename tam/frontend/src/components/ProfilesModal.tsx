import { useEffect, useState } from "react";
import { Modal, useConfirm, useNotice, useProfile, errMsg } from "@agile-suite/core";
import { DeleteProfile, ExportProfile, ImportProfile } from "../api";
import type { Profile, Settings } from "../api";
import { ProfileForm } from "./ProfileForm";

// ProfilesModal is the master-detail profile manager, the same feature XTM's
// Manage Profiles dialog is: every profile on the left with a star for the
// launch default, the selected one's ProfileForm on the right, New / Import
// above the list and Export / Delete in the open profile's footer. The one
// thing TAM says that XTM does not is that the list is shared: a profile made
// or deleted here appears or disappears in XTM too.
export function ProfilesModal({ onClose }: { onClose: () => void }) {
  const { profiles, activeId, defaultProfileId, reload, setDefault } =
    useProfile<Profile, Settings>();
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  // Nothing is open when the modal appears: the user must explicitly pick a
  // profile to edit or choose to create one. Auto-loading the active profile
  // is exactly what lets people edit their live connection by accident.
  const [selectedId, setSelectedId] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  // If the open profile disappears from under us (deleted), fall back to the
  // calm start state rather than silently snapping to the active profile.
  useEffect(() => {
    if (creating) return;
    if (selectedId && !profiles.some((p) => p.id === selectedId)) {
      setSelectedId("");
    }
  }, [profiles, selectedId, creating]);

  const selected = profiles.find((p) => p.id === selectedId) ?? null;
  const editingActive = !!selected && selected.id === activeId;

  function startCreate() {
    // Clear any open profile so Cancel returns to the start state, not to
    // whatever was previously being edited.
    setSelectedId("");
    setCreating(true);
    setError("");
  }

  async function saved(p: Profile) {
    setError("");
    await reload();
    setCreating(false);
    setSelectedId(p.id);
  }

  async function exportProfile(id: string) {
    try {
      const path = await ExportProfile(id);
      if (path) {
        notice({ title: "Profile exported", message: `Saved to ${path}` });
      }
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function importProfile() {
    try {
      const p = await ImportProfile();
      if (!p.id) return; // the dialog was cancelled
      await reload();
      setCreating(false);
      setSelectedId(p.id);
      notice({
        title: "Profile imported",
        message: "A profile file carries no token. Enter one here before syncing.",
      });
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function remove(p: Profile) {
    const ok = await confirm({
      title: `Delete profile "${p.name}"?`,
      message:
        "This removes the profile from Xray Test Manager too, along with its stored token and every issue TAM cached for it. This cannot be undone.",
      confirmLabel: "Delete profile",
      danger: true,
    });
    if (!ok) return;
    try {
      await DeleteProfile(p.id);
      await reload();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  return (
    <Modal onClose={onClose} className="modal profiles-modal" labelledBy="profiles-modal-title">
      <div className="profiles-modal-head">
        <div className="profiles-modal-head-text">
          <h2 id="profiles-modal-title">Manage Profiles</h2>
          <span className="profiles-modal-sub">
            {creating
              ? "A new profile won't become active until you switch to it."
              : "Editing here won't switch your active connection. The list is shared with Xray Test Manager."}
          </span>
        </div>
        <button className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">
          ✕
        </button>
      </div>

      <div className="profiles-modal-body">
        <div className="profiles-list">
          <div className="profiles-create-cta">
            <button
              className="btn btn-primary btn-block"
              onClick={startCreate}
              title="Create a new profile"
            >
              <svg
                width="15"
                height="15"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.4"
                strokeLinecap="round"
                aria-hidden="true"
              >
                <path d="M12 5v14M5 12h14" />
              </svg>
              Create new profile
            </button>
            <button
              className="btn btn-block profiles-import-btn"
              onClick={() => void importProfile()}
              title="Import a profile from a file"
            >
              Import from file…
            </button>
          </div>
          <div className="profiles-list-label">Your profiles</div>
          <ul className="profiles-list-items">
            {profiles.map((p) => (
              <li
                key={p.id}
                className={`profiles-list-row${
                  !creating && p.id === selectedId ? " profiles-list-row-selected" : ""
                }`}
                onClick={() => {
                  setCreating(false);
                  setSelectedId(p.id);
                }}
              >
                <button
                  className="profiles-star"
                  title={
                    defaultProfileId === p.id
                      ? "Default on launch (click to clear)"
                      : "Set as default on launch"
                  }
                  aria-label={defaultProfileId === p.id ? "Clear default" : "Set as default"}
                  onClick={(e) => {
                    e.stopPropagation();
                    void setDefault(p.id);
                  }}
                >
                  {defaultProfileId === p.id ? "★" : "☆"}
                </button>
                <span className="profiles-row-name">{p.name}</span>
                <span className="profiles-row-key muted">({p.projectKey})</span>
                {p.id === activeId && <span className="profiles-badge-active">Active</span>}
              </li>
            ))}
            {profiles.length === 0 && <li className="muted">No profiles yet.</li>}
          </ul>
          <p className="profiles-list-note muted small">
            Kiwi TCMS profiles from Xray Test Manager are not listed here; TAM talks to
            Jira only.
          </p>
        </div>

        <div className="profiles-detail">
          {error && (
            <div className="error-text" role="alert">
              {error}
            </div>
          )}
          {creating ? (
            <>
              <div className="profiles-mode-head">
                <span className="profiles-mode-kicker profiles-mode-kicker-new">New profile</span>
              </div>
              <div className="profiles-mode-title">Add a connection</div>
              <ProfileForm
                profiles={profiles}
                onSaved={(p) => void saved(p)}
                onCancel={() => setCreating(false)}
              />
            </>
          ) : selected ? (
            <>
              <div className="profiles-mode-head">
                <span className="profiles-mode-kicker profiles-mode-kicker-edit">
                  Editing profile
                </span>
                <span className="profiles-mode-title">
                  · <span className="mono">{selected.name}</span>
                </span>
              </div>
              {editingActive && (
                <div className="profiles-active-caution">
                  <svg
                    width="15"
                    height="15"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    aria-hidden="true"
                  >
                    <path d="M12 9v4m0 4h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z" />
                  </svg>
                  <span>
                    <b>This is your active profile.</b> Saving updates the Jira connection
                    you&apos;re using right now. It won&apos;t create a new one.
                  </span>
                </div>
              )}
              <ProfileForm
                key={selected.id}
                profile={selected}
                profiles={profiles}
                onSaved={(p) => void saved(p)}
                extraActions={
                  <>
                    <button
                      className="btn"
                      onClick={() => void exportProfile(selected.id)}
                      title="Export this profile (without its token)"
                    >
                      Export
                    </button>
                    <button
                      className="btn btn-danger"
                      onClick={() => void remove(selected)}
                      title="Delete this profile"
                    >
                      Delete
                    </button>
                  </>
                }
              />
            </>
          ) : (
            <div className="profiles-empty">
              <div className="profiles-empty-mark">
                <svg
                  width="22"
                  height="22"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.7"
                  aria-hidden="true"
                >
                  <rect x="3" y="4" width="18" height="16" rx="2" />
                  <path d="M3 9h18" />
                  <circle cx="7" cy="6.5" r="0.6" fill="currentColor" />
                </svg>
              </div>
              <h3>Nothing open yet</h3>
              <p>
                Pick a profile on the left to edit it, or create a new one. Your active
                connection stays as it is until you choose.
              </p>
              <button className="btn btn-primary btn-block" onClick={startCreate}>
                Create new profile
              </button>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}
