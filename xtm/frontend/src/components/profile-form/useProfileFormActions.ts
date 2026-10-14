import { normalizeJiraUrl } from "@agile-suite/core";
import { CreateProfile, CreateProfileReusingToken, UpdateProfile, SetProfileCrossProjectSources, AddConnection, UpdateConnection, TestConnection, TestProfileConnection, SaveBugConnection, DeleteBugConnection, errMsg } from "../../api";
import type { Profile } from "../../api";
import type { ProfileFormProps, ProfileFormState } from "./useProfileFormState";

// useProfileFormActions tests the connection and saves the form: a profile
// (create, create reusing a token, or update, then its cross-project sources
// and Kiwi bug tracker connection) or a workspace connection.
export function useProfileFormActions(s: ProfileFormState, p: ProfileFormProps) {
  const { confirm, name, backend, jiraUrl, projectKey, scopeJql, crossProjectSources, bugIssueType, bugProjectMode, bugProjectKey, role, reuseFrom, caCert, allowUntrustedTLS, bugConnId, setBugConnId, bugConnLoadedUrl, setBugConnLoadedUrl, bugConnLoadedProjectKey, setBugConnLoadedProjectKey, bugConnLoadedIssueType, setBugConnLoadedIssueType, bugConnLoadedCaCert, setBugConnLoadedCaCert, bugConnLoadedAllowUntrustedTLS, setBugConnLoadedAllowUntrustedTLS, bugTrackerUrl, setBugTrackerUrl, bugTrackerProjectKey, setBugTrackerProjectKey, bugTrackerIssueType, setBugTrackerIssueType, bugTrackerToken, bugTrackerCaCert, setBugTrackerCaCert, bugTrackerAllowUntrustedTLS, setBugTrackerAllowUntrustedTLS, setTesting, setTestResult, setTestOk, setSaving, setError, isConnection, isEdit, backendIsKiwi, currentToken } = s;
  const { onCreated, profile, workspaceId, connection, onSaved } = p;

  async function test() {
    setTesting(true);
    setTestResult("");
    setTestOk(false);
    try {
      let user: string;
      if (!isConnection && isEdit && currentToken() === "") {
        // Editing a profile with no new credential typed: test against the
        // stored one. TestProfileConnection reads the backend type from the
        // saved profile, so it doesn't need it passed in here.
        user = await TestProfileConnection(profile!.id, normalizeJiraUrl(jiraUrl), caCert.trim(), allowUntrustedTLS);
      } else {
        // No saved profile yet (or a new credential was typed), or a
        // connection (always tested with a freshly typed credential) --
        // route through the selected backend so testing a live-Kiwi URL
        // hits the Kiwi backend, not Xray.
        user = await TestConnection(normalizeJiraUrl(jiraUrl), currentToken(), caCert.trim(), allowUntrustedTLS, backend);
      }
      setTestResult(`Connected as ${user}`);
      setTestOk(true);
    } catch (e) {
      setTestResult(errMsg(e));
    } finally {
      setTesting(false);
    }
  }

  async function save() {
    setSaving(true);
    setError("");
    try {
      // Kiwi's "Product" is a free-form name, not a Jira-style project key --
      // don't force it upper-case.
      const key = backendIsKiwi ? projectKey.trim() : projectKey.trim().toUpperCase();
      const bugProjKey =
        bugProjectMode === "dedicated"
          ? bugProjectKey.trim().toUpperCase()
          : "";
      if (isConnection) {
        const c = isEdit
          ? await UpdateConnection(
              connection!.id,
              name.trim(),
              backend,
              normalizeJiraUrl(jiraUrl),
              key,
              scopeJql.trim(),
              bugIssueType.trim(),
              bugProjectMode,
              bugProjKey,
              currentToken(),
              caCert.trim(),
              allowUntrustedTLS,
              role,
            )
          : await AddConnection(
              workspaceId!,
              name.trim(),
              backend,
              normalizeJiraUrl(jiraUrl),
              key,
              scopeJql.trim(),
              bugIssueType.trim(),
              bugProjectMode,
              bugProjKey,
              currentToken(),
              caCert.trim(),
              allowUntrustedTLS,
              role,
            );
        onSaved?.(c);
        return;
      }
      let p: Profile;
      if (isEdit) {
        p = await UpdateProfile(
          profile!.id,
          name.trim(),
          normalizeJiraUrl(jiraUrl),
          key,
          scopeJql.trim(),
          bugIssueType.trim(),
          bugProjectMode,
          bugProjKey,
          currentToken(),
          caCert.trim(),
          allowUntrustedTLS,
          backend,
        );
      } else if (reuseFrom !== "") {
        p = await CreateProfileReusingToken(
          name.trim(),
          normalizeJiraUrl(jiraUrl),
          key,
          scopeJql.trim(),
          bugIssueType.trim(),
          bugProjectMode,
          bugProjKey,
          reuseFrom,
          backend,
          caCert.trim(),
          allowUntrustedTLS,
        );
      } else {
        p = await CreateProfile(
          name.trim(),
          normalizeJiraUrl(jiraUrl),
          key,
          scopeJql.trim(),
          bugIssueType.trim(),
          bugProjectMode,
          bugProjKey,
          currentToken(),
          caCert.trim(),
          allowUntrustedTLS,
          backend,
        );
      }
      // Cross-project source projects are stored via a targeted setter
      // (RND_P_4TFINT_05-322), separate from the main profile write.
      await SetProfileCrossProjectSources(p.id, crossProjectSources.trim());

      // Bug tracker connection (RND_P_4TFINT_05-359): also a targeted write,
      // separate from the main profile write, and only relevant for Kiwi.
      // The profile itself is already saved by this point -- a failure here
      // is reported alongside the success rather than thrown, so onCreated
      // still fires below and a retry updates the existing profile instead
      // of creating a duplicate (review, Important 3).
      if (backendIsKiwi) {
        const trackerUrl = normalizeJiraUrl(bugTrackerUrl);
        const trackerKey = bugTrackerProjectKey.trim();
        try {
          if (trackerUrl !== "" && trackerKey !== "") {
            // A blank token here means "keep the stored one" (or "none yet"
            // on create) -- SaveBugConnection treats that as keep-on-update
            // and requires a real token only when nothing is stored yet.
            const saved = await SaveBugConnection(
              p.id,
              trackerUrl,
              trackerKey,
              bugTrackerIssueType.trim(),
              bugTrackerToken.trim(),
              bugTrackerCaCert.trim(),
              bugTrackerAllowUntrustedTLS,
            );
            setBugConnId(saved.id);
            setBugConnLoadedUrl(saved.url);
            setBugConnLoadedProjectKey(saved.projectKey);
            setBugConnLoadedIssueType(saved.bugIssueType);
            setBugConnLoadedCaCert(saved.caCert);
            setBugConnLoadedAllowUntrustedTLS(saved.allowUntrustedTls);
          } else if (trackerUrl === "" && trackerKey === "" && bugConnId !== "") {
            // Both cleared with a connection on record: this is the one
            // unambiguous "turn bug routing back off" signal (a half-cleared
            // pair is blocked before Save is even enabled -- bugTrackerPartial
            // above). Confirm first: this also deletes the stored credential
            // and can't be undone from here.
            const ok = await confirm({
              title: "Remove the bug tracker connection?",
              message: `This stops bugs from being filed at ${bugConnLoadedUrl} (project ${bugConnLoadedProjectKey}) and deletes its stored credential.`,
              confirmLabel: "Remove connection",
              danger: true,
            });
            if (ok) {
              await DeleteBugConnection(p.id);
              setBugConnId("");
              setBugConnLoadedUrl("");
              setBugConnLoadedProjectKey("");
              setBugConnLoadedIssueType("");
              setBugConnLoadedCaCert("");
              setBugConnLoadedAllowUntrustedTLS(false);
            } else {
              // Declined: the connection survives, so the form must go back
              // to showing what is actually stored rather than the blanks
              // that prompted this confirm -- otherwise the displayed state
              // contradicts reality and the next Save re-prompts for the
              // same deletion.
              setBugTrackerUrl(bugConnLoadedUrl);
              setBugTrackerProjectKey(bugConnLoadedProjectKey);
              setBugTrackerIssueType(bugConnLoadedIssueType);
              setBugTrackerCaCert(bugConnLoadedCaCert);
              setBugTrackerAllowUntrustedTLS(bugConnLoadedAllowUntrustedTLS);
            }
          }
        } catch (e) {
          setError(`Profile saved, but the bug tracker connection could not be saved: ${errMsg(e)}`);
        }
      }
      onCreated?.(p);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }

  return { test, save };
}

export type ProfileFormActions = ReturnType<typeof useProfileFormActions>;
