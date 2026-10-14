import type { ProfileFormState } from "./useProfileFormState";

// CredentialFields are Start from, and the Kiwi username and password or the
// Jira personal access token.
export function CredentialFields({ s }: { s: ProfileFormState }) {
  const { token, setToken, kiwiUsername, setKiwiUsername, kiwiPassword, setKiwiPassword, showToken, setShowToken, reuseFrom, isEdit, backendIsKiwi, reusable, kiwiCredInvalid, copyFrom } = s;
  return (
    <>
      {!isEdit && reusable.length > 0 && (
        <label>
          Start from
          <select value={reuseFrom} onChange={(e) => copyFrom(e.target.value)}>
            <option value="">A blank profile</option>
            {reusable.map((p) => (
              <option key={p.id} value={p.id}>
                Copy of: {p.name} ({p.projectKey})
              </option>
            ))}
          </select>
          <span className="field-hint muted">
            Copies the settings below and reuses the stored{" "}
            {backendIsKiwi ? "credential" : "token"}. Everything stays editable.
          </span>
        </label>
      )}

      {(isEdit || reuseFrom === "") && backendIsKiwi && (
        <>
          <label>
            Kiwi username
            <input
              value={kiwiUsername}
              onChange={(e) => setKiwiUsername(e.target.value)}
              placeholder={
                isEdit
                  ? "Leave blank (with password) to keep the current credential"
                  : "Kiwi TCMS username"
              }
              autoComplete="off"
              spellCheck={false}
            />
          </label>
          <label>
            Kiwi password
            <div className="pat-field">
              <input
                type={showToken ? "text" : "password"}
                value={kiwiPassword}
                onChange={(e) => setKiwiPassword(e.target.value)}
                placeholder={
                  isEdit
                    ? "Leave blank (with username) to keep the current credential"
                    : "Kiwi TCMS password"
                }
                autoComplete="off"
              />
              <button
                type="button"
                className="btn btn-ghost pat-toggle"
                onClick={() => setShowToken((v) => !v)}
                title={showToken ? "Hide password" : "Show password"}
                aria-label={showToken ? "Hide password" : "Show password"}
              >
                {showToken ? "🙈" : "👁"}
              </button>
            </div>
            {kiwiCredInvalid && (
              <span className="field-error">
                Enter both username and password, or leave both blank to keep
                the current credential.
              </span>
            )}
          </label>
        </>
      )}

      {(isEdit || reuseFrom === "") && !backendIsKiwi && (
        <label>
          {reusable.length > 0 && !isEdit ? "New token" : "Personal Access Token"}
          <div className="pat-field">
            <input
              type={showToken ? "text" : "password"}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={
                isEdit
                  ? "Leave blank to keep the current token"
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
    </>
  );
}
