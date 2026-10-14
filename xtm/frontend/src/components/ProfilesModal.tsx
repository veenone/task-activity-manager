import { ProfilesDialog } from "@agile-suite/core";
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
  // Delete a profile; the dialog has already asked for confirmation.
  onDelete: (id: string) => Promise<void>;
}

// ProfilesModal is XTM's Manage Profiles: core's ProfilesDialog with XTM's
// ProfileForm. App owns the profile state, so every change goes through the
// callbacks it passes.
export function ProfilesModal({ profiles, activeId, defaultProfileId, onClose, onSetDefault, onExport,
  onImport, onSaved, onDelete }: Props) {
  return (
    <ProfilesDialog<Profile>
      profiles={profiles}
      activeId={activeId}
      defaultProfileId={defaultProfileId}
      deleteMessage="This removes its stored token and all cached test data. This cannot be undone."
      onSetDefault={onSetDefault}
      onExport={onExport}
      onImport={onImport}
      onSaved={onSaved}
      onDelete={onDelete}
      onClose={onClose}
      renderForm={({ profile, onSaved: saved, onCancel, extraActions }) => (
        <ProfileForm
          hideHeading
          profile={profile}
          profiles={profiles}
          onCreated={saved}
          onCancel={onCancel}
          extraActions={extraActions}
        />
      )}
    />
  );
}
