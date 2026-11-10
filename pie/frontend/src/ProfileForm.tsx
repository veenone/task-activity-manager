import { useEffect, useState } from "react";
import type { ProfileFormSlot } from "@agile-suite/core";
import { ConfluenceSettings, CreateProfile, SetConfluenceSettings, UpdateProfile } from "../wailsjs/go/main/App";
import type { profile } from "../wailsjs/go/models";

// errorText reads what a bound Go method rejected with: Wails sends the error's
// text as a plain string.
export function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

// isDemo matches demo.Is in pie/internal/demo: the URL that selects invented data.
export function isDemo(url: string): boolean {
  return url.trim().toLowerCase() === "demo";
}

const CONFLUENCE: { key: "baseURL" | "spaceKey" | "rootPageID"; label: string }[] = [
  { key: "baseURL", label: "Confluence URL" },
  { key: "spaceKey", label: "Space key" },
  { key: "rootPageID", label: "Parent page id" },
];

const NO_CONFLUENCE: profile.ConfluenceConfig = { baseURL: "", spaceKey: "", rootPageID: "", reportsSpaceKey: "", reportsRootPageID: "" };

// ProfileForm is PIE's form inside core's Manage Profiles (#453): the Jira
// connection, then where the profile's cycles are published. A PIE profile
// has no project key, which is why it is not core's shared form. Tokens go
// to the credential store; on an edit an empty one keeps what is stored.
export function ProfileForm({ profile: editing, onSaved, onCancel, extraActions }: ProfileFormSlot<profile.Profile>) {
  const [name, setName] = useState(editing?.name ?? "");
  const [url, setUrl] = useState(editing?.jiraUrl ?? "");
  const [token, setToken] = useState("");
  // null until an edited profile's settings arrive, so Save cannot write
  // blanks over them.
  const [cc, setCc] = useState<profile.ConfluenceConfig | null>(editing ? null : NO_CONFLUENCE);
  const [ccToken, setCcToken] = useState("");
  // The id of a profile this form created, so a retry after a failed
  // Confluence save edits it instead of creating a second one.
  const [id, setId] = useState(editing?.id ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const demo = isDemo(url);

  useEffect(() => {
    if (!editing) return;
    let current = true;
    ConfluenceSettings(editing.id).then(
      (got) => current && setCc(got),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [editing]);

  async function save() {
    setSaving(true);
    setError("");
    try {
      const jiraToken = demo ? "" : token;
      const p = id ? await UpdateProfile(id, name, url, jiraToken) : await CreateProfile(name, url, jiraToken);
      setId(p.id);
      // A new profile without Confluence stores nothing for it.
      if (cc && (editing || cc.baseURL.trim() !== "" || ccToken !== "")) {
        await SetConfluenceSettings(p.id, cc, ccToken);
      }
      onSaved(p);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <form
      className="profile-form"
      onSubmit={(e) => {
        e.preventDefault();
        void save();
      }}
    >
      <label>
        Name
        <input value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <label>
        Jira URL
        <input value={url} placeholder="https://jira.example.com" onChange={(e) => setUrl(e.target.value)} />
      </label>
      {demo ? (
        <p className="muted">Demo shows invented issues and needs no token.</p>
      ) : (
        <label>
          Personal access token
          <input
            type="password"
            autoComplete="off"
            value={token}
            placeholder={id ? "Leave empty to keep the stored token" : ""}
            onChange={(e) => setToken(e.target.value)}
          />
        </label>
      )}
      <fieldset className="pie-profile-confluence">
        <legend>Confluence</legend>
        <p className="muted">Published cycles go under the parent page, or at the top of the space when it is empty.</p>
        {cc &&
          CONFLUENCE.map(({ key, label }) => (
            <label key={key}>
              {label}
              <input value={cc[key]} onChange={(e) => setCc({ ...cc, [key]: e.target.value })} />
            </label>
          ))}
        <label>
          Confluence token
          <input
            type="password"
            autoComplete="off"
            value={ccToken}
            placeholder={id ? "Leave empty to keep the stored token" : ""}
            onChange={(e) => setCcToken(e.target.value)}
          />
        </label>
      </fieldset>
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
      <div className="form-actions form-actions-end">
        {extraActions}
        {onCancel && (
          <button type="button" className="btn" onClick={onCancel}>
            Cancel
          </button>
        )}
        <button type="submit" className="btn btn-primary" disabled={saving || !cc}>
          Save
        </button>
      </div>
    </form>
  );
}
