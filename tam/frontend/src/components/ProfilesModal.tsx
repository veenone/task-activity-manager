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
export function ProfilesModal({ onClose }: { onClose: () => void }) {
  const { profiles, defaultProfileId, reload, setDefault } = useProfile<Profile, Settings>();
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
    <Modal onClose={onClose} className="modal profiles-modal" labelledBy="profiles-title">
      <div className="pending-head">
        <h2 id="profiles-title">Profiles</h2>
        <button className="btn btn-ghost" onClick={onClose} aria-label="Close">×</button>
      </div>
      <div className="bulk-body">
        <p className="muted">
          One list for the whole suite. A profile made here shows up in Xray Test Manager too.
        </p>
        <ul className="profile-list">
          {profiles.map((p) => (
            <li key={p.id} className="profile-row">
              <span className="profile-name">
                {p.name} ({p.projectKey})
              </span>
              <span className="muted">
                {isDemoUrl(p.jiraUrl) ? "demo" : p.jiraUrl}
                {p.id === defaultProfileId ? " · default" : ""}
              </span>
              <RequirementTypeField profile={p} />
              <button className="btn" onClick={() => setDefault(p.id)}>
                {p.id === defaultProfileId ? "Clear default" : "Make default"}
              </button>
              <button className="btn btn-danger" onClick={() => remove(p)}>
                Delete
              </button>
            </li>
          ))}
          {profiles.length === 0 && <li className="muted">No profiles yet.</li>}
        </ul>
        <p className="muted small">
          Kiwi TCMS profiles from XTM are not listed; TAM talks to Jira only. The requirement
          field is the Jira issue type name TAM syncs as a requirement; leave it empty for
          "Requirement".
        </p>

        <form onSubmit={submit} className="profile-form">
          <h3>New profile</h3>
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
          {error && <p className="form-error" role="alert">{error}</p>}
          <div className="form-actions form-actions-end">
            <button type="button" className="btn" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={saving}>Save</button>
          </div>
        </form>
      </div>
    </Modal>
  );
}
