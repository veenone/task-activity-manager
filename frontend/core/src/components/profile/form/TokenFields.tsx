import type { ReactNode } from "react";
import type { ProfileFormApi, ReuseCandidate } from "./useProfileForm";

// How the apps word the "start from another profile" choice. TAM reuses only
// the token ("Reuse token from"); XTM copies the settings too ("Copy of").
export interface ReuseWording<C extends ReuseCandidate = ReuseCandidate> {
  label: string;
  blank: string;
  option: (p: C) => string;
  hint?: ReactNode;
}

interface Props<C extends ReuseCandidate> {
  f: ProfileFormApi<C>;
  reuse: ReuseWording<C>;
  // Runs when a profile is picked to start from, after the choice is
  // recorded. XTM copies that profile's settings here.
  onReuse?: (id: string) => void;
  // Shown instead of the token field (XTM's Kiwi username and password).
  credential?: ReactNode;
}

// TokenFields is the choice to reuse another profile's credential (create
// only, when there is one to reuse) and the personal access token with its
// show and hide toggle. The token field goes away once a profile is reused.
export function TokenFields<C extends ReuseCandidate>({ f, reuse, onReuse, credential }: Props<C>) {
  const offerReuse = !f.isEdit && f.reusable.length > 0;
  const placeholder = f.isEdit
    ? "Leave blank to keep the current token"
    : f.demoFree
      ? "not needed for a demo profile"
      : "Jira PAT (stored in Windows Credential Manager)";
  return (
    <>
      {offerReuse && (
        <label>
          {reuse.label}
          <select
            value={f.reuseFrom}
            onChange={(e) => {
              f.setReuseFrom(e.target.value);
              onReuse?.(e.target.value);
            }}
          >
            <option value="">{reuse.blank}</option>
            {f.reusable.map((p) => (
              <option key={p.id} value={p.id}>{reuse.option(p)}</option>
            ))}
          </select>
          {reuse.hint && <span className="field-hint muted">{reuse.hint}</span>}
        </label>
      )}

      {(f.isEdit || f.reuseFrom === "") && (credential ?? (
        <label>
          {offerReuse ? "New token" : "Personal Access Token"}
          <div className="pat-field">
            <input
              type={f.showToken ? "text" : "password"}
              value={f.token}
              onChange={(e) => f.setToken(e.target.value)}
              placeholder={placeholder}
              autoComplete="off"
            />
            <button
              type="button"
              className="btn btn-ghost pat-toggle"
              onClick={() => f.setShowToken((v) => !v)}
              title={f.showToken ? "Hide token" : "Show token"}
              aria-label={f.showToken ? "Hide token" : "Show token"}
            >
              {f.showToken ? "🙈" : "👁"}
            </button>
          </div>
        </label>
      ))}
    </>
  );
}
