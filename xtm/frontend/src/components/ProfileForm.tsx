import type { ProfileFormProps } from "./profile-form/useProfileFormState";
import { useProfileFormState } from "./profile-form/useProfileFormState";
import { useProfileFormActions } from "./profile-form/useProfileFormActions";
import { ConnectionFields } from "./profile-form/ConnectionFields";
import { BugTrackerFieldset } from "./profile-form/BugTrackerFieldset";
import { CredentialFields } from "./profile-form/CredentialFields";
import { TlsSettings } from "./profile-form/TlsSettings";
import { FormFooter } from "./profile-form/FormFooter";

// ProfileForm creates or edits a profile, or, in "connection" mode, a
// workspace connection. The hooks in profile-form/ hold its fields and
// validation and its test and save calls; each group of fields is its own
// component there.
export function ProfileForm(p: ProfileFormProps) {
  const s = useProfileFormState(p);
  const a = useProfileFormActions(s, p);
  const { isConnection, isEdit } = s;
  const { hideHeading = false } = p;

  return (
    <div className="profile-form">
      {!hideHeading && (
        <h2>
          {isConnection
            ? isEdit
              ? "Edit connection"
              : "Add connection"
            : isEdit
              ? "Edit profile"
              : "New profile"}
        </h2>
      )}
      <ConnectionFields s={s} />
      <BugTrackerFieldset s={s} />
      <CredentialFields s={s} />

      <TlsSettings s={s} />

      <FormFooter s={s} a={a} p={p} />
    </div>
  );
}
