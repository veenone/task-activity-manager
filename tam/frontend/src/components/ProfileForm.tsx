import { useEffect, useId, useState, type ReactNode } from "react";
import { errMsg } from "@agile-suite/core";
import {
  CreateProfile,
  CreateProfileReusingToken,
  UpdateProfile,
  TestConnection,
  TestProfileConnection,
  GetProfileSetting,
  SetProfileSetting,
  GetConfluenceConfig,
  SetConfluenceConfig,
  CreateReportRoot,
  isDemoUrl,
} from "../api";
import type { Profile } from "../api";
import { ROOT_FIX_BEFORE_SAVE, readRootPageInput } from "../lib/confluenceRoot";
import { ReportRootDialog } from "./ReportRootDialog";

// REQUIREMENT_TYPE_KEY is the per-profile setting holding the Jira issue type
// name TAM syncs as a requirement. It lives in tam.db, not on the shared
// profile row, so it is read and written separately from the profile itself.
const REQUIREMENT_TYPE_KEY = "requirement_issue_type";

// DETAIL_CACHE_KEY is the per-profile setting holding how long a cached issue
// detail is served before TAM asks Jira again. It lives in tam.db beside the
// requirement type and is read and written the same way.
const DETAIL_CACHE_KEY = "detail_cache_minutes";

export const DETAIL_MINUTES_NOT_A_NUMBER =
  "Minutes is a whole number, such as 10. Use 0 to keep issue details cached until a Refresh.";

// detailMinutesError checks the freshness field. Blank is the default (ten
// minutes) and 0 is the offline setting, so the only thing refused is a value
// that is not a whole number of minutes.
function detailMinutesError(value: string): string {
  const v = value.trim();
  if (v === "" || /^\d+$/.test(v)) return "";
  return DETAIL_MINUTES_NOT_A_NUMBER;
}

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
  const [detailMinutes, setDetailMinutes] = useState("");
  const [token, setToken] = useState("");
  const [showToken, setShowToken] = useState(false);
  // Reuse a stored credential from an existing profile (create only). "" =
  // enter a new one below.
  const [reuseFrom, setReuseFrom] = useState("");
  const [caCert, setCaCert] = useState(profile?.caCert ?? "");
  const [allowUntrustedTLS, setAllowUntrustedTLS] = useState(
    profile?.allowUntrustedTls ?? false,
  );
  const [confluenceURL, setConfluenceURL] = useState("");
  const [confluenceSpace, setConfluenceSpace] = useState("");
  const [confluenceRootPageID, setConfluenceRootPageID] = useState("");
  const [confluenceToken, setConfluenceToken] = useState("");
  const [confluenceLoaded, setConfluenceLoaded] = useState(false);
  // Where sprint reports are published. Both blank is what every profile had
  // before these existed, and it publishes where it always did: the rituals
  // space, under the sprint's own page or the rituals root.
  const [reportsSpace, setReportsSpace] = useState("");
  const [reportsRootPageID, setReportsRootPageID] = useState("");
  const [pickingReportsRoot, setPickingReportsRoot] = useState(false);

  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState("");
  const [testOk, setTestOk] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const demo = isDemoUrl(jiraUrl);

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
    GetProfileSetting(profile.id, DETAIL_CACHE_KEY)
      .then((v) => live && setDetailMinutes(v))
      .catch(() => {
        /* same: blank is the default freshness */
      });
    return () => {
      live = false;
    };
  }, [profile]);

  useEffect(() => {
    if (profile || !demo) return;
    setConfluenceURL((value) => value || "demo");
    setConfluenceSpace((value) => value || "DEMO");
    setConfluenceRootPageID((value) => value || "demo-root");
  }, [demo, profile]);

  useEffect(() => {
    if (!profile) return;
    let live = true;
    Promise.resolve()
      .then(() => GetConfluenceConfig(profile.id))
      .then((c) => {
        if (!live) return;
        setConfluenceURL(c.baseURL);
        setConfluenceSpace(c.spaceKey);
        setConfluenceRootPageID(c.rootPageID);
        setReportsSpace(c.reportsSpaceKey ?? "");
        setReportsRootPageID(c.reportsRootPageID ?? "");
        setConfluenceLoaded(true);
      })
      .catch(() => { /* optional integration remains blank when unavailable */ });
    return () => { live = false; };
  }, [profile]);

  const keyError = projectKeyError(projectKey);
  const urlError = jiraUrlError(jiraUrl);
  // Each field error is the description of its input, so a screen reader
  // reads it with the field rather than only where it sits on the page.
  const urlErrorId = useId();
  const keyErrorId = useId();
  const rootErrorId = useId();
  const reportsRootId = useId();
  const minutesErrorId = useId();
  const minutesError = detailMinutesError(detailMinutes);

  // A Sync against a root page id that is not one answers 404 and offers to
  // create a root nobody needed, so the id is checked where it is typed.
  const rootInput = readRootPageInput(confluenceRootPageID, confluenceURL);

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
    tokenSatisfied &&
    rootInput.error === "" &&
    minutesError === "";

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
      await SetProfileSetting(p.id, DETAIL_CACHE_KEY, detailMinutes.trim());
      if ((isEdit && confluenceLoaded) || ((!demo && (confluenceURL.trim() || confluenceSpace.trim() || confluenceRootPageID.trim() || reportsSpace.trim())) || confluenceToken.trim())) {
        await SetConfluenceConfig(p.id, {
          baseURL: confluenceURL.trim(),
          spaceKey: confluenceSpace.trim(),
          rootPageID: rootInput.id,
          reportsSpaceKey: reportsSpace.trim(),
          reportsRootPageID: reportsRootPageID.trim(),
        }, confluenceToken.trim());
        setConfluenceURL(confluenceURL.trim().replace(/\/+$/, ""));
        setConfluenceSpace(confluenceSpace.trim());
        setConfluenceRootPageID(rootInput.id);
        setReportsSpace(reportsSpace.trim());
        setConfluenceLoaded(true);
      }
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
          aria-invalid={urlError ? true : undefined}
          aria-describedby={urlError ? urlErrorId : undefined}
        />
        {urlError && <span id={urlErrorId} className="field-error">{urlError}</span>}
      </label>
      <label>
        Project key
        <input
          value={projectKey}
          onChange={(e) => setProjectKey(e.target.value.toUpperCase())}
          placeholder="PLAT"
          spellCheck={false}
          aria-invalid={keyError ? true : undefined}
          aria-describedby={keyError ? keyErrorId : undefined}
        />
        {keyError && <span id={keyErrorId} className="field-error">{keyError}</span>}
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
      <label>
        Issue detail freshness (minutes)
        <input
          value={detailMinutes}
          onChange={(e) => setDetailMinutes(e.target.value)}
          placeholder="10"
          inputMode="numeric"
          spellCheck={false}
          aria-invalid={minutesError ? true : undefined}
          aria-describedby={minutesError ? minutesErrorId : undefined}
        />
        {minutesError && <span id={minutesErrorId} className="field-error">{minutesError}</span>}
        <span className="field-hint">
          How long a cached issue detail is shown before TAM asks Jira for it
          again. Leave it blank for 10 minutes. 0 keeps issue details cached
          until you press Refresh, so TAM can be used with no Jira connection.
        </span>
      </label>

      <details className="profile-form-advanced">
        <summary>Confluence Rituals (optional)</summary>
        <label>
          Confluence base URL
          <input value={confluenceURL} onChange={(e) => setConfluenceURL(e.target.value)} placeholder="https://confluence.example.com" spellCheck={false} />
        </label>
        <label>
          Space key
          <input value={confluenceSpace} onChange={(e) => setConfluenceSpace(e.target.value)} placeholder="TEAM" spellCheck={false} />
        </label>
        <label>
          Root page ID (optional)
          <input
            value={confluenceRootPageID}
            onChange={(e) => setConfluenceRootPageID(e.target.value)}
            onBlur={() => {
              if (rootInput.id && !rootInput.error) setConfluenceRootPageID(rootInput.id);
            }}
            placeholder="123456, or paste the page's address"
            spellCheck={false}
            aria-invalid={rootInput.error ? true : undefined}
            aria-describedby={rootInput.error ? rootErrorId : undefined}
          />
          {rootInput.error && <span id={rootErrorId} className="field-error">{rootInput.error}</span>}
        </label>
        <label>
          Confluence personal access token
          <input type="password" value={confluenceToken} onChange={(e) => setConfluenceToken(e.target.value)} placeholder={isEdit ? "Leave blank to keep the current token" : "Stored in Windows Credential Manager"} autoComplete="off" />
        </label>
        {/* Where sprint reports go. A report is not a ritual and may belong in
            another space or under another page, so it has its own pair. Both
            left blank publishes where the profile always did. */}
        <label>
          Reports space key (optional)
          <input
            value={reportsSpace}
            onChange={(e) => setReportsSpace(e.target.value)}
            placeholder="The rituals space above"
            spellCheck={false}
          />
        </label>
        <label htmlFor={reportsRootId}>Reports root page (optional)</label>
        <div className="reports-root-field">
          <input
            id={reportsRootId}
            value={reportsRootPageID}
            readOnly
            spellCheck={false}
            placeholder="The sprint's own ritual page"
          />
          <button
            type="button"
            className="btn"
            disabled={!isEdit || !confluenceLoaded}
            title={isEdit ? "Create or take the page reports hang under" : "Save the profile first, then choose the page"}
            onClick={() => setPickingReportsRoot(true)}
          >
            Choose page...
          </button>
          {reportsRootPageID && (
            <button type="button" className="btn" onClick={() => setReportsRootPageID("")}>Clear</button>
          )}
        </div>
        <span className="field-hint">
          Leave both blank and a report is published where it always was: the rituals space, under the sprint's
          own page or the rituals root.
        </span>
        {demo && <span className="field-hint">Demo profile sample: use <code>demo</code> as the Confluence URL. It is a local configuration example and does not contact a server.</span>}
      </details>
      {pickingReportsRoot && profile && (
        <ReportRootDialog
          spaceKey={reportsSpace.trim() || confluenceSpace.trim()}
          suggestedTitle={`${projectKey.trim().toUpperCase()} Reports`.trim()}
          create={(title, adopt) => CreateReportRoot(profile.id, reportsSpace.trim(), title, adopt)}
          onChosen={(root) => {
            setReportsRootPageID(root.pageId);
            setPickingReportsRoot(false);
          }}
          onClose={() => setPickingReportsRoot(false)}
        />
      )}

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

      {rootInput.error && <div className="field-error">{ROOT_FIX_BEFORE_SAVE}</div>}

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
