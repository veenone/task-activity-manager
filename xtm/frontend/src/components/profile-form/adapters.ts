import type { ProfileFormAdapter } from "@agile-suite/core";
import { AddConnection, CreateProfile, CreateProfileReusingToken, SetProfileCrossProjectSources, TestConnection, TestProfileConnection, UpdateConnection, UpdateProfile } from "../../api";
import type { Connection, Profile } from "../../api";
import type { XtmProfileExtras } from "./useXtmProfileExtras";
import type { BugTracker } from "./useBugTracker";

// testWith tests the connection on the selected backend, or the stored
// credential of a profile being edited with none typed. TestProfileConnection
// reads the backend from the saved profile.
function testWith(x: XtmProfileExtras, profileId?: string): ProfileFormAdapter["test"] {
  return (i) =>
    i.stored && profileId
      ? TestProfileConnection(profileId, i.url, i.caCert, i.allowTls)
      : TestConnection(i.url, i.token, i.caCert, i.allowTls, x.backend);
}

// profileAdapter saves a profile (create, create reusing another profile's
// credential, or update), then its cross-project sources and Kiwi bug
// tracker, which are separate writes. A bug tracker failure comes back as a
// warning: the profile exists by then, so onCreated still fires and a retry
// updates it rather than creating a second one.
export function profileAdapter(x: XtmProfileExtras, b: BugTracker, profile: Profile | undefined, onCreated?: (p: Profile) => void): ProfileFormAdapter {
  return {
    test: testWith(x, profile?.id),
    save: async (i) => {
      const p = profile
        ? await UpdateProfile(profile.id, i.name, i.url, i.key, i.scope, x.bugIssueType.trim(), x.bugProjectMode, x.bugProjKey, i.token, i.caCert, i.allowTls, x.backend)
        : i.reuseFrom !== ""
          ? await CreateProfileReusingToken(i.name, i.url, i.key, i.scope, x.bugIssueType.trim(), x.bugProjectMode, x.bugProjKey, i.reuseFrom, x.backend, i.caCert, i.allowTls)
          : await CreateProfile(i.name, i.url, i.key, i.scope, x.bugIssueType.trim(), x.bugProjectMode, x.bugProjKey, i.token, i.caCert, i.allowTls, x.backend);
      await SetProfileCrossProjectSources(p.id, x.crossProjectSources.trim());
      const warning = await b.save(p.id);
      onCreated?.(p);
      return warning;
    },
  };
}

// connectionAdapter saves a workspace connection. A connection has no call to
// test its stored credential, so the form always asks for a typed one.
export function connectionAdapter(x: XtmProfileExtras, connection: Connection | undefined, workspaceId: string | undefined, onSaved?: (c: Connection) => void): ProfileFormAdapter {
  return {
    test: testWith(x),
    save: async (i) => {
      const c = connection
        ? await UpdateConnection(connection.id, i.name, x.backend, i.url, i.key, i.scope, x.bugIssueType.trim(), x.bugProjectMode, x.bugProjKey, i.token, i.caCert, i.allowTls, x.role)
        : await AddConnection(workspaceId!, i.name, x.backend, i.url, i.key, i.scope, x.bugIssueType.trim(), x.bugProjectMode, x.bugProjKey, i.token, i.caCert, i.allowTls, x.role);
      onSaved?.(c);
    },
  };
}
