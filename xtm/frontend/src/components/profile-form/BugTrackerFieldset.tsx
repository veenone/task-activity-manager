import { normalizeJiraUrl } from "@agile-suite/core";
import type { ProfileFormState } from "./useProfileFormState";

// BugTrackerFieldset is the Jira connection a Kiwi profile files bugs into.
export function BugTrackerFieldset({ s }: { s: ProfileFormState }) {
  const { bugConnId, bugTrackerUrl, setBugTrackerUrl, bugTrackerProjectKey, setBugTrackerProjectKey, bugTrackerIssueType, setBugTrackerIssueType, bugTrackerToken, setBugTrackerToken, bugTrackerCaCert, setBugTrackerCaCert, bugTrackerAllowUntrustedTLS, setBugTrackerAllowUntrustedTLS, bugTrackerLoadError, isConnection, backendIsKiwi, bugTrackerUrlError, bugTrackerPartial } = s;
  return (
    <>
      {backendIsKiwi && !isConnection && (
        <fieldset className="profile-bug-tracker">
          <legend>Bug tracker</legend>
          <p className="field-hint">
            Kiwi has no issue type of its own, so defects raised here are
            filed into a Jira project instead. Leave this empty to keep bug
            reporting off.
          </p>
          {bugTrackerLoadError && (
            <div className="error-text">
              Could not load the saved bug tracker connection:{" "}
              {bugTrackerLoadError}. Saving now could overwrite it -- fix the
              connection (or toggle Backend off and back on to Kiwi to retry)
              before saving.
            </div>
          )}
          <label>
            Bug tracker URL
            <input
              value={bugTrackerUrl}
              onChange={(e) => setBugTrackerUrl(e.target.value)}
              onBlur={() => setBugTrackerUrl(normalizeJiraUrl(bugTrackerUrl))}
              placeholder="https://jira.example.com"
              spellCheck={false}
            />
            {bugTrackerUrlError && (
              <span className="field-error">{bugTrackerUrlError}</span>
            )}
          </label>
          <label>
            Bug project key
            <input
              value={bugTrackerProjectKey}
              onChange={(e) => setBugTrackerProjectKey(e.target.value.toUpperCase())}
              placeholder="e.g. DEF"
              spellCheck={false}
            />
          </label>
          {bugTrackerPartial && (
            <span className="field-error">
              Enter both a bug tracker URL and a project key, or clear both to
              leave bug reporting off.
            </span>
          )}
          <label>
            Bug issue type
            <input
              value={bugTrackerIssueType}
              onChange={(e) => setBugTrackerIssueType(e.target.value)}
              placeholder="Bug"
              spellCheck={false}
            />
          </label>
          <label>
            Bug tracker token
            <input
              type="password"
              autoComplete="new-password"
              value={bugTrackerToken}
              onChange={(e) => setBugTrackerToken(e.target.value)}
              placeholder={
                bugConnId
                  ? "Leave blank to keep the current token"
                  : "Personal access token"
              }
            />
          </label>
          <span className="field-hint">
            Stored in Windows Credential Manager, never in the database.
            Leave it blank to keep the one already saved.
          </span>

          <details className="profile-form-advanced">
            <summary>Advanced: TLS / certificate settings (bug tracker)</summary>
            <label>
              CA certificate (PEM, optional)
              <textarea
                value={bugTrackerCaCert}
                onChange={(e) => setBugTrackerCaCert(e.target.value)}
                placeholder={"-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----"}
                rows={5}
                spellCheck={false}
              />
              <span className="field-hint">
                Paste a PEM-encoded CA certificate to trust when connecting to this Jira
                instance. You'll need this if the server uses a private or internal CA
                that isn't in your system's trust store (for example, a corporate CA on
                macOS).
              </span>
            </label>
            <label className="profile-form-checkbox">
              <input
                type="checkbox"
                checked={bugTrackerAllowUntrustedTLS}
                onChange={(e) => setBugTrackerAllowUntrustedTLS(e.target.checked)}
              />
              Allow untrusted certificate (skip TLS verification)
              <span className="field-hint field-hint-warn">
                Turns off all TLS certificate checks. Only use this for trusted
                internal servers when no CA certificate is available. This is insecure
                and should not be used in production.
              </span>
            </label>
          </details>
        </fieldset>
      )}
    </>
  );
}
