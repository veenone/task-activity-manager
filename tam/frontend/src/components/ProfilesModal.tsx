import { ProfilesDialog, useNotice, useProfile } from "@agile-suite/core";
import { DeleteProfile, ExportProfile, ImportProfile } from "../api";
import type { Profile, Settings } from "../api";
import { ProfileForm } from "./ProfileForm";

// ProfilesModal is TAM's Manage Profiles: core's ProfilesDialog over the
// profile context and TAM's bindings, with TAM's ProfileForm. The one thing
// TAM says that XTM does not is that the list is shared: a profile made or
// deleted here appears or disappears in XTM too.
export function ProfilesModal({ onClose }: { onClose: () => void }) {
  const { profiles, activeId, defaultProfileId, reload, setDefault } = useProfile<Profile, Settings>();
  const { notice } = useNotice();

  return (
    <ProfilesDialog<Profile>
      profiles={profiles}
      activeId={activeId}
      defaultProfileId={defaultProfileId}
      subtitle="The list is shared with Xray Test Manager."
      listNote="Kiwi TCMS profiles from Xray Test Manager are not listed here; TAM talks to Jira only."
      deleteMessage="This removes the profile from Xray Test Manager too, along with its stored token and every issue TAM cached for it. This cannot be undone."
      onSetDefault={setDefault}
      onExport={async (id) => {
        const path = await ExportProfile(id);
        if (path) notice({ title: "Profile exported", message: `Saved to ${path}` });
      }}
      onImport={async () => {
        const p = await ImportProfile();
        if (!p.id) return null; // the file dialog was cancelled
        await reload();
        notice({
          title: "Profile imported",
          message: "A profile file carries no token. Enter one here before syncing.",
        });
        return p;
      }}
      onSaved={() => reload()}
      onDelete={async (id) => {
        await DeleteProfile(id);
        await reload();
      }}
      onClose={onClose}
      renderForm={({ profile, onSaved, onCancel, extraActions }) => (
        <ProfileForm
          profile={profile}
          profiles={profiles}
          onSaved={onSaved}
          onCancel={onCancel}
          extraActions={extraActions}
        />
      )}
    />
  );
}
