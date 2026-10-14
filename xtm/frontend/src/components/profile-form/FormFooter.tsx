import type { ProfileFormProps, ProfileFormState } from "./useProfileFormState";
import type { ProfileFormActions } from "./useProfileFormActions";

// FormFooter is the cache warning, Test connection with its result, the save
// error, and Cancel and Save.
export function FormFooter({ s, a, p }: { s: ProfileFormState; a: ProfileFormActions; p: ProfileFormProps }) {
  const { testing, testResult, testOk, saving, error, isConnection, isEdit, canTest, canSave, willClearCache } = s;
  const { test, save } = a;
  const { onCancel, extraActions } = p;
  return (
    <>
      {willClearCache && (
        <div className="form-warning">
          Changing the project key or Jira URL clears this profile's cached
          data. Re-sync afterwards to pull in the new project.
        </div>
      )}

      <div className="form-actions">
        <button className="btn" onClick={test} disabled={!canTest || testing}>
          {testing ? "Testing…" : "Test connection"}
        </button>
        {testResult && (
          <span className={testOk ? "ok-text" : "error-text"}>
            {testResult}
          </span>
        )}
      </div>

      {error && <div className="error-text">{error}</div>}

      <div className="form-actions form-actions-end">
        {extraActions && <div className="profile-form-extra">{extraActions}</div>}
        {onCancel && (
          <button className="btn" onClick={onCancel} disabled={saving}>
            Cancel
          </button>
        )}
        <button
          className="btn btn-primary"
          onClick={save}
          disabled={!canSave || saving}
        >
          {saving
            ? "Saving…"
            : isEdit
              ? "Save changes"
              : isConnection
                ? "Add connection"
                : "Create profile"}
        </button>
      </div>
    </>
  );
}
