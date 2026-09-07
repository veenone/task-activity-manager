import { useEffect, useState, type ReactNode } from "react";
import { errMsg } from "@agile-suite/core";
import {
  CreateProfile,
  CreateProfileReusingToken,
  UpdateProfile,
  TestConnection,
  TestProfileConnection,
  GetProfileSetting,
  SetProfileSetting,
  isDemoUrl,
} from "../api";
import type { Profile } from "../api";

// REQUIREMENT_TYPE_KEY is the per-profile setting holding the Jira issue type
// name TAM syncs as a requirement. It lives in tam.db, not on the shared
// profile row, so it is read and written separately from the profile itself.
const REQUIREMENT_TYPE_KEY = "requirement_issue_type";

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

// projectKeyError validates a Jira project key, rejecting trailing slashes,
// spaces, and other invalid characters. Jira DC project keys start with a
// letter and contain letters, digits, and underscores (e.g. RND_P_4TFINT_05);
// any case is accepted and upper-cased on input.
function projectKeyError(key: string): string {
  const k = key.trim();
  if (k === "") return "";
  if (!/^[A-Z][A-Z0-9_]+$/.test(k)) {
    return "Project key must start with a letter and contain only letters, digits, and underscores — no spaces, slashes, or other special characters.";
  }
  return "";
}

// normalizeJiraUrl trims surrounding whitespace and strips trailing slashes so
// the stored base URL is clean: "https://jira.example.com/" or a pasted
// ".../secure/Dashboard.jspa " reduces to the bare base.
export function normalizeJiraUrl(url: string): string {
  return url.trim().replace(/[\s/]+$/, "");
}

// jiraUrlError validates the Jira base URL. A demo URL is allowed, in the
// forms suiteprofiles.IsDemoURL accepts ("demo", "demo:…", "demo-…");
// otherwise it must be a well-formed http(s) URL with a host and no spaces.
function jiraUrlError(url: string): string {
  const u = normalizeJiraUrl(url);
  if (u === "") return "";
  if (isDemoUrl(u)) return "";
  if (/\s/.test(u)) return "The URL must not contain spaces.";
  let parsed: URL;
  try {
    parsed = new URL(u);
  } catch {
    return "Enter a full base URL, e.g. https://jira.example.com (or 'demo').";
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return "The URL must start with http:// or https:// (or be 'demo').";
  }
  if (!parsed.hostname) {
    return "The URL must include a host, e.g. https://jira.example.com.";
  }
  return "";
}

// ProfileForm creates or edits one profile. It mirrors XTM's ProfileForm --
// same fields, same validation, same Test connection button, same advanced
// TLS section -- minus the parts that only mean something to Xray: the
// Kiwi/Xray backend selector, the bug-filing fields, and the cross-project
// sources. Those stay on the saved row untouched (App.UpdateProfile writes
// them back), so a profile edited here still works in XTM. TAM's own
// per-profile requirement issue type is the one field XTM has no equivalent
// for; it is stored in tam.db, so it is loaded and saved around the profile
// write rather than with it.
export function ProfileForm({
  onSaved,
  onCancel,
  profile,
  profiles,
  extraActions,
}: Props) {
  const isEdit = !!profile;
  const others = (profiles ?? []).filter((p) => p.id !== profile?.id);
  const [name, setName] = useState(profile?.name ?? "");
  const [jiraUrl, setJiraUrl] = useState(profile?.jiraUrl ?? "");
  const [projectKey, setProjectKey] = useState(profile?.projectKey ?? "");
  const [scopeJql, setScopeJql] = useState(profile?.scopeJql ?? "");
  const [requirementType, setRequirementType] = useState("");
  const [token, setToken] = useState("");
  const [showToken, setShowToken] = useState(false);
  // Reuse a stored credential from an existing profile (create only). "" =
  // enter a new one below.
  const [reuseFrom, setReuseFrom] = useState("");
  const [caCert, setCaCert] = useState(profile?.caCert ?? "");
  const [allowUntrustedTLS, setAllowUntrustedTLS] = useState(
    profile?.allowUntrustedTls ?? false,
  );

  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState("");
  const [testOk, setTestOk] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  // The requirement type is only readable once the profile exists, so a new
  // profile starts blank and saves the field after its id comes back.
  useEffect(() => {
    if (!profile) return;
    let live = true;
    GetProfileSetting(profile.id, REQUIREMENT_TYPE_KEY)
      .then((v) => live && setRequirementType(v))
      .catch(() => {
        /* a missing setting reads as blank; the placeholder says the default */
      });
    return () => {
      live = false;
    };
  }, [profile]);

  const keyError = projectKeyError(projectKey);
  const urlError = jiraUrlError(jiraUrl);
  const demo = isDemoUrl(jiraUrl);

  // A demo profile needs no credential; a live one needs one on create unless
  // it is reusing another profile's. On edit a blank token keeps the stored one.
  const tokenSatisfied = isEdit || demo || reuseFrom !== "" || token.trim() !== "";
  const canTest =
    jiraUrl.trim() !== "" &&
    urlError === "" &&
    (demo || token.trim() !== "" || isEdit);
  const canSave =
    name.trim() !== "" &&
    jiraUrl.trim() !== "" &&
    urlError === "" &&
    projectKey.trim() !== "" &&
    keyError === "" &&
    tokenSatisfied;

  // Warn when an edit changes the project key or URL: App.UpdateProfile purges
  // the rows cached for the old project, so the next sync starts from nothing.
  const willClearCache =
    isEdit &&
    (projectKey.trim().toUpperCase() !== profile!.projectKey ||
      normalizeJiraUrl(jiraUrl) !== profile!.jiraUrl);

  async function test() {
    setTesting(true);
    setTestResult("");
    setTestOk(false);
    try {
      const user =
        isEdit && token.trim() === ""
          ? // Editing with no new token typed: test the stored one.
            await TestProfileConnection(
              profile!.id,
              normalizeJiraUrl(jiraUrl),
              caCert.trim(),
              allowUntrustedTLS,
            )
          : await TestConnection(
              normalizeJiraUrl(jiraUrl),
              token.trim(),
              caCert.trim(),
              allowUntrustedTLS,
            );
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
      const url = normalizeJiraUrl(jiraUrl);
      const key = projectKey.trim().toUpperCase();
      let p: Profile;
      if (isEdit) {
        p = await UpdateProfile(
          profile!.id, name.trim(), url, key, scopeJql.trim(),
          token.trim(), caCert.trim(), allowUntrustedTLS,
        );
      } else if (reuseFrom !== "") {
        p = await CreateProfileReusingToken(name.trim(), url, key, scopeJql.trim(), reuseFrom);
      } else {
        p = await CreateProfile(
          name.trim(), url, key, scopeJql.trim(),
          token.trim(), caCert.trim(), allowUntrustedTLS,
        );
      }
      // The requirement type lives in TAM's own store, keyed by profile id,
      // so it is written after the profile write hands one back.
      await SetProfileSetting(p.id, REQUIREMENT_TYPE_KEY, requirementType.trim());
      onSaved(p);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="profile-form">
      <label>
        Profile name
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Team — Project X" />
      </label>
      <label>
        Jira base URL
        <input
          value={jiraUrl}
          onChange={(e) => setJiraUrl(e.target.value)}
          onBlur={() => setJiraUrl(normalizeJiraUrl(jiraUrl))}
          placeholder="https://jira.example.com (or 'demo')"
          spellCheck={false}
        />
        {urlError && <span className="field-error">{urlError}</span>}
      </label>
      <label>
        Project key
        <input
          value={projectKey}
          onChange={(e) => setProjectKey(e.target.value.toUpperCase())}
          placeholder="PLAT"
          spellCheck={false}
        />
        {keyError && <span className="field-error">{keyError}</span>}
      </label>
      <label>
        Scope JQL (optional)
        <input
          value={scopeJql}
          onChange={(e) => setScopeJql(e.target.value)}
          placeholder="e.g. labels = team-a (narrows which issues sync)"
        />
      </label>
      <label>
        Requirement issue type (optional)
        <input
          value={requirementType}
          onChange={(e) => setRequirementType(e.target.value)}
          placeholder="Requirement"
          spellCheck={false}
        />
        <span className="field-hint">
          The Jira issue type TAM syncs as a requirement. Leave it blank for
          "Requirement". Changing it resets the sync cursor, so the next sync
          pulls everything again; run a Full sync to drop rows of the old type.
        </span>
      </label>

      {!isEdit && others.length > 0 && (
        <label>
          Personal Access Token
          <select value={reuseFrom} onChange={(e) => setReuseFrom(e.target.value)}>
            <option value="">Enter a new token…</option>
            {others.map((p) => (
              <option key={p.id} value={p.id}>
                Reuse token from: {p.name} ({p.projectKey})
              </option>
            ))}
          </select>
        </label>
      )}

      {(isEdit || reuseFrom === "") && (
        <label>
          {others.length > 0 && !isEdit ? "New token" : "Personal Access Token"}
          <div className="pat-field">
            <input
              type={showToken ? "text" : "password"}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={
                isEdit
                  ? "Leave blank to keep the current token"
                  : demo
                    ? "not needed for a demo profile"
                    : "Jira PAT (stored in Windows Credential Manager)"
              }
              autoComplete="off"
            />
            <button
              type="button"
              className="btn btn-ghost pat-toggle"
              onClick={() => setShowToken((v) => !v)}
              title={showToken ? "Hide token" : "Show token"}
              aria-label={showToken ? "Hide token" : "Show token"}
            >
              {showToken ? "🙈" : "👁"}
            </button>
          </div>
        </label>
      )}

      <details className="profile-form-advanced">
        <summary>Advanced: TLS / certificate settings</summary>
        <label>
          CA certificate (PEM, optional)
          <textarea
            value={caCert}
            onChange={(e) => setCaCert(e.target.value)}
            placeholder={"-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----"}
            rows={5}
            spellCheck={false}
          />
          <span className="field-hint">
            Paste a PEM-encoded CA certificate to trust when connecting to this
            Jira instance. You need this if the server uses a private or
            internal CA that is not in your system's trust store.
          </span>
        </label>
        <label className="profile-form-checkbox">
          <input
            type="checkbox"
            checked={allowUntrustedTLS}
            onChange={(e) => setAllowUntrustedTLS(e.target.checked)}
          />
          Allow untrusted certificate (skip TLS verification)
          <span className="field-hint field-hint-warn">
            Turns off all TLS certificate checks. Only use this for trusted
            internal servers when no CA certificate is available. This is
            insecure and should not be used in production.
          </span>
        </label>
      </details>

      {willClearCache && (
        <div className="form-warning">
          Changing the project key or Jira URL clears this profile's cached
          issues. Re-sync afterwards to pull in the new project.
        </div>
      )}

      <div className="form-actions">
        <button className="btn" onClick={test} disabled={!canTest || testing}>
          {testing ? "Testing…" : "Test connection"}
        </button>
        {testResult && (
          <span className={testOk ? "ok-text" : "error-text"}>{testResult}</span>
        )}
      </div>

      {error && (
        <div className="error-text" role="alert">
          {error}
        </div>
      )}

      <div className="form-actions form-actions-end">
        {extraActions && <div className="profile-form-extra">{extraActions}</div>}
        {onCancel && (
          <button className="btn" onClick={onCancel} disabled={saving}>
            Cancel
          </button>
        )}
        <button className="btn btn-primary" onClick={save} disabled={!canSave || saving}>
          {saving ? "Saving…" : isEdit ? "Save changes" : "Create profile"}
        </button>
      </div>
    </div>
  );
}
