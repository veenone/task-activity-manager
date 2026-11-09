import { useEffect, useState } from "react";
import { Modal } from "@agile-suite/core";
import { ConfluenceSettings, SetConfluenceSettings } from "../wailsjs/go/main/App";
import type { profile } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";

const FIELDS: { key: "baseURL" | "spaceKey" | "rootPageID"; label: string }[] = [
  { key: "baseURL", label: "Confluence URL" },
  { key: "spaceKey", label: "Space key" },
  { key: "rootPageID", label: "Parent page id" },
];

// ConfluenceDialog says where the profile's cycles are published. The token
// goes to the credential store; an empty one keeps what is stored.
export function ConfluenceDialog({ profileId, onClose }: { profileId: string; onClose: () => void }) {
  const [draft, setDraft] = useState<profile.ConfluenceConfig | null>(null);
  const [token, setToken] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    ConfluenceSettings(profileId).then(setDraft, (e) => setError(errorText(e)));
  }, [profileId]);

  async function save() {
    if (!draft) return;
    setError("");
    try {
      await SetConfluenceSettings(profileId, draft, token);
      onClose();
    } catch (e) {
      setError(errorText(e));
    }
  }

  return (
    <Modal onClose={onClose} labelledBy="pie-confluence">
      <form
        className="profile-form"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <h2 id="pie-confluence">Confluence settings</h2>
        <p className="muted">Published cycles go under the parent page, or at the top of the space when it is empty.</p>
        {draft &&
          FIELDS.map(({ key, label }) => (
            <label key={key}>
              {label}
              <input value={draft[key]} onChange={(e) => setDraft({ ...draft, [key]: e.target.value } as profile.ConfluenceConfig)} />
            </label>
          ))}
        <label>
          Token
          <input
            type="password"
            value={token}
            placeholder="Leave empty to keep the stored token"
            onChange={(e) => setToken(e.target.value)}
          />
        </label>
        {error && <p className="error-text">{error}</p>}
        <div className="form-actions form-actions-end">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn btn-primary" disabled={!draft}>
            Save
          </button>
        </div>
      </form>
    </Modal>
  );
}
