import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Modal, useConfirm, useProfile, errMsg } from "@agile-suite/core";
import { CreateProfile, DeleteProfile, GetProfileSetting, SetProfileSetting, isDemoUrl } from "../api";
import type { Profile, Settings } from "../api";

const REQUIREMENT_TYPE_KEY = "requirement_issue_type";

// RequirementTypeField edits the one TAM setting a profile has today: the
// Jira issue type name TAM treats as a requirement. It saves on blur or
// Enter and shows the backend's default as the placeholder.
function RequirementTypeField({ profile }: { profile: Profile }) {
  const [value, setValue] = useState("");
  const [saved, setSaved] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    GetProfileSetting(profile.id, REQUIREMENT_TYPE_KEY)
      .then((v) => {
        if (live) {
          setValue(v);
          setSaved(v);
        }
      })
      .catch((e) => live && setError(errMsg(e)));
    return () => {
      live = false;
    };
  }, [profile.id]);

  async function save() {
    const next = value.trim();
    if (next === saved) return;
    try {
      await SetProfileSetting(profile.id, REQUIREMENT_TYPE_KEY, next);
      setSaved(next);
      setError("");
    } catch (e) {
      setError(errMsg(e));
    }
  }

  return (
    <span className="profile-setting">
      <input
        className="detail-input detail-input-inline"
        aria-label={`Requirement issue type for ${profile.name}`}
        placeholder="Requirement"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={() => void save()}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            void save();
          }
        }}
      />
      {error && <span className="error-text small">{error}</span>}
    </span>
  );
}

// ProfilesModal lists the profiles the suite shares and creates or deletes
// them. Deleting removes the row from XTM as well, so the confirm says so.
// Structure and class names mirror XTM's Manage Profiles dialog: the head,
// the list, and its rows are the same markup; TAM has no per-profile edit
// view to open, so the per-row requirement field and Delete stay on the row
// itself, and the create form always sits below the list instead of behind
// a "Create new profile" step.
export function ProfilesModal({ onClose }: { onClose: () => void }) {
  const { profiles, activeId, defaultProfileId, reload, setDefault } = useProfile<Profile, Settings>();
  const { confirm } = useConfirm();
  const [name, setName] = useState("");
  const [jiraUrl, setJiraUrl] = useState("");
  const [projectKey, setProjectKey] = useState("");
  const [token, setToken] = useState("");
  const [makeDefault, setMakeDefault] = useState(false);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await CreateProfile(name, jiraUrl, projectKey, token, makeDefault);
      await reload();
      setName("");
      setJiraUrl("");
      setProjectKey("");
      setToken("");
      setMakeDefault(false);
    } catch (err) {
      setError(errMsg(err));
    } finally {
      setSaving(false);
    }
  }

  async function remove(p: Profile) {
    const ok = await confirm({
      title: `Delete ${p.name}?`,
      message: "This removes the profile from Xray Test Manager too, along with its stored token.",
      confirmLabel: "Delete",
      danger: true,
    });
    if (!ok) return;
    try {
      await DeleteProfile(p.id);
      await reload();
    } catch (err) {
      setError(errMsg(err));
    }
  }

  return (
    <Modal onClose={onClose} className="modal profiles-modal" labelledBy="profiles-modal-title">
      <div className="profiles-modal-head">
        <div className="profiles-modal-head-text">
          <h2 id="profiles-modal-title">Manage Profiles</h2>
          <span className="profiles-modal-sub">
            One list for the whole suite. A profile made here shows up in Xray Test Manager too.
          </span>
        </div>
        <button className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">
          ✕
        </button>
      </div>

      <div className="profiles-list-label">Your profiles</div>
      <ul className="profiles-list-items">
        {profiles.map((p) => (
          <li key={p.id} className="profiles-row-item">
            <div className="profiles-list-row">
              <button
                className="profiles-star"
                title={
                  defaultProfileId === p.id
                    ? "Default on launch (click to clear)"
                    : "Set as default on launch"
                }
                aria-label={defaultProfileId === p.id ? "Clear default" : "Set as default"}
                onClick={() => setDefault(p.id)}
              >
                {defaultProfileId === p.id ? "★" : "☆"}
              </button>
              <span className="profiles-row-name">{p.name}</span>
              <span className="profiles-row-key muted">({p.projectKey})</span>
              {p.id === activeId && <span className="profiles-badge-active">Active</span>}
              <button className="btn btn-danger" onClick={() => remove(p)}>
                Delete
              </button>
            </div>
            <div className="profiles-row-meta muted small">
              {isDemoUrl(p.jiraUrl) ? "demo" : p.jiraUrl}
            </div>
            <RequirementTypeField profile={p} />
          </li>
        ))}
        {profiles.length === 0 && <li className="muted">No profiles yet.</li>}
      </ul>

      <p className="muted small">
        Kiwi TCMS profiles from XTM are not listed; TAM talks to Jira only. The requirement
        field is the Jira issue type name TAM syncs as a requirement; leave it empty for
        "Requirement". Changing it resets the sync cursor, so the next sync pulls
        everything again; run a Full sync to drop rows of the old type.
      </p>

      <div className="profiles-mode-head">
        <span className="profiles-mode-kicker profiles-mode-kicker-new">New profile</span>
      </div>
      <form onSubmit={submit} className="profile-form">
        <label className="field-label" htmlFor="pf-name">Name</label>
        <input id="pf-name" className="detail-input" value={name} onChange={(e) => setName(e.target.value)} />
        <label className="field-label" htmlFor="pf-url">Jira URL</label>
        <input id="pf-url" className="detail-input" value={jiraUrl} onChange={(e) => setJiraUrl(e.target.value)} placeholder="https://jira.example.com or demo" />
        <label className="field-label" htmlFor="pf-key">Project key</label>
        <input id="pf-key" className="detail-input" value={projectKey} onChange={(e) => setProjectKey(e.target.value.toUpperCase())} />
        <label className="field-label" htmlFor="pf-token">Personal access token</label>
        <input id="pf-token" className="detail-input" type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder={isDemoUrl(jiraUrl) ? "not needed for demo" : "stored in the OS credential manager"} />
        <label className="check-row">
          <input type="checkbox" checked={makeDefault} onChange={(e) => setMakeDefault(e.target.checked)} />
          Make this the default profile
        </label>
        {error && <p className="error-text" role="alert">{error}</p>}
        <div className="form-actions form-actions-end">
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={saving}>Save</button>
        </div>
      </form>
    </Modal>
  );
}
