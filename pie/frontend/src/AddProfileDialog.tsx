import { useState } from "react";
import { Modal } from "@agile-suite/core";
import { CreateProfile } from "../wailsjs/go/main/App";
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

export function AddProfileDialog({
  onClose,
  onSaved,
}: {
  onClose: () => void;
  onSaved: (saved: profile.Profile) => void;
}) {
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [token, setToken] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const demo = isDemo(url);

  async function save() {
    setSaving(true);
    setError("");
    try {
      onSaved(await CreateProfile(name, url, demo ? "" : token));
    } catch (e) {
      setError(errorText(e));
      setSaving(false);
    }
  }

  return (
    <Modal onClose={onClose} labelledBy="pie-add-profile">
      <form
        className="profile-form"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <h2 id="pie-add-profile">Add profile</h2>
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
            <input type="password" autoComplete="off" value={token} onChange={(e) => setToken(e.target.value)} />
          </label>
        )}
        {error && <p className="error-text">{error}</p>}
        <div className="form-actions form-actions-end">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={saving}>
            Save
          </button>
        </div>
      </form>
    </Modal>
  );
}
