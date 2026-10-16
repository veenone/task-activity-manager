import type { ReactNode } from "react";
import { KeyField, NameField, ProfileFormFooter, ScopeField, TlsDetails, TokenFields, UrlField, useProfileForm } from "@agile-suite/core";
import type { Connection, Profile } from "../api";
import { isXtmDemoUrl } from "./profile-form/demoUrl";
import { useXtmProfileExtras } from "./profile-form/useXtmProfileExtras";
import { useBugTracker } from "./profile-form/useBugTracker";
import { connectionAdapter, profileAdapter } from "./profile-form/adapters";
import { BackendFields, KiwiCredentialFields, XrayFields } from "./profile-form/XtmFields";
import { BugTrackerFieldset } from "./profile-form/BugTrackerFieldset";

export interface ProfileFormProps {
  // Fires when a profile is created or updated ("profile" mode).
  onCreated?: (p: Profile) => void;
  onCancel?: () => void;
  // When set, the form edits this profile instead of creating one (FR-5).
  profile?: Profile;
  // Existing profiles, which a new profile can start from (FR-5).
  profiles?: Profile[];
  // Extra footer controls (Export / Delete) left of Cancel / Save.
  extraActions?: ReactNode;
  // Leave the heading to a caller that draws its own (Manage Profiles).
  hideHeading?: boolean;
  // "connection" saves a workspace connection through the connection calls
  // and shows a Role select (the Connections manager, P6.3 B6a). The fields
  // are the same in both modes.
  mode?: "profile" | "connection";
  // The workspace a new connection is created in.
  workspaceId?: string;
  // When set, the form edits this connection.
  connection?: Connection;
  // Fires when a connection is created or updated ("connection" mode).
  onSaved?: (c: Connection) => void;
}

// ProfileForm creates or edits a profile, or a workspace connection. The
// shared fields, their checks, and the test and save flow are core's
// (useProfileForm); XTM's backend select, Xray bug-filing fields, Kiwi
// credential and bug tracker are in profile-form/, and the two adapters there
// make the profile and connection calls.
export function ProfileForm(p: ProfileFormProps) {
  const { profile, profiles, connection, mode = "profile", hideHeading = false } = p;
  const isConnection = mode === "connection";
  const isEdit = isConnection ? !!connection : !!profile;
  const seed = profile ?? connection;
  const x = useXtmProfileExtras(profile, connection);
  const b = useBugTracker(profile?.id, x.isKiwi && !isConnection);
  const others = (profiles ?? []).filter((o) => o.id !== profile?.id);
  const f = useProfileForm<Profile>({
    isDemoUrl: isXtmDemoUrl,
    isEdit,
    seed: seed && {
      name: seed.name, url: profile?.jiraUrl ?? connection?.url, key: seed.projectKey,
      scope: seed.scopeJql, caCert: seed.caCert, allowTls: seed.allowUntrustedTls,
    },
    // Kiwi's "Product" is a free-form name, not a Jira key.
    keyMode: x.isKiwi ? "free" : "jira",
    canTestStored: !isConnection,
    // A Jira token and a Kiwi login are not interchangeable.
    reuseCandidates: isConnection ? [] : others.filter((o) => (o.backend === "kiwi") === x.isKiwi),
    credential: x.credential,
    extraValid: b.valid,
    cacheBaseline: !isConnection && profile ? { key: profile.projectKey, url: profile.jiraUrl } : undefined,
    clearsCacheToo: !isConnection && !!profile && x.backend !== profile.backend,
    adapter: isConnection
      ? connectionAdapter(x, connection, p.workspaceId, p.onSaved)
      : profileAdapter(x, b, profile, p.onCreated),
  });

  // Starting from another profile copies its settings, not only its
  // credential; the name stays blank, since it must be new. TLS travels with
  // the URL it was set up for.
  function copyFrom(id: string) {
    const src = others.find((o) => o.id === id);
    if (!src) return;
    f.setUrl(src.jiraUrl ?? "");
    f.setKey(src.projectKey ?? "");
    f.setScope(src.scopeJql ?? "");
    f.setCaCert(src.caCert ?? "");
    f.setAllowTls(src.allowUntrustedTls ?? false);
    x.copyFrom(src);
  }

  return (
    <div className="profile-form">
      {!hideHeading && (
        <h2>{isConnection ? (isEdit ? "Edit connection" : "Add connection") : isEdit ? "Edit profile" : "New profile"}</h2>
      )}
      <NameField
        f={f}
        label={isConnection ? "Connection name" : "Profile name"}
        placeholder={isConnection ? "Kiwi target" : "QA: Project X"}
      />
      <BackendFields x={x} isConnection={isConnection} onBackendChange={() => f.setReuseFrom("")} />
      <UrlField
        f={f}
        label={x.isKiwi ? "Kiwi server URL" : "Jira base URL"}
        placeholder={x.isKiwi ? "https://kiwi.example.com" : "https://jira.example.com"}
      />
      <KeyField f={f} label={x.isKiwi ? "Product" : "Project key"} placeholder={x.isKiwi ? "e.g. MyProduct" : "QA"} />
      {!x.isKiwi && <ScopeField f={f} placeholder="e.g. labels = smoke (narrows which tests sync)" />}
      <XrayFields x={x} isConnection={isConnection} />
      <BugTrackerFieldset b={b} active={x.isKiwi && !isConnection} />
      <TokenFields
        f={f}
        reuse={{
          label: "Start from",
          blank: "A blank profile",
          option: (o) => `Copy of: ${o.name} (${o.projectKey})`,
          hint: `Copies the settings below and reuses the stored ${x.isKiwi ? "credential" : "token"}. Everything stays editable.`,
        }}
        onReuse={copyFrom}
        credential={x.isKiwi ? <KiwiCredentialFields x={x} isEdit={isEdit} /> : undefined}
      />
      <TlsDetails
        f={f}
        summary="Advanced: TLS / certificate settings (this connection)"
        hint="Paste a PEM-encoded CA certificate to trust when connecting to this Jira instance. You'll need this if the server uses a private or internal CA that isn't in your system's trust store (for example, a corporate CA on macOS)."
      />
      <ProfileFormFooter
        f={f}
        createLabel={isConnection ? "Add connection" : "Create profile"}
        cacheWarning="Changing the project key or Jira URL clears this profile's cached data. Re-sync afterwards to pull in the new project."
        onCancel={p.onCancel}
        extraActions={p.extraActions}
      />
    </div>
  );
}
