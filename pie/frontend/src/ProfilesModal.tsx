import { useContext } from "react";
import { ProfilesDialog } from "@agile-suite/core";
import { DeleteProfile, ExportProfile, ImportProfile, SetDefaultProfile } from "../wailsjs/go/main/App";
import type { profile } from "../wailsjs/go/models";
import { ProfileForm } from "./ProfileForm";
import { NoticeContext } from "./StatusBar";

// ProfilesModal is PIE's Manage Profiles (#453): core's ProfilesDialog, as in
// TAM and XTM, with PIE's form. App owns the list; reload rereads it.
export function ProfilesModal({
  profiles,
  activeId,
  defaultId,
  onDefault,
  reload,
  onClose,
}: {
  profiles: profile.Profile[];
  activeId: string;
  defaultId: string;
  onDefault: (id: string) => void;
  reload: () => Promise<void>;
  onClose: () => void;
}) {
  const notify = useContext(NoticeContext);
  return (
    <ProfilesDialog<profile.Profile>
      profiles={profiles}
      activeId={activeId}
      defaultProfileId={defaultId}
      deleteMessage="This removes its stored tokens, saved queries, cases and cycles from this computer. This cannot be undone."
      onSetDefault={async (id) => {
        // The star toggles: choosing the default again clears it.
        const next = id === defaultId ? "" : id;
        await SetDefaultProfile(next);
        onDefault(next);
      }}
      onExport={async (id) => {
        const path = await ExportProfile(id);
        if (path) notify({ text: `Profile saved to ${path}` });
      }}
      onImport={async () => {
        const p = await ImportProfile();
        if (!p.id) return null; // the file dialog was cancelled
        await reload();
        notify({ text: "Profile imported. A profile file carries no token, so enter one before running a query." });
        return p;
      }}
      onSaved={reload}
      onDelete={async (id) => {
        await DeleteProfile(id);
        await reload();
      }}
      onClose={onClose}
      renderForm={(slot) => <ProfileForm {...slot} />}
    />
  );
}
