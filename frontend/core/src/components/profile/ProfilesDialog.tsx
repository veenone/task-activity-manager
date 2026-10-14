import { Fragment, useState } from "react";
import type { ReactNode } from "react";
import { Modal } from "../Modal";
import { useConfirm } from "../useConfirm";
import { errMsg } from "../../lib/errMsg";

// What an app's ProfileForm is handed: the profile to edit (none when
// creating), what to call once it has saved, a cancel for the create form, and
// the dialog's Export and Delete buttons for the form's footer.
export interface ProfileFormSlot<P> {
  profile?: P;
  onSaved: (p: P) => void;
  onCancel?: () => void;
  extraActions?: ReactNode;
}

interface Props<P> {
  profiles: P[];
  activeId: string;
  defaultProfileId: string;
  // A sentence added to the subtitle while editing, e.g. TAM's note that the
  // list is shared with XTM.
  subtitle?: string;
  // A note under the list, e.g. which profiles an app does not show.
  listNote?: ReactNode;
  // What deleting a profile removes, in the app's words.
  deleteMessage: string;
  onSetDefault: (id: string) => unknown;
  onExport: (id: string) => unknown;
  // Resolves to the imported profile, or null when the user cancelled.
  onImport: () => Promise<P | null>;
  onSaved: (p: P) => unknown;
  onDelete: (id: string) => unknown;
  onClose: () => void;
  renderForm: (slot: ProfileFormSlot<P>) => ReactNode;
}

// ProfilesDialog is Manage Profiles for both apps: every profile on the left
// with a star for the launch default, the open profile's form on the right,
// create and import above the list, export and delete in the form's footer.
// The app owns the profiles and passes callbacks; the form is the app's own,
// through renderForm.
//
// Nothing is open when the dialog appears. Opening the active profile by
// default is how people edited their live connection by accident, so the
// start state asks them to pick, and editing the active one carries a caution.
export function ProfilesDialog<P extends { id: string; name: string; projectKey: string }>({
  profiles, activeId, defaultProfileId, subtitle, listNote, deleteMessage,
  onSetDefault, onExport, onImport, onSaved, onDelete, onClose, renderForm,
}: Props<P>) {
  const { confirm } = useConfirm();
  const [selectedId, setSelectedId] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  const selected = profiles.find((p) => p.id === selectedId) ?? null;
  const editingActive = !creating && selected !== null && selected.id === activeId;

  // Every callback reports its failure here rather than vanishing.
  async function run(work: () => unknown) {
    setError("");
    try {
      await work();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  function open(id: string) {
    setCreating(false);
    setSelectedId(id);
  }

  function startCreate() {
    setError("");
    setSelectedId("");
    setCreating(true);
  }

  const saved = (p: P) =>
    void run(async () => {
      await onSaved(p);
      open(p.id);
    });

  const importProfile = () =>
    void run(async () => {
      const p = await onImport();
      if (p) open(p.id);
    });

  const remove = (p: P) =>
    void run(async () => {
      const ok = await confirm({
        title: `Delete profile "${p.name}"?`,
        message: deleteMessage,
        confirmLabel: "Delete profile",
        danger: true,
      });
      if (ok) await onDelete(p.id);
    });

  return (
    <Modal onClose={onClose} className="modal profiles-modal" labelledBy="profiles-modal-title">
      <div className="profiles-modal-head">
        <div className="profiles-modal-head-text">
          <h2 id="profiles-modal-title">Manage Profiles</h2>
          <span className="profiles-modal-sub">
            {creating
              ? "A new profile won't become active until you switch to it."
              : `Editing here won't switch your active connection.${subtitle ? ` ${subtitle}` : ""}`}
          </span>
        </div>
        <button className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">
          ✕
        </button>
      </div>

      <div className="profiles-modal-body">
        <div className="profiles-list">
          <div className="profiles-create-cta">
            <button className="btn btn-primary btn-block" onClick={startCreate} title="Create a new profile">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                strokeWidth="2.4" strokeLinecap="round" aria-hidden="true">
                <path d="M12 5v14M5 12h14" />
              </svg>
              Create new profile
            </button>
            <button className="btn btn-block profiles-import-btn" onClick={importProfile}
              title="Import a profile from a file">
              Import from file…
            </button>
          </div>
          <div className="profiles-list-label">Your profiles</div>
          <ul className="profiles-list-items">
            {profiles.map((p) => (
              <li
                key={p.id}
                className={`profiles-list-row row-stretch${
                  !creating && p.id === selectedId ? " profiles-list-row-selected" : ""
                }`}
              >
                <button
                  className="profiles-star"
                  title={defaultProfileId === p.id ? "Default on launch (click to clear)" : "Set as default on launch"}
                  aria-label={defaultProfileId === p.id ? "Clear default" : "Set as default"}
                  onClick={() => void run(() => onSetDefault(p.id))}
                >
                  {defaultProfileId === p.id ? "★" : "☆"}
                </button>
                <button
                  className="row-select profiles-row-select"
                  aria-current={!creating && p.id === selectedId ? "true" : undefined}
                  onClick={() => open(p.id)}
                >
                  <span className="profiles-row-name">{p.name}</span>
                  <span className="profiles-row-key muted">({p.projectKey})</span>
                </button>
                {p.id === activeId && <span className="profiles-badge-active">Active</span>}
              </li>
            ))}
            {profiles.length === 0 && <li className="muted">No profiles yet.</li>}
          </ul>
          {listNote && <p className="profiles-list-note muted small">{listNote}</p>}
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
              {renderForm({ onSaved: saved, onCancel: () => setCreating(false) })}
            </>
          ) : selected ? (
            <>
              <div className="profiles-mode-head">
                <span className="profiles-mode-kicker profiles-mode-kicker-edit">Editing profile</span>
                <span className="profiles-mode-title">
                  · <span className="mono">{selected.name}</span>
                </span>
              </div>
              {editingActive && (
                <div className="profiles-active-caution">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                    strokeWidth="2" aria-hidden="true">
                    <path d="M12 9v4m0 4h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z" />
                  </svg>
                  <span>
                    <b>This is your active profile.</b> Saving updates the Jira connection you&apos;re using right
                    now. It won&apos;t create a new one.
                  </span>
                </div>
              )}
              {/* Keyed so the form starts fresh for each profile opened. */}
              <Fragment key={selected.id}>
                {renderForm({
                  profile: selected,
                  onSaved: saved,
                  extraActions: (
                    <>
                      <button className="btn" onClick={() => void run(() => onExport(selected.id))}
                        title="Export this profile (without its token)">
                        Export
                      </button>
                      <button className="btn btn-danger" onClick={() => remove(selected)} title="Delete this profile">
                        Delete
                      </button>
                    </>
                  ),
                })}
              </Fragment>
            </>
          ) : (
            <div className="profiles-empty">
              <div className="profiles-empty-mark">
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                  strokeWidth="1.7" aria-hidden="true">
                  <rect x="3" y="4" width="18" height="16" rx="2" />
                  <path d="M3 9h18" />
                  <circle cx="7" cy="6.5" r="0.6" fill="currentColor" />
                </svg>
              </div>
              <h3>Nothing open yet</h3>
              <p>
                Pick a profile on the left to edit it, or create a new one. Your active connection stays as it is
                until you choose.
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
