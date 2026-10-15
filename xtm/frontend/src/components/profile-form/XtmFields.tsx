import type { XtmProfileExtras } from "./useXtmProfileExtras";

// BackendFields are a connection's role and the backend select. Changing the
// backend clears the "start from" choice, since that list only offers
// profiles on the same backend.
export function BackendFields({ x, isConnection, onBackendChange }: { x: XtmProfileExtras; isConnection: boolean; onBackendChange: () => void }) {
  return (
    <>
      {isConnection && (
        <label>
          Role
          <select value={x.role} onChange={(e) => x.setRole(e.target.value)}>
            <option value="target">Target (publish destination)</option>
            <option value="source">Source (pulled from)</option>
            <option value="both">Both</option>
          </select>
        </label>
      )}
      <label>
        Backend
        <select
          value={x.backend}
          onChange={(e) => {
            x.setBackend(e.target.value);
            onBackendChange();
          }}
        >
          <option value="xray">Xray / Jira</option>
          <option value="kiwi">Kiwi TCMS</option>
        </select>
      </label>
    </>
  );
}

// XrayFields are the Xray-only settings: cross-project sources (profiles
// only) and where bugs are filed.
export function XrayFields({ x, isConnection }: { x: XtmProfileExtras; isConnection: boolean }) {
  if (x.isKiwi) return null;
  return (
    <>
      {!isConnection && (
        <label>
          Cross-project source projects (optional)
          <input
            value={x.crossProjectSources}
            onChange={(e) => x.setCrossProjectSources(e.target.value)}
            placeholder="e.g. PROJVAL, JKTEE"
            spellCheck={false}
          />
          <span className="field-hint">
            Project keys you can link preconditions, test calls, and cloned steps
            from. Separate multiple keys with a comma or space (e.g. "PROJVAL,
            JKTEE"). Leave blank to disable cross-project linking.
          </span>
        </label>
      )}
      <label>
        Bug issue type
        <input
          value={x.bugIssueType}
          onChange={(e) => x.setBugIssueType(e.target.value)}
          placeholder="Bug (the Jira issue type used when filing a defect)"
          spellCheck={false}
        />
      </label>
      <label>
        Bug project
        <select value={x.bugProjectMode} onChange={(e) => x.setBugProjectMode(e.target.value)}>
          <option value="test">Same as test (the test's project)</option>
          <option value="execution">Same as the Test Execution</option>
          <option value="dedicated">Dedicated project…</option>
        </select>
      </label>
      {x.bugProjectMode === "dedicated" && (
        <label>
          Dedicated bug project key
          <input
            value={x.bugProjectKey}
            onChange={(e) => x.setBugProjectKey(e.target.value.toUpperCase())}
            placeholder="e.g. DEFECTS (the project where bugs are filed)"
            spellCheck={false}
          />
        </label>
      )}
    </>
  );
}

// KiwiCredentialFields replace the token field on a Kiwi profile.
export function KiwiCredentialFields({ x, isEdit }: { x: XtmProfileExtras; isEdit: boolean }) {
  return (
    <>
      <label>
        Kiwi username
        <input
          value={x.kiwiUsername}
          onChange={(e) => x.setKiwiUsername(e.target.value)}
          placeholder={isEdit ? "Leave blank (with password) to keep the current credential" : "Kiwi TCMS username"}
          autoComplete="off"
          spellCheck={false}
        />
      </label>
      <label>
        Kiwi password
        <div className="pat-field">
          <input
            type={x.showPassword ? "text" : "password"}
            value={x.kiwiPassword}
            onChange={(e) => x.setKiwiPassword(e.target.value)}
            placeholder={isEdit ? "Leave blank (with username) to keep the current credential" : "Kiwi TCMS password"}
            autoComplete="off"
          />
          <button
            type="button"
            className="btn btn-ghost pat-toggle"
            onClick={() => x.setShowPassword((v) => !v)}
            title={x.showPassword ? "Hide password" : "Show password"}
            aria-label={x.showPassword ? "Hide password" : "Show password"}
          >
            {x.showPassword ? "🙈" : "👁"}
          </button>
        </div>
        {x.kiwiCredInvalid && (
          <span className="field-error">
            Enter both username and password, or leave both blank to keep
            the current credential.
          </span>
        )}
      </label>
    </>
  );
}
