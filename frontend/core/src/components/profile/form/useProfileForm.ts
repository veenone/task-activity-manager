import { useId, useState } from "react";
import { errMsg } from "../../../lib/errMsg";
import { jiraUrlError, normalizeJiraUrl, projectKeyError } from "../profileValidation";

// What the form hands the app's adapter: every shared field, cleaned. url is
// normalised, key is upper-cased in "jira" key mode, and token is the
// credential to send ("" on edit means keep the stored one).
export interface ProfileFormFields {
  name: string;
  url: string;
  key: string;
  scope: string;
  token: string;
  caCert: string;
  allowTls: boolean;
  // The profile whose stored credential a new profile reuses, or "".
  reuseFrom: string;
}

// The two calls that differ between the apps. test resolves to the account
// name it connected as; stored is true when the form is editing and no new
// credential was typed, so the app tests the credential it already has.
// save resolves to nothing, or to a warning when the main write went through
// but something after it did not.
export interface ProfileFormAdapter {
  test(fields: ProfileFormFields & { stored: boolean }): Promise<string>;
  save(fields: ProfileFormFields): Promise<string | void>;
}

// A profile the new one can start from or reuse a credential of.
export interface ReuseCandidate {
  id: string;
  name: string;
  projectKey: string;
}

// An app that does not use the token field (XTM's Kiwi login) reports its own
// credential: the value to send, whether one was typed, and whether what was
// typed is unusable.
export interface CredentialOverride {
  value: string;
  entered: boolean;
  invalid: boolean;
}

export interface ProfileFormOptions<C extends ReuseCandidate = ReuseCandidate> {
  isDemoUrl: (url: string) => boolean;
  // The stored values when editing.
  seed?: { name: string; url: string; key: string; scope: string; caCert: string; allowTls: boolean };
  isEdit?: boolean;
  // "jira" upper-cases the key and checks its shape; "free" takes it as typed.
  keyMode?: "jira" | "free";
  checkKeyPattern?: boolean;
  // A demo URL needs no credential (TAM). XTM asks for one either way.
  demoNeedsNoToken?: boolean;
  // Whether an edit can be tested against the stored credential. A workspace
  // connection has no such call, so it always needs a typed one.
  canTestStored?: boolean;
  reuseCandidates?: C[];
  credential?: CredentialOverride;
  // The app's own fields are valid; Save waits for them.
  extraValid?: boolean;
  // The stored key and URL; changing either clears the profile's cache.
  cacheBaseline?: { key: string; url: string };
  // Something else the app knows clears the cache, such as a new backend.
  clearsCacheToo?: boolean;
  adapter: ProfileFormAdapter;
}

// useProfileForm holds the fields both apps' profile forms share, their
// validation, and the test and save flow. Everything app-specific reaches it
// through the options: the adapter for the calls, a credential override, and
// extraValid for fields the app renders itself.
export function useProfileForm<C extends ReuseCandidate = ReuseCandidate>(o: ProfileFormOptions<C>) {
  const keyMode = o.keyMode ?? "jira";
  const isEdit = !!o.isEdit;
  const [name, setName] = useState(o.seed?.name ?? "");
  const [url, setUrl] = useState(o.seed?.url ?? "");
  const [key, setKeyRaw] = useState(o.seed?.key ?? "");
  const [scope, setScope] = useState(o.seed?.scope ?? "");
  const [token, setToken] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [reuseFrom, setReuseFrom] = useState("");
  const [caCert, setCaCert] = useState(o.seed?.caCert ?? "");
  const [allowTls, setAllowTls] = useState(o.seed?.allowTls ?? false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState("");
  const [testOk, setTestOk] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const urlErrorId = useId();
  const keyErrorId = useId();

  const setKey = (v: string) => setKeyRaw(keyMode === "jira" ? v.toUpperCase() : v);
  const demo = o.isDemoUrl(normalizeJiraUrl(url));
  const urlError = jiraUrlError(url, o.isDemoUrl);
  const keyError = keyMode === "free" ? "" : projectKeyError(key, o.checkKeyPattern ?? true);
  const reusable = o.reuseCandidates ?? [];

  const cred = o.credential ?? { value: token.trim(), entered: token.trim() !== "", invalid: false };
  const demoFree = !!o.demoNeedsNoToken && demo;
  const tokenSatisfied = isEdit || demoFree || reuseFrom !== "" || cred.entered;
  const canTest =
    url.trim() !== "" && urlError === "" && !cred.invalid &&
    (cred.entered || demoFree || (isEdit && (o.canTestStored ?? true)));
  const canSave =
    name.trim() !== "" && url.trim() !== "" && urlError === "" &&
    key.trim() !== "" && keyError === "" && tokenSatisfied && !cred.invalid &&
    (o.extraValid ?? true);
  const willClearCache =
    !!o.cacheBaseline &&
    (key.trim() !== o.cacheBaseline.key || normalizeJiraUrl(url) !== o.cacheBaseline.url || !!o.clearsCacheToo);

  function fields(): ProfileFormFields {
    return {
      name: name.trim(),
      url: normalizeJiraUrl(url),
      key: keyMode === "jira" ? key.trim().toUpperCase() : key.trim(),
      scope: scope.trim(),
      token: cred.value,
      caCert: caCert.trim(),
      allowTls,
      reuseFrom,
    };
  }

  async function test() {
    setTesting(true);
    setTestResult("");
    setTestOk(false);
    try {
      const user = await o.adapter.test({ ...fields(), stored: isEdit && cred.value === "" });
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
      const warning = await o.adapter.save(fields());
      if (warning) setError(warning);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }

  return {
    name, setName, url, setUrl, key, setKey, scope, setScope, token, setToken, showToken, setShowToken,
    reuseFrom, setReuseFrom, caCert, setCaCert, allowTls, setAllowTls,
    testing, testResult, testOk, saving, error, setError,
    isEdit, demo, demoFree, keyMode, urlError, keyError, urlErrorId, keyErrorId, reusable,
    canTest, canSave, willClearCache, fields, test, save,
  };
}

export type ProfileFormApi<C extends ReuseCandidate = ReuseCandidate> = ReturnType<typeof useProfileForm<C>>;
