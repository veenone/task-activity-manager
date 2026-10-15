import { useEffect, type ReactNode } from "react";
import { KeyField, NameField, ProfileFormFooter, ScopeField, TlsDetails, TokenFields, UrlField, useProfileForm } from "@agile-suite/core";
import { CreateProfile, CreateProfileReusingToken, UpdateProfile, TestConnection, TestProfileConnection, isDemoUrl } from "../api";
import type { Profile } from "../api";
import { ROOT_FIX_BEFORE_SAVE } from "../lib/confluenceRoot";
import { useTamProfileExtras } from "./profile-form/useTamProfileExtras";
import { TamProfileFields } from "./profile-form/TamProfileFields";
import { ConfluenceFieldset } from "./profile-form/ConfluenceFieldset";

export { DETAIL_MINUTES_NOT_A_NUMBER } from "./profile-form/useTamProfileExtras";

interface Props {
  // Fires when the profile has been created or updated.
  onSaved: (p: Profile) => void;
  onCancel?: () => void;
  // When set, the form edits this profile instead of creating a new one.
  profile?: Profile;
  // The other profiles, which drive the "reuse a stored token" option on create.
  profiles?: Profile[];
  // Extra footer controls (Export / Delete) rendered left of Cancel / Save.
  extraActions?: ReactNode;
}

// ProfileForm creates or edits one profile. The shared fields, their checks,
// and the test and save flow are core's (useProfileForm); what TAM adds is in
// profile-form/: the requirement issue type, the detail freshness and the
// Confluence settings, all stored in tam.db after the profile write. The
// Xray-only fields XTM shows (backend, bug filing, cross-project sources) stay
// on the saved row untouched, because App.UpdateProfile writes them back, so a
// profile edited here still works in XTM.
export function ProfileForm({ onSaved, onCancel, profile, profiles, extraActions }: Props) {
  const isEdit = !!profile;
  const x = useTamProfileExtras(profile);
  const f = useProfileForm<Profile>({
    isDemoUrl,
    isEdit,
    seed: profile && {
      name: profile.name, url: profile.jiraUrl, key: profile.projectKey,
      scope: profile.scopeJql, caCert: profile.caCert, allowTls: profile.allowUntrustedTls,
    },
    checkKeyPattern: x.checkKeyPattern,
    demoNeedsNoToken: true,
    reuseCandidates: (profiles ?? []).filter((p) => p.id !== profile?.id),
    extraValid: x.valid,
    cacheBaseline: profile && { key: profile.projectKey, url: profile.jiraUrl },
    adapter: {
      test: (i) =>
        i.stored
          ? TestProfileConnection(profile!.id, i.url, i.caCert, i.allowTls)
          : TestConnection(i.url, i.token, i.caCert, i.allowTls),
      save: async (i) => {
        const p = isEdit
          ? await UpdateProfile(profile!.id, i.name, i.url, i.key, i.scope, i.token, i.caCert, i.allowTls)
          : i.reuseFrom !== ""
            ? await CreateProfileReusingToken(i.name, i.url, i.key, i.scope, i.reuseFrom)
            : await CreateProfile(i.name, i.url, i.key, i.scope, i.token, i.caCert, i.allowTls);
        await x.saveFor(p.id, isEdit, isDemoUrl(i.url));
        onSaved(p);
      },
    },
  });

  // A new demo profile gets the sample Confluence settings.
  const { fillDemo } = x;
  useEffect(() => {
    if (!profile && f.demo) fillDemo();
  }, [f.demo, profile, fillDemo]);

  return (
    <div className="profile-form">
      <NameField f={f} label="Profile name" placeholder="Team: Project X" />
      <UrlField f={f} label="Jira base URL" placeholder="https://jira.example.com (or 'demo')" />
      <KeyField f={f} label="Project key" placeholder="PLAT" />
      <ScopeField f={f} placeholder="e.g. labels = team-a (narrows which issues sync)" />
      <TamProfileFields x={x} />
      <ConfluenceFieldset x={x} profile={profile} projectKey={f.key} demo={f.demo} />
      <TokenFields
        f={f}
        reuse={{
          label: "Personal Access Token",
          blank: "Enter a new token…",
          option: (p) => `Reuse token from: ${p.name} (${p.projectKey})`,
        }}
      />
      <TlsDetails
        f={f}
        summary="Advanced: TLS / certificate settings"
        hint="Paste a PEM-encoded CA certificate to trust when connecting to this Jira instance. You need this if the server uses a private or internal CA that is not in your system's trust store."
      />
      <ProfileFormFooter
        f={f}
        createLabel="Create profile"
        cacheWarning="Changing the project key or Jira URL clears this profile's cached issues. Re-sync afterwards to pull in the new project."
        onCancel={onCancel}
        extraActions={extraActions}
        beforeError={x.rootInput.error && <div className="field-error">{ROOT_FIX_BEFORE_SAVE}</div>}
      />
    </div>
  );
}
