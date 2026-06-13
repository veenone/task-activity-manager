import { useEffect, useState } from "react";
import type { Profile } from "../api";
import { ProfileForm } from "./ProfileForm";

interface Props {
  profiles: Profile[];
  activeId: string;
  defaultProfileId: string;
  onClose: () => void;
  // Toggle the launch-default for a profile (clears it if already default).
  onSetDefault: (id: string) => void;
  // Export a profile's config (no credential) to a file.
  onExport: (id: string) => void;
  // Import a profile from a file; resolves to the created profile or null.
  onImport: () => Promise<Profile | null>;
  // Persist a created/edited profile (replace-or-append + refresh in App).
  onSaved: (p: Profile) => void;
  // Delete a profile after confirming; resolves true if it was deleted.
  onDelete: (id: string) => Promise<boolean>;
}

// ProfilesModal is the master-detail profile manager: a list of every profile
// on the left (with a star to set the launch default) and the selected
// profile's ProfileForm on the right. New / Import create or bring in profiles;
// Export / Delete act on the selected one. App owns the profile state, so every
// mutation flows through the callback props.
export function ProfilesModal({
  profiles,
  activeId,
  defaultProfileId,
  onClose,
  onSetDefault,
  onExport,
  onImport,
  onSaved,
  onDelete,
}: Props) {
  const [selectedId, setSelectedId] = useState(
    activeId || profiles[0]?.id || "",
  );
  const [creating, setCreating] = useState(false);

  // Keep the selection valid: if the selected profile is deleted (and we're not
  // mid-create), fall back to the active profile, else the first one.
  useEffect(() => {
    if (creating) return;
    if (!profiles.some((p) => p.id === selectedId)) {
      setSelectedId(activeId || profiles[0]?.id || "");
    }
  }, [profiles, selectedId, creating, activeId]);

  const selected = profiles.find((p) => p.id === selectedId) ?? null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal profiles-modal"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="profiles-modal-head">
          <h2>Manage Profiles</h2>
          <button className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">
            ✕
          </button>
        </div>

        <div className="profiles-modal-body">
          <div className="profiles-list">
            <ul className="profiles-list-items">
              {profiles.map((p) => (
                <li
                  key={p.id}
                  className={`profiles-list-row${
                    !creating && p.id === selectedId
                      ? " profiles-list-row-selected"
                      : ""
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
                        ? "Default on launch — click to clear"
                        : "Set as default on launch"
                    }
                    aria-label={
                      defaultProfileId === p.id ? "Clear default" : "Set as default"
                    }
                    onClick={(e) => {
                      e.stopPropagation();
                      onSetDefault(p.id);
                    }}
                  >
                    {defaultProfileId === p.id ? "★" : "☆"}
                  </button>
                  <span className="profiles-row-name">{p.name}</span>
                  <span className="profiles-row-key muted">
                    ({p.projectKey})
                  </span>
                </li>
              ))}
            </ul>
            <div className="profiles-list-footer">
              <button
                className="btn"
                onClick={() => setCreating(true)}
                title="Create a new profile"
              >
                + New
              </button>
              <button
                className="btn"
                onClick={async () => {
                  const p = await onImport();
                  if (p) {
                    setCreating(false);
                    setSelectedId(p.id);
                  }
                }}
                title="Import a profile from a file"
              >
                Import…
              </button>
            </div>
          </div>

          <div className="profiles-detail">
            {creating ? (
              <ProfileForm
                profiles={profiles}
                onCreated={(p) => {
                  onSaved(p);
                  setCreating(false);
                  setSelectedId(p.id);
                }}
                onCancel={() => setCreating(false)}
              />
            ) : selected ? (
              <ProfileForm
                key={selected.id}
                profile={selected}
                profiles={profiles}
                onCreated={(p) => {
                  onSaved(p);
                  setSelectedId(p.id);
                }}
                extraActions={
                  <>
                    <button
                      className="btn"
                      onClick={() => onExport(selected.id)}
                      title="Export this profile (without its token)"
                    >
                      Export
                    </button>
                    <button
                      className="btn btn-danger"
                      onClick={() => void onDelete(selected.id)}
                      title="Delete this profile"
                    >
                      Delete
                    </button>
                  </>
                }
              />
            ) : (
              <p className="muted">Select a profile, or create a new one.</p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
