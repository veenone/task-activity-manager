import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { jiraUrlError, normalizeJiraUrl, projectKeyError } from "@agile-suite/core";
import { GetBugConnection, errMsg } from "../../api";
import type { Connection, Profile } from "../../api";
import { useConfirm } from "../useConfirm";

export interface ProfileFormProps {
  // Fires when a profile is created/updated. Required in "profile" mode
  // (the default); unused in "connection" mode.
  onCreated?: (p: Profile) => void;
  onCancel?: () => void;
  // When set, the form edits this profile instead of creating a new one (FR-5).
  profile?: Profile;
  // Existing profiles — drives the "reuse token" option when creating (FR-5).
  profiles?: Profile[];
  // Optional extra footer controls (e.g. Export / Delete) rendered left of
  // Cancel / Save. Used by the Manage Profiles modal.
  extraActions?: ReactNode;
  // Suppress the form's own heading when the caller renders its own mode
  // header above the form (the Manage Profiles modal), avoiding a double title.
  hideHeading?: boolean;

  // mode selects which entity the form saves to. "profile" (default) keeps
  // the existing single-connection Manage Profiles behavior unchanged.
  // "connection" reuses the same backend-selector + Kiwi relabel + TLS
  // fields (P6.1b) but saves via the connection App methods, scoped to a
  // workspace, and shows a Role selector — the Connections manager (P6.3
  // B6a). Do not fork the field UI: everything below the mode branches is
  // shared between both.
  mode?: "profile" | "connection";
  // The workspace a new connection is created in ("connection" mode, create).
  workspaceId?: string;
  // When set, the form edits this connection instead of creating a new one
  // ("connection" mode).
  connection?: Connection;
  // Fires when a connection is created/updated ("connection" mode).
  onSaved?: (c: Connection) => void;
}

// isXtmDemoUrl is what XTM accepts as a demo base URL: "demo", a "demo:" or
// "mock:" prefix, a "demo-" variant naming a built-in dataset (demo-pkcs), and
// the offline Kiwi demo in the same forms. Keep in sync with isDemoURL in
// internal/jira/demo.go and kiwi.IsKiwiDemoURL.
const isXtmDemoUrl = (u: string) => /^(demo|demo[-:].*|mock:.*|kiwi-demo|kiwi-demo[-:].*)$/i.test(u);

// useProfileFormState holds the form's fields for a profile or a connection,
// the Kiwi bug tracker connection it loads once per profile, and the derived
// validation that gates Test and Save. copyFrom seeds the fields from another
// profile; currentToken is the credential the save and test calls send.
export function useProfileFormState(p: ProfileFormProps) {
  const { profile, profiles, mode = "profile", connection } = p;
  const { confirm } = useConfirm();
  const isConnection = mode === "connection";
  const isEdit = isConnection ? !!connection : !!profile;
  const others = (profiles ?? []).filter((p) => p.id !== profile?.id);
  const [name, setName] = useState(profile?.name ?? connection?.name ?? "");
  // backend selects which system the profile/connection connects to: "xray"
  // (default, Jira Data Center + Xray Server/DC) or "kiwi" (Kiwi TCMS).
  // Seeded from the saved entity on edit; new ones default to Xray so
  // existing behavior is unchanged unless the user explicitly picks Kiwi.
  const seedBackend = (profile ?? connection)?.backend;
  const [backend, setBackend] = useState(seedBackend === "kiwi" ? "kiwi" : "xray");
  const backendIsKiwi = backend === "kiwi";
  // Only offer to reuse a credential from a profile on the same backend --
  // an Xray PAT and a Kiwi "username:password" string aren't interchangeable.
  // Reuse-from is a profile-only convenience; connections are created one at
  // a time in a modal that doesn't carry a sibling-connections list.
  const reusable = others.filter((p) => (p.backend === "kiwi") === backendIsKiwi);
  const [jiraUrl, setJiraUrl] = useState(profile?.jiraUrl ?? connection?.url ?? "");
  const [projectKey, setProjectKey] = useState(profile?.projectKey ?? connection?.projectKey ?? "");
  const [scopeJql, setScopeJql] = useState(profile?.scopeJql ?? connection?.scopeJql ?? "");
  const [crossProjectSources, setCrossProjectSources] = useState(
    profile?.crossProjectSources ?? "",
  );
  const [bugIssueType, setBugIssueType] = useState(profile?.bugIssueType ?? connection?.bugIssueType ?? "");
  const [bugProjectMode, setBugProjectMode] = useState(
    profile?.bugProjectMode || connection?.bugProjectMode || "test",
  );
  const [bugProjectKey, setBugProjectKey] = useState(
    profile?.bugProjectKey ?? connection?.bugProjectKey ?? "",
  );
  // role only applies in connection mode: "source" | "target" | "both". New
  // connections default to "target" — the common bridge case is adding a
  // target beside the workspace's existing (primary, "both") connection.
  const [role, setRole] = useState(connection?.role || "target");
  const [token, setToken] = useState("");
  // Kiwi's credential is a session-login username + password, combined into a
  // single "username:password" string and passed as the existing token
  // parameter (internal/backend/kiwi splits on the first ":"). Two separate
  // fields so the user isn't asked to hand-craft the combined string.
  const [kiwiUsername, setKiwiUsername] = useState("");
  const [kiwiPassword, setKiwiPassword] = useState("");
  const [showToken, setShowToken] = useState(false);
  // Start a new profile from an existing one (create only). "" = start blank.
  // Selecting a source copies its settings AND reuses its stored credential,
  // so a profile that differs in a field or two is a pick plus one edit.
  const [reuseFrom, setReuseFrom] = useState("");
  const [caCert, setCaCert] = useState(profile?.caCert ?? connection?.caCert ?? "");
  const [allowUntrustedTLS, setAllowUntrustedTLS] = useState(
    profile?.allowUntrustedTls ?? connection?.allowUntrustedTls ?? false,
  );

  // Bug tracker connection (Kiwi profiles only): routes defects into a Jira
  // project since Kiwi has no issue type of its own (RND_P_4TFINT_05-359).
  // Distinct from bugIssueType/bugProjectMode/bugProjectKey above, which only
  // apply to a non-Kiwi backend filing bugs into its own Jira instance.
  // bugConnId is "" until an existing connection loads; the profile form is
  // the only place this is configured, so it's skipped entirely in
  // "connection" mode.
  const [bugConnId, setBugConnId] = useState("");
  // bugConnLoaded* mirror exactly what was last loaded/saved from the
  // backend -- kept separate from the editable fields below so (a) a confirm
  // dialog can name the connection being removed even after the user has
  // cleared the visible URL/project key fields, and (b) declining that
  // confirm can restore the form to what is actually stored, instead of
  // leaving it showing blanks that contradict the surviving connection.
  const [bugConnLoadedUrl, setBugConnLoadedUrl] = useState("");
  const [bugConnLoadedProjectKey, setBugConnLoadedProjectKey] = useState("");
  const [bugConnLoadedIssueType, setBugConnLoadedIssueType] = useState("");
  const [bugConnLoadedCaCert, setBugConnLoadedCaCert] = useState("");
  const [bugConnLoadedAllowUntrustedTLS, setBugConnLoadedAllowUntrustedTLS] = useState(false);
  const [bugTrackerUrl, setBugTrackerUrl] = useState("");
  const [bugTrackerProjectKey, setBugTrackerProjectKey] = useState("");
  const [bugTrackerIssueType, setBugTrackerIssueType] = useState("");
  const [bugTrackerToken, setBugTrackerToken] = useState("");
  const [bugTrackerCaCert, setBugTrackerCaCert] = useState("");
  const [bugTrackerAllowUntrustedTLS, setBugTrackerAllowUntrustedTLS] = useState(false);
  // Set when GetBugConnection itself fails (as opposed to "nothing
  // configured", which resolves normally with a zero Connection). Left
  // visible in the section and blocks Save -- an unreadable connection must
  // never be mistaken for an unconfigured one, or Save would silently
  // overwrite it (RND_P_4TFINT_05-359 review, Important 2).
  const [bugTrackerLoadError, setBugTrackerLoadError] = useState("");

  // Load any bug connection already saved for this profile, once per
  // profile id -- not on every Backend-dropdown toggle, which would
  // overwrite whatever the user had already typed into the section
  // (RND_P_4TFINT_05-359 review, minor 2). A new (unsaved) profile has no id
  // to look one up under yet, and GetBugConnection returns a zero Connection
  // (never an error) when none is configured, which is the normal case.
  const loadedBugConnFor = useRef<string | null>(null);
  useEffect(() => {
    if (isConnection || !backendIsKiwi || !profile) return;
    if (loadedBugConnFor.current === profile.id) return;
    loadedBugConnFor.current = profile.id;
    let cancelled = false;
    GetBugConnection(profile.id)
      .then((c) => {
        if (cancelled) return;
        setBugConnId(c.id);
        setBugConnLoadedUrl(c.url);
        setBugConnLoadedProjectKey(c.projectKey);
        setBugConnLoadedIssueType(c.bugIssueType);
        setBugConnLoadedCaCert(c.caCert);
        setBugConnLoadedAllowUntrustedTLS(c.allowUntrustedTls);
        setBugTrackerUrl(c.url);
        setBugTrackerProjectKey(c.projectKey);
        setBugTrackerIssueType(c.bugIssueType);
        setBugTrackerCaCert(c.caCert);
        setBugTrackerAllowUntrustedTLS(c.allowUntrustedTls);
        setBugTrackerLoadError("");
      })
      .catch((e) => {
        if (cancelled) return;
        // Allow a retry: an id that never loaded shouldn't stay "sticky".
        loadedBugConnFor.current = null;
        setBugTrackerLoadError(errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [isConnection, backendIsKiwi, profile?.id]);

  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState("");
  const [testOk, setTestOk] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  // Kiwi has no Jira-style project key -- "Product" is a free-form name, not
  // validated against the strict key format.
  const keyError = backendIsKiwi ? "" : projectKeyError(projectKey);
  const urlError = jiraUrlError(jiraUrl, isXtmDemoUrl);

  // Bug tracker validation. Reuses jiraUrlError -- the bug tracker's target
  // is always a Jira instance, same as the main URL field above.
  const bugTrackerUrlError =
    backendIsKiwi && !isConnection ? jiraUrlError(bugTrackerUrl, isXtmDemoUrl) : "";
  // Exactly one of URL/project key filled is an edit in progress, not a
  // removal request -- block Save rather than guess what the user meant
  // (RND_P_4TFINT_05-359 review, Important 1).
  const bugTrackerPartial =
    backendIsKiwi &&
    !isConnection &&
    (bugTrackerUrl.trim() !== "") !== (bugTrackerProjectKey.trim() !== "");

  const kiwiUser = kiwiUsername.trim();
  const kiwiPass = kiwiPassword.trim();
  const kiwiCredComplete = kiwiUser !== "" && kiwiPass !== "";
  // Exactly one of username/password filled is never a valid credential --
  // catches a partial edit before it's combined into "username:password".
  // Gated on backendIsKiwi: leftover Kiwi username/password state from a
  // backend-selector round-trip must not deadlock the (now hidden) Xray form.
  const kiwiCredInvalid = backendIsKiwi && (kiwiUser !== "" || kiwiPass !== "") && !kiwiCredComplete;

  // currentToken is the value sent as the App methods' token/credential
  // parameter: the raw PAT for Xray, or the combined "username:password" for
  // Kiwi. "" means "leave the stored credential unchanged" on edit.
  function currentToken(): string {
    if (backendIsKiwi) {
      return kiwiCredComplete ? `${kiwiUser}:${kiwiPass}` : "";
    }
    return token.trim();
  }

  const credEnteredForCreate = backendIsKiwi ? kiwiCredComplete : token.trim() !== "";
  // A credential is needed for create unless reusing one from another
  // profile; on edit a blank credential keeps the stored one.
  const tokenSatisfied = isEdit || reuseFrom !== "" || credEnteredForCreate;
  // copyFrom fills the form from an existing profile. The name is left alone:
  // it must be new, and it is the one field the user always has to supply.
  // Everything stays editable, so this is a starting point rather than a lock.
  function copyFrom(sourceId: string) {
    setReuseFrom(sourceId);
    if (sourceId === "") return;
    const src = others.find((p) => p.id === sourceId);
    if (!src) return;
    setJiraUrl(src.jiraUrl ?? "");
    setProjectKey(src.projectKey ?? "");
    setScopeJql(src.scopeJql ?? "");
    setCrossProjectSources(src.crossProjectSources ?? "");
    setBugIssueType(src.bugIssueType ?? "");
    setBugProjectMode(src.bugProjectMode || "test");
    setBugProjectKey(src.bugProjectKey ?? "");
    // TLS travels with the URL it was configured for. Copying the server but
    // not its certificate settings produces a profile that cannot connect.
    setCaCert(src.caCert ?? "");
    setAllowUntrustedTLS(src.allowUntrustedTls ?? false);
  }

  const canTest =
    jiraUrl.trim() !== "" &&
    urlError === "" &&
    !kiwiCredInvalid &&
    // Connections have no "test the stored credential" endpoint (unlike
    // TestProfileConnection) -- testing always needs a freshly typed one.
    (credEnteredForCreate || (isEdit && !isConnection));
  const canSave =
    name.trim() !== "" &&
    jiraUrl.trim() !== "" &&
    urlError === "" &&
    projectKey.trim() !== "" &&
    keyError === "" &&
    tokenSatisfied &&
    !kiwiCredInvalid &&
    !bugTrackerPartial &&
    bugTrackerUrlError === "" &&
    bugTrackerLoadError === "";

  // Warn when an edit changes the project/URL/backend — the cached data will
  // be cleared (app.go's UpdateProfile purges the store cache on any of the
  // three). Profile-only: connection edits don't carry the same
  // profile-switch cache implications, and profile!/connection! below stay
  // correctly narrowed.
  const willClearCache =
    !isConnection &&
    isEdit &&
    (projectKey.trim() !== profile!.projectKey ||
      normalizeJiraUrl(jiraUrl) !== profile!.jiraUrl ||
      backend !== profile!.backend);

  return { confirm, name, setName, backend, setBackend, jiraUrl, setJiraUrl, projectKey, setProjectKey, scopeJql, setScopeJql, crossProjectSources, setCrossProjectSources, bugIssueType, setBugIssueType, bugProjectMode, setBugProjectMode, bugProjectKey, setBugProjectKey, role, setRole, token, setToken, kiwiUsername, setKiwiUsername, kiwiPassword, setKiwiPassword, showToken, setShowToken, reuseFrom, setReuseFrom, caCert, setCaCert, allowUntrustedTLS, setAllowUntrustedTLS, bugConnId, setBugConnId, bugConnLoadedUrl, setBugConnLoadedUrl, bugConnLoadedProjectKey, setBugConnLoadedProjectKey, bugConnLoadedIssueType, setBugConnLoadedIssueType, bugConnLoadedCaCert, setBugConnLoadedCaCert, bugConnLoadedAllowUntrustedTLS, setBugConnLoadedAllowUntrustedTLS, bugTrackerUrl, setBugTrackerUrl, bugTrackerProjectKey, setBugTrackerProjectKey, bugTrackerIssueType, setBugTrackerIssueType, bugTrackerToken, setBugTrackerToken, bugTrackerCaCert, setBugTrackerCaCert, bugTrackerAllowUntrustedTLS, setBugTrackerAllowUntrustedTLS, bugTrackerLoadError, setBugTrackerLoadError, testing, setTesting, testResult, setTestResult, testOk, setTestOk, saving, setSaving, error, setError, isConnection, isEdit, others, seedBackend, backendIsKiwi, reusable, loadedBugConnFor, keyError, urlError, bugTrackerUrlError, bugTrackerPartial, kiwiUser, kiwiPass, kiwiCredComplete, kiwiCredInvalid, credEnteredForCreate, tokenSatisfied, canTest, canSave, willClearCache, currentToken, copyFrom };
}

export type ProfileFormState = ReturnType<typeof useProfileFormState>;
