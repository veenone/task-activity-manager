import type { ReactNode } from "react";
import type { ProfileFormApi } from "./useProfileForm";

interface Props {
  f: ProfileFormApi;
  // The Save button's label when creating ("Create profile", "Add connection").
  createLabel: string;
  // What the app tells the user an edited key or URL clears.
  cacheWarning: string;
  onCancel?: () => void;
  extraActions?: ReactNode;
  // A notice above the error, for an app field that holds Save back.
  beforeError?: ReactNode;
}

// ProfileFormFooter is the cache warning, Test connection with its result, the
// save error, and the Export / Delete, Cancel and Save buttons.
export function ProfileFormFooter({ f, createLabel, cacheWarning, onCancel, extraActions, beforeError }: Props) {
  return (
    <>
      {f.willClearCache && <div className="form-warning">{cacheWarning}</div>}

      <div className="form-actions">
        <button className="btn" onClick={() => void f.test()} disabled={!f.canTest || f.testing}>
          {f.testing ? "Testing…" : "Test connection"}
        </button>
        {f.testResult && <span className={f.testOk ? "ok-text" : "error-text"}>{f.testResult}</span>}
      </div>

      {beforeError}

      {f.error && <div className="error-text" role="alert">{f.error}</div>}

      <div className="form-actions form-actions-end">
        {extraActions && <div className="profile-form-extra">{extraActions}</div>}
        {onCancel && (
          <button className="btn" onClick={onCancel} disabled={f.saving}>
            Cancel
          </button>
        )}
        <button className="btn btn-primary" onClick={() => void f.save()} disabled={!f.canSave || f.saving}>
          {f.saving ? "Saving…" : f.isEdit ? "Save changes" : createLabel}
        </button>
      </div>
    </>
  );
}
